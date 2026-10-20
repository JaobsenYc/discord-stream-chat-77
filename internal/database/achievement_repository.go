package database

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// AchievementRepository 成就数据访问层
type AchievementRepository struct {
	db *DB
}

// NewAchievementRepository 创建成就仓库
func NewAchievementRepository(db *DB) *AchievementRepository {
	return &AchievementRepository{db: db}
}

// ============== 成就定义管理 ==============

// GetAllAchievements 获取所有活跃成就
func (r *AchievementRepository) GetAllAchievements() ([]Achievement, error) {
	query := `
		SELECT * FROM achievements 
		WHERE is_active = true 
		ORDER BY display_order, created_at`

	var achievements []Achievement
	err := r.db.Select(&achievements, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get achievements: %w", err)
	}

	return achievements, nil
}

// GetAchievementsByType 根据类型获取成就
func (r *AchievementRepository) GetAchievementsByType(achievementType string) ([]Achievement, error) {
	query := `
		SELECT * FROM achievements 
		WHERE type = $1 AND is_active = true 
		ORDER BY display_order, created_at`

	var achievements []Achievement
	err := r.db.Select(&achievements, query, achievementType)
	if err != nil {
		return nil, fmt.Errorf("failed to get achievements by type: %w", err)
	}

	return achievements, nil
}

// GetAchievementByID 根据ID获取成就
func (r *AchievementRepository) GetAchievementByID(id uuid.UUID) (*Achievement, error) {
	var achievement Achievement
	query := `SELECT * FROM achievements WHERE id = $1`

	err := r.db.Get(&achievement, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get achievement by ID: %w", err)
	}

	return &achievement, nil
}

// ============== 用户成就进度管理 ==============

// GetUserAchievements 获取用户所有成就进度
func (r *AchievementRepository) GetUserAchievements(userID uuid.UUID) ([]AchievementWithProgress, error) {
	query := `
		SELECT 
			a.id, a.name, a.description, a.type, a.target_value, 
			a.reward_type, a.reward_value, a.icon_emoji, a.category, 
			a.display_order, a.is_hidden, a.is_active, a.created_at, a.updated_at,
			COALESCE(ua.progress, 0) as user_progress,
			COALESCE(ua.unlocked, false) as is_unlocked,
			ua.unlocked_at,
			COALESCE(ua.reward_claimed, false) as reward_claimed,
			CASE 
				WHEN a.target_value > 0 THEN ROUND((COALESCE(ua.progress, 0)::numeric / a.target_value::numeric * 100), 2)
				ELSE 0 
			END as progress_percent
		FROM achievements a
		LEFT JOIN user_achievements ua ON a.id = ua.achievement_id AND ua.user_id = $1
		WHERE a.is_active = true AND (a.is_hidden = false OR ua.id IS NOT NULL)
		ORDER BY a.display_order, a.created_at`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user achievements: %w", err)
	}
	defer rows.Close()

	var achievements []AchievementWithProgress
	for rows.Next() {
		var achievement AchievementWithProgress
		err := rows.Scan(
			&achievement.ID, &achievement.Name, &achievement.Description, &achievement.Type,
			&achievement.TargetValue, &achievement.RewardType, &achievement.RewardValue,
			&achievement.IconEmoji, &achievement.Category, &achievement.DisplayOrder,
			&achievement.IsHidden, &achievement.IsActive, &achievement.CreatedAt, &achievement.UpdatedAt,
			&achievement.UserProgress, &achievement.IsUnlocked, &achievement.UnlockedAt,
			&achievement.RewardClaimed, &achievement.ProgressPercent,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user achievement: %w", err)
		}
		achievements = append(achievements, achievement)
	}

	return achievements, nil
}

