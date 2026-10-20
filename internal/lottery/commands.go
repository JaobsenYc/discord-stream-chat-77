package lottery

import (
	"fmt"
	"strings"

	"github.com/jaobsenyc/discord-lfg-bot-go/internal/achievement"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/database"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

// CommandHandler 抽奖命令处理器
type CommandHandler struct {
	service        *Service
	achievementSvc *achievement.Service
	db             *database.DB
	session        *discordgo.Session
}

// NewCommandHandler 创建抽奖命令处理器
func NewCommandHandler(db *database.DB, achievementSvc *achievement.Service) *CommandHandler {
	return &CommandHandler{
		service:        NewService(db, achievementSvc),
		achievementSvc: achievementSvc,
		db:             db,
	}
}

// NewCommandHandlerWithSession 创建带Discord会话的抽奖命令处理器
func NewCommandHandlerWithSession(db *database.DB, achievementSvc *achievement.Service, session *discordgo.Session) *CommandHandler {
	return &CommandHandler{
		service:        NewServiceWithSession(db, achievementSvc, session),
		achievementSvc: achievementSvc,
		db:             db,
		session:        session,
	}
}

// SetSession 设置Discord会话
func (h *CommandHandler) SetSession(session *discordgo.Session) {
	h.session = session
	h.service = NewServiceWithSession(h.db, h.achievementSvc, session)
}

// RegisterCommands 注册抽奖相关的斜杠命令
func (h *CommandHandler) RegisterCommands(session *discordgo.Session) error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "lottery",
			Description: "抽奖相关命令",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "create",
					Description: "创建新的抽奖活动",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "name",
							Description: "抽奖名称",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "prize",
							Description: "奖品描述",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionInteger,
							Name:        "duration",
							Description: "持续时间（分钟）",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "description",
							Description: "抽奖描述",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionInteger,
							Name:        "cost",
							Description: "参与费用（积分）",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionInteger,
							Name:        "max_participants",
							Description: "最大参与人数（0为无限制）",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionInteger,
							Name:        "winner_count",
							Description: "中奖人数（默认1人）",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "required_achievements",
							Description: "必需的成就ID列表（用逗号分隔，如：achievement1,achievement2）",
							Required:    false,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "join",
					Description: "参与抽奖",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "lottery_id",
							Description: "抽奖ID",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "list",
					Description: "查看当前进行中的抽奖",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "info",
					Description: "查看抽奖详情",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "lottery_id",
							Description: "抽奖ID",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "draw",
					Description: "立即开奖（仅创建者）",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "lottery_id",
							Description: "抽奖ID",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "history",
					Description: "查看个人抽奖历史",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionInteger,
							Name:        "limit",
							Description: "显示数量",
							Required:    false,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "leaderboard",
					Description: "查看抽奖排行榜",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionInteger,
							Name:        "limit",
							Description: "显示数量",
							Required:    false,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "stats",
					Description: "查看抽奖统计信息",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "achievements",
					Description: "查看可用的成就列表（用于设置抽奖门槛）",
				},
			},
		},
	}

	for _, command := range commands {
		_, err := session.ApplicationCommandCreate(session.State.User.ID, "", command)
		if err != nil {
			return fmt.Errorf("failed to create command %s: %w", command.Name, err)
		}
	}

	return nil
}

// HandleInteraction 处理交互命令
func (h *CommandHandler) HandleInteraction(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	if interaction.ApplicationCommandData().Name != "lottery" {
		return
	}

	options := interaction.ApplicationCommandData().Options
	if len(options) == 0 {
		h.respondWithError(session, interaction, "请指定子命令")
		return
	}

	subCommand := options[0]
	switch subCommand.Name {
	case "create":
		h.handleCreateCommand(session, interaction)
	case "join":
		h.handleJoinCommand(session, interaction)
	case "list":
		h.handleListCommand(session, interaction)
	case "info":
		h.handleInfoCommand(session, interaction)
	case "draw":
		h.handleDrawCommand(session, interaction)
	case "history":
		h.handleHistoryCommand(session, interaction)
	case "leaderboard":
		h.handleLeaderboardCommand(session, interaction)
	case "stats":
		h.handleStatsCommand(session, interaction)
	case "achievements":
		h.handleAchievementsCommand(session, interaction)
	default:
		h.respondWithError(session, interaction, "未知的子命令")
	}
}

