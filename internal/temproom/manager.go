package temproom

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// TempRoom 临时房间信息
type TempRoom struct {
	ID          string      // 房间ID
	OwnerID     string      // 房主ID
	OwnerName   string      // 房主名称
	CategoryID  string      // 所属分类ID
	CreatedAt   time.Time   // 创建时间
	LastActive  time.Time   // 最后活跃时间
	CleanupTask *time.Timer // 清理任务
}

// Manager 临时房间管理器
type Manager struct {
	session        *discordgo.Session
	guildID        string
	triggerChannel string        // 触发频道ID
	categoryID     string        // 临时房间分类ID
	nameTemplate   string        // 房间名称模板
	cleanupDelay   time.Duration // 清理延迟

	// 房间管理
	rooms map[string]*TempRoom // channelID -> TempRoom
	mutex sync.RWMutex

	// 上下文
	ctx    context.Context
	cancel context.CancelFunc
}

// NewManager 创建临时房间管理器
func NewManager(session *discordgo.Session, guildID, triggerChannel, categoryID, nameTemplate string, cleanupDelay time.Duration) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		session:        session,
		guildID:        guildID,
		triggerChannel: triggerChannel,
		categoryID:     categoryID,
		nameTemplate:   nameTemplate,
		cleanupDelay:   cleanupDelay,
		rooms:          make(map[string]*TempRoom),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start 启动管理器
func (m *Manager) Start() error {
	logger.Info("临时房间管理器已启动")
	return nil
}

// Stop 停止管理器
func (m *Manager) Stop() {
	m.cancel()

	// 清理所有房间
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for channelID, room := range m.rooms {
		if room.CleanupTask != nil {
			room.CleanupTask.Stop()
		}

		// 删除房间
		if _, err := m.session.ChannelDelete(channelID); err != nil {
			logger.Errorf("删除临时房间失败 %s: %v", channelID, err)
		}
	}

	logger.Info("临时房间管理器已停止")
}

// HandleVoiceStateUpdate 处理语音状态更新
func (m *Manager) HandleVoiceStateUpdate(vs *discordgo.VoiceStateUpdate) {
	// 检查是否启用临时房间功能
	if m.triggerChannel == "" {
		return
	}

	// 处理用户加入触发频道
	if vs.ChannelID == m.triggerChannel {
		m.handleTriggerChannelJoin(vs)
		return
	}

	// 处理临时房间的用户变化
	if m.isTempRoom(vs.ChannelID) {
		m.handleTempRoomUpdate(vs)
		return
	}

	// 处理用户离开临时房间
	if vs.BeforeUpdate != nil && m.isTempRoom(vs.BeforeUpdate.ChannelID) {
		m.handleTempRoomLeave(vs.BeforeUpdate.ChannelID)
	}
}

// handleTriggerChannelJoin 处理用户加入触发频道
func (m *Manager) handleTriggerChannelJoin(vs *discordgo.VoiceStateUpdate) {
	userID := vs.UserID

	// 获取用户信息
	member, err := m.session.GuildMember(m.guildID, userID)
	if err != nil {
		logger.Errorf("获取用户信息失败 %s: %v", userID, err)
		return
	}

	// 优先使用服务器昵称，其次是全局显示名，最后是用户名
	username := member.User.Username
	if member.User.GlobalName != "" {
		username = member.User.GlobalName
	}
	if member.Nick != "" {
		username = member.Nick
	}

	logger.Debugf("用户信息: ID=%s, Username=%s, GlobalName=%s, Nick=%s, 最终使用=%s",
		userID, member.User.Username, member.User.GlobalName, member.Nick, username)

	// 检查用户是否已有临时房间
	if existingRoomID := m.getUserTempRoom(userID); existingRoomID != "" {
		// 直接移动到现有房间
		if err := m.session.GuildMemberMove(m.guildID, userID, &existingRoomID); err != nil {
			logger.Errorf("移动用户到现有临时房间失败: %v", err)
		}
		return
	}

	// 创建新的临时房间
	roomName := fmt.Sprintf(m.nameTemplate, username)
	logger.Debugf("创建临时房间: 模板=%s, 用户名=%s, 最终名称=%s", m.nameTemplate, username, roomName)

	channel, err := m.session.GuildChannelCreateComplex(m.guildID, discordgo.GuildChannelCreateData{
		Name:     roomName,
		Type:     discordgo.ChannelTypeGuildVoice,
		ParentID: m.categoryID,
		PermissionOverwrites: []*discordgo.PermissionOverwrite{
			{
				ID:    userID,
				Type:  discordgo.PermissionOverwriteTypeMember,
				Allow: discordgo.PermissionVoiceConnect | discordgo.PermissionVoiceSpeak | discordgo.PermissionManageChannels,
			},
		},
	})

	if err != nil {
		logger.Errorf("创建临时房间失败: %v", err)
		return
	}

	logger.Infof("为用户 %s 创建临时房间: %s (%s)", username, roomName, channel.ID)

	// 移动用户到新房间
	if err := m.session.GuildMemberMove(m.guildID, userID, &channel.ID); err != nil {
		logger.Errorf("移动用户到临时房间失败: %v", err)
		// 如果移动失败，删除刚创建的房间
		_, _ = m.session.ChannelDelete(channel.ID)
		return
	}

	// 记录房间信息
	m.addTempRoom(channel.ID, userID, username)
}

