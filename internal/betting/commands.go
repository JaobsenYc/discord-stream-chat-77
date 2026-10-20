package betting

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/database"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// CommandHandler 命令处理器
type CommandHandler struct {
	service  *Service
	session  *discordgo.Session
	guildID  string
	userRepo *database.UserRepository
}

// NewCommandHandler 创建命令处理器
func NewCommandHandler(service *Service, session *discordgo.Session, guildID string, userRepo *database.UserRepository) *CommandHandler {
	return &CommandHandler{
		service:  service,
		session:  session,
		guildID:  guildID,
		userRepo: userRepo,
	}
}

// RegisterCommands 注册命令
func (h *CommandHandler) RegisterCommands() error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "bet",
			Description: "LOL比赛竞猜系统",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "create",
					Description: "创建竞猜",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "team_a",
							Description: "队伍A名称",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "team_b",
							Description: "队伍B名称",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "name",
							Description: "竞猜名称（可选）",
							Required:    false,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "list",
					Description: "查看当前活跃的竞猜",
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

// HandleCommand 处理命令
func (h *CommandHandler) HandleCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.ApplicationCommandData().Name != "bet" {
		return
	}

	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		return
	}

	switch options[0].Name {
	case "create":
		h.handleCreate(s, i, options[0].Options)
	case "list":
		h.handleList(s, i)
	}
}

// handleCreate 处理创建竞猜命令
func (h *CommandHandler) handleCreate(s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) {
	// 解析参数
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	teamA := optionMap["team_a"].StringValue()
	teamB := optionMap["team_b"].StringValue()

	var name *string
	if nameOpt, ok := optionMap["name"]; ok {
		nameValue := nameOpt.StringValue()
		name = &nameValue
	}

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

	// 创建竞猜（不再需要语音频道）
	bet, err := h.service.CreateBet(CreateBetRequest{
		Name:             name,
		GuildID:          h.guildID,
		ChannelID:        i.ChannelID,
		CreatorID:        user.ID,
		CreatorDiscordID: i.Member.User.ID,
		TeamAName:        teamA,
		TeamBName:        teamB,
	})

	if err != nil {
		logger.Errorf("创建竞猜失败: %v", err)
		h.respondError(s, i, fmt.Sprintf("创建竞猜失败: %v", err))
		return
	}

	// 创建嵌入消息
	embed := h.createBetEmbed(bet)

	// 创建按钮
	components := h.createBettingComponents(bet)

	// 响应命令
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
		},
	})

	if err != nil {
		logger.Errorf("响应命令失败: %v", err)
		return
	}

	// 获取刚才发送的消息ID并更新到数据库
	msg, err := s.InteractionResponse(i.Interaction)
	if err == nil && msg != nil {
		bet.MessageID = &msg.ID
		_ = h.service.betRepo.Update(bet)
	}
}

// handleList 处理查看竞猜列表命令
func (h *CommandHandler) handleList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	bets, err := h.service.GetActiveBets(h.guildID)
	if err != nil {
		logger.Errorf("获取竞猜列表失败: %v", err)
		h.respondError(s, i, "获取竞猜列表失败")
		return
	}

	if len(bets) == 0 {
		h.respondEphemeral(s, i, "当前没有活跃的竞猜")
		return
	}

	// 创建列表嵌入
	embed := &discordgo.MessageEmbed{
		Title:       "🎮 当前竞猜列表",
		Description: fmt.Sprintf("共有 %d 个活跃竞猜", len(bets)),
		Color:       0x3498db,
		Fields:      []*discordgo.MessageEmbedField{},
	}

	for _, bet := range bets {
		status := "📝 下注中"
		if bet.Status == database.BetStatusLocked {
			status = "🔒 已锁定"
		}

		field := &discordgo.MessageEmbedField{
			Name: fmt.Sprintf("%s - %s", getBetDisplayName(&bet), status),
			Value: fmt.Sprintf(
				"🔵 %s: %d积分 (%d人)\n🔴 %s: %d积分 (%d人)\n创建者: <@%s>",
				bet.TeamAName, bet.TeamATotalBet, countTeamParticipants(&bet, database.TeamChoiceA),
				bet.TeamBName, bet.TeamBTotalBet, countTeamParticipants(&bet, database.TeamChoiceB),
				bet.CreatorDiscordID,
			),
			Inline: false,
		}
		embed.Fields = append(embed.Fields, field)
	}

	h.respondEphemeral(s, i, "", embed)
}

