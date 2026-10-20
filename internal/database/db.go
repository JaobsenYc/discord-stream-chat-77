package database

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver
)

// DB 封装数据库连接
type DB struct {
	*sqlx.DB
	Redis *redis.Client
	ctx   context.Context
}

// Config 数据库配置
type Config struct {
	PostgresURL     string
	RedisURL        string
	RedisPassword   string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	}
}

// New 创建新的数据库连接
func New(cfg *Config) (*DB, error) {
	// 连接 PostgreSQL
	db, err := sqlx.Connect("postgres", cfg.PostgresURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	// 设置连接池参数
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	// 解析 Redis 配置
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis url: %w", err)
	}

	if cfg.RedisPassword != "" {
		opt.Password = cfg.RedisPassword
	}

	// 连接 Redis
	rdb := redis.NewClient(opt)
	ctx := context.Background()

	// 测试 Redis 连接
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &DB{
		DB:    db,
		Redis: rdb,
		ctx:   ctx,
	}, nil
}

// Close 关闭数据库连接
func (db *DB) Close() error {
	if err := db.Redis.Close(); err != nil {
		return fmt.Errorf("failed to close redis: %w", err)
	}

	if err := db.DB.Close(); err != nil {
		return fmt.Errorf("failed to close postgres: %w", err)
	}

	return nil
}

// Ping 测试数据库连接
func (db *DB) Ping() error {
	if err := db.DB.Ping(); err != nil {
		return fmt.Errorf("postgres ping failed: %w", err)
	}

	if err := db.Redis.Ping(db.ctx).Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}

	return nil
}

// WithTx 在事务中执行函数
func (db *DB) WithTx(fn func(*sqlx.Tx) error) error {
	tx, err := db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx failed: %v, rb failed: %v", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Context 返回数据库的 context
func (db *DB) Context() context.Context {
	return db.ctx
}

// SetCacheWithTTL 设置缓存（带过期时间）
func (db *DB) SetCacheWithTTL(key string, value interface{}, ttl time.Duration) error {
	return db.Redis.Set(db.ctx, key, value, ttl).Err()
}

// GetCache 获取缓存
func (db *DB) GetCache(key string) (string, error) {
	return db.Redis.Get(db.ctx, key).Result()
}

// DeleteCache 删除缓存
func (db *DB) DeleteCache(keys ...string) error {
	return db.Redis.Del(db.ctx, keys...).Err()
}

// DeleteCachePattern 按通配符模式删除缓存（DEL 不支持通配符，需先 SCAN 匹配）
func (db *DB) DeleteCachePattern(pattern string) error {
	iter := db.Redis.Scan(db.ctx, 0, pattern, 100).Iterator()
	var keys []string
	for iter.Next(db.ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	return db.Redis.Del(db.ctx, keys...).Err()
}

// CacheExists 检查缓存是否存在
func (db *DB) CacheExists(key string) (bool, error) {
	n, err := db.Redis.Exists(db.ctx, key).Result()
	return n > 0, err
}
