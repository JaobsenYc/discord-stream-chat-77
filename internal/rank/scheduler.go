package rank

import (
	"context"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// Scheduler handles periodic tasks for the rank system
type Scheduler struct {
	service       *Service
	cmdHandler    *CommandHandler
	session       *discordgo.Session
	leaderboardCh string // Channel ID for scheduled leaderboard posts

	// Intervals
	rankUpdateInterval      time.Duration
	leaderboardPostInterval time.Duration
	cleanupInterval         time.Duration

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewScheduler creates a new Scheduler
func NewScheduler(
	service *Service,
	cmdHandler *CommandHandler,
	session *discordgo.Session,
	leaderboardChannelID string,
) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())

	return &Scheduler{
		service:                 service,
		cmdHandler:              cmdHandler,
		session:                 session,
		leaderboardCh:           leaderboardChannelID,
		rankUpdateInterval:      30 * time.Minute, // Update ranks every 30 minutes
		leaderboardPostInterval: 12 * time.Hour,   // Post leaderboard every 12 hours
		cleanupInterval:         5 * time.Minute,  // Cleanup expired verifications every 5 minutes
		ctx:                     ctx,
		cancel:                  cancel,
	}
}

// Start starts all scheduled tasks
func (s *Scheduler) Start() {
	logger.Info("Rank scheduler starting")

	s.wg.Add(3)
	go s.runRankUpdates()
	go s.runLeaderboardPosts()
	go s.runCleanup()

	logger.Info("Rank scheduler started")
}

// Stop gracefully stops all scheduled tasks
func (s *Scheduler) Stop() {
	logger.Info("Rank scheduler stopping")
	s.cancel()
	s.wg.Wait()
	logger.Info("Rank scheduler stopped")
}

// runRankUpdates periodically updates rank data for all verified bindings
func (s *Scheduler) runRankUpdates() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.rankUpdateInterval)
	defer ticker.Stop()

	// Initial update on start (after a short delay to let bot fully initialize)
	time.Sleep(10 * time.Second)
	if err := s.service.UpdateAllRanks(s.ctx); err != nil {
		logger.Errorf("Initial rank update failed: %v", err)
	}

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.service.UpdateAllRanks(s.ctx); err != nil {
				logger.Errorf("Scheduled rank update failed: %v", err)
			}
		}
	}
}

// runLeaderboardPosts periodically posts the leaderboard to the designated channel
func (s *Scheduler) runLeaderboardPosts() {
	defer s.wg.Done()

	if s.leaderboardCh == "" {
		logger.Warn("Leaderboard channel not configured, scheduled posts disabled")
		return
	}

	ticker := time.NewTicker(s.leaderboardPostInterval)
	defer ticker.Stop()

	// Calculate time until next hour for better alignment
	now := time.Now()
	nextHour := now.Truncate(time.Hour).Add(time.Hour)
	initialDelay := nextHour.Sub(now)

	// Wait until the next hour boundary for the first post
	select {
	case <-s.ctx.Done():
		return
	case <-time.After(initialDelay):
		s.postLeaderboard()
	}

	// Reset ticker to be aligned with hour boundary
	ticker.Reset(s.leaderboardPostInterval)

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.postLeaderboard()
		}
	}
}

// postLeaderboard posts the current leaderboard to the designated channel
func (s *Scheduler) postLeaderboard() {
	entries, err := s.service.GetLeaderboard(20)
	if err != nil {
		logger.Errorf("Failed to get leaderboard for scheduled post: %v", err)
		return
	}

	// Don't post if there are no entries
	if len(entries) == 0 {
		logger.Debug("Skipping leaderboard post: no entries")
		return
	}

	embed := s.cmdHandler.BuildScheduledLeaderboardEmbed(entries)

	_, err = s.session.ChannelMessageSendEmbed(s.leaderboardCh, embed)
	if err != nil {
		logger.Errorf("Failed to post scheduled leaderboard: %v", err)
		return
	}

	logger.Infof("Posted scheduled leaderboard with %d entries", len(entries))
}

// runCleanup periodically cleans up expired verification records
func (s *Scheduler) runCleanup() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.service.CleanupExpiredVerifications(); err != nil {
				logger.Errorf("Failed to cleanup expired verifications: %v", err)
			}
		}
	}
}

// ForceRankUpdate triggers an immediate rank update (for admin use)
func (s *Scheduler) ForceRankUpdate() error {
	return s.service.UpdateAllRanks(s.ctx)
}

// ForceLeaderboardPost triggers an immediate leaderboard post (for admin use)
func (s *Scheduler) ForceLeaderboardPost() {
	s.postLeaderboard()
}
