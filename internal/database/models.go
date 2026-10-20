package database

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// User 用户表模型
type User struct {
	ID            uuid.UUID `db:"id" json:"id"`
	DiscordID     string    `db:"discord_id" json:"discord_id"`
	Username      string    `db:"username" json:"username"`
	Discriminator *string   `db:"discriminator" json:"discriminator,omitempty"`
	AvatarURL     *string   `db:"avatar_url" json:"avatar_url,omitempty"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at" json:"updated_at"`

	// 用户统计
	TotalTeamsCreated int `db:"total_teams_created" json:"total_teams_created"`
	TotalTeamsJoined  int `db:"total_teams_joined" json:"total_teams_joined"`
	TotalCheckIns     int `db:"total_check_ins" json:"total_check_ins"`

	// 用户设置
	Timezone    string          `db:"timezone" json:"timezone"`
	Language    string          `db:"language" json:"language"`
	Preferences UserPreferences `db:"preferences" json:"preferences"`
}

// UserPreferences 用户偏好设置
type UserPreferences map[string]interface{}

// Value 实现 driver.Valuer 接口
func (p UserPreferences) Value() (driver.Value, error) {
	return json.Marshal(p)
}

// Scan 实现 sql.Scanner 接口
func (p *UserPreferences) Scan(value interface{}) error {
	if value == nil {
		*p = make(UserPreferences)
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, p)
	case string:
		return json.Unmarshal([]byte(v), p)
	default:
		return json.Unmarshal([]byte("{}"), p)
	}
}

