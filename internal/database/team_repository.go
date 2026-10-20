package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// TeamRepository 团队数据访问层
type TeamRepository struct {
	db *DB
}

// NewTeamRepository 创建团队仓库
func NewTeamRepository(db *DB) *TeamRepository {
	return &TeamRepository{db: db}
}

// Create 创建新团队
func (r *TeamRepository) Create(team *Team) error {
	query := `
		INSERT INTO teams (
			team_id, guild_id, text_channel_id, voice_channel_id, 
			game_name, max_players, current_players, 
			creator_id, creator_discord_id, status
		) VALUES (
			:team_id, :guild_id, :text_channel_id, :voice_channel_id,
			:game_name, :max_players, :current_players,
			:creator_id, :creator_discord_id, :status
		) RETURNING id, created_at, updated_at`

	rows, err := r.db.NamedQuery(query, team)
	if err != nil {
		return fmt.Errorf("failed to create team: %w", err)
	}
	defer rows.Close()

	if rows.Next() {
		err = rows.Scan(&team.ID, &team.CreatedAt, &team.UpdatedAt)
		if err != nil {
			return fmt.Errorf("failed to scan team: %w", err)
		}
	}

	// 缓存团队信息
	r.cacheTeam(team)

	return nil
}

// GetByID 根据 ID 获取团队
func (r *TeamRepository) GetByID(id uuid.UUID) (*Team, error) {
	// 先尝试从缓存获取
	cacheKey := fmt.Sprintf("team:id:%s", id)
	cached, err := r.db.GetCache(cacheKey)
	if err == nil && cached != "" {
		var team Team
		if err := json.Unmarshal([]byte(cached), &team); err == nil {
			return &team, nil
		}
	}

	// 从数据库获取
	var team Team
	query := `SELECT * FROM teams WHERE id = $1`
	err = r.db.Get(&team, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get team by id: %w", err)
	}

	// 缓存结果
	r.cacheTeam(&team)

	return &team, nil
}

// GetByTeamID 根据团队 ID 获取团队
func (r *TeamRepository) GetByTeamID(teamID string) (*Team, error) {
	// 先尝试从缓存获取
	cacheKey := fmt.Sprintf("team:teamid:%s", teamID)
	cached, err := r.db.GetCache(cacheKey)
	if err == nil && cached != "" {
		var team Team
		if err := json.Unmarshal([]byte(cached), &team); err == nil {
			return &team, nil
		}
	}

	// 从数据库获取
	var team Team
	query := `SELECT * FROM teams WHERE team_id = $1`
	err = r.db.Get(&team, query, teamID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get team by team id: %w", err)
	}

	// 缓存结果
	r.cacheTeam(&team)

	return &team, nil
}

// GetActiveTeamsByGuild 获取服务器的活跃团队
func (r *TeamRepository) GetActiveTeamsByGuild(guildID string) ([]Team, error) {
	query := `
		SELECT * FROM teams 
		WHERE guild_id = $1 AND status = $2 
		ORDER BY created_at DESC`

	var teams []Team
	err := r.db.Select(&teams, query, guildID, TeamStatusActive)
	if err != nil {
		return nil, fmt.Errorf("failed to get active teams: %w", err)
	}

	return teams, nil
}

// GetActiveTeamByCreator 获取创建者的活跃团队
func (r *TeamRepository) GetActiveTeamByCreator(creatorID uuid.UUID) (*Team, error) {
	var team Team
	query := `SELECT * FROM teams WHERE creator_id = $1 AND status = $2`
	err := r.db.Get(&team, query, creatorID, TeamStatusActive)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get active team by creator: %w", err)
	}

	return &team, nil
}

// UpdateStatus 更新团队状态
func (r *TeamRepository) UpdateStatus(teamID uuid.UUID, status string) error {
	query := `UPDATE teams SET status = $2 WHERE id = $1`
	_, err := r.db.Exec(query, teamID, status)
	if err != nil {
		return fmt.Errorf("failed to update team status: %w", err)
	}

	// 如果是解散，更新解散时间
	if status == TeamStatusDisbanded {
		disbandQuery := `UPDATE teams SET disbanded_at = $2 WHERE id = $1`
		_, err = r.db.Exec(disbandQuery, teamID, time.Now().UTC())
		if err != nil {
			return fmt.Errorf("failed to update disbanded time: %w", err)
		}
	}

	// 清除缓存
	r.clearTeamCache(teamID)

	return nil
}

// UpdatePlayerCount 更新玩家数量
func (r *TeamRepository) UpdatePlayerCount(teamID uuid.UUID, count int) error {
	query := `UPDATE teams SET current_players = $2 WHERE id = $1`
	_, err := r.db.Exec(query, teamID, count)
	if err != nil {
		return fmt.Errorf("failed to update player count: %w", err)
	}

	// 如果满员，更新状态
	var team Team
	getQuery := `SELECT max_players FROM teams WHERE id = $1`
	err = r.db.Get(&team, getQuery, teamID)
	if err == nil && count >= team.MaxPlayers {
		_ = r.UpdateStatus(teamID, TeamStatusFull)
	}

	// 清除缓存
	r.clearTeamCache(teamID)

	return nil
}

