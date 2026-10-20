package lol

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// CustomMatchStatus 内战状态
type CustomMatchStatus int

const (
	StatusRecruiting CustomMatchStatus = iota // 招募中
	StatusInProgress                          // 进行中
	StatusCompleted                           // 已完成
	StatusCancelled                           // 已取消
)

// Team 队伍信息
type Team struct {
	Name           string   // 队伍名称
	Members        []string // 成员Discord ID列表
	Captain        string   // 队长Discord ID
	VoiceChannelID string   // 语音频道ID
}

// CustomMatch 内战匹配信息
type CustomMatch struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"` // 内战名称
	GuildID      string            `json:"guild_id"`
	ChannelID    string            `json:"channel_id"`
	MessageID    string            `json:"message_id"`
	CreatorID    string            `json:"creator_id"`
	Status       CustomMatchStatus `json:"status"`
	Participants []string          `json:"participants"` // 参与者Discord ID列表
	BlueTeam     *Team             `json:"blue_team"`    // 蓝色方
	RedTeam      *Team             `json:"red_team"`     // 红色方
	CreatedAt    time.Time         `json:"created_at"`
	StartedAt    *time.Time        `json:"started_at"`
	CompletedAt  *time.Time        `json:"completed_at"`
	MaxPlayers   int               `json:"max_players"` // 最大玩家数量 (默认10)
	CategoryID   string            `json:"category_id"` // 语音频道分类ID
	// 内鬼局相关字段
	IsSpyMode bool     `json:"is_spy_mode"` // 是否为内鬼局
	SpyCount  int      `json:"spy_count"`   // 每队内鬼数量
	BlueSpies []string `json:"blue_spies"`  // 蓝队内鬼列表
	RedSpies  []string `json:"red_spies"`   // 红队内鬼列表
}

// Manager 内战管理器
type Manager struct {
	session      *discordgo.Session
	guildID      string
	matches      map[string]*CustomMatch // key: matchID
	userMatches  map[string]string       // key: userID, value: matchID (用户当前参与的内战)
	mutex        sync.RWMutex
	categoryID   string        // 语音频道分类ID
	cleanupDelay time.Duration // 语音频道清理延迟时间
}

// NewManager 创建内战管理器
func NewManager(session *discordgo.Session, guildID, categoryID string, cleanupDelay time.Duration) *Manager {
	return &Manager{
		session:      session,
		guildID:      guildID,
		matches:      make(map[string]*CustomMatch),
		userMatches:  make(map[string]string),
		categoryID:   categoryID,
		cleanupDelay: cleanupDelay,
	}
}

// CreateMatch 创建新的内战
func (m *Manager) CreateMatch(channelID, creatorID, name string, isSpyMode bool, spyCount int) (*CustomMatch, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查创建者是否已经在其他内战中
	if existingMatchID, exists := m.userMatches[creatorID]; exists {
		if match, ok := m.matches[existingMatchID]; ok && match.Status == StatusRecruiting {
			return nil, fmt.Errorf("您已经在其他内战中了")
		}
		// 清理过期的记录
		delete(m.userMatches, creatorID)
	}

	// 生成唯一ID
	matchID := fmt.Sprintf("custom_%s_%d", creatorID, time.Now().Unix())

	// 如果没有提供名称，使用默认名称
	if name == "" {
		if isSpyMode {
			name = "LOL内鬼局"
		} else {
			name = "LOL内战"
		}
	}

	// 验证内鬼数量
	if isSpyMode {
		if spyCount < 1 {
			spyCount = 1 // 默认每队1个内鬼
		} else if spyCount > 4 {
			spyCount = 4 // 最多每队4个内鬼
		}
	} else {
		spyCount = 0 // 非内鬼局不设置内鬼
	}

	match := &CustomMatch{
		ID:           matchID,
		Name:         name,
		GuildID:      m.guildID,
		ChannelID:    channelID,
		CreatorID:    creatorID,
		Status:       StatusRecruiting,
		Participants: []string{creatorID}, // 创建者自动加入
		CreatedAt:    time.Now(),
		MaxPlayers:   10,
		CategoryID:   m.categoryID,
		// 内鬼局设置
		IsSpyMode: isSpyMode,
		SpyCount:  spyCount,
		BlueSpies: []string{},
		RedSpies:  []string{},
	}

	m.matches[matchID] = match
	m.userMatches[creatorID] = matchID

	if isSpyMode {
		logger.Infof("创建LOL内鬼局: %s (%s), 创建者: %s, 每队内鬼数: %d", matchID, name, creatorID, spyCount)
	} else {
		logger.Infof("创建LOL内战: %s (%s), 创建者: %s", matchID, name, creatorID)
	}
	return snapshotMatch(match), nil
}

