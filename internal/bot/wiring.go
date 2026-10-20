package bot

import (
	"github.com/bwmarrin/discordgo"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/achievement"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/betting"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/channel"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/config"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/database"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/lol"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/lottery"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/points"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/protrack"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/rank"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/riotwatch"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/role"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/team"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/temproom"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// modules 聚合所有业务模块的服务、命令处理器与后台任务,
// 由 buildModules 统一装配;Bot 通过嵌入该结构体直接访问各字段。
// 新增业务模块时只需在这里加字段并在 buildModules 中装配。
type modules struct {
	roleManager     *role.Manager
	teamManager     *team.Manager
	channelManager  *channel.Manager
	tempRoomManager *temproom.Manager

	achievementService  *achievement.Service
	achievementCommands *achievement.CommandHandler
	lotteryCommands     *lottery.CommandHandler
	channelCommands     *channel.CommandHandler

	// LOL内战系统
	lolManager  *lol.Manager
	lolCommands *lol.CommandHandler

	// 竞猜系统
	bettingService  *betting.Service
	bettingCommands *betting.CommandHandler
	bettingMonitor  *betting.Monitor

	// 积分系统
	pointsCommands *points.CommandHandler

	// Riot Watch 自动竞猜系统(未配置 API Key 时为 nil)
	riotWatchService  *riotwatch.Service
	riotWatchCommands *riotwatch.CommandHandler
	riotWatchMonitor  *riotwatch.Monitor

	// Rank 绑定系统(未配置 API Key 时为 nil)
	rankService   *rank.Service
	rankCommands  *rank.CommandHandler
	rankScheduler *rank.Scheduler

	// 职业选手追踪(未配置 API Key 时为 nil)
	proTrackCommands *protrack.CommandHandler
	proTracker       *protrack.Tracker
}

// buildModules 装配全部业务模块(依赖注入集中在此,与事件处理逻辑分离)
func buildModules(cfg *config.Config, repo *database.Repository, session *discordgo.Session) *modules {
	m := &modules{}

	// 基础管理器
	m.roleManager = role.NewManager(session, cfg.GuildID, cfg)
	m.teamManager = team.NewManager()
	m.channelManager = channel.NewManager(session, cfg.GuildID, cfg)
	m.channelCommands = channel.NewCommandHandler(m.channelManager, m.teamManager, m.roleManager, cfg)
	m.tempRoomManager = temproom.NewManager(session, cfg.GuildID, cfg.TempRoomTriggerChannel,
		cfg.TempRoomCategoryID, cfg.TempRoomNameTemplate, cfg.TempRoomCleanupDelay)

	// 成就与抽奖
	m.achievementService = achievement.NewService(repo.DB())
	m.achievementCommands = achievement.NewCommandHandler(m.achievementService, repo.DB())
	m.lotteryCommands = lottery.NewCommandHandlerWithSession(repo.DB(), m.achievementService, session)

	// LOL内战系统
	m.lolManager = lol.NewManager(session, cfg.GuildID, cfg.GetLOLCustomCategoryID(), cfg.GetLOLCustomCleanupDelay())
	m.lolCommands = lol.NewCommandHandler(m.lolManager, session, cfg.GuildID, cfg)

	// 竞猜系统
	m.bettingService = betting.NewService(repo.DB(), session)
	m.bettingCommands = betting.NewCommandHandler(m.bettingService, session, cfg.GuildID, database.NewUserRepository(repo.DB()))
	m.bettingMonitor = betting.NewMonitor(m.bettingService, session, cfg.GuildID)

	// 积分系统
	m.pointsCommands = points.NewCommandHandler(repo.DB(), session, cfg.GuildID)

	// Riot Watch 与 Rank 绑定系统(依赖 Riot API Key,未配置则禁用)
	if cfg.RiotAPIKey != "" {
		riotClient := riotwatch.NewRiotAPIClient(cfg.RiotAPIKey, cfg.RiotRegion, cfg.RiotPlatform)
		m.riotWatchService = riotwatch.NewService(repo.DB(), riotClient, m.bettingService, m.bettingCommands, session)
		m.riotWatchCommands = riotwatch.NewCommandHandler(m.riotWatchService, session, cfg.GuildID, m.roleManager, repo.DB())
		m.riotWatchMonitor = riotwatch.NewMonitor(m.riotWatchService, cfg.GuildID)
		logger.Info("Riot Watch 系统已初始化")

		// Rank 绑定系统复用同一个 riotClient
		m.rankService = rank.NewService(repo.DB(), riotClient)
		m.rankCommands = rank.NewCommandHandler(m.rankService, session, cfg.GuildID, cfg.RiotRegion)
		m.rankScheduler = rank.NewScheduler(m.rankService, m.rankCommands, session, cfg.RankLeaderboardChannelID)
		logger.Info("Rank 绑定系统已初始化")

		// 职业选手追踪同样复用 riotClient
		proTrackService := protrack.NewService(repo.DB(), riotClient, cfg.ProTrackMax)
		m.proTrackCommands = protrack.NewCommandHandler(proTrackService, session, cfg.GuildID, m.roleManager, cfg.ProTrackChannelID, cfg.RiotRegion)
		m.proTracker = protrack.NewTracker(proTrackService, session, cfg.ProTrackInterval, cfg.ProTrackMentionRoleID, cfg.RiotRegion)
		logger.Info("职业选手追踪已初始化")
	} else {
		logger.Warn("未配置 RIOT_API_KEY，Riot Watch、Rank 绑定和职业选手追踪功能禁用")
	}

	return m
}
