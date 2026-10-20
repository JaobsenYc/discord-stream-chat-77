package rank

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/database"
)

// Repository handles database operations for rank bindings
type Repository struct {
	db *database.DB
}

// NewRepository creates a new Repository
func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

// ============================================================================
// Binding Operations
// ============================================================================

// CreateBinding creates a new binding record
func (r *Repository) CreateBinding(binding *RiotBinding) error {
	query := `
		INSERT INTO user_riot_bindings (
			id, discord_id, riot_puuid, riot_id, riot_tag, summoner_id,
			is_verified, verification_icon_id, verification_expires_at,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)`

	_, err := r.db.Exec(query,
		binding.ID, binding.DiscordID, binding.RiotPUUID,
		binding.RiotID, binding.RiotTag, binding.SummonerID,
		binding.IsVerified, binding.VerificationIconID, binding.VerificationExpiresAt,
		binding.CreatedAt, binding.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create binding: %w", err)
	}
	return nil
}

// GetBindingByID retrieves a binding by its UUID
func (r *Repository) GetBindingByID(id uuid.UUID) (*RiotBinding, error) {
	var binding RiotBinding
	query := `SELECT * FROM user_riot_bindings WHERE id = $1`
	err := r.db.Get(&binding, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get binding by id: %w", err)
	}
	return &binding, nil
}

// GetBindingByPUUID retrieves a binding by PUUID
func (r *Repository) GetBindingByPUUID(puuid string) (*RiotBinding, error) {
	var binding RiotBinding
	query := `SELECT * FROM user_riot_bindings WHERE riot_puuid = $1`
	err := r.db.Get(&binding, query, puuid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get binding by puuid: %w", err)
	}
	return &binding, nil
}

// GetBindingsByDiscordID retrieves all bindings for a Discord user
func (r *Repository) GetBindingsByDiscordID(discordID string) ([]RiotBinding, error) {
	var bindings []RiotBinding
	query := `SELECT * FROM user_riot_bindings WHERE discord_id = $1 ORDER BY created_at`
	err := r.db.Select(&bindings, query, discordID)
	if err != nil {
		return nil, fmt.Errorf("failed to get bindings by discord id: %w", err)
	}
	return bindings, nil
}

// GetVerifiedBindingsByDiscordID retrieves only verified bindings for a Discord user
func (r *Repository) GetVerifiedBindingsByDiscordID(discordID string) ([]RiotBinding, error) {
	var bindings []RiotBinding
	query := `SELECT * FROM user_riot_bindings WHERE discord_id = $1 AND is_verified = TRUE ORDER BY created_at`
	err := r.db.Select(&bindings, query, discordID)
	if err != nil {
		return nil, fmt.Errorf("failed to get verified bindings: %w", err)
	}
	return bindings, nil
}

