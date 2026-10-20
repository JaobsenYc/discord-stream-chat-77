package riotwatch

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/database"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/role"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// CommandHandler Riot Watch 命令处理器
type CommandHandler struct {
	service     *Service
	session     *discordgo.Session
	guildID     string
	roleManager *role.Manager
	userRepo    *database.UserRepository
}

// NewCommandHandler 创建新的命令处理器
func NewCommandHandler(
	service *Service,
	session *discordgo.Session,
	guildID string,
	roleManager *role.Manager,
	db *database.DB,
) *CommandHandler {
	return &CommandHandler{
		service:     service,
		session:     session,
		guildID:     guildID,
		roleManager: roleManager,
		userRepo:    database.NewUserRepository(db),
	}
}

// RegisterCommands 注册斜杠命令
func (h *CommandHandler) RegisterCommands() error {
	minDeathThreshold := 0.5
	maxDeathThreshold := 99.5

	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "riotwatch",
			Description: "Riot 自动竞猜管理",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "add",
					Description: "绑定 Riot 账号到当前频道（仅管理员）",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "riot_id",
							Description: "Riot ID (格式: 玩家名#TAG)",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionNumber,
							Name:        "death_threshold",
							Description: "死亡数阈值 (默认: 5.5，建议使用 .5 结尾避免平局)",
							Required:    false,
							MinValue:    &minDeathThreshold,
							MaxValue:    maxDeathThreshold,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "remove",
					Description: "移除 Riot 账号绑定（仅管理员）",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "riot_id",
							Description: "要移除的 Riot ID",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "list",
					Description: "查看当前频道的所有绑定",
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

// HandleCommand 处理斜杠命令
func (h *CommandHandler) HandleCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.ApplicationCommandData().Name != "riotwatch" {
		return
	}

	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		h.respondError(s, i, "请指定子命令")
		return
	}

	switch options[0].Name {
	case "add":
		h.handleAdd(s, i, options[0].Options)
	case "remove":
		h.handleRemove(s, i, options[0].Options)
	case "list":
		h.handleList(s, i)
	default:
		h.respondError(s, i, "未知的子命令")
	}
}

// handleAdd 处理添加绑定命令
func (h *CommandHandler) handleAdd(s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) {
	// 检查管理员权限
	isAdmin, err := h.roleManager.IsAdmin(i.Member.User.ID)
	if err != nil || !isAdmin {
		h.respondError(s, i, "❌ 只有管理员才能添加 Riot Watch 绑定")
		return
	}

	// 解析参数
	var riotIDFull string
	deathThreshold := 5.5 // 默认使用 .5 避免平局

	for _, opt := range options {
		switch opt.Name {
		case "riot_id":
			riotIDFull = opt.StringValue()
		case "death_threshold":
			deathThreshold = opt.FloatValue()
		}
	}

	// 解析 Riot ID (格式: Name#TAG)
	riotID, riotTag, err := parseRiotID(riotIDFull)
	if err != nil {
		h.respondError(s, i, fmt.Sprintf("❌ 无效的 Riot ID 格式: %v\n正确格式: 玩家名#TAG", err))
		return
	}

	// 获取或创建用户
	user, err := h.userRepo.GetOrCreate(i.Member.User.ID, i.Member.User.Username, nil, nil)
	if err != nil {
		h.respondError(s, i, "❌ 获取用户信息失败")
		return
	}

	// 先响应"处理中"（因为 API 调用可能需要时间）
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	// 创建绑定
	ctx := context.Background()
	binding, err := h.service.AddBinding(
		ctx,
		i.GuildID,
		i.ChannelID,
		user.ID,
		i.Member.User.ID,
		riotID,
		riotTag,
		deathThreshold,
	)

	if err != nil {
		h.editResponse(s, i, fmt.Sprintf("❌ 添加绑定失败: %v", err))
		return
	}

	// 成功响应
	embed := &discordgo.MessageEmbed{
		Title:       "✅ Riot Watch 绑定已添加",
		Description: "当该玩家开始单双排游戏时，将自动创建竞猜",
		Color:       0x00ff00,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "🎮 Riot ID",
				Value:  fmt.Sprintf("%s#%s", binding.RiotID, binding.RiotTag),
				Inline: true,
			},
			{
				Name:   "💀 死亡阈值",
				Value:  fmt.Sprintf("%g", binding.DeathThreshold),
				Inline: true,
			},
			{
				Name:   "📺 频道",
				Value:  fmt.Sprintf("<#%s>", i.ChannelID),
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("绑定ID: %s | 由 %s 创建", binding.ID.String()[:8], i.Member.User.Username),
		},
	}

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
	})
}