// HandleButtonInteraction 处理按钮交互
func (h *CommandHandler) HandleButtonInteraction(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	customID := interaction.MessageComponentData().CustomID

	// 检查是否是抽奖相关的按钮
	if strings.HasPrefix(customID, "lottery_") {
		h.service.HandleLotteryButtonInteraction(session, interaction)
	}
}

// handleCreateCommand 处理创建抽奖命令
func (h *CommandHandler) handleCreateCommand(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	options := interaction.ApplicationCommandData().Options[0].Options

	// 解析参数
	var name, prize, description, requiredAchievements string
	var duration, cost, maxParticipants, winnerCount int64

	for _, option := range options {
		switch option.Name {
		case "name":
			name = option.StringValue()
		case "prize":
			prize = option.StringValue()
		case "duration":
			duration = option.IntValue()
		case "description":
			description = option.StringValue()
		case "cost":
			cost = option.IntValue()
		case "max_participants":
			maxParticipants = option.IntValue()
		case "winner_count":
			winnerCount = option.IntValue()
		case "required_achievements":
			requiredAchievements = option.StringValue()
		}
	}

	// 设置默认中奖人数
	if winnerCount == 0 {
		winnerCount = 1
	}

	// 获取或创建用户
	user, err := h.getOrCreateUser(interaction.Member.User, interaction.GuildID)
	if err != nil {
		h.respondWithError(session, interaction, "用户信息获取失败")
		return
	}

	// 创建抽奖请求
	var desc *string
	if description != "" {
		desc = &description
	}

	// 解析成就ID列表
	var achievementIDs []uuid.UUID
	if requiredAchievements != "" {
		achievementStrs := strings.Split(requiredAchievements, ",")
		for _, achievementStr := range achievementStrs {
			achievementStr = strings.TrimSpace(achievementStr)
			if achievementStr != "" {
				achievementID, err := uuid.Parse(achievementStr)
				if err != nil {
					h.respondWithError(session, interaction, fmt.Sprintf("无效的成就ID: %s", achievementStr))
					return
				}
				achievementIDs = append(achievementIDs, achievementID)
			}
		}
	}

	req := CreateLotteryRequest{
		Name:                 name,
		Description:          desc,
		GuildID:              interaction.GuildID,
		ChannelID:            interaction.ChannelID,
		CreatorID:            user.ID,
		MaxParticipants:      int(maxParticipants),
		CostPoints:           int(cost),
		PrizeDescription:     prize,
		DurationMinutes:      int(duration),
		WinnerCount:          int(winnerCount),
		RequiredAchievements: achievementIDs,
	}

	// 创建抽奖
	lottery, err := h.service.CreateLottery(req)
	if err != nil {
		h.respondWithError(session, interaction, fmt.Sprintf("创建抽奖失败: %v", err))
		return
	}

	// 发送抽奖公告到指定频道
	announcementChannelID := "1383728525223399565" // 抽奖频道ID
	if h.service.session != nil {
		err = h.service.SendLotteryAnnouncement(lottery, announcementChannelID)
		if err != nil {
			fmt.Printf("发送抽奖公告失败: %v\n", err)
			// 不影响抽奖创建，只记录错误
		}
	}

	// 响应结果
	embed := &discordgo.MessageEmbed{
		Title:       "🎲 抽奖创建成功！",
		Color:       0x2ecc71, // 绿色
		Description: fmt.Sprintf("抽奖 **%s** 已成功创建", lottery.Name),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "🎁 奖品",
				Value:  lottery.PrizeDescription,
				Inline: true,
			},
			{
				Name:   "⏰ 结束时间",
				Value:  lottery.EndTime.Format("2006-01-02 15:04:05"),
				Inline: true,
			},
			{
				Name:   "🆔 抽奖ID",
				Value:  fmt.Sprintf("`%s`", lottery.ID.String()[:8]),
				Inline: true,
			},
			{
				Name:   "🏆 中奖人数",
				Value:  fmt.Sprintf("%d 人", lottery.WinnerCount),
				Inline: true,
			},
		},
	}

	if lottery.CostPoints > 0 {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "💰 参与费用",
			Value:  fmt.Sprintf("%d 积分", lottery.CostPoints),
			Inline: true,
		})
	}

	if lottery.MaxParticipants > 0 {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "👥 人数限制",
			Value:  fmt.Sprintf("%d 人", lottery.MaxParticipants),
			Inline: true,
		})
	}

	embed.Footer = &discordgo.MessageEmbedFooter{
		Text: "其他用户可以使用 /lottery join " + lottery.ID.String()[:8] + " 参与抽奖",
	}

	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	}

	err = session.InteractionRespond(interaction.Interaction, response)
	if err != nil {
		fmt.Printf("Error responding to lottery create command: %v\n", err)
	}
}

