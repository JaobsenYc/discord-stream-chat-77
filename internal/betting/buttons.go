package betting

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/database"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// HandleButtonInteraction 处理按钮交互
func (h *CommandHandler) HandleButtonInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	customID := i.MessageComponentData().CustomID

	// 解析按钮ID
	parts := strings.Split(customID, "_")
	if len(parts) < 3 {
		logger.Errorf("无效的按钮ID: %s", customID)
		return
	}

	if parts[0] != "bet" {
		return
	}

	action := parts[1] // place, lock, cancel, settle
	betIDStr := parts[2]

	// 解析竞猜ID
	betID, err := uuid.Parse(betIDStr)
	if err != nil {
		h.respondButtonError(s, i, "无效的竞猜ID")
		return
	}

	switch action {
	case "place":
		if len(parts) < 4 {
			h.respondButtonError(s, i, "无效的按钮参数")
			return
		}
		team := parts[3] // A or B
		h.handlePlaceButton(s, i, betID, team)
	case "lock":
		h.handleLockButton(s, i, betID)
	case "cancel":
		h.handleCancelButton(s, i, betID)
	case "settle":
		if len(parts) < 4 {
			h.respondButtonError(s, i, "无效的按钮参数")
			return
		}
		winnerTeam := parts[3] // A or B
		h.handleSettleButton(s, i, betID, winnerTeam)
	default:
		h.respondButtonError(s, i, "未知的操作")
	}
}

// handlePlaceButton 处理下注按钮
func (h *CommandHandler) handlePlaceButton(s *discordgo.Session, i *discordgo.InteractionCreate, betID uuid.UUID, team string) {
	// 显示模态框让用户输入下注金额
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: fmt.Sprintf("bet_amount_%s_%s", betID.String(), team),
			Title:    "输入下注金额",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "amount",
							Label:       fmt.Sprintf("下注积分 (%d-%d)", database.MinBetAmount, database.MaxBetAmount),
							Style:       discordgo.TextInputShort,
							Placeholder: "请输入下注金额",
							Required:    true,
							MinLength:   3,
							MaxLength:   3,
						},
					},
				},
			},
		},
	})

	if err != nil {
		logger.Errorf("显示模态框失败: %v", err)
	}
}

// HandleModalSubmit 处理模态框提交
func (h *CommandHandler) HandleModalSubmit(s *discordgo.Session, i *discordgo.InteractionCreate) {
	customID := i.ModalSubmitData().CustomID

	// 解析模态框ID
	parts := strings.Split(customID, "_")
	if len(parts) < 4 {
		logger.Errorf("无效的模态框ID: %s", customID)
		return
	}

	if parts[0] != "bet" || parts[1] != "amount" {
		return
	}

	betIDStr := parts[2]
	team := parts[3]

	// 解析竞猜ID
	betID, err := uuid.Parse(betIDStr)
	if err != nil {
		h.respondButtonError(s, i, "无效的竞猜ID")
		return
	}

	// 获取用户输入的金额
	data := i.ModalSubmitData()
	amountStr := ""
	for _, component := range data.Components {
		if actionRow, ok := component.(*discordgo.ActionsRow); ok {
			for _, c := range actionRow.Components {
				if textInput, ok := c.(*discordgo.TextInput); ok && textInput.CustomID == "amount" {
					amountStr = textInput.Value
					break
				}
			}
		}
	}

	if amountStr == "" {
		h.respondButtonError(s, i, "请输入下注金额")
		return
	}

	// 解析金额
	amount, err := strconv.Atoi(amountStr)
	if err != nil {
		h.respondButtonError(s, i, "无效的金额格式")
		return
	}

	// 获取或创建用户
	user, err := h.userRepo.GetOrCreate(
		i.Member.User.ID,
		i.Member.User.Username,
		nil, nil,
	)
	if err != nil {
		logger.Errorf("获取用户失败: %v", err)
		h.respondButtonError(s, i, "获取用户信息失败")
		return
	}

	// 执行下注
	result, err := h.service.PlaceBet(betID, user.ID, i.Member.User.ID, h.guildID, team, amount)
	if err != nil {
		logger.Errorf("下注失败: %v", err)
		h.respondButtonError(s, i, fmt.Sprintf("下注失败: %v", err))
		return
	}

	if !result.Success {
		h.respondButtonError(s, i, result.Message)
		return
	}

	// 更新竞猜消息
	bet, err := h.service.GetBet(betID)
	if err == nil && bet != nil {
		h.updateBetMessage(s, i.Message, bet)
	}

	// 响应成功
	message := fmt.Sprintf("✅ %s\n💰 剩余积分: %d\n📊 预估收益: %d积分",
		result.Message, result.RemainingPoints, result.EstimatedPayout)

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	if err != nil {
		logger.Errorf("响应模态框失败: %v", err)
	}
}

