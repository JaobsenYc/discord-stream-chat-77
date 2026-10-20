package rank

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// CommandHandler handles rank-related Discord commands
type CommandHandler struct {
	service    *Service
	session    *discordgo.Session
	guildID    string
	opggRegion string
}

// NewCommandHandler creates a new CommandHandler
func NewCommandHandler(service *Service, session *discordgo.Session, guildID string, riotRegion string) *CommandHandler {
	return &CommandHandler{
		service:    service,
		session:    session,
		guildID:    guildID,
		opggRegion: ToOpggRegion(riotRegion),
	}
}

// ToOpggRegion converts a Riot region code (e.g. "na1") to OP.GG's uppercase region slug (e.g. "NA")
func ToOpggRegion(riotRegion string) string {
	mapping := map[string]string{
		"na1":  "NA",
		"euw1": "EUW",
		"kr":   "KR",
		"eun1": "EUNE",
		"br1":  "BR",
		"jp1":  "JP",
		"la1":  "LAN",
		"la2":  "LAS",
		"oc1":  "OCE",
		"tr1":  "TR",
		"ru":   "RU",
		"ph2":  "PH",
		"sg2":  "SG",
		"th2":  "TH",
		"tw2":  "TW",
		"vn2":  "VN",
	}
	if slug, ok := mapping[riotRegion]; ok {
		return slug
	}
	return strings.ToUpper(riotRegion)
}

// OpggURL returns the OP.GG profile URL for a Riot ID (opggRegion comes from ToOpggRegion)
func OpggURL(opggRegion, riotID, riotTag string) string {
	return fmt.Sprintf("https://www.op.gg/summoners/%s/%s-%s",
		opggRegion, url.PathEscape(riotID), url.PathEscape(riotTag))
}

// opggLink returns a clickable OP.GG markdown link for a Riot ID
func (h *CommandHandler) opggLink(riotID, riotTag string) string {
	return fmt.Sprintf("[%s#%s](%s)", riotID, riotTag, OpggURL(h.opggRegion, riotID, riotTag))
}

// RegisterCommands registers the /rank slash commands
func (h *CommandHandler) RegisterCommands() error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "rank",
			Description: "Riot 账号绑定与段位排行榜",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "bind",
					Description: "绑定 Riot 账号（需要验证）",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "riot_id",
							Description: "Riot ID (格式: 玩家名#TAG)",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "verify",
					Description: "完成账号验证",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "riot_id",
							Description: "要验证的 Riot ID (格式: 玩家名#TAG)",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "unbind",
					Description: "解除 Riot 账号绑定",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "riot_id",
							Description: "要解绑的 Riot ID (格式: 玩家名#TAG)",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "list",
					Description: "查看段位排行榜",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionInteger,
							Name:        "limit",
							Description: "显示人数（默认20人）",
							Required:    false,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "me",
					Description: "查看自己绑定的账号和段位",
				},
			},
		},
	}

	for _, cmd := range commands {
		_, err := h.session.ApplicationCommandCreate(h.session.State.User.ID, h.guildID, cmd)
		if err != nil {
			return fmt.Errorf("注册命令 %s 失败: %w", cmd.Name, err)
		}
		logger.Infof("已注册命令: %s", cmd.Name)
	}

	return nil
}

// HandleCommand routes slash commands to appropriate handlers
func (h *CommandHandler) HandleCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.ApplicationCommandData().Name != "rank" {
		return
	}

	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		h.respondError(s, i, "请指定子命令")
		return
	}

	switch options[0].Name {
	case "bind":
		h.handleBind(s, i, options[0].Options)
	case "verify":
		h.handleVerify(s, i, options[0].Options)
	case "unbind":
		h.handleUnbind(s, i, options[0].Options)
	case "list":
		h.handleList(s, i, options[0].Options)
	case "me":
		h.handleMe(s, i)
	default:
		h.respondError(s, i, "未知的子命令")
	}
}

