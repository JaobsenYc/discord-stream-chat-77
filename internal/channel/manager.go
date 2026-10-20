package channel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/config"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/team"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// Manager 频道管理器
type Manager struct {
	session           *discordgo.Session
	guildID           string
	config            *config.Config
	emptyChannelTimes map[string]time.Time // 记录频道空闲开始时间
	mutex             sync.RWMutex         // 保护emptyChannelTimes
}

// NewManager 创建新的频道管理器
func NewManager(session *discordgo.Session, guildID string, cfg *config.Config) *Manager {
	return &Manager{
		session:           session,
		guildID:           guildID,
		config:            cfg,
		emptyChannelTimes: make(map[string]time.Time),
	}
}

// CreateVoiceChannel 创建语音频道
func (m *Manager) CreateVoiceChannel(ctx context.Context, teamInfo *team.Team, categoryID string) (string, error) {
	// 获取创建者用户名
	creatorUsername := m.getUsernameSafe(teamInfo.CreatorID)

	channelName := m.generateChannelName(teamInfo, creatorUsername)

	// 使用传入的分类ID，如果为空则使用默认分类
	if categoryID == "" {
		categoryID = m.config.DefaultCategoryID
	}

	channelData := &discordgo.GuildChannelCreateData{
		Name:      channelName,
		Type:      discordgo.ChannelTypeGuildVoice,
		ParentID:  categoryID,
		UserLimit: 0, // 不限制人数，支持观战用户
	}

	// 设置频道权限
	permissions := []*discordgo.PermissionOverwrite{
		{
			ID:   m.guildID, // @everyone role
			Type: discordgo.PermissionOverwriteTypeRole,
			Allow: discordgo.PermissionViewChannel |
				discordgo.PermissionVoiceConnect |
				discordgo.PermissionVoiceSpeak,
		},
	}
	channelData.PermissionOverwrites = permissions

	// 创建频道
	channel, err := m.session.GuildChannelCreateComplex(m.guildID, *channelData)
	if err != nil {
		return "", fmt.Errorf("创建语音频道失败: %w", err)
	}

	logger.Infof("创建语音频道成功: %s (ID: %s, 分类: %s, 游戏: %s, TeamID: %s)",
		channelName, channel.ID, categoryID, teamInfo.Game, teamInfo.ID)
	return channel.ID, nil
}

// UpdateChannelName 更新频道名称
func (m *Manager) UpdateChannelName(ctx context.Context, channelID string, teamInfo *team.Team) error {
	// 获取创建者用户名
	creatorUsername := m.getUsernameSafe(teamInfo.CreatorID)

	newName := m.generateChannelName(teamInfo, creatorUsername)

	edit := &discordgo.ChannelEdit{
		Name: newName,
	}

	_, err := m.session.ChannelEditComplex(channelID, edit)
	if err != nil {
		return fmt.Errorf("更新频道名称失败: %w", err)
	}

	logger.Debugf("更新频道名称: %s", newName)
	return nil
}

// DeleteChannel 删除频道
func (m *Manager) DeleteChannel(ctx context.Context, channelID string) error {
	_, err := m.session.ChannelDelete(channelID)
	if err != nil {
		return fmt.Errorf("删除频道失败: %w", err)
	}

	logger.Infof("删除频道: %s", channelID)
	return nil
}

// generateChannelName 生成频道名称
func (m *Manager) generateChannelName(teamInfo *team.Team, creatorUsername string) string {
	return fmt.Sprintf("%s-%s", teamInfo.Game, creatorUsername)
}

// MoveUserToChannel 将用户移动到指定频道
func (m *Manager) MoveUserToChannel(ctx context.Context, userID, channelID string) error {
	err := m.session.GuildMemberMove(m.guildID, userID, &channelID)
	if err != nil {
		return fmt.Errorf("移动用户到频道失败: %w", err)
	}

	logger.Debugf("移动用户 %s 到频道 %s", userID, channelID)
	return nil
}