// CheckIn 签到记录
type CheckIn struct {
	ID              uuid.UUID `db:"id" json:"id"`
	UserID          uuid.UUID `db:"user_id" json:"user_id"`
	DiscordID       string    `db:"discord_id" json:"discord_id"`
	GuildID         string    `db:"guild_id" json:"guild_id"`
	ChannelID       string    `db:"channel_id" json:"channel_id"`
	CheckInDate     time.Time `db:"check_in_date" json:"check_in_date"`
	CheckInTime     time.Time `db:"check_in_time" json:"check_in_time"`
	PointsEarned    int       `db:"points_earned" json:"points_earned"`
	ConsecutiveDays int       `db:"consecutive_days" json:"consecutive_days"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
}

// Lottery 抽奖活动
type Lottery struct {
	ID               uuid.UUID      `db:"id" json:"id"`
	Name             string         `db:"name" json:"name"`
	Description      *string        `db:"description" json:"description,omitempty"`
	GuildID          string         `db:"guild_id" json:"guild_id"`
	ChannelID        string         `db:"channel_id" json:"channel_id"`
	CreatedBy        uuid.UUID      `db:"created_by" json:"created_by"`
	MaxParticipants  int            `db:"max_participants" json:"max_participants"`
	CostPoints       int            `db:"cost_points" json:"cost_points"`
	PrizeDescription string         `db:"prize_description" json:"prize_description"`
	StartTime        time.Time      `db:"start_time" json:"start_time"`
	EndTime          time.Time      `db:"end_time" json:"end_time"`
	Status           string         `db:"status" json:"status"`
	WinnerCount      int            `db:"winner_count" json:"winner_count"`
	Winners          LotteryWinners `db:"winners" json:"winners"`
	WinnerSelectedAt *time.Time     `db:"winner_selected_at" json:"winner_selected_at,omitempty"`
	CreatedAt        time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time      `db:"updated_at" json:"updated_at"`
}

// LotteryParticipant 抽奖参与者
type LotteryParticipant struct {
	ID             uuid.UUID `db:"id" json:"id"`
	LotteryID      uuid.UUID `db:"lottery_id" json:"lottery_id"`
	UserID         uuid.UUID `db:"user_id" json:"user_id"`
	DiscordID      string    `db:"discord_id" json:"discord_id"`
	PointsSpent    int       `db:"points_spent" json:"points_spent"`
	ParticipatedAt time.Time `db:"participated_at" json:"participated_at"`
}

// LotteryWinner 单个获奖者信息（用于JSON存储）
type LotteryWinner struct {
	UserID     string    `json:"user_id"`
	DiscordID  string    `json:"discord_id"`
	Username   string    `json:"username"`
	SelectedAt time.Time `json:"selected_at"`
}

// LotteryWinners 获奖者数组类型（自定义JSON处理）
type LotteryWinners []LotteryWinner

// Value 实现 driver.Valuer 接口，用于数据库写入
func (lw LotteryWinners) Value() (driver.Value, error) {
	if lw == nil {
		return "[]", nil
	}
	return json.Marshal(lw)
}

// Scan 实现 sql.Scanner 接口，用于数据库读取
func (lw *LotteryWinners) Scan(value interface{}) error {
	if value == nil {
		*lw = LotteryWinners{}
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("无法将 %T 转换为 LotteryWinners", value)
	}

	return json.Unmarshal(bytes, lw)
}

// AddWinner 添加获奖者
func (lw *LotteryWinners) AddWinner(userID, discordID, username string) {
	winner := LotteryWinner{
		UserID:     userID,
		DiscordID:  discordID,
		Username:   username,
		SelectedAt: time.Now(),
	}
	*lw = append(*lw, winner)
}

// ContainsUser 检查用户是否已中奖
func (lw LotteryWinners) ContainsUser(userID string) bool {
	for _, winner := range lw {
		if winner.UserID == userID {
			return true
		}
	}
	return false
}

// GetWinnerCount 获取中奖人数
func (lw LotteryWinners) GetWinnerCount() int {
	return len(lw)
}

// GetWinnerNames 获取获奖者用户名列表
func (lw LotteryWinners) GetWinnerNames() []string {
	names := make([]string, len(lw))
	for i, winner := range lw {
		names[i] = winner.Username
	}
	return names
}

// Team 团队记录
type Team struct {
	ID                uuid.UUID      `db:"id" json:"id"`
	TeamID            string         `db:"team_id" json:"team_id"`
	GuildID           string         `db:"guild_id" json:"guild_id"`
	TextChannelID     string         `db:"text_channel_id" json:"text_channel_id"`
	VoiceChannelID    *string        `db:"voice_channel_id" json:"voice_channel_id,omitempty"`
	GameName          string         `db:"game_name" json:"game_name"`
	MaxPlayers        int            `db:"max_players" json:"max_players"`
	CurrentPlayers    int            `db:"current_players" json:"current_players"`
	CreatorID         uuid.UUID      `db:"creator_id" json:"creator_id"`
	CreatorDiscordID  string         `db:"creator_discord_id" json:"creator_discord_id"`
	Status            string         `db:"status" json:"status"`
	CreatedAt         time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time      `db:"updated_at" json:"updated_at"`
	DisbandedAt       *time.Time     `db:"disbanded_at" json:"disbanded_at,omitempty"`
	TotalJoinRequests int            `db:"total_join_requests" json:"total_join_requests"`
	AverageFillTime   *time.Duration `db:"average_fill_time" json:"average_fill_time,omitempty"`
}

// TeamMember 团队成员
type TeamMember struct {
	ID        uuid.UUID      `db:"id" json:"id"`
	TeamID    uuid.UUID      `db:"team_id" json:"team_id"`
	UserID    uuid.UUID      `db:"user_id" json:"user_id"`
	DiscordID string         `db:"discord_id" json:"discord_id"`
	JoinedAt  time.Time      `db:"joined_at" json:"joined_at"`
	LeftAt    *time.Time     `db:"left_at" json:"left_at,omitempty"`
	Duration  *time.Duration `db:"duration" json:"duration,omitempty"`
	IsActive  bool           `db:"is_active" json:"is_active"`
}

// UserPoints 用户积分
type UserPoints struct {
	ID              uuid.UUID `db:"id" json:"id"`
	UserID          uuid.UUID `db:"user_id" json:"user_id"`
	GuildID         string    `db:"guild_id" json:"guild_id"`
	TotalPoints     int       `db:"total_points" json:"total_points"`
	AvailablePoints int       `db:"available_points" json:"available_points"`
	LifetimeEarned  int       `db:"lifetime_earned" json:"lifetime_earned"`
	LifetimeSpent   int       `db:"lifetime_spent" json:"lifetime_spent"`
	Level           int       `db:"level" json:"level"`
	Experience      int       `db:"experience" json:"experience"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time `db:"updated_at" json:"updated_at"`
}

// PointTransaction 积分交易记录
type PointTransaction struct {
	ID              uuid.UUID  `db:"id" json:"id"`
	UserID          uuid.UUID  `db:"user_id" json:"user_id"`
	GuildID         string     `db:"guild_id" json:"guild_id"`
	Amount          int        `db:"amount" json:"amount"`
	TransactionType string     `db:"transaction_type" json:"transaction_type"`
	Description     *string    `db:"description" json:"description,omitempty"`
	RelatedID       *uuid.UUID `db:"related_id" json:"related_id,omitempty"`
	RelatedType     *string    `db:"related_type" json:"related_type,omitempty"`
	CreatedAt       time.Time  `db:"created_at" json:"created_at"`
}