// AddMember 添加团队成员
func (r *TeamRepository) AddMember(teamID, userID uuid.UUID, discordID string) error {
	return r.db.WithTx(func(tx *sqlx.Tx) error {
		// 检查是否已经是成员
		var exists bool
		checkQuery := `SELECT EXISTS(SELECT 1 FROM team_members WHERE team_id = $1 AND user_id = $2 AND is_active = true)`
		err := tx.Get(&exists, checkQuery, teamID, userID)
		if err != nil {
			return fmt.Errorf("failed to check member existence: %w", err)
		}

		if exists {
			return fmt.Errorf("user is already a member")
		}

		// 添加成员
		member := &TeamMember{
			ID:        uuid.New(),
			TeamID:    teamID,
			UserID:    userID,
			DiscordID: discordID,
			JoinedAt:  time.Now().UTC(),
			IsActive:  true,
		}

		insertQuery := `
			INSERT INTO team_members (id, team_id, user_id, discord_id, joined_at, is_active)
			VALUES ($1, $2, $3, $4, $5, $6)`

		_, err = tx.Exec(insertQuery, member.ID, member.TeamID, member.UserID, member.DiscordID, member.JoinedAt, member.IsActive)
		if err != nil {
			return fmt.Errorf("failed to add member: %w", err)
		}

		// 更新团队人数
		updateQuery := `UPDATE teams SET current_players = current_players + 1, total_join_requests = total_join_requests + 1 WHERE id = $1`
		_, err = tx.Exec(updateQuery, teamID)
		if err != nil {
			return fmt.Errorf("failed to update team count: %w", err)
		}

		// 更新用户统计
		userUpdateQuery := `UPDATE users SET total_teams_joined = total_teams_joined + 1 WHERE id = $1`
		_, err = tx.Exec(userUpdateQuery, userID)
		if err != nil {
			return fmt.Errorf("failed to update user stats: %w", err)
		}

		return nil
	})
}

// RemoveMember 移除团队成员
func (r *TeamRepository) RemoveMember(teamID, userID uuid.UUID) error {
	return r.db.WithTx(func(tx *sqlx.Tx) error {
		// 更新成员状态
		updateMemberQuery := `
			UPDATE team_members 
			SET is_active = false, left_at = $3 
			WHERE team_id = $1 AND user_id = $2 AND is_active = true`

		_, err := tx.Exec(updateMemberQuery, teamID, userID, time.Now().UTC())
		if err != nil {
			return fmt.Errorf("failed to update member: %w", err)
		}

		// 更新团队人数
		updateTeamQuery := `UPDATE teams SET current_players = current_players - 1 WHERE id = $1`
		_, err = tx.Exec(updateTeamQuery, teamID)
		if err != nil {
			return fmt.Errorf("failed to update team count: %w", err)
		}

		return nil
	})
}

// GetMembers 获取团队成员
func (r *TeamRepository) GetMembers(teamID uuid.UUID) ([]TeamMember, error) {
	query := `
		SELECT * FROM team_members 
		WHERE team_id = $1 AND is_active = true 
		ORDER BY joined_at`

	var members []TeamMember
	err := r.db.Select(&members, query, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team members: %w", err)
	}

	return members, nil
}

// GetTeamStats 获取团队统计
func (r *TeamRepository) GetTeamStats(guildID string) ([]ActiveTeamStats, error) {
	query := `SELECT * FROM active_team_stats WHERE guild_id = $1`

	var stats []ActiveTeamStats
	err := r.db.Select(&stats, query, guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team stats: %w", err)
	}

	return stats, nil
}

// cacheTeam 缓存团队信息
func (r *TeamRepository) cacheTeam(team *Team) {
	if data, err := json.Marshal(team); err == nil {
		_ = r.db.SetCacheWithTTL(fmt.Sprintf("team:id:%s", team.ID), data, 5*time.Minute)
		_ = r.db.SetCacheWithTTL(fmt.Sprintf("team:teamid:%s", team.TeamID), data, 5*time.Minute)
	}
}

// clearTeamCache 清除团队缓存
func (r *TeamRepository) clearTeamCache(teamID uuid.UUID) {
	team, _ := r.GetByID(teamID)
	if team != nil {
		_ = r.db.DeleteCache(
			fmt.Sprintf("team:id:%s", team.ID),
			fmt.Sprintf("team:teamid:%s", team.TeamID),
		)
	}
}

// CleanupInactiveTeams 清理不活跃的团队
func (r *TeamRepository) CleanupInactiveTeams(inactiveDuration time.Duration) error {
	cutoffTime := time.Now().UTC().Add(-inactiveDuration)

	query := `
		UPDATE teams 
		SET status = $1, disbanded_at = $2 
		WHERE status = $3 AND updated_at < $4`

	result, err := r.db.Exec(query, TeamStatusDisbanded, time.Now().UTC(), TeamStatusActive, cutoffTime)
	if err != nil {
		return fmt.Errorf("failed to cleanup inactive teams: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected > 0 {
		// 清理相关的成员记录
		memberQuery := `
			UPDATE team_members 
			SET is_active = false, left_at = $1 
			WHERE team_id IN (
				SELECT id FROM teams 
				WHERE status = $2 AND disbanded_at IS NOT NULL
			) AND is_active = true`

		_, err = r.db.Exec(memberQuery, time.Now().UTC(), TeamStatusDisbanded)
		if err != nil {
			return fmt.Errorf("failed to cleanup team members: %w", err)
		}
	}

	return nil
}