// GetChannelMembers 获取频道中的成员
func (m *Manager) GetChannelMembers(ctx context.Context, channelID string) ([]*discordgo.Member, error) {
	guild, err := m.session.State.Guild(m.guildID)
	if err != nil {
		return nil, fmt.Errorf("获取服务器信息失败: %w", err)
	}

	var members []*discordgo.Member
	for _, vs := range guild.VoiceStates {
		if vs.ChannelID == channelID {
			member, err := m.session.GuildMember(m.guildID, vs.UserID)
			if err != nil {
				logger.Warnf("获取成员信息失败 %s: %v", vs.UserID, err)
				continue
			}
			members = append(members, member)
		}
	}

	return members, nil
}

// SetChannelUserLimit 设置频道用户限制
func (m *Manager) SetChannelUserLimit(ctx context.Context, channelID string, limit int) error {
	edit := &discordgo.ChannelEdit{
		UserLimit: limit,
	}

	_, err := m.session.ChannelEditComplex(channelID, edit)
	if err != nil {
		return fmt.Errorf("设置频道用户限制失败: %w", err)
	}

	logger.Debugf("设置频道 %s 用户限制: %d", channelID, limit)
	return nil
}

// SendChannelInvite 发送频道邀请链接并管理旧邀请
func (m *Manager) SendChannelInvite(ctx context.Context, textChannelID, voiceChannelID, userMessage string, teamInfo *team.Team) error {
	logger.Debugf("SendChannelInvite: 开始发送邀请 - 文字频道=%s, 语音频道=%s, 团队=%s", textChannelID, voiceChannelID, teamInfo.ID)

	// 如果存在旧的邀请消息，先禁用它
	if teamInfo.LastInviteMessageID != "" && teamInfo.LastInviteChannelID != "" {
		logger.Debugf("禁用旧邀请消息: %s (频道: %s)", teamInfo.LastInviteMessageID, teamInfo.LastInviteChannelID)
		if err := m.disableOldInviteMessage(teamInfo.LastInviteChannelID, teamInfo.LastInviteMessageID, teamInfo); err != nil {
			logger.Warnf("禁用旧邀请消息失败: %v", err)
		}
	}
	// 获取创建者用户信息
	user, err := m.session.User(teamInfo.CreatorID)
	if err != nil {
		logger.Warnf("无法获取用户信息: %v", err)
		user = &discordgo.User{Username: "未知用户"}
	}

	embed := &discordgo.MessageEmbed{
		Title: userMessage, // 使用用户发送的原始消息
		Color: 0x00ff00,    // 绿色
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "📢 语音频道",
				Value:  fmt.Sprintf("<#%s>", voiceChannelID),
				Inline: false,
			},
			{
				Name:   "👤 发起用户",
				Value:  fmt.Sprintf("<@%s>", teamInfo.CreatorID),
				Inline: false,
			},
			{
				Name:   "⏰ 邀请创建时间",
				Value:  fmt.Sprintf("<t:%d:R>", time.Now().Unix()), // Discord时间戳格式
				Inline: false,
			},
			{
				Name:   "•---------------------点击下方按钮进入房间--------------------•",
				Value:  " ",
				Inline: false,
			},
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: user.AvatarURL("256"), // 用户头像
		},
	}

	// 创建交互按钮
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "🔔",
					},
					Label: "点我进入房间",
					Style: discordgo.LinkButton,
					URL:   fmt.Sprintf("https://discord.com/channels/%s/%s", m.guildID, voiceChannelID),
				},
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "🚫",
					},
					Label:    "房间满员点我",
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("room_full_%s", teamInfo.ID),
				},
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "❎",
					},
					Label:    "放弃组队",
					Style:    discordgo.DangerButton,
					CustomID: fmt.Sprintf("abandon_team_%s", teamInfo.ID),
				},
			},
		},
	}

	messageSend := &discordgo.MessageSend{
		Embed:      embed,
		Components: components,
	}

	logger.Debugf("发送邀请消息到频道: %s", textChannelID)
	message, err := m.session.ChannelMessageSendComplex(textChannelID, messageSend)
	if err != nil {
		logger.Errorf("ChannelMessageSendComplex 失败: %v", err)
		return fmt.Errorf("发送频道邀请失败: %w", err)
	}

	logger.Debugf("邀请消息发送成功: %s", message.ID)

	// 更新团队的邀请消息信息
	teamInfo.UpdateInviteMessage(message.ID, textChannelID)

	logger.Debugf("SendChannelInvite: 完成发送邀请")
	return nil
}

