package points

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/database"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// CommandHandler 积分命令处理器
type CommandHandler struct {
	db       *database.DB
	userRepo *database.UserRepository
	session  *discordgo.Session
	guildID  string
}

// NewCommandHandler 创建积分命令处理器
func NewCommandHandler(db *database.DB, session *discordgo.Session, guildID string) *CommandHandler {
	return &CommandHandler{
		db:       db,
		userRepo: database.NewUserRepository(db),
		session:  session,
		guildID:  guildID,
	}
}

// RegisterCommands 注册积分命令
func (h *CommandHandler) RegisterCommands() error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "points",
			Description: "积分系统",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "check",
					Description: "查看自己的积分",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "leaderboard",
					Description: "查看积分排行榜",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionInteger,
							Name:        "limit",
							Description: "显示人数（默认10人）",
							Required:    false,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "redeem",
					Description: "兑换1350RP皮肤（消耗10000积分）",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "riot_id",
							Description: "Riot ID（例如：Player#TAG）",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "skin_name",
							Description: "皮肤名称",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "add",
					Description: "【管理员】给用户添加积分",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionUser,
							Name:        "user",
							Description: "目标用户",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionInteger,
							Name:        "amount",
							Description: "积分数量",
							Required:    true,
							MinValue:    &minPointsValue,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "reason",
							Description: "原因（可选）",
							Required:    false,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "remove",
					Description: "【管理员】扣除用户积分",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionUser,
							Name:        "user",
							Description: "目标用户",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionInteger,
							Name:        "amount",
							Description: "积分数量",
							Required:    true,
							MinValue:    &minPointsValue,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "reason",
							Description: "原因（可选）",
							Required:    false,
						},
					},
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

// HandleInteraction 处理交互命令
func (h *CommandHandler) HandleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.ApplicationCommandData().Name != "points" {
		return
	}

	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		h.respondError(s, i, "请指定子命令")
		return
	}

	switch options[0].Name {
	case "check":
		h.handleCheck(s, i)
	case "leaderboard":
		h.handleLeaderboard(s, i, options[0].Options)
	case "redeem":
		h.handleRedeem(s, i, options[0].Options)
	case "add":
		h.handleAddPoints(s, i, options[0].Options)
	case "remove":
		h.handleRemovePoints(s, i, options[0].Options)
	default:
		h.respondError(s, i, "未知的子命令")
	}
}

// handleCheck 处理查看积分命令
func (h *CommandHandler) handleCheck(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// 获取或创建用户
	user, err := h.userRepo.GetOrCreate(
		i.Member.User.ID,
		i.Member.User.Username,
		nil, nil,
	)
	if err != nil {
		logger.Errorf("获取用户失败: %v", err)
		h.respondError(s, i, "获取用户信息失败")
		return
	}

	// 获取用户积分
	points, err := h.userRepo.GetUserPoints(user.ID, h.guildID)
	if err != nil {
		logger.Errorf("获取积分失败: %v", err)
		h.respondError(s, i, "获取积分信息失败")
		return
	}

	// 创建嵌入消息
	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("💰 %s 的积分信息", i.Member.User.Username),
		Color: 0x3498db,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "🎯 可用积分",
				Value:  fmt.Sprintf("%d", points.AvailablePoints),
				Inline: true,
			},
			{
				Name:   "📊 总积分",
				Value:  fmt.Sprintf("%d", points.TotalPoints),
				Inline: true,
			},
			{
				Name:   "⭐ 等级",
				Value:  fmt.Sprintf("Lv.%d", points.Level),
				Inline: true,
			},
			{
				Name:   "📈 累计获得",
				Value:  fmt.Sprintf("%d", points.LifetimeEarned),
				Inline: true,
			},
			{
				Name:   "📉 累计消费",
				Value:  fmt.Sprintf("%d", points.LifetimeSpent),
				Inline: true,
			},
			{
				Name:   "✨ 经验值",
				Value:  fmt.Sprintf("%d", points.Experience),
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "使用 /points leaderboard 查看排行榜",
		},
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})

	if err != nil {
		logger.Errorf("响应命令失败: %v", err)
	}
}