// handleRemove 处理移除绑定命令
func (h *CommandHandler) handleRemove(s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) {
	// 检查管理员权限
	isAdmin, err := h.roleManager.IsAdmin(i.Member.User.ID)
	if err != nil || !isAdmin {
		h.respondError(s, i, "❌ 只有管理员才能移除 Riot Watch 绑定")
		return
	}

	// 解析参数
	var riotIDFull string
	for _, opt := range options {
		if opt.Name == "riot_id" {
			riotIDFull = opt.StringValue()
		}
	}

	// 解析 Riot ID
	riotID, riotTag, err := parseRiotID(riotIDFull)
	if err != nil {
		h.respondError(s, i, fmt.Sprintf("❌ 无效的 Riot ID 格式: %v", err))
		return
	}

	// 先响应"处理中"
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	// 移除绑定
	err = h.service.RemoveBinding(i.GuildID, i.ChannelID, riotID, riotTag)
	if err != nil {
		h.editResponse(s, i, fmt.Sprintf("❌ 移除绑定失败: %v", err))
		return
	}

	// 成功响应
	embed := &discordgo.MessageEmbed{
		Title:       "✅ Riot Watch 绑定已移除",
		Description: fmt.Sprintf("已停止监控 **%s#%s**", riotID, riotTag),
		Color:       0xff9900,
	}

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
	})
}

// handleList 处理列表命令
func (h *CommandHandler) handleList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	bindings, err := h.service.GetBindingsByChannel(i.GuildID, i.ChannelID)
	if err != nil {
		h.respondError(s, i, "❌ 获取绑定列表失败")
		return
	}

	embed := &discordgo.MessageEmbed{
		Title: "📋 Riot Watch 绑定列表",
		Color: 0x3498db,
	}

	if len(bindings) == 0 {
		embed.Description = "当前频道没有任何 Riot Watch 绑定\n\n使用 `/riotwatch add` 添加绑定"
	} else {
		embed.Description = fmt.Sprintf("当前频道共有 **%d** 个绑定", len(bindings))

		var bindingList string
		for i, b := range bindings {
			status := "⏸️ 待机"
			if b.CurrentGameID != nil {
				status = "🎮 游戏中"
			}

			bindingList += fmt.Sprintf(
				"**%d.** %s#%s\n   └ 死亡阈值: %g | 状态: %s\n",
				i+1, b.RiotID, b.RiotTag, b.DeathThreshold, status,
			)
		}

		embed.Fields = []*discordgo.MessageEmbedField{
			{
				Name:  "绑定玩家",
				Value: bindingList,
			},
		}
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
}

// respondError 响应错误消息
func (h *CommandHandler) respondError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

// editResponse 编辑延迟响应
func (h *CommandHandler) editResponse(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &message,
	})
}

// parseRiotID 解析 Riot ID (格式: Name#TAG)
func parseRiotID(fullID string) (name, tag string, err error) {
	// 移除空格
	fullID = strings.TrimSpace(fullID)

	// 查找 # 分隔符
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

	// Riot TAG 通常是 2-5 个字符
	if len(tag) < 2 || len(tag) > 5 {
		return "", "", fmt.Errorf("TAG 长度应为 2-5 个字符")
	}

	return name, tag, nil
}
