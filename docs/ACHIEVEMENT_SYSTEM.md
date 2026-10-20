# 🏅 成就系统设计文档

## 概述

本成就系统是为Discord LFG机器人设计的茶系主题成就体验，旨在提升用户活跃度和参与感，通过完成各种活动解锁成就并获得奖励。

## 🎯 设计目标

- **增强用户参与度**：通过成就系统激励用户更多地使用机器人功能
- **茶文化主题**：所有成就都围绕"茶道"概念设计，营造独特的文化氛围
- **多样化奖励**：支持积分、身份组、徽章等多种奖励类型
- **渐进式解锁**：合理设置成就难度，提供持续的成长感
- **与抽奖系统集成**：特定成就可作为抽奖参与条件

## 📋 功能特性

### 成就类型

| 类型 | 描述 | 触发条件 |
|------|------|----------|
| `checkin` | 签到成就 | 用户执行 `/checkin daily` |
| `voice` | 语音成就 | 用户在语音频道活动 |
| `message` | 消息成就 | 用户发送消息 |
| `lottery` | 抽奖成就 | 用户参与抽奖活动 |
| `lottery_win` | 中奖成就 | 用户在抽奖中获胜 |
| `activity` | 活跃成就 | 用户日常活跃行为 |

### 奖励类型

- **积分奖励** (`points`)：直接增加用户积分余额
- **角色奖励** (`role`)：授予特殊Discord身份组
- **徽章奖励** (`badge`)：记录特殊荣誉标识

### 茶系成就列表

#### 🫖 签到成就
- **初识茶香**：初次踏入茶道之旅，品味第一口清香（签到1次，奖励50积分）
- **连饮七日**：一周不忘，每日一饮，成习惯也（连续签到7天，奖励100积分）
- **不离不弃**：如老茶沉香，日久弥坚（连续签到30天，奖励300积分）

#### 🎤 语音成就
- **语茶有声**：茶话一席，声落杯中（累计语音1小时，奖励100积分）
- **长谈对饮**：与友对谈，茶伴夜长（累计语音24小时，奖励500积分）

#### 💬 社交成就
- **以茶会友**：云起茶香，你是群中最会说的那位（发送100条消息，奖励150积分）
- **茶会主持人**：无论文字或语音，处处可见你煮茶论道（发送1000条消息，获得特殊身份组）

#### 🎲 抽奖成就
- **幸运首抽**：参与任意一次抽奖（奖励50积分）
- **连胜王者**：连续三次抽奖中得大奖（获得王者身份组）

#### ☕ 活跃成就
- **信手拈茶**：你已习惯在茶馆游走，茶气裹身（活跃10天，奖励200积分）
- **茶道宗师**：无茶不欢，炉火纯青（活跃100天，获得宗师徽章）

## 🖥️ 命令系统

### 用户命令

| 命令 | 描述 | 参数 |
|------|------|------|
| `/achievement` | 查看自己的成就进度 | `category` (可选) |
| `/achievement-summary` | 查看成就总览 | 无 |
| `/achievement-leaderboard` | 查看成就排行榜 | `limit` (可选，1-20) |
| `/achievement-list` | 查看全部成就说明 | 无 |
| `/achievement-info` | 查看特定成就信息 | `name` (必需) |

### 管理员命令

| 命令 | 描述 | 权限要求 |
|------|------|----------|
| `/achievement-admin process-rewards` | 处理待处理的奖励 | 管理服务器 |
| `/achievement-admin reset-progress` | 重置用户成就进度 | 管理服务器 |
| `/achievement-admin grant-achievement` | 手动授予成就 | 管理服务器 |

## 📊 数据库结构

### 核心表

#### `achievements` - 成就定义表
```sql
CREATE TABLE achievements (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL,
    type TEXT NOT NULL,
    target_value INT NOT NULL,
    reward_type TEXT NOT NULL,
    reward_value TEXT NOT NULL,
    icon_emoji TEXT,
    category TEXT NOT NULL,
    display_order INT NOT NULL,
    is_hidden BOOLEAN NOT NULL,
    is_active BOOLEAN NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE
);
```

#### `user_achievements` - 用户成就进度表
```sql
CREATE TABLE user_achievements (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    achievement_id UUID NOT NULL REFERENCES achievements(id),
    progress INT NOT NULL DEFAULT 0,
    unlocked BOOLEAN NOT NULL DEFAULT FALSE,
    unlocked_at TIMESTAMP WITH TIME ZONE,
    reward_claimed BOOLEAN NOT NULL DEFAULT FALSE,
    reward_claimed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE,
    UNIQUE(user_id, achievement_id)
);
```

#### `achievement_rewards` - 成就奖励记录表
```sql
CREATE TABLE achievement_rewards (
    id UUID PRIMARY KEY,
    user_achievement_id UUID NOT NULL,
    user_id UUID NOT NULL,
    guild_id TEXT NOT NULL,
    reward_type TEXT NOT NULL,
    reward_value TEXT NOT NULL,
    points_awarded INT DEFAULT 0,
    role_id TEXT,
    badge_name TEXT,
    processed BOOLEAN NOT NULL DEFAULT FALSE,
    processed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE
);
```

