package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// BetRepository 竞猜数据访问层
type BetRepository struct {
	db *DB
}

// NewBetRepository 创建竞猜数据访问层
func NewBetRepository(db *DB) *BetRepository {
	return &BetRepository{db: db}
}

// Create 创建竞猜
func (r *BetRepository) Create(bet *Bet) error {
	query := `
		INSERT INTO bets (
			id, name, guild_id, channel_id,
			creator_id, creator_discord_id, team_a_name, team_b_name,
			status, expires_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)
	`

	_, err := r.db.Exec(query,
		bet.ID, bet.Name, bet.GuildID, bet.ChannelID,
		bet.CreatorID, bet.CreatorDiscordID, bet.TeamAName, bet.TeamBName,
		bet.Status, bet.ExpiresAt, bet.CreatedAt, bet.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("创建竞猜失败: %w", err)
	}

	return nil
}

// GetByID 根据ID获取竞猜
func (r *BetRepository) GetByID(id uuid.UUID) (*Bet, error) {
	var bet Bet
	query := `
		SELECT id, name, guild_id, channel_id, voice_channel_id, message_id,
			creator_id, creator_discord_id, team_a_name, team_b_name,
			status, team_a_total_bet, team_b_total_bet, total_participants,
			winner_team, settled_by, settled_at,
			created_at, locked_at, expires_at, updated_at
		FROM bets
		WHERE id = $1
	`

	err := r.db.Get(&bet, query, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询竞猜失败: %w", err)
	}

	return &bet, nil
}

// GetByVoiceChannel 根据语音频道获取活跃的竞猜
func (r *BetRepository) GetByVoiceChannel(voiceChannelID string) (*Bet, error) {
	var bet Bet
	query := `
		SELECT id, name, guild_id, channel_id, voice_channel_id, message_id,
			creator_id, creator_discord_id, team_a_name, team_b_name,
			status, team_a_total_bet, team_b_total_bet, total_participants,
			winner_team, settled_by, settled_at,
			created_at, locked_at, expires_at, updated_at
		FROM bets
		WHERE voice_channel_id = $1
			AND status IN ('betting', 'locked')
		ORDER BY created_at DESC
		LIMIT 1
	`

	err := r.db.Get(&bet, query, voiceChannelID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询竞猜失败: %w", err)
	}

	return &bet, nil
}

// Update 更新竞猜信息
func (r *BetRepository) Update(bet *Bet) error {
	query := `
		UPDATE bets SET
			name = $2,
			message_id = $3,
			status = $4,
			team_a_total_bet = $5,
			team_b_total_bet = $6,
			total_participants = $7,
			winner_team = $8,
			settled_by = $9,
			settled_at = $10,
			locked_at = $11,
			updated_at = $12
		WHERE id = $1
	`

	_, err := r.db.Exec(query,
		bet.ID, bet.Name, bet.MessageID, bet.Status,
		bet.TeamATotalBet, bet.TeamBTotalBet, bet.TotalParticipants,
		bet.WinnerTeam, bet.SettledBy, bet.SettledAt,
		bet.LockedAt, time.Now(),
	)

	if err != nil {
		return fmt.Errorf("更新竞猜失败: %w", err)
	}

	return nil
}

