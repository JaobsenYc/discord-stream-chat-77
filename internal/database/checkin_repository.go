package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// CheckInRepository 签到数据访问层
type CheckInRepository struct {
	db *DB
}

// NewCheckInRepository 创建签到仓库
func NewCheckInRepository(db *DB) *CheckInRepository {
	return &CheckInRepository{db: db}
}

// CheckInResult 签到结果
type CheckInResult struct {
	Success         bool   `json:"success"`
	Message         string `json:"message"`
	PointsEarned    int    `json:"points_earned"`
	ConsecutiveDays int    `json:"consecutive_days"`
	TotalCheckIns   int    `json:"total_check_ins"`
	TotalPoints     int    `json:"total_points"`
	IsFirstTime     bool   `json:"is_first_time"`
}

// CheckInStatus 签到状态
type CheckInStatus struct {
	HasCheckedInToday  bool       `json:"has_checked_in_today"`
	LastCheckInDate    *time.Time `json:"last_check_in_date,omitempty"`
	ConsecutiveDays    int        `json:"consecutive_days"`
	TotalCheckIns      int        `json:"total_check_ins"`
	TotalPoints        int        `json:"total_points"`
	MaxConsecutiveDays int        `json:"max_consecutive_days"`
}

// CheckInLeaderboard 签到排行榜项
type CheckInLeaderboard struct {
	Rank               int    `db:"rank" json:"rank"`
	DiscordID          string `db:"discord_id" json:"discord_id"`
	Username           string `db:"username" json:"username"`
	TotalCheckIns      int    `db:"total_check_ins" json:"total_check_ins"`
	ConsecutiveDays    int    `db:"consecutive_days" json:"consecutive_days"`
	MaxConsecutiveDays int    `db:"max_consecutive_days" json:"max_consecutive_days"`
	TotalPoints        int    `db:"total_points" json:"total_points"`
}

