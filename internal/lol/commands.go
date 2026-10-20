package lol

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/config"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// CommandHandler LOL内战命令处理器
type CommandHandler struct {
	manager *Manager
	session *discordgo.Session
	guildID string
	config  *config.Config
}

// NewCommandHandler 创建命令处理器
func NewCommandHandler(manager *Manager, session *discordgo.Session, guildID string, cfg *config.Config) *CommandHandler {
	return &CommandHandler{
		manager: manager,
		session: session,
		guildID: guildID,
		config:  cfg,
	}
}

// RegisterCommands 注册Slash Commands
func (h *CommandHandler) RegisterCommands() error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "lol",
			Description: "英雄联盟相关命令",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "start",
					Description: "开始内战",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "type",
							Description: "比赛类型",
							Required:    true,
							Choices: []*discordgo.ApplicationCommandOptionChoice{
								{
									Name:  "custom",
									Value: "custom",
								},
							},
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "name",
							Description: "内战名称 (可选)",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionBoolean,
							Name:        "spy",
							Description: "是否开启内鬼局 (可选)",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionInteger,
							Name:        "spy_num",
							Description: "每队内鬼数量 (1-4，默认为1)",
							Required:    false,
							MinValue:    &[]float64{1}[0],
							MaxValue:    4,
						},
					},
				},
			},
		},
	}

	for _, cmd := range commands {
		_, err := h.session.ApplicationCommandCreate(h.session.State.User.ID, h.guildID, cmd)
		if err != nil {
			return fmt.Errorf("创建命令 %s 失败: %w", cmd.Name, err)
		}
		logger.Infof("已注册Slash Command: %s", cmd.Name)
	}

	return nil
}

// HandleSlashCommand 处理Slash Command
func (h *CommandHandler) HandleSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.ApplicationCommandData().Name != "lol" {
		return
	}

	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		return
	}

	switch options[0].Name {
	case "start":
		h.handleStartCommand(s, i, options[0].Options)
	default:
		h.respondError(s, i, "未知的子命令")
	}
}

// handleStartCommand 处理开始内战命令
func (h *CommandHandler) handleStartCommand(s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) {
	// 检查LOL内战功能是否启用
	if !h.config.IsLOLCustomEnabled() {
		h.respondError(s, i, "LOL内战功能未启用")
		return
	}

	// 检查当前频道是否允许使用LOL内战功能
	if !h.config.IsLOLCustomAllowedChannel(i.ChannelID) {
		h.respondError(s, i, "当前频道不允许使用LOL内战功能")
		return
	}

	// 检查参数
	if len(options) == 0 {
		h.respondError(s, i, "缺少必要参数")
		return
	}

	var matchName string
	var isSpyMode bool
	var spyCount int = 1 // 默认每队1个内鬼

	matchType := options[0].StringValue()

	// 解析可选参数
	for _, option := range options[1:] {
		switch option.Name {
		case "name":
			matchName = option.StringValue()
		case "spy":
			isSpyMode = option.BoolValue()
		case "spy_num":
			spyCount = int(option.IntValue())
		}
	}

	if matchType != "custom" {
		h.respondError(s, i, "目前只支持自定义比赛")
		return
	}

	// 验证内鬼局参数
	if isSpyMode {
		if spyCount < 1 || spyCount > 4 {
			h.respondError(s, i, "每队内鬼数量必须在1-4之间")
			return
		}
		if spyCount*2 >= 10 {
			h.respondError(s, i, "内鬼数量过多，每队最多4个内鬼")
			return
		}
	}

	// 创建内战
	match, err := h.manager.CreateMatch(i.ChannelID, i.Member.User.ID, matchName, isSpyMode, spyCount)
	if err != nil {
		h.respondError(s, i, fmt.Sprintf("创建内战失败: %v", err))
		return
	}

	// 生成招募消息
	embed := h.createRecruitmentEmbed(match)
	components := h.createRecruitmentComponents(match.ID)

	// 响应交互
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
		},
	})

	if err != nil {
		logger.Errorf("响应内战创建失败: %v", err)
		return
	}

	// 获取响应消息ID
	msg, err := s.InteractionResponse(i.Interaction)
	if err != nil {
		logger.Errorf("获取响应消息失败: %v", err)
		return
	}

	// 更新内战消息ID（经 Manager 加锁写入，处理器持有的是快照）
	h.manager.SetMessageID(match.ID, msg.ID)
	match.MessageID = msg.ID

	if match.IsSpyMode {
		logger.Infof("LOL内鬼局创建成功: %s (%s), 每队内鬼数: %d", match.ID, match.Name, match.SpyCount)
	} else {
		logger.Infof("LOL内战创建成功: %s (%s)", match.ID, match.Name)
	}
}

