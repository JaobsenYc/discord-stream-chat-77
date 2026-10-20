// Package protrack 追踪职业选手的美服单双排对局:维护职业选手账号库,
// 定时轮询 spectator 接口,开局与结束时在频道播报,方便排队撞车。
package protrack

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ProAccount 职业选手的一个 Riot 账号及其追踪状态(对应 pro_accounts 表)
type ProAccount struct {
	ID                   uuid.UUID  `db:"id" json:"id"`
	ProName              string     `db:"pro_name" json:"pro_name"`
	Team                 *string    `db:"team" json:"team,omitempty"`
	RiotGameName         string     `db:"riot_game_name" json:"riot_game_name"`
	RiotTagLine          string     `db:"riot_tag_line" json:"riot_tag_line"`
	RiotPUUID            *string    `db:"riot_puuid" json:"riot_puuid,omitempty"`
	IsTracked            bool       `db:"is_tracked" json:"is_tracked"`
	TrackChannelID       *string    `db:"track_channel_id" json:"track_channel_id,omitempty"`
	TrackedByDiscordID   *string    `db:"tracked_by_discord_id" json:"tracked_by_discord_id,omitempty"`
	CurrentGameID        *int64     `db:"current_game_id" json:"current_game_id,omitempty"`
	CurrentGameStartedAt *time.Time `db:"current_game_started_at" json:"current_game_started_at,omitempty"`
	CurrentChampionID    *int64     `db:"current_champion_id" json:"current_champion_id,omitempty"`
	LastGameID           *int64     `db:"last_game_id" json:"last_game_id,omitempty"`
	LastCheckedAt        *time.Time `db:"last_checked_at" json:"last_checked_at,omitempty"`
	CreatedAt            time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt            time.Time  `db:"updated_at" json:"updated_at"`
}

// RiotID 返回 名字#TAG 形式的 Riot ID
func (a *ProAccount) RiotID() string {
	return a.RiotGameName + "#" + a.RiotTagLine
}

// DisplayName 返回带战队的选手名,如 "Faker (T1)"
func (a *ProAccount) DisplayName() string {
	if a.Team != nil && *a.Team != "" {
		return a.ProName + " (" + *a.Team + ")"
	}
	return a.ProName
}

// 面向用户的业务错误(文案直接展示在 Discord 中)
var (
	ErrProNotFound    = errors.New("选手库中没有该选手,请用 名字#TAG 添加,例如 `/trackpro add target:Hide in bush#NYKR name:Faker`")
	ErrRiotIDNotFound = errors.New("该 Riot ID 不存在")
	ErrNotTracked     = errors.New("该选手当前没有被追踪")
)
