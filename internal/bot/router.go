package bot

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/team"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// handlerFunc 交互处理器统一签名
type handlerFunc func(s *discordgo.Session, i *discordgo.InteractionCreate)

// prefixRoute customID 前缀 → 处理器
type prefixRoute struct {
	prefix  string
	handler handlerFunc
}

// interactionRouter 按注册表分发 Discord 交互:
// slash 命令按名称精确匹配,按钮/模态框按 customID 前缀匹配。
// 新增模块只需在 registerRoutes 中登记一行,无需改动分发逻辑。
type interactionRouter struct {
	slash      map[string]handlerFunc
	components []prefixRoute
	modals     []prefixRoute
}

func newInteractionRouter() *interactionRouter {
	return &interactionRouter{slash: make(map[string]handlerFunc)}
}

// slashCmd 注册 slash 命令处理器(可一次登记多个命令名)
func (r *interactionRouter) slashCmd(h handlerFunc, names ...string) {
	for _, name := range names {
		r.slash[name] = h
	}
}

// component 注册按钮交互的 customID 前缀路由
func (r *interactionRouter) component(prefix string, h handlerFunc) {
	r.components = append(r.components, prefixRoute{prefix: prefix, handler: h})
}

// modal 注册模态框提交的 customID 前缀路由
func (r *interactionRouter) modal(prefix string, h handlerFunc) {
	r.modals = append(r.modals, prefixRoute{prefix: prefix, handler: h})
}

// dispatchSlash 查找 slash 命令处理器
func (r *interactionRouter) dispatchSlash(name string) (handlerFunc, bool) {
	h, ok := r.slash[name]
	return h, ok
}

// dispatchComponent 按前缀查找按钮处理器
func (r *interactionRouter) dispatchComponent(customID string) (handlerFunc, bool) {
	for _, route := range r.components {
		if strings.HasPrefix(customID, route.prefix) {
			return route.handler, true
		}
	}
	return nil, false
}

// dispatchModal 按前缀查找模态框处理器
func (r *interactionRouter) dispatchModal(customID string) (handlerFunc, bool) {
	for _, route := range r.modals {
		if strings.HasPrefix(customID, route.prefix) {
			return route.handler, true
		}
	}
	return nil, false
}

// registerRoutes 登记全部交互路由(新增命令/按钮在这里加一行)
func (b *Bot) registerRoutes() {
	r := newInteractionRouter()

	// slash 命令
	r.slashCmd(b.handleCheckInCommand, "checkin")
	r.slashCmd(b.achievementCommands.HandleInteraction,
		"achievement", "achievement-summary", "achievement-leaderboard",
		"achievement-list", "achievement-info", "achievement-admin")
	r.slashCmd(b.lotteryCommands.HandleInteraction, "lottery")
	r.slashCmd(b.channelCommands.HandleInteraction, "channel")
	r.slashCmd(b.lolCommands.HandleSlashCommand, "lol")
	r.slashCmd(b.bettingCommands.HandleCommand, "bet")
	r.slashCmd(b.pointsCommands.HandleInteraction, "points")
	if b.riotWatchCommands != nil {
		r.slashCmd(b.riotWatchCommands.HandleCommand, "riotwatch")
	}
	if b.rankCommands != nil {
		r.slashCmd(b.rankCommands.HandleCommand, "rank")
	}
	if b.proTrackCommands != nil {
		r.slashCmd(b.proTrackCommands.HandleCommand, "trackpro")
	}

	// 模态框提交
	r.modal("bet_", b.bettingCommands.HandleModalSubmit)

	// 按钮交互(未匹配前缀的走组队按钮兜底逻辑 handleTeamButton)
	r.component("lottery_", b.lotteryCommands.HandleButtonInteraction)
	r.component("lol_", b.lolCommands.HandleButtonInteraction)
	r.component("bet_", b.bettingCommands.HandleButtonInteraction)
	r.component("redeem_", b.pointsCommands.HandleRedeemButton)

	b.router = r
}

// handleInteractionCreate 处理交互事件,仅做分发,具体逻辑在各模块
func (b *Bot) handleInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		if h, ok := b.router.dispatchSlash(i.ApplicationCommandData().Name); ok {
			h(s, i)
		}

	case discordgo.InteractionModalSubmit:
		if h, ok := b.router.dispatchModal(i.ModalSubmitData().CustomID); ok {
			h(s, i)
		}

	case discordgo.InteractionMessageComponent:
		// 确保是我们的服务器
		if i.GuildID != b.config.GuildID {
			return
		}
		customID := i.MessageComponentData().CustomID
		if h, ok := b.router.dispatchComponent(customID); ok {
			h(s, i)
			return
		}
		b.handleTeamButton(s, i, customID)
	}
}

