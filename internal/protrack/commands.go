package protrack

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/rank"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/role"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// CommandHandler /trackpro 命令处理器
type CommandHandler struct {
	service     *Service
	session     *discordgo.Session
	guildID     string
	roleManager *role.Manager
	channelID   string // 非空时 add/remove 只能在该频道使用
	opggRegion  string
}

// NewCommandHandler 创建命令处理器
func NewCommandHandler(service *Service, session *discordgo.Session, guildID string, roleManager *role.Manager, channelID, riotRegion string) *CommandHandler {
	return &CommandHandler{
		service:     service,
		session:     session,
		guildID:     guildID,
		roleManager: roleManager,
		channelID:   channelID,
		opggRegion:  rank.ToOpggRegion(riotRegion),
	}
}

// RegisterCommands 注册 /trackpro
func (h *CommandHandler) RegisterCommands() error {
	targetOption := &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionString,
		Name:        "target",
		Description: "选手名(如 Faker)或 Riot ID(名字#TAG)",
		Required:    true,
	}

	cmd := &discordgo.ApplicationCommand{
		Name:        "trackpro",
		Description: "追踪职业选手的单双排对局,开局和结束时在频道播报",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "add",
				Description: "开始追踪一名选手(选手名会追踪其全部账号)",
				Options: []*discordgo.ApplicationCommandOption{
					targetOption,
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "name",
						Description: "选手名(Riot ID 不在选手库时用于入库,如 Faker)",
						Required:    false,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "remove",
				Description: "取消追踪一名选手",
				Options:     []*discordgo.ApplicationCommandOption{targetOption},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "list",
				Description: "查看追踪中的选手及是否在游戏中",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "pros",
				Description: "查看职业选手库",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "register",
				Description: "录入或更新选手库中的账号(管理员)",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "riot_id",
						Description: "Riot ID (格式: 名字#TAG)",
						Required:    true,
					},
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "name",
						Description: "选手名,如 Faker",
						Required:    true,
					},
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "team",
						Description: "战队,如 T1",
						Required:    false,
					},
				},
			},
		},
	}

	if _, err := h.session.ApplicationCommandCreate(h.session.State.User.ID, h.guildID, cmd); err != nil {
		return fmt.Errorf("注册命令 %s 失败: %w", cmd.Name, err)
	}
	logger.Infof("已注册命令: %s", cmd.Name)
	return nil
}

// HandleCommand 分发 /trackpro 子命令
func (h *CommandHandler) HandleCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	if data.Name != "trackpro" {
		return
	}
	if len(data.Options) == 0 {
		h.respondError(s, i, "请指定子命令")
		return
	}

	sub := data.Options[0]
	opts := make(map[string]string, len(sub.Options))
	for _, opt := range sub.Options {
		if opt.Type == discordgo.ApplicationCommandOptionString {
			opts[opt.Name] = opt.StringValue()
		}
	}

	switch sub.Name {
	case "add":
		h.handleAdd(s, i, opts)
	case "remove":
		h.handleRemove(s, i, opts)
	case "list":
		h.handleList(s, i)
	case "pros":
		h.handlePros(s, i)
	case "register":
		h.handleRegister(s, i, opts)
	default:
		h.respondError(s, i, "未知的子命令")
	}
}

func (h *CommandHandler) handleAdd(s *discordgo.Session, i *discordgo.InteractionCreate, opts map[string]string) {
	if !h.checkChannel(s, i) {
		return
	}
	h.deferResponse(s, i, false)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := h.service.Track(ctx, opts["target"], opts["name"], i.ChannelID, interactionUserID(i))
	if err != nil {
		h.editResponse(s, i, fmt.Sprintf("❌ 追踪失败: %v", err))
		return
	}

	var lines []string
	for _, a := range result.Added {
		lines = append(lines, fmt.Sprintf("✅ 开始追踪 **%s** · `%s`", a.DisplayName(), a.RiotID()))
	}
	for _, a := range result.Already {
		lines = append(lines, fmt.Sprintf("ℹ️ **%s** · `%s` 已在追踪中", a.DisplayName(), a.RiotID()))
	}
	lines = append(lines, "单双排开局和结束都会在本频道播报")
	h.editResponse(s, i, strings.Join(lines, "\n"))
}

func (h *CommandHandler) handleRemove(s *discordgo.Session, i *discordgo.InteractionCreate, opts map[string]string) {
	if !h.checkChannel(s, i) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	removed, err := h.service.Untrack(ctx, opts["target"])
	if err != nil {
		h.respondError(s, i, fmt.Sprintf("❌ 取消追踪失败: %v", err))
		return
	}

	var lines []string
	for _, a := range removed {
		lines = append(lines, fmt.Sprintf("🛑 已取消追踪 **%s** · `%s`", a.DisplayName(), a.RiotID()))
	}
	h.respond(s, i, strings.Join(lines, "\n"), nil, false)
}