// GuildConfig 服务器配置
type GuildConfig struct {
	ID                      uuid.UUID     `db:"id" json:"id"`
	GuildID                 string        `db:"guild_id" json:"guild_id"`
	CheckInEnabled          bool          `db:"check_in_enabled" json:"check_in_enabled"`
	CheckInPoints           int           `db:"check_in_points" json:"check_in_points"`
	CheckInChannelID        *string       `db:"check_in_channel_id" json:"check_in_channel_id,omitempty"`
	ConsecutiveBonusEnabled bool          `db:"consecutive_bonus_enabled" json:"consecutive_bonus_enabled"`
	MaxConsecutiveBonus     int           `db:"max_consecutive_bonus" json:"max_consecutive_bonus"`
	LotteryEnabled          bool          `db:"lottery_enabled" json:"lottery_enabled"`
	DefaultLotteryDuration  time.Duration `db:"default_lottery_duration" json:"default_lottery_duration"`
	MinLotteryDuration      time.Duration `db:"min_lottery_duration" json:"min_lottery_duration"`
	MaxLotteryDuration      time.Duration `db:"max_lottery_duration" json:"max_lottery_duration"`
	PointsEnabled           bool          `db:"points_enabled" json:"points_enabled"`
	TeamCreationPoints      int           `db:"team_creation_points" json:"team_creation_points"`
	TeamCompletionPoints    int           `db:"team_completion_points" json:"team_completion_points"`
	CreatedAt               time.Time     `db:"created_at" json:"created_at"`
	UpdatedAt               time.Time     `db:"updated_at" json:"updated_at"`
}

// 视图模型

// UserCheckInStats 用户签到统计视图
type UserCheckInStats struct {
	DiscordID               string     `db:"discord_id" json:"discord_id"`
	Username                string     `db:"username" json:"username"`
	TotalCheckIns           int        `db:"total_check_ins" json:"total_check_ins"`
	MaxConsecutiveDays      int        `db:"max_consecutive_days" json:"max_consecutive_days"`
	LastCheckInDate         *time.Time `db:"last_check_in_date" json:"last_check_in_date,omitempty"`
	TotalPointsFromCheckIns int        `db:"total_points_from_check_ins" json:"total_points_from_check_ins"`
}

// ActiveTeamStats 活跃团队统计视图
type ActiveTeamStats struct {
	GuildID           string  `db:"guild_id" json:"guild_id"`
	GameName          string  `db:"game_name" json:"game_name"`
	ActiveTeams       int     `db:"active_teams" json:"active_teams"`
	AvgCurrentPlayers float64 `db:"avg_current_players" json:"avg_current_players"`
	AvgMaxPlayers     float64 `db:"avg_max_players" json:"avg_max_players"`
}

// UserLeaderboard 用户排行榜视图
type UserLeaderboard struct {
	DiscordID       string `db:"discord_id" json:"discord_id"`
	Username        string `db:"username" json:"username"`
	TotalPoints     int    `db:"total_points" json:"total_points"`
	AvailablePoints int    `db:"available_points" json:"available_points"`
	Level           int    `db:"level" json:"level"`
	Experience      int    `db:"experience" json:"experience"`
	Rank            int    `db:"rank" json:"rank"`
}

// VoiceSession 语音会话
type VoiceSession struct {
	ID          uuid.UUID  `db:"id" json:"id"`
	UserID      uuid.UUID  `db:"user_id" json:"user_id"`
	DiscordID   string     `db:"discord_id" json:"discord_id"`
	GuildID     string     `db:"guild_id" json:"guild_id"`
	ChannelID   *string    `db:"channel_id" json:"channel_id"`
	ChannelName *string    `db:"channel_name" json:"channel_name"`
	JoinedAt    time.Time  `db:"joined_at" json:"joined_at"`
	LeftAt      *time.Time `db:"left_at" json:"left_at"`
	Duration    *string    `db:"duration" json:"duration"` // PostgreSQL INTERVAL type
	IsActive    bool       `db:"is_active" json:"is_active"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at" json:"updated_at"`
}

