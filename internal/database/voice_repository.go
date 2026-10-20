package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// VoiceRepository 语音会话数据访问层
type VoiceRepository struct {
	db *DB
}

// NewVoiceRepository 创建语音仓库
func NewVoiceRepository(db *DB) *VoiceRepository {
	return &VoiceRepository{db: db}
}

// JoinVoiceChannel 用户加入语音频道
func (r *VoiceRepository) JoinVoiceChannel(userID uuid.UUID, discordID, guildID, channelID, channelName string) (*VoiceSession, error) {
	// 检查是否已经在语音频道中
	activeSession, err := r.GetActiveSession(userID, guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to check active session: %w", err)
	}

	if activeSession != nil {
		// 如果在同一个频道，不需要重复记录
		if activeSession.ChannelID != nil && *activeSession.ChannelID == channelID {
			return activeSession, nil
		}

		// 如果在不同频道，先结束当前会话
		if err := r.LeaveVoiceChannel(userID, guildID); err != nil {
			return nil, fmt.Errorf("failed to end previous session: %w", err)
		}
	}

	// 创建新的语音会话
	session := &VoiceSession{
		ID:          uuid.New(),
		UserID:      userID,
		DiscordID:   discordID,
		GuildID:     guildID,
		ChannelID:   &channelID,
		ChannelName: &channelName,
		JoinedAt:    time.Now().UTC(),
		IsActive:    true,
	}

	insertQuery := `
		INSERT INTO voice_sessions (id, user_id, discord_id, guild_id, channel_id, channel_name, joined_at, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at, updated_at`

	err = r.db.QueryRow(
		insertQuery,
		session.ID,
		session.UserID,
		session.DiscordID,
		session.GuildID,
		session.ChannelID,
		session.ChannelName,
		session.JoinedAt,
		session.IsActive,
	).Scan(&session.CreatedAt, &session.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create voice session: %w", err)
	}

	// 更新缓存
	r.updateVoiceActivityCache(userID, guildID, &VoiceActivityCache{
		IsInVoice:      true,
		CurrentChannel: &channelID,
		JoinedAt:       session.JoinedAt,
		LastUpdated:    time.Now().UTC(),
	})

	return session, nil
}

// LeaveVoiceChannel 用户离开语音频道
func (r *VoiceRepository) LeaveVoiceChannel(userID uuid.UUID, guildID string) error {
	// 获取活跃会话
	activeSession, err := r.GetActiveSession(userID, guildID)
	if err != nil {
		return fmt.Errorf("failed to get active session: %w", err)
	}

	if activeSession == nil {
		return nil // 没有活跃会话
	}

	// 结束会话
	leftAt := time.Now().UTC()
	updateQuery := `
		UPDATE voice_sessions 
		SET left_at = $1, is_active = false, updated_at = $2
		WHERE id = $3`

	_, err = r.db.Exec(updateQuery, leftAt, leftAt, activeSession.ID)
	if err != nil {
		return fmt.Errorf("failed to end voice session: %w", err)
	}

	// 计算会话时长并更新缓存统计
	duration := leftAt.Sub(activeSession.JoinedAt)
	r.updateVoiceStatsCache(userID, guildID, duration.Seconds())

	// 更新活动缓存
	r.updateVoiceActivityCache(userID, guildID, &VoiceActivityCache{
		IsInVoice:   false,
		LastUpdated: time.Now().UTC(),
	})

	// 清除相关排行榜缓存
	r.clearVoiceLeaderboardCache(guildID)

	return nil
}

// GetActiveSession 获取用户的活跃语音会话
func (r *VoiceRepository) GetActiveSession(userID uuid.UUID, guildID string) (*VoiceSession, error) {
	// 先检查缓存
	cacheKey := fmt.Sprintf("voice:activity:%s:%s", userID, guildID)
	cached, err := r.db.GetCache(cacheKey)
	if err == nil && cached != "" {
		var activityCache VoiceActivityCache
		if err := json.Unmarshal([]byte(cached), &activityCache); err == nil {
			if !activityCache.IsInVoice {
				return nil, nil
			}
		}
	}

	// 从数据库获取
	var session VoiceSession
	query := `SELECT * FROM voice_sessions WHERE user_id = $1 AND guild_id = $2 AND is_active = true`
	err = r.db.Get(&session, query, userID, guildID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get active session: %w", err)
	}

	return &session, nil
}

