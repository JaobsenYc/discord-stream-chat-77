# 频道管理功能说明

## 概述

Discord LFG机器人现在支持智能频道管理，确保每个用户只能同时创建一个寻组房间，并且所有频道都会创建在指定的分类下。

## 主要功能

### 1. 指定分类下创建频道

所有语音频道都会自动创建在配置的分类下，保持服务器整洁有序。

**配置方法:**
```bash
# 在 .env 文件中设置
VOICE_CHANNEL_CATEGORY_ID=your_category_id_here
```

**获取分类ID的方法:**
1. 在Discord中启用开发者模式：用户设置 → 高级 → 开发者模式
2. 右键点击分类名称，选择"复制ID"
3. 将ID粘贴到环境变量中

### 2. 每用户单频道限制

每个用户同时只能创建一个寻组房间，避免频道混乱。

**工作原理:**
- 用户首次发送LFG消息时，创建新的语音频道
- 用户再次发送LFG消息时，更新现有频道而不是创建新频道
- 频道名称会根据新的游戏名称自动更新
- 团队成员列表会根据新的游戏要求重新设置

## 使用示例

### 场景1：首次创建房间
```
用户发送: LOL 5=1
结果: 创建新的语音频道 "LOL-用户名"
```

### 场景2：更换游戏
```
用户之前创建了: LOL 5=1
用户发送: Valorant 5=1
结果: 将现有频道重命名为 "Valorant-用户名"，并更新团队设置
```

### 场景3：调整人数
```
用户之前创建了: LOL 5=1
用户发送: LOL 3=1
结果: 保持频道名称不变，但将最大人数调整为3人
```

## 技术实现

### 用户频道追踪
- 使用内存映射 `map[string]string` 追踪用户ID到频道ID的关系
- 线程安全，支持并发访问
- 团队删除时自动清理记录

### 频道更新逻辑
```go
// 检查用户是否已有频道
if existingChannelID, hasChannel := teamManager.GetUserChannel(userID); hasChannel {
    // 更新现有频道
    channelManager.UpdateExistingChannel(channelID, teamInfo, categoryID)
} else {
    // 创建新频道
    channelManager.CreateVoiceChannel(teamInfo)
}
```

### 数据结构
```go
type Manager struct {
    teams        map[string]*Team
    userChannels map[string]string // userID -> channelID 映射
    mutex        sync.RWMutex      // 读写锁保证并发安全
}
```

## 配置选项

### 必需配置
```bash
# Discord机器人Token
DISCORD_TOKEN=your_bot_token

# 服务器ID
GUILD_ID=your_guild_id
```

### 可选配置
```bash
# 语音频道分类ID (推荐设置)
VOICE_CHANNEL_CATEGORY_ID=your_category_id

# 频道清理超时时间
CHANNEL_CLEANUP_TIMEOUT=30m

# 团队人数限制
MAX_TEAM_SIZE=10
MIN_TEAM_SIZE=2
```

## 权限要求

机器人需要以下Discord权限：
- ✅ **管理频道** (Manage Channels) - 创建、编辑、删除频道
- ✅ **发送消息** (Send Messages) - 发送邀请和通知
- ✅ **连接语音频道** (Connect) - 验证频道可用性
- ✅ **移动成员** (Move Members) - 将用户移动到频道

## 故障处理

### 频道更新失败
如果更新现有频道失败（例如频道被删除），系统会：
1. 记录警告日志
2. 自动创建新频道
3. 清理过期的用户频道记录

### 权限不足
如果机器人权限不足：
1. 检查机器人角色权限
2. 确保分类权限正确设置
3. 查看日志获取详细错误信息

## 监控和日志

### 关键日志事件
```
[INFO] 创建语音频道: LOL-用户名 (ID: 123456789)
[INFO] 更新用户 用户名 的现有频道: 123456789 -> Valorant
[INFO] 用户 用户ID 的频道设置为: 123456789
[WARN] 更新现有频道失败: 频道不存在
```

### 状态监控
使用 `!status` 命令查看当前活跃团队和频道状态。

## 最佳实践

1. **设置专用分类**: 为LFG频道创建专用分类，保持服务器整洁
2. **定期检查权限**: 确保机器人始终有足够权限
3. **监控日志**: 定期查看日志了解系统运行状况
4. **合理设置超时**: 根据游戏特性调整频道清理时间

## 未来扩展

计划中的功能：
- [ ] 数据库持久化用户频道记录
- [ ] 支持用户主动删除自己的频道
- [ ] 频道使用统计和分析
- [ ] 自定义频道命名模板 