// parseTeamButtonID 解析组队按钮 customID,格式: <动作两段>_<teamID>
// 例如 room_full_123_456 → action=room_full, teamID=123_456(teamID 可含下划线)
func parseTeamButtonID(customID string) (action, teamID string, ok bool) {
	parts := strings.Split(customID, "_")
	if len(parts) < 3 {
		return "", "", false
	}
	return parts[0] + "_" + parts[1], strings.Join(parts[2:], "_"), true
}

// handleTeamButton 处理组队相关按钮(join_room / room_full / abandon_team)
func (b *Bot) handleTeamButton(s *discordgo.Session, i *discordgo.InteractionCreate, customID string) {
	action, teamID, ok := parseTeamButtonID(customID)
	if !ok {
		logger.Warnf("无效的按钮CustomID: %s", customID)
		return
	}

	// 获取团队信息
	teamInfo, exists := b.teamManager.GetTeam(teamID)
	if !exists {
		b.respondToInteraction(s, i, "❌ 团队不存在或已解散", true)
		return
	}

	switch action {
	case "room_full":
		b.handleRoomFull(s, i, teamInfo)
	case "abandon_team":
		b.handleAbandonTeam(s, i, teamInfo)
	default:
		logger.Warnf("未知的按钮操作: %s", action)
	}
}

// handleRoomFull 处理房间满员按钮
func (b *Bot) handleRoomFull(s *discordgo.Session, i *discordgo.InteractionCreate, teamInfo *team.Team) {
	// 更新原消息，将标题改为带删除线的满员状态
	newTitle := fmt.Sprintf("🚫房间已满🚫 ~~%s~~", teamInfo.Game)

	// 复制原始embed（消息可能不带 embed，直接取下标会 panic 并因 discordgo
	// 在裸 goroutine 中分发事件而击穿整个进程）
	var originalEmbed discordgo.MessageEmbed
	if i.Message != nil && len(i.Message.Embeds) > 0 {
		originalEmbed = *i.Message.Embeds[0]
	}
	originalEmbed.Title = newTitle
	originalEmbed.Color = 0xffd700 // 金色

	// 禁用所有按钮
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "🔔",
					},
					Label:    "点我进入房间",
					Style:    discordgo.LinkButton,
					URL:      fmt.Sprintf("https://discord.com/channels/%s/%s", b.config.GuildID, teamInfo.ChannelID),
					Disabled: true,
				},
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "🚫",
					},
					Label:    "房间满员点我",
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("room_full_%s", teamInfo.ID),
					Disabled: true,
				},
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "❎",
					},
					Label:    "放弃组队",
					Style:    discordgo.DangerButton,
					CustomID: fmt.Sprintf("abandon_team_%s", teamInfo.ID),
					Disabled: true,
				},
			},
		},
	}

	// 先回应交互，然后更新消息
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{&originalEmbed},
			Components: components,
		},
	})

	if err != nil {
		logger.Errorf("更新消息失败: %v", err)
		return
	}
}

// handleAbandonTeam 处理放弃组队按钮
func (b *Bot) handleAbandonTeam(s *discordgo.Session, i *discordgo.InteractionCreate, teamInfo *team.Team) {
	userID := i.Member.User.ID

	// 检查是否是团队创建者
	if teamInfo.CreatorID != userID {
		b.respondToInteraction(s, i, "❌ 只有团队创建者可以解散团队！", true)
		return
	}

	// 删除语音频道
	if err := b.channelManager.DeleteChannel(b.ctx, teamInfo.ChannelID); err != nil {
		logger.Errorf("删除频道失败: %v", err)
	}

	// 移除团队
	b.teamManager.RemoveTeam(teamInfo.ID)

	b.respondToInteraction(s, i, "✅ 团队已解散，频道已删除", false)
}

// respondToInteraction 回应交互
func (b *Bot) respondToInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, message string, ephemeral bool) {
	var flags discordgo.MessageFlags = 0
	if ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   flags,
		},
	})

	if err != nil {
		logger.Errorf("发送交互回应失败: %v", err)
	}
}
