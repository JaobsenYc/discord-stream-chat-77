# Discord LFG Bot

一个用 Go 语言编写的 Discord 寻组机器人，帮助玩家快速组建游戏团队。

## ✨ 特性

- 🎮 **智能消息解析** - 自动识别 "LOL 4=1" 格式的寻组消息
- 🔊 **自动频道管理** - 创建和管理临时语音频道
- 👥 **团队状态跟踪** - 实时更新团队成员状态
- 📊 **数据持久化** - 用户签到、积分系统、团队历史
- 🎯 **多游戏支持** - 支持多种游戏的分类管理
- 🐳 **容器化部署** - 使用 Docker Compose 一键部署

## 🚀 快速开始

### 使用 Docker（推荐）

```bash
# 1. 克隆项目
git clone <repository-url>
cd discord-lfg-bot-go

# 2. 配置环境变量
cp config/consise.env.example .env
# 编辑 .env 文件，设置 Discord Token 等必要配置

# 3. 一键部署
make docker-setup

# 4. 查看运行状态
make docker-status
```

### 本地开发

```bash
# 1. 安装依赖
make deps

# 2. 配置环境变量
cp config/consise.env.example .env

# 3. 启动数据库（使用 Docker）
make docker-up postgres redis

# 4.（可选）查看 schema 状态；首次启动已由 sql/init 自动全量建库
make db-status

# 5. 启动应用
make run
```

## 🏗️ 技术架构

- **后端**: Go + [DiscordGo](https://github.com/bwmarrin/discordgo)
- **数据库**: PostgreSQL 15 (主数据库) + Redis 8 (缓存/会话)
- **部署**: Docker Compose(bot + 数据库 + 每日自动备份容器)
- **健康检查**: 内置 `/health` 端点 + Docker HEALTHCHECK

## 📚 文档

详细文档请查看 [`docs/`](docs/) 目录：

- **[快速开始指南](docs/QUICK_START.md)** - 完整的安装和配置指南
- **[Docker 部署指南](docs/DOCKER_SETUP.md)** - 使用 Docker 部署
- **[数据库使用指南](docs/DATABASE_GUIDE.md)** - 数据库架构和使用
- **[游戏分类设置](docs/GAME_CATEGORIES_SETUP.md)** - 游戏配置说明
- **[调试指南](docs/DEBUG_GUIDE.md)** - 问题排查和调试

## 🔧 常用命令

```bash
# 开发相关
make run              # 启动开发模式
make test             # 运行测试
make build            # 构建应用

# Docker 相关
make docker-setup     # 一键 Docker 部署
make docker-up        # 启动 Docker 服务
make docker-logs-bot  # 查看 Bot 日志

# 数据库相关
make db-migrate       # 运行数据库迁移
make db-health        # 检查数据库健康状态

# 查看所有可用命令
make help
```

## 🌐 访问服务

部署成功后可以访问：

- **健康检查**: http://localhost:8080/health
- **PostgreSQL**: localhost:5433(宿主机映射端口)
- **Redis**: localhost:6380(宿主机映射端口)

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📄 许可证

[MIT License](LICENSE)

---

**快速链接**: [文档目录](docs/README.md) | [快速开始](docs/QUICK_START.md) | [Docker 部署](docs/DOCKER_SETUP.md) 