// HandleButtonInteraction 处理按钮交互
func (h *CommandHandler) HandleButtonInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	customID := i.MessageComponentData().CustomID

	// 解析自定义ID
	parts := strings.Split(customID, "_")
	if len(parts) < 3 || parts[0] != "lol" {
		return
	}

	action := parts[1]
	matchID := strings.Join(parts[2:], "_")

	switch action {
	case "join":
		h.handleJoinButton(s, i, matchID)
	case "leave":
		h.handleLeaveButton(s, i, matchID)
	case "cancel":
		h.handleCancelButton(s, i, matchID)
	}
}

// handleJoinButton 处理加入按钮
func (h *CommandHandler) handleJoinButton(s *discordgo.Session, i *discordgo.InteractionCreate, matchID string) {
	userID := i.Member.User.ID

	// 先检查内战是否存在
	match, exists := h.manager.GetMatch(matchID)
	if !exists {
		// 内战不存在或已被取消，更新消息显示取消状态
		cancelledEmbed := h.createCancelledEmbed()
		h.updateMessageToCancelled(s, i, cancelledEmbed)

		// 响应交互
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})
		return
	}

	// 检查内战状态
	if match.Status == StatusCancelled {
		// 内战已取消，更新消息显示取消状态
		cancelledEmbed := h.createCancelledEmbed()
		h.updateMessageToCancelled(s, i, cancelledEmbed)

		// 响应交互
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})
		return
	}

	err := h.manager.JoinMatch(matchID, userID)
	if err != nil {
		h.respondError(s, i, fmt.Sprintf("加入失败: %v", err))
		return
	}

	// 获取更新后的内战信息
	match, exists = h.manager.GetMatch(matchID)
	if !exists {
		h.respondError(s, i, "内战不存在")
		return
	}

	// 根据内战状态更新消息
	if match.Status == StatusInProgress {
		// 内战已开始，发送分组结果
		embed := h.createTeamResultEmbed(match)
		h.updateMessage(s, i, embed, nil)

		// 发送额外的通知消息
		h.sendTeamNotification(s, match)
	} else {
		// 更新招募消息
		embed := h.createRecruitmentEmbed(match)
		components := h.createRecruitmentComponents(match.ID)
		h.updateMessage(s, i, embed, components)
	}

	// 响应交互
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
}

// handleLeaveButton 处理离开按钮
func (h *CommandHandler) handleLeaveButton(s *discordgo.Session, i *discordgo.InteractionCreate, matchID string) {
	userID := i.Member.User.ID

	err, cancelled := h.manager.LeaveMatch(matchID, userID)
	if err != nil {
		h.respondError(s, i, fmt.Sprintf("离开失败: %v", err))
		return
	}

	// 如果内战被取消了（创建者离开）
	if cancelled {
		// 更新消息为取消状态
		cancelledEmbed := h.createCancelledEmbed()
		h.updateMessageToCancelled(s, i, cancelledEmbed)

		// 响应交互
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})
		return
	}

	// 获取更新后的内战信息
	match, exists := h.manager.GetMatch(matchID)
	if !exists {
		// 内战可能已被取消
		cancelledEmbed := h.createCancelledEmbed()
		h.updateMessageToCancelled(s, i, cancelledEmbed)

		// 响应交互
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})
		return
	}

	// 更新招募消息
	embed := h.createRecruitmentEmbed(match)
	components := h.createRecruitmentComponents(match.ID)
	h.updateMessage(s, i, embed, components)

	// 响应交互
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
}

// handleCancelButton 处理取消按钮
func (h *CommandHandler) handleCancelButton(s *discordgo.Session, i *discordgo.InteractionCreate, matchID string) {
	userID := i.Member.User.ID

	// 检查是否是创建者
	match, exists := h.manager.GetMatch(matchID)
	if !exists {
		h.respondError(s, i, "内战不存在")
		return
	}

	if match.CreatorID != userID {
		h.respondError(s, i, "只有创建者可以取消内战")
		return
	}

	err, cancelled := h.manager.LeaveMatch(matchID, userID) // 创建者离开会自动取消
	if err != nil {
		h.respondError(s, i, fmt.Sprintf("取消失败: %v", err))
		return
	}

	// 如果成功取消了内战
	if cancelled {
		// 更新消息为取消状态
		cancelledEmbed := h.createCancelledEmbed()
		h.updateMessageToCancelled(s, i, cancelledEmbed)
	}

	// 响应交互
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
}