// handleLeaderboard 处理排行榜命令
func (h *CommandHandler) handleLeaderboard(s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) {
	// 解析参数
	limit := 10
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

	// 获取排行榜
	leaderboard, err := h.userRepo.GetLeaderboard(h.guildID, limit)
	if err != nil {
		logger.Errorf("获取排行榜失败: %v", err)
		h.respondError(s, i, "获取排行榜失败")
		return
	}

	// 创建嵌入消息
	embed := &discordgo.MessageEmbed{
		Title:       "🏆 积分排行榜",
		Description: fmt.Sprintf("显示前 %d 名", limit),
		Color:       0xffd700,
	}

	if len(leaderboard) == 0 {
		embed.Description = "暂无排行数据"
	} else {
		// Discord embed field value 限制为 1024 字符
		// 每个条目约 90 字符，每个 field 最多显示 10 条
		const maxPerField = 10

		var fields []*discordgo.MessageEmbedField
		var fieldValue string
		fieldCount := 0

		for idx, entry := range leaderboard {
			rankIcon := getRankIcon(entry.Rank)
			// 系统账户显示为"庄家账户"，其他用户使用 mention 格式
			var displayName string
			if entry.DiscordID == database.SystemDiscordID {
				displayName = "🏦 **庄家账户**"
			} else {
				displayName = fmt.Sprintf("<@%s>", entry.DiscordID)
			}
			fieldValue += fmt.Sprintf("%s **第%d名** %s\n", rankIcon, entry.Rank, displayName)
			fieldValue += fmt.Sprintf("   💰 积分: %d | ⭐ Lv.%d | ✨ 经验: %d\n\n",
				entry.AvailablePoints, entry.Level, entry.Experience)
			fieldCount++

			// 每 10 条创建一个新的 field
			if fieldCount >= maxPerField || idx == len(leaderboard)-1 {
				fieldName := "🥇 排行榜"
				if len(fields) > 0 {
					fieldName = "\u200b" // 零宽空格，避免重复标题
				}
				fields = append(fields, &discordgo.MessageEmbedField{
					Name:   fieldName,
					Value:  fieldValue,
					Inline: false,
				})
				fieldValue = ""
				fieldCount = 0
			}
		}

		embed.Fields = fields
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})

	if err != nil {
		logger.Errorf("响应命令失败: %v", err)
	}
}

// respondError 响应错误消息
func (h *CommandHandler) respondError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "❌ " + message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
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

// 皮肤兑换常量
const (
	RedeemCost  = 10000 // 兑换皮肤消耗的积分
	AdminRoleID = ""    // 管理员角色ID，留空则@所有管理员
)

// 管理员权限常量
var minPointsValue float64 = 1

// handleRedeem 处理皮肤兑换命令
func (h *CommandHandler) handleRedeem(s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) {
	// 解析参数
	var riotID, skinName string
	for _, opt := range options {
		switch opt.Name {
		case "riot_id":
			riotID = opt.StringValue()
		case "skin_name":
			skinName = opt.StringValue()
		}
	}

	if riotID == "" || skinName == "" {
		h.respondError(s, i, "请提供 Riot ID 和皮肤名称")
		return
	}

	// 获取用户
	user, err := h.userRepo.GetOrCreate(
		i.Member.User.ID,
		i.Member.User.Username,
		nil, nil,
	)
	if err != nil {
		logger.Errorf("获取用户失败: %v", err)
		h.respondError(s, i, "获取用户信息失败")
		return
	}

	// 获取用户积分
	points, err := h.userRepo.GetUserPoints(user.ID, h.guildID)
	if err != nil {
		logger.Errorf("获取积分失败: %v", err)
		h.respondError(s, i, "获取积分信息失败")
		return
	}

	// 检查积分是否足够
	if points.AvailablePoints < RedeemCost {
		h.respondError(s, i, fmt.Sprintf("积分不足！兑换皮肤需要 %d 积分，您当前只有 %d 积分", RedeemCost, points.AvailablePoints))
		return
	}

	// 扣除积分
	err = h.userRepo.SpendPoints(
		user.ID, h.guildID, RedeemCost,
		"skin_redeem",
		fmt.Sprintf("兑换皮肤: %s", skinName),
		nil, nil,
	)
	if err != nil {
		logger.Errorf("扣除积分失败: %v", err)
		h.respondError(s, i, "扣除积分失败，请稍后重试")
		return
	}

	// 生成兑换ID
	redeemID := uuid.New().String()[:8]

	// 创建管理员通知消息
	embed := &discordgo.MessageEmbed{
		Title:       "🎁 皮肤兑换申请",
		Description: fmt.Sprintf("<@%s> 申请兑换皮肤", i.Member.User.ID),
		Color:       0xf1c40f, // 金色
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "🆔 兑换编号",
				Value:  redeemID,
				Inline: true,
			},
			{
				Name:   "🎮 Riot ID",
				Value:  riotID,
				Inline: true,
			},
			{
				Name:   "🎨 皮肤名称",
				Value:  skinName,
				Inline: true,
			},
			{
				Name:   "💰 消耗积分",
				Value:  fmt.Sprintf("%d", RedeemCost),
				Inline: true,
			},
			{
				Name:   "📅 申请时间",
				Value:  time.Now().Format("2006-01-02 15:04:05"),
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("⚠️ 只能兑换1350RP皮肤 | 用户ID: %s | Discord: %s", user.ID, i.Member.User.Username),
		},
	}

	// 创建按钮
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "✅ 已发送",
					Style:    discordgo.SuccessButton,
					CustomID: fmt.Sprintf("redeem_confirm_%s_%s", redeemID, user.ID),
				},
				discordgo.Button{
					Label:    "❌ 退回积分",
					Style:    discordgo.DangerButton,
					CustomID: fmt.Sprintf("redeem_refund_%s_%s", redeemID, user.ID),
				},
			},
		},
	}

	// 发送管理员通知（@管理员）
	adminMention := "@here" // 默认@所有在线人员，可以改成特定角色
	_, err = s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{
		Content:    fmt.Sprintf("📢 %s 有新的皮肤兑换申请！", adminMention),
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: components,
	})
	if err != nil {
		logger.Errorf("发送管理员通知失败: %v", err)
	}

	// 响应用户
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("✅ 兑换申请已提交！\n\n🆔 兑换编号: **%s**\n🎮 Riot ID: **%s**\n🎨 皮肤名称: **%s**\n💰 已扣除: **%d** 积分\n\n⚠️ 注意：只能兑换1350RP皮肤\n\n请等待管理员处理...", redeemID, riotID, skinName, RedeemCost),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		logger.Errorf("响应命令失败: %v", err)
	}

	logger.Infof("用户 %s 申请兑换皮肤: %s (Riot ID: %s)", i.Member.User.Username, skinName, riotID)
}

