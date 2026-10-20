package rank

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/database"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/riotwatch"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// Errors
var (
	ErrMaxBindingsReached  = errors.New("maximum bindings reached (3)")
	ErrAlreadyBound        = errors.New("this Riot account is already bound to another user")
	ErrAlreadyBoundToYou   = errors.New("this Riot account is already bound to your account")
	ErrBindingNotFound     = errors.New("binding not found")
	ErrVerificationExpired = errors.New("verification expired, please start again with /rank bind")
	ErrVerificationFailed  = errors.New("verification failed: profile icon does not match")
	ErrWrongChannel        = errors.New("binding commands can only be used in the designated channel")
	ErrNoPendingVerify     = errors.New("no pending verification found, please use /rank bind first")
)

// Service handles rank binding business logic
type Service struct {
	repo      *Repository
	apiClient *RankAPIClient
	db        *database.DB
}

// NewService creates a new Service
func NewService(db *database.DB, riotClient *riotwatch.RiotAPIClient) *Service {
	return &Service{
		repo:      NewRepository(db),
		apiClient: NewRankAPIClient(riotClient),
		db:        db,
	}
}

// StartBinding initiates the binding process with verification
// Returns the binding and the verification icon ID
func (s *Service) StartBinding(ctx context.Context, discordID, channelID, riotID, riotTag string) (*RiotBinding, int, error) {
	// Check if in correct channel
	if channelID != BindingChannelID {
		return nil, 0, ErrWrongChannel
	}

	// Check binding count (including unverified ones to prevent spam)
	count, err := s.repo.CountBindingsByDiscordID(discordID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count bindings: %w", err)
	}
	if count >= MaxBindingsPerUser {
		return nil, 0, ErrMaxBindingsReached
	}

	// Verify Riot ID exists via API
	account, err := s.apiClient.GetAccountByRiotID(ctx, riotID, riotTag)
	if err != nil {
		if IsNotFound(err) {
			return nil, 0, fmt.Errorf("Riot account not found: %s#%s", riotID, riotTag)
		}
		return nil, 0, fmt.Errorf("failed to verify Riot account: %w", err)
	}

	// Check if already bound to any user
	existing, err := s.repo.GetBindingByPUUID(account.PUUID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to check existing binding: %w", err)
	}

	if existing != nil {
		if existing.IsVerified {
			if existing.DiscordID == discordID {
				return nil, 0, ErrAlreadyBoundToYou
			}
			return nil, 0, ErrAlreadyBound
		}
		// Existing unverified binding - check if it belongs to this user
		if existing.DiscordID != discordID {
			// Someone else started binding this account, check if expired
			if existing.VerificationExpiresAt != nil && time.Now().After(*existing.VerificationExpiresAt) {
				// Expired, delete it and allow new binding
				s.repo.DeleteBinding(existing.ID)
			} else {
				return nil, 0, fmt.Errorf("this account is being verified by another user")
			}
		}
	}

	// Generate random verification icon
	iconID, err := s.generateVerificationIcon()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to generate verification icon: %w", err)
	}

	now := time.Now()
	expiresAt := now.Add(VerificationTimeout)

	// If existing unverified binding for this user, update it
	if existing != nil && existing.DiscordID == discordID {
		err = s.repo.UpdateVerificationCode(existing.ID, iconID, expiresAt)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to update verification: %w", err)
		}
		existing.VerificationIconID = &iconID
		existing.VerificationExpiresAt = &expiresAt
		existing.RiotID = account.GameName
		existing.RiotTag = account.TagLine
		return existing, iconID, nil
	}

	// Create new binding
	binding := &RiotBinding{
		ID:                    uuid.New(),
		DiscordID:             discordID,
		RiotPUUID:             account.PUUID,
		RiotID:                account.GameName, // Use API-returned name (correct casing)
		RiotTag:               account.TagLine,
		IsVerified:            false,
		VerificationIconID:    &iconID,
		VerificationExpiresAt: &expiresAt,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	if err := s.repo.CreateBinding(binding); err != nil {
		return nil, 0, fmt.Errorf("failed to create binding: %w", err)
	}

	logger.Infof("Started Riot binding verification: %s -> %s#%s (icon: %d)", discordID, account.GameName, account.TagLine, iconID)
	return binding, iconID, nil
}