func (h *CommandHandler) handleList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	accounts, err := h.service.ListTracked(ctx)
	if err != nil {
		logger.Errorf("查询追踪列表失败: %v", err)
		h.respondError(s, i, "❌ 查询追踪列表失败")
		return
	}

	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("📡 追踪中的职业选手 (%d)", len(accounts)),
		Color: colorInfo,
	}
	if len(accounts) == 0 {
		embed.Description = "当前没有追踪任何选手,使用 `/trackpro add target:Faker` 开始追踪"
	} else {
		// 游戏中的排在前面
		var inGame, idle []string
		for _, a := range accounts {
			link := fmt.Sprintf("[%s](%s)", a.RiotID(), rank.OpggURL(h.opggRegion, a.RiotGameName, a.RiotTagLine))
			if a.CurrentGameID == nil {
				idle = append(idle, fmt.Sprintf("⚪ **%s** · %s · 空闲", a.DisplayName(), link))
				continue
			}
			status := "游戏中"
			if a.CurrentChampionID != nil {
				status += " · " + h.service.ChampionName(*a.CurrentChampionID)
			}
			if a.CurrentGameStartedAt != nil {
				status += fmt.Sprintf(" · 已进行 %d 分钟", int(time.Since(*a.CurrentGameStartedAt).Minutes()))
			}
			inGame = append(inGame, fmt.Sprintf("🟢 **%s** · %s · %s", a.DisplayName(), link, status))
		}
		embed.Description = joinLines(append(inGame, idle...), embedDescriptionLimit)
	}

	h.respond(s, i, "", embed, false)
}

func (h *CommandHandler) handlePros(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	accounts, err := h.service.ListAll(ctx)
	if err != nil {
		logger.Errorf("查询选手库失败: %v", err)
		h.respondError(s, i, "❌ 查询选手库失败")
		return
	}

	lines := make([]string, 0, len(accounts))
	for _, a := range accounts {
		line := fmt.Sprintf("**%s** · `%s`", a.DisplayName(), a.RiotID())
		if a.IsTracked {
			line += " · 📡 追踪中"
		}
		lines = append(lines, line)
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("📖 职业选手库 (%d 个账号)", len(accounts)),
		Color:       colorInfo,
		Description: joinLines(lines, embedDescriptionLimit),
		Footer:      &discordgo.MessageEmbedFooter{Text: "管理员可用 /trackpro register 录入选手"},
	}
	if len(accounts) == 0 {
		embed.Description = "选手库为空"
	}
	h.respond(s, i, "", embed, true)
}

func (h *CommandHandler) handleRegister(s *discordgo.Session, i *discordgo.InteractionCreate, opts map[string]string) {
	isAdmin, err := h.roleManager.IsAdmin(interactionUserID(i))
	if err != nil || !isAdmin {
		h.respondError(s, i, "❌ 只有管理员才能修改选手库")
		return
	}

	proName := strings.TrimSpace(opts["name"])
	if proName == "" {
		h.respondError(s, i, "❌ 选手名不能为空")
		return
	}
	var team *string
	if t := strings.TrimSpace(opts["team"]); t != "" {
		team = &t
	}

	h.deferResponse(s, i, true)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	a, created, err := h.service.Register(ctx, opts["riot_id"], proName, team)
	if err != nil {
		h.editResponse(s, i, fmt.Sprintf("❌ 录入失败: %v", err))
		return
	}

	verb := "已更新"
	if created {
		verb = "已录入"
	}
	h.editResponse(s, i, fmt.Sprintf("✅ %s **%s** · `%s`", verb, a.DisplayName(), a.RiotID()))
}

// checkChannel 配置了专用频道时,add/remove 只能在该频道使用(播报发往执行命令的频道)
func (h *CommandHandler) checkChannel(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
	if h.channelID == "" || i.ChannelID == h.channelID {
		return true
	}
	h.respondError(s, i, fmt.Sprintf("❌ 请在 <#%s> 使用该命令", h.channelID))
	return false
}

// interactionUserID 取发起交互的用户 ID(服务器内为 Member,私信为 User)
func interactionUserID(i *discordgo.InteractionCreate) string {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID
	}
	if i.User != nil {
		return i.User.ID
	}
	return ""
}

func (h *CommandHandler) respond(s *discordgo.Session, i *discordgo.InteractionCreate, content string, embed *discordgo.MessageEmbed, ephemeral bool) {
	data := &discordgo.InteractionResponseData{
		Content:         content,
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	}
	if embed != nil {
		data.Embeds = []*discordgo.MessageEmbed{embed}
	}
	if ephemeral {
		data.Flags = discordgo.MessageFlagsEphemeral
	}
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: data,
	}); err != nil {
		logger.Errorf("响应 /trackpro 失败: %v", err)
	}
}

func (h *CommandHandler) respondError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	h.respond(s, i, message, nil, true)
}

func (h *CommandHandler) deferResponse(s *discordgo.Session, i *discordgo.InteractionCreate, ephemeral bool) {
	data := &discordgo.InteractionResponseData{}
	if ephemeral {
		data.Flags = discordgo.MessageFlagsEphemeral
	}
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: data,
	}); err != nil {
		logger.Errorf("延迟响应 /trackpro 失败: %v", err)
	}
}

func (h *CommandHandler) editResponse(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content:         &message,
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	}); err != nil {
		logger.Errorf("编辑 /trackpro 响应失败: %v", err)
	}
}
