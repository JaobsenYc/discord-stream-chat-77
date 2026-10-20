# Discord LFG Bot 数据库使用指南

本指南介绍如何使用 Discord LFG Bot 的数据库系统，包括 PostgreSQL 和 Redis 的配置、使用和维护。

## 目录

- [架构概述](#架构概述)
- [快速开始](#快速开始)
- [数据库配置](#数据库配置)
- [数据模型](#数据模型)
- [API 使用](#api-使用)
- [迁移管理](#迁移管理)
- [缓存策略](#缓存策略)
- [监控和健康检查](#监控和健康检查)
- [故障排除](#故障排除)

## 架构概述

### 技术栈

- **PostgreSQL 15**: 主要关系数据库，存储用户、团队、签到等核心数据
- **Redis 7**: 缓存层，用于会话存储、速率限制和临时数据
- **sqlx**: Go 数据库访问库，提供类型安全的 SQL 操作
- **Docker**: 容器化部署，简化开发和生产环境

### 数据库结构

```
PostgreSQL (主数据库)
├── users (用户表)
├── check_ins (签到记录)
├── teams (团队信息)
├── team_members (团队成员)
├── user_points (用户积分)
├── point_transactions (积分交易记录)
├── lotteries (抽奖活动)
├── lottery_participants (抽奖参与者)
└── guild_configs (服务器配置)

Redis (缓存层)
├── user:* (用户缓存)
├── team:* (团队缓存)
├── session:* (会话数据)
├── ratelimit:* (速率限制)
└── leaderboard:* (排行榜缓存)
```

## 快速开始

### 1. 环境准备

```bash
# 复制环境变量配置
cp config/consise.env.example .env

# 编辑配置文件，设置数据库连接信息
vim .env
```

### 2. 启动数据库服务

```bash
# 使用 Docker Compose 启动 PostgreSQL 和 Redis
make docker-compose-up

# 或者手动启动
docker-compose up -d postgres redis
```

### 3. 运行数据库迁移

```bash
# 运行所有迁移
make db-migrate

# 查看迁移状态
make db-status
```

### 4. 验证安装

```bash
# 检查数据库健康状态
make db-health

# 运行示例程序
make db-example
```

## 数据库配置

### 环境变量

在根目录 `.env` 中配置以下变量：

```env
# PostgreSQL 配置
DATABASE_URL=postgres://lfguser:your_password@localhost:5432/lfgbot?sslmode=disable
POSTGRES_PASSWORD=your_secure_password

# Redis 配置
REDIS_URL=redis://localhost:6379
REDIS_PASSWORD=your_redis_password
```

### 连接池配置

```go
dbConfig := &database.Config{
    PostgresURL:     os.Getenv("DATABASE_URL"),
    RedisURL:        os.Getenv("REDIS_URL"),
    RedisPassword:   os.Getenv("REDIS_PASSWORD"),
    MaxOpenConns:    25,        // 最大打开连接数
    MaxIdleConns:    5,         // 最大空闲连接数
    ConnMaxLifetime: 5 * time.Minute,  // 连接最大生存时间
    ConnMaxIdleTime: 5 * time.Minute,  // 连接最大空闲时间
}
```

## 数据模型

### 核心实体

#### User (用户)
```go
type User struct {
    ID            uuid.UUID  `db:"id"`
    DiscordID     string     `db:"discord_id"`
    Username      string     `db:"username"`
    Discriminator *string    `db:"discriminator"`
    AvatarURL     *string    `db:"avatar_url"`
    CreatedAt     time.Time  `db:"created_at"`
    UpdatedAt     time.Time  `db:"updated_at"`
    
    // 统计信息
    TotalTeamsCreated int `db:"total_teams_created"`
    TotalTeamsJoined  int `db:"total_teams_joined"`
    TotalCheckIns     int `db:"total_check_ins"`
    
    // 用户设置
    Timezone    string          `db:"timezone"`
    Language    string          `db:"language"`
    Preferences UserPreferences `db:"preferences"`
}
```

#### Team (团队)
```go
type Team struct {
    ID              uuid.UUID  `db:"id"`
    TeamID          string     `db:"team_id"`
    GuildID         string     `db:"guild_id"`
    TextChannelID   string     `db:"text_channel_id"`
    VoiceChannelID  *string    `db:"voice_channel_id"`
    GameName        string     `db:"game_name"`
    MaxPlayers      int        `db:"max_players"`
    CurrentPlayers  int        `db:"current_players"`
    CreatorID       uuid.UUID  `db:"creator_id"`
    Status          string     `db:"status"`
    CreatedAt       time.Time  `db:"created_at"`
    UpdatedAt       time.Time  `db:"updated_at"`
}
```

#### CheckIn (签到)
```go
type CheckIn struct {
    ID             uuid.UUID `db:"id"`
    UserID         uuid.UUID `db:"user_id"`
    DiscordID      string    `db:"discord_id"`
    GuildID        string    `db:"guild_id"`
    ChannelID      string    `db:"channel_id"`
    CheckInDate    time.Time `db:"check_in_date"`
    CheckInTime    time.Time `db:"check_in_time"`
    PointsEarned   int       `db:"points_earned"`
    ConsecutiveDays int      `db:"consecutive_days"`
    CreatedAt      time.Time `db:"created_at"`
}
```

## API 使用

### 初始化数据库连接

```go
package main

import (
    "github.com/jaobsenyc/discord-lfg-bot-go/internal/database"
)

func main() {
    // 创建数据库配置
    config := &database.Config{
        PostgresURL:   "postgres://user:pass@localhost/db",
        RedisURL:      "redis://localhost:6379",
        RedisPassword: "password",
    }
    
    // 连接数据库
    db, err := database.New(config)
    if err != nil {
        panic(err)
    }
    defer db.Close()
    
    // 创建 Repository
    repo := database.NewRepository(db)
    
    // 使用 Repository 进行数据操作
    user, err := repo.User.GetOrCreate("123456789", "TestUser", nil, nil)
    if err != nil {
        panic(err)
    }
}
```

### 用户操作

```go
// 获取或创建用户
user, err := repo.User.GetOrCreate("123456789", "TestUser", nil, nil)

// 根据 Discord ID 获取用户
user, err := repo.User.GetByDiscordID("123456789")

// 更新用户信息
user.Username = "NewUsername"
err := repo.User.Update(user)

// 增加用户统计
err := repo.User.IncrementTeamsCreated(user.ID)
```

### 签到操作

```go
// 用户签到
checkIn, err := repo.CheckIn.CheckIn(user.ID, user.DiscordID, "guildID", "channelID")

// 获取今天的签到记录
todayCheckIn, err := repo.CheckIn.GetTodayCheckIn(user.ID, "guildID")

// 获取用户签到历史
checkIns, err := repo.CheckIn.GetUserCheckIns(user.ID, "guildID", 30)

// 获取签到排行榜
leaderboard, err := repo.CheckIn.GetGuildCheckInLeaderboard("guildID", 10)
```

### 团队操作

```go
// 创建团队
team := &database.Team{
    TeamID:           "team_123",
    GuildID:          "guild_456",
    TextChannelID:    "channel_789",
    GameName:         "英雄联盟",
    MaxPlayers:       5,
    CurrentPlayers:   1,
    CreatorID:        user.ID,
    CreatorDiscordID: user.DiscordID,
    Status:           database.TeamStatusActive,
}
err := repo.Team.Create(team)

// 添加团队成员
err := repo.Team.AddMember(team.ID, memberUserID, "memberDiscordID")

// 获取团队成员
members, err := repo.Team.GetMembers(team.ID)

// 更新团队状态
err := repo.Team.UpdateStatus(team.ID, database.TeamStatusFull)
```

### 积分操作

```go
// 添加积分
err := repo.User.AddPoints(
    user.ID,
    "guildID",
    10,
    database.TransactionTypeCheckIn,
    "Daily check-in",
    nil,
    nil,
)

// 消费积分
err := repo.User.SpendPoints(
    user.ID,
    "guildID",
    5,
    database.TransactionTypeLotteryParticipation,
    "Lottery participation",
    &lotteryID,
    stringPtr("lottery"),
)

// 获取用户积分
points, err := repo.User.GetUserPoints(user.ID, "guildID")

// 获取积分排行榜
leaderboard, err := repo.User.GetLeaderboard("guildID", 10)
```

### 缓存操作

```go
// 创建缓存管理器
cache := database.NewCacheManager(db.Redis)

// 设置缓存
err := cache.SetJSON("key", data, 5*time.Minute)

// 获取缓存
var result MyStruct
err := cache.GetJSON("key", &result)

// 速率限制检查
allowed, err := cache.RateLimitCheck("user:123", 5, time.Minute)

// 会话管理
sessionID := "session_123"
sessionData := map[string]interface{}{"user_id": "123"}
err := cache.SetSession(sessionID, sessionData, 24*time.Hour)
```

## 迁移管理

### 创建新迁移

```bash
# 创建新的迁移文件
make db-create-migration NAME="add_new_feature"

# 这会创建两个文件：
# sql/migrations/003_20240101120000_add_new_feature.sql
# sql/migrations/003_20240101120000_add_new_feature_down.sql
```

### 迁移文件示例

**向上迁移 (003_add_new_feature.sql):**
```sql
-- =====================================================
-- Migration: add_new_feature
-- Version: 3
-- Created: 2024-01-01 12:00:00
-- =====================================================

CREATE TABLE new_feature (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_new_feature_name ON new_feature(name);
```

**向下迁移 (003_add_new_feature_down.sql):**
```sql
-- =====================================================
-- Rollback Migration: add_new_feature
-- Version: 3
-- Created: 2024-01-01 12:00:00
-- =====================================================

DROP INDEX IF EXISTS idx_new_feature_name;
DROP TABLE IF EXISTS new_feature;
```

### 迁移命令

```bash
# 运行所有待执行的迁移
make db-migrate

# 回滚最近的 1 个迁移
make db-migrate-down STEPS=1

# 查看迁移状态
make db-status

# 手动运行迁移工具
go run ./cmd/migrate up
go run ./cmd/migrate down 2
go run ./cmd/migrate status
```

## 缓存策略

### 缓存层次

1. **L1 缓存**: 应用内存缓存 (未实现)
2. **L2 缓存**: Redis 缓存
3. **L3 存储**: PostgreSQL 数据库

### 缓存键命名规范

```
user:id:{uuid}           - 用户信息 (按 ID)
user:discord:{discord_id} - 用户信息 (按 Discord ID)
team:id:{uuid}           - 团队信息 (按 ID)
team:teamid:{team_id}    - 团队信息 (按团队 ID)
session:{session_id}     - 会话数据
ratelimit:{identifier}:{window} - 速率限制
leaderboard:{guild_id}:{type}   - 排行榜
```

### 缓存过期时间

```go
const (
    TTLUser        = 5 * time.Minute   // 用户信息
    TTLTeam        = 5 * time.Minute   // 团队信息
    TTLCheckIn     = 10 * time.Minute  // 签到信息
    TTLLeaderboard = 1 * time.Minute   // 排行榜
    TTLSession     = 24 * time.Hour    // 会话数据
)
```

### 缓存失效策略

```go
// 用户信息更新时清除相关缓存
func (r *UserRepository) Update(user *User) error {
    // 更新数据库
    _, err := r.db.NamedExec(query, user)
    if err != nil {
        return err
    }
    
    // 清除缓存
    _ = r.db.DeleteCache(
        fmt.Sprintf("user:id:%s", user.ID),
        fmt.Sprintf("user:discord:%s", user.DiscordID),
    )
    
    return nil
}
```

## 监控和健康检查

### 健康检查

```go
// 创建健康检查器
checker := database.NewHealthChecker(db)

// 执行健康检查
status := checker.CheckHealth()

// 检查结果
if status.Healthy {
    fmt.Println("数据库健康状态良好")
} else {
    fmt.Println("数据库健康状态异常")
}

// 查看详细信息
for service, info := range status.Services {
    fmt.Printf("%s: %s (延迟: %v)\n", service, info.Status, info.Latency)
    if info.Error != "" {
        fmt.Printf("  错误: %s\n", info.Error)
    }
}
```

### 监控指标

```go
// 获取数据库指标
metrics := checker.GetDatabaseMetrics()

// PostgreSQL 连接池指标
pgMetrics := metrics["postgresql"].(map[string]interface{})
connections := pgMetrics["connections"].(map[string]interface{})
fmt.Printf("打开连接数: %v\n", connections["open"])
fmt.Printf("使用中连接数: %v\n", connections["in_use"])
fmt.Printf("空闲连接数: %v\n", connections["idle"])

// 持续监控
monitorCh := checker.MonitorConnections()
for metrics := range monitorCh {
    // 处理监控数据
    fmt.Printf("监控数据: %+v\n", metrics)
}
```

### 性能监控

```bash
# 使用 Makefile 命令检查健康状态
make db-health

# 查看 Docker 容器状态
docker-compose ps

# 查看 PostgreSQL 日志
docker-compose logs postgres

# 查看 Redis 日志
docker-compose logs redis
```

## 故障排除

### 常见问题

#### 1. 连接失败

**问题**: `failed to connect to postgres: connection refused`

**解决方案**:
```bash
# 检查 Docker 服务是否运行
docker-compose ps

# 启动数据库服务
make docker-compose-up

# 检查端口是否被占用
netstat -tulpn | grep 5432
```

#### 2. 迁移失败

**问题**: `migration failed: relation already exists`

**解决方案**:
```bash
# 检查迁移状态
make db-status

# 手动修复迁移记录
psql $DATABASE_URL -c "INSERT INTO schema_migrations (version, name) VALUES (1, '001_initial_schema');"

# 重新运行迁移
make db-migrate
```

#### 3. Redis 连接问题

**问题**: `failed to connect to redis: connection refused`

**解决方案**:
```bash
# 检查 Redis 服务
docker-compose logs redis

# 测试 Redis 连接
redis-cli -h localhost -p 6379 ping

# 重启 Redis 服务
docker-compose restart redis
```

#### 4. 性能问题

**问题**: 数据库查询缓慢

**解决方案**:
```sql
-- 检查慢查询
SELECT query, mean_time, calls 
FROM pg_stat_statements 
ORDER BY mean_time DESC 
LIMIT 10;

-- 检查索引使用情况
EXPLAIN ANALYZE SELECT * FROM users WHERE discord_id = '123456789';

-- 检查连接池状态
SELECT * FROM pg_stat_activity;
```

### 调试技巧

#### 1. 启用详细日志

```go
// 在开发环境中启用 SQL 日志
db.SetLogger(log.New(os.Stdout, "SQL: ", log.LstdFlags))
```

#### 2. 使用数据库工具

```bash
# 连接到 PostgreSQL
psql $DATABASE_URL

# 连接到 Redis
redis-cli -h localhost -p 6379

# 使用 pgAdmin (Web 界面)
docker run -p 5050:80 -e PGADMIN_DEFAULT_EMAIL=admin@admin.com -e PGADMIN_DEFAULT_PASSWORD=admin dpage/pgadmin4
```

#### 3. 监控查询性能

```sql
-- 启用查询统计
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- 查看最耗时的查询
SELECT query, total_time, mean_time, calls
FROM pg_stat_statements
ORDER BY total_time DESC
LIMIT 10;
```

### 备份和恢复

#### 数据库备份

```bash
# 备份 PostgreSQL
docker-compose exec postgres pg_dump -U lfguser lfgbot > backup.sql

# 备份 Redis
docker-compose exec redis redis-cli BGSAVE
docker-compose exec redis cp /data/dump.rdb /backup/
```

#### 数据库恢复

```bash
# 恢复 PostgreSQL
docker-compose exec -T postgres psql -U lfguser lfgbot < backup.sql

# 恢复 Redis
docker-compose exec redis cp /backup/dump.rdb /data/
docker-compose restart redis
```

## 最佳实践

### 1. 数据库设计

- 使用 UUID 作为主键，避免 ID 冲突
- 合理使用索引，平衡查询性能和写入性能
- 使用外键约束保证数据一致性
- 为时间字段使用 `TIMESTAMP WITH TIME ZONE`

### 2. 缓存使用

- 缓存热点数据，如用户信息、团队状态
- 设置合理的过期时间，避免缓存雪崩
- 在数据更新时及时清除相关缓存
- 使用缓存预热策略

### 3. 性能优化

- 使用连接池管理数据库连接
- 批量操作减少数据库往返次数
- 使用事务保证数据一致性
- 定期分析查询性能并优化

### 4. 安全考虑

- 使用参数化查询防止 SQL 注入
- 限制数据库用户权限
- 定期更新密码和证书
- 启用数据库审计日志

### 5. 监控和维护

- 定期检查数据库健康状态
- 监控连接池使用情况
- 设置告警机制
- 定期备份重要数据

---

## 相关文档

- [Docker Compose 配置](../docker-compose.yml)
- [环境变量配置](../config/consise.env.example)
- [API 文档](./API_REFERENCE.md)
- [部署指南](./DEPLOYMENT_GUIDE.md)

## 支持

如果遇到问题，请：

1. 查看本文档的故障排除部分
2. 检查 GitHub Issues
3. 提交新的 Issue 并提供详细信息

---

*最后更新: 2024-01-01* 