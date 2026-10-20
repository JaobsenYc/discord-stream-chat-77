package riotwatch

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/database"
)

// Repository Riot Watch 数据库操作
type Repository struct {
	db *database.DB
}

// NewRepository 创建新的 Repository
func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

// Create 创建新的绑定
func (r *Repository) Create(binding *Binding) error {
	query := `
		INSERT INTO riot_watch_bindings (
			id, guild_id, channel_id, created_by, created_by_discord_id,
			riot_id, riot_tag, riot_puuid, death_threshold, is_active,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)`

	_, err := r.db.Exec(query,
		binding.ID, binding.GuildID, binding.ChannelID,
		binding.CreatedBy, binding.CreatedByDiscordID,
		binding.RiotID, binding.RiotTag, binding.RiotPUUID,
		binding.DeathThreshold, binding.IsActive,
		binding.CreatedAt, binding.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create binding: %w", err)
	}

	return nil
}

// GetByID 根据 ID 获取绑定
func (r *Repository) GetByID(id uuid.UUID) (*Binding, error) {
	var binding Binding
	query := `SELECT * FROM riot_watch_bindings WHERE id = $1`

	err := r.db.Get(&binding, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get binding: %w", err)
	}

	return &binding, nil
}

// GetByChannelAndPUUID 检查频道是否已绑定该 PUUID (仅活跃)
func (r *Repository) GetByChannelAndPUUID(guildID, channelID, puuid string) (*Binding, error) {
	var binding Binding
	query := `
		SELECT * FROM riot_watch_bindings
		WHERE guild_id = $1 AND channel_id = $2 AND riot_puuid = $3 AND is_active = TRUE`

	err := r.db.Get(&binding, query, guildID, channelID, puuid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get binding: %w", err)
	}

	return &binding, nil
}

// GetByChannelAndPUUIDIncludeInactive 检查频道是否已绑定该 PUUID (包括已停用)
func (r *Repository) GetByChannelAndPUUIDIncludeInactive(guildID, channelID, puuid string) (*Binding, error) {
	var binding Binding
	query := `
		SELECT * FROM riot_watch_bindings
		WHERE guild_id = $1 AND channel_id = $2 AND riot_puuid = $3`

	err := r.db.Get(&binding, query, guildID, channelID, puuid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get binding: %w", err)
	}

	return &binding, nil
}

// ReactivateBinding 重新激活已停用的绑定，同时更新阈值和玩家名称（可能改名了）
func (r *Repository) ReactivateBinding(id uuid.UUID, riotID, riotTag string, deathThreshold float64) error {
	query := `
		UPDATE riot_watch_bindings
		SET is_active = TRUE, riot_id = $2, riot_tag = $3, death_threshold = $4, updated_at = NOW()
		WHERE id = $1`

	_, err := r.db.Exec(query, id, riotID, riotTag, deathThreshold)
	if err != nil {
		return fmt.Errorf("failed to reactivate binding: %w", err)
	}

	return nil
}

// GetActiveBindings 获取指定服务器的所有活跃绑定
func (r *Repository) GetActiveBindings(guildID string) ([]Binding, error) {
	var bindings []Binding
	query := `
		SELECT * FROM riot_watch_bindings
		WHERE guild_id = $1 AND is_active = TRUE
		ORDER BY last_checked_at ASC NULLS FIRST`

	err := r.db.Select(&bindings, query, guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active bindings: %w", err)
	}

	return bindings, nil
}

// GetAllActiveBindings 获取所有活跃绑定（跨服务器）
func (r *Repository) GetAllActiveBindings() ([]Binding, error) {
	var bindings []Binding
	query := `
		SELECT * FROM riot_watch_bindings
		WHERE is_active = TRUE
		ORDER BY last_checked_at ASC NULLS FIRST`

	err := r.db.Select(&bindings, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all active bindings: %w", err)
	}

	return bindings, nil
}

// GetBindingsInGame 获取当前正在追踪游戏的绑定
func (r *Repository) GetBindingsInGame() ([]Binding, error) {
	var bindings []Binding
	query := `
		SELECT * FROM riot_watch_bindings
		WHERE is_active = TRUE AND current_game_id IS NOT NULL`

	err := r.db.Select(&bindings, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get bindings in game: %w", err)
	}

	return bindings, nil
}

