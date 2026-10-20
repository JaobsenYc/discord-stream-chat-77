# Discord LFG Bot Makefile
# 版本: 1.0.0

# 变量定义
BINARY_NAME=discord-lfg-bot
BINARY_PATH=bin/$(BINARY_NAME)
GO_VERSION=1.25
DOCKER_IMAGE=$(BINARY_NAME):latest
DOCKER_COMPOSE_PATH=docker-compose.yml

# Go 相关变量
GOBASE=$(shell pwd)
GOBIN=$(GOBASE)/bin
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# 构建信息
BUILD_TIME=$(shell date +%Y-%m-%d\ %H:%M:%S)
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# 构建标志
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)"

# 默认目标
.DEFAULT_GOAL := help

# 帮助信息
.PHONY: help
help: ## 显示帮助信息
	@echo "Discord LFG Bot - 可用命令:"
	@echo ""
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ""

# 环境检查
.PHONY: check
check: ## 检查开发环境
	@echo "🔍 检查开发环境..."
	@which go > /dev/null || (echo "❌ Go 未安装" && exit 1)
	@echo "✅ Go 版本: $(shell go version)"
	@which docker > /dev/null || echo "⚠️  Docker 未安装"
	@which docker-compose > /dev/null || echo "⚠️  Docker Compose 未安装"
	@test -f .env || (echo "⚠️  .env 文件不存在，请复制 config/consise.env.example" && exit 1)
	@echo "✅ 环境检查完成"

# 依赖管理
.PHONY: deps
deps: ## 安装和更新依赖
	@echo "📦 安装 Go 依赖..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "✅ 依赖安装完成"

.PHONY: deps-update
deps-update: ## 更新依赖到最新版本
	@echo "🔄 更新依赖..."
	$(GOGET) -u ./...
	$(GOMOD) tidy
	@echo "✅ 依赖更新完成"

# 构建相关
.PHONY: build
build: deps ## 构建应用程序
	@echo "🔨 构建应用程序..."
	@mkdir -p $(GOBIN)
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_PATH) ./cmd/bot
	@echo "✅ 构建完成: $(BINARY_PATH)"

.PHONY: build-linux
build-linux: deps ## 构建 Linux 版本
	@echo "🔨 构建 Linux 版本..."
	@mkdir -p $(GOBIN)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_PATH)-linux ./cmd/bot
	@echo "✅ Linux 版本构建完成: $(BINARY_PATH)-linux"

.PHONY: build-windows
build-windows: deps ## 构建 Windows 版本
	@echo "🔨 构建 Windows 版本..."
	@mkdir -p $(GOBIN)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_PATH).exe ./cmd/bot
	@echo "✅ Windows 版本构建完成: $(BINARY_PATH).exe"

.PHONY: build-all
build-all: build build-linux build-windows ## 构建所有平台版本

# 运行相关
.PHONY: run
run: ## 运行应用程序 (开发模式)
	@echo "🚀 启动开发模式..."
	@export LOG_LEVEL=debug && $(GOCMD) run ./cmd/bot

.PHONY: run-prod
run-prod: build ## 运行应用程序 (生产模式)
	@echo "🚀 启动生产模式..."
	./$(BINARY_PATH)

# 测试相关
.PHONY: test
test: ## 运行所有测试
	@echo "🧪 运行测试..."
	$(GOTEST) -v ./...

.PHONY: test-coverage
test-coverage: ## 运行测试并生成覆盖率报告
	@echo "🧪 运行测试覆盖率..."
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "✅ 覆盖率报告生成: coverage.html"

.PHONY: test-race
test-race: ## 运行竞态条件检测
	@echo "🧪 运行竞态条件检测..."
	$(GOTEST) -race ./...

.PHONY: bench
bench: ## 运行基准测试
	@echo "🧪 运行基准测试..."
	$(GOTEST) -bench=. -benchmem ./...

# 代码质量
.PHONY: fmt
fmt: ## 格式化代码
	@echo "🎨 格式化代码..."
	$(GOCMD) fmt ./...
	@echo "✅ 代码格式化完成"

.PHONY: lint
lint: ## 运行代码检查
	@echo "🔍 运行代码检查..."
	@which golangci-lint > /dev/null || (echo "请安装 golangci-lint" && exit 1)
	golangci-lint run ./...
	@echo "✅ 代码检查完成"

.PHONY: vet
vet: ## 运行 go vet
	@echo "🔍 运行 go vet..."
	$(GOCMD) vet ./...
	@echo "✅ go vet 检查完成"

