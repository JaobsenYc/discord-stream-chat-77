package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jaobsenyc/discord-lfg-bot-go/internal/bot"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/config"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/database"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
	"github.com/joho/godotenv"
)

// runHealthCheckProbe 探针模式：请求本机健康检查端点后按结果退出。
// 供 Docker HEALTHCHECK 使用（scratch 镜像内没有 curl/shell）。
func runHealthCheckProbe() {
	port := os.Getenv("HEALTH_CHECK_PORT")
	if port == "" {
		port = "8080"
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%s/health", port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "health check failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "health check failed: status %d\n", resp.StatusCode)
		os.Exit(1)
	}
	os.Exit(0)
}

// startHealthServer 启动健康检查 HTTP 服务，/health 检查数据库连通性
func startHealthServer(port string, repository *database.Repository) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := repository.Ping(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "unhealthy: %v", err)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("健康检查服务异常退出: %v", err)
		}
	}()
	logger.Infof("健康检查服务已启动: :%s/health", port)
	return srv
}

func main() {
	// 探针模式：docker HEALTHCHECK 调用 `discord-lfg-bot -health-check`
	if len(os.Args) > 1 && (os.Args[1] == "-health-check" || os.Args[1] == "--health-check") {
		runHealthCheckProbe()
	}

	// 加载环境变量
	if err := godotenv.Load(); err != nil {
		logger.Warn("未找到 .env 文件，使用系统环境变量")
	}

	// 初始化配置
	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("配置加载失败: %v", err)
	}

	// 初始化日志器
	logger.Init(cfg.LogLevel)
	logger.Info("正在启动 Discord LFG Bot...")

	// 初始化数据库连接
	dbConfig := &database.Config{
		PostgresURL:     os.Getenv("DATABASE_URL"),
		RedisURL:        os.Getenv("REDIS_URL"),
		RedisPassword:   os.Getenv("REDIS_PASSWORD"),
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	}

	db, err := database.New(dbConfig)
	if err != nil {
		logger.Fatalf("数据库连接失败: %v", err)
	}
	defer db.Close()

	// 创建数据库仓库
	repository := database.NewRepository(db)

	// 测试数据库连接
	if err := repository.Ping(); err != nil {
		logger.Fatalf("数据库连接测试失败: %v", err)
	}
	logger.Info("数据库连接成功")

	// 启动健康检查 HTTP 服务（供 Docker HEALTHCHECK / 运维探测）
	healthServer := startHealthServer(cfg.HealthCheckPort, repository)

	// 初始化机器人
	lfgBot, err := bot.New(cfg, repository)
	if err != nil {
		logger.Fatalf("机器人初始化失败: %v", err)
	}

	// 启动机器人(后台任务生命周期由 Bot 内部统一管理，Shutdown 时取消并等待)
	if err := lfgBot.Start(); err != nil {
		logger.Fatalf("机器人启动失败: %v", err)
	}

	logger.Info("Discord LFG Bot 已启动并运行中...")

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 阻塞直到收到信号
	<-sigChan
	logger.Info("收到关闭信号，正在优雅关闭...")

	// 给机器人一些时间来清理资源
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := lfgBot.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("机器人关闭时出错: %v", err)
	}

	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("健康检查服务关闭时出错: %v", err)
	}

	logger.Info("Discord LFG Bot 已安全关闭")
}
