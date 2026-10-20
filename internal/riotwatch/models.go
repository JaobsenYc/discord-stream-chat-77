package riotwatch

import (
	"time"

	"github.com/google/uuid"
)

// Binding 表示频道与 Riot 账号的绑定关系
type Binding struct {
	ID                 uuid.UUID  `db:"id" json:"id"`
	GuildID            string     `db:"guild_id" json:"guild_id"`
	ChannelID          string     `db:"channel_id" json:"channel_id"`
	CreatedBy          uuid.UUID  `db:"created_by" json:"created_by"`
	CreatedByDiscordID string     `db:"created_by_discord_id" json:"created_by_discord_id"`
	RiotID             string     `db:"riot_id" json:"riot_id"`
	RiotTag            string     `db:"riot_tag" json:"riot_tag"`
	RiotPUUID          *string    `db:"riot_puuid" json:"riot_puuid,omitempty"`
	DeathThreshold     float64    `db:"death_threshold" json:"death_threshold"`
	IsActive           bool       `db:"is_active" json:"is_active"`
	CurrentGameID      *string    `db:"current_game_id" json:"current_game_id,omitempty"`
	CurrentWinBetID    *uuid.UUID `db:"current_win_bet_id" json:"current_win_bet_id,omitempty"`
	CurrentDeathBetID  *uuid.UUID `db:"current_death_bet_id" json:"current_death_bet_id,omitempty"`
	LastCheckedAt      *time.Time `db:"last_checked_at" json:"last_checked_at,omitempty"`
	LastGameEndedAt    *time.Time `db:"last_game_ended_at" json:"last_game_ended_at,omitempty"`
	CreatedAt          time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time  `db:"updated_at" json:"updated_at"`
}

// ============================================================================
// Riot API Response Models
// ============================================================================

// Account 表示 Riot Account API 响应
type Account struct {
	PUUID    string `json:"puuid"`
	GameName string `json:"gameName"`
	TagLine  string `json:"tagLine"`
}

// ActiveGame 表示 Spectator API 响应 - 当前进行中的游戏
type ActiveGame struct {
	GameID            int64         `json:"gameId"`
	GameType          string        `json:"gameType"`
	GameStartTime     int64         `json:"gameStartTime"`
	MapID             int64         `json:"mapId"`
	GameLength        int64         `json:"gameLength"`
	GameQueueConfigID int64         `json:"gameQueueConfigId"`
	Participants      []Participant `json:"participants"`
}

// Participant 表示游戏中的参与者
type Participant struct {
	PUUID        string `json:"puuid"`
	SummonerName string `json:"summonerName"`
	SummonerID   string `json:"summonerId"`
	TeamID       int64  `json:"teamId"`
	ChampionID   int64  `json:"championId"`
}

// Match 表示 Match API 响应 - 已结束的对局
type Match struct {
	Metadata MatchMetadata `json:"metadata"`
	Info     MatchInfo     `json:"info"`
}

// MatchMetadata 对局元数据
type MatchMetadata struct {
	MatchID      string   `json:"matchId"`
	Participants []string `json:"participants"` // PUUIDs
}

// MatchInfo 对局详细信息
type MatchInfo struct {
	GameDuration     int64              `json:"gameDuration"`
	GameEndTimestamp int64              `json:"gameEndTimestamp"`
	QueueID          int64              `json:"queueId"`
	Participants     []MatchParticipant `json:"participants"`
}

// MatchParticipant 对局中的参与者结果
type MatchParticipant struct {
	PUUID        string `json:"puuid"`
	SummonerName string `json:"summonerName"`
	ChampionName string `json:"championName"`
	TeamID       int    `json:"teamId"`
	Win          bool   `json:"win"`
	Kills        int    `json:"kills"`
	Deaths       int    `json:"deaths"`
	Assists      int    `json:"assists"`
}

// ============================================================================
// Constants
// ============================================================================

const (
	// QueueSoloDuo 单双排队列ID
	QueueSoloDuo int64 = 420

	// QueueFlexRanked 灵活排位队列ID
	QueueFlexRanked int64 = 440
)

// IsSoloDuoRanked 检查是否是单双排游戏
func IsSoloDuoRanked(queueID int64) bool {
	return queueID == QueueSoloDuo
}

// IsRankedGame 检查是否是排位游戏（包括灵活排位）
func IsRankedGame(queueID int64) bool {
	return queueID == QueueSoloDuo || queueID == QueueFlexRanked
}