// handleBind processes the /rank bind command
func (h *CommandHandler) handleBind(s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) {
	var riotIDFull string
	for _, opt := range options {
		if opt.Name == "riot_id" {
			riotIDFull = opt.StringValue()
		}
	}

	riotID, riotTag, err := ParseRiotID(riotIDFull)
	if err != nil {
		h.respondError(s, i, fmt.Sprintf("❌ 无效的 Riot ID 格式: %v\n正确格式: 玩家名#TAG", err))
		return
	}

	// Defer response for API call
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	binding, iconID, err := h.service.StartBinding(ctx, i.Member.User.ID, i.ChannelID, riotID, riotTag)
	if err != nil {
		h.editResponse(s, i, fmt.Sprintf("❌ 绑定失败: %v", err))
		return
	}

	// 获取图标图片 URL (使用 Data Dragon API，动态获取最新版本)
	ddVersion := h.service.GetLatestDataDragonVersion()
	iconImageURL := fmt.Sprintf("https://ddragon.leagueoflegends.com/cdn/%s/img/profileicon/%d.png", ddVersion, iconID)

	embed := &discordgo.MessageEmbed{
		Title:       "🔐 需要账号验证",
		Description: fmt.Sprintf("为了验证您拥有 **%s#%s** 这个账号，请将您的召唤师图标更改为下图所示的图标：", binding.RiotID, binding.RiotTag),
		Color:       0x3498db,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: iconImageURL,
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "📍 第一步",
				Value:  "打开英雄联盟客户端，点击右上角头像 > **收藏** > **图标**，找到并选择右边显示的这个图标",
				Inline: false,
			},
			{
				Name:   "📍 第二步",
				Value:  fmt.Sprintf("更改图标后，运行命令：\n`/rank verify riot_id:%s#%s`", binding.RiotID, binding.RiotTag),
				Inline: false,
			},
			{
				Name:   "⏰ 验证时限",
				Value:  "10 分钟内完成验证",
				Inline: true,
			},
			{
				Name:   "🆔 图标 ID",
				Value:  fmt.Sprintf("%d", iconID),
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "这是默认图标之一，所有玩家都可以免费使用",
		},
	}

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
	})
}

// handleVerify processes the /rank verify command
func (h *CommandHandler) handleVerify(s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) {
	var riotIDFull string
	for _, opt := range options {
		if opt.Name == "riot_id" {
			riotIDFull = opt.StringValue()
		}
	}

	riotID, riotTag, err := ParseRiotID(riotIDFull)
	if err != nil {
		h.respondError(s, i, fmt.Sprintf("❌ 无效的 Riot ID 格式: %v", err))
		return
	}

	// Defer response for API call
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	binding, err := h.service.VerifyBinding(ctx, i.Member.User.ID, i.ChannelID, riotID, riotTag)
	if err != nil {
		h.editResponse(s, i, fmt.Sprintf("❌ 验证失败: %v", err))
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       "✅ 账号绑定成功",
		Description: fmt.Sprintf("已成功绑定 **%s#%s** 到您的 Discord 账号！", binding.RiotID, binding.RiotTag),
		Color:       0x00ff00,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "📊 段位数据",
				Value:  "段位信息正在获取中，稍后可使用 `/rank me` 查看",
				Inline: false,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "您可以绑定最多 3 个 Riot 账号",
		},
	}

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
	})
}

// handleUnbind processes the /rank unbind command
func (h *CommandHandler) handleUnbind(s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) {
	var riotIDFull string
	for _, opt := range options {
		if opt.Name == "riot_id" {
			riotIDFull = opt.StringValue()
		}
	}

	riotID, riotTag, err := ParseRiotID(riotIDFull)
	if err != nil {
		h.respondError(s, i, fmt.Sprintf("❌ 无效的 Riot ID 格式: %v", err))
		return
	}

	// Defer response for API call
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = h.service.UnbindAccount(ctx, i.Member.User.ID, i.ChannelID, riotID, riotTag)
	if err != nil {
		h.editResponse(s, i, fmt.Sprintf("❌ 解绑失败: %v", err))
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       "✅ 账号已解绑",
		Description: fmt.Sprintf("已成功解除 **%s#%s** 的绑定", riotID, riotTag),
		Color:       0xff9900,
	}

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
	})
}

