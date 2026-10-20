package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// UserRepository 用户数据访问层
type UserRepository struct {
	db *DB
}

// NewUserRepository 创建用户仓库
func NewUserRepository(db *DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create 创建新用户
func (r *UserRepository) Create(user *User) error {
	query := `
		INSERT INTO users (discord_id, username, discriminator, avatar_url, timezone, language, preferences)
		VALUES (:discord_id, :username, :discriminator, :avatar_url, :timezone, :language, :preferences)
		RETURNING id, created_at, updated_at`

	rows, err := r.db.NamedQuery(query, user)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	defer rows.Close()

	if rows.Next() {
		err = rows.Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
		if err != nil {
			return fmt.Errorf("failed to scan user: %w", err)
		}
	}

	return nil
}

// GetByID 根据 ID 获取用户
func (r *UserRepository) GetByID(id uuid.UUID) (*User, error) {
	// 先尝试从缓存获取
	cacheKey := fmt.Sprintf("user:id:%s", id)
	cached, err := r.db.GetCache(cacheKey)
	if err == nil && cached != "" {
		var user User
		if err := json.Unmarshal([]byte(cached), &user); err == nil {
			return &user, nil
		}
	}

	// 从数据库获取
	var user User
	query := `SELECT * FROM users WHERE id = $1`
	err = r.db.Get(&user, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}

	// 缓存结果
	if data, err := json.Marshal(&user); err == nil {
		_ = r.db.SetCacheWithTTL(cacheKey, data, 5*time.Minute)
	}

	return &user, nil
}

// GetByDiscordID 根据 Discord ID 获取用户
func (r *UserRepository) GetByDiscordID(discordID string) (*User, error) {
	// 先尝试从缓存获取
	cacheKey := fmt.Sprintf("user:discord:%s", discordID)
	cached, err := r.db.GetCache(cacheKey)
	if err == nil && cached != "" {
		var user User
		if err := json.Unmarshal([]byte(cached), &user); err == nil {
			return &user, nil
		}
	}

	// 从数据库获取
	var user User
	query := `SELECT * FROM users WHERE discord_id = $1`
	err = r.db.Get(&user, query, discordID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by discord id: %w", err)
	}

	// 缓存结果
	if data, err := json.Marshal(&user); err == nil {
		_ = r.db.SetCacheWithTTL(cacheKey, data, 5*time.Minute)
	}

	return &user, nil
}

// GetOrCreate 获取或创建用户
func (r *UserRepository) GetOrCreate(discordID, username string, discriminator, avatarURL *string) (*User, error) {
	// 先尝试获取
	user, err := r.GetByDiscordID(discordID)
	if err != nil {
		return nil, err
	}

	if user != nil {
		// 更新用户信息
		needUpdate := false
		if user.Username != username {
			user.Username = username
			needUpdate = true
		}
		if discriminator != nil && (user.Discriminator == nil || *user.Discriminator != *discriminator) {
			user.Discriminator = discriminator
			needUpdate = true
		}
		if avatarURL != nil && (user.AvatarURL == nil || *user.AvatarURL != *avatarURL) {
			user.AvatarURL = avatarURL
			needUpdate = true
		}

		if needUpdate {
			if err := r.Update(user); err != nil {
				return nil, err
			}
		}

		return user, nil
	}

	// 创建新用户
	user = &User{
		DiscordID:     discordID,
		Username:      username,
		Discriminator: discriminator,
		AvatarURL:     avatarURL,
		Timezone:      "UTC",
		Language:      "zh-CN",
		Preferences:   make(UserPreferences),
	}

	if err := r.Create(user); err != nil {
		// 并发创建同一用户时后到的一方会撞 discord_id 唯一约束，此时改为读取已存在的记录
		if existing, getErr := r.GetByDiscordID(discordID); getErr == nil && existing != nil {
			return existing, nil
		}
		return nil, err
	}

	return user, nil
}

// Update 更新用户
func (r *UserRepository) Update(user *User) error {
	query := `
		UPDATE users 
		SET username = :username,
		    discriminator = :discriminator,
		    avatar_url = :avatar_url,
		    timezone = :timezone,
		    language = :language,
		    preferences = :preferences,
		    total_teams_created = :total_teams_created,
		    total_teams_joined = :total_teams_joined,
		    total_check_ins = :total_check_ins
		WHERE id = :id`

	_, err := r.db.NamedExec(query, user)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	// 清除缓存
	_ = r.db.DeleteCache(
		fmt.Sprintf("user:id:%s", user.ID),
		fmt.Sprintf("user:discord:%s", user.DiscordID),
	)

	return nil
}

// IncrementTeamsCreated 增加创建团队数
func (r *UserRepository) IncrementTeamsCreated(userID uuid.UUID) error {
	return r.db.WithTx(func(tx *sqlx.Tx) error {
		// 更新用户团队创建计数
		updateQuery := `
			UPDATE users 
			SET total_teams_created = total_teams_created + 1 
			WHERE id = $1
			RETURNING total_teams_created`

		var newCount int
		err := tx.QueryRow(updateQuery, userID).Scan(&newCount)
		if err != nil {
			return fmt.Errorf("failed to increment teams created: %w", err)
		}

		// 清除缓存
		_ = r.db.DeleteCache(fmt.Sprintf("user:id:%s", userID))

		return nil
	})
}

// IncrementTeamsJoined 增加加入团队数
func (r *UserRepository) IncrementTeamsJoined(userID uuid.UUID) error {
	query := `UPDATE users SET total_teams_joined = total_teams_joined + 1 WHERE id = $1`
	_, err := r.db.Exec(query, userID)
	if err != nil {
		return fmt.Errorf("failed to increment teams joined: %w", err)
	}

	// 清除缓存
	_ = r.db.DeleteCache(fmt.Sprintf("user:id:%s", userID))

	return nil
}

// GetUserPoints 获取用户积分
func (r *UserRepository) GetUserPoints(userID uuid.UUID, guildID string) (*UserPoints, error) {
	var points UserPoints
	query := `SELECT * FROM user_points WHERE user_id = $1 AND guild_id = $2`
	err := r.db.Get(&points, query, userID, guildID)
	if err != nil {
		if err == sql.ErrNoRows {
			// 创建新的积分记录并发放初始积分(系统账户为 0,由对冲逻辑按 SystemInitialPool 初始化)
			initialPoints := initialPointsFor(userID)
			points = UserPoints{
				ID:              uuid.New(),
				UserID:          userID,
				GuildID:         guildID,
				Level:           1,
				TotalPoints:     initialPoints,
				AvailablePoints: initialPoints,
				LifetimeEarned:  initialPoints,
			}

			insertQuery := `
				INSERT INTO user_points (id, user_id, guild_id, level, total_points, available_points, lifetime_earned)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
				RETURNING created_at, updated_at`

			err = r.db.QueryRow(insertQuery, points.ID, points.UserID, points.GuildID, points.Level,
				points.TotalPoints, points.AvailablePoints, points.LifetimeEarned).
				Scan(&points.CreatedAt, &points.UpdatedAt)
			if err != nil {
				return nil, fmt.Errorf("failed to create user points: %w", err)
			}

			return &points, nil
		}
		return nil, fmt.Errorf("failed to get user points: %w", err)
	}

	return &points, nil
}

// AddPoints 添加积分
func (r *UserRepository) AddPoints(userID uuid.UUID, guildID string, amount int, transactionType, description string, relatedID *uuid.UUID, relatedType *string) error {
	return r.db.WithTx(func(tx *sqlx.Tx) error {
		// 更新用户积分（用户无积分记录时自动创建并叠加初始积分，避免 UPDATE 影响 0 行导致积分静默丢失）
		updateQuery := `
			INSERT INTO user_points (id, user_id, guild_id, total_points, available_points, lifetime_earned, level)
			VALUES (uuid_generate_v4(), $1, $2, $3 + $4, $3 + $4, $3 + $4, 1)
			ON CONFLICT (user_id, guild_id)
			DO UPDATE SET
				total_points = user_points.total_points + $3,
				available_points = user_points.available_points + $3,
				lifetime_earned = user_points.lifetime_earned + $3`

		_, err := tx.Exec(updateQuery, userID, guildID, amount, initialPointsFor(userID))
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
	})
}

// SpendPoints 消费积分
func (r *UserRepository) SpendPoints(userID uuid.UUID, guildID string, amount int, transactionType, description string, relatedID *uuid.UUID, relatedType *string) error {
	return r.db.WithTx(func(tx *sqlx.Tx) error {
		// 检查积分是否足够
		var availablePoints int
		checkQuery := `SELECT available_points FROM user_points WHERE user_id = $1 AND guild_id = $2`
		err := tx.Get(&availablePoints, checkQuery, userID, guildID)
		if err != nil {
			return fmt.Errorf("failed to check points: %w", err)
		}

		if availablePoints < amount {
			return fmt.Errorf("insufficient points: have %d, need %d", availablePoints, amount)
		}

		// 更新用户积分（条件更新：并发扣减时余额不足的一方影响 0 行，防止余额为负）
		updateQuery := `
			UPDATE user_points
			SET available_points = available_points - $3,
			    lifetime_spent = lifetime_spent + $3
			WHERE user_id = $1 AND guild_id = $2 AND available_points >= $3`

		result, err := tx.Exec(updateQuery, userID, guildID, amount)
		if err != nil {
			return fmt.Errorf("failed to update points: %w", err)
		}
		if rows, err := result.RowsAffected(); err == nil && rows == 0 {
			return fmt.Errorf("insufficient points: have %d, need %d", availablePoints, amount)
		}

		// 记录交易（负数表示消费）
		transactionQuery := `
			INSERT INTO point_transactions (user_id, guild_id, amount, transaction_type, description, related_id, related_type)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`

		_, err = tx.Exec(transactionQuery, userID, guildID, -amount, transactionType, description, relatedID, relatedType)
		if err != nil {
			return fmt.Errorf("failed to create transaction: %w", err)
		}

		return nil
	})
}

// GetLeaderboard 获取积分排行榜
func (r *UserRepository) GetLeaderboard(guildID string, limit int) ([]UserLeaderboard, error) {
	query := `
		SELECT
			u.discord_id,
			u.username,
			up.total_points,
			up.available_points,
			up.level,
			up.experience,
			RANK() OVER (ORDER BY up.available_points DESC) as rank
		FROM users u
		JOIN user_points up ON u.id = up.user_id
		WHERE up.guild_id = $1 AND u.discord_id != 'SYSTEM'
		ORDER BY up.available_points DESC
		LIMIT $2`

	var leaderboard []UserLeaderboard
	err := r.db.Select(&leaderboard, query, guildID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get leaderboard: %w", err)
	}

	return leaderboard, nil
}