// CheckIn 用户签到
func (r *CheckInRepository) CheckIn(userID uuid.UUID, discordID, guildID, channelID string) (*CheckIn, error) {
	// 获取今天的日期
	today := time.Now().UTC().Truncate(24 * time.Hour)

	// 检查今天是否已经签到
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM check_ins WHERE user_id = $1 AND guild_id = $2 AND check_in_date = $3)`
	err := r.db.Get(&exists, checkQuery, userID, guildID, today)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing check-in: %w", err)
	}

	if exists {
		return nil, fmt.Errorf("already checked in today")
	}

	// 获取昨天的签到记录，计算连续天数
	yesterday := today.AddDate(0, 0, -1)
	var consecutiveDays int
	var lastCheckIn CheckIn

	lastQuery := `SELECT * FROM check_ins WHERE user_id = $1 AND guild_id = $2 AND check_in_date = $3`
	err = r.db.Get(&lastCheckIn, lastQuery, userID, guildID, yesterday)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get last check-in: %w", err)
	}

	if err == nil {
		// 昨天有签到，连续天数+1
		consecutiveDays = lastCheckIn.ConsecutiveDays + 1
	} else {
		// 昨天没有签到，连续天数重置为1
		consecutiveDays = 1
	}

	// 计算积分奖励
	pointsEarned := r.calculatePoints(consecutiveDays, guildID)

	// 创建签到记录
	checkIn := &CheckIn{
		ID:              uuid.New(),
		UserID:          userID,
		DiscordID:       discordID,
		GuildID:         guildID,
		ChannelID:       channelID,
		CheckInDate:     today,
		CheckInTime:     time.Now().UTC(),
		PointsEarned:    pointsEarned,
		ConsecutiveDays: consecutiveDays,
	}

	insertQuery := `
		INSERT INTO check_ins (id, user_id, discord_id, guild_id, channel_id, check_in_date, check_in_time, points_earned, consecutive_days)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at`

	err = r.db.QueryRow(
		insertQuery,
		checkIn.ID,
		checkIn.UserID,
		checkIn.DiscordID,
		checkIn.GuildID,
		checkIn.ChannelID,
		checkIn.CheckInDate,
		checkIn.CheckInTime,
		checkIn.PointsEarned,
		checkIn.ConsecutiveDays,
	).Scan(&checkIn.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create check-in: %w", err)
	}

	// 更新用户统计
	updateUserQuery := `UPDATE users SET total_check_ins = total_check_ins + 1 WHERE id = $1`
	_, err = r.db.Exec(updateUserQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to update user stats: %w", err)
	}

	return checkIn, nil
}

// calculatePoints 计算签到积分
// 基础 100 积分，每连续签到一天额外 +10 积分，最高 200 积分
func (r *CheckInRepository) calculatePoints(consecutiveDays int, guildID string) int {
	basePoints := 100 // 基础积分
	bonusPerDay := 10 // 每天额外奖励
	maxPoints := 200  // 最高积分上限

	points := basePoints

	// 连续签到奖励：从第2天开始，每天+10
	if consecutiveDays > 1 {
		bonus := (consecutiveDays - 1) * bonusPerDay
		points += bonus
	}

	// 上限检查
	if points > maxPoints {
		points = maxPoints
	}

	return points
}

// GetUserCheckIns 获取用户签到记录
func (r *CheckInRepository) GetUserCheckIns(userID uuid.UUID, guildID string, limit int) ([]CheckIn, error) {
	query := `
		SELECT * FROM check_ins 
		WHERE user_id = $1 AND guild_id = $2 
		ORDER BY check_in_date DESC 
		LIMIT $3`

	var checkIns []CheckIn
	err := r.db.Select(&checkIns, query, userID, guildID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get user check-ins: %w", err)
	}

	return checkIns, nil
}

// GetTodayCheckIn 获取今天的签到记录
func (r *CheckInRepository) GetTodayCheckIn(userID uuid.UUID, guildID string) (*CheckIn, error) {
	today := time.Now().UTC().Truncate(24 * time.Hour)

	var checkIn CheckIn
	query := `SELECT * FROM check_ins WHERE user_id = $1 AND guild_id = $2 AND check_in_date = $3`
	err := r.db.Get(&checkIn, query, userID, guildID, today)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get today's check-in: %w", err)
	}

	return &checkIn, nil
}

// GetCheckInStats 获取签到统计
func (r *CheckInRepository) GetCheckInStats(discordID string) (*UserCheckInStats, error) {
	var stats UserCheckInStats
	query := `SELECT * FROM user_check_in_stats WHERE discord_id = $1`
	err := r.db.Get(&stats, query, discordID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get check-in stats: %w", err)
	}

	return &stats, nil
}

// GetGuildCheckInLeaderboard 获取服务器签到排行榜
func (r *CheckInRepository) GetGuildCheckInLeaderboard(guildID string, limit int) ([]UserCheckInStats, error) {
	query := `
		SELECT u.discord_id, u.username, 
		       COUNT(c.id) as total_check_ins,
		       COALESCE(MAX(c.consecutive_days), 0) as max_consecutive_days,
		       MAX(c.check_in_date) as last_check_in_date,
		       COALESCE(SUM(c.points_earned), 0) as total_points_from_check_ins
		FROM users u
		LEFT JOIN check_ins c ON u.id = c.user_id AND c.guild_id = $1
		WHERE EXISTS (SELECT 1 FROM check_ins WHERE user_id = u.id AND guild_id = $1)
		GROUP BY u.id, u.discord_id, u.username
		ORDER BY total_check_ins DESC, max_consecutive_days DESC
		LIMIT $2`

	var leaderboard []UserCheckInStats
	err := r.db.Select(&leaderboard, query, guildID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get check-in leaderboard: %w", err)
	}

	return leaderboard, nil
}

// GetMonthlyCheckInStats 获取月度签到统计
func (r *CheckInRepository) GetMonthlyCheckInStats(userID uuid.UUID, guildID string, year, month int) (map[int]bool, error) {
	startDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, 0).Add(-time.Second)

	query := `
		SELECT EXTRACT(DAY FROM check_in_date)::int as day
		FROM check_ins
		WHERE user_id = $1 AND guild_id = $2 
		  AND check_in_date >= $3 AND check_in_date <= $4`

	rows, err := r.db.Query(query, userID, guildID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get monthly check-ins: %w", err)
	}
	defer rows.Close()

	checkIns := make(map[int]bool)
	for rows.Next() {
		var day int
		if err := rows.Scan(&day); err != nil {
			return nil, fmt.Errorf("failed to scan day: %w", err)
		}
		checkIns[day] = true
	}

	return checkIns, nil
}

// PerformCheckIn 执行签到
func (r *CheckInRepository) PerformCheckIn(userID uuid.UUID, discordID, guildID, channelID string, basePoints, bonusPoints int) (*CheckInResult, error) {
	var result *CheckInResult
	err := r.db.WithTx(func(tx *sqlx.Tx) error {
		var txErr error
		result, txErr = r.performCheckInInTx(tx, userID, discordID, guildID, channelID, basePoints, bonusPoints)
		return txErr
	})
	return result, err
}

// performCheckInInTx 在事务中执行签到
func (r *CheckInRepository) performCheckInInTx(tx *sqlx.Tx, userID uuid.UUID, discordID, guildID, channelID string, basePoints, bonusPoints int) (*CheckInResult, error) {
	// 获取今天的日期
	today := time.Now().UTC().Truncate(24 * time.Hour)

	// 检查今天是否已经签到
	var existingCheckIn CheckIn
	checkQuery := `SELECT id FROM check_ins WHERE user_id = $1 AND guild_id = $2 AND check_in_date = $3`
	err := tx.Get(&existingCheckIn, checkQuery, userID, guildID, today)
	if err == nil {
		// 已经签到过了
		return &CheckInResult{
			Success: false,
			Message: "今天已经签到过了！",
		}, nil
	} else if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to check existing check-in: %w", err)
	}

	// 获取用户最近的签到记录来计算连续天数
	var lastCheckIn CheckIn
	lastQuery := `SELECT * FROM check_ins WHERE user_id = $1 AND guild_id = $2 ORDER BY check_in_date DESC LIMIT 1`
	err = tx.Get(&lastCheckIn, lastQuery, userID, guildID)

	consecutiveDays := 1
	if err == nil {
		// 检查是否是连续签到
		yesterday := today.Add(-24 * time.Hour)
		if lastCheckIn.CheckInDate.Equal(yesterday) {
			consecutiveDays = lastCheckIn.ConsecutiveDays + 1
		}
	} else if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get last check-in: %w", err)
	}

	// 计算积分奖励：基础分 + 连续天数奖励，封顶200分
	pointsEarned := basePoints
	if consecutiveDays > 1 {
		pointsEarned += bonusPoints * (consecutiveDays - 1)
	}
	// 积分上限200
	if pointsEarned > 200 {
		pointsEarned = 200
	}

	// 创建签到记录
	checkInID := uuid.New()
	insertQuery := `
		INSERT INTO check_ins (id, user_id, discord_id, guild_id, channel_id, check_in_date, points_earned, consecutive_days)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err = tx.Exec(insertQuery, checkInID, userID, discordID, guildID, channelID, today, pointsEarned, consecutiveDays)
	if err != nil {
		return nil, fmt.Errorf("failed to create check-in record: %w", err)
	}

	// 更新用户总签到次数
	updateUserQuery := `UPDATE users SET total_check_ins = total_check_ins + 1 WHERE id = $1`
	_, err = tx.Exec(updateUserQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to update user check-in count: %w", err)
	}

	// 添加积分
	err = r.addPointsInTx(tx, userID, guildID, pointsEarned, "check_in", "每日签到奖励", &checkInID, stringPtr("check_in"))
	if err != nil {
		return nil, fmt.Errorf("failed to add points: %w", err)
	}

	// 获取用户当前总积分和总签到次数
	var totalPoints, totalCheckIns int
	pointsQuery := `SELECT COALESCE(total_points, 0) FROM user_points WHERE user_id = $1 AND guild_id = $2`
	err = tx.Get(&totalPoints, pointsQuery, userID, guildID)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get total points: %w", err)
	}

	checkInsQuery := `SELECT total_check_ins FROM users WHERE id = $1`
	err = tx.Get(&totalCheckIns, checkInsQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get total check-ins: %w", err)
	}

	return &CheckInResult{
		Success:         true,
		Message:         "签到成功！",
		PointsEarned:    pointsEarned,
		ConsecutiveDays: consecutiveDays,
		TotalCheckIns:   totalCheckIns,
		TotalPoints:     totalPoints + pointsEarned,
		IsFirstTime:     totalCheckIns == 1,
	}, nil
}