// handleJoinCommand 处理参与抽奖命令
func (h *CommandHandler) handleJoinCommand(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	options := interaction.ApplicationCommandData().Options[0].Options
	lotteryIDStr := options[0].StringValue()

	// 解析抽奖ID
	lotteryID, err := h.parseLotteryID(lotteryIDStr)
	if err != nil {
		h.respondWithError(session, interaction, "无效的抽奖ID")
		return
	}

	// 获取或创建用户
	user, err := h.getOrCreateUser(interaction.Member.User, interaction.GuildID)
	if err != nil {
		h.respondWithError(session, interaction, "用户信息获取失败")
		return
	}

	// 参与抽奖
	result, err := h.service.ParticipateInLottery(lotteryID, user.ID, user.DiscordID, interaction.GuildID)
	if err != nil {
		h.respondWithError(session, interaction, fmt.Sprintf("参与抽奖失败: %v", err))
		return
	}

	// 响应结果
	var color int
	if result.Success {
		color = 0x2ecc71 // 绿色
	} else {
		color = 0xe74c3c // 红色
	}

	embed := &discordgo.MessageEmbed{
		Title:       "🎲 抽奖参与结果",
		Description: result.Message,
		Color:       color,
	}

	if result.Success && result.UserPoints > 0 {
		embed.Footer = &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("剩余积分: %d", result.UserPoints),
		}
	}

	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	}

	err = session.InteractionRespond(interaction.Interaction, response)
	if err != nil {
		fmt.Printf("Error responding to lottery join command: %v\n", err)
	}
}

// handleListCommand 处理查看抽奖列表命令
func (h *CommandHandler) handleListCommand(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	lotteries, err := h.service.GetActiveLotteries(interaction.GuildID)
	if err != nil {
		h.respondWithError(session, interaction, "获取抽奖列表失败")
		return
	}

	embed := h.service.FormatLotteryList(lotteries)

	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	}

	err = session.InteractionRespond(interaction.Interaction, response)
	if err != nil {
		fmt.Printf("Error responding to lottery list command: %v\n", err)
	}
}

// handleInfoCommand 处理查看抽奖详情命令
func (h *CommandHandler) handleInfoCommand(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	options := interaction.ApplicationCommandData().Options[0].Options
	lotteryIDStr := options[0].StringValue()

	// 解析抽奖ID
	lotteryID, err := h.parseLotteryID(lotteryIDStr)
	if err != nil {
		h.respondWithError(session, interaction, "无效的抽奖ID")
		return
	}

	// 获取抽奖详情
	details, err := h.service.GetLotteryDetails(lotteryID)
	if err != nil {
		h.respondWithError(session, interaction, fmt.Sprintf("获取抽奖详情失败: %v", err))
		return
	}

	embed := h.service.FormatLotteryDetails(details)

	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	}

	err = session.InteractionRespond(interaction.Interaction, response)
	if err != nil {
		fmt.Printf("Error responding to lottery info command: %v\n", err)
	}
}

// handleDrawCommand 处理立即开奖命令
func (h *CommandHandler) handleDrawCommand(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	options := interaction.ApplicationCommandData().Options[0].Options
	lotteryIDStr := options[0].StringValue()

	// 解析抽奖ID
	lotteryID, err := h.parseLotteryID(lotteryIDStr)
	if err != nil {
		h.respondWithError(session, interaction, "无效的抽奖ID")
		return
	}

	// 获取抽奖信息
	lottery, err := h.service.repo.Lottery.GetByID(lotteryID)
	if err != nil {
		h.respondWithError(session, interaction, "抽奖不存在")
		return
	}

	// 检查权限
	user, err := h.getOrCreateUser(interaction.Member.User, interaction.GuildID)
	if err != nil {
		h.respondWithError(session, interaction, "用户信息获取失败")
		return
	}

	if lottery.CreatedBy != user.ID {
		h.respondWithError(session, interaction, "只有抽奖创建者可以执行开奖")
		return
	}

	// 执行开奖
	result, err := h.service.DrawLottery(lotteryID, interaction.GuildID)
	if err != nil {
		h.respondWithError(session, interaction, fmt.Sprintf("开奖失败: %v", err))
		return
	}

	embed := h.service.FormatLotteryResult(result, lottery)

	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	}

	err = session.InteractionRespond(interaction.Interaction, response)
	if err != nil {
		fmt.Printf("Error responding to lottery draw command: %v\n", err)
	}
}