.PHONY: security
security: ## 运行安全检查
	@echo "🔒 运行安全检查..."
	@which gosec > /dev/null || (echo "请安装 gosec: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest" && exit 1)
	gosec ./...
	@echo "✅ 安全检查完成"

# Docker 相关
.PHONY: docker-build
docker-build: ## 构建 Docker 镜像
	@echo "🐳 构建 Docker 镜像..."
	docker build -f docker/Dockerfile -t $(DOCKER_IMAGE) .
	@echo "✅ Docker 镜像构建完成: $(DOCKER_IMAGE)"

.PHONY: docker-run
docker-run: ## 运行 Docker 容器
	@echo "🐳 运行 Docker 容器..."
	docker run --rm --env-file .env $(DOCKER_IMAGE)

.PHONY: docker-compose-up
docker-compose-up: ## 使用 Docker Compose 启动服务
	@echo "🐳 启动 Docker Compose 服务..."
	docker compose -f $(DOCKER_COMPOSE_PATH) up -d
	@echo "✅ 服务已启动"

.PHONY: docker-compose-down
docker-compose-down: ## 停止 Docker Compose 服务
	@echo "🐳 停止 Docker Compose 服务..."
	docker compose -f $(DOCKER_COMPOSE_PATH) down
	@echo "✅ 服务已停止"

.PHONY: docker-compose-logs
docker-compose-logs: ## 查看 Docker Compose 日志
	docker compose -f $(DOCKER_COMPOSE_PATH) logs -f

# 清理
.PHONY: clean
clean: ## 清理构建文件
	@echo "🧹 清理构建文件..."
	$(GOCLEAN)
	rm -rf $(GOBIN)
	rm -f coverage.out coverage.html
	@echo "✅ 清理完成"

.PHONY: clean-docker
clean-docker: ## 清理 Docker 资源
	@echo "🧹 清理 Docker 资源..."
	docker system prune -f
	@echo "✅ Docker 清理完成"

# 开发工具
.PHONY: install-tools
install-tools: ## 安装开发工具
	@echo "🔧 安装开发工具..."
	$(GOGET) github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOGET) github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	@echo "✅ 开发工具安装完成"

.PHONY: mod-verify
mod-verify: ## 验证依赖
	@echo "🔍 验证依赖..."
	$(GOMOD) verify
	@echo "✅ 依赖验证完成"

# 部署相关
.PHONY: deploy
deploy: build-linux docker-build ## 准备部署 (构建 Linux 版本和 Docker 镜像)
	@echo "🚀 部署准备完成"

# 项目信息
.PHONY: info
info: ## 显示项目信息
	@echo "项目信息:"
	@echo "  名称: $(BINARY_NAME)"
	@echo "  版本: $(VERSION)"
	@echo "  构建时间: $(BUILD_TIME)"
	@echo "  Git 提交: $(GIT_COMMIT)"
	@echo "  Go 版本: $(shell go version)"

# 数据库相关
.PHONY: db-migrate
db-migrate: ## 运行数据库迁移
	@echo "🗄️  运行数据库迁移..."
	$(GOCMD) run ./cmd/migrate up
	@echo "✅ 数据库迁移完成"

.PHONY: db-migrate-down
db-migrate-down: ## 回滚数据库迁移 (需要指定步数: make db-migrate-down STEPS=1)
	@echo "🗄️  回滚数据库迁移..."
	$(GOCMD) run ./cmd/migrate down $(STEPS)
	@echo "✅ 数据库回滚完成"

.PHONY: db-status
db-status: ## 查看数据库迁移状态
	@echo "🗄️  查看数据库迁移状态..."
	$(GOCMD) run ./cmd/migrate status

.PHONY: db-create-migration
db-create-migration: ## 创建新的数据库迁移 (需要指定名称: make db-create-migration NAME="add_new_table")
	@echo "🗄️  创建数据库迁移..."
	$(GOCMD) run ./cmd/migrate create "$(NAME)"
	@echo "✅ 迁移文件创建完成"