// HandleRedeemButton 处理兑换按钮交互
func (h *CommandHandler) HandleRedeemButton(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// 审批按钮发在公开频道，任何人可见可点，必须校验管理员权限
	if !h.isAdmin(s, i) {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ 只有管理员可以处理兑换申请",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			logger.Errorf("响应权限校验失败: %v", err)
		}
		return
	}

	customID := i.MessageComponentData().CustomID

	// 解析 customID: redeem_confirm_xxx_userID 或 redeem_refund_xxx_userID
	parts := strings.Split(customID, "_")
	if len(parts) < 4 {
		return
	}

	action := parts[1]    // confirm 或 refund
	redeemID := parts[2]  // 兑换编号
	userIDStr := parts[3] // 用户ID

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		logger.Errorf("解析用户ID失败: %v", err)
		return
	}

	switch action {
	case "confirm":
		// 管理员确认已发送
		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    fmt.Sprintf("✅ 兑换 **%s** 已完成，皮肤已发送！\n处理人: <@%s>", redeemID, i.Member.User.ID),
				Embeds:     i.Message.Embeds,
				Components: []discordgo.MessageComponent{}, // 移除按钮
			},
		})
		if err != nil {
			logger.Errorf("更新消息失败: %v", err)
		}
		logger.Infof("管理员 %s 确认兑换 %s 已完成", i.Member.User.Username, redeemID)

	case "refund":
		// 退回积分
		err = h.userRepo.AddPoints(
			userID, h.guildID, RedeemCost,
			"skin_redeem_refund",
			fmt.Sprintf("皮肤兑换退款: %s", redeemID),
			nil, nil,
		)
		if err != nil {
			logger.Errorf("退回积分失败: %v", err)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "❌ 退回积分失败，请手动处理",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    fmt.Sprintf("❌ 兑换 **%s** 已取消，积分已退回！\n处理人: <@%s>", redeemID, i.Member.User.ID),
				Embeds:     i.Message.Embeds,
				Components: []discordgo.MessageComponent{}, // 移除按钮
			},
		})
		if err != nil {
			logger.Errorf("更新消息失败: %v", err)
		}
		logger.Infof("管理员 %s 退回兑换 %s 的积分", i.Member.User.Username, redeemID)
	}
}

// isAdmin 检查用户是否有管理员权限
func (h *CommandHandler) isAdmin(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
	// 检查是否有管理员权限（Administrator 或 ManageGuild）
	permissions := i.Member.Permissions
	return permissions&discordgo.PermissionAdministrator != 0 || permissions&discordgo.PermissionManageGuild != 0
}

