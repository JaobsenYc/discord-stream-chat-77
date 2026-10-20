package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// LotteryRepository 抽奖数据访问层
type LotteryRepository struct {
	db *DB
}

// NewLotteryRepository 创建抽奖仓库
func NewLotteryRepository(db *DB) *LotteryRepository {
	return &LotteryRepository{db: db}
}

// Create 创建抽奖
func (r *LotteryRepository) Create(lottery *Lottery) error {
	query := `
		INSERT INTO lotteries (id, name, description, guild_id, channel_id, created_by,
			max_participants, cost_points, prize_description, start_time, end_time, 
			status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`

	_, err := r.db.Exec(query,
		lottery.ID, lottery.Name, lottery.Description, lottery.GuildID, lottery.ChannelID,
		lottery.CreatedBy, lottery.MaxParticipants, lottery.CostPoints, lottery.PrizeDescription,
		lottery.StartTime, lottery.EndTime, lottery.Status, lottery.CreatedAt, lottery.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create lottery: %w", err)
	}

	return nil
}

// GetByID 根据ID获取抽奖
func (r *LotteryRepository) GetByID(id uuid.UUID) (*Lottery, error) {
	var lottery Lottery
	query := `SELECT * FROM lotteries WHERE id = $1`

	err := r.db.Get(&lottery, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("lottery not found")
		}
		return nil, fmt.Errorf("failed to get lottery: %w", err)
	}

	return &lottery, nil
}

// Update 更新抽奖
func (r *LotteryRepository) Update(lottery *Lottery) error {
	query := `
		UPDATE lotteries 
		SET name = $2, description = $3, max_participants = $4, cost_points = $5,
			prize_description = $6, end_time = $7, status = $8, winner_count = $9,
			winners = $10, winner_selected_at = $11, updated_at = $12
		WHERE id = $1`

	_, err := r.db.Exec(query,
		lottery.ID, lottery.Name, lottery.Description, lottery.MaxParticipants,
		lottery.CostPoints, lottery.PrizeDescription, lottery.EndTime, lottery.Status,
		lottery.WinnerCount, lottery.Winners, lottery.WinnerSelectedAt, lottery.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update lottery: %w", err)
	}

	return nil
}

// UpdateStatus 更新抽奖状态
func (r *LotteryRepository) UpdateStatus(id uuid.UUID, status string) error {
	query := `UPDATE lotteries SET status = $2, updated_at = $3 WHERE id = $1`

	_, err := r.db.Exec(query, id, status, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update lottery status: %w", err)
	}

	return nil
}

// TryMarkEnded 原子地把抽奖从 active 置为 ended，返回是否认领成功。
// 定时开奖与手动 /lottery draw 并发触发时只有一方成功，防止重复开奖。
func (r *LotteryRepository) TryMarkEnded(id uuid.UUID) (bool, error) {
	query := `UPDATE lotteries SET status = $2, updated_at = $3 WHERE id = $1 AND status = $4`

	result, err := r.db.Exec(query, id, LotteryStatusEnded, time.Now(), LotteryStatusActive)
	if err != nil {
		return false, fmt.Errorf("failed to mark lottery ended: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to read rows affected: %w", err)
	}
	return rows > 0, nil
}

// GetActiveByGuild 获取服务器的活跃抽奖
func (r *LotteryRepository) GetActiveByGuild(guildID string) ([]Lottery, error) {
	var lotteries []Lottery
	query := `
		SELECT * FROM lotteries 
		WHERE guild_id = $1 AND status = $2 
		ORDER BY created_at DESC`

	err := r.db.Select(&lotteries, query, guildID, LotteryStatusActive)
	if err != nil {
		return nil, fmt.Errorf("failed to get active lotteries: %w", err)
	}

	return lotteries, nil
}

// GetExpiredLotteries 获取过期但未开奖的抽奖
func (r *LotteryRepository) GetExpiredLotteries(guildID string) ([]Lottery, error) {
	var lotteries []Lottery
	query := `
		SELECT * FROM lotteries 
		WHERE guild_id = $1 AND status = $2 AND end_time < $3
		ORDER BY end_time ASC`

	err := r.db.Select(&lotteries, query, guildID, LotteryStatusActive, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to get expired lotteries: %w", err)
	}

	return lotteries, nil
}

// AddParticipant 添加参与者
func (r *LotteryRepository) AddParticipant(participant *LotteryParticipant) error {
	query := `
		INSERT INTO lottery_participants (id, lottery_id, user_id, discord_id, points_spent, participated_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := r.db.Exec(query,
		participant.ID, participant.LotteryID, participant.UserID,
		participant.DiscordID, participant.PointsSpent, participant.ParticipatedAt)

	if err != nil {
		return fmt.Errorf("failed to add participant: %w", err)
	}

	return nil
}

// HasUserParticipated 检查用户是否已参与
func (r *LotteryRepository) HasUserParticipated(lotteryID, userID uuid.UUID) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM lottery_participants WHERE lottery_id = $1 AND user_id = $2)`

	err := r.db.Get(&exists, query, lotteryID, userID)
	if err != nil {
		return false, fmt.Errorf("failed to check participation: %w", err)
	}

	return exists, nil
}

