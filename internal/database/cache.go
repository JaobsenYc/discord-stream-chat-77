package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// CacheManager Redis 缓存管理器
type CacheManager struct {
	client *redis.Client
	ctx    context.Context
}

// NewCacheManager 创建缓存管理器
func NewCacheManager(client *redis.Client) *CacheManager {
	return &CacheManager{
		client: client,
		ctx:    context.Background(),
	}
}

// 缓存键前缀
const (
	PrefixUser        = "user:"
	PrefixTeam        = "team:"
	PrefixCheckIn     = "checkin:"
	PrefixRateLimit   = "ratelimit:"
	PrefixSession     = "session:"
	PrefixLeaderboard = "leaderboard:"
	PrefixActiveTeams = "activeteams:"
)

// 默认过期时间
const (
	TTLUser        = 5 * time.Minute
	TTLTeam        = 5 * time.Minute
	TTLCheckIn     = 10 * time.Minute
	TTLLeaderboard = 1 * time.Minute
	TTLSession     = 24 * time.Hour
)

// SetJSON 设置 JSON 格式的缓存
func (c *CacheManager) SetJSON(key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	return c.client.Set(c.ctx, key, data, ttl).Err()
}

// GetJSON 获取 JSON 格式的缓存
func (c *CacheManager) GetJSON(key string, dest interface{}) error {
	data, err := c.client.Get(c.ctx, key).Result()
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(data), dest)
}

// Delete 删除缓存
func (c *CacheManager) Delete(keys ...string) error {
	return c.client.Del(c.ctx, keys...).Err()
}

// Exists 检查缓存是否存在
func (c *CacheManager) Exists(key string) (bool, error) {
	n, err := c.client.Exists(c.ctx, key).Result()
	return n > 0, err
}

// SetWithLock 带锁的设置操作
func (c *CacheManager) SetWithLock(key string, value interface{}, ttl time.Duration, lockKey string, lockTTL time.Duration) error {
	// 获取锁
	locked, err := c.AcquireLock(lockKey, lockTTL)
	if err != nil || !locked {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer c.ReleaseLock(lockKey)

	// 设置值
	return c.SetJSON(key, value, ttl)
}

// AcquireLock 获取分布式锁
func (c *CacheManager) AcquireLock(key string, ttl time.Duration) (bool, error) {
	result, err := c.client.SetNX(c.ctx, key, "1", ttl).Result()
	return result, err
}

// ReleaseLock 释放分布式锁
func (c *CacheManager) ReleaseLock(key string) error {
	return c.client.Del(c.ctx, key).Err()
}

// IncrementCounter 增加计数器
func (c *CacheManager) IncrementCounter(key string) (int64, error) {
	return c.client.Incr(c.ctx, key).Result()
}

// GetCounter 获取计数器值
func (c *CacheManager) GetCounter(key string) (int64, error) {
	val, err := c.client.Get(c.ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

// SetCounter 设置计数器
func (c *CacheManager) SetCounter(key string, value int64, ttl time.Duration) error {
	return c.client.Set(c.ctx, key, value, ttl).Err()
}

// RateLimitCheck 速率限制检查
func (c *CacheManager) RateLimitCheck(identifier string, limit int64, window time.Duration) (bool, error) {
	key := fmt.Sprintf("%s%s:%d", PrefixRateLimit, identifier, time.Now().Unix()/int64(window.Seconds()))

	// 增加计数
	count, err := c.IncrementCounter(key)
	if err != nil {
		return false, err
	}

	// 设置过期时间
	if count == 1 {
		c.client.Expire(c.ctx, key, window)
	}

	return count <= limit, nil
}

// GetRateLimitInfo 获取速率限制信息
func (c *CacheManager) GetRateLimitInfo(identifier string, window time.Duration) (current int64, ttl time.Duration, err error) {
	key := fmt.Sprintf("%s%s:%d", PrefixRateLimit, identifier, time.Now().Unix()/int64(window.Seconds()))

	current, err = c.GetCounter(key)
	if err != nil {
		return 0, 0, err
	}

	ttl, err = c.client.TTL(c.ctx, key).Result()
	return current, ttl, err
}

// SetSession 设置会话
func (c *CacheManager) SetSession(sessionID string, data interface{}, ttl time.Duration) error {
	key := PrefixSession + sessionID
	return c.SetJSON(key, data, ttl)
}

// GetSession 获取会话
func (c *CacheManager) GetSession(sessionID string, dest interface{}) error {
	key := PrefixSession + sessionID
	return c.GetJSON(key, dest)
}

// DeleteSession 删除会话
func (c *CacheManager) DeleteSession(sessionID string) error {
	key := PrefixSession + sessionID
	return c.Delete(key)
}

// RefreshSession 刷新会话过期时间
func (c *CacheManager) RefreshSession(sessionID string, ttl time.Duration) error {
	key := PrefixSession + sessionID
	return c.client.Expire(c.ctx, key, ttl).Err()
}

// SetLeaderboard 设置排行榜
func (c *CacheManager) SetLeaderboard(guildID string, leaderboardType string, data interface{}) error {
	key := fmt.Sprintf("%s%s:%s", PrefixLeaderboard, guildID, leaderboardType)
	return c.SetJSON(key, data, TTLLeaderboard)
}

// GetLeaderboard 获取排行榜
func (c *CacheManager) GetLeaderboard(guildID string, leaderboardType string, dest interface{}) error {
	key := fmt.Sprintf("%s%s:%s", PrefixLeaderboard, guildID, leaderboardType)
	return c.GetJSON(key, dest)
}

// InvalidateUserCache 使用户缓存失效
func (c *CacheManager) InvalidateUserCache(userID, discordID string) error {
	keys := []string{
		fmt.Sprintf("%sid:%s", PrefixUser, userID),
		fmt.Sprintf("%sdiscord:%s", PrefixUser, discordID),
	}
	return c.Delete(keys...)
}

// InvalidateTeamCache 使团队缓存失效
func (c *CacheManager) InvalidateTeamCache(teamID, teamCode string) error {
	keys := []string{
		fmt.Sprintf("%sid:%s", PrefixTeam, teamID),
		fmt.Sprintf("%steamid:%s", PrefixTeam, teamCode),
	}
	return c.Delete(keys...)
}

// BatchGet 批量获取缓存
func (c *CacheManager) BatchGet(keys []string) (map[string]string, error) {
	if len(keys) == 0 {
		return make(map[string]string), nil
	}

	values, err := c.client.MGet(c.ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for i, key := range keys {
		if s, ok := values[i].(string); ok {
			result[key] = s
		}
	}

	return result, nil
}

// BatchSet 批量设置缓存
func (c *CacheManager) BatchSet(items map[string]interface{}, ttl time.Duration) error {
	pipe := c.client.Pipeline()

	for key, value := range items {
		data, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal value for key %s: %w", key, err)
		}
		pipe.Set(c.ctx, key, data, ttl)
	}

	_, err := pipe.Exec(c.ctx)
	return err
}

// Publish 发布消息
func (c *CacheManager) Publish(channel string, message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return c.client.Publish(c.ctx, channel, data).Err()
}

// Subscribe 订阅频道
func (c *CacheManager) Subscribe(channel string) *redis.PubSub {
	return c.client.Subscribe(c.ctx, channel)
}