## 🔧 技术实现

### 架构分层

1. **数据访问层** (`internal/database/achievement_repository.go`)
   - 数据库CRUD操作
   - 成就进度计算和更新
   - 奖励处理和记录

2. **服务层** (`internal/achievement/service.go`)
   - 业务逻辑处理
   - 事件触发和成就检查
   - Discord消息格式化

3. **命令层** (`internal/achievement/commands.go`)
   - Discord斜杠命令处理
   - 用户交互管理
   - 权限检查

4. **集成层** (`internal/bot/bot.go`)
   - 与现有系统集成
   - 事件监听和分发
   - 异步处理

### 事件触发机制

系统通过监听以下事件自动更新成就进度：

```go
// 签到事件
achievementService.OnCheckIn(userID, discordID, guildID, consecutiveDays)

// 语音活动事件
achievementService.OnVoiceActivity(userID, discordID, guildID, durationSeconds)

// 消息发送事件
achievementService.OnMessageSent(userID, discordID, guildID)

// 抽奖参与事件
achievementService.OnLotteryParticipation(userID, discordID, guildID)

// 抽奖中奖事件
achievementService.OnLotteryWin(userID, discordID, guildID, isWin)

// 日常活跃事件
achievementService.OnDailyActivity(userID, discordID, guildID)
```

### 性能优化

- **异步处理**：所有成就更新都在goroutine中异步执行，不阻塞主要功能
- **数据库索引**：关键查询字段都建立了索引
- **批量处理**：支持批量处理奖励发放
- **缓存策略**：排行榜等查询结果会被缓存

## 🎮 使用示例

### 查看个人成就
```
用户: /achievement
机器人: [返回包含进度条和完成状态的美观嵌入消息]
```

### 查看特定分类成就
```
用户: /achievement category:checkin
机器人: [只显示签到相关的成就进度]
```

### 查看成就总览
```
用户: /achievement-summary
机器人: [显示完成率、总积分、最近解锁等统计信息]
```

### 查看排行榜
```
用户: /achievement-leaderboard limit:10
机器人: [显示前10名用户的成就完成情况]
```

## 🎨 UI设计

### 成就进度显示
```
🏅 张三 的成就进度

🫖 签到成就
🫖 **初识茶香 ✅**
▓▓▓▓▓▓▓▓▓▓▓▓ 已完成
初次踏入茶道之旅，品味第一口清香

🍃 **连饮七日**
▓▓▓▓▓▓▓░░░░░ 7/7 天
一周不忘，每日一饮，成习惯也

🎤 语音成就
🎤 **语茶有声**
▓▓▓▓▓░░░░░░░ 2100/3600 秒
茶话一席，声落杯中
```

### 成就解锁通知
```
🎉 成就解锁！

🫖 **初识茶香**
初次踏入茶道之旅，品味第一口清香

恭喜获得 50 积分奖励！
```

## 🔮 扩展性

### 新增成就类型
1. 在数据库中插入新的成就记录
2. 在相应的事件处理器中添加触发逻辑
3. 更新命令处理器中的分类映射

### 新增奖励类型
1. 在 `RewardType` 常量中添加新类型
2. 在 `processRewards` 方法中添加处理逻辑
3. 更新显示格式化方法

### 抽奖系统集成
通过 `lottery_achievement_requirements` 表建立抽奖池与成就的关联关系，实现基于成就的抽奖资格验证：

```go
isEligible, missingAchievements, err := achievementService.CheckLotteryEligibility(userID, lotteryID)
```

## 🐛 故障排除

### 常见问题

1. **成就未正确解锁**
   - 检查事件触发是否正常
   - 验证成就条件逻辑
   - 查看数据库中的进度记录

2. **奖励未发放**
   - 运行 `/achievement-admin process-rewards` 手动处理
   - 检查 `achievement_rewards` 表中的错误信息
   - 验证Discord权限设置

3. **排行榜不更新**
   - 清除相关缓存
   - 检查数据库连接状态
   - 重启机器人服务

### 调试模式

在开发环境中，可以通过日志查看详细的成就处理信息：

```go
logger.Debugf("Achievement progress updated: user=%s, achievement=%s, progress=%d", 
    userID, achievementName, newProgress)
```

## 📈 监控指标

建议监控以下指标来评估成就系统的效果：

- 用户成就完成率
- 各类型成就的受欢迎程度
- 成就解锁后的用户活跃度变化
- 系统性能和响应时间

## 🚀 部署说明

1. **数据库迁移**
   ```bash
   # 应用迁移
   ./migrate up
   
   # 如需回滚
   ./migrate down 1
   ```

2. **配置检查**
   - 确保Discord机器人有足够的权限管理角色
   - 验证数据库连接配置
   - 检查缓存系统设置

3. **功能测试**
   - 测试各类成就的触发条件
   - 验证命令响应正常
   - 确认奖励发放机制

通过这个完整的成就系统，用户可以在使用Discord LFG机器人的过程中享受到类似RPG游戏的成长体验，同时机器人管理员也能通过成就数据更好地了解社区活跃情况。 