// handleHistoryCommand 处理查看个人抽奖历史命令
func (h *CommandHandler) handleHistoryCommand(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	// 获取参数
	limit := 10
	options := interaction.ApplicationCommandData().Options[0].Options
	for _, option := range options {
		if option.Name == "limit" {
			limit = int(option.IntValue())
		}
	}

	// 获取用户
	user, err := h.getOrCreateUser(interaction.Member.User, interaction.GuildID)
	if err != nil {
		h.respondWithError(session, interaction, "用户信息获取失败")
		return
	}

	// 获取抽奖历史
	history, err := h.service.repo.Lottery.GetUserLotteryHistory(user.ID, interaction.GuildID, limit)
	if err != nil {
		h.respondWithError(session, interaction, "获取抽奖历史失败")
		return
	}

	embed := h.formatLotteryHistory(history, user.Username)

	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	}

	err = session.InteractionRespond(interaction.Interaction, response)
	if err != nil {
		fmt.Printf("Error responding to lottery history command: %v\n", err)
	}
}

// handleLeaderboardCommand 处理查看抽奖排行榜命令
func (h *CommandHandler) handleLeaderboardCommand(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	// 获取参数
	limit := 10
	options := interaction.ApplicationCommandData().Options[0].Options
	for _, option := range options {
		if option.Name == "limit" {
			limit = int(option.IntValue())
		}
	}

	// 获取排行榜
	leaderboard, err := h.service.repo.Lottery.GetLotteryLeaderboard(interaction.GuildID, limit)
	if err != nil {
		h.respondWithError(session, interaction, "获取排行榜失败")
		return
	}

	embed := h.formatLotteryLeaderboard(leaderboard)

	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	}

	err = session.InteractionRespond(interaction.Interaction, response)
	if err != nil {
		fmt.Printf("Error responding to lottery leaderboard command: %v\n", err)
	}
}

// handleStatsCommand 处理查看抽奖统计命令
func (h *CommandHandler) handleStatsCommand(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	stats, err := h.service.repo.Lottery.GetLotteryStats(interaction.GuildID)
	if err != nil {
		h.respondWithError(session, interaction, "获取统计信息失败")
		return
	}

	embed := h.formatLotteryStats(stats)

	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	}

	err = session.InteractionRespond(interaction.Interaction, response)
	if err != nil {
		fmt.Printf("Error responding to lottery stats command: %v\n", err)
	}
}

// 辅助方法

// getOrCreateUser 获取或创建用户
func (h *CommandHandler) getOrCreateUser(discordUser *discordgo.User, guildID string) (*database.User, error) {
	repo := database.NewRepository(h.db)
	return repo.User.GetOrCreate(discordUser.ID, discordUser.Username, nil, nil)
}

// parseLotteryID 解析抽奖ID（支持短ID和完整UUID）
func (h *CommandHandler) parseLotteryID(idStr string) (uuid.UUID, error) {
	// 如果是短ID（8位），通过数据库查找匹配的完整ID
	if len(idStr) == 8 {
		return h.findLotteryByShortID(idStr)
	}

	// 尝试解析完整的UUID
	return uuid.Parse(idStr)
}

// findLotteryByShortID 通过短ID查找完整的抽奖UUID
func (h *CommandHandler) findLotteryByShortID(shortID string) (uuid.UUID, error) {
	// 使用SQL查询直接匹配短ID
	query := `
		SELECT id FROM lotteries 
		WHERE LOWER(LEFT(id::text, 8)) = LOWER($1)
		ORDER BY created_at DESC
		LIMIT 2`

	var ids []string
	err := h.db.Select(&ids, query, shortID)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("查询抽奖失败: %w", err)
	}

	if len(ids) == 0 {
		return uuid.UUID{}, fmt.Errorf("未找到ID以 %s 开头的抽奖", shortID)
	}

	if len(ids) > 1 {
		return uuid.UUID{}, fmt.Errorf("找到多个匹配的抽奖，请提供更长的ID")
	}

	return uuid.Parse(ids[0])
}