// UserVoiceStats 用户语音统计
type UserVoiceStats struct {
	UserID            uuid.UUID  `db:"user_id" json:"user_id"`
	DiscordID         string     `db:"discord_id" json:"discord_id"`
	Username          string     `db:"username" json:"username"`
	GuildID           *string    `db:"guild_id" json:"guild_id"`
	TotalSessions     int        `db:"total_sessions" json:"total_sessions"`
	TotalSeconds      float64    `db:"total_seconds" json:"total_seconds"`
	AvgSessionSeconds float64    `db:"avg_session_seconds" json:"avg_session_seconds"`
	LastVoiceActivity *time.Time `db:"last_voice_activity" json:"last_voice_activity"`
	TodaySessions     int        `db:"today_sessions" json:"today_sessions"`
	TodaySeconds      float64    `db:"today_seconds" json:"today_seconds"`
}

// VoiceActivityCache 语音活动缓存
type VoiceActivityCache struct {
	IsInVoice      bool      `json:"is_in_voice"`
	CurrentChannel *string   `json:"current_channel"`
	JoinedAt       time.Time `json:"joined_at"`
	TodaySeconds   float64   `json:"today_seconds"`
	WeeklySeconds  float64   `json:"weekly_seconds"`
	MonthlySeconds float64   `json:"monthly_seconds"`
	LastUpdated    time.Time `json:"last_updated"`
}

// 常量定义

// LotteryStatus 抽奖状态
const (
	LotteryStatusActive    = "active"
	LotteryStatusEnded     = "ended"
	LotteryStatusCancelled = "cancelled"
)

// TeamStatus 团队状态
const (
	TeamStatusActive    = "active"
	TeamStatusFull      = "full"
	TeamStatusDisbanded = "disbanded"
)

// TransactionType 交易类型
const (
	TransactionTypeCheckIn              = "check_in"
	TransactionTypeLotteryParticipation = "lottery_participation"
	TransactionTypeTeamCreation         = "team_creation"
	TransactionTypeTeamCompletion       = "team_completion"
	TransactionTypeManualAdjustment     = "manual_adjustment"
	TransactionTypeAchievementReward    = "achievement_reward"
)

// 成就系统相关模型

// Achievement 成就定义
type Achievement struct {
	ID           uuid.UUID `db:"id" json:"id"`
	Name         string    `db:"name" json:"name"`
	Description  string    `db:"description" json:"description"`
	Type         string    `db:"type" json:"type"`
	TargetValue  int       `db:"target_value" json:"target_value"`
	RewardType   string    `db:"reward_type" json:"reward_type"`
	RewardValue  string    `db:"reward_value" json:"reward_value"`
	IconEmoji    *string   `db:"icon_emoji" json:"icon_emoji,omitempty"`
	Category     string    `db:"category" json:"category"`
	DisplayOrder int       `db:"display_order" json:"display_order"`
	IsHidden     bool      `db:"is_hidden" json:"is_hidden"`
	IsActive     bool      `db:"is_active" json:"is_active"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

// UserAchievement 用户成就进度
type UserAchievement struct {
	ID              uuid.UUID  `db:"id" json:"id"`
	UserID          uuid.UUID  `db:"user_id" json:"user_id"`
	AchievementID   uuid.UUID  `db:"achievement_id" json:"achievement_id"`
	Progress        int        `db:"progress" json:"progress"`
	Unlocked        bool       `db:"unlocked" json:"unlocked"`
	UnlockedAt      *time.Time `db:"unlocked_at" json:"unlocked_at,omitempty"`
	RewardClaimed   bool       `db:"reward_claimed" json:"reward_claimed"`
	RewardClaimedAt *time.Time `db:"reward_claimed_at" json:"reward_claimed_at,omitempty"`
	CreatedAt       time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at" json:"updated_at"`

	// 关联字段（通过JOIN获取）
	Achievement *Achievement `db:"-" json:"achievement,omitempty"`
}