// JoinMatch 加入内战
func (m *Manager) JoinMatch(matchID, userID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	match, exists := m.matches[matchID]
	if !exists {
		return fmt.Errorf("内战不存在")
	}

	if match.Status != StatusRecruiting {
		return fmt.Errorf("内战已经开始或结束，无法加入")
	}

	// 检查用户是否已经在其他内战中
	if existingMatchID, exists := m.userMatches[userID]; exists {
		if existingMatchID == matchID {
			return fmt.Errorf("您已经在这个内战中了")
		}
		return fmt.Errorf("您已经在其他内战中了")
	}

	// 检查人数是否已满
	if len(match.Participants) >= match.MaxPlayers {
		return fmt.Errorf("内战人数已满")
	}

	// 加入内战
	match.Participants = append(match.Participants, userID)
	m.userMatches[userID] = matchID

	logger.Infof("用户 %s 加入内战 %s", userID, matchID)

	// 如果人数满了，开始分组
	if len(match.Participants) == match.MaxPlayers {
		return m.startMatch(match)
	}

	return nil
}

// LeaveMatch 离开内战
// 返回值: error - 离开时的错误; bool - 是否取消了内战(创建者离开)
func (m *Manager) LeaveMatch(matchID, userID string) (error, bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	match, exists := m.matches[matchID]
	if !exists {
		return fmt.Errorf("内战不存在"), false
	}

	if match.Status != StatusRecruiting {
		return fmt.Errorf("内战已经开始，无法离开"), false
	}

	// 从参与者列表中移除
	for i, participant := range match.Participants {
		if participant == userID {
			match.Participants = append(match.Participants[:i], match.Participants[i+1:]...)
			delete(m.userMatches, userID)

			logger.Infof("用户 %s 离开内战 %s", userID, matchID)

			// 如果创建者离开，取消内战
			if userID == match.CreatorID {
				err := m.cancelMatch(match)
				return err, true // 返回true表示内战被取消了
			}
			return nil, false
		}
	}

	return fmt.Errorf("您不在这个内战中"), false
}

// startMatch 开始内战（内部方法）
func (m *Manager) startMatch(match *CustomMatch) error {
	if len(match.Participants) != match.MaxPlayers {
		return fmt.Errorf("参与者人数不足")
	}

	// 随机分组
	err := m.randomTeamSplit(match)
	if err != nil {
		return fmt.Errorf("分组失败: %w", err)
	}

	// 如果是内鬼局，选择内鬼
	if match.IsSpyMode {
		err = m.selectSpies(match)
		if err != nil {
			logger.Errorf("选择内鬼失败: %v", err)
			// 内鬼选择失败不影响游戏继续
		} else {
			// 发送内鬼通知
			m.notifySpies(match)
		}
	}

	// 创建语音频道
	err = m.createVoiceChannels(match)
	if err != nil {
		return fmt.Errorf("创建语音频道失败: %w", err)
	}

	match.Status = StatusInProgress
	now := time.Now()
	match.StartedAt = &now

	logger.Infof("内战开始: %s, 蓝队: %v, 红队: %v", match.ID, match.BlueTeam.Members, match.RedTeam.Members)
	if match.IsSpyMode {
		logger.Infof("内鬼局开始: %s, 蓝队内鬼: %v, 红队内鬼: %v", match.ID, match.BlueSpies, match.RedSpies)
	}

	return nil
}

// randomTeamSplit 随机分组
func (m *Manager) randomTeamSplit(match *CustomMatch) error {
	participants := make([]string, len(match.Participants))
	copy(participants, match.Participants)

	// 使用Fisher-Yates洗牌算法随机打乱
	for i := len(participants) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		participants[i], participants[j] = participants[j], participants[i]
	}

	// 分配队伍：5v5
	blueMembers := participants[:5]
	redMembers := participants[5:10]

	// 创建队伍
	match.BlueTeam = &Team{
		Name:           "蓝色方",
		Members:        blueMembers,
		Captain:        blueMembers[0], // 第一个成员作为队长
		VoiceChannelID: "",
	}

	match.RedTeam = &Team{
		Name:           "红色方",
		Members:        redMembers,
		Captain:        redMembers[0], // 第一个成员作为队长
		VoiceChannelID: "",
	}

	return nil
}