// GetUserAchievementsByCategory 根据分类获取用户成就
func (r *AchievementRepository) GetUserAchievementsByCategory(userID uuid.UUID, category string) ([]AchievementWithProgress, error) {
	query := `
		SELECT 
			a.id, a.name, a.description, a.type, a.target_value, 
			a.reward_type, a.reward_value, a.icon_emoji, a.category, 
			a.display_order, a.is_hidden, a.is_active, a.created_at, a.updated_at,
			COALESCE(ua.progress, 0) as user_progress,
			COALESCE(ua.unlocked, false) as is_unlocked,
			ua.unlocked_at,
			COALESCE(ua.reward_claimed, false) as reward_claimed,
			CASE 
				WHEN a.target_value > 0 THEN ROUND((COALESCE(ua.progress, 0)::numeric / a.target_value::numeric * 100), 2)
				ELSE 0 
			END as progress_percent
		FROM achievements a
		LEFT JOIN user_achievements ua ON a.id = ua.achievement_id AND ua.user_id = $1
		WHERE a.is_active = true AND a.category = $2 AND (a.is_hidden = false OR ua.id IS NOT NULL)
		ORDER BY a.display_order, a.created_at`

	rows, err := r.db.Query(query, userID, category)
	if err != nil {
		return nil, fmt.Errorf("failed to get user achievements by category: %w", err)
	}
	defer rows.Close()

	var achievements []AchievementWithProgress
	for rows.Next() {
		var achievement AchievementWithProgress
		err := rows.Scan(
			&achievement.ID, &achievement.Name, &achievement.Description, &achievement.Type,
			&achievement.TargetValue, &achievement.RewardType, &achievement.RewardValue,
			&achievement.IconEmoji, &achievement.Category, &achievement.DisplayOrder,
			&achievement.IsHidden, &achievement.IsActive, &achievement.CreatedAt, &achievement.UpdatedAt,
			&achievement.UserProgress, &achievement.IsUnlocked, &achievement.UnlockedAt,
			&achievement.RewardClaimed, &achievement.ProgressPercent,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user achievement: %w", err)
		}
		achievements = append(achievements, achievement)
	}

	return achievements, nil
}

// GetUserAchievementSummary 获取用户成就总览
func (r *AchievementRepository) GetUserAchievementSummary(userID uuid.UUID) (*UserAchievementSummary, error) {
	query := `
		SELECT 
			u.id as user_id,
			u.discord_id,
			u.username,
			COUNT(a.id) as total_achievements,
			COUNT(CASE WHEN ua.unlocked = true THEN 1 END) as unlocked_count,
			CASE 
				WHEN COUNT(a.id) > 0 THEN ROUND((COUNT(CASE WHEN ua.unlocked = true THEN 1 END)::numeric / COUNT(a.id)::numeric * 100), 2)
				ELSE 0 
			END as completion_rate,
			COALESCE(SUM(CASE 
				WHEN ua.unlocked = true AND a.reward_type = 'points' 
				THEN CAST(a.reward_value AS INTEGER) 
				ELSE 0 
			END), 0) as total_points,
			MAX(ua.unlocked_at) as last_unlocked
		FROM users u
		CROSS JOIN achievements a
		LEFT JOIN user_achievements ua ON u.id = ua.user_id AND a.id = ua.achievement_id
		WHERE u.id = $1 AND a.is_active = true
		GROUP BY u.id, u.discord_id, u.username`

	var summary UserAchievementSummary
	err := r.db.Get(&summary, query, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user achievement summary: %w", err)
	}

	return &summary, nil
}

// ============== 成就进度更新 ==============

