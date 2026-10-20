# CLAUDE.md

用 Go 编写的 Discord 寻组(LFG)机器人:解析寻组消息、管理临时语音频道,并提供签到积分、下注、抽奖、成就、LOL 段位排行榜/对局监控等功能。生产部署基于 docker-compose(bot + PostgreSQL 15 + Redis 8 + 每日备份容器)。

## 常用命令

```bash
go build ./...            # 编译全部包
make run                  # 开发模式运行 (LOG_LEVEL=debug go run ./cmd/bot)
go test ./...             # 全部测试;make test 等价 (-v)
make test-race            # 竞态检测
gofmt -l .                # 格式检查;make fmt 执行 go fmt ./...
go vet ./...              # 静态检查
staticcheck ./...         # 本机已安装;make lint 需要 golangci-lint(本机未装)
make db-migrate           # 运行迁移 (go run ./cmd/migrate up);migrate status 查看状态
docker compose up -d --build   # 整套服务;Postgres 宿主机端口 5433,Redis 6380
```

修代码前先跑一遍 `go test ./...` 确认基线。staticcheck 目前仅剩 4 条有意保留的 ST1005("Riot" 专有名词开头的错误串,改动会影响用户可见文案)。
健康检查:bot 在 `HEALTH_CHECK_PORT`(默认 8080)提供 `/health`(校验数据库连通);`discord-lfg-bot -health-check` 是容器探针模式,Docker HEALTHCHECK 依赖它(scratch 镜像内无 curl/shell,勿改回 curl 探测)。

## 入口与目录职责

- `cmd/bot` — 主程序;`cmd/migrate` — 迁移工具;`cmd/test_api_key` — 验证 Riot API Key 的手工调试工具
- `internal/bot` — 生命周期与事件分发:模块装配集中在 `wiring.go`(buildModules),slash 命令/按钮/模态框路由在 `router.go` 的 registerRoutes 注册表,slash 命令注册在 `commands_registry.go`;**新增业务模块 = wiring 加装配 + 两张注册表各加一行**,不要在分发逻辑里加 switch 分支
- `internal/parser` — 寻组消息解析("LOL 4=1" 等格式),有测试
- `internal/team`、`internal/channel`、`internal/temproom` — 团队状态与临时语音频道生命周期
- `internal/points`、`internal/betting`、`internal/lottery`、`internal/achievement` — 虚拟积分体系(签到/下注/抽奖/成就),涉及余额增减的逻辑必须走数据库事务
- `internal/rank`、`internal/riotwatch`、`internal/lol` — Riot API:段位绑定与排行榜、对局状态轮询、内战
- `internal/protrack` — `/trackpro` 职业选手追踪:`pro_accounts` 选手账号库(迁移 007),按 `PRO_TRACK_INTERVAL` 轮询 spectator,单双排开局/结束播报;复用 wiring 里的 riotClient
- `internal/database` — sqlx + lib/pq 的仓储层与 Redis 客户端;模型在 `models.go`
- `internal/config` — 环境变量加载校验;游戏与频道的映射只来自 `CHANNEL_MAPPINGS` 环境变量(格式 `游戏名:文字频道ID:语音分类ID`,逗号分隔),没有独立的游戏配置文件
- `pkg/logger` — logrus 封装,全局单例
- `sql/init/`(001-006 + zzz 基线登记)为完整 schema,由 Postgres 容器首次启动自动全量执行,部署无需迁移步骤;`sql/migrations/` 只放上线后的增量变更(007 起,经 `cmd/migrate` 执行)
- Go 1.25(`go.mod` 的 go 指令与 `docker/Dockerfile` 的 `golang:1.25-alpine` 必须保持一致)

## 配置与敏感信息

- 运行配置来自 `.env`(godotenv 加载,gitignored)+ docker-compose 的 environment 注入;变量清单见 `docker-compose.yml` 与 `internal/config/config.go`
- 敏感变量:`DISCORD_TOKEN`、`RIOT_API_KEY`、`POSTGRES_PASSWORD`、`REDIS_PASSWORD`、含凭据的 `DATABASE_URL`/`REDIS_URL`
- **禁止**把真实凭据写进任何被 git 跟踪的文件(历史上 `config/development.env` 泄露过真实 token,已要求轮换;该文件与 `production.env` 现已 gitignore,唯一入库的模板是 `config/consise.env.example`);禁止在日志中输出 token/密码/完整连接串
- 所有入口(bot/migrate/example/Makefile db-health)统一从根目录 `.env` 加载配置,不要再引用 `config/*.env`
- `backups/`、`scripts/bindings.csv` 含用户数据,已 gitignore,不要提交

## 编码约定(与现有代码保持一致)

- 注释、日志、Discord 面向用户的文案均为中文;新代码保持一致
- 错误处理:`fmt.Errorf("描述: %w", err)` 包装并携带上下文;不吞错,不重复打日志又返回
- 数据库访问一律经 `internal/database` 仓储方法并传递 `context.Context`;跨表的积分变动用事务
- 后台任务(scheduler/poller)必须支持 Stop:ticker 要 `defer ticker.Stop()`,goroutine 监听退出通道或 ctx.Done()
- Discord interaction 处理器内不要做长时间阻塞操作;对 `InteractionData` 做类型断言必须用 `v, ok :=` 形式防 panic
- Riot API 调用统一走 `internal/riotwatch`/`internal/rank` 的 client(带 X-Riot-Token header),不要新建裸 http.Get
- 提交信息用 Conventional Commits,scope 为模块名,如 `fix(betting): ...`(见 `.cursor/rules/git_commit.mdc`)

## 谨慎操作

- 不要改数据库表结构而不加迁移文件;迁移只增不改历史文件
- 不要变更用户可见行为(命令名、积分公式、赔率/对冲逻辑、文案格式)除非明确要求——积分公式等业务规则以代码现状为准
- 构建产物(`/bot`、`/migrate`、`bin/`)已 gitignore,不要提交;旧数据备份统一放 `backups/`(gitignored)
- 部署/恢复流程见 `REDEPLOY.md`;文档与代码冲突时以代码为准