// handleList processes the /rank list command
func (h *CommandHandler) handleList(s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) {
	limit := 20
	for _, opt := range options {
		if opt.Name == "limit" {
			limit = int(opt.IntValue())
			if limit < 1 {
				limit = 1
			}
			if limit > 50 {
				limit = 50
			}
		}
	}

	entries, err := h.service.GetLeaderboard(limit)
	if err != nil {
		h.respondError(s, i, "❌ 获取排行榜失败")
		return
	}

	embed := h.buildLeaderboardEmbed(entries, limit)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
}

// handleMe processes the /rank me command
func (h *CommandHandler) handleMe(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userInfos, err := h.service.GetUserBindingsWithRanks(i.Member.User.ID)
	if err != nil {
		h.respondError(s, i, "❌ 获取绑定信息失败")
		return
	}

	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("🎮 %s 的 Riot 账号", i.Member.User.Username),
		Color: 0x3498db,
	}

	if len(userInfos) == 0 {
		embed.Description = "您还没有绑定任何 Riot 账号\n\n使用 `/rank bind` 绑定您的账号！"
	} else {
		var fields []*discordgo.MessageEmbedField

		for _, info := range userInfos {
			rankStr := "Unranked"
			winRateStr := "N/A"

			if info.Rank != nil && info.Rank.Tier != nil {
				tier := *info.Rank.Tier
				division := ""
				if info.Rank.Division != nil {
					division = *info.Rank.Division
				}
				rankStr = RankDisplay(tier, division, info.Rank.LeaguePoints)
				winRate := CalculateWinRate(info.Rank.Wins, info.Rank.Losses)
				winRateStr = fmt.Sprintf("%.1f%% (%dW/%dL)", winRate, info.Rank.Wins, info.Rank.Losses)
			}

			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   fmt.Sprintf("🎯 %s#%s", info.Binding.RiotID, info.Binding.RiotTag),
				Value:  fmt.Sprintf("**段位:** %s\n**胜率:** %s\n%s", rankStr, winRateStr, h.opggLink(info.Binding.RiotID, info.Binding.RiotTag)),
				Inline: true,
			})
		}

		embed.Fields = fields
		embed.Footer = &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("已绑定 %d/%d 个账号", len(userInfos), MaxBindingsPerUser),
		}
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
}

// buildLeaderboardEmbed creates the leaderboard embed
func (h *CommandHandler) buildLeaderboardEmbed(entries []LeaderboardEntry, limit int) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title: "🏆 段位排行榜",
		Color: 0xffd700,
	}

	if len(entries) == 0 {
		embed.Description = "暂无排行数据\n\n使用 `/rank bind` 绑定您的 Riot 账号！"
		return embed
	}

	var sb strings.Builder
	for _, entry := range entries {
		rankIcon := getRankIcon(entry.Rank)

		rankStr := "Unranked"
		if entry.Tier != "" {
			rankStr = RankDisplay(entry.Tier, entry.Division, entry.LeaguePoints)
		}

		sb.WriteString(fmt.Sprintf("%s **#%d** <@%s>\n", rankIcon, entry.Rank, entry.DiscordID))
		sb.WriteString(fmt.Sprintf("└ %s | %s | %.1f%% WR\n\n",
			h.opggLink(entry.RiotID, entry.RiotTag), rankStr, entry.WinRate))
	}

	embed.Description = sb.String()
	embed.Footer = &discordgo.MessageEmbedFooter{
		Text: fmt.Sprintf("显示前 %d 名 | 按段位+LP排序", len(entries)),
	}

	return embed
}