// GetCheckInStatus 获取签到状态
func (r *CheckInRepository) GetCheckInStatus(userID uuid.UUID, guildID string) (*CheckInStatus, error) {
	today := time.Now().UTC().Truncate(24 * time.Hour)

	// 检查今天是否已签到
	var todayCheckIn CheckIn
	todayQuery := `SELECT * FROM check_ins WHERE user_id = $1 AND guild_id = $2 AND check_in_date = $3`
	err := r.db.Get(&todayCheckIn, todayQuery, userID, guildID, today)
	hasCheckedInToday := err == nil

	// 获取最近的签到记录
	var lastCheckIn CheckIn
	var lastCheckInDate *time.Time
	var consecutiveDays int
	lastQuery := `SELECT * FROM check_ins WHERE user_id = $1 AND guild_id = $2 ORDER BY check_in_date DESC LIMIT 1`
	err = r.db.Get(&lastCheckIn, lastQuery, userID, guildID)
	if err == nil {
		lastCheckInDate = &lastCheckIn.CheckInTime
		consecutiveDays = lastCheckIn.ConsecutiveDays
	}

	// 获取总签到次数
	var totalCheckIns int
	totalQuery := `SELECT total_check_ins FROM users WHERE id = $1`
	err = r.db.Get(&totalCheckIns, totalQuery, userID)
	if err != nil {
		totalCheckIns = 0
	}

	// 获取最大连续签到天数
	var maxConsecutiveDays int
	maxQuery := `SELECT COALESCE(MAX(consecutive_days), 0) FROM check_ins WHERE user_id = $1 AND guild_id = $2`
	err = r.db.Get(&maxConsecutiveDays, maxQuery, userID, guildID)
	if err != nil {
		maxConsecutiveDays = 0
	}

	// 获取总积分
	var totalPoints int
	pointsQuery := `SELECT COALESCE(total_points, 0) FROM user_points WHERE user_id = $1 AND guild_id = $2`
	err = r.db.Get(&totalPoints, pointsQuery, userID, guildID)
	if err != nil {
		totalPoints = 0
	}

	return &CheckInStatus{
		HasCheckedInToday:  hasCheckedInToday,
		LastCheckInDate:    lastCheckInDate,
		ConsecutiveDays:    consecutiveDays,
		TotalCheckIns:      totalCheckIns,
		TotalPoints:        totalPoints,
		MaxConsecutiveDays: maxConsecutiveDays,
	}, nil
}