// respondWithError 响应错误消息
func (h *CommandHandler) respondWithError(session *discordgo.Session, interaction *discordgo.InteractionCreate, message string) {
	embed := &discordgo.MessageEmbed{
		Title:       "❌ 错误",
		Description: message,
		Color:       0xe74c3c, // 红色
	}

	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	}

	err := session.InteractionRespond(interaction.Interaction, response)
	if err != nil {
		fmt.Printf("Error responding with error: %v\n", err)
	}
}

// formatLotteryHistory 格式化抽奖历史
func (h *CommandHandler) formatLotteryHistory(history []database.UserLotteryHistory, username string) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("🎲 %s 的抽奖历史", username),
		Color:       0x9b59b6, // 紫色
		Description: "您参与过的抽奖活动记录",
	}

	if len(history) == 0 {
		embed.Description = "您还没有参与过任何抽奖活动"
		return embed
	}

	var fieldValue string
	winCount := 0
	totalSpent := 0

	for i, entry := range history {
		if i >= 10 { // 限制显示数量
			break
		}

		statusIcon := "📋"
		if entry.Status == database.LotteryStatusEnded {
			if entry.IsWinner {
				statusIcon = "🏆"
				winCount++
			} else {
				statusIcon = "😔"
			}
		} else if entry.Status == database.LotteryStatusCancelled {
			statusIcon = "❌"
		}

		fieldValue += fmt.Sprintf("%s **%s**\n", statusIcon, entry.LotteryName)
		fieldValue += fmt.Sprintf("   奖品: %s\n", entry.PrizeDescription)
		fieldValue += fmt.Sprintf("   消费: %d 积分\n", entry.PointsSpent)
		fieldValue += fmt.Sprintf("   时间: %s\n\n", entry.ParticipatedAt.Format("2006-01-02 15:04"))

		totalSpent += entry.PointsSpent
	}

	embed.Fields = []*discordgo.MessageEmbedField{
		{
			Name:   "📋 参与记录",
			Value:  fieldValue,
			Inline: false,
		},
		{
			Name:   "📊 统计",
			Value:  fmt.Sprintf("参与次数: %d\n中奖次数: %d\n总消费: %d 积分", len(history), winCount, totalSpent),
			Inline: false,
		},
	}

	return embed
}

// formatLotteryLeaderboard 格式化抽奖排行榜
func (h *CommandHandler) formatLotteryLeaderboard(leaderboard []database.LotteryLeaderboard) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:       "🏆 抽奖排行榜",
		Color:       0xffd700, // 金色
		Description: "服务器抽奖活跃度排行榜",
	}

	if len(leaderboard) == 0 {
		embed.Description = "暂无排行数据"
		return embed
	}

	var fieldValue string
	for _, entry := range leaderboard {
		rankIcon := getRankIcon(entry.Rank)
		fieldValue += fmt.Sprintf("%s **第%d名** %s\n", rankIcon, entry.Rank, entry.Username)
		fieldValue += fmt.Sprintf("   参与: %d次 | 中奖: %d次 | 胜率: %.1f%%\n",
			entry.ParticipationCount, entry.WinCount, entry.WinRate)
		fieldValue += fmt.Sprintf("   消费: %d积分\n\n", entry.TotalPointsSpent)
	}

	embed.Fields = []*discordgo.MessageEmbedField{
		{
			Name:   "🥇 排行榜",
			Value:  fieldValue,
			Inline: false,
		},
	}

	return embed
}

