package bot

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// registerCheckInCommands 注册签到相关的 Slash Commands
func (b *Bot) registerCheckInCommands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name:        "checkin",
			Description: "签到系统",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "daily",
					Description: "每日签到",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "status",
					Description: "查看签到状态",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "leaderboard",
					Description: "查看签到排行榜",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
					Name:        "admin",
					Description: "管理员设置",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionSubCommand,
							Name:        "set-reward",
							Description: "设置签到奖励",
							Options: []*discordgo.ApplicationCommandOption{
								{
									Type:        discordgo.ApplicationCommandOptionInteger,
									Name:        "base",
									Description: "基础积分奖励",
									Required:    true,
									MinValue:    &[]float64{1}[0],
									MaxValue:    100,
								},
								{
									Type:        discordgo.ApplicationCommandOptionInteger,
									Name:        "bonus",
									Description: "连续签到每天额外奖励",
									Required:    true,
									MinValue:    &[]float64{0}[0],
									MaxValue:    50,
								},
							},
						},
					},
				},
			},
		},
	}
}

// handleCheckInCommand 处理签到命令
func (b *Bot) handleCheckInCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if b.repository == nil {
		b.respondToInteraction(s, i, "❌ 数据库连接不可用", true)
		return
	}

	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		b.respondToInteraction(s, i, "❌ 请选择一个子命令", true)
		return
	}

	switch options[0].Name {
	case "daily":
		b.handleDailyCheckIn(s, i)
	case "status":
		b.handleCheckInStatus(s, i)
	case "leaderboard":
		b.handleCheckInLeaderboard(s, i)
	case "admin":
		b.handleCheckInAdmin(s, i, options[0].Options)
	default:
		b.respondToInteraction(s, i, "❌ 未知的子命令", true)
	}
}

// handleDailyCheckIn 处理每日签到
func (b *Bot) handleDailyCheckIn(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// 获取或创建用户
	user, err := b.repository.User.GetOrCreate(
		i.Member.User.ID,
		i.Member.User.Username,
		&i.Member.User.Discriminator,
		nil,
	)
	if err != nil {
		logger.Errorf("获取用户失败: %v", err)
		b.respondToInteraction(s, i, "❌ 获取用户信息失败", true)
		return
	}

	// 签到积分配置：基础100分，每天+10分，封顶200分
	basePoints := 100
	bonusPoints := 10

	// 执行签到
	result, err := b.repository.CheckIn.PerformCheckIn(
		user.ID,
		i.Member.User.ID,
		i.GuildID,
		i.ChannelID,
		basePoints,
		bonusPoints,
	)
	if err != nil {
		logger.Errorf("签到失败: %v", err)
		b.respondToInteraction(s, i, "❌ 签到失败，请稍后重试", true)
		return
	}

	// 构建响应消息
	embed := &discordgo.MessageEmbed{
		Title: "📅 每日签到",
		Color: 0x00ff00,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: i.Member.User.AvatarURL(""),
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "状态",
				Value:  result.Message,
				Inline: false,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "LFG Bot 签到系统",
		},
	}

	if result.Success {
		embed.Color = 0x00ff00
		embed.Fields = append(embed.Fields,
			&discordgo.MessageEmbedField{
				Name:   "🎁 获得积分",
				Value:  fmt.Sprintf("%d 积分", result.PointsEarned),
				Inline: true,
			},
			&discordgo.MessageEmbedField{
				Name:   "🔥 连续签到",
				Value:  fmt.Sprintf("%d 天", result.ConsecutiveDays),
				Inline: true,
			},
			&discordgo.MessageEmbedField{
				Name:   "💰 当前积分",
				Value:  fmt.Sprintf("%d 积分", result.TotalPoints),
				Inline: true,
			},
			&discordgo.MessageEmbedField{
				Name:   "📊 累计签到",
				Value:  fmt.Sprintf("%d 次", result.TotalCheckIns),
				Inline: true,
			},
		)

		if result.IsFirstTime {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   "🎉 特别提示",
				Value:  "恭喜你的第一次签到！",
				Inline: false,
			})
		}

		// 触发成就系统检查
		go func() {
			err := b.achievementService.OnCheckIn(user.ID, i.Member.User.ID, i.GuildID, result.ConsecutiveDays)
			if err != nil {
				logger.Errorf("更新签到成就失败: %v", err)
			}
		}()
	} else {
		embed.Color = 0xff9900
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		logger.Errorf("响应签到命令失败: %v", err)
	}
}