// disableOldInviteMessage 禁用旧的邀请消息
func (m *Manager) disableOldInviteMessage(channelID, messageID string, teamInfo *team.Team) error {
	// 创建禁用的按钮组件
	disabledComponents := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "🔔",
					},
					Label:    "邀请已过期",
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("expired_join_%s", teamInfo.ID),
					Disabled: true,
				},
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "🚫",
					},
					Label:    "邀请已过期",
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("expired_full_%s", teamInfo.ID),
					Disabled: true,
				},
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "❎",
					},
					Label:    "邀请已过期",
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("expired_abandon_%s", teamInfo.ID),
					Disabled: true,
				},
			},
		},
	}

	// 获取原消息
	oldMessage, err := m.session.ChannelMessage(channelID, messageID)
	if err != nil {
		return fmt.Errorf("获取旧消息失败: %w", err)
	}

	// 复制原embed并修改标题
	var updatedEmbed *discordgo.MessageEmbed
	if len(oldMessage.Embeds) > 0 {
		embed := *oldMessage.Embeds[0]
		embed.Title = fmt.Sprintf("🚫 邀请已过期 🚫 ~~%s~~", teamInfo.Game)
		embed.Color = 0x808080 // 灰色
		updatedEmbed = &embed
	}

	// 更新消息
	edit := &discordgo.MessageEdit{
		ID:         messageID,
		Channel:    channelID,
		Components: &disabledComponents,
	}

	if updatedEmbed != nil {
		embeds := []*discordgo.MessageEmbed{updatedEmbed}
		edit.Embeds = &embeds
	}

	_, err = m.session.ChannelMessageEditComplex(edit)
	if err != nil {
		return fmt.Errorf("更新旧邀请消息失败: %w", err)
	}

	logger.Infof("已禁用旧邀请消息: %s (频道: %s)", messageID, channelID)
	return nil
}

