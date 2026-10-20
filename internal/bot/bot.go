package bot

import (
	"context"
	"fmt"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/config"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/database"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// Bot Discord LFG机器人:负责生命周期管理与事件分发,
// 业务模块的装配见 wiring.go,交互路由见 router.go
type Bot struct {
	session    *discordgo.Session
	config     *config.Config
	repository *database.Repository

	// 全部业务模块(嵌入,直接以 b.xxx 访问字段)
	*modules

	// 交互路由表
	router *interactionRouter

	// 用于优雅关闭
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New 创建新的机器人实例
func New(cfg *config.Config, repo *database.Repository) (*Bot, error) {
	// 创建Discord会话
	session, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		return nil, fmt.Errorf("创建Discord会话失败: %w", err)
	}

	// 设置Intent
	session.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsGuildVoiceStates |
		discordgo.IntentsMessageContent

	ctx, cancel := context.WithCancel(context.Background())

	bot := &Bot{
		session:    session,
		config:     cfg,
		repository: repo,
		modules:    buildModules(cfg, repo, session),
		ctx:        ctx,
		cancel:     cancel,
	}

	// 登记交互路由并注册事件处理器
	bot.registerRoutes()
	bot.registerHandlers()

	return bot, nil
}

// Start 启动机器人
func (b *Bot) Start() error {
	logger.Info("正在连接到Discord...")

	if err := b.session.Open(); err != nil {
		return fmt.Errorf("连接Discord失败: %w", err)
	}

	logger.Info("Discord连接成功，机器人已上线")

	// 验证配置的频道是否存在和可访问
	if err := b.validateChannels(); err != nil {
		logger.Warnf("频道验证失败: %v", err)
		// 不要因为频道验证失败而停止启动，只记录警告
	}

	// 如果是开发环境，并且配置了启动时清理文字频道消息
	if b.config.DevelopmentMode && b.config.CleanupTextChannelsOnStart {
		logger.Info("开发环境模式：清空文字频道消息")
		if err := b.cleanupTextChannels(); err != nil {
			logger.Warnf("清空文字频道消息失败: %v", err)
		}
	}

	// 如果是开发环境，并且配置了启动时清理语音频道
	if b.config.DevelopmentMode && b.config.CleanupVoiceChannelsOnStart {
		logger.Info("开发环境模式：清空语音频道")
		if err := b.cleanupVoiceChannels(); err != nil {
			logger.Warnf("清空语音频道失败: %v", err)
		}
	}

	// 启动临时房间管理器
	if b.config.TempRoomEnabled {
		if err := b.tempRoomManager.Start(); err != nil {
			return fmt.Errorf("启动临时房间管理器失败: %w", err)
		}
		logger.Info("临时房间功能已启用")
	}

	// 启动 Bot 自身的后台周期任务(频道清理/语音会话清理/抽奖过期检查)
	b.startBackgroundTasks()

	// 启动竞猜监控器
	b.bettingMonitor.Start()
	logger.Info("竞猜监控器已启动")

	// 启动 Riot Watch 监控器（如果已启用）
	if b.riotWatchMonitor != nil {
		b.riotWatchMonitor.Start()
		logger.Info("Riot Watch 监控器已启动")
	}

	// 启动 Rank 绑定系统调度器（如果已启用）
	if b.rankScheduler != nil {
		b.rankScheduler.Start()
		logger.Info("Rank 绑定系统调度器已启动")
	}

	// 启动职业选手追踪器（如果已启用）
	if b.proTracker != nil {
		b.proTracker.Start()
		logger.Infof("职业选手追踪器已启动，轮询间隔 %v", b.config.ProTrackInterval)
	}

	return nil
}

// Shutdown 优雅关闭机器人:停止全部后台任务并等待退出,再关闭 Discord 连接;
// ctx 到期仍未退出则放弃等待(由 main 传入超时上下文)
func (b *Bot) Shutdown(ctx context.Context) error {
	logger.Info("正在关闭机器人...")

	// 停止临时房间管理器
	if b.config.TempRoomEnabled {
		b.tempRoomManager.Stop()
	}

	// 停止 Rank 绑定系统调度器
	if b.rankScheduler != nil {
		b.rankScheduler.Stop()
	}

	// 停止职业选手追踪器
	if b.proTracker != nil {
		b.proTracker.Stop()
	}

	// 停止 Riot Watch 监控器（否则轮询 goroutine 会在会话关闭后继续访问数据库和 API）
	if b.riotWatchMonitor != nil {
		b.riotWatchMonitor.Stop()
	}

	// 停止竞猜监控器
	b.bettingMonitor.Stop()

	// 取消上下文并等待 Bot 自身的后台任务退出
	b.cancel()
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		logger.Warn("等待后台任务退出超时，强制关闭")
	}

	// 关闭Discord连接
	if err := b.session.Close(); err != nil {
		return fmt.Errorf("关闭Discord连接失败: %w", err)
	}

	logger.Info("机器人已安全关闭")
	return nil
}

// registerHandlers 注册 Discord 事件处理器
func (b *Bot) registerHandlers() {
	b.session.AddHandler(b.handleReady)
	b.session.AddHandler(b.handleMessageCreate)
	b.session.AddHandler(b.handleVoiceStateUpdate)
	b.session.AddHandler(b.handleInteractionCreate)
	b.session.AddHandler(b.handleLegacyTextCommand)
}

// handleReady 处理机器人就绪事件:注册全部 slash 命令并设置状态
func (b *Bot) handleReady(s *discordgo.Session, event *discordgo.Ready) {
	logger.Infof("机器人 %s 已就绪", event.User.Username)

	if b.repository != nil {
		for _, reg := range b.commandRegistrars() {
			if err := reg.register(s); err != nil {
				logger.Errorf("注册%s失败: %v", reg.name, err)
			} else {
				logger.Infof("成功注册%s", reg.name)
			}
		}
	}

	// 设置机器人状态
	err := s.UpdateGameStatus(0, "🎮 寻组助手 | 发送游戏名 人数=当前 开始寻组")
	if err != nil {
		logger.Errorf("设置机器人状态失败: %v", err)
	}
}