// CountBindingsByDiscordID counts the number of bindings for a Discord user
func (r *Repository) CountBindingsByDiscordID(discordID string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM user_riot_bindings WHERE discord_id = $1`
	err := r.db.Get(&count, query, discordID)
	if err != nil {
		return 0, fmt.Errorf("failed to count bindings: %w", err)
	}
	return count, nil
}

// GetPendingVerification retrieves a pending verification for a user and PUUID
func (r *Repository) GetPendingVerification(discordID, puuid string) (*RiotBinding, error) {
	var binding RiotBinding
	query := `
		SELECT * FROM user_riot_bindings
		WHERE discord_id = $1 AND riot_puuid = $2
		AND is_verified = FALSE
		AND verification_expires_at > NOW()`
	err := r.db.Get(&binding, query, discordID, puuid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get pending verification: %w", err)
	}
	return &binding, nil
}

// UpdateVerificationCode updates the verification icon and expiry for a binding
func (r *Repository) UpdateVerificationCode(id uuid.UUID, iconID int, expiresAt time.Time) error {
	query := `
		UPDATE user_riot_bindings
		SET verification_icon_id = $2,
		    verification_expires_at = $3,
		    updated_at = NOW()
		WHERE id = $1`
	_, err := r.db.Exec(query, id, iconID, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to update verification code: %w", err)
	}
	return nil
}

// VerifyBinding marks a binding as verified and stores the summoner ID
func (r *Repository) VerifyBinding(id uuid.UUID, summonerID string) error {
	query := `
		UPDATE user_riot_bindings
		SET is_verified = TRUE,
		    summoner_id = $2,
		    verification_icon_id = NULL,
		    verification_expires_at = NULL,
		    updated_at = NOW()
		WHERE id = $1`
	_, err := r.db.Exec(query, id, summonerID)
	if err != nil {
		return fmt.Errorf("failed to verify binding: %w", err)
	}
	return nil
}

// DeleteBinding removes a binding by ID
func (r *Repository) DeleteBinding(id uuid.UUID) error {
	query := `DELETE FROM user_riot_bindings WHERE id = $1`
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete binding: %w", err)
	}
	return nil
}

// DeleteBindingByDiscordAndPUUID removes a specific binding for a user
func (r *Repository) DeleteBindingByDiscordAndPUUID(discordID, puuid string) error {
	query := `DELETE FROM user_riot_bindings WHERE discord_id = $1 AND riot_puuid = $2`
	result, err := r.db.Exec(query, discordID, puuid)
	if err != nil {
		return fmt.Errorf("failed to delete binding: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("binding not found")
	}
	return nil
}

// CleanupExpiredVerifications removes bindings with expired verification
func (r *Repository) CleanupExpiredVerifications() (int64, error) {
	query := `
		DELETE FROM user_riot_bindings
		WHERE is_verified = FALSE AND verification_expires_at < NOW()`
	result, err := r.db.Exec(query)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired verifications: %w", err)
	}
	return result.RowsAffected()
}

// ============================================================================
// Rank Operations
// ============================================================================

// UpsertRank inserts or updates rank data for a binding
func (r *Repository) UpsertRank(rank *RiotRank) error {
	query := `
		INSERT INTO user_riot_ranks (
			id, binding_id, queue_type, tier, division,
			league_points, wins, losses, last_updated_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (binding_id, queue_type) DO UPDATE SET
			tier = EXCLUDED.tier,
			division = EXCLUDED.division,
			league_points = EXCLUDED.league_points,
			wins = EXCLUDED.wins,
			losses = EXCLUDED.losses,
			last_updated_at = EXCLUDED.last_updated_at`

	_, err := r.db.Exec(query,
		rank.ID, rank.BindingID, rank.QueueType, rank.Tier, rank.Division,
		rank.LeaguePoints, rank.Wins, rank.Losses, rank.LastUpdatedAt, rank.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert rank: %w", err)
	}
	return nil
}

// GetRankByBindingID retrieves the Solo/Duo rank for a binding
func (r *Repository) GetRankByBindingID(bindingID uuid.UUID) (*RiotRank, error) {
	var rank RiotRank
	query := `SELECT * FROM user_riot_ranks WHERE binding_id = $1 AND queue_type = $2`
	err := r.db.Get(&rank, query, bindingID, QueueTypeSoloDuo)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get rank: %w", err)
	}
	return &rank, nil
}

// GetRanksByDiscordID retrieves all ranks for a user's verified bindings
func (r *Repository) GetRanksByDiscordID(discordID string) ([]RiotRank, error) {
	var ranks []RiotRank
	query := `
		SELECT r.* FROM user_riot_ranks r
		JOIN user_riot_bindings b ON r.binding_id = b.id
		WHERE b.discord_id = $1 AND b.is_verified = TRUE AND r.queue_type = $2`
	err := r.db.Select(&ranks, query, discordID, QueueTypeSoloDuo)
	if err != nil {
		return nil, fmt.Errorf("failed to get ranks by discord id: %w", err)
	}
	return ranks, nil
}

// GetLeaderboard retrieves the rank leaderboard sorted by tier + LP
func (r *Repository) GetLeaderboard(limit int) ([]LeaderboardEntry, error) {
	query := `
		SELECT
			b.discord_id,
			b.riot_id,
			b.riot_tag,
			COALESCE(r.tier, '') as tier,
			COALESCE(r.division, '') as division,
			COALESCE(r.league_points, 0) as league_points,
			COALESCE(r.wins, 0) as wins,
			COALESCE(r.losses, 0) as losses
		FROM user_riot_bindings b
		LEFT JOIN user_riot_ranks r ON b.id = r.binding_id AND r.queue_type = $1
		WHERE b.is_verified = TRUE
		ORDER BY
			CASE r.tier
				WHEN 'CHALLENGER' THEN 10
				WHEN 'GRANDMASTER' THEN 9
				WHEN 'MASTER' THEN 8
				WHEN 'DIAMOND' THEN 7
				WHEN 'EMERALD' THEN 6
				WHEN 'PLATINUM' THEN 5
				WHEN 'GOLD' THEN 4
				WHEN 'SILVER' THEN 3
				WHEN 'BRONZE' THEN 2
				WHEN 'IRON' THEN 1
				ELSE 0
			END DESC,
			CASE r.division
				WHEN 'I' THEN 4
				WHEN 'II' THEN 3
				WHEN 'III' THEN 2
				WHEN 'IV' THEN 1
				ELSE 0
			END DESC,
			r.league_points DESC NULLS LAST
		LIMIT $2`

	var entries []LeaderboardEntry
	err := r.db.Select(&entries, query, QueueTypeSoloDuo, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get leaderboard: %w", err)
	}

	// Add rank numbers and calculate win rates
	for i := range entries {
		entries[i].Rank = i + 1
		entries[i].WinRate = CalculateWinRate(entries[i].Wins, entries[i].Losses)
	}

	return entries, nil
}

// GetAllVerifiedBindings retrieves all verified bindings for rank updates
func (r *Repository) GetAllVerifiedBindings() ([]RiotBinding, error) {
	var bindings []RiotBinding
	query := `SELECT * FROM user_riot_bindings WHERE is_verified = TRUE`
	err := r.db.Select(&bindings, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get verified bindings: %w", err)
	}
	return bindings, nil
}

// UpdateBindingRiotID updates the Riot ID and tag for a binding (when user renames)
func (r *Repository) UpdateBindingRiotID(id uuid.UUID, riotID, riotTag string) error {
	query := `
		UPDATE user_riot_bindings
		SET riot_id = $2, riot_tag = $3, updated_at = NOW()
		WHERE id = $1`
	_, err := r.db.Exec(query, id, riotID, riotTag)
	if err != nil {
		return fmt.Errorf("failed to update binding riot id: %w", err)
	}
	return nil
}

// GetBindingsNeedingRankUpdate retrieves verified bindings that haven't been updated recently
func (r *Repository) GetBindingsNeedingRankUpdate(olderThan time.Duration) ([]RiotBinding, error) {
	var bindings []RiotBinding
	cutoff := time.Now().Add(-olderThan)
	query := `
		SELECT b.* FROM user_riot_bindings b
		LEFT JOIN user_riot_ranks r ON b.id = r.binding_id AND r.queue_type = $1
		WHERE b.is_verified = TRUE
		AND (r.last_updated_at IS NULL OR r.last_updated_at < $2)`
	err := r.db.Select(&bindings, query, QueueTypeSoloDuo, cutoff)
	if err != nil {
		return nil, fmt.Errorf("failed to get bindings needing update: %w", err)
	}
	return bindings, nil
}