// SendTeamFullNotification 发送团队满员通知
func (m *Manager) SendTeamFullNotification(ctx context.Context, textChannelID string, teamInfo *team.Team) error {
	embed := &discordgo.MessageEmbed{
		Title:       "✅ 团队已满员！",
		Description: fmt.Sprintf("**%s** 团队已经满员，祝你们游戏愉快！", teamInfo.Game),
		Color:       0xffd700, // 金色
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "团队成员",
				Value:  m.formatPlayerList(teamInfo.PlayersSnapshot()),
				Inline: false,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	_, err := m.session.ChannelMessageSendEmbed(textChannelID, embed)
	if err != nil {
		return fmt.Errorf("发送满员通知失败: %w", err)
	}

	return nil
}

// formatPlayerList 格式化玩家列表
func (m *Manager) formatPlayerList(players []team.Player) string {
	if len(players) == 0 {
		return "无"
	}

	var result string
	for i, player := range players {
		if i > 0 {
			result += "\n"
		}
		result += fmt.Sprintf("%d. %s", i+1, player.Username)
	}

	return result
}

// getUsername 获取用户名
func (m *Manager) getUsername(userID string) (string, error) {
	user, err := m.session.User(userID)
	if err != nil {
		return "", fmt.Errorf("获取用户信息失败: %w", err)
	}
	return user.Username, nil
}

// getUsernameSafe 安全获取用户名，失败时返回默认值
func (m *Manager) getUsernameSafe(userID string) string {
	username, err := m.getUsername(userID)
	if err != nil {
		logger.Warnf("无法获取用户 %s 名称: %v", userID, err)
		return "未知用户"
	}
	return username
}

// CleanupEmptyChannels 清理空闲频道
func (m *Manager) CleanupEmptyChannels(ctx context.Context, teamManager *team.Manager, emptyTimeout time.Duration) error {
	logger.Debug("开始检查空闲频道...")

	// 获取所有LFG频道（通过团队记录）
	allTeams := teamManager.GetAllTeams()
	logger.Debugf("检查 %d 个团队的频道", len(allTeams))

	if len(allTeams) == 0 {
		logger.Debug("没有团队频道需要检查")
		return nil
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	cleanedCount := 0
	now := time.Now()

	for _, team := range allTeams {
		channelID := team.ChannelID
		if channelID == "" {
			continue
		}

		// 检查频道是否还存在
		channel, err := m.session.Channel(channelID)
		if err != nil {
			logger.Infof("频道 %s 已不存在，清理团队记录: %s", channelID, team.Game)
			teamManager.RemoveTeam(team.ID)
			delete(m.emptyChannelTimes, channelID)
			cleanedCount++
			continue
		}

		// 检查频道是否有成员
		members, err := m.GetChannelMembers(ctx, channelID)
		if err != nil {
			logger.Warnf("获取频道成员失败 %s: %v", channelID, err)
			continue
		}

		memberCount := len(members)
		logger.Debugf("频道 %s (%s) 当前成员数: %d", channel.Name, channelID, memberCount)

		if memberCount == 0 {
			// 频道为空
			emptyStartTime, exists := m.emptyChannelTimes[channelID]
			if !exists {
				// 首次发现频道为空，记录开始时间
				m.emptyChannelTimes[channelID] = now
				logger.Debugf("频道 %s 开始空闲计时", channel.Name)
			} else {
				// 检查空闲时间
				emptyDuration := now.Sub(emptyStartTime)
				logger.Debugf("频道 %s 已空闲 %v (需要超过 %v)", channel.Name, emptyDuration, emptyTimeout)

				if emptyDuration >= emptyTimeout {
					// 空闲时间超过阈值，删除频道
					logger.Infof("删除空闲频道: %s (空闲时间: %v)", channel.Name, emptyDuration)
					if err := m.DeleteChannel(ctx, channelID); err != nil {
						logger.Errorf("删除空闲频道失败 %s: %v", channelID, err)
						continue
					}

					teamManager.RemoveTeam(team.ID)
					delete(m.emptyChannelTimes, channelID)
					cleanedCount++
					logger.Infof("成功清理空闲频道: %s", channel.Name)
				}
			}
		} else {
			// 频道有成员，移除空闲记录
			if _, exists := m.emptyChannelTimes[channelID]; exists {
				delete(m.emptyChannelTimes, channelID)
				logger.Debugf("频道 %s 有成员加入，取消空闲计时", channel.Name)
			}
		}
	}

	// 清理已删除频道的空闲时间记录
	m.cleanupStaleEmptyTimes(allTeams)

	if cleanedCount > 0 {
		logger.Infof("频道清理完成，共清理了 %d 个空闲频道", cleanedCount)
	} else {
		logger.Debug("频道清理完成，没有频道需要清理")
	}

	return nil
}

// cleanupStaleEmptyTimes 清理已删除频道的空闲时间记录
func (m *Manager) cleanupStaleEmptyTimes(currentTeams []*team.Team) {
	activeChannels := make(map[string]bool)
	for _, team := range currentTeams {
		if team.ChannelID != "" {
			activeChannels[team.ChannelID] = true
		}
	}

	for channelID := range m.emptyChannelTimes {
		if !activeChannels[channelID] {
			delete(m.emptyChannelTimes, channelID)
			logger.Debugf("清理陈旧的空闲时间记录: %s", channelID)
		}
	}
}

// OnChannelMemberChange 当频道成员变化时调用，用于更新空闲状态
func (m *Manager) OnChannelMemberChange(channelID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 如果有空闲记录，重置计时（实际检查会在下次清理任务中进行）
	if _, exists := m.emptyChannelTimes[channelID]; exists {
		logger.Debugf("频道 %s 成员变化，将在下次清理时重新检查", channelID)
	}
}

// GetChannelEmptyStatus 获取频道的空闲状态
func (m *Manager) GetChannelEmptyStatus(channelID string) (isTracking bool, emptyDuration time.Duration) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if emptyTime, exists := m.emptyChannelTimes[channelID]; exists {
		return true, time.Since(emptyTime)
	}
	return false, 0
}

// CleanupExpiredChannels 保持兼容性的方法，内部调用新的清理逻辑
func (m *Manager) CleanupExpiredChannels(ctx context.Context, teamManager *team.Manager, timeout time.Duration) error {
	// 使用固定的5分钟空闲超时
	emptyTimeout := 5 * time.Minute
	logger.Debugf("使用空闲频道清理逻辑，空闲超时: %v", emptyTimeout)
	return m.CleanupEmptyChannels(ctx, teamManager, emptyTimeout)
}

// UpdateExistingChannel 更新现有频道（修改名称和分类）
func (m *Manager) UpdateExistingChannel(ctx context.Context, channelID string, teamInfo *team.Team, categoryID string) error {
	// 获取创建者用户名
	creatorUsername := m.getUsernameSafe(teamInfo.CreatorID)

	newName := m.generateChannelName(teamInfo, creatorUsername)

	edit := &discordgo.ChannelEdit{
		Name: newName,
	}

	// 如果指定了分类ID，则移动频道
	if categoryID != "" {
		edit.ParentID = categoryID
	}

	_, err := m.session.ChannelEditComplex(channelID, edit)
	if err != nil {
		return fmt.Errorf("更新现有频道失败: %w", err)
	}

	logger.Infof("更新现有频道: %s (ID: %s, 新分类: %s)", newName, channelID, categoryID)
	return nil
}

// CreateOrUpdateVoiceChannel 创建或更新语音频道
// 核心逻辑：每个用户最多只能有一个活跃的语音频道
func (m *Manager) CreateOrUpdateVoiceChannel(ctx context.Context, teamInfo *team.Team, existingChannelID, categoryID string) (string, error) {
	logger.Debugf("CreateOrUpdateVoiceChannel: 用户=%s, 现有频道=%s, 游戏=%s, 分类=%s",
		teamInfo.CreatorID, existingChannelID, teamInfo.Game, categoryID)

	if existingChannelID != "" {
		// 检查现有频道是否仍然存在
		_, err := m.session.Channel(existingChannelID)
		if err != nil {
			logger.Warnf("现有频道 %s 不存在或无法访问: %v，将创建新频道", existingChannelID, err)
			existingChannelID = "" // 重置为空，创建新频道
		} else {
			// 频道存在，更新它
			if err := m.UpdateExistingChannel(ctx, existingChannelID, teamInfo, categoryID); err != nil {
				logger.Errorf("更新现有频道失败 %s: %v，将创建新频道", existingChannelID, err)
				// 更新失败，删除旧频道并创建新的
				if deleteErr := m.DeleteChannel(ctx, existingChannelID); deleteErr != nil {
					logger.Errorf("删除失效频道失败 %s: %v", existingChannelID, deleteErr)
				}
				existingChannelID = "" // 重置为空，创建新频道
			} else {
				logger.Infof("更新用户 %s 的现有频道成功: %s -> %s", teamInfo.CreatorID, existingChannelID, teamInfo.Game)
				return existingChannelID, nil
			}
		}
	}

	// 创建新频道
	channelID, err := m.CreateVoiceChannel(ctx, teamInfo, categoryID)
	if err != nil {
		return "", fmt.Errorf("创建新语音频道失败: %w", err)
	}

	logger.Infof("为用户 %s 创建新频道: %s (%s)", teamInfo.CreatorID, channelID, teamInfo.Game)
	return channelID, nil
}
