package achievement

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/database"
)

// CommandHandler 成就命令处理器
type CommandHandler struct {
	service *Service
	db      *database.DB
}

// NewCommandHandler 创建成就命令处理器
func NewCommandHandler(service *Service, db *database.DB) *CommandHandler {
	return &CommandHandler{
		service: service,
		db:      db,
	}
}

// RegisterCommands 注册所有成就相关的斜杠命令
func (h *CommandHandler) RegisterCommands(session *discordgo.Session) error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "achievement",
			Description: "成就相关命令",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "list",
					Description: "查看自己的成就进度",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "category",
							Description: "成就分类",
							Required:    false,
							Choices: []*discordgo.ApplicationCommandOptionChoice{
								{Name: "🫖 签到成就", Value: "checkin"},
								{Name: "🎤 语音成就", Value: "voice"},
								{Name: "💬 社交成就", Value: "message"},
								{Name: "🎲 抽奖成就", Value: "lottery"},
								{Name: "☕ 活跃成就", Value: "activity"},
							},
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "summary",
					Description: "查看成就总览",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "leaderboard",
					Description: "查看成就排行榜",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionInteger,
							Name:        "limit",
							Description: "显示排行榜数量 (1-20)",
							Required:    false,
							MinValue:    &[]float64{1}[0],
							MaxValue:    20,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "all",
					Description: "查看全部成就说明",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "info",
					Description: "查看特定成就的详细信息",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "name",
							Description: "成就名称",
							Required:    true,
						},
					},
				},
			},
		},
	}

	// 管理员命令
	adminCommands := []*discordgo.ApplicationCommand{
		{
			Name:                     "achievement-admin",
			Description:              "成就系统管理命令 (仅管理员)",
			DefaultMemberPermissions: &[]int64{discordgo.PermissionManageGuild}[0],
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "process-rewards",
					Description: "处理待处理的奖励",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "reset-progress",
					Description: "重置用户成就进度",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionUser,
							Name:        "user",
							Description: "要重置的用户",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "achievement",
							Description: "成就名称 (留空重置全部)",
							Required:    false,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "grant-achievement",
					Description: "手动授予成就",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionUser,
							Name:        "user",
							Description: "要授予的用户",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "achievement",
							Description: "成就名称",
							Required:    true,
						},
					},
				},
			},
		},
	}

	// 注册普通用户命令
	for _, command := range commands {
		_, err := session.ApplicationCommandCreate(session.State.User.ID, "", command)
		if err != nil {
			return fmt.Errorf("failed to create command %s: %w", command.Name, err)
		}
	}

	// 注册管理员命令
	for _, command := range adminCommands {
		_, err := session.ApplicationCommandCreate(session.State.User.ID, "", command)
		if err != nil {
			return fmt.Errorf("failed to create admin command %s: %w", command.Name, err)
		}
	}

	return nil
}

// HandleInteraction 处理交互命令
func (h *CommandHandler) HandleInteraction(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	commandName := interaction.ApplicationCommandData().Name

	if commandName == "achievement" {
		options := interaction.ApplicationCommandData().Options
		if len(options) == 0 {
			h.respondWithError(session, interaction, "请指定子命令")
			return
		}

		subCommand := options[0]
		switch subCommand.Name {
		case "list":
			h.handleAchievementListCommand(session, interaction)
		case "summary":
			h.handleAchievementSummaryCommand(session, interaction)
		case "leaderboard":
			h.handleAchievementLeaderboardCommand(session, interaction)
		case "all":
			h.handleAchievementAllCommand(session, interaction)
		case "info":
			h.handleAchievementInfoCommand(session, interaction)
		default:
			h.respondWithError(session, interaction, "未知的子命令")
		}
	} else if commandName == "achievement-admin" {
		h.handleAchievementAdminCommand(session, interaction)
	}
}