// createRecruitmentEmbed 创建招募Embed
func (h *CommandHandler) createRecruitmentEmbed(match *CustomMatch) *discordgo.MessageEmbed {
	participantsCount := len(match.Participants)

	// 构建参与者列表
	var participantsList strings.Builder
	for i, participantID := range match.Participants {
		user, err := h.session.User(participantID)
		if err != nil {
			participantsList.WriteString(fmt.Sprintf("%d. <@%s>\n", i+1, participantID))
		} else {
			participantsList.WriteString(fmt.Sprintf("%d. %s\n", i+1, user.Username))
		}
	}

	// 如果没有参与者显示默认文本
	if participantsCount == 0 {
		participantsList.WriteString("暂无参与者")
	}

	// 使用内战名称作为标题，并显示是否为内鬼局
	var title string
	var description string

	if match.IsSpyMode {
		title = fmt.Sprintf("🕵️ %s 招募中", match.Name)
		description = fmt.Sprintf("点击下方按钮参与5v5内鬼局！每队将有 %d 个内鬼", match.SpyCount)
	} else {
		title = fmt.Sprintf("🎮 %s 招募中", match.Name)
		description = "点击下方按钮参与5v5内战！"
	}

	fields := []*discordgo.MessageEmbedField{
		{
			Name:   "👥 当前人数",
			Value:  fmt.Sprintf("%d/%d", participantsCount, match.MaxPlayers),
			Inline: true,
		},
		{
			Name:   "⏰ 创建时间",
			Value:  match.CreatedAt.Format("15:04:05"),
			Inline: true,
		},
	}

	// 如果是内鬼局，添加内鬼信息字段
	if match.IsSpyMode {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "🕵️ 内鬼模式",
			Value:  fmt.Sprintf("每队 %d 个内鬼", match.SpyCount),
			Inline: true,
		})
	}

	fields = append(fields, &discordgo.MessageEmbedField{
		Name:   "📋 参与者列表",
		Value:  participantsList.String(),
		Inline: false,
	})

	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       0x00ff00, // 绿色
		Fields:      fields,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("内战ID: %s", match.ID[7:13]),
		},
		Timestamp: match.CreatedAt.Format(time.RFC3339),
	}

	return embed
}

// createTeamResultEmbed 创建分组结果Embed
func (h *CommandHandler) createTeamResultEmbed(match *CustomMatch) *discordgo.MessageEmbed {
	var blueTeamList, redTeamList strings.Builder

	// 构建蓝队成员列表
	for i, memberID := range match.BlueTeam.Members {
		user, err := h.session.User(memberID)
		if err != nil {
			blueTeamList.WriteString(fmt.Sprintf("%d. <@%s>\n", i+1, memberID))
		} else {
			captain := ""
			if memberID == match.BlueTeam.Captain {
				captain = " 👑"
			}
			blueTeamList.WriteString(fmt.Sprintf("%d. %s%s\n", i+1, user.Username, captain))
		}
	}

	// 构建红队成员列表
	for i, memberID := range match.RedTeam.Members {
		user, err := h.session.User(memberID)
		if err != nil {
			redTeamList.WriteString(fmt.Sprintf("%d. <@%s>\n", i+1, memberID))
		} else {
			captain := ""
			if memberID == match.RedTeam.Captain {
				captain = " 👑"
			}
			redTeamList.WriteString(fmt.Sprintf("%d. %s%s\n", i+1, user.Username, captain))
		}
	}

	// 使用内战名称作为标题，并显示是否为内鬼局
	var title string
	var description string
	if match.IsSpyMode {
		title = fmt.Sprintf("🕵️ %s 分组完成！", match.Name)
		description = fmt.Sprintf("内鬼局分组已完成！每队已选出 %d 个内鬼并已私信通知，请各队员加入对应语音频道开始游戏！", match.SpyCount)
	} else {
		title = fmt.Sprintf("⚔️ %s 分组完成！", match.Name)
		description = "分组已完成，请各队员加入对应语音频道开始游戏！"
	}

	fields := []*discordgo.MessageEmbedField{
		{
			Name:   "🔵 蓝色方",
			Value:  blueTeamList.String(),
			Inline: true,
		},
		{
			Name:   "🔴 红色方",
			Value:  redTeamList.String(),
			Inline: true,
		},
	}

	// 如果是内鬼局，添加提示信息
	if match.IsSpyMode {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "🕵️ 内鬼提示",
			Value:  fmt.Sprintf("• 每队有 %d 个内鬼，身份已私信通知\n• 内鬼的目标是让自己的队伍输掉比赛\n• 比赛结束后将进行投票猜测内鬼", match.SpyCount),
			Inline: false,
		})
	}

	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       0xffd700, // 金色
		Fields:      fields,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("内战ID: %s | 开始时间: %s", match.ID[7:13], match.StartedAt.Format("15:04:05")),
		},
		Timestamp: match.StartedAt.Format(time.RFC3339),
	}

	// 添加语音频道信息
	if match.BlueTeam.VoiceChannelID != "" && match.RedTeam.VoiceChannelID != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "🎵 语音频道",
			Value:  fmt.Sprintf("🔵 <#%s>\n🔴 <#%s>", match.BlueTeam.VoiceChannelID, match.RedTeam.VoiceChannelID),
			Inline: false,
		})
	}

	return embed
}