// selectSpies 选择内鬼
func (m *Manager) selectSpies(match *CustomMatch) error {
	if !match.IsSpyMode || match.SpyCount == 0 {
		return nil
	}

	// 从蓝队随机选择内鬼
	blueSpies, err := m.randomSelectFromTeam(match.BlueTeam.Members, match.SpyCount)
	if err != nil {
		return fmt.Errorf("选择蓝队内鬼失败: %w", err)
	}
	match.BlueSpies = blueSpies

	// 从红队随机选择内鬼
	redSpies, err := m.randomSelectFromTeam(match.RedTeam.Members, match.SpyCount)
	if err != nil {
		return fmt.Errorf("选择红队内鬼失败: %w", err)
	}
	match.RedSpies = redSpies

	return nil
}

// randomSelectFromTeam 从队伍中随机选择指定数量的成员
func (m *Manager) randomSelectFromTeam(members []string, count int) ([]string, error) {
	if count <= 0 || count > len(members) {
		return nil, fmt.Errorf("选择数量无效: %d, 队伍人数: %d", count, len(members))
	}

	// 复制成员列表
	membersCopy := make([]string, len(members))
	copy(membersCopy, members)

	// 使用Fisher-Yates洗牌算法
	for i := len(membersCopy) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		membersCopy[i], membersCopy[j] = membersCopy[j], membersCopy[i]
	}

	// 返回前count个成员
	return membersCopy[:count], nil
}

// notifySpies 通知内鬼
func (m *Manager) notifySpies(match *CustomMatch) {
	if !match.IsSpyMode {
		return
	}

	// 通知蓝队内鬼
	for _, spyID := range match.BlueSpies {
		go m.sendSpyNotification(spyID, match, "蓝色方")
	}

	// 通知红队内鬼
	for _, spyID := range match.RedSpies {
		go m.sendSpyNotification(spyID, match, "红色方")
	}
}