// GetUserVoiceStats 获取用户语音统计
func (r *VoiceRepository) GetUserVoiceStats(userID uuid.UUID, guildID string) (*UserVoiceStats, error) {
	// 先尝试从缓存获取
	cacheKey := fmt.Sprintf("voice:stats:%s:%s", userID, guildID)
	cached, err := r.db.GetCache(cacheKey)
	if err == nil && cached != "" {
		var stats UserVoiceStats
		if err := json.Unmarshal([]byte(cached), &stats); err == nil {
			return &stats, nil
		}
	}

	// 从数据库视图获取
	var stats UserVoiceStats
	query := `SELECT * FROM user_voice_stats WHERE user_id = $1 AND guild_id = $2`
	err = r.db.Get(&stats, query, userID, guildID)
	if err != nil {
		if err == sql.ErrNoRows {
			return &UserVoiceStats{
				UserID:  userID,
				GuildID: &guildID,
			}, nil
		}
		return nil, fmt.Errorf("failed to get user voice stats: %w", err)
	}

	// 缓存结果（10分钟）
	if data, err := json.Marshal(&stats); err == nil {
		r.db.SetCacheWithTTL(cacheKey, data, 10*time.Minute)
	}

	return &stats, nil
}

// GetVoiceLeaderboard 获取语音活跃度排行榜
func (r *VoiceRepository) GetVoiceLeaderboard(guildID string, limit int, period string) ([]UserVoiceStats, error) {
	// 先尝试从缓存获取
	cacheKey := fmt.Sprintf("voice:leaderboard:%s:%s:%d", guildID, period, limit)
	cached, err := r.db.GetCache(cacheKey)
	if err == nil && cached != "" {
		var leaderboard []UserVoiceStats
		if err := json.Unmarshal([]byte(cached), &leaderboard); err == nil {
			return leaderboard, nil
		}
	}

	// 构建查询条件
	var timeCondition string
	switch period {
	case "today":
		timeCondition = "AND vs.joined_at >= CURRENT_DATE"
	case "week":
		timeCondition = "AND vs.joined_at >= CURRENT_DATE - INTERVAL '7 days'"
	case "month":
		timeCondition = "AND vs.joined_at >= CURRENT_DATE - INTERVAL '30 days'"
	default:
		timeCondition = "" // 全部时间
	}

	query := fmt.Sprintf(`
		SELECT
			u.id as user_id,
			u.discord_id,
			u.username,
			$1::varchar as guild_id,
			COUNT(vs.id) as total_sessions,
			COALESCE(SUM(EXTRACT(EPOCH FROM vs.duration)), 0) as total_seconds,
			COALESCE(AVG(EXTRACT(EPOCH FROM vs.duration)), 0) as avg_session_seconds,
			MAX(vs.joined_at) as last_voice_activity,
			0 as today_sessions,
			0 as today_seconds
		FROM users u
		LEFT JOIN voice_sessions vs ON u.id = vs.user_id AND vs.guild_id = $1 AND vs.left_at IS NOT NULL %s
		WHERE EXISTS (SELECT 1 FROM voice_sessions WHERE user_id = u.id AND guild_id = $1 %s)
		GROUP BY u.id, u.discord_id, u.username
		ORDER BY total_seconds DESC
		LIMIT $2`, timeCondition, timeCondition)

	var leaderboard []UserVoiceStats
	err = r.db.Select(&leaderboard, query, guildID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get voice leaderboard: %w", err)
	}

	// 缓存结果（30分钟）
	if data, err := json.Marshal(leaderboard); err == nil {
		r.db.SetCacheWithTTL(cacheKey, data, 30*time.Minute)
	}

	return leaderboard, nil
}

// GetUserVoiceSessions 获取用户语音会话历史
func (r *VoiceRepository) GetUserVoiceSessions(userID uuid.UUID, guildID string, limit int) ([]VoiceSession, error) {
	query := `
		SELECT * FROM voice_sessions 
		WHERE user_id = $1 AND guild_id = $2 
		ORDER BY joined_at DESC 
		LIMIT $3`

	var sessions []VoiceSession
	err := r.db.Select(&sessions, query, userID, guildID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get user voice sessions: %w", err)
	}

	return sessions, nil
}

// updateVoiceActivityCache 更新语音活动缓存
func (r *VoiceRepository) updateVoiceActivityCache(userID uuid.UUID, guildID string, cache *VoiceActivityCache) {
	cacheKey := fmt.Sprintf("voice:activity:%s:%s", userID, guildID)

	// 如果有现有缓存，合并数据
	if cached, err := r.db.GetCache(cacheKey); err == nil && cached != "" {
		var existingCache VoiceActivityCache
		if err := json.Unmarshal([]byte(cached), &existingCache); err == nil {
			if cache.TodaySeconds == 0 {
				cache.TodaySeconds = existingCache.TodaySeconds
			}
			if cache.WeeklySeconds == 0 {
				cache.WeeklySeconds = existingCache.WeeklySeconds
			}
			if cache.MonthlySeconds == 0 {
				cache.MonthlySeconds = existingCache.MonthlySeconds
			}
		}
	}

	data, _ := json.Marshal(cache)

	// 缓存到明天
	now := time.Now().UTC()
	tomorrow := now.Truncate(24 * time.Hour).Add(24 * time.Hour)
	ttl := tomorrow.Sub(now)

	r.db.SetCacheWithTTL(cacheKey, data, ttl)
}