// AchievementReward 成就奖励记录
type AchievementReward struct {
	ID                uuid.UUID  `db:"id" json:"id"`
	UserAchievementID uuid.UUID  `db:"user_achievement_id" json:"user_achievement_id"`
	UserID            uuid.UUID  `db:"user_id" json:"user_id"`
	GuildID           string     `db:"guild_id" json:"guild_id"`
	RewardType        string     `db:"reward_type" json:"reward_type"`
	RewardValue       string     `db:"reward_value" json:"reward_value"`
	PointsAwarded     int        `db:"points_awarded" json:"points_awarded"`
	RoleID            *string    `db:"role_id" json:"role_id,omitempty"`
	BadgeName         *string    `db:"badge_name" json:"badge_name,omitempty"`
	Processed         bool       `db:"processed" json:"processed"`
	ProcessedAt       *time.Time `db:"processed_at" json:"processed_at,omitempty"`
	ErrorMessage      *string    `db:"error_message" json:"error_message,omitempty"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
}

// LotteryAchievementRequirement 抽奖成就要求
type LotteryAchievementRequirement struct {
	ID            uuid.UUID `db:"id" json:"id"`
	LotteryID     uuid.UUID `db:"lottery_id" json:"lottery_id"`
	AchievementID uuid.UUID `db:"achievement_id" json:"achievement_id"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
}

// 成就类型常量
const (
	AchievementTypeCheckIn     = "checkin"
	AchievementTypeVoice       = "voice"
	AchievementTypeMessage     = "message"
	AchievementTypeLottery     = "lottery"
	AchievementTypeLotteryWin  = "lottery_win"
	AchievementTypeTeam        = "team"
	AchievementTypeTeamCreated = "team_created"
	AchievementTypeActivity    = "activity"
)

// 成就奖励类型常量
const (
	RewardTypePoints = "points"
	RewardTypeRole   = "role"
	RewardTypeBadge  = "badge"
)

// UserAchievementSummary 用户成就总览
type UserAchievementSummary struct {
	UserID            uuid.UUID  `db:"user_id" json:"user_id"`
	DiscordID         string     `db:"discord_id" json:"discord_id"`
	Username          string     `db:"username" json:"username"`
	TotalAchievements int        `db:"total_achievements" json:"total_achievements"`
	UnlockedCount     int        `db:"unlocked_count" json:"unlocked_count"`
	CompletionRate    float64    `db:"completion_rate" json:"completion_rate"`
	TotalPoints       int        `db:"total_points" json:"total_points"`
	LastUnlocked      *time.Time `db:"last_unlocked" json:"last_unlocked,omitempty"`
}

// AchievementWithProgress 带进度的成就信息
type AchievementWithProgress struct {
	Achievement
	UserProgress    int        `json:"user_progress"`
	IsUnlocked      bool       `json:"is_unlocked"`
	UnlockedAt      *time.Time `json:"unlocked_at,omitempty"`
	RewardClaimed   bool       `json:"reward_claimed"`
	ProgressPercent float64    `json:"progress_percent"`
}

// AchievementLeaderboard 成就排行榜
type AchievementLeaderboard struct {
	Rank           int     `db:"rank" json:"rank"`
	DiscordID      string  `db:"discord_id" json:"discord_id"`
	Username       string  `db:"username" json:"username"`
	UnlockedCount  int     `db:"unlocked_count" json:"unlocked_count"`
	CompletionRate float64 `db:"completion_rate" json:"completion_rate"`
	TotalPoints    int     `db:"total_points" json:"total_points"`
}

// AchievementUpdateEvent 成就更新事件
type AchievementUpdateEvent struct {
	UserID      uuid.UUID              `json:"user_id"`
	DiscordID   string                 `json:"discord_id"`
	GuildID     string                 `json:"guild_id"`
	EventType   string                 `json:"event_type"`
	Value       int                    `json:"value"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	ProcessedAt time.Time              `json:"processed_at"`
}

// =====================================================
// 竞猜系统模型
// =====================================================

// Bet 竞猜记录
type Bet struct {
	ID                uuid.UUID  `db:"id" json:"id"`
	Name              *string    `db:"name" json:"name,omitempty"`
	GuildID           string     `db:"guild_id" json:"guild_id"`
	ChannelID         string     `db:"channel_id" json:"channel_id"`
	VoiceChannelID    *string    `db:"voice_channel_id" json:"voice_channel_id,omitempty"` // 可选，不再必填
	MessageID         *string    `db:"message_id" json:"message_id,omitempty"`
	CreatorID         uuid.UUID  `db:"creator_id" json:"creator_id"`
	CreatorDiscordID  string     `db:"creator_discord_id" json:"creator_discord_id"`
	TeamAName         string     `db:"team_a_name" json:"team_a_name"`
	TeamBName         string     `db:"team_b_name" json:"team_b_name"`
	Status            string     `db:"status" json:"status"`
	TeamATotalBet     int        `db:"team_a_total_bet" json:"team_a_total_bet"`
	TeamBTotalBet     int        `db:"team_b_total_bet" json:"team_b_total_bet"`
	TotalParticipants int        `db:"total_participants" json:"total_participants"`
	WinnerTeam        *string    `db:"winner_team" json:"winner_team,omitempty"`
	SettledBy         *uuid.UUID `db:"settled_by" json:"settled_by,omitempty"`
	SettledAt         *time.Time `db:"settled_at" json:"settled_at,omitempty"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
	LockedAt          *time.Time `db:"locked_at" json:"locked_at,omitempty"`
	ExpiresAt         time.Time  `db:"expires_at" json:"expires_at"`
	UpdatedAt         time.Time  `db:"updated_at" json:"updated_at"`
}