// sendSpyNotification 发送内鬼通知私信
func (m *Manager) sendSpyNotification(userID string, match *CustomMatch, teamName string) {
	channel, err := m.session.UserChannelCreate(userID)
	if err != nil {
		logger.Errorf("创建私信频道失败 (用户 %s): %v", userID, err)
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       "🕵️ 内鬼任务",
		Description: fmt.Sprintf("您在内战 **%s** 中被选为 **%s** 的内鬼！", match.Name, teamName),
		Color:       0x8B0000, // 深红色
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "🎯 任务目标",
				Value:  "在不被发现的情况下，让您的队伍输掉比赛",
				Inline: false,
			},
			{
				Name:   "⚠️ 注意事项",
				Value:  "• 不要太明显地送人头或做出异常行为\n• 适度地影响团战和决策\n• 比赛结束后会有投票环节",
				Inline: false,
			},
			{
				Name:   "🤫 保密",
				Value:  "请勿向任何人透露您的内鬼身份！",
				Inline: false,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("内战ID: %s", match.ID[7:13]),
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	_, err = m.session.ChannelMessageSendEmbed(channel.ID, embed)
	if err != nil {
		logger.Errorf("发送内鬼通知失败 (用户 %s): %v", userID, err)
	} else {
		logger.Infof("已发送内鬼通知给用户 %s (队伍: %s)", userID, teamName)
	}
}

// cancelMatch 取消内战
func (m *Manager) cancelMatch(match *CustomMatch) error {
	match.Status = StatusCancelled

	// 清理用户映射
	for _, participant := range match.Participants {
		delete(m.userMatches, participant)
	}

	logger.Infof("内战 %s 已取消", match.ID)
	return nil
}

// snapshotMatch 返回 match 的深拷贝。Manager 在锁内修改 match 字段，
// 处理器一律拿快照读取，避免与 JoinMatch/LeaveMatch 的并发读写竞争。
// 必须在持有 m.mutex 时调用。
func snapshotMatch(match *CustomMatch) *CustomMatch {
	cp := *match
	cp.Participants = append([]string(nil), match.Participants...)
	cp.BlueSpies = append([]string(nil), match.BlueSpies...)
	cp.RedSpies = append([]string(nil), match.RedSpies...)
	if match.BlueTeam != nil {
		t := *match.BlueTeam
		t.Members = append([]string(nil), match.BlueTeam.Members...)
		cp.BlueTeam = &t
	}
	if match.RedTeam != nil {
		t := *match.RedTeam
		t.Members = append([]string(nil), match.RedTeam.Members...)
		cp.RedTeam = &t
	}
	return &cp
}

// GetMatch 获取内战信息（返回快照副本）
func (m *Manager) GetMatch(matchID string) (*CustomMatch, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	match, exists := m.matches[matchID]
	if !exists {
		return nil, false
	}
	return snapshotMatch(match), true
}

// SetMessageID 在锁保护下更新内战的招募消息ID
func (m *Manager) SetMessageID(matchID, messageID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if match, exists := m.matches[matchID]; exists {
		match.MessageID = messageID
	}
}

// GetUserMatch 获取用户当前参与的内战
func (m *Manager) GetUserMatch(userID string) (*CustomMatch, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	matchID, exists := m.userMatches[userID]
	if !exists {
		return nil, false
	}

	match, exists := m.matches[matchID]
	if !exists {
		return nil, false
	}
	return snapshotMatch(match), true
}

// CompleteMatch 完成内战
func (m *Manager) CompleteMatch(matchID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	match, exists := m.matches[matchID]
	if !exists {
		return fmt.Errorf("内战不存在")
	}

	match.Status = StatusCompleted
	now := time.Now()
	match.CompletedAt = &now

	// 清理用户映射
	for _, participant := range match.Participants {
		delete(m.userMatches, participant)
	}

	// 可以选择性地删除语音频道
	go m.cleanupVoiceChannels(match)

	logger.Infof("内战 %s 已完成", match.ID)
	return nil
}

// cleanupVoiceChannels 清理语音频道（延迟执行）
func (m *Manager) cleanupVoiceChannels(match *CustomMatch) {
	// 等待30分钟后清理频道
	time.Sleep(m.cleanupDelay)

	if match.BlueTeam != nil && match.BlueTeam.VoiceChannelID != "" {
		if _, err := m.session.ChannelDelete(match.BlueTeam.VoiceChannelID); err != nil {
			logger.Errorf("删除蓝队语音频道失败: %v", err)
		}
	}

	if match.RedTeam != nil && match.RedTeam.VoiceChannelID != "" {
		if _, err := m.session.ChannelDelete(match.RedTeam.VoiceChannelID); err != nil {
			logger.Errorf("删除红队语音频道失败: %v", err)
		}
	}
}

// createVoiceChannels 创建语音频道
func (m *Manager) createVoiceChannels(match *CustomMatch) error {
	// 获取队长用户名
	var blueCaptainName, redCaptainName string

	// 获取蓝队队长名称
	if blueUser, err := m.session.User(match.BlueTeam.Captain); err == nil {
		blueCaptainName = blueUser.Username
	} else {
		// 如果获取失败，使用用户ID
		blueCaptainName = match.BlueTeam.Captain
	}

	// 获取红队队长名称
	if redUser, err := m.session.User(match.RedTeam.Captain); err == nil {
		redCaptainName = redUser.Username
	} else {
		// 如果获取失败，使用用户ID
		redCaptainName = match.RedTeam.Captain
	}

	// 创建蓝队语音频道
	blueChannelData := discordgo.GuildChannelCreateData{
		Name:      fmt.Sprintf("🔵 LOL内战 - 蓝队 [%s-%s]", match.ID[7:13], blueCaptainName),
		Type:      discordgo.ChannelTypeGuildVoice,
		ParentID:  match.CategoryID,
		UserLimit: 5,
	}

	blueChannel, err := m.session.GuildChannelCreateComplex(m.guildID, blueChannelData)
	if err != nil {
		return fmt.Errorf("创建蓝队语音频道失败: %w", err)
	}
	match.BlueTeam.VoiceChannelID = blueChannel.ID

	// 创建红队语音频道
	redChannelData := discordgo.GuildChannelCreateData{
		Name:      fmt.Sprintf("🔴 LOL内战 - 红队 [%s-%s]", match.ID[7:13], redCaptainName),
		Type:      discordgo.ChannelTypeGuildVoice,
		ParentID:  match.CategoryID,
		UserLimit: 5,
	}

	redChannel, err := m.session.GuildChannelCreateComplex(m.guildID, redChannelData)
	if err != nil {
		// 如果红队频道创建失败，删除已创建的蓝队频道
		m.session.ChannelDelete(blueChannel.ID)
		return fmt.Errorf("创建红队语音频道失败: %w", err)
	}
	match.RedTeam.VoiceChannelID = redChannel.ID

	return nil
}