// createBetEmbed 创建竞猜嵌入消息
func (h *CommandHandler) createBetEmbed(bet *database.Bet) *discordgo.MessageEmbed {
	calculator := NewCalculator()

	oddsA := calculator.CalculateOdds(bet.TeamATotalBet, bet.TeamBTotalBet)
	oddsB := calculator.CalculateOdds(bet.TeamBTotalBet, bet.TeamATotalBet)

	// 从数据库获取实际参与人数
	countA := h.getTeamParticipantCount(bet.ID, database.TeamChoiceA)
	countB := h.getTeamParticipantCount(bet.ID, database.TeamChoiceB)

	description := fmt.Sprintf("**%s** vs **%s**\n\n点击下方按钮下注（%d-%d积分）",
		bet.TeamAName, bet.TeamBName, database.MinBetAmount, database.MaxBetAmount)

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("🎮 %s", getBetDisplayName(bet)),
		Description: description,
		Color:       0x3498db,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name: fmt.Sprintf("🔵 %s", bet.TeamAName),
				Value: fmt.Sprintf(
					"下注总额: %d积分\n参与人数: %d人\n当前赔率: %s",
					bet.TeamATotalBet,
					countA,
					calculator.FormatOdds(oddsA),
				),
				Inline: true,
			},
			{
				Name: fmt.Sprintf("🔴 %s", bet.TeamBName),
				Value: fmt.Sprintf(
					"下注总额: %d积分\n参与人数: %d人\n当前赔率: %s",
					bet.TeamBTotalBet,
					countB,
					calculator.FormatOdds(oddsB),
				),
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("创建者: %s | 竞猜ID: %s", bet.CreatorDiscordID, bet.ID.String()[:8]),
		},
	}

	return embed
}

// getTeamParticipantCount 获取队伍实际参与人数
func (h *CommandHandler) getTeamParticipantCount(betID uuid.UUID, team string) int {
	count, err := h.service.betRepo.GetTeamParticipantCount(betID, team)
	if err != nil {
		logger.Errorf("获取队伍参与人数失败: %v", err)
		return 0
	}
	return count
}

// createBettingComponents 创建下注阶段的按钮
func (h *CommandHandler) createBettingComponents(bet *database.Bet) []discordgo.MessageComponent {
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					CustomID: fmt.Sprintf("bet_place_%s_A", bet.ID.String()),
					Label:    fmt.Sprintf("下注 %s", bet.TeamAName),
					Style:    discordgo.PrimaryButton,
					Emoji:    &discordgo.ComponentEmoji{Name: "🔵"},
				},
				discordgo.Button{
					CustomID: fmt.Sprintf("bet_place_%s_B", bet.ID.String()),
					Label:    fmt.Sprintf("下注 %s", bet.TeamBName),
					Style:    discordgo.PrimaryButton,
					Emoji:    &discordgo.ComponentEmoji{Name: "🔴"},
				},
				discordgo.Button{
					CustomID: fmt.Sprintf("bet_lock_%s", bet.ID.String()),
					Label:    "开始比赛",
					Style:    discordgo.SuccessButton,
					Emoji:    &discordgo.ComponentEmoji{Name: "🔒"},
				},
				discordgo.Button{
					CustomID: fmt.Sprintf("bet_cancel_%s", bet.ID.String()),
					Label:    "取消竞猜",
					Style:    discordgo.DangerButton,
					Emoji:    &discordgo.ComponentEmoji{Name: "❌"},
				},
			},
		},
	}
}

// 辅助函数
func (h *CommandHandler) respondError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "❌ " + message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func (h *CommandHandler) respondEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, message string, embeds ...*discordgo.MessageEmbed) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Embeds:  embeds,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func getBetDisplayName(bet *database.Bet) string {
	if bet.Name != nil && *bet.Name != "" {
		return *bet.Name
	}
	return fmt.Sprintf("%s vs %s", bet.TeamAName, bet.TeamBName)
}

// SendBetMessage 发送竞猜消息到指定频道（带下注按钮）
// 供 riotwatch 等外部模块调用
func (h *CommandHandler) SendBetMessage(channelID string, bet *database.Bet) error {
	// 创建嵌入消息
	embed := h.createBetEmbed(bet)

	// 创建按钮
	components := h.createBettingComponents(bet)

	// 发送消息
	msg, err := h.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: components,
	})
	if err != nil {
		return fmt.Errorf("发送竞猜消息失败: %w", err)
	}

	// 保存 message ID 到数据库
	bet.MessageID = &msg.ID
	if err := h.service.betRepo.Update(bet); err != nil {
		logger.Warnf("更新竞猜消息ID失败: %v", err)
	}

	return nil
}

// countTeamParticipants 已废弃，使用 getTeamParticipantCount 替代
func countTeamParticipants(bet *database.Bet, team string) int {
	// 使用 TotalParticipants 字段，该字段在每次下注时递增
	// 注意：这个方法无法区分队伍，只能返回总人数的估算
	// 对于精确统计，应该使用 CommandHandler.getTeamParticipantCount
	return 0
}