// handleAchievementListCommand 处理查看成就列表命令
func (h *CommandHandler) handleAchievementListCommand(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	// 获取用户
	userRepo := database.NewUserRepository(h.db)
	user, err := userRepo.GetByDiscordID(interaction.Member.User.ID)
	if err != nil {
		h.respondWithError(session, interaction, "获取用户信息失败")
		return
	}

	if user == nil {
		h.respondWithError(session, interaction, "用户未注册，请先使用其他功能")
		return
	}

	// 获取分类参数（从子命令的options中）
	var category string
	subCommandOptions := interaction.ApplicationCommandData().Options[0].Options
	for _, option := range subCommandOptions {
		if option.Name == "category" {
			category = option.StringValue()
			break
		}
	}

	// 获取用户成就
	achievements, err := h.service.GetUserAchievements(user.ID, category)
	if err != nil {
		h.respondWithError(session, interaction, "获取成就信息失败")
		return
	}

	// 格式化响应
	embed := h.service.FormatUserAchievements(achievements, user.Username)

	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	}

	err = session.InteractionRespond(interaction.Interaction, response)
	if err != nil {
		fmt.Printf("Error responding to achievement command: %v\n", err)
	}
}

// handleAchievementSummaryCommand 处理成就总览命令
func (h *CommandHandler) handleAchievementSummaryCommand(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	// 获取用户
	userRepo := database.NewUserRepository(h.db)
	user, err := userRepo.GetByDiscordID(interaction.Member.User.ID)
	if err != nil {
		h.respondWithError(session, interaction, "获取用户信息失败")
		return
	}

	if user == nil {
		h.respondWithError(session, interaction, "用户未注册，请先使用其他功能")
		return
	}

	// 获取成就总览
	summary, err := h.service.GetUserAchievementSummary(user.ID)
	if err != nil {
		h.respondWithError(session, interaction, "获取成就总览失败")
		return
	}

	if summary == nil {
		h.respondWithError(session, interaction, "暂无成就数据")
		return
	}

	// 格式化响应
	embed := h.service.FormatAchievementSummary(summary)

	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	}

	err = session.InteractionRespond(interaction.Interaction, response)
	if err != nil {
		fmt.Printf("Error responding to achievement summary command: %v\n", err)
	}
}

// handleAchievementLeaderboardCommand 处理成就排行榜命令
func (h *CommandHandler) handleAchievementLeaderboardCommand(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	// 获取限制参数（从子命令的options中）
	limit := 10 // 默认值
	subCommandOptions := interaction.ApplicationCommandData().Options[0].Options
	for _, option := range subCommandOptions {
		if option.Name == "limit" {
			limit = int(option.IntValue())
			break
		}
	}

	// 获取排行榜
	leaderboard, err := h.service.GetAchievementLeaderboard(limit)
	if err != nil {
		h.respondWithError(session, interaction, "获取排行榜失败")
		return
	}

	// 格式化响应
	embed := h.service.FormatAchievementLeaderboard(leaderboard)

	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	}

	err = session.InteractionRespond(interaction.Interaction, response)
	if err != nil {
		fmt.Printf("Error responding to achievement leaderboard command: %v\n", err)
	}
}

// handleAchievementAllCommand 处理查看所有成就命令
func (h *CommandHandler) handleAchievementAllCommand(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	// 获取所有成就
	repo := database.NewAchievementRepository(h.db)
	achievements, err := repo.GetAllAchievements()
	if err != nil {
		h.respondWithError(session, interaction, "获取成就列表失败")
		return
	}

	// 按分类分组
	categories := make(map[string][]database.Achievement)
	for _, achievement := range achievements {
		categories[achievement.Category] = append(categories[achievement.Category], achievement)
	}

	embed := &discordgo.MessageEmbed{
		Title:       "🏆 全部成就列表",
		Color:       0x84c318,
		Description: "茶道之旅的所有成就 - 解锁它们获得奖励！",
	}

	// 分类名称映射
	categoryNames := map[string]string{
		"checkin":  "🫖 签到成就",
		"voice":    "🎤 语音成就",
		"message":  "💬 社交成就",
		"lottery":  "🎲 抽奖成就",
		"activity": "☕ 活跃成就",
	}

	for category, categoryAchievements := range categories {
		categoryName, ok := categoryNames[category]
		if !ok {
			categoryName = fmt.Sprintf("📋 %s", category)
		}

		var fieldValue string
		for _, achievement := range categoryAchievements {
			rewardDesc := h.formatRewardDescription(achievement)
			fieldValue += fmt.Sprintf("%s **%s**\n%s\n*目标：%d* | *奖励：%s*\n\n",
				*achievement.IconEmoji, achievement.Name, achievement.Description,
				achievement.TargetValue, rewardDesc)
		}

		if fieldValue != "" {
			// Discord字段值限制1024字符
			if len(fieldValue) > 1024 {
				fieldValue = fieldValue[:1021] + "..."
			}

			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   categoryName,
				Value:  fieldValue,
				Inline: false,
			})
		}
	}

	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	}

	err = session.InteractionRespond(interaction.Interaction, response)
	if err != nil {
		fmt.Printf("Error responding to achievement list command: %v\n", err)
	}
}

