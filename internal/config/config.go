package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// ChannelMapping 频道映射配置
type ChannelMapping struct {
	TextChannelID   string // 监听的文字频道ID
	VoiceCategoryID string // 创建语音频道的分类ID
	GameName        string // 游戏名称
}

// Config 包含应用程序的所有配置
type Config struct {
	// Discord配置
	DiscordToken string
	GuildID      string
	BotPrefix    string

	// 日志配置
	LogLevel string

	// 频道映射配置
	ChannelMappings       []ChannelMapping // 文字频道到语音分类的映射
	DefaultTextChannelID  string           // 默认文字频道ID
	DefaultCategoryID     string           // 默认分类ID
	ChannelCleanupTimeout time.Duration
	MaxTeamSize           int
	MinTeamSize           int

	// 开发环境配置
	DevelopmentMode             bool // 是否为开发环境
	CleanupTextChannelsOnStart  bool // 启动时是否清理文字频道消息
	CleanupVoiceChannelsOnStart bool // 启动时是否清理语音频道

	// 临时语音房间配置
	TempRoomEnabled        bool          // 是否启用临时房间功能
	TempRoomTriggerChannel string        // 触发创建临时房间的入口频道ID
	TempRoomCategoryID     string        // 临时房间所在的分类ID
	TempRoomCleanupDelay   time.Duration // 房间空闲后的清理延迟时间
	TempRoomNameTemplate   string        // 房间名称模板

	// 管理员配置
	AdminUserIDs    []string // 管理员用户ID列表
	AdminChannelIDs []string // 允许使用管理员命令的频道ID列表

	// LOL内战配置
	LOLCustomEnabled      bool          // 是否启用LOL内战功能
	LOLCustomChannelIDs   []string      // 允许使用LOL内战命令的频道ID列表 (空则允许所有频道)
	LOLCustomCategoryID   string        // LOL内战语音频道专用分类ID
	LOLCustomCleanupDelay time.Duration // LOL内战语音频道清理延迟时间 (默认30分钟)

	// Riot API 配置
	RiotAPIKey   string // Riot API Key
	RiotRegion   string // 游戏区域 (na1, euw1, kr, etc.)
	RiotPlatform string // 平台路由 (americas, europe, asia)

	// Rank 绑定系统配置
	RankLeaderboardChannelID string // 段位排行榜定时发送的频道ID

	// 职业选手追踪配置
	ProTrackChannelID     string        // 限定 /trackpro 只能在该频道使用(为空则不限制)
	ProTrackMentionRoleID string        // 对局结束播报时 @ 的身份组ID(为空则不 @)
	ProTrackMax           int           // 同时追踪的账号数上限 (默认10,个人 key 每 2 分钟仅 100 次请求)
	ProTrackInterval      time.Duration // 对局状态轮询间隔 (默认20秒)

	// 健康检查
	HealthCheckPort string
}

// GetMappingByTextChannel 根据文字频道ID获取映射配置
func (c *Config) GetMappingByTextChannel(textChannelID string) *ChannelMapping {
	for _, mapping := range c.ChannelMappings {
		if mapping.TextChannelID == textChannelID {
			return &mapping
		}
	}
	return nil
}

// IsMonitoredChannel 检查是否是被监听的频道
func (c *Config) IsMonitoredChannel(channelID string) bool {
	// 检查是否在频道映射中
	if c.GetMappingByTextChannel(channelID) != nil {
		return true
	}
	// 检查是否是默认文字频道
	return c.DefaultTextChannelID != "" && channelID == c.DefaultTextChannelID
}

// GetVoiceCategoryForTextChannel 根据文字频道ID获取对应的语音分类ID
func (c *Config) GetVoiceCategoryForTextChannel(textChannelID string) string {
	if mapping := c.GetMappingByTextChannel(textChannelID); mapping != nil {
		return mapping.VoiceCategoryID
	}
	return c.DefaultCategoryID
}

// GetGameNameForTextChannel 根据文字频道ID获取对应的游戏名称
func (c *Config) GetGameNameForTextChannel(textChannelID string) string {
	if mapping := c.GetMappingByTextChannel(textChannelID); mapping != nil {
		return mapping.GameName
	}
	// 如果是默认文字频道，返回通用游戏名称
	if c.DefaultTextChannelID != "" && textChannelID == c.DefaultTextChannelID {
		return "通用寻组"
	}
	return "未知游戏"
}