// formatLotteryStats 格式化抽奖统计
func (h *CommandHandler) formatLotteryStats(stats *database.LotteryStats) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:       "📊 抽奖统计",
		Color:       0x3498db, // 蓝色
		Description: "服务器抽奖活动统计信息",
	}

	embed.Fields = []*discordgo.MessageEmbedField{
		{
			Name:   "🎲 抽奖活动",
			Value:  fmt.Sprintf("总计: %d\n进行中: %d", stats.TotalLotteries, stats.ActiveLotteries),
			Inline: true,
		},
		{
			Name:   "👥 参与情况",
			Value:  fmt.Sprintf("总参与次数: %d", stats.TotalParticipations),
			Inline: true,
		},
		{
			Name:   "💰 积分消费",
			Value:  fmt.Sprintf("总消费: %d积分", stats.TotalPointsSpent),
			Inline: true,
		},
	}

	// 计算平均值
	if stats.TotalLotteries > 0 {
		avgParticipation := float64(stats.TotalParticipations) / float64(stats.TotalLotteries)
		avgSpent := float64(stats.TotalPointsSpent) / float64(stats.TotalLotteries)

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "📈 平均数据",
			Value:  fmt.Sprintf("平均参与: %.1f人次/抽奖\n平均消费: %.1f积分/抽奖", avgParticipation, avgSpent),
			Inline: false,
		})
	}

	return embed
}

// getRankIcon 获取排名图标
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

// handleAchievementsCommand 处理查看成就列表命令
func (h *CommandHandler) handleAchievementsCommand(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	// 获取所有成就列表
	repo := database.NewAchievementRepository(h.db)
	achievements, err := repo.GetAllAchievements()
	if err != nil {
		h.respondWithError(session, interaction, "获取成就列表失败")
		return
	}

	// 格式化成就列表
	embed := h.formatAchievementsForLottery(achievements)

	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	}

	err = session.InteractionRespond(interaction.Interaction, response)
	if err != nil {
		fmt.Printf("Error responding to achievements command: %v\n", err)
	}
}

// formatAchievementsForLottery 格式化成就列表（用于抽奖门槛设置）
func (h *CommandHandler) formatAchievementsForLottery(achievements []database.Achievement) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:       "🏆 抽奖成就门槛设置",
		Color:       0xffd700,
		Description: "以下是可用于设置抽奖门槛的成就列表",
	}

	if len(achievements) == 0 {
		embed.Description = "暂无可用成就"
		return embed
	}

	// 按分类分组
	categoryMap := make(map[string][]database.Achievement)
	for _, achievement := range achievements {
		if !achievement.IsHidden { // 只显示非隐藏成就
			categoryMap[achievement.Category] = append(categoryMap[achievement.Category], achievement)
		}
	}

	// 分类图标映射
	categoryIcons := map[string]string{
		"checkin":  "🫖",
		"voice":    "🎤",
		"message":  "💬",
		"lottery":  "🎲",
		"activity": "☕",
	}

	var fields []*discordgo.MessageEmbedField

	// 添加使用说明
	fields = append(fields, &discordgo.MessageEmbedField{
		Name:   "💡 使用方法",
		Value:  "在创建抽奖时，使用 `required_achievements` 参数：\n```\nrequired_achievements: uuid1,uuid2,uuid3\n```",
		Inline: false,
	})

	// 为每个分类创建字段
	for category, categoryAchievements := range categoryMap {
		if len(categoryAchievements) == 0 {
			continue
		}

		icon := categoryIcons[category]
		if icon == "" {
			icon = "🏆"
		}

		var fieldValue string
		for i, achievement := range categoryAchievements {
			if i >= 8 { // 限制每个分类显示的数量
				fieldValue += "... 更多成就\n"
				break
			}

			// 显示成就信息和完整UUID
			fieldValue += fmt.Sprintf("**%s**\n", achievement.Name)
			fieldValue += fmt.Sprintf("📋 %s\n", achievement.Description)
			fieldValue += fmt.Sprintf("🆔 `%s`\n\n", achievement.ID.String())
		}

		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("%s %s成就", icon, h.getCategoryName(category)),
			Value:  fieldValue,
			Inline: false,
		})
	}

	// 添加复制提示
	fields = append(fields, &discordgo.MessageEmbedField{
		Name:   "📋 复制提示",
		Value:  "点击UUID可以快速选择复制。多个成就用逗号分隔：\n```\nuuid1,uuid2,uuid3\n```",
		Inline: false,
	})

	embed.Fields = fields
	return embed
}

// getCategoryName 获取分类中文名称
func (h *CommandHandler) getCategoryName(category string) string {
	categoryNames := map[string]string{
		"checkin":  "签到",
		"voice":    "语音",
		"message":  "社交",
		"lottery":  "抽奖",
		"activity": "活跃",
	}

	if name, exists := categoryNames[category]; exists {
		return name
	}
	return category
}