// handleLockButton 处理锁定按钮
func (h *CommandHandler) handleLockButton(s *discordgo.Session, i *discordgo.InteractionCreate, betID uuid.UUID) {
	// 获取用户
	user, err := h.userRepo.GetOrCreate(
		i.Member.User.ID,
		i.Member.User.Username,
		nil, nil,
	)
	if err != nil {
		logger.Errorf("获取用户失败: %v", err)
		h.respondButtonError(s, i, "获取用户信息失败")
		return
	}

	// 锁定竞猜
	err = h.service.LockBetting(betID, user.ID)
	if err != nil {
		h.respondButtonError(s, i, err.Error())
		return
	}

	// 更新消息
	bet, err := h.service.GetBet(betID)
	if err != nil {
		logger.Errorf("获取竞猜失败: %v", err)
		return
	}

	// 更新为锁定状态的按钮
	components := h.createLockedComponents(bet)
	embed := h.createBetEmbed(bet)

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
		},
	})

	if err != nil {
		logger.Errorf("更新消息失败: %v", err)
	}
}

// handleCancelButton 处理取消按钮
func (h *CommandHandler) handleCancelButton(s *discordgo.Session, i *discordgo.InteractionCreate, betID uuid.UUID) {
	// 获取用户
	user, err := h.userRepo.GetOrCreate(
		i.Member.User.ID,
		i.Member.User.Username,
		nil, nil,
	)
	if err != nil {
		logger.Errorf("获取用户失败: %v", err)
		h.respondButtonError(s, i, "获取用户信息失败")
		return
	}

	// 取消竞猜
	err = h.service.CancelBet(betID, user.ID)
	if err != nil {
		h.respondButtonError(s, i, err.Error())
		return
	}

	// 更新消息
	bet, err := h.service.GetBet(betID)
	if err != nil {
		logger.Errorf("获取竞猜失败: %v", err)
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("🎮 %s - ❌ 已取消", getBetDisplayName(bet)),
		Description: "竞猜已取消，所有积分已退还",
		Color:       0x95a5a6,
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: []discordgo.MessageComponent{},
		},
	})

	if err != nil {
		logger.Errorf("更新消息失败: %v", err)
	}
}

// handleSettleButton 处理结算按钮
func (h *CommandHandler) handleSettleButton(s *discordgo.Session, i *discordgo.InteractionCreate, betID uuid.UUID, winnerTeam string) {
	// 获取用户
	user, err := h.userRepo.GetOrCreate(
		i.Member.User.ID,
		i.Member.User.Username,
		nil, nil,
	)
	if err != nil {
		logger.Errorf("获取用户失败: %v", err)
		h.respondButtonError(s, i, "获取用户信息失败")
		return
	}

	// 结算竞猜
	result, err := h.service.SettleBet(betID, user.ID, winnerTeam)
	if err != nil {
		h.respondButtonError(s, i, err.Error())
		return
	}

	// 获取竞猜信息
	bet, err := h.service.GetBet(betID)
	if err != nil {
		logger.Errorf("获取竞猜失败: %v", err)
		return
	}

	// 创建结算消息
	embed := h.createSettlementEmbed(bet, result)

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: []discordgo.MessageComponent{},
		},
	})

	if err != nil {
		logger.Errorf("更新消息失败: %v", err)
	}
}

