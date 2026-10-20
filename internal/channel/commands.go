package channel

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/config"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/role"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/team"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// CommandHandler 频道管理命令处理器
type CommandHandler struct {
	channelManager *Manager
	teamManager    *team.Manager
	roleManager    *role.Manager
	config         *config.Config
}

// NewCommandHandler 创建新的频道管理命令处理器
func NewCommandHandler(channelManager *Manager, teamManager *team.Manager, roleManager *role.Manager, config *config.Config) *CommandHandler {
	return &CommandHandler{
		channelManager: channelManager,
		teamManager:    teamManager,
		roleManager:    roleManager,
		config:         config,
	}
}

// GetCommands 获取频道管理相关的slash命令
func (h *CommandHandler) GetCommands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name:        "channel",
			Description: "频道管理命令",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "cleanup",
					Description: "清理空闲频道",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionInteger,
							Name:        "timeout",
							Description: "空闲超时时间（秒），默认300秒（5分钟）",
							Required:    false,
							MinValue:    &[]float64{10}[0],
							MaxValue:    1800, // 最大30分钟
						},
						{
							Type:        discordgo.ApplicationCommandOptionBoolean,
							Name:        "test_mode",
							Description: "测试模式（使用10秒超时）",
							Required:    false,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "status",
					Description: "查看频道状态",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionBoolean,
							Name:        "detailed",
							Description: "显示详细信息",
							Required:    false,
						},
					},
				},
			},
		},
	}
}

// HandleInteraction 处理频道管理命令交互
func (h *CommandHandler) HandleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.ApplicationCommandData().Name != "channel" {
		return
	}

	// 检查管理员权限
	isAdmin, err := h.roleManager.IsAdmin(i.Member.User.ID)
	if err != nil {
		logger.Errorf("检查管理员权限失败: %v", err)
		h.respondError(s, i, "权限检查失败")
		return
	}

	if !isAdmin {
		h.respondError(s, i, "❌ 只有管理员才能使用频道管理命令！")
		return
	}

	// 检查频道权限
	if !h.roleManager.IsAdminChannel(i.ChannelID) {
		h.respondError(s, i, "❌ 此命令只能在管理员频道中使用！")
		return
	}

	// 获取子命令
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		h.respondError(s, i, "❌ 请指定子命令")
		return
	}

	subcommand := options[0]
	switch subcommand.Name {
	case "cleanup":
		h.handleCleanupCommand(s, i, subcommand.Options)
	case "status":
		h.handleStatusCommand(s, i, subcommand.Options)
	default:
		h.respondError(s, i, "❌ 未知的子命令")
	}
}

// handleCleanupCommand 处理清理命令
func (h *CommandHandler) handleCleanupCommand(s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) {
	// 解析参数
	timeout := 5 * time.Minute // 默认5分钟
	testMode := false

	for _, option := range options {
		switch option.Name {
		case "timeout":
			timeout = time.Duration(option.IntValue()) * time.Second
		case "test_mode":
			if option.BoolValue() {
				timeout = 10 * time.Second
				testMode = true
			}
		}
	}

	// 响应开始清理
	var statusMsg string
	if testMode {
		statusMsg = fmt.Sprintf("🧪 开始测试清理频道...\n⏰ 测试超时时间: %v", timeout)
	} else {
		statusMsg = fmt.Sprintf("🧹 开始清理频道...\n⏰ 空闲超时时间: %v", timeout)
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: statusMsg,
		},
	})
	if err != nil {
		logger.Errorf("响应交互失败: %v", err)
		return
	}

	// 显示当前状态
	teamCount := h.teamManager.GetTeamCount()
	followupMsg := fmt.Sprintf("📊 当前团队数量: %d", teamCount)

	// 执行清理
	ctx := context.Background()
	cleanupErr := h.channelManager.CleanupEmptyChannels(ctx, h.teamManager, timeout)

	// 获取清理结果
	newTeamCount := h.teamManager.GetTeamCount()
	cleanedCount := teamCount - newTeamCount

	var resultMsg string
	if cleanupErr != nil {
		logger.Errorf("清理频道失败: %v", cleanupErr)
		resultMsg = fmt.Sprintf("%s\n❌ 清理失败：%v", followupMsg, cleanupErr)
	} else {
		resultMsg = fmt.Sprintf("%s\n✅ 清理完成！清理了 %d 个空闲频道，剩余 %d 个团队",
			followupMsg, cleanedCount, newTeamCount)
	}

	// 发送后续消息
	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: resultMsg,
	})
	if err != nil {
		logger.Errorf("发送后续消息失败: %v", err)
	}
}

// handleStatusCommand 处理状态查看命令
func (h *CommandHandler) handleStatusCommand(s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) {
	// 解析参数
	detailed := false
	for _, option := range options {
		if option.Name == "detailed" {
			detailed = option.BoolValue()
		}
	}

	teams := h.teamManager.GetAllTeams()
	if len(teams) == 0 {
		h.respondSuccess(s, i, "📊 当前没有活跃团队")
		return
	}

	// 构建状态消息
	content := fmt.Sprintf("📊 **频道状态报告**\n总计: %d 个团队\n\n", len(teams))

	displayCount := 0
	maxDisplay := 10
	if detailed {
		maxDisplay = 20
	}

	for _, team := range teams {
		if displayCount >= maxDisplay {
			content += "... 更多团队\n"
			break
		}

		// 检查频道成员数
		ctx := context.Background()
		members, err := h.channelManager.GetChannelMembers(ctx, team.ChannelID)
		memberCount := 0
		if err == nil {
			memberCount = len(members)
		}

		content += fmt.Sprintf("**%s** (团队: %d/%d, 频道: %d 人)\n",
			team.Game, team.PlayerCount(), team.MaxPlayers, memberCount)

		if detailed {
			content += fmt.Sprintf("频道: <#%s>\n", team.ChannelID)
			content += fmt.Sprintf("最后活动: %v 前\n", time.Since(team.LastActivity).Truncate(time.Second))

			// 检查空闲状态
			if isTracking, emptyDuration := h.channelManager.GetChannelEmptyStatus(team.ChannelID); isTracking {
				content += fmt.Sprintf("⚠️ 空闲时间: %v\n", emptyDuration.Truncate(time.Second))
				if emptyDuration >= 5*time.Minute {
					content += "🚨 **应被清理**\n"
				}
			} else if memberCount == 0 {
				content += "⚠️ 频道为空但未开始计时\n"
			}
		}

		content += "\n"
		displayCount++
	}

	h.respondSuccess(s, i, content)
}

// respondError 响应错误消息
func (h *CommandHandler) respondError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   discordgo.MessageFlagsEphemeral, // 只有用户自己能看到
		},
	})
	if err != nil {
		logger.Errorf("响应错误消息失败: %v", err)
	}
}

// respondSuccess 响应成功消息
func (h *CommandHandler) respondSuccess(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
		},
	})
	if err != nil {
		logger.Errorf("响应成功消息失败: %v", err)
	}
}