// GetParticipants 获取抽奖参与者
func (r *LotteryRepository) GetParticipants(lotteryID uuid.UUID) ([]LotteryParticipant, error) {
	var participants []LotteryParticipant
	query := `
		SELECT * FROM lottery_participants 
		WHERE lottery_id = $1 
		ORDER BY participated_at ASC`

	err := r.db.Select(&participants, query, lotteryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participants: %w", err)
	}

	return participants, nil
}

// GetParticipantCount 获取参与者数量
func (r *LotteryRepository) GetParticipantCount(lotteryID uuid.UUID) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM lottery_participants WHERE lottery_id = $1`

	err := r.db.Get(&count, query, lotteryID)
	if err != nil {
		return 0, fmt.Errorf("failed to get participant count: %w", err)
	}

	return count, nil
}

// AddAchievementRequirements 添加成就要求
func (r *LotteryRepository) AddAchievementRequirements(lotteryID uuid.UUID, achievementIDs []uuid.UUID) error {
	if len(achievementIDs) == 0 {
		return nil
	}

	return r.db.WithTx(func(tx *sqlx.Tx) error {
		query := `
			INSERT INTO lottery_achievement_requirements (id, lottery_id, achievement_id, created_at)
			VALUES ($1, $2, $3, $4)`

		for _, achievementID := range achievementIDs {
			_, err := tx.Exec(query, uuid.New(), lotteryID, achievementID, time.Now())
			if err != nil {
				return fmt.Errorf("failed to add achievement requirement: %w", err)
			}
		}

		return nil
	})
}

// GetAchievementRequirements 获取成就要求
func (r *LotteryRepository) GetAchievementRequirements(lotteryID uuid.UUID) ([]LotteryAchievementRequirement, error) {
	var requirements []LotteryAchievementRequirement
	query := `
		SELECT * FROM lottery_achievement_requirements 
		WHERE lottery_id = $1 
		ORDER BY created_at ASC`

	err := r.db.Select(&requirements, query, lotteryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get achievement requirements: %w", err)
	}

	return requirements, nil
}

// GetUserLotteryHistory 获取用户抽奖历史
func (r *LotteryRepository) GetUserLotteryHistory(userID uuid.UUID, guildID string, limit int) ([]UserLotteryHistory, error) {
	var history []UserLotteryHistory
	query := `
		SELECT 
			l.id as lottery_id,
			l.name as lottery_name,
			l.prize_description,
			l.status,
			l.created_at as lottery_created_at,
			l.end_time as lottery_end_time,
			lp.participated_at,
			lp.points_spent,
			CASE WHEN l.winners @> jsonb_build_array(jsonb_build_object('user_id', $1::text)) THEN true ELSE false END as is_winner
		FROM lottery_participants lp
		JOIN lotteries l ON lp.lottery_id = l.id
		WHERE lp.user_id = $1 AND l.guild_id = $2
		ORDER BY lp.participated_at DESC
		LIMIT $3`

	err := r.db.Select(&history, query, userID, guildID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get user lottery history: %w", err)
	}

	return history, nil
}

// GetLotteryStats 获取抽奖统计
func (r *LotteryRepository) GetLotteryStats(guildID string) (*LotteryStats, error) {
	var stats LotteryStats

	// 总抽奖数
	totalQuery := `SELECT COUNT(*) FROM lotteries WHERE guild_id = $1`
	err := r.db.Get(&stats.TotalLotteries, totalQuery, guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get total lotteries: %w", err)
	}

	// 活跃抽奖数
	activeQuery := `SELECT COUNT(*) FROM lotteries WHERE guild_id = $1 AND status = $2`
	err = r.db.Get(&stats.ActiveLotteries, activeQuery, guildID, LotteryStatusActive)
	if err != nil {
		return nil, fmt.Errorf("failed to get active lotteries count: %w", err)
	}

	// 总参与次数
	participationsQuery := `
		SELECT COUNT(*) 
		FROM lottery_participants lp 
		JOIN lotteries l ON lp.lottery_id = l.id 
		WHERE l.guild_id = $1`
	err = r.db.Get(&stats.TotalParticipations, participationsQuery, guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get total participations: %w", err)
	}

	// 总消费积分
	pointsQuery := `
		SELECT COALESCE(SUM(lp.points_spent), 0) 
		FROM lottery_participants lp 
		JOIN lotteries l ON lp.lottery_id = l.id 
		WHERE l.guild_id = $1`
	err = r.db.Get(&stats.TotalPointsSpent, pointsQuery, guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get total points spent: %w", err)
	}

	return &stats, nil
}

// GetLotteryLeaderboard 获取抽奖排行榜
func (r *LotteryRepository) GetLotteryLeaderboard(guildID string, limit int) ([]LotteryLeaderboard, error) {
	var leaderboard []LotteryLeaderboard
	query := `
		SELECT 
			ROW_NUMBER() OVER (ORDER BY win_count DESC, participation_count DESC) as rank,
			u.discord_id,
			u.username,
			stats.participation_count,
			stats.win_count,
			stats.total_points_spent,
			CASE 
				WHEN stats.participation_count > 0 
				THEN ROUND((stats.win_count::numeric / stats.participation_count::numeric * 100), 2)
				ELSE 0 
			END as win_rate
		FROM (
			SELECT 
				lp.user_id,
				COUNT(*) as participation_count,
				COUNT(CASE WHEN l.winner_id = lp.user_id THEN 1 END) as win_count,
				COALESCE(SUM(lp.points_spent), 0) as total_points_spent
			FROM lottery_participants lp
			JOIN lotteries l ON lp.lottery_id = l.id
			WHERE l.guild_id = $1
			GROUP BY lp.user_id
		) stats
		JOIN users u ON stats.user_id = u.id
		ORDER BY rank
		LIMIT $2`

	err := r.db.Select(&leaderboard, query, guildID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get lottery leaderboard: %w", err)
	}

	return leaderboard, nil
}

// DeleteExpiredLotteries 删除过期的抽奖记录（清理任务）
func (r *LotteryRepository) DeleteExpiredLotteries(beforeDate time.Time) error {
	return r.db.WithTx(func(tx *sqlx.Tx) error {
		// 先删除参与记录
		_, err := tx.Exec(`
			DELETE FROM lottery_participants 
			WHERE lottery_id IN (
				SELECT id FROM lotteries 
				WHERE status IN ($1, $2) AND end_time < $3
			)`, LotteryStatusEnded, LotteryStatusCancelled, beforeDate)
		if err != nil {
			return fmt.Errorf("failed to delete lottery participants: %w", err)
		}

		// 删除成就要求
		_, err = tx.Exec(`
			DELETE FROM lottery_achievement_requirements 
			WHERE lottery_id IN (
				SELECT id FROM lotteries 
				WHERE status IN ($1, $2) AND end_time < $3
			)`, LotteryStatusEnded, LotteryStatusCancelled, beforeDate)
		if err != nil {
			return fmt.Errorf("failed to delete achievement requirements: %w", err)
		}

		// 删除抽奖记录
		_, err = tx.Exec(`
			DELETE FROM lotteries 
			WHERE status IN ($1, $2) AND end_time < $3`,
			LotteryStatusEnded, LotteryStatusCancelled, beforeDate)
		if err != nil {
			return fmt.Errorf("failed to delete lotteries: %w", err)
		}

		return nil
	})
}

// 扩展模型

// UserLotteryHistory 用户抽奖历史
type UserLotteryHistory struct {
	LotteryID        uuid.UUID `db:"lottery_id" json:"lottery_id"`
	LotteryName      string    `db:"lottery_name" json:"lottery_name"`
	PrizeDescription string    `db:"prize_description" json:"prize_description"`
	Status           string    `db:"status" json:"status"`
	LotteryCreatedAt time.Time `db:"lottery_created_at" json:"lottery_created_at"`
	LotteryEndTime   time.Time `db:"lottery_end_time" json:"lottery_end_time"`
	ParticipatedAt   time.Time `db:"participated_at" json:"participated_at"`
	PointsSpent      int       `db:"points_spent" json:"points_spent"`
	IsWinner         bool      `db:"is_winner" json:"is_winner"`
}

// LotteryStats 抽奖统计
type LotteryStats struct {
	TotalLotteries      int `db:"total_lotteries" json:"total_lotteries"`
	ActiveLotteries     int `db:"active_lotteries" json:"active_lotteries"`
	TotalParticipations int `db:"total_participations" json:"total_participations"`
	TotalPointsSpent    int `db:"total_points_spent" json:"total_points_spent"`
}

// LotteryLeaderboard 抽奖排行榜
type LotteryLeaderboard struct {
	Rank               int     `db:"rank" json:"rank"`
	DiscordID          string  `db:"discord_id" json:"discord_id"`
	Username           string  `db:"username" json:"username"`
	ParticipationCount int     `db:"participation_count" json:"participation_count"`
	WinCount           int     `db:"win_count" json:"win_count"`
	TotalPointsSpent   int     `db:"total_points_spent" json:"total_points_spent"`
	WinRate            float64 `db:"win_rate" json:"win_rate"`
}