// updateVoiceStatsCache 更新语音统计缓存
func (r *VoiceRepository) updateVoiceStatsCache(userID uuid.UUID, guildID string, durationSeconds float64) {
	cacheKey := fmt.Sprintf("voice:activity:%s:%s", userID, guildID)

	cached, err := r.db.GetCache(cacheKey)
	if err != nil {
		return
	}

	var activityCache VoiceActivityCache
	if err := json.Unmarshal([]byte(cached), &activityCache); err != nil {
		return
	}

	// 更新统计
	activityCache.TodaySeconds += durationSeconds
	activityCache.WeeklySeconds += durationSeconds
	activityCache.MonthlySeconds += durationSeconds
	activityCache.LastUpdated = time.Now().UTC()

	// 更新缓存
	r.updateVoiceActivityCache(userID, guildID, &activityCache)

	// 清除用户统计缓存
	statsKey := fmt.Sprintf("voice:stats:%s:%s", userID, guildID)
	r.db.DeleteCache(statsKey)
}

// clearVoiceLeaderboardCache 清除语音排行榜缓存
func (r *VoiceRepository) clearVoiceLeaderboardCache(guildID string) {
	// 缓存键为 voice:leaderboard:<guild>:<period>:<limit>，limit 不定，需按模式删除
	pattern := fmt.Sprintf("voice:leaderboard:%s:*", guildID)
	if err := r.db.DeleteCachePattern(pattern); err != nil {
		logger.Warnf("清除语音排行榜缓存失败 (guild=%s): %v", guildID, err)
	}
}

// CleanupInactiveSessions 清理长时间未结束的会话（定时任务用）
func (r *VoiceRepository) CleanupInactiveSessions(maxDuration time.Duration) error {
	cutoffTime := time.Now().UTC().Add(-maxDuration)

	query := `
		UPDATE voice_sessions 
		SET left_at = joined_at + INTERVAL '%d seconds', is_active = false, updated_at = NOW()
		WHERE is_active = true AND joined_at < $1`

	_, err := r.db.Exec(fmt.Sprintf(query, int(maxDuration.Seconds())), cutoffTime)
	if err != nil {
		return fmt.Errorf("failed to cleanup inactive sessions: %w", err)
	}

	return nil
}

// GetVoiceActivitySummary 获取语音活动摘要
func (r *VoiceRepository) GetVoiceActivitySummary(guildID string) (map[string]interface{}, error) {
	query := `
		SELECT 
			COUNT(DISTINCT user_id) as active_users,
			COUNT(DISTINCT channel_id) as active_channels,
			COUNT(*) as total_sessions,
			COALESCE(SUM(EXTRACT(EPOCH FROM duration)), 0) as total_seconds,
			COUNT(CASE WHEN joined_at >= CURRENT_DATE THEN 1 END) as today_sessions,
			COALESCE(SUM(CASE WHEN joined_at >= CURRENT_DATE THEN EXTRACT(EPOCH FROM duration) ELSE 0 END), 0) as today_seconds
		FROM voice_sessions 
		WHERE guild_id = $1 AND left_at IS NOT NULL`

	var summary struct {
		ActiveUsers    int     `db:"active_users"`
		ActiveChannels int     `db:"active_channels"`
		TotalSessions  int     `db:"total_sessions"`
		TotalSeconds   float64 `db:"total_seconds"`
		TodaySessions  int     `db:"today_sessions"`
		TodaySeconds   float64 `db:"today_seconds"`
	}

	err := r.db.Get(&summary, query, guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get voice activity summary: %w", err)
	}

	result := map[string]interface{}{
		"active_users":     summary.ActiveUsers,
		"active_channels":  summary.ActiveChannels,
		"total_sessions":   summary.TotalSessions,
		"total_hours":      summary.TotalSeconds / 3600,
		"today_sessions":   summary.TodaySessions,
		"today_hours":      summary.TodaySeconds / 3600,
		"avg_session_time": 0.0,
	}

	if summary.TotalSessions > 0 {
		result["avg_session_time"] = summary.TotalSeconds / float64(summary.TotalSessions) / 60 // 分钟
	}

	return result, nil
}