.PHONY: db-health
db-health: ## 检查数据库健康状态
	@echo "🗄️  检查数据库健康状态..."
	@$(GOCMD) run -c 'package main; import ("fmt"; "os"; "github.com/jaobsenyc/discord-lfg-bot-go/internal/database"; "github.com/joho/godotenv"); func main() { godotenv.Load(); cfg := &database.Config{PostgresURL: os.Getenv("DATABASE_URL"), RedisURL: os.Getenv("REDIS_URL"), RedisPassword: os.Getenv("REDIS_PASSWORD")}; db, err := database.New(cfg); if err != nil { fmt.Printf("❌ 数据库连接失败: %v\n", err); os.Exit(1) }; defer db.Close(); checker := database.NewHealthChecker(db); status := checker.CheckHealth(); if status.Healthy { fmt.Println("✅ 数据库健康状态良好") } else { fmt.Println("❌ 数据库健康状态异常") }; for service, info := range status.Services { fmt.Printf("  %s: %s (延迟: %v)\n", service, info.Status, info.Latency) } }'

.PHONY: db-setup
db-setup: ## 设置数据库 (启动 Docker 服务并运行迁移)
	@echo "🗄️  设置数据库..."
	@make docker-compose-up
	@echo "⏳ 等待数据库启动..."
	@sleep 10
	@make db-migrate
	@echo "✅ 数据库设置完成"

# Docker Compose 增强命令
.PHONY: docker-setup
docker-setup: ## Docker Compose 快速设置
	@echo "🐳 运行 Docker Compose 快速设置..."
	@./scripts/docker-setup.sh

.PHONY: docker-up
docker-up: ## 启动所有 Docker 服务
	@echo "🐳 启动 Docker 服务..."
	@docker-compose up -d

.PHONY: docker-down
docker-down: ## 停止所有 Docker 服务
	@echo "🐳 停止 Docker 服务..."
	@docker-compose down

.PHONY: docker-restart
docker-restart: ## 重启 Docker 服务
	@echo "🐳 重启 Docker 服务..."
	@docker-compose restart

.PHONY: docker-status
docker-status: ## 查看 Docker 服务状态
	@echo "🐳 Docker 服务状态:"
	@docker-compose ps

.PHONY: docker-logs
docker-logs: ## 查看 Docker 服务日志
	@echo "🐳 查看所有服务日志:"
	@docker-compose logs

.PHONY: docker-logs-bot
docker-logs-bot: ## 查看 Bot 日志
	@echo "🐳 查看 Bot 日志:"
	@docker-compose logs -f lfg-bot

.PHONY: docker-logs-db
docker-logs-db: ## 查看数据库日志
	@echo "🐳 查看数据库日志:"
	@docker-compose logs postgres redis

.PHONY: docker-shell
docker-shell: ## 进入 Bot 容器
	@echo "🐳 进入 Bot 容器..."
	@docker-compose exec lfg-bot /bin/sh

.PHONY: docker-db-shell
docker-db-shell: ## 进入数据库容器
	@echo "🐳 进入 PostgreSQL 容器..."
	@docker-compose exec postgres psql -U lfguser -d lfgbot

.PHONY: docker-redis-shell
docker-redis-shell: ## 进入 Redis 容器
	@echo "🐳 进入 Redis 容器..."
	@docker-compose exec redis redis-cli -a $$(grep REDIS_PASSWORD .env | cut -d '=' -f2)

.PHONY: docker-migrate
docker-migrate: ## 在 Docker 容器中运行数据库迁移
	@echo "🐳 在 Docker 容器中运行数据库迁移..."
	@docker-compose exec lfg-bot ./migrate up

.PHONY: docker-db-health
docker-db-health: ## 检查 Docker 数据库健康状态
	@echo "🐳 检查 Docker 数据库健康状态..."
	@docker-compose exec lfg-bot ./migrate health

.PHONY: docker-clean
docker-clean: ## 清理 Docker 资源
	@echo "🐳 清理 Docker 资源..."
	@docker-compose down -v
	@docker system prune -f

.PHONY: docker-rebuild
docker-rebuild: ## 重建 Docker 镜像
	@echo "🐳 重建 Docker 镜像..."
	@docker-compose build --no-cache
	@docker-compose up -d

# 快速开始
.PHONY: quick-start
quick-start: check deps test build ## 快速开始 (检查、安装依赖、测试、构建)
	@echo "🎉 快速开始完成!"
	@echo "💡 运行 'make run' 启动开发模式"
	@echo "💡 运行 'make docker-setup' 使用 Docker 快速部署"
	@echo "💡 运行 'make db-setup' 设置数据库"

.PHONY: quick-start-docker
quick-start-docker: docker-setup ## Docker 快速开始
	@echo "🎉 Docker 快速开始完成!"
	@echo "💡 运行 'make docker-logs-bot' 查看 Bot 日志"
	@echo "💡 运行 'make docker-status' 查看服务状态" 