// handleTempRoomUpdate 处理临时房间用户变化
func (m *Manager) handleTempRoomUpdate(vs *discordgo.VoiceStateUpdate) {
	channelID := vs.ChannelID

	// 更新房间活跃时间
	m.updateRoomActivity(channelID)

	// 取消可能存在的清理任务
	m.cancelCleanupTask(channelID)
}

// handleTempRoomLeave 处理用户离开临时房间
func (m *Manager) handleTempRoomLeave(channelID string) {
	// 检查房间是否为空
	if m.isRoomEmpty(channelID) {
		// 启动延迟清理任务
		m.scheduleCleanup(channelID)
	}
}

// addTempRoom 添加临时房间记录
func (m *Manager) addTempRoom(channelID, ownerID, ownerName string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := time.Now()
	m.rooms[channelID] = &TempRoom{
		ID:         channelID,
		OwnerID:    ownerID,
		OwnerName:  ownerName,
		CategoryID: m.categoryID,
		CreatedAt:  now,
		LastActive: now,
	}
}

// isTempRoom 检查是否为临时房间
func (m *Manager) isTempRoom(channelID string) bool {
	if channelID == "" {
		return false
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	_, exists := m.rooms[channelID]
	return exists
}

// getUserTempRoom 获取用户的临时房间ID
func (m *Manager) getUserTempRoom(userID string) string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for channelID, room := range m.rooms {
		if room.OwnerID == userID {
			return channelID
		}
	}
	return ""
}

// updateRoomActivity 更新房间活跃时间
func (m *Manager) updateRoomActivity(channelID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if room, exists := m.rooms[channelID]; exists {
		room.LastActive = time.Now()
	}
}

// isRoomEmpty 检查房间是否为空
func (m *Manager) isRoomEmpty(channelID string) bool {
	guild, err := m.session.State.Guild(m.guildID)
	if err != nil {
		logger.Errorf("获取服务器信息失败: %v", err)
		return true
	}

	// 统计在该频道的用户数量
	userCount := 0
	for _, vs := range guild.VoiceStates {
		if vs.ChannelID == channelID {
			userCount++
		}
	}

	return userCount == 0
}

// scheduleCleanup 安排清理任务
func (m *Manager) scheduleCleanup(channelID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	room, exists := m.rooms[channelID]
	if !exists {
		return
	}

	// 如果已有清理任务，先取消
	if room.CleanupTask != nil {
		room.CleanupTask.Stop()
	}

	// 创建新的清理任务
	room.CleanupTask = time.AfterFunc(m.cleanupDelay, func() {
		m.cleanupRoom(channelID)
	})

	logger.Infof("为临时房间 %s 安排清理任务，%v 后执行", channelID, m.cleanupDelay)
}

// cancelCleanupTask 取消清理任务
func (m *Manager) cancelCleanupTask(channelID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	room, exists := m.rooms[channelID]
	if !exists {
		return
	}

	if room.CleanupTask != nil {
		room.CleanupTask.Stop()
		room.CleanupTask = nil
		logger.Infof("取消临时房间 %s 的清理任务", channelID)
	}
}

// cleanupRoom 清理房间
func (m *Manager) cleanupRoom(channelID string) {
	// 再次检查房间是否为空
	if !m.isRoomEmpty(channelID) {
		logger.Infof("临时房间 %s 不为空，取消清理", channelID)
		return
	}

	// 删除房间
	if _, err := m.session.ChannelDelete(channelID); err != nil {
		logger.Errorf("删除临时房间失败 %s: %v", channelID, err)
		return
	}

	// 从记录中移除
	m.mutex.Lock()
	if room, exists := m.rooms[channelID]; exists {
		logger.Infof("已清理临时房间: %s (房主: %s, 存在时长: %v)",
			channelID, room.OwnerName, time.Since(room.CreatedAt))
		delete(m.rooms, channelID)
	}
	m.mutex.Unlock()
}

// GetTempRoomStats 获取临时房间统计信息
func (m *Manager) GetTempRoomStats() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	stats := map[string]interface{}{
		"total_rooms": len(m.rooms),
		"rooms":       make([]map[string]interface{}, 0, len(m.rooms)),
	}

	for channelID, room := range m.rooms {
		roomInfo := map[string]interface{}{
			"channel_id":  channelID,
			"owner_id":    room.OwnerID,
			"owner_name":  room.OwnerName,
			"created_at":  room.CreatedAt,
			"last_active": room.LastActive,
			"duration":    time.Since(room.CreatedAt),
		}
		stats["rooms"] = append(stats["rooms"].([]map[string]interface{}), roomInfo)
	}

	return stats
}