// handleCheckInStatus 处理签到状态查询
func (b *Bot) handleCheckInStatus(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// 获取用户
	user, err := b.repository.User.GetByDiscordID(i.Member.User.ID)
	if err != nil {
		b.respondToInteraction(s, i, "❌ 用户不存在，请先进行签到", true)
		return
	}

	// 获取签到状态
	status, err := b.repository.CheckIn.GetCheckInStatus(user.ID, i.GuildID)
	if err != nil {
		logger.Errorf("获取签到状态失败: %v", err)
		b.respondToInteraction(s, i, "❌ 获取签到状态失败", true)
		return
	}

	// 构建响应消息
	embed := &discordgo.MessageEmbed{
		Title: "👀 签到状态",
		Color: 0x0099ff,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: i.Member.User.AvatarURL(""),
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "今日签到",
				Value:  map[bool]string{true: "✅ 已签到", false: "❌ 未签到"}[status.HasCheckedInToday],
				Inline: true,
			},
			{
				Name:   "💰 当前积分",
				Value:  fmt.Sprintf("%d 积分", status.TotalPoints),
				Inline: true,
			},
			{
				Name:   "🔥 连续签到",
				Value:  fmt.Sprintf("%d 天", status.ConsecutiveDays),
				Inline: true,
			},
			{
				Name:   "📊 累计签到",
				Value:  fmt.Sprintf("%d 次", status.TotalCheckIns),
				Inline: true,
			},
			{
				Name:   "🏆 最大连续",
				Value:  fmt.Sprintf("%d 天", status.MaxConsecutiveDays),
				Inline: true,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "LFG Bot 签到系统",
		},
	}

	if status.LastCheckInDate != nil {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "🕐 上次签到",
			Value:  status.LastCheckInDate.In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05"),
			Inline: true,
		})
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		logger.Errorf("响应签到状态命令失败: %v", err)
	}
}

// handleCheckInLeaderboard 处理签到排行榜
func (b *Bot) handleCheckInLeaderboard(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// 获取排行榜
	leaderboard, err := b.repository.CheckIn.GetCheckInLeaderboard(i.GuildID, 10)
	if err != nil {
		logger.Errorf("获取签到排行榜失败: %v", err)
		b.respondToInteraction(s, i, "❌ 获取排行榜失败", true)
		return
	}

	if len(leaderboard) == 0 {
		b.respondToInteraction(s, i, "📊 暂无签到记录", true)
		return
	}

	// 构建排行榜消息
	embed := &discordgo.MessageEmbed{
		Title:       "🏆 签到排行榜 (前10名)",
		Color:       0xffd700,
		Description: "根据累计签到次数排序",
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "LFG Bot 签到系统",
		},
	}

	for i, entry := range leaderboard {
		rankEmoji := "🥉"
		if i == 0 {
			rankEmoji = "🥇"
		} else if i == 1 {
			rankEmoji = "🥈"
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name: fmt.Sprintf("%s #%d %s", rankEmoji, entry.Rank, entry.Username),
			Value: fmt.Sprintf("📊 签到: %d次\n🔥 连续: %d天\n💰 积分: %d",
				entry.TotalCheckIns, entry.ConsecutiveDays, entry.TotalPoints),
			Inline: true,
		})
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
	if err != nil {
		logger.Errorf("响应签到排行榜命令失败: %v", err)
	}
}

// handleCheckInAdmin 处理管理员命令
func (b *Bot) handleCheckInAdmin(s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) {
	// 检查权限
	if !b.hasAdminPermission(i.Member) {
		b.respondToInteraction(s, i, "❌ 你没有权限执行此命令", true)
		return
	}

	if len(options) == 0 {
		b.respondToInteraction(s, i, "❌ 请选择一个管理员子命令", true)
		return
	}

	switch options[0].Name {
	case "set-reward":
		b.handleSetCheckInReward(s, i, options[0].Options)
	default:
		b.respondToInteraction(s, i, "❌ 未知的管理员命令", true)
	}
}

// handleSetCheckInReward 处理设置签到奖励
func (b *Bot) handleSetCheckInReward(s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) {
	if len(options) < 2 {
		b.respondToInteraction(s, i, "❌ 请提供基础积分和奖励积分参数", true)
		return
	}

	basePoints := int(options[0].IntValue())
	bonusPoints := int(options[1].IntValue())

	// 这里应该保存到数据库配置中，暂时只显示设置成功
	embed := &discordgo.MessageEmbed{
		Title: "⚙️ 签到奖励设置",
		Color: 0x00ff00,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "基础积分",
				Value:  fmt.Sprintf("%d 积分", basePoints),
				Inline: true,
			},
			{
				Name:   "连续奖励",
				Value:  fmt.Sprintf("每天额外 %d 积分", bonusPoints),
				Inline: true,
			},
			{
				Name:   "状态",
				Value:  "✅ 设置成功",
				Inline: false,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "LFG Bot 管理系统",
		},
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		logger.Errorf("响应设置签到奖励命令失败: %v", err)
	}
}

// hasAdminPermission 检查用户是否有管理员权限
func (b *Bot) hasAdminPermission(member *discordgo.Member) bool {
	// 检查是否有管理员权限
	permissions := int64(member.Permissions)
	return (permissions & discordgo.PermissionAdministrator) != 0
}