// GetBindingsByChannel 获取指定频道的所有绑定
func (r *Repository) GetBindingsByChannel(guildID, channelID string) ([]Binding, error) {
	var bindings []Binding
	query := `
		SELECT * FROM riot_watch_bindings
		WHERE guild_id = $1 AND channel_id = $2 AND is_active = TRUE
		ORDER BY created_at DESC`

	err := r.db.Select(&bindings, query, guildID, channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get bindings by channel: %w", err)
	}

	return bindings, nil
}

// UpdateGameState 更新游戏追踪状态
func (r *Repository) UpdateGameState(id uuid.UUID, gameID *string, winBetID, deathBetID *uuid.UUID) error {
	query := `
		UPDATE riot_watch_bindings
		SET current_game_id = $2,
		    current_win_bet_id = $3,
		    current_death_bet_id = $4,
		    updated_at = NOW()
		WHERE id = $1`

	_, err := r.db.Exec(query, id, gameID, winBetID, deathBetID)
	if err != nil {
		return fmt.Errorf("failed to update game state: %w", err)
	}

	return nil
}

// UpdateLastChecked 更新最后检查时间
func (r *Repository) UpdateLastChecked(id uuid.UUID) error {
	query := `
		UPDATE riot_watch_bindings
		SET last_checked_at = NOW(), updated_at = NOW()
		WHERE id = $1`

	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to update last checked: %w", err)
	}

	return nil
}

// UpdateLastGameEnded 更新最后游戏结束时间
func (r *Repository) UpdateLastGameEnded(id uuid.UUID) error {
	query := `
		UPDATE riot_watch_bindings
		SET last_game_ended_at = NOW(), updated_at = NOW()
		WHERE id = $1`

	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to update last game ended: %w", err)
	}

	return nil
}

// ClearGameState 清除游戏状态（游戏结束后调用）
func (r *Repository) ClearGameState(id uuid.UUID) error {
	query := `
		UPDATE riot_watch_bindings
		SET current_game_id = NULL,
		    current_win_bet_id = NULL,
		    current_death_bet_id = NULL,
		    last_game_ended_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1`

	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to clear game state: %w", err)
	}

	return nil
}

// SetInactive 停用绑定
func (r *Repository) SetInactive(id uuid.UUID) error {
	query := `
		UPDATE riot_watch_bindings
		SET is_active = FALSE, updated_at = NOW()
		WHERE id = $1`

	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to set inactive: %w", err)
	}

	return nil
}

// Delete 删除绑定（物理删除）
func (r *Repository) Delete(id uuid.UUID) error {
	query := `DELETE FROM riot_watch_bindings WHERE id = $1`

	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete binding: %w", err)
	}

	return nil
}

// UpdateRiotInfo 更新 Riot 账号显示信息（用于玩家改名后刷新）
func (r *Repository) UpdateRiotInfo(id uuid.UUID, riotID, riotTag string) error {
	query := `
		UPDATE riot_watch_bindings
		SET riot_id = $2, riot_tag = $3, updated_at = NOW()
		WHERE id = $1`

	_, err := r.db.Exec(query, id, riotID, riotTag)
	if err != nil {
		return fmt.Errorf("failed to update riot info: %w", err)
	}

	return nil
}

// GetBindingsNeedingRefresh 获取需要刷新显示名的绑定（超过24小时未更新）
func (r *Repository) GetBindingsNeedingRefresh(olderThan time.Duration) ([]Binding, error) {
	var bindings []Binding
	cutoff := time.Now().Add(-olderThan)

	query := `
		SELECT * FROM riot_watch_bindings
		WHERE is_active = TRUE AND updated_at < $1
		ORDER BY updated_at ASC
		LIMIT 10`

	err := r.db.Select(&bindings, query, cutoff)
	if err != nil {
		return nil, fmt.Errorf("failed to get bindings needing refresh: %w", err)
	}

	return bindings, nil
}