// GetAllMonitoredTextChannels 获取所有监听的文字频道ID，包括默认文字频道
func (c *Config) GetAllMonitoredTextChannels() []string {
	channels := make([]string, 0, len(c.ChannelMappings)+1)

	// 添加所有映射的文字频道
	for _, mapping := range c.ChannelMappings {
		channels = append(channels, mapping.TextChannelID)
	}

	// 添加默认文字频道（如果不在列表中）
	if c.DefaultTextChannelID != "" {
		found := false
		for _, ch := range channels {
			if ch == c.DefaultTextChannelID {
				found = true
				break
			}
		}
		if !found {
			channels = append(channels, c.DefaultTextChannelID)
		}
	}

	return channels
}

// GetAllMonitoredVoiceChannels 获取所有监听的语音分类ID，包括默认分类
func (c *Config) GetAllMonitoredVoiceChannels() []string {
	categories := make([]string, 0, len(c.ChannelMappings)+1)

	// 添加所有映射的语音分类
	for _, mapping := range c.ChannelMappings {
		categories = append(categories, mapping.VoiceCategoryID)
	}

	// 添加默认分类（如果不在列表中）
	if c.DefaultCategoryID != "" {
		found := false
		for _, cat := range categories {
			if cat == c.DefaultCategoryID {
				found = true
				break
			}
		}
		if !found {
			categories = append(categories, c.DefaultCategoryID)
		}
	}

	return categories
}

// IsLOLCustomEnabled 检查LOL内战功能是否启用
func (c *Config) IsLOLCustomEnabled() bool {
	return c.LOLCustomEnabled
}

// IsLOLCustomAllowedChannel 检查指定频道是否允许使用LOL内战功能
func (c *Config) IsLOLCustomAllowedChannel(channelID string) bool {
	// 如果功能未启用，直接返回false
	if !c.LOLCustomEnabled {
		return false
	}

	// 如果没有配置特定频道列表，则允许所有频道
	if len(c.LOLCustomChannelIDs) == 0 {
		return true
	}

	// 检查频道是否在允许列表中
	for _, allowedChannelID := range c.LOLCustomChannelIDs {
		if allowedChannelID == channelID {
			return true
		}
	}

	return false
}

// GetLOLCustomCategoryID 获取LOL内战语音频道分类ID
func (c *Config) GetLOLCustomCategoryID() string {
	if c.LOLCustomCategoryID != "" {
		return c.LOLCustomCategoryID
	}
	return c.DefaultCategoryID
}

// GetLOLCustomCleanupDelay 获取LOL内战语音频道清理延迟时间
func (c *Config) GetLOLCustomCleanupDelay() time.Duration {
	return c.LOLCustomCleanupDelay
}