// UpdateProgress 更新用户成就进度
func (r *AchievementRepository) UpdateProgress(userID uuid.UUID, achievementType string, value int, metadata map[string]interface{}) ([]Achievement, error) {
	tx, err := r.db.Beginx()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 获取该类型的所有成就
	var achievements []Achievement
	query := `SELECT * FROM achievements WHERE type = $1 AND is_active = true`
	err = tx.Select(&achievements, query, achievementType)
	if err != nil {
		return nil, fmt.Errorf("failed to get achievements: %w", err)
	}

	var newlyUnlocked []Achievement

	for _, achievement := range achievements {
		// 检查是否需要特殊逻辑处理
		shouldUpdate, newValue := r.shouldUpdateAchievement(achievement, value, metadata)
		if !shouldUpdate {
			continue
		}

		// 获取或创建用户成就记录（FOR UPDATE 行锁：并发进度更新串行化，
		// 防止两个事务同时读到 unlocked=false 而重复解锁、重复发奖）
		var userAchievement UserAchievement
		getUserQuery := `
			SELECT * FROM user_achievements
			WHERE user_id = $1 AND achievement_id = $2
			FOR UPDATE`

		err = tx.Get(&userAchievement, getUserQuery, userID, achievement.ID)
		if err == sql.ErrNoRows {
			// 创建新记录；并发首次创建时靠唯一约束去重，随后重读拿行锁
			insertQuery := `
				INSERT INTO user_achievements (id, user_id, achievement_id, progress, unlocked)
				VALUES ($1, $2, $3, 0, false)
				ON CONFLICT (user_id, achievement_id) DO NOTHING`

			_, err = tx.Exec(insertQuery, uuid.New(), userID, achievement.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to create user achievement: %w", err)
			}

			err = tx.Get(&userAchievement, getUserQuery, userID, achievement.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to get user achievement: %w", err)
			}
		} else if err != nil {
			return nil, fmt.Errorf("failed to get user achievement: %w", err)
		}

		// 如果已经解锁，跳过
		if userAchievement.Unlocked {
			continue
		}

		// 更新进度
		oldProgress := userAchievement.Progress
		userAchievement.Progress = r.calculateNewProgress(achievement, oldProgress, newValue)

		// 检查是否达成成就
		if userAchievement.Progress >= achievement.TargetValue {
			userAchievement.Unlocked = true
			now := time.Now()
			userAchievement.UnlockedAt = &now
			newlyUnlocked = append(newlyUnlocked, achievement)
		}

		// 更新数据库
		updateQuery := `
			UPDATE user_achievements 
			SET progress = $1, unlocked = $2, unlocked_at = $3, updated_at = NOW()
			WHERE id = $4`

		_, err = tx.Exec(updateQuery, userAchievement.Progress, userAchievement.Unlocked,
			userAchievement.UnlockedAt, userAchievement.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to update user achievement: %w", err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return newlyUnlocked, nil
}

// shouldUpdateAchievement 判断是否应该更新成就进度
func (r *AchievementRepository) shouldUpdateAchievement(achievement Achievement, value int, metadata map[string]interface{}) (bool, int) {
	switch achievement.Type {
	case AchievementTypeCheckIn:
		// 签到成就：累积签到次数或连续签到天数
		if achievement.Name == "连饮七日" || achievement.Name == "不离不弃" {
			// 连续签到成就，使用consecutive_days
			if consecutiveDays, ok := metadata["consecutive_days"].(int); ok {
				return true, consecutiveDays
			}
		}
		// 普通签到成就，累积次数
		return true, 1

	case AchievementTypeVoice:
		// 语音成就：累积在线时长（秒）
		return true, value

	case AchievementTypeMessage:
		// 消息成就：累积发言数
		return true, 1

	case AchievementTypeLottery:
		// 抽奖参与成就：累积参与次数
		return true, 1

	case AchievementTypeLotteryWin:
		// 抽奖中奖成就：需要特殊处理连胜逻辑
		if achievement.Name == "连胜王者" {
			// 检查是否是连续中奖
			if isWin, ok := metadata["is_win"].(bool); ok && isWin {
				return true, 1
			} else {
				// 没中奖则重置进度
				return true, 0
			}
		}
		return true, 1

	case AchievementTypeTeamCreated:
		// 组队创建成就：累积创建团队次数
		return true, 1

	case AchievementTypeActivity:
		// 活跃度成就：累积活跃天数
		return true, 1

	default:
		return false, 0
	}
}

// calculateNewProgress 计算新的进度值
func (r *AchievementRepository) calculateNewProgress(achievement Achievement, oldProgress, value int) int {
	switch achievement.Type {
	case AchievementTypeCheckIn:
		if achievement.Name == "连饮七日" || achievement.Name == "不离不弃" {
			// 连续签到成就，直接使用新值
			return value
		}
		// 累积签到成就
		return oldProgress + value

	case AchievementTypeVoice:
		// 语音时长累积
		return oldProgress + value

	case AchievementTypeMessage:
		// 消息数累积
		return oldProgress + value

	case AchievementTypeLottery:
		// 抽奖参与次数累积
		return oldProgress + value

	case AchievementTypeLotteryWin:
		if achievement.Name == "连胜王者" {
			if value == 0 {
				// 重置连胜
				return 0
			}
			// 连胜累积
			return oldProgress + value
		}
		return oldProgress + value

	case AchievementTypeTeamCreated:
		// 组队创建次数累积
		return oldProgress + value

	case AchievementTypeActivity:
		// 活跃天数累积
		return oldProgress + value

	default:
		return oldProgress + value
	}
}

// ============== 成就奖励处理 ==============

// ProcessRewards 处理成就奖励
func (r *AchievementRepository) ProcessRewards(userID uuid.UUID, guildID string, achievements []Achievement) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, achievement := range achievements {
		// 获取用户成就记录
		var userAchievement UserAchievement
		query := `
			SELECT * FROM user_achievements 
			WHERE user_id = $1 AND achievement_id = $2`

		err = tx.Get(&userAchievement, query, userID, achievement.ID)
		if err != nil {
			return fmt.Errorf("failed to get user achievement: %w", err)
		}

		// 创建奖励记录
		reward := AchievementReward{
			ID:                uuid.New(),
			UserAchievementID: userAchievement.ID,
			UserID:            userID,
			GuildID:           guildID,
			RewardType:        achievement.RewardType,
			RewardValue:       achievement.RewardValue,
			CreatedAt:         time.Now(),
		}

		// 根据奖励类型处理
		switch achievement.RewardType {
		case RewardTypePoints:
			points, err := strconv.Atoi(achievement.RewardValue)
			if err != nil {
				return fmt.Errorf("invalid points value: %w", err)
			}
			reward.PointsAwarded = points

			// 添加积分
			description := "achievement"
			err = r.addPointsInTx(tx, userID, guildID, points,
				TransactionTypeAchievementReward,
				fmt.Sprintf("成就奖励: %s", achievement.Name),
				&userAchievement.ID, &description)
			if err != nil {
				return fmt.Errorf("failed to add points: %w", err)
			}

		case RewardTypeRole:
			reward.RoleID = &achievement.RewardValue

		case RewardTypeBadge:
			reward.BadgeName = &achievement.RewardValue
		}

		// 插入奖励记录
		insertRewardQuery := `
			INSERT INTO achievement_rewards 
			(id, user_achievement_id, user_id, guild_id, reward_type, reward_value, 
			 points_awarded, role_id, badge_name, processed, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

		_, err = tx.Exec(insertRewardQuery,
			reward.ID, reward.UserAchievementID, reward.UserID, reward.GuildID,
			reward.RewardType, reward.RewardValue, reward.PointsAwarded,
			reward.RoleID, reward.BadgeName, false, reward.CreatedAt)
		if err != nil {
			return fmt.Errorf("failed to insert achievement reward: %w", err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// addPointsInTx 在事务中添加积分
func (r *AchievementRepository) addPointsInTx(tx *sqlx.Tx, userID uuid.UUID, guildID string, amount int, transactionType, description string, relatedID *uuid.UUID, relatedType *string) error {
	// 获取或创建用户积分记录
	var userPoints UserPoints
	query := `SELECT * FROM user_points WHERE user_id = $1 AND guild_id = $2`
	err := tx.Get(&userPoints, query, userID, guildID)
	if err == sql.ErrNoRows {
		// 创建新的积分记录(首次建档叠加初始积分)
		initial := initialPointsFor(userID)
		userPoints = UserPoints{
			ID:              uuid.New(),
			UserID:          userID,
			GuildID:         guildID,
			TotalPoints:     amount + initial,
			AvailablePoints: amount + initial,
			LifetimeEarned:  amount + initial,
			LifetimeSpent:   0,
			Level:           1,
			Experience:      amount,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}

		insertQuery := `
			INSERT INTO user_points (id, user_id, guild_id, total_points, available_points, 
				lifetime_earned, lifetime_spent, level, experience, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

		_, err = tx.Exec(insertQuery, userPoints.ID, userPoints.UserID, userPoints.GuildID,
			userPoints.TotalPoints, userPoints.AvailablePoints, userPoints.LifetimeEarned,
			userPoints.LifetimeSpent, userPoints.Level, userPoints.Experience,
			userPoints.CreatedAt, userPoints.UpdatedAt)
		if err != nil {
			return fmt.Errorf("failed to create user points: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to get user points: %w", err)
	} else {
		// 更新现有积分记录
		userPoints.TotalPoints += amount
		userPoints.AvailablePoints += amount
		userPoints.LifetimeEarned += amount
		userPoints.Experience += amount
		userPoints.UpdatedAt = time.Now()

		updateQuery := `
			UPDATE user_points 
			SET total_points = $1, available_points = $2, lifetime_earned = $3, 
				experience = $4, updated_at = $5
			WHERE id = $6`

		_, err = tx.Exec(updateQuery, userPoints.TotalPoints, userPoints.AvailablePoints,
			userPoints.LifetimeEarned, userPoints.Experience, userPoints.UpdatedAt, userPoints.ID)
		if err != nil {
			return fmt.Errorf("failed to update user points: %w", err)
		}
	}

	// 创建积分交易记录
	transaction := PointTransaction{
		ID:              uuid.New(),
		UserID:          userID,
		GuildID:         guildID,
		Amount:          amount,
		TransactionType: transactionType,
		Description:     &description,
		RelatedID:       relatedID,
		RelatedType:     relatedType,
		CreatedAt:       time.Now(),
	}

	insertTransactionQuery := `
		INSERT INTO point_transactions (id, user_id, guild_id, amount, transaction_type, 
			description, related_id, related_type, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err = tx.Exec(insertTransactionQuery, transaction.ID, transaction.UserID,
		transaction.GuildID, transaction.Amount, transaction.TransactionType,
		transaction.Description, transaction.RelatedID, transaction.RelatedType,
		transaction.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create point transaction: %w", err)
	}

	return nil
}

// ============== 排行榜功能 ==============

// GetAchievementLeaderboard 获取成就排行榜
func (r *AchievementRepository) GetAchievementLeaderboard(limit int) ([]AchievementLeaderboard, error) {
	query := `
		WITH user_achievement_stats AS (
			SELECT 
				u.discord_id,
				u.username,
				COUNT(CASE WHEN ua.unlocked = true THEN 1 END) as unlocked_count,
				COUNT(a.id) as total_achievements,
				CASE 
					WHEN COUNT(a.id) > 0 THEN ROUND((COUNT(CASE WHEN ua.unlocked = true THEN 1 END)::numeric / COUNT(a.id)::numeric * 100), 2)
					ELSE 0 
				END as completion_rate,
				COALESCE(SUM(CASE 
					WHEN ua.unlocked = true AND a.reward_type = 'points' 
					THEN CAST(a.reward_value AS INTEGER) 
					ELSE 0 
				END), 0) as total_points
			FROM users u
			CROSS JOIN achievements a
			LEFT JOIN user_achievements ua ON u.id = ua.user_id AND a.id = ua.achievement_id
			WHERE a.is_active = true
			GROUP BY u.id, u.discord_id, u.username
		)
		SELECT 
			ROW_NUMBER() OVER (ORDER BY unlocked_count DESC, completion_rate DESC, total_points DESC) as rank,
			discord_id,
			username,
			unlocked_count,
			completion_rate,
			total_points
		FROM user_achievement_stats
		WHERE unlocked_count > 0
		ORDER BY rank
		LIMIT $1`

	var leaderboard []AchievementLeaderboard
	err := r.db.Select(&leaderboard, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get achievement leaderboard: %w", err)
	}

	return leaderboard, nil
}

// ============== 抽奖集成功能 ==============

// CheckLotteryEligibility 检查用户是否有资格参与抽奖
func (r *AchievementRepository) CheckLotteryEligibility(userID uuid.UUID, lotteryID uuid.UUID) (bool, []string, error) {
	// 获取抽奖需要的成就
	query := `
		SELECT a.name, ua.unlocked
		FROM lottery_achievement_requirements lar
		JOIN achievements a ON lar.achievement_id = a.id
		LEFT JOIN user_achievements ua ON a.id = ua.achievement_id AND ua.user_id = $1
		WHERE lar.lottery_id = $2`

	rows, err := r.db.Query(query, userID, lotteryID)
	if err != nil {
		return false, nil, fmt.Errorf("failed to check lottery eligibility: %w", err)
	}
	defer rows.Close()

	var missingAchievements []string
	isEligible := true

	for rows.Next() {
		var achievementName string
		var unlocked sql.NullBool

		err := rows.Scan(&achievementName, &unlocked)
		if err != nil {
			return false, nil, fmt.Errorf("failed to scan achievement requirement: %w", err)
		}

		if !unlocked.Valid || !unlocked.Bool {
			isEligible = false
			missingAchievements = append(missingAchievements, achievementName)
		}
	}

	return isEligible, missingAchievements, nil
}

// GetUnprocessedRewards 获取未处理的奖励
func (r *AchievementRepository) GetUnprocessedRewards(guildID string) ([]AchievementReward, error) {
	query := `SELECT * FROM achievement_rewards WHERE guild_id = $1 AND processed = false ORDER BY created_at`

	var rewards []AchievementReward
	err := r.db.Select(&rewards, query, guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get unprocessed rewards: %w", err)
	}

	return rewards, nil
}

// MarkRewardProcessed 标记奖励为已处理
func (r *AchievementRepository) MarkRewardProcessed(rewardID uuid.UUID, success bool, errorMessage string) error {
	var query string
	var args []interface{}

	if success {
		query = `UPDATE achievement_rewards SET processed = true, processed_at = NOW() WHERE id = $1`
		args = []interface{}{rewardID}
	} else {
		query = `UPDATE achievement_rewards SET processed = false, error_message = $2 WHERE id = $1`
		args = []interface{}{rewardID, errorMessage}
	}

	_, err := r.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to mark reward processed: %w", err)
	}

	return nil
}
