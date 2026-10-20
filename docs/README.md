# Discord LFG Bot 文档

Discord LFG (Looking For Group) Bot 是一个用 Go 语言编写的 Discord 机器人，帮助玩家组建游戏团队。

## 📚 文档目录

### 🚀 快速开始
- **[快速开始指南](QUICK_START.md)** - 项目安装、配置和运行的完整指南

### 🐳 部署相关
- **[Docker 部署指南](DOCKER_SETUP.md)** - 使用 Docker Compose 部署完整环境

### 🗄️ 数据库
- **[数据库使用指南](DATABASE_GUIDE.md)** - PostgreSQL + Redis 数据库架构和使用说明

### ⚙️ 配置和管理
- **[游戏分类设置](GAME_CATEGORIES_SETUP.md)** - 游戏分类和频道映射配置
- **[频道管理](CHANNEL_MANAGEMENT.md)** - Discord 频道管理功能说明

### 🔧 开发和调试
- **[调试指南](DEBUG_GUIDE.md)** - 问题排查和调试方法
- **[项目规范](PROJECT_RULES.md)** - 代码规范和开发约定

## 🎯 项目概述

### 核心功能
- **智能消息解析**: 识别 "LOL 4=1" 格式的寻组消息
- **自动频道管理**: 创建和管理临时语音频道
- **团队状态跟踪**: 实时更新团队成员状态
- **数据持久化**: 用户签到、积分系统、团队历史
- **多游戏支持**: 支持多种游戏的分类管理

### 技术架构
- **后端**: Go + Discord.go
- **数据库**: PostgreSQL 15 (主数据库) + Redis 8 (缓存/会话)
- **部署**: Docker Compose(bot + 数据库 + 每日自动备份容器)
- **健康检查**: 内置 `/health` 端点 + Docker HEALTHCHECK

### 快速部署

```bash
# 1. 克隆项目
git clone <repository-url>
cd discord-lfg-bot-go

# 2. 配置环境变量
cp config/consise.env.example .env
# 编辑 .env 文件设置 Discord Token 等

# 3. 使用 Docker 一键部署
make docker-setup

# 4. 查看运行状态
make docker-status
```

## 📖 详细文档

每个文档都包含详细的说明和示例：

- **新用户**: 从 [快速开始指南](QUICK_START.md) 开始
- **部署运维**: 查看 [Docker 部署指南](DOCKER_SETUP.md)
- **数据库开发**: 参考 [数据库使用指南](DATABASE_GUIDE.md)
- **功能配置**: 查看相应的配置文档
- **问题排查**: 使用 [调试指南](DEBUG_GUIDE.md)

## 🆘 获取帮助

如果遇到问题：
1. 查看相关文档的故障排除部分
2. 检查 [调试指南](DEBUG_GUIDE.md)
3. 查看项目 Issues
4. 使用 `make help` 查看可用命令

---

**注意**: 所有配置文件中的敏感信息（如 Token、密码）都不应提交到版本控制系统。
