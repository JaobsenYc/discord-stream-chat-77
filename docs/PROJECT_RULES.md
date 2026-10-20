# Discord LFG Bot - 项目规则与开发指南

## 🎯 项目核心定位

这是一个用Golang编写的Discord寻组(LFG)机器人项目，专注于以下几个方面：

### 功能定位
- **主要功能**: 解析玩家发送的寻组消息，自动管理临时语音频道和团队状态
- **目标用户**: Discord游戏社区的玩家
- **核心价值**: 简化寻组流程，提高组队效率

### 技术学习目标
- 掌握Golang WebSocket通信
- 实践并发编程(goroutines)
- 学习Discord API集成
- 练习结构体和map数据管理
- 实现定时器和优雅关闭机制

## 📐 技术架构规则

### 编程语言与框架
- **主语言**: Go (Golang) - 版本1.24.4
- **Discord库**: [bwmarrin/discordgo](https://github.com/bwmarrin/discordgo) v0.29.0
- **配置管理**: 环境变量或`.env`文件
- **日志库**: 使用标准库或`logrus`

### 项目结构规范
```
discord-lfg-bot-go/
├── cmd/                    # 入口点
│   └── bot/
│       └── main.go
├── doc/                    # 文档
├── internal/               # 内部包
│   ├── bot/               # 机器人核心逻辑
│   ├── parser/            # 消息解析
│   ├── channel/           # 频道管理
│   └── team/              # 团队状态管理
├── pkg/                   # 可复用包
├── config/                # 配置文件
├── scripts/               # 部署脚本
├── docker/                # Docker相关文件
└── docs/                  # 文档
```

### 编码规范
- 使用`gofmt`和`golint`保持代码格式一致
- 函数名使用驼峰命名法
- 包名使用小写字母
- 添加必要的注释，特别是公开函数
- 错误处理必须明确，不能忽略error

## 🎮 功能实现规则

### 消息解析规范
- **格式**: `游戏名 团队总人数=当前人数`
- **示例**: 
  - `LOL 5=1` (英雄联盟，5人团队，当前1人)
  - `Valorant 5=3` (Valorant，5人团队，当前3人)
  - `CS2 5=2` (CS2，5人团队，当前2人)

### 频道管理规则
- 为每个寻组请求创建临时语音频道
- 频道名格式: `[游戏名] 寻组 (当前人数/总人数)`
- 团队满员时自动通知并可选择锁定频道
- 设置频道超时清理机制（默认30分钟无活动）

### 数据结构设计
```go
type Team struct {
    ID          string
    Game        string
    MaxPlayers  int
    Players     []Player
    ChannelID   string
    CreatedAt   time.Time
    LastActivity time.Time
}

type Player struct {
    UserID   string
    Username string
    JoinedAt time.Time
}
```

## 🔧 开发流程规则

### Git工作流
- 使用feature分支开发新功能
- 提交信息使用中文，格式: `类型: 简短描述`
  - `feat: 添加消息解析功能`
  - `fix: 修复频道创建bug`
  - `docs: 更新README文档`

### 测试要求
- 核心功能必须有单元测试
- 测试覆盖率目标: 70%+
- 集成测试使用测试服务器

### 部署规则
- 支持Docker容器化部署
- 提供docker-compose配置
- 环境变量配置敏感信息
- 实现健康检查端点

## 🚀 开发优先级

### Phase 1: 基础功能
1. Discord机器人连接和基础事件处理
2. 消息解析逻辑实现
3. 基本的频道创建功能

### Phase 2: 核心功能
1. 团队状态管理
2. 频道自动更新
3. 成员加入/离开处理

### Phase 3: 高级功能
1. 团队满员通知
2. 频道自动清理
3. 多游戏支持

### Phase 4: 优化与部署
1. 性能优化
2. 错误处理完善
3. 容器化部署
4. 监控和日志

## 📝 代码质量要求

- **可读性**: 代码应当自文档化，变量和函数名要有意义
- **可维护性**: 模块化设计，低耦合高内聚
- **健壮性**: 全面的错误处理和边界情况考虑
- **性能**: 合理使用goroutines，避免资源泄漏
- **安全性**: 输入验证，敏感信息保护

## 🔄 持续改进

- 定期review代码质量
- 收集用户反馈优化功能
- 关注Discord API更新
- 学习Go最佳实践并应用 