// VerifyBinding completes the verification process
func (s *Service) VerifyBinding(ctx context.Context, discordID, channelID, riotID, riotTag string) (*RiotBinding, error) {
	// Check if in correct channel
	if channelID != BindingChannelID {
		return nil, ErrWrongChannel
	}

	// Get the Riot account
	account, err := s.apiClient.GetAccountByRiotID(ctx, riotID, riotTag)
	if err != nil {
		if IsNotFound(err) {
			return nil, fmt.Errorf("Riot account not found: %s#%s", riotID, riotTag)
		}
		return nil, fmt.Errorf("failed to get Riot account: %w", err)
	}

	// Get pending verification
	binding, err := s.repo.GetPendingVerification(discordID, account.PUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending verification: %w", err)
	}
	if binding == nil {
		return nil, ErrNoPendingVerify
	}

	// Check if expired
	if binding.VerificationExpiresAt != nil && time.Now().After(*binding.VerificationExpiresAt) {
		return nil, ErrVerificationExpired
	}

	// Get current profile icon via Summoner API
	summoner, err := s.apiClient.GetSummonerByPUUID(ctx, account.PUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get summoner data: %w", err)
	}

	// Verify icon matches
	if binding.VerificationIconID == nil {
		return nil, fmt.Errorf("%w (no verification icon recorded)", ErrVerificationFailed)
	}
	if summoner.ProfileIconID != *binding.VerificationIconID {
		return nil, fmt.Errorf("%w (expected: %d, got: %d)", ErrVerificationFailed, *binding.VerificationIconID, summoner.ProfileIconID)
	}

	// Mark as verified
	if err := s.repo.VerifyBinding(binding.ID, summoner.ID); err != nil {
		return nil, fmt.Errorf("failed to verify binding: %w", err)
	}

	// Update binding struct
	binding.IsVerified = true
	binding.SummonerID = &summoner.ID
	binding.VerificationIconID = nil
	binding.VerificationExpiresAt = nil

	// Fetch and cache initial rank data asynchronously using PUUID
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.UpdateRankForBinding(ctx, binding.ID, binding.RiotPUUID); err != nil {
			logger.Errorf("Failed to fetch initial rank for %s#%s: %v", binding.RiotID, binding.RiotTag, err)
		}
	}()

	logger.Infof("Riot binding verified: %s -> %s#%s", discordID, binding.RiotID, binding.RiotTag)
	return binding, nil
}

// UnbindAccount removes a Riot account binding
func (s *Service) UnbindAccount(ctx context.Context, discordID, channelID, riotID, riotTag string) error {
	// Check if in correct channel
	if channelID != BindingChannelID {
		return ErrWrongChannel
	}

	// Get the Riot account to find PUUID
	account, err := s.apiClient.GetAccountByRiotID(ctx, riotID, riotTag)
	if err != nil {
		if IsNotFound(err) {
			return fmt.Errorf("Riot account not found: %s#%s", riotID, riotTag)
		}
		return fmt.Errorf("failed to get Riot account: %w", err)
	}

	// Get the binding
	binding, err := s.repo.GetBindingByPUUID(account.PUUID)
	if err != nil {
		return fmt.Errorf("failed to get binding: %w", err)
	}
	if binding == nil {
		return ErrBindingNotFound
	}

	// Verify ownership
	if binding.DiscordID != discordID {
		return errors.New("this account is not bound to you")
	}

	// Delete binding (cascades to ranks)
	if err := s.repo.DeleteBinding(binding.ID); err != nil {
		return fmt.Errorf("failed to delete binding: %w", err)
	}

	logger.Infof("Riot binding removed: %s -> %s#%s", discordID, riotID, riotTag)
	return nil
}

// GetUserBindings returns all verified bindings for a user
func (s *Service) GetUserBindings(discordID string) ([]RiotBinding, error) {
	return s.repo.GetVerifiedBindingsByDiscordID(discordID)
}

// GetUserBindingsWithRanks returns all verified bindings with their rank data
func (s *Service) GetUserBindingsWithRanks(discordID string) ([]UserRankInfo, error) {
	bindings, err := s.repo.GetVerifiedBindingsByDiscordID(discordID)
	if err != nil {
		return nil, err
	}

	result := make([]UserRankInfo, len(bindings))
	for i, binding := range bindings {
		rank, _ := s.repo.GetRankByBindingID(binding.ID)
		result[i] = UserRankInfo{
			Binding: binding,
			Rank:    rank,
		}
	}

	return result, nil
}

