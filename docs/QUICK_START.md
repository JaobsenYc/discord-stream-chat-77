# Discord LFG Bot - 快速开始指南

## 🚀 项目简介

这是一个用Golang编写的Discord寻组(LFG)机器人。机器人监听Discord消息并帮助玩家组建游戏团队。当用户发送类似"LOL 5=1"的消息时，机器人会自动创建临时语音频道，管理团队状态，并在团队满员时发送通知。

## 📋 功能特性

- 🎮 **智能消息解析**: 支持多种LFG消息格式 (`LOL 5=1`, `Valorant 3/5`, `CS2: 5=2`)
- 🎯 **自动频道管理**: 创建/更新/删除临时语音频道
- 👥 **实时团队状态**: 自动跟踪玩家加入/离开
- 🔔 **满员通知**: 团队满员时自动发送美观的通知消息
- 🧹 **智能清理**: 自动清理过期的空频道
- 🌐 **多游戏支持**: 支持LOL、Valorant、CS2、Apex等热门游戏
- 🐳 **容器化部署**: 完整的Docker部署方案

## ⚡ 快速开始

### 1. 环境准备

确保你的系统已安装：
- **Go 1.19+** ([下载地址](https://golang.org/dl/))
- **Git** ([下载地址](https://git-scm.com/downloads))
- **Docker** ([下载地址](https://www.docker.com/get-started)) (可选，用于容器化部署)

### 2. 克隆项目

```bash
git clone https://github.com/jaobsenyc/discord-lfg-bot-go.git
cd discord-lfg-bot-go
```

### 3. 配置环境变量

```bash
# 复制环境变量模板
cp config/consise.env.example .env

# 编辑环境变量
vim .env  # 或使用你喜欢的编辑器
```

**必需配置项**:
- `DISCORD_TOKEN`: 你的Discord机器人Token
- `GUILD_ID`: 你的Discord服务器ID

**可选配置项**:
- `VOICE_CHANNEL_CATEGORY_ID`: 语音频道分类ID
- `LOG_LEVEL`: 日志级别 (debug/info/warn/error)
- `CHANNEL_CLEANUP_TIMEOUT`: 频道清理超时时间

### 4. 获取Discord机器人Token

1. 访问 [Discord Developer Portal](https://discord.com/developers/applications)
2. 创建新应用程序
3. 在"Bot"页面创建机器人并复制Token
4. 在"OAuth2"页面生成邀请链接，权限包括：
   - `Send Messages`
   - `Manage Channels`
   - `Connect`
   - `Speak`
   - `Move Members`

### 5. 运行机器人

#### 方式1: 使用Makefile (推荐)

```bash
# 检查环境
make check

# 快速开始 (安装依赖、测试、构建)
make quick-start

# 运行开发模式
make run
```

#### 方式2: 使用脚本

```bash
# 检查环境
./scripts/run.sh check

# 运行开发模式
./scripts/run.sh dev
```

#### 方式3: 直接运行Go

```bash
# 安装依赖
go mod download

# 运行机器人
go run cmd/bot/main.go
```

### 6. Docker部署 (推荐生产环境)

```bash
# 使用Docker Compose部署
make docker-compose-up

# 查看日志
make docker-compose-logs

# 停止服务
make docker-compose-down
```

## 📖 使用方法

### 基本命令

机器人启动后，在Discord服务器的任意文本频道发送：

```
!help          # 显示帮助信息
!status        # 查看当前寻组状态
```

### 寻组消息格式

支持多种格式，选择你喜欢的：

```
LOL 5=1           # 英雄联盟，5人团队，当前1人
Valorant 5=3      # Valorant，5人团队，当前3人  
CS2 3/5           # CS2，当前3人，总共5人
PUBG: 4=2         # PUBG，4人团队，当前2人
apex 3=1          # Apex，3人团队，当前1人 (大小写不敏感)
英雄联盟 5=2      # 支持中文游戏名
```

### 支持的游戏

- **英雄联盟** (LOL, League, 撸啊撸)
- **Valorant** (VAL, 瓦洛兰特)
- **CS2** (CS, 反恐精英)
- **守望先锋2** (OW2, Overwatch)
- **Apex Legends** (Apex, 顶点传说)
- **绝地求生** (PUBG, 吃鸡)
- **Dota 2** (Dota, 刀塔)

## 🛠️ 开发指南

### 项目结构

```
discord-lfg-bot-go/
├── cmd/bot/main.go           # 程序入口
├── doc/                    # 文档
├── internal/                 # 内部包
│   ├── bot/                 # 机器人核心逻辑
│   ├── parser/              # 消息解析
│   ├── channel/             # 频道管理
│   ├── team/                # 团队状态管理
│   └── config/              # 配置管理
├── pkg/logger/              # 日志工具
├── docker/                  # Docker配置
├── scripts/                 # 部署脚本
└── tests/                   # 测试文件
```

### 常用开发命令

```bash
# 代码格式化
make fmt

# 运行测试
make test

# 生成测试覆盖率报告
make test-coverage

# 运行代码检查
make lint

# 构建所有平台版本
make build-all

# 安装开发工具
make install-tools
```

### 运行测试

```bash
# 运行所有测试
make test

# 运行特定包的测试
go test ./internal/parser/

# 运行竞态条件检测
make test-race

# 运行基准测试
make bench
```

## 🐛 故障排除

### 常见问题

1. **机器人无响应**
   - 检查Discord Token是否正确
   - 确认机器人已加入服务器
   - 检查机器人权限设置

2. **无法创建语音频道**
   - 确认机器人有"管理频道"权限
   - 检查`VOICE_CHANNEL_CATEGORY_ID`配置

3. **消息解析失败**
   - 检查消息格式是否正确
   - 查看日志获取详细错误信息

### 查看日志

```bash
# 本地运行查看日志
export LOG_LEVEL=debug
go run cmd/bot/main.go

# Docker部署查看日志
docker compose logs -f lfg-bot
```

## 🚀 部署

### 生产环境部署

1. **使用Docker Compose (推荐)**:
   ```bash
   # 构建并启动
   make docker-compose-up
   
   # 检查状态
   docker-compose ps
   ```

2. **二进制部署**:
   ```bash
   # 构建Linux版本
   make build-linux
   
   # 复制到服务器
   scp bin/discord-lfg-bot-linux user@server:/opt/discord-lfg-bot/
   ```

### 进程管理

推荐使用systemd管理服务：

```ini
# /etc/systemd/system/discord-lfg-bot.service
[Unit]
Description=Discord LFG Bot
After=network.target

[Service]
Type=simple
User=discord-bot
WorkingDirectory=/opt/discord-lfg-bot
ExecStart=/opt/discord-lfg-bot/discord-lfg-bot
Restart=always
RestartSec=5
EnvironmentFile=/opt/discord-lfg-bot/.env

[Install]
WantedBy=multi-user.target
```

## 🤝 贡献指南

欢迎贡献代码！请遵循以下步骤：

1. Fork本项目
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'feat: 添加优秀功能'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建Pull Request

### 提交信息格式

使用约定式提交格式：
- `feat: 新功能`
- `fix: 修复bug`
- `docs: 文档更新`
- `style: 代码格式`
- `refactor: 重构`
- `test: 测试相关`

## 📄 许可证

本项目采用MIT许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 🙋‍♂️ 支持

如果你遇到问题或有建议，请：

1. 查看 [故障排除](#故障排除) 部分
2. 搜索现有的 [Issues](https://github.com/jaobsenyc/discord-lfg-bot-go/issues)
3. 创建新的Issue并提供详细信息

---

**Happy Gaming! 🎮** 