// handleAchievementInfoCommand 处理成就信息命令
func (h *CommandHandler) handleAchievementInfoCommand(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	// 获取成就名称参数（从子命令的options中）
	var achievementName string
	subCommandOptions := interaction.ApplicationCommandData().Options[0].Options
	for _, option := range subCommandOptions {
		if option.Name == "name" {
			achievementName = option.StringValue()
			break
		}
	}

	if achievementName == "" {
		h.respondWithError(session, interaction, "请提供成就名称")
		return
	}

	// 查找成就
	repo := database.NewAchievementRepository(h.db)
	achievements, err := repo.GetAllAchievements()
	if err != nil {
		h.respondWithError(session, interaction, "获取成就信息失败")
		return
	}

	var targetAchievement *database.Achievement
	for _, achievement := range achievements {
		if strings.EqualFold(achievement.Name, achievementName) {
			targetAchievement = &achievement
			break
		}
	}

	if targetAchievement == nil {
		h.respondWithError(session, interaction, fmt.Sprintf("未找到成就「%s」", achievementName))
		return
	}

	// 获取用户进度（如果用户已注册）
	userRepo := database.NewUserRepository(h.db)
	user, _ := userRepo.GetByDiscordID(interaction.Member.User.ID)

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("%s %s", *targetAchievement.IconEmoji, targetAchievement.Name),
		Color:       0x84c318,
		Description: targetAchievement.Description,
	}

	embed.Fields = []*discordgo.MessageEmbedField{
		{
			Name:   "📊 完成条件",
			Value:  h.formatTargetDescription(*targetAchievement),
			Inline: true,
		},
		{
			Name:   "🎁 奖励",
			Value:  h.formatRewardDescription(*targetAchievement),
			Inline: true,
		},
		{
			Name:   "📂 分类",
			Value:  h.getCategoryDisplayName(targetAchievement.Category),
			Inline: true,
		},
	}

	// 如果用户已注册，显示进度
	if user != nil {
		userAchievements, err := h.service.GetUserAchievements(user.ID, "")
		if err == nil {
			for _, userAchievement := range userAchievements {
				if userAchievement.ID == targetAchievement.ID {
					progressBar := h.service.createProgressBar(userAchievement.ProgressPercent)
					status := ""
					if userAchievement.IsUnlocked {
						status = "✅ 已完成"
					} else {
						status = fmt.Sprintf("%d/%d", userAchievement.UserProgress, userAchievement.TargetValue)
					}

					embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
						Name:   "📈 你的进度",
						Value:  fmt.Sprintf("%s %s", progressBar, status),
						Inline: false,
					})
					break
				}
			}
		}
	}

	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	}

	err = session.InteractionRespond(interaction.Interaction, response)
	if err != nil {
		fmt.Printf("Error responding to achievement info command: %v\n", err)
	}
}

// handleAchievementAdminCommand 处理管理员命令
func (h *CommandHandler) handleAchievementAdminCommand(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	// 检查权限
	if !h.hasManageServerPermission(interaction.Member) {
		h.respondWithError(session, interaction, "❌ 您没有权限执行此命令")
		return
	}

	subcommand := interaction.ApplicationCommandData().Options[0]

	switch subcommand.Name {
	case "process-rewards":
		h.handleProcessRewardsCommand(session, interaction)
	case "reset-progress":
		h.handleResetProgressCommand(session, interaction, subcommand)
	case "grant-achievement":
		h.handleGrantAchievementCommand(session, interaction, subcommand)
	}
}