// BetParticipant 下注记录
type BetParticipant struct {
	ID           uuid.UUID  `db:"id" json:"id"`
	BetID        uuid.UUID  `db:"bet_id" json:"bet_id"`
	UserID       uuid.UUID  `db:"user_id" json:"user_id"`
	DiscordID    string     `db:"discord_id" json:"discord_id"`
	TeamChoice   string     `db:"team_choice" json:"team_choice"` // "A" or "B"
	BetAmount    int        `db:"bet_amount" json:"bet_amount"`
	PayoutAmount int        `db:"payout_amount" json:"payout_amount"`
	IsWinner     *bool      `db:"is_winner" json:"is_winner,omitempty"`
	Settled      bool       `db:"settled" json:"settled"`
	PlacedAt     time.Time  `db:"placed_at" json:"placed_at"`
	SettledAt    *time.Time `db:"settled_at" json:"settled_at,omitempty"`
}

// 竞猜状态常量
const (
	BetStatusBetting   = "betting"   // 下注中
	BetStatusLocked    = "locked"    // 已锁定（比赛进行中）
	BetStatusSettled   = "settled"   // 已结算
	BetStatusCancelled = "cancelled" // 已取消
	BetStatusExpired   = "expired"   // 已过期
)

// 队伍选择常量
const (
	TeamChoiceA = "A"
	TeamChoiceB = "B"
)

// 下注金额限制
const (
	MinBetAmount = 100
	MaxBetAmount = 500
)

// 系统对冲账户常量
const (
	SystemDiscordID   = "0"  // 系统虚拟账户 Discord ID（使用 "0" 以满足数据库约束）
	SystemHedgeRatio  = 0.8  // 对冲比例 80%
	SystemInitialPool = 5000 // 系统初始积分池
)

// SystemUserID 系统对冲账户的固定用户 ID
var SystemUserID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// InitialUserPoints 普通用户首次产生积分记录时的初始积分
// (系统对冲账户除外,其初始化走 SystemInitialPool)
const InitialUserPoints = 1000

// initialPointsFor 返回该用户建档时应获得的初始积分
func initialPointsFor(userID uuid.UUID) int {
	if userID == SystemUserID {
		return 0
	}
	return InitialUserPoints
}

// BetStatistics 竞猜统计
type BetStatistics struct {
	GuildID         string  `db:"guild_id" json:"guild_id"`
	TotalBets       int     `db:"total_bets" json:"total_bets"`
	ActiveBets      int     `db:"active_bets" json:"active_bets"`
	LockedBets      int     `db:"locked_bets" json:"locked_bets"`
	SettledBets     int     `db:"settled_bets" json:"settled_bets"`
	CancelledBets   int     `db:"cancelled_bets" json:"cancelled_bets"`
	ExpiredBets     int     `db:"expired_bets" json:"expired_bets"`
	TotalBetAmount  int     `db:"total_bet_amount" json:"total_bet_amount"`
	AvgParticipants float64 `db:"avg_participants" json:"avg_participants"`
}

// UserBetStatistics 用户竞猜统计
type UserBetStatistics struct {
	UserID        uuid.UUID `db:"user_id" json:"user_id"`
	DiscordID     string    `db:"discord_id" json:"discord_id"`
	Username      string    `db:"username" json:"username"`
	TotalBets     int       `db:"total_bets" json:"total_bets"`
	Wins          int       `db:"wins" json:"wins"`
	Losses        int       `db:"losses" json:"losses"`
	TotalWagered  int       `db:"total_wagered" json:"total_wagered"`
	TotalWinnings int       `db:"total_winnings" json:"total_winnings"`
	NetProfit     int       `db:"net_profit" json:"net_profit"`
}