// Load 从环境变量加载配置
func Load() (*Config, error) {
	cfg := &Config{
		// 默认值
		BotPrefix:             "!",
		LogLevel:              "info",
		ChannelCleanupTimeout: 30 * time.Minute,
		MaxTeamSize:           10,
		MinTeamSize:           2,
		HealthCheckPort:       "8080",
		ChannelMappings:       make([]ChannelMapping, 0),
	}

	// 必需的配置项
	cfg.DiscordToken = os.Getenv("DISCORD_TOKEN")
	if cfg.DiscordToken == "" {
		return nil, fmt.Errorf("DISCORD_TOKEN 环境变量是必需的")
	}
	fmt.Printf("DEBUG: DISCORD_TOKEN 已设置 (长度: %d)\n", len(cfg.DiscordToken))

	cfg.GuildID = os.Getenv("GUILD_ID")
	if cfg.GuildID == "" {
		return nil, fmt.Errorf("GUILD_ID 环境变量是必需的")
	}
	fmt.Printf("DEBUG: GUILD_ID = %s\n", cfg.GuildID)

	// 必需的频道映射配置
	channelMappingsStr := os.Getenv("CHANNEL_MAPPINGS")
	if channelMappingsStr == "" {
		return nil, fmt.Errorf("CHANNEL_MAPPINGS 环境变量是必需的")
	}
	fmt.Printf("DEBUG: CHANNEL_MAPPINGS = %s\n", channelMappingsStr)

	if err := cfg.parseChannelMappings(channelMappingsStr); err != nil {
		return nil, fmt.Errorf("解析频道映射配置失败: %w", err)
	}
	fmt.Printf("DEBUG: 解析到 %d 个频道映射\n", len(cfg.ChannelMappings))

	// 默认分类ID
	cfg.DefaultCategoryID = os.Getenv("DEFAULT_CATEGORY_ID")
	if cfg.DefaultCategoryID == "" {
		return nil, fmt.Errorf("DEFAULT_CATEGORY_ID 环境变量是必需的")
	}
	fmt.Printf("DEBUG: DEFAULT_CATEGORY_ID = %s\n", cfg.DefaultCategoryID)

	// 默认文字频道ID
	cfg.DefaultTextChannelID = os.Getenv("DEFAULT_TEXT_CHANNEL_ID")
	if cfg.DefaultTextChannelID == "" {
		return nil, fmt.Errorf("DEFAULT_TEXT_CHANNEL_ID 环境变量是必需的")
	}
	fmt.Printf("DEBUG: DEFAULT_TEXT_CHANNEL_ID = %s\n", cfg.DefaultTextChannelID)

	// 可选配置项
	if prefix := os.Getenv("BOT_PREFIX"); prefix != "" {
		cfg.BotPrefix = prefix
	}

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		cfg.LogLevel = logLevel
	}

	// 开发环境配置
	cfg.DevelopmentMode = os.Getenv("DEVELOPMENT_MODE") == "true"
	cfg.CleanupTextChannelsOnStart = os.Getenv("CLEANUP_TEXT_CHANNELS_ON_START") == "true"
	cfg.CleanupVoiceChannelsOnStart = os.Getenv("CLEANUP_VOICE_CHANNELS_ON_START") == "true"

	// 临时语音房间配置
	cfg.TempRoomEnabled = os.Getenv("TEMP_ROOM_ENABLED") == "true"
	cfg.TempRoomTriggerChannel = os.Getenv("TEMP_ROOM_TRIGGER_CHANNEL")
	cfg.TempRoomCategoryID = os.Getenv("TEMP_ROOM_CATEGORY_ID")
	cfg.TempRoomNameTemplate = os.Getenv("TEMP_ROOM_NAME_TEMPLATE")
	if cfg.TempRoomNameTemplate == "" {
		cfg.TempRoomNameTemplate = "临时房间-%s"
	}

	// 临时房间清理延迟
	if delayStr := os.Getenv("TEMP_ROOM_CLEANUP_DELAY"); delayStr != "" {
		if delay, err := time.ParseDuration(delayStr); err == nil {
			cfg.TempRoomCleanupDelay = delay
		} else {
			cfg.TempRoomCleanupDelay = 30 * time.Second // 默认30秒
		}
	} else {
		cfg.TempRoomCleanupDelay = 30 * time.Second
	}

	// 管理员配置
	if adminUserIDsStr := os.Getenv("ADMIN_USER_IDS"); adminUserIDsStr != "" {
		userIDs := strings.Split(adminUserIDsStr, ",")
		for _, userID := range userIDs {
			userID = strings.TrimSpace(userID)
			if userID != "" {
				cfg.AdminUserIDs = append(cfg.AdminUserIDs, userID)
			}
		}
	}

	if adminChannelIDsStr := os.Getenv("ADMIN_CHANNEL_IDS"); adminChannelIDsStr != "" {
		channelIDs := strings.Split(adminChannelIDsStr, ",")
		for _, channelID := range channelIDs {
			channelID = strings.TrimSpace(channelID)
			if channelID != "" {
				cfg.AdminChannelIDs = append(cfg.AdminChannelIDs, channelID)
			}
		}
	}

	// LOL内战配置
	cfg.LOLCustomEnabled = os.Getenv("LOL_CUSTOM_ENABLED") == "true"

	// 解析LOL内战允许的频道列表
	if channelIDsStr := os.Getenv("LOL_CUSTOM_CHANNEL_IDS"); channelIDsStr != "" {
		channelIDs := strings.Split(channelIDsStr, ",")
		for _, channelID := range channelIDs {
			channelID = strings.TrimSpace(channelID)
			if channelID != "" {
				cfg.LOLCustomChannelIDs = append(cfg.LOLCustomChannelIDs, channelID)
			}
		}
	}

	// LOL内战语音频道分类ID - 如果未设置则使用默认分类
	cfg.LOLCustomCategoryID = os.Getenv("LOL_CUSTOM_CATEGORY_ID")
	if cfg.LOLCustomCategoryID == "" {
		cfg.LOLCustomCategoryID = cfg.DefaultCategoryID
		fmt.Printf("DEBUG: LOL_CUSTOM_CATEGORY_ID 未设置，使用默认分类: %s\n", cfg.LOLCustomCategoryID)
	} else {
		fmt.Printf("DEBUG: LOL_CUSTOM_CATEGORY_ID = %s\n", cfg.LOLCustomCategoryID)
	}

	// LOL内战清理延迟时间
	if delayStr := os.Getenv("LOL_CUSTOM_CLEANUP_DELAY"); delayStr != "" {
		if delay, err := time.ParseDuration(delayStr); err == nil {
			cfg.LOLCustomCleanupDelay = delay
		} else {
			cfg.LOLCustomCleanupDelay = 30 * time.Minute // 默认30分钟
		}
	} else {
		cfg.LOLCustomCleanupDelay = 30 * time.Minute
	}

	// Riot API 配置
	cfg.RiotAPIKey = os.Getenv("RIOT_API_KEY")
	cfg.RiotRegion = os.Getenv("RIOT_REGION")
	if cfg.RiotRegion == "" {
		cfg.RiotRegion = "na1" // 默认北美区
	}
	cfg.RiotPlatform = os.Getenv("RIOT_PLATFORM")
	if cfg.RiotPlatform == "" {
		cfg.RiotPlatform = "americas" // 默认美洲平台
	}

	// Rank 绑定系统配置
	cfg.RankLeaderboardChannelID = os.Getenv("RANK_LEADERBOARD_CHANNEL_ID")

	// 职业选手追踪配置
	cfg.ProTrackChannelID = os.Getenv("PRO_TRACK_CHANNEL_ID")
	cfg.ProTrackMentionRoleID = os.Getenv("PRO_TRACK_MENTION_ROLE_ID")
	cfg.ProTrackMax = 10
	if maxStr := os.Getenv("PRO_TRACK_MAX"); maxStr != "" {
		if n, err := strconv.Atoi(maxStr); err == nil && n > 0 {
			cfg.ProTrackMax = n
		}
	}
	cfg.ProTrackInterval = 20 * time.Second
	if intervalStr := os.Getenv("PRO_TRACK_INTERVAL"); intervalStr != "" {
		if interval, err := time.ParseDuration(intervalStr); err == nil && interval >= 5*time.Second {
			cfg.ProTrackInterval = interval
		}
	}

	if timeoutStr := os.Getenv("CHANNEL_CLEANUP_TIMEOUT"); timeoutStr != "" {
		if timeout, err := time.ParseDuration(timeoutStr); err == nil {
			cfg.ChannelCleanupTimeout = timeout
		}
	}

	if maxSizeStr := os.Getenv("MAX_TEAM_SIZE"); maxSizeStr != "" {
		if maxSize, err := strconv.Atoi(maxSizeStr); err == nil && maxSize > 0 {
			cfg.MaxTeamSize = maxSize
		}
	}

	if minSizeStr := os.Getenv("MIN_TEAM_SIZE"); minSizeStr != "" {
		if minSize, err := strconv.Atoi(minSizeStr); err == nil && minSize > 0 {
			cfg.MinTeamSize = minSize
		}
	}

	if port := os.Getenv("HEALTH_CHECK_PORT"); port != "" {
		cfg.HealthCheckPort = port
	}

	// 验证配置
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return cfg, nil
}