// BuildScheduledLeaderboardEmbed creates the embed for scheduled leaderboard posts
func (h *CommandHandler) BuildScheduledLeaderboardEmbed(entries []LeaderboardEntry) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:     "🏆 段位排行榜 (每12小时更新)",
		Color:     0xffd700,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	if len(entries) == 0 {
		embed.Description = "暂无排行数据\n\n使用 `/rank bind` 绑定您的 Riot 账号加入排行榜！"
		return embed
	}

	var sb strings.Builder
	for _, entry := range entries {
		rankIcon := getRankIcon(entry.Rank)

		rankStr := "Unranked"
		if entry.Tier != "" {
			rankStr = RankDisplay(entry.Tier, entry.Division, entry.LeaguePoints)
		}

		sb.WriteString(fmt.Sprintf("%s **#%d** <@%s>\n", rankIcon, entry.Rank, entry.DiscordID))
		sb.WriteString(fmt.Sprintf("└ %s | %s | %.1f%% WR\n\n",
			h.opggLink(entry.RiotID, entry.RiotTag), rankStr, entry.WinRate))
	}

	embed.Description = sb.String()
	embed.Footer = &discordgo.MessageEmbedFooter{
		Text: "使用 /rank bind 绑定账号 | /rank list 查看完整排行榜",
	}

	return embed
}

// respondError sends an ephemeral error response
func (h *CommandHandler) respondError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

// editResponse edits a deferred response
func (h *CommandHandler) editResponse(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &message,
	})
}

// ParseRiotID parses a Riot ID string (Name#TAG) into name and tag (shared with protrack)
func ParseRiotID(fullID string) (name, tag string, err error) {
	// 清理不可见的 Unicode 控制字符（常见于从 Discord 复制的文本）
	fullID = cleanInvisibleChars(fullID)
	fullID = strings.TrimSpace(fullID)

	parts := strings.Split(fullID, "#")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("格式错误，请使用: 玩家名#TAG")
	}

	name = strings.TrimSpace(parts[0])
	tag = strings.TrimSpace(parts[1])

	if name == "" {
		return "", "", fmt.Errorf("玩家名不能为空")
	}
	if tag == "" {
		return "", "", fmt.Errorf("TAG 不能为空")
	}

	return name, tag, nil
}

// cleanInvisibleChars removes invisible Unicode control characters
func cleanInvisibleChars(s string) string {
	// 移除常见的不可见字符：
	// U+2066 LEFT-TO-RIGHT ISOLATE
	// U+2067 RIGHT-TO-LEFT ISOLATE
	// U+2068 FIRST STRONG ISOLATE
	// U+2069 POP DIRECTIONAL ISOLATE
	// U+202A LEFT-TO-RIGHT EMBEDDING
	// U+202B RIGHT-TO-LEFT EMBEDDING
	// U+202C POP DIRECTIONAL FORMATTING
	// U+202D LEFT-TO-RIGHT OVERRIDE
	// U+202E RIGHT-TO-LEFT OVERRIDE
	// U+200B ZERO WIDTH SPACE
	// U+200C ZERO WIDTH NON-JOINER
	// U+200D ZERO WIDTH JOINER
	// U+FEFF BYTE ORDER MARK
	invisibleChars := []string{
		"\u2066", "\u2067", "\u2068", "\u2069",
		"\u202A", "\u202B", "\u202C", "\u202D", "\u202E",
		"\u200B", "\u200C", "\u200D", "\uFEFF",
	}

	result := s
	for _, char := range invisibleChars {
		result = strings.ReplaceAll(result, char, "")
	}
	return result
}

// getRankIcon returns an icon for the leaderboard position
func getRankIcon(rank int) string {
	switch rank {
	case 1:
		return "🥇"
	case 2:
		return "🥈"
	case 3:
		return "🥉"
	default:
		return "🏅"
	}
}