// TryMarkSettled 原子地把竞猜从 locked 置为 settled，返回是否认领成功。
// 并发结算（双击/重试）时只有一方成功，防止重复赔付。
func (r *BetRepository) TryMarkSettled(betID, settledBy uuid.UUID, winnerTeam string) (bool, error) {
	now := time.Now()
	result, err := r.db.Exec(
		`UPDATE bets SET status = $2, winner_team = $3, settled_by = $4, settled_at = $5, updated_at = $5
		WHERE id = $1 AND status = $6`,
		betID, BetStatusSettled, winnerTeam, settledBy, now, BetStatusLocked,
	)
	if err != nil {
		return false, fmt.Errorf("更新竞猜结算状态失败: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("读取结算状态影响行数失败: %w", err)
	}
	return rows > 0, nil
}

// UpdateStatus 更新竞猜状态
func (r *BetRepository) UpdateStatus(betID uuid.UUID, status string) error {
	query := `UPDATE bets SET status = $2, updated_at = $3 WHERE id = $1`
	_, err := r.db.Exec(query, betID, status, time.Now())
	if err != nil {
		return fmt.Errorf("更新竞猜状态失败: %w", err)
	}
	return nil
}

// AddParticipant 添加参与者
func (r *BetRepository) AddParticipant(participant *BetParticipant) error {
	query := `
		INSERT INTO bet_participants (
			id, bet_id, user_id, discord_id, team_choice, bet_amount, placed_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		)
	`

	_, err := r.db.Exec(query,
		participant.ID, participant.BetID, participant.UserID,
		participant.DiscordID, participant.TeamChoice, participant.BetAmount,
		participant.PlacedAt,
	)

	if err != nil {
		return fmt.Errorf("添加参与者失败: %w", err)
	}

	return nil
}

// GetParticipants 获取所有参与者
func (r *BetRepository) GetParticipants(betID uuid.UUID) ([]BetParticipant, error) {
	var participants []BetParticipant
	query := `
		SELECT id, bet_id, user_id, discord_id, team_choice, bet_amount,
			payout_amount, is_winner, settled, placed_at, settled_at
		FROM bet_participants
		WHERE bet_id = $1
		ORDER BY placed_at ASC
	`

	err := r.db.Select(&participants, query, betID)
	if err != nil {
		return nil, fmt.Errorf("查询参与者失败: %w", err)
	}

	return participants, nil
}

// GetParticipantsByTeam 获取某个队伍的参与者
func (r *BetRepository) GetParticipantsByTeam(betID uuid.UUID, team string) ([]BetParticipant, error) {
	var participants []BetParticipant
	query := `
		SELECT id, bet_id, user_id, discord_id, team_choice, bet_amount,
			payout_amount, is_winner, settled, placed_at, settled_at
		FROM bet_participants
		WHERE bet_id = $1 AND team_choice = $2
		ORDER BY placed_at ASC
	`

	err := r.db.Select(&participants, query, betID, team)
	if err != nil {
		return nil, fmt.Errorf("查询队伍参与者失败: %w", err)
	}

	return participants, nil
}

// GetTeamParticipantCount 获取某个队伍的参与人数
func (r *BetRepository) GetTeamParticipantCount(betID uuid.UUID, team string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM bet_participants WHERE bet_id = $1 AND team_choice = $2`

	err := r.db.Get(&count, query, betID, team)
	if err != nil {
		return 0, fmt.Errorf("查询队伍参与人数失败: %w", err)
	}

	return count, nil
}

// HasUserParticipated 检查用户是否已参与
func (r *BetRepository) HasUserParticipated(betID, userID uuid.UUID) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM bet_participants WHERE bet_id = $1 AND user_id = $2`

	err := r.db.Get(&count, query, betID, userID)
	if err != nil {
		return false, fmt.Errorf("检查参与状态失败: %w", err)
	}

	return count > 0, nil
}

// UpdateParticipantPayout 更新参与者赔付
func (r *BetRepository) UpdateParticipantPayout(participantID uuid.UUID, payoutAmount int, isWinner bool) error {
	query := `
		UPDATE bet_participants
		SET payout_amount = $2, is_winner = $3, settled = true, settled_at = $4
		WHERE id = $1
	`

	_, err := r.db.Exec(query, participantID, payoutAmount, isWinner, time.Now())
	if err != nil {
		return fmt.Errorf("更新参与者赔付失败: %w", err)
	}

	return nil
}

// GetExpiredBets 获取过期的竞猜
func (r *BetRepository) GetExpiredBets(guildID string) ([]Bet, error) {
	var bets []Bet
	query := `
		SELECT id, name, guild_id, channel_id, voice_channel_id, message_id,
			creator_id, creator_discord_id, team_a_name, team_b_name,
			status, team_a_total_bet, team_b_total_bet, total_participants,
			winner_team, settled_by, settled_at,
			created_at, locked_at, expires_at, updated_at
		FROM bets
		WHERE guild_id = $1
			AND status = 'betting'
			AND expires_at < $2
	`

	err := r.db.Select(&bets, query, guildID, time.Now())
	if err != nil {
		return nil, fmt.Errorf("查询过期竞猜失败: %w", err)
	}

	return bets, nil
}

// GetActiveBets 获取活跃的竞猜
func (r *BetRepository) GetActiveBets(guildID string) ([]Bet, error) {
	var bets []Bet
	query := `
		SELECT id, name, guild_id, channel_id, voice_channel_id, message_id,
			creator_id, creator_discord_id, team_a_name, team_b_name,
			status, team_a_total_bet, team_b_total_bet, total_participants,
			winner_team, settled_by, settled_at,
			created_at, locked_at, expires_at, updated_at
		FROM bets
		WHERE guild_id = $1 AND status IN ('betting', 'locked')
		ORDER BY created_at DESC
	`

	err := r.db.Select(&bets, query, guildID)
	if err != nil {
		return nil, fmt.Errorf("查询活跃竞猜失败: %w", err)
	}

	return bets, nil
}

// GetUserBetHistory 获取用户竞猜历史
func (r *BetRepository) GetUserBetHistory(userID uuid.UUID, limit int) ([]BetParticipant, error) {
	var participants []BetParticipant
	query := `
		SELECT id, bet_id, user_id, discord_id, team_choice, bet_amount,
			payout_amount, is_winner, settled, placed_at, settled_at
		FROM bet_participants
		WHERE user_id = $1
		ORDER BY placed_at DESC
		LIMIT $2
	`

	err := r.db.Select(&participants, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("查询用户竞猜历史失败: %w", err)
	}

	return participants, nil
}

// IncrementBetStats 增加竞猜统计（用于下注时）
func (r *BetRepository) IncrementBetStats(betID uuid.UUID, team string, amount int) error {
	var query string
	if team == TeamChoiceA {
		query = `
			UPDATE bets
			SET team_a_total_bet = team_a_total_bet + $2,
				total_participants = total_participants + 1,
				updated_at = $3
			WHERE id = $1
		`
	} else {
		query = `
			UPDATE bets
			SET team_b_total_bet = team_b_total_bet + $2,
				total_participants = total_participants + 1,
				updated_at = $3
			WHERE id = $1
		`
	}

	_, err := r.db.Exec(query, betID, amount, time.Now())
	if err != nil {
		return fmt.Errorf("更新竞猜统计失败: %w", err)
	}

	return nil
}

// GetBetStatistics 获取竞猜统计
func (r *BetRepository) GetBetStatistics(guildID string) (*BetStatistics, error) {
	var stats BetStatistics
	query := `
		SELECT * FROM bet_statistics WHERE guild_id = $1
	`

	err := r.db.Get(&stats, query, guildID)
	if err == sql.ErrNoRows {
		// 返回空统计
		return &BetStatistics{GuildID: guildID}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询竞猜统计失败: %w", err)
	}

	return &stats, nil
}

// GetUserBetStatistics 获取用户竞猜统计
func (r *BetRepository) GetUserBetStatistics(userID uuid.UUID) (*UserBetStatistics, error) {
	var stats UserBetStatistics
	query := `
		SELECT * FROM user_bet_statistics WHERE user_id = $1
	`

	err := r.db.Get(&stats, query, userID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询用户竞猜统计失败: %w", err)
	}

	return &stats, nil
}

// CancelBet 取消竞猜（事务性操作，退还所有积分）
func (r *BetRepository) CancelBet(bet *Bet) error {
	// 开启事务
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	// 更新竞猜状态（仅允许从可取消状态转移，防止过期监控与取消按钮并发触发双重退款）
	result, err := tx.Exec(
		`UPDATE bets SET status = $2, updated_at = $3 WHERE id = $1 AND status IN ($4, $5)`,
		bet.ID, BetStatusCancelled, time.Now(), BetStatusBetting, BetStatusLocked,
	)
	if err != nil {
		return fmt.Errorf("更新竞猜状态失败: %w", err)
	}
	if rows, err := result.RowsAffected(); err == nil && rows == 0 {
		return fmt.Errorf("竞猜已结算或已取消，无法重复取消")
	}

	// 获取所有参与者
	participants, err := r.GetParticipants(bet.ID)
	if err != nil {
		return fmt.Errorf("获取参与者失败: %w", err)
	}

	// 退还所有参与者的积分（已结算过的跳过，保证退款幂等）
	for _, participant := range participants {
		if participant.Settled {
			continue
		}
		// 增加用户积分
		_, err = tx.Exec(
			`UPDATE user_points
			SET available_points = available_points + $1, updated_at = $2
			WHERE user_id = $3 AND guild_id = $4`,
			participant.BetAmount, time.Now(), participant.UserID, bet.GuildID,
		)
		if err != nil {
			return fmt.Errorf("退还用户积分失败: %w", err)
		}

		// 记录积分变动
		_, err = tx.Exec(
			`INSERT INTO point_transactions
			(id, user_id, guild_id, amount, transaction_type, description, related_id, related_type, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			uuid.New(), participant.UserID, bet.GuildID, participant.BetAmount,
			"bet_refund", "竞猜取消退款", bet.ID, "bet", time.Now(),
		)
		if err != nil {
			return fmt.Errorf("记录积分变动失败: %w", err)
		}

		// 标记为已结算
		_, err = tx.Exec(
			`UPDATE bet_participants SET settled = true, settled_at = $2 WHERE id = $1`,
			participant.ID, time.Now(),
		)
		if err != nil {
			return fmt.Errorf("更新参与者状态失败: %w", err)
		}
	}

	// 提交事务
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	return nil
}