// handleAddPoints 处理管理员添加积分命令
func (h *CommandHandler) handleAddPoints(s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) {
	// 检查管理员权限
	if !h.isAdmin(s, i) {
		h.respondError(s, i, "只有管理员可以使用此命令")
		return
	}

	// 解析参数
	var targetUser *discordgo.User
	var amount int64
	var reason string = "管理员添加"

	for _, opt := range options {
		switch opt.Name {
		case "user":
			targetUser = opt.UserValue(s)
		case "amount":
			amount = opt.IntValue()
		case "reason":
			reason = opt.StringValue()
		}
	}

	if targetUser == nil {
		h.respondError(s, i, "请指定目标用户")
		return
	}

	if amount <= 0 {
		h.respondError(s, i, "积分数量必须大于0")
		return
	}

	// 获取或创建目标用户
	user, err := h.userRepo.GetOrCreate(
		targetUser.ID,
		targetUser.Username,
		nil, nil,
	)
	if err != nil {
		logger.Errorf("获取用户失败: %v", err)
		h.respondError(s, i, "获取用户信息失败")
		return
	}

	// 添加积分
	err = h.userRepo.AddPoints(
		user.ID, h.guildID, int(amount),
		"admin_add",
		fmt.Sprintf("%s (操作人: %s)", reason, i.Member.User.Username),
		nil, nil,
	)
	if err != nil {
		logger.Errorf("添加积分失败: %v", err)
		h.respondError(s, i, "添加积分失败")
		return
	}

	// 获取更新后的积分
	points, _ := h.userRepo.GetUserPoints(user.ID, h.guildID)

	// 响应
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("✅ 成功给 <@%s> 添加 **%d** 积分\n📝 原因: %s\n💰 当前积分: **%d**", targetUser.ID, amount, reason, points.AvailablePoints),
		},
	})
	if err != nil {
		logger.Errorf("响应命令失败: %v", err)
	}

	logger.Infof("管理员 %s 给 %s 添加 %d 积分，原因: %s", i.Member.User.Username, targetUser.Username, amount, reason)
}

// handleRemovePoints 处理管理员扣除积分命令
func (h *CommandHandler) handleRemovePoints(s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) {
	// 检查管理员权限
	if !h.isAdmin(s, i) {
		h.respondError(s, i, "只有管理员可以使用此命令")
		return
	}

	// 解析参数
	var targetUser *discordgo.User
	var amount int64
	var reason string = "管理员扣除"

	for _, opt := range options {
		switch opt.Name {
		case "user":
			targetUser = opt.UserValue(s)
		case "amount":
			amount = opt.IntValue()
		case "reason":
			reason = opt.StringValue()
		}
	}

	if targetUser == nil {
		h.respondError(s, i, "请指定目标用户")
		return
	}

	if amount <= 0 {
		h.respondError(s, i, "积分数量必须大于0")
		return
	}

	// 获取或创建目标用户
	user, err := h.userRepo.GetOrCreate(
		targetUser.ID,
		targetUser.Username,
		nil, nil,
	)
	if err != nil {
		logger.Errorf("获取用户失败: %v", err)
		h.respondError(s, i, "获取用户信息失败")
		return
	}

	// 获取当前积分
	points, err := h.userRepo.GetUserPoints(user.ID, h.guildID)
	if err != nil {
		logger.Errorf("获取积分失败: %v", err)
		h.respondError(s, i, "获取积分信息失败")
		return
	}

	// 检查积分是否足够
	if points.AvailablePoints < int(amount) {
		h.respondError(s, i, fmt.Sprintf("该用户积分不足！当前只有 %d 积分，无法扣除 %d 积分", points.AvailablePoints, amount))
		return
	}

	// 扣除积分
	err = h.userRepo.SpendPoints(
		user.ID, h.guildID, int(amount),
		"admin_remove",
		fmt.Sprintf("%s (操作人: %s)", reason, i.Member.User.Username),
		nil, nil,
	)
	if err != nil {
		logger.Errorf("扣除积分失败: %v", err)
		h.respondError(s, i, "扣除积分失败")
		return
	}

	// 获取更新后的积分
	points, _ = h.userRepo.GetUserPoints(user.ID, h.guildID)

	// 响应
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("✅ 成功扣除 <@%s> 的 **%d** 积分\n📝 原因: %s\n💰 当前积分: **%d**", targetUser.ID, amount, reason, points.AvailablePoints),
		},
	})
	if err != nil {
		logger.Errorf("响应命令失败: %v", err)
	}

	logger.Infof("管理员 %s 扣除 %s %d 积分，原因: %s", i.Member.User.Username, targetUser.Username, amount, reason)
}
