# Discord LFG Bot - 本机调试指南

## 🚀 快速开始调试

### 1. 环境准备

确保你已经完成以下设置：

```bash
# 检查Go版本 (需要 Go 1.21+)
go version

# 检查项目依赖
go mod tidy

# 如果网络有问题，可以设置代理
export GOPROXY=https://goproxy.cn,direct
go mod tidy
```

### 2. 配置环境变量

编辑你的 `.env` 文件，确保包含以下调试配置：

```bash
# 必需配置 (替换为你的实际值)
DISCORD_TOKEN=your_discord_bot_token_here
GUILD_ID=your_guild_id_here

# 调试配置
LOG_LEVEL=debug
DEVELOPMENT_MODE=true
BOT_PREFIX=debug!

# 快速测试设置
CHANNEL_CLEANUP_TIMEOUT=2m
MAX_TEAM_SIZE=5
MIN_TEAM_SIZE=1

# 启用所有功能
ENABLE_AUTO_CLEANUP=true
ENABLE_FULL_TEAM_NOTIFICATIONS=true
ENABLE_CUSTOM_GAMES=true
ENABLE_ADMIN_COMMANDS=true
```

### 3. 运行调试命令

#### 方法 1: 使用 Makefile (推荐)

```bash
# 开发模式运行
make run

# 或者使用调试模式
make debug

# 运行测试
make test

# 检查代码
make lint
```

#### 方法 2: 直接使用 Go 命令

```bash
# 开发模式运行
go run cmd/bot/main.go

# 或者先构建再运行
go build -o bin/discord-lfg-bot cmd/bot/main.go
./bin/discord-lfg-bot
```

#### 方法 3: 使用脚本运行

```bash
# 使用项目脚本
./scripts/run.sh --dev

# 或者调试模式
./scripts/run.sh --debug
```

### 4. 调试工具和技巧

#### 4.1 日志调试

```bash
# 实时查看日志
tail -f logs/discord-lfg-bot.log

# 或者直接在控制台查看 (如果没有设置日志文件)
# 日志会直接输出到控制台
```

#### 4.2 性能分析

如果启用了性能分析 (`ENABLE_PROFILING=true`)：

```bash
# 访问性能分析界面
open http://localhost:6060/debug/pprof/

# 查看CPU使用情况
go tool pprof http://localhost:6060/debug/pprof/profile

# 查看内存使用情况
go tool pprof http://localhost:6060/debug/pprof/heap
```

#### 4.3 健康检查

```bash
# 检查服务状态
curl http://localhost:8080/health

# 查看指标 (如果启用)
curl http://localhost:8080/metrics
```

### 5. 测试Discord功能

#### 5.1 基础测试

在Discord服务器中发送以下消息测试：

```
# 基础LFG测试
lol 5=1
cs2 5=2
valorant 5=3

# 测试不同格式
LOL 5/5
英雄联盟 5-1
```

#### 5.2 管理员命令测试

如果设置了管理员权限：

```
debug!status
debug!teams
debug!cleanup
debug!help
```

### 6. 常见问题和解决方案

#### 6.1 机器人无法启动

**检查清单：**
```bash
# 1. 验证Token
echo $DISCORD_TOKEN  # 或查看 .env 文件

# 2. 验证Guild ID
echo $GUILD_ID  # 或查看 .env 文件

# 3. 检查网络连接
ping discord.com

# 4. 检查Go模块
go mod verify
```

#### 6.2 机器人无响应

**调试步骤：**
```bash
# 1. 查看日志输出
# 寻找错误信息和警告

# 2. 检查机器人权限
# 确保机器人有以下权限：
# - Send Messages
# - Manage Channels  
# - Connect
# - Speak
# - Move Members

# 3. 验证服务器ID
# 确保GUILD_ID与你测试的服务器一致
```

#### 6.3 语音频道创建失败

**可能原因：**
```bash
# 1. 权限不足
# 检查机器人是否有"管理频道"权限

# 2. 分类ID错误
# 如果设置了VOICE_CHANNEL_CATEGORY_ID，确保ID正确

# 3. 频道数量限制
# Discord服务器可能达到了频道数量上限
```

### 7. IDE调试配置

#### 7.1 VS Code 调试配置

创建 `.vscode/launch.json`：

```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Debug Discord Bot",
            "type": "go",
            "request": "launch",
            "mode": "auto",
            "program": "${workspaceFolder}/cmd/bot/main.go",
            "env": {
                "LOG_LEVEL": "debug",
                "DEVELOPMENT_MODE": "true"
            },
            "envFile": "${workspaceFolder}/.env"
        }
    ]
}
```

#### 7.2 GoLand 调试配置

1. 打开 "Run/Debug Configurations"
2. 创建新的 "Go Build" 配置
3. 设置以下参数：
   - **Package path**: `cmd/bot`
   - **Working directory**: 项目根目录
   - **Environment variables**: 从 `.env` 文件加载

### 8. 单元测试调试

```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./internal/parser
go test ./internal/team

# 运行测试并查看详细输出
go test -v ./...

# 运行测试并生成覆盖率报告
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### 9. 调试最佳实践

#### 9.1 日志级别使用

```go
// 调试时使用不同级别的日志
log.Debug("详细的调试信息")
log.Info("一般信息")
log.Warn("警告信息") 
log.Error("错误信息")
```

#### 9.2 断点调试

```go
// 在关键位置添加调试输出
fmt.Printf("Debug: variable value = %+v\n", variable)

// 或使用日志
log.WithFields(logrus.Fields{
    "user_id": userID,
    "message": message,
}).Debug("Processing LFG request")
```

### 10. 生产环境准备

调试完成后，准备部署：

```bash
# 1. 构建生产版本
make build

# 2. 运行测试确保一切正常
make test

# 3. 切换到生产配置(以模板为基础填入生产值)
cp config/consise.env.example .env
# 编辑 .env 填入生产环境的值

# 4. 使用生产模式运行
make run-prod
```

## 🆘 需要帮助？

如果遇到问题：

1. 查看日志输出寻找错误信息
2. 检查 [QUICK_START.md](QUICK_START.md) 获取更多信息
3. 验证Discord机器人配置和权限
4. 确保网络连接正常
5. 检查Go版本和依赖

**常用调试命令：**
```bash
# 查看进程
ps aux | grep discord

# 查看端口占用
lsof -i :8080

# 查看系统资源
top -p $(pgrep discord-lfg-bot)
```

---

**祝你调试愉快！** 🎯 