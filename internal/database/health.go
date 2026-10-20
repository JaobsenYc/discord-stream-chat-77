package database

import (
	"context"
	"fmt"
	"time"
)

// HealthStatus 健康状态
type HealthStatus struct {
	Healthy   bool                   `json:"healthy"`
	Timestamp time.Time              `json:"timestamp"`
	Services  map[string]ServiceInfo `json:"services"`
}

// ServiceInfo 服务信息
type ServiceInfo struct {
	Status  string        `json:"status"`
	Latency time.Duration `json:"latency"`
	Error   string        `json:"error,omitempty"`
	Details interface{}   `json:"details,omitempty"`
}

// HealthChecker 健康检查器
type HealthChecker struct {
	db *DB
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(db *DB) *HealthChecker {
	return &HealthChecker{db: db}
}

// CheckHealth 执行完整的健康检查
func (h *HealthChecker) CheckHealth() *HealthStatus {
	status := &HealthStatus{
		Timestamp: time.Now(),
		Services:  make(map[string]ServiceInfo),
	}

	// 检查 PostgreSQL
	pgInfo := h.checkPostgreSQL()
	status.Services["postgresql"] = pgInfo

	// 检查 Redis
	redisInfo := h.checkRedis()
	status.Services["redis"] = redisInfo

	// 检查数据库连接池
	poolInfo := h.checkConnectionPool()
	status.Services["connection_pool"] = poolInfo

	// 检查缓存性能
	cacheInfo := h.checkCachePerformance()
	status.Services["cache"] = cacheInfo

	// 总体健康状态
	status.Healthy = pgInfo.Status == "healthy" &&
		redisInfo.Status == "healthy" &&
		poolInfo.Status == "healthy"

	return status
}

// checkPostgreSQL 检查 PostgreSQL 连接和性能
func (h *HealthChecker) checkPostgreSQL() ServiceInfo {
	start := time.Now()

	// 基本连接测试
	if err := h.db.Ping(); err != nil {
		return ServiceInfo{
			Status:  "unhealthy",
			Latency: time.Since(start),
			Error:   fmt.Sprintf("ping failed: %v", err),
		}
	}

	// 执行简单查询测试
	var result int
	if err := h.db.Get(&result, "SELECT 1"); err != nil {
		return ServiceInfo{
			Status:  "unhealthy",
			Latency: time.Since(start),
			Error:   fmt.Sprintf("query failed: %v", err),
		}
	}

	// 获取数据库统计信息
	stats := h.db.Stats()
	details := map[string]interface{}{
		"open_connections":     stats.OpenConnections,
		"in_use":               stats.InUse,
		"idle":                 stats.Idle,
		"wait_count":           stats.WaitCount,
		"wait_duration":        stats.WaitDuration,
		"max_idle_closed":      stats.MaxIdleClosed,
		"max_idle_time_closed": stats.MaxIdleTimeClosed,
		"max_lifetime_closed":  stats.MaxLifetimeClosed,
	}

	return ServiceInfo{
		Status:  "healthy",
		Latency: time.Since(start),
		Details: details,
	}
}

// checkRedis 检查 Redis 连接和性能
func (h *HealthChecker) checkRedis() ServiceInfo {
	start := time.Now()
	ctx := context.Background()

	// 基本连接测试
	if err := h.db.Redis.Ping(ctx).Err(); err != nil {
		return ServiceInfo{
			Status:  "unhealthy",
			Latency: time.Since(start),
			Error:   fmt.Sprintf("ping failed: %v", err),
		}
	}

	// 读写测试
	testKey := "health_check_test"
	testValue := fmt.Sprintf("test_%d", time.Now().Unix())

	// 写入测试
	if err := h.db.Redis.Set(ctx, testKey, testValue, time.Minute).Err(); err != nil {
		return ServiceInfo{
			Status:  "unhealthy",
			Latency: time.Since(start),
			Error:   fmt.Sprintf("set failed: %v", err),
		}
	}

	// 读取测试
	val, err := h.db.Redis.Get(ctx, testKey).Result()
	if err != nil {
		return ServiceInfo{
			Status:  "unhealthy",
			Latency: time.Since(start),
			Error:   fmt.Sprintf("get failed: %v", err),
		}
	}

	if val != testValue {
		return ServiceInfo{
			Status:  "unhealthy",
			Latency: time.Since(start),
			Error:   "value mismatch",
		}
	}

	// 清理测试数据
	h.db.Redis.Del(ctx, testKey)

	// 获取 Redis 信息
	info, err := h.db.Redis.Info(ctx).Result()
	if err != nil {
		return ServiceInfo{
			Status:  "degraded",
			Latency: time.Since(start),
			Error:   fmt.Sprintf("info failed: %v", err),
		}
	}

	return ServiceInfo{
		Status:  "healthy",
		Latency: time.Since(start),
		Details: map[string]interface{}{
			"info_available": len(info) > 0,
		},
	}
}

// checkConnectionPool 检查连接池状态
func (h *HealthChecker) checkConnectionPool() ServiceInfo {
	start := time.Now()
	stats := h.db.Stats()

	// 检查连接池是否健康
	status := "healthy"
	var issues []string

	// 检查是否有太多等待的连接
	if stats.WaitCount > 100 {
		status = "degraded"
		issues = append(issues, fmt.Sprintf("high wait count: %d", stats.WaitCount))
	}

	// 检查等待时间是否过长
	if stats.WaitDuration > 5*time.Second {
		status = "degraded"
		issues = append(issues, fmt.Sprintf("high wait duration: %v", stats.WaitDuration))
	}

	// 检查是否有连接泄漏
	if stats.OpenConnections > 20 && stats.InUse < stats.OpenConnections/2 {
		status = "degraded"
		issues = append(issues, "possible connection leak")
	}

	details := map[string]interface{}{
		"open_connections": stats.OpenConnections,
		"in_use":           stats.InUse,
		"idle":             stats.Idle,
		"wait_count":       stats.WaitCount,
		"wait_duration":    stats.WaitDuration.String(),
	}

	if len(issues) > 0 {
		details["issues"] = issues
	}

	return ServiceInfo{
		Status:  status,
		Latency: time.Since(start),
		Details: details,
	}
}

// checkCachePerformance 检查缓存性能
func (h *HealthChecker) checkCachePerformance() ServiceInfo {
	start := time.Now()
	ctx := context.Background()

	// 执行多个缓存操作来测试性能
	testKey := "perf_test"
	testData := make(map[string]interface{})
	for i := 0; i < 100; i++ {
		testData[fmt.Sprintf("key_%d", i)] = fmt.Sprintf("value_%d", i)
	}

	// 测试批量写入
	writeStart := time.Now()
	pipe := h.db.Redis.Pipeline()
	for k, v := range testData {
		pipe.Set(ctx, fmt.Sprintf("%s:%s", testKey, k), v, time.Minute)
	}
	_, err := pipe.Exec(ctx)
	writeLatency := time.Since(writeStart)

	if err != nil {
		return ServiceInfo{
			Status:  "unhealthy",
			Latency: time.Since(start),
			Error:   fmt.Sprintf("batch write failed: %v", err),
		}
	}

	// 测试批量读取
	readStart := time.Now()
	keys := make([]string, 0, len(testData))
	for k := range testData {
		keys = append(keys, fmt.Sprintf("%s:%s", testKey, k))
	}
	_, err = h.db.Redis.MGet(ctx, keys...).Result()
	readLatency := time.Since(readStart)

	if err != nil {
		return ServiceInfo{
			Status:  "unhealthy",
			Latency: time.Since(start),
			Error:   fmt.Sprintf("batch read failed: %v", err),
		}
	}

	// 清理测试数据
	h.db.Redis.Del(ctx, keys...)

	// 评估性能
	status := "healthy"
	if writeLatency > 100*time.Millisecond || readLatency > 50*time.Millisecond {
		status = "degraded"
	}

	return ServiceInfo{
		Status:  status,
		Latency: time.Since(start),
		Details: map[string]interface{}{
			"write_latency": writeLatency.String(),
			"read_latency":  readLatency.String(),
			"operations":    len(testData),
		},
	}
}

// GetDatabaseMetrics 获取数据库指标
func (h *HealthChecker) GetDatabaseMetrics() map[string]interface{} {
	metrics := make(map[string]interface{})

	// PostgreSQL 指标
	stats := h.db.Stats()
	metrics["postgresql"] = map[string]interface{}{
		"connections": map[string]interface{}{
			"open":   stats.OpenConnections,
			"in_use": stats.InUse,
			"idle":   stats.Idle,
		},
		"wait": map[string]interface{}{
			"count":    stats.WaitCount,
			"duration": stats.WaitDuration.Nanoseconds(),
		},
		"closed": map[string]interface{}{
			"max_idle":      stats.MaxIdleClosed,
			"max_idle_time": stats.MaxIdleTimeClosed,
			"max_lifetime":  stats.MaxLifetimeClosed,
		},
	}

	// Redis 指标
	ctx := context.Background()
	info, err := h.db.Redis.Info(ctx, "memory", "stats", "clients").Result()
	if err == nil {
		metrics["redis"] = map[string]interface{}{
			"info_available": true,
			"info_length":    len(info),
		}
	} else {
		metrics["redis"] = map[string]interface{}{
			"info_available": false,
			"error":          err.Error(),
		}
	}

	return metrics
}

// MonitorConnections 监控数据库连接
func (h *HealthChecker) MonitorConnections() <-chan map[string]interface{} {
	ch := make(chan map[string]interface{})

	go func() {
		defer close(ch)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			metrics := h.GetDatabaseMetrics()
			metrics["timestamp"] = time.Now()

			select {
			case ch <- metrics:
			default:
				// 如果通道满了，跳过这次发送
			}
		}
	}()

	return ch
}