// GetLeaderboard returns the rank leaderboard
func (s *Service) GetLeaderboard(limit int) ([]LeaderboardEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.GetLeaderboard(limit)
}

// UpdateRankForBinding updates rank data for a single binding using PUUID
// Also updates the Riot ID if the player has renamed
func (s *Service) UpdateRankForBinding(ctx context.Context, bindingID uuid.UUID, puuid string) error {
	// First, refresh the Riot ID in case the player renamed
	account, err := s.apiClient.GetAccountByPUUID(ctx, puuid)
	if err != nil {
		logger.Warnf("Failed to refresh Riot ID for binding %s: %v", bindingID, err)
		// Continue with rank update even if we can't refresh the name
	} else {
		// Update the Riot ID in database
		if err := s.repo.UpdateBindingRiotID(bindingID, account.GameName, account.TagLine); err != nil {
			logger.Warnf("Failed to update Riot ID for binding %s: %v", bindingID, err)
		}
	}

	entries, err := s.apiClient.GetLeagueEntriesByPUUID(ctx, puuid)
	if err != nil {
		return fmt.Errorf("failed to get league entries: %w", err)
	}

	now := time.Now()

	for _, entry := range entries {
		if entry.QueueType != QueueTypeSoloDuo {
			continue
		}

		rank := &RiotRank{
			ID:            uuid.New(),
			BindingID:     bindingID,
			QueueType:     entry.QueueType,
			Tier:          &entry.Tier,
			Division:      &entry.Rank, // API uses "Rank" field for division (I, II, III, IV)
			LeaguePoints:  entry.LeaguePoints,
			Wins:          entry.Wins,
			Losses:        entry.Losses,
			LastUpdatedAt: now,
			CreatedAt:     now,
		}

		if err := s.repo.UpsertRank(rank); err != nil {
			return fmt.Errorf("failed to upsert rank: %w", err)
		}

		logger.Debugf("Updated rank for binding %s: %s %s %d LP", bindingID, entry.Tier, entry.Rank, entry.LeaguePoints)
		return nil // Only one Solo/Duo entry per player
	}

	// No Solo/Duo rank found (player is unranked in Solo/Duo)
	// We can either leave the existing rank or clear it
	// For now, we leave any existing rank data
	logger.Debugf("No Solo/Duo rank found for binding %s", bindingID)
	return nil
}

// UpdateAllRanks updates rank data for all verified bindings
func (s *Service) UpdateAllRanks(ctx context.Context) error {
	bindings, err := s.repo.GetAllVerifiedBindings()
	if err != nil {
		return fmt.Errorf("failed to get bindings: %w", err)
	}

	logger.Infof("Updating ranks for %d verified bindings", len(bindings))
	updated := 0
	failed := 0

	for _, binding := range bindings {
		// Use PUUID instead of SummonerID for the new League API endpoint
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := s.UpdateRankForBinding(ctx, binding.ID, binding.RiotPUUID); err != nil {
			logger.Errorf("Failed to update rank for %s#%s: %v", binding.RiotID, binding.RiotTag, err)
			failed++
		} else {
			updated++
		}

		// Rate limit protection - 200ms delay between API calls
		time.Sleep(200 * time.Millisecond)
	}

	logger.Infof("Rank update complete: %d updated, %d failed", updated, failed)
	return nil
}

// CleanupExpiredVerifications removes expired unverified bindings
func (s *Service) CleanupExpiredVerifications() error {
	deleted, err := s.repo.CleanupExpiredVerifications()
	if err != nil {
		return err
	}
	if deleted > 0 {
		logger.Infof("Cleaned up %d expired verification(s)", deleted)
	}
	return nil
}

// generateVerificationIcon picks a random icon from the safe range
func (s *Service) generateVerificationIcon() (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(VerificationIcons))))
	if err != nil {
		return 0, err
	}
	return VerificationIcons[n.Int64()], nil
}

// GetRepository returns the repository for scheduler access
func (s *Service) GetRepository() *Repository {
	return s.repo
}

// GetLatestDataDragonVersion returns the latest Data Dragon version for icon URLs
func (s *Service) GetLatestDataDragonVersion() string {
	return s.apiClient.GetLatestDataDragonVersion()
}