// GetCheckInLeaderboard 获取签到排行榜
func (r *CheckInRepository) GetCheckInLeaderboard(guildID string, limit int) ([]CheckInLeaderboard, error) {
	query := `
		SELECT 
			ROW_NUMBER() OVER (ORDER BY u.total_check_ins DESC, COALESCE(MAX(c.consecutive_days), 0) DESC) as rank,
			u.discord_id as discord_id,
			u.username as username,
			u.total_check_ins as total_check_ins,
			COALESCE(MAX(c.consecutive_days), 0) as consecutive_days,
			COALESCE(MAX(c.consecutive_days), 0) as max_consecutive_days,
			COALESCE(up.total_points, 0) as total_points
		FROM users u
		LEFT JOIN check_ins c ON u.id = c.user_id AND c.guild_id = $1
		LEFT JOIN user_points up ON u.id = up.user_id AND up.guild_id = $1
		WHERE u.total_check_ins > 0
		GROUP BY u.id, u.discord_id, u.username, u.total_check_ins, up.total_points
		ORDER BY u.total_check_ins DESC, consecutive_days DESC
		LIMIT $2`

	var leaderboard []CheckInLeaderboard
	err := r.db.Select(&leaderboard, query, guildID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get check-in leaderboard: %w", err)
	}

	return leaderboard, nil
}

// addPointsInTx 在事务中添加积分
func (r *CheckInRepository) addPointsInTx(tx *sqlx.Tx, userID uuid.UUID, guildID string, amount int, transactionType, description string, relatedID *uuid.UUID, relatedType *string) error {
	// 确保用户积分记录存在(首次建档叠加初始积分)
	upsertQuery := `
		INSERT INTO user_points (id, user_id, guild_id, total_points, available_points, lifetime_earned, level)
		VALUES (uuid_generate_v4(), $1, $2, $3 + $4, $3 + $4, $3 + $4, 1)
		ON CONFLICT (user_id, guild_id)
		DO UPDATE SET
			total_points = user_points.total_points + $3,
			available_points = user_points.available_points + $3,
			lifetime_earned = user_points.lifetime_earned + $3`

	_, err := tx.Exec(upsertQuery, userID, guildID, amount, initialPointsFor(userID))
	if err != nil {
		return fmt.Errorf("failed to update points: %w", err)
	}

	// 记录交易
	transactionQuery := `
		INSERT INTO point_transactions (user_id, guild_id, amount, transaction_type, description, related_id, related_type)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err = tx.Exec(transactionQuery, userID, guildID, amount, transactionType, description, relatedID, relatedType)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	return nil
}

// stringPtr 返回字符串指针
func stringPtr(s string) *string {
	return &s
}
