package team

import (
	"fmt"
	"sync"
	"time"

	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// Player 表示团队中的玩家
type Player struct {
	UserID   string    `json:"user_id"`
	Username string    `json:"username"`
	JoinedAt time.Time `json:"joined_at"`
}

// Team 表示一个游戏团队
type Team struct {
	ID           string    `json:"id"`
	Game         string    `json:"game"`
	MaxPlayers   int       `json:"max_players"`
	Players      []Player  `json:"players"`
	ChannelID    string    `json:"channel_id"`
	CreatedAt    time.Time `json:"created_at"`
	LastActivity time.Time `json:"last_activity"`
	CreatorID    string    `json:"creator_id"`
	// 邀请消息管理
	LastInviteMessageID string `json:"last_invite_message_id"` // 最后一条邀请消息ID
	LastInviteChannelID string `json:"last_invite_channel_id"` // 最后一条邀请消息的频道ID

	// 并发安全保护
	Mutex sync.RWMutex `json:"-"` // 不序列化到JSON
}

// IsFull 检查团队是否已满
func (t *Team) IsFull() bool {
	t.Mutex.RLock()
	defer t.Mutex.RUnlock()
	return len(t.Players) >= t.MaxPlayers
}

// PlayerCount 返回当前成员数（加锁读取，供外部包使用，避免直接读 Players 产生数据竞争）
func (t *Team) PlayerCount() int {
	t.Mutex.RLock()
	defer t.Mutex.RUnlock()
	return len(t.Players)
}

// PlayersSnapshot 返回成员列表的副本（加锁读取，供外部包遍历使用）
func (t *Team) PlayersSnapshot() []Player {
	t.Mutex.RLock()
	defer t.Mutex.RUnlock()
	return append([]Player(nil), t.Players...)
}

// IsEmpty 检查团队是否为空
func (t *Team) IsEmpty() bool {
	t.Mutex.RLock()
	defer t.Mutex.RUnlock()
	return len(t.Players) == 0
}

// HasPlayer 检查玩家是否在团队中
func (t *Team) HasPlayer(userID string) bool {
	t.Mutex.RLock()
	defer t.Mutex.RUnlock()
	for _, player := range t.Players {
		if player.UserID == userID {
			return true
		}
	}
	return false
}

// AddPlayer 添加玩家到团队
func (t *Team) AddPlayer(userID, username string) error {
	t.Mutex.Lock()
	defer t.Mutex.Unlock()

	if len(t.Players) >= t.MaxPlayers {
		return fmt.Errorf("团队已满 (%d/%d)", len(t.Players), t.MaxPlayers)
	}

	// 检查玩家是否已在团队中
	for _, player := range t.Players {
		if player.UserID == userID {
			return fmt.Errorf("玩家 %s 已在团队中", username)
		}
	}

	player := Player{
		UserID:   userID,
		Username: username,
		JoinedAt: time.Now(),
	}

	t.Players = append(t.Players, player)
	t.LastActivity = time.Now()

	logger.Infof("玩家 %s 加入团队 %s (%d/%d)", username, t.Game, len(t.Players), t.MaxPlayers)
	return nil
}

// RemovePlayer 从团队中移除玩家
func (t *Team) RemovePlayer(userID string) error {
	t.Mutex.Lock()
	defer t.Mutex.Unlock()

	for i, player := range t.Players {
		if player.UserID == userID {
			// 移除玩家
			t.Players = append(t.Players[:i], t.Players[i+1:]...)
			t.LastActivity = time.Now()
			logger.Infof("玩家 %s 离开团队 %s (%d/%d)", player.Username, t.Game, len(t.Players), t.MaxPlayers)
			return nil
		}
	}
	return fmt.Errorf("玩家未在团队中")
}

// GetStatusString 获取团队状态字符串
func (t *Team) GetStatusString() string {
	t.Mutex.RLock()
	defer t.Mutex.RUnlock()

	status := "🎮"
	if len(t.Players) >= t.MaxPlayers {
		status = "✅"
	}
	return fmt.Sprintf("%s **%s** 寻组 (%d/%d)", status, t.Game, len(t.Players), t.MaxPlayers)
}

// UpdateInviteMessage 更新邀请消息信息
func (t *Team) UpdateInviteMessage(messageID, channelID string) {
	t.Mutex.Lock()
	defer t.Mutex.Unlock()

	t.LastInviteMessageID = messageID
	t.LastInviteChannelID = channelID
	t.LastActivity = time.Now()
	logger.Debugf("更新团队 %s 的邀请消息: %s (频道: %s)", t.ID, messageID, channelID)
}

// UpdateTeamInfo 更新团队基本信息
func (t *Team) UpdateTeamInfo(game string, maxPlayers int) {
	t.Mutex.Lock()
	defer t.Mutex.Unlock()

	t.Game = game
	t.MaxPlayers = maxPlayers
	t.LastActivity = time.Now()
}

// Manager 团队管理器
type Manager struct {
	teams        map[string]*Team
	userChannels map[string]string // userID -> channelID 映射，追踪用户创建的频道
	mutex        sync.RWMutex
}

// NewManager 创建新的团队管理器
func NewManager() *Manager {
	return &Manager{
		teams:        make(map[string]*Team),
		userChannels: make(map[string]string), // 初始化用户频道映射
	}
}

// GetUserChannel 获取用户已创建的频道ID
func (m *Manager) GetUserChannel(userID string) (string, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	channelID, exists := m.userChannels[userID]
	return channelID, exists
}

// SetUserChannel 设置用户的频道ID
func (m *Manager) SetUserChannel(userID, channelID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.userChannels[userID] = channelID
	logger.Infof("用户 %s 的频道设置为: %s", userID, channelID)
}

// RemoveUserChannel 移除用户的频道记录
func (m *Manager) RemoveUserChannel(userID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if channelID, exists := m.userChannels[userID]; exists {
		delete(m.userChannels, userID)
		logger.Infof("移除用户 %s 的频道记录: %s", userID, channelID)
	}
}

// UpdateTeamChannel 更新团队的频道ID并同步用户频道映射
func (m *Manager) UpdateTeamChannel(teamID, newChannelID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	team, exists := m.teams[teamID]
	if !exists {
		return fmt.Errorf("团队 %s 不存在", teamID)
	}

	oldChannelID := team.ChannelID
	team.ChannelID = newChannelID

	// 更新用户频道映射
	m.userChannels[team.CreatorID] = newChannelID

	if oldChannelID == "" {
		logger.Infof("设置团队 %s (%s) 的频道ID: %s", teamID, team.Game, newChannelID)
	} else {
		logger.Infof("更新团队 %s (%s) 的频道: %s -> %s", teamID, team.Game, oldChannelID, newChannelID)
	}
	return nil
}

// CreateTeam 创建新团队
func (m *Manager) CreateTeam(id, game string, maxPlayers int, creatorID, channelID string) *Team {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	team := &Team{
		ID:           id,
		Game:         game,
		MaxPlayers:   maxPlayers,
		Players:      make([]Player, 0),
		ChannelID:    channelID,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		CreatorID:    creatorID,
	}

	m.teams[id] = team

	// 记录创建团队信息
	logger.Infof("创建新团队: %s (最大人数: %d)", game, maxPlayers)

	// 只有当channelID不为空时才记录用户频道映射
	if channelID != "" {
		m.userChannels[creatorID] = channelID
	}

	return team
}

// GetTeam 获取团队
func (m *Manager) GetTeam(id string) (*Team, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	team, exists := m.teams[id]
	return team, exists
}

// GetTeamByChannel 通过频道ID获取团队
func (m *Manager) GetTeamByChannel(channelID string) (*Team, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for _, team := range m.teams {
		if team.ChannelID == channelID {
			return team, true
		}
	}
	return nil, false
}

// RemoveTeam 移除团队
func (m *Manager) RemoveTeam(id string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if team, exists := m.teams[id]; exists {
		logger.Infof("移除团队: %s", team.Game)
		// 移除用户频道记录
		delete(m.userChannels, team.CreatorID)
		delete(m.teams, id)
	}
}

// GetAllTeams 获取所有团队
func (m *Manager) GetAllTeams() []*Team {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	teams := make([]*Team, 0, len(m.teams))
	for _, team := range m.teams {
		teams = append(teams, team)
	}
	return teams
}

// GetExpiredTeams 获取过期的团队
func (m *Manager) GetExpiredTeams(timeout time.Duration) []*Team {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	now := time.Now()
	expired := make([]*Team, 0)

	for _, team := range m.teams {
		if now.Sub(team.LastActivity) > timeout {
			expired = append(expired, team)
		}
	}

	return expired
}

// GetTeamCount 获取当前团队数量
func (m *Manager) GetTeamCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.teams)
}