// parseChannelMappings 解析频道映射配置
// 格式: "游戏名1:文字频道ID1:语音分类ID1,游戏名2:文字频道ID2:语音分类ID2"
func (c *Config) parseChannelMappings(mappingsStr string) error {
	pairs := strings.Split(mappingsStr, ",")
	for _, pair := range pairs {
		parts := strings.Split(strings.TrimSpace(pair), ":")
		if len(parts) != 3 {
			return fmt.Errorf("无效的频道映射格式: %s (期望格式: 游戏名称:文字频道ID:语音分类ID)", pair)
		}

		gameName := strings.TrimSpace(parts[0])
		textChannelID := strings.TrimSpace(parts[1])
		voiceCategoryID := strings.TrimSpace(parts[2])

		if gameName == "" || textChannelID == "" || voiceCategoryID == "" {
			return fmt.Errorf("游戏名称、文字频道ID和语音分类ID都不能为空: %s", pair)
		}

		mapping := ChannelMapping{
			TextChannelID:   textChannelID,
			VoiceCategoryID: voiceCategoryID,
			GameName:        gameName,
		}

		c.ChannelMappings = append(c.ChannelMappings, mapping)
	}
	return nil
}

// validate 验证配置的有效性
func (c *Config) validate() error {
	if c.MinTeamSize >= c.MaxTeamSize {
		return fmt.Errorf("MIN_TEAM_SIZE (%d) 必须小于 MAX_TEAM_SIZE (%d)", c.MinTeamSize, c.MaxTeamSize)
	}

	if c.ChannelCleanupTimeout < time.Minute {
		return fmt.Errorf("CHANNEL_CLEANUP_TIMEOUT 不能少于1分钟")
	}

	if len(c.ChannelMappings) == 0 {
		return fmt.Errorf("至少需要配置一个频道映射")
	}

	// 检查频道映射的唯一性
	textChannels := make(map[string]bool)
	for _, mapping := range c.ChannelMappings {
		if textChannels[mapping.TextChannelID] {
			return fmt.Errorf("重复的文字频道ID: %s", mapping.TextChannelID)
		}
		textChannels[mapping.TextChannelID] = true
	}

	return nil
}