// handleProcessRewardsCommand 处理处理奖励命令
func (h *CommandHandler) handleProcessRewardsCommand(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	guildID := interaction.GuildID

	err := h.service.ProcessPendingRewards(session, guildID)
	if err != nil {
		h.respondWithError(session, interaction, fmt.Sprintf("处理奖励失败: %v", err))
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       "✅ 奖励处理完成",
		Description: "所有待处理的成就奖励已处理完毕",
		Color:       0x00ff00,
	}

	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	}

	err = session.InteractionRespond(interaction.Interaction, response)
	if err != nil {
		fmt.Printf("Error responding to process rewards command: %v\n", err)
	}
}

// 辅助方法

// formatRewardDescription 格式化奖励描述
func (h *CommandHandler) formatRewardDescription(achievement database.Achievement) string {
	switch achievement.RewardType {
	case database.RewardTypePoints:
		return fmt.Sprintf("%s 积分", achievement.RewardValue)
	case database.RewardTypeRole:
		return "特殊身份组"
	case database.RewardTypeBadge:
		return fmt.Sprintf("徽章: %s", achievement.RewardValue)
	default:
		return achievement.RewardValue
	}
}

// formatTargetDescription 格式化目标描述
func (h *CommandHandler) formatTargetDescription(achievement database.Achievement) string {
	switch achievement.Type {
	case database.AchievementTypeCheckIn:
		if achievement.Name == "连饮七日" || achievement.Name == "不离不弃" {
			return fmt.Sprintf("连续签到 %d 天", achievement.TargetValue)
		}
		return fmt.Sprintf("累计签到 %d 次", achievement.TargetValue)
	case database.AchievementTypeVoice:
		hours := achievement.TargetValue / 3600
		if hours > 0 {
			return fmt.Sprintf("累计语音在线 %d 小时", hours)
		}
		return fmt.Sprintf("累计语音在线 %d 秒", achievement.TargetValue)
	case database.AchievementTypeMessage:
		return fmt.Sprintf("累计发送 %d 条消息", achievement.TargetValue)
	case database.AchievementTypeLottery:
		return fmt.Sprintf("参与 %d 次抽奖", achievement.TargetValue)
	case database.AchievementTypeLotteryWin:
		if achievement.Name == "连胜王者" {
			return fmt.Sprintf("连续中奖 %d 次", achievement.TargetValue)
		}
		return fmt.Sprintf("中奖 %d 次", achievement.TargetValue)
	case database.AchievementTypeActivity:
		return fmt.Sprintf("活跃 %d 天", achievement.TargetValue)
	default:
		return fmt.Sprintf("达到 %d", achievement.TargetValue)
	}
}

// getCategoryDisplayName 获取分类显示名称
func (h *CommandHandler) getCategoryDisplayName(category string) string {
	categoryNames := map[string]string{
		"checkin":  "🫖 签到成就",
		"voice":    "🎤 语音成就",
		"message":  "💬 社交成就",
		"lottery":  "🎲 抽奖成就",
		"activity": "☕ 活跃成就",
	}

	if name, ok := categoryNames[category]; ok {
		return name
	}
	return category
}

// hasManageServerPermission 检查是否有管理服务器权限
func (h *CommandHandler) hasManageServerPermission(member *discordgo.Member) bool {
	permissions := int64(member.Permissions)
	return permissions&discordgo.PermissionManageGuild != 0 || permissions&discordgo.PermissionAdministrator != 0
}

// respondWithError 响应错误消息
func (h *CommandHandler) respondWithError(session *discordgo.Session, interaction *discordgo.InteractionCreate, message string) {
	embed := &discordgo.MessageEmbed{
		Title:       "❌ 错误",
		Description: message,
		Color:       0xff0000,
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

// handleResetProgressCommand 和 handleGrantAchievementCommand 的实现由于篇幅限制省略
// 这些方法将实现管理员重置用户进度和手动授予成就的功能

func (h *CommandHandler) handleResetProgressCommand(session *discordgo.Session, interaction *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption) {
	// TODO: 实现重置用户成就进度的逻辑
	h.respondWithError(session, interaction, "此功能尚未实现")
}

func (h *CommandHandler) handleGrantAchievementCommand(session *discordgo.Session, interaction *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption) {
	// TODO: 实现手动授予成就的逻辑
	h.respondWithError(session, interaction, "此功能尚未实现")
}