// createRecruitmentComponents 创建招募按钮组件
func (h *CommandHandler) createRecruitmentComponents(matchID string) []discordgo.MessageComponent {
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "🎮 参与内战",
					Style:    discordgo.PrimaryButton,
					CustomID: fmt.Sprintf("lol_join_%s", matchID),
				},
				discordgo.Button{
					Label:    "❌ 离开",
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("lol_leave_%s", matchID),
				},
				discordgo.Button{
					Label:    "🚫 取消内战",
					Style:    discordgo.DangerButton,
					CustomID: fmt.Sprintf("lol_cancel_%s", matchID),
				},
			},
		},
	}
}

// sendTeamNotification 发送分组通知
func (h *CommandHandler) sendTeamNotification(s *discordgo.Session, match *CustomMatch) {
	// 创建通知消息
	notificationEmbed := &discordgo.MessageEmbed{
		Title:       "🎉 内战即将开始！",
		Description: "所有玩家已到齐，请加入对应的语音频道准备游戏！",
		Color:       0xff4500, // 橙红色
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "🔵 蓝队语音频道",
				Value:  fmt.Sprintf("<#%s>", match.BlueTeam.VoiceChannelID),
				Inline: true,
			},
			{
				Name:   "🔴 红队语音频道",
				Value:  fmt.Sprintf("<#%s>", match.RedTeam.VoiceChannelID),
				Inline: true,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// 提及所有参与者
	var mentions strings.Builder
	for _, participantID := range match.Participants {
		mentions.WriteString(fmt.Sprintf("<@%s> ", participantID))
	}

	// 发送通知消息
	_, err := s.ChannelMessageSendComplex(match.ChannelID, &discordgo.MessageSend{
		Content: mentions.String(),
		Embeds:  []*discordgo.MessageEmbed{notificationEmbed},
	})

	if err != nil {
		logger.Errorf("发送分组通知失败: %v", err)
	}
}

// 辅助方法
func (h *CommandHandler) respondError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "❌ " + message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func (h *CommandHandler) updateMessage(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed, components []discordgo.MessageComponent) {
	embeds := []*discordgo.MessageEmbed{embed}
	_, err := s.ChannelMessageEditComplex(&discordgo.MessageEdit{
		Channel:    i.ChannelID,
		ID:         i.Message.ID,
		Embeds:     &embeds,
		Components: &components,
	})

	if err != nil {
		logger.Errorf("更新消息失败: %v", err)
	}
}

func (h *CommandHandler) updateMessageToCancelled(s *discordgo.Session, i *discordgo.InteractionCreate, cancelledEmbed *discordgo.MessageEmbed) {
	embeds := []*discordgo.MessageEmbed{cancelledEmbed}
	_, err := s.ChannelMessageEditComplex(&discordgo.MessageEdit{
		Channel:    i.ChannelID,
		ID:         i.Message.ID,
		Embeds:     &embeds,
		Components: nil, // 清空按钮
	})

	if err != nil {
		logger.Errorf("更新消息为取消状态失败: %v", err)
	}
}

func (h *CommandHandler) createCancelledEmbed() *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       "🚫 内战已取消",
		Description: "内战已被创建者取消",
		Color:       0xff0000, // 红色
		Timestamp:   time.Now().Format(time.RFC3339),
	}
}