// createLockedComponents 创建锁定状态的按钮
func (h *CommandHandler) createLockedComponents(bet *database.Bet) []discordgo.MessageComponent {
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					CustomID: fmt.Sprintf("bet_settle_%s_A", bet.ID.String()),
					Label:    fmt.Sprintf("%s获胜", bet.TeamAName),
					Style:    discordgo.SuccessButton,
					Emoji:    &discordgo.ComponentEmoji{Name: "🏆"},
				},
				discordgo.Button{
					CustomID: fmt.Sprintf("bet_settle_%s_B", bet.ID.String()),
					Label:    fmt.Sprintf("%s获胜", bet.TeamBName),
					Style:    discordgo.SuccessButton,
					Emoji:    &discordgo.ComponentEmoji{Name: "🏆"},
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

// createSettlementEmbed 创建结算消息
func (h *CommandHandler) createSettlementEmbed(bet *database.Bet, result *SettlementResult) *discordgo.MessageEmbed {
	var winnerTeamName string
	if result.WinnerTeam == database.TeamChoiceA {
		winnerTeamName = bet.TeamAName
	} else {
		winnerTeamName = bet.TeamBName
	}

	// 调试日志
	logger.Infof("结算消息: Winners=%d, Losers=%d, Payouts=%v", len(result.Winners), len(result.Losers), result.Payouts)

	// 构建获胜者列表（显示拿回的总积分）
	winnerLines := make([]string, 0)
	for _, winner := range result.Winners {
		payout := 0
		if result.Payouts != nil {
			payout = result.Payouts[winner.UserID]
		}

		logger.Infof("获胜者: %s, BetAmount=%d, Payout=%d", winner.DiscordID, winner.BetAmount, payout)

		if winner.DiscordID == database.SystemDiscordID {
			// 系统账户
			winnerLines = append(winnerLines, fmt.Sprintf("🤖 系统对冲: 拿回 %d积分", payout))
		} else {
			// 普通用户
			winnerLines = append(winnerLines, fmt.Sprintf("<@%s>: 拿回 %d积分", winner.DiscordID, payout))
		}
	}

	// 构建失败者列表（显示每个人失去的积分）
	loserLines := make([]string, 0)
	for _, loser := range result.Losers {
		if loser.DiscordID == database.SystemDiscordID {
			// 系统账户
			loserLines = append(loserLines, fmt.Sprintf("🤖 系统对冲: -%d积分", loser.BetAmount))
		} else {
			// 普通用户
			loserLines = append(loserLines, fmt.Sprintf("<@%s>: -%d积分", loser.DiscordID, loser.BetAmount))
		}
	}

	// 构建奖池信息（包含抽水说明）
	poolInfo := fmt.Sprintf("%d积分", result.TotalPool)
	if result.Commission > 0 {
		poolInfo = fmt.Sprintf("%d积分 (管理费 %d积分)", result.TotalPool, result.Commission)
	}

	// 构建嵌入消息字段
	fields := []*discordgo.MessageEmbedField{}

	// 只有有获胜者时才显示获胜者列表
	if len(winnerLines) > 0 {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "🎉 获胜者",
			Value:  strings.Join(winnerLines, "\n"),
			Inline: false,
		})
	}

	// 只有有失败者时才显示失败者列表
	if len(loserLines) > 0 {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "💔 失败者",
			Value:  strings.Join(loserLines, "\n"),
			Inline: false,
		})
	}

	// 添加奖池和参与人数信息
	fields = append(fields,
		&discordgo.MessageEmbedField{
			Name:   "💰 奖池",
			Value:  poolInfo,
			Inline: true,
		},
		&discordgo.MessageEmbedField{
			Name:   "👥 参与人数",
			Value:  fmt.Sprintf("%d人", result.WinnerCount+result.LoserCount),
			Inline: true,
		},
	)

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("🏆 %s - 已结算", getBetDisplayName(bet)),
		Description: fmt.Sprintf("**%s** 获胜！", winnerTeamName),
		Color:       0x2ecc71,
		Fields:      fields,
	}

	return embed
}

// updateBetMessage 更新竞猜消息
func (h *CommandHandler) updateBetMessage(s *discordgo.Session, msg *discordgo.Message, bet *database.Bet) {
	embed := h.createBetEmbed(bet)

	var components []discordgo.MessageComponent
	if bet.Status == database.BetStatusBetting {
		components = h.createBettingComponents(bet)
	} else if bet.Status == database.BetStatusLocked {
		components = h.createLockedComponents(bet)
	}

	_, err := s.ChannelMessageEditComplex(&discordgo.MessageEdit{
		Channel:    msg.ChannelID,
		ID:         msg.ID,
		Embeds:     &[]*discordgo.MessageEmbed{embed},
		Components: &components,
	})

	if err != nil {
		logger.Errorf("更新消息失败: %v", err)
	}
}

// 辅助函数
func (h *CommandHandler) respondButtonError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "❌ " + message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}
