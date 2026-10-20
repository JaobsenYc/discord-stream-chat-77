package team

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTeam_AddPlayer(t *testing.T) {
	team := &Team{
		ID:         "test-team",
		Game:       "LOL",
		MaxPlayers: 5,
		Players:    make([]Player, 0),
	}

	tests := []struct {
		name        string
		userID      string
		username    string
		expectError bool
	}{
		{
			name:        "添加第一个玩家",
			userID:      "user1",
			username:    "Player1",
			expectError: false,
		},
		{
			name:        "添加第二个玩家",
			userID:      "user2",
			username:    "Player2",
			expectError: false,
		},
		{
			name:        "添加重复玩家",
			userID:      "user1",
			username:    "Player1",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := team.AddPlayer(tt.userID, tt.username)

			if tt.expectError {
				if err == nil {
					t.Errorf("期望出现错误，但没有错误")
				}
				return
			}

			if err != nil {
				t.Errorf("意外错误: %v", err)
				return
			}

			// 检查玩家是否添加成功
			if !team.HasPlayer(tt.userID) {
				t.Errorf("玩家 %s 未成功添加到团队", tt.userID)
			}
		})
	}
}

func TestTeam_AddPlayer_FullTeam(t *testing.T) {
	team := &Team{
		ID:         "test-team",
		Game:       "LOL",
		MaxPlayers: 2, // 设置小的最大人数便于测试
		Players:    make([]Player, 0),
	}

	// 添加到满员
	_ = team.AddPlayer("user1", "Player1")
	_ = team.AddPlayer("user2", "Player2")

	// 尝试添加第三个玩家，应该失败
	err := team.AddPlayer("user3", "Player3")
	if err == nil {
		t.Errorf("期望团队满员错误，但没有错误")
	}

	if !team.IsFull() {
		t.Errorf("团队应该已满员")
	}
}

func TestTeam_RemovePlayer(t *testing.T) {
	team := &Team{
		ID:         "test-team",
		Game:       "LOL",
		MaxPlayers: 5,
		Players:    make([]Player, 0),
	}

	// 先添加一些玩家
	_ = team.AddPlayer("user1", "Player1")
	_ = team.AddPlayer("user2", "Player2")

	tests := []struct {
		name        string
		userID      string
		expectError bool
	}{
		{
			name:        "移除存在的玩家",
			userID:      "user1",
			expectError: false,
		},
		{
			name:        "移除不存在的玩家",
			userID:      "user3",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := team.RemovePlayer(tt.userID)

			if tt.expectError {
				if err == nil {
					t.Errorf("期望出现错误，但没有错误")
				}
				return
			}

			if err != nil {
				t.Errorf("意外错误: %v", err)
				return
			}

			// 检查玩家是否移除成功
			if team.HasPlayer(tt.userID) {
				t.Errorf("玩家 %s 未成功从团队移除", tt.userID)
			}
		})
	}
}

func TestTeam_HasPlayer(t *testing.T) {
	team := &Team{
		ID:         "test-team",
		Game:       "LOL",
		MaxPlayers: 5,
		Players: []Player{
			{UserID: "user1", Username: "Player1"},
			{UserID: "user2", Username: "Player2"},
		},
	}

	tests := []struct {
		name     string
		userID   string
		expected bool
	}{
		{
			name:     "存在的玩家",
			userID:   "user1",
			expected: true,
		},
		{
			name:     "不存在的玩家",
			userID:   "user3",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := team.HasPlayer(tt.userID)
			if result != tt.expected {
				t.Errorf("期望 %v, 实际 %v", tt.expected, result)
			}
		})
	}
}

func TestTeam_IsFull(t *testing.T) {
	tests := []struct {
		name       string
		maxPlayers int
		players    []Player
		expected   bool
	}{
		{
			name:       "空团队",
			maxPlayers: 5,
			players:    []Player{},
			expected:   false,
		},
		{
			name:       "部分满员",
			maxPlayers: 5,
			players:    []Player{{UserID: "user1"}},
			expected:   false,
		},
		{
			name:       "满员团队",
			maxPlayers: 2,
			players:    []Player{{UserID: "user1"}, {UserID: "user2"}},
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			team := &Team{
				MaxPlayers: tt.maxPlayers,
				Players:    tt.players,
			}

			result := team.IsFull()
			if result != tt.expected {
				t.Errorf("期望 %v, 实际 %v", tt.expected, result)
			}
		})
	}
}

func TestTeam_IsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		players  []Player
		expected bool
	}{
		{
			name:     "空团队",
			players:  []Player{},
			expected: true,
		},
		{
			name:     "有玩家的团队",
			players:  []Player{{UserID: "user1"}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			team := &Team{
				Players: tt.players,
			}

			result := team.IsEmpty()
			if result != tt.expected {
				t.Errorf("期望 %v, 实际 %v", tt.expected, result)
			}
		})
	}
}

func TestTeam_GetStatusString(t *testing.T) {
	tests := []struct {
		name       string
		game       string
		maxPlayers int
		players    []Player
		expected   string
	}{
		{
			name:       "空团队",
			game:       "LOL",
			maxPlayers: 5,
			players:    []Player{},
			expected:   "🎮 **LOL** 寻组 (0/5)",
		},
		{
			name:       "部分满员",
			game:       "Valorant",
			maxPlayers: 5,
			players:    []Player{{UserID: "user1"}},
			expected:   "🎮 **Valorant** 寻组 (1/5)",
		},
		{
			name:       "满员团队",
			game:       "CS2",
			maxPlayers: 2,
			players:    []Player{{UserID: "user1"}, {UserID: "user2"}},
			expected:   "✅ **CS2** 寻组 (2/2)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			team := &Team{
				Game:       tt.game,
				MaxPlayers: tt.maxPlayers,
				Players:    tt.players,
			}

			result := team.GetStatusString()
			if result != tt.expected {
				t.Errorf("期望 %s, 实际 %s", tt.expected, result)
			}
		})
	}
}

func TestManager_CreateTeam(t *testing.T) {
	manager := NewManager()

	team := manager.CreateTeam("team1", "LOL", 5, "creator1", "channel1")

	if team == nil {
		t.Errorf("创建团队失败")
		return
	}

	if team.ID != "team1" {
		t.Errorf("团队ID不匹配: 期望 team1, 实际 %s", team.ID)
	}

	if team.Game != "LOL" {
		t.Errorf("游戏名不匹配: 期望 LOL, 实际 %s", team.Game)
	}

	if team.MaxPlayers != 5 {
		t.Errorf("最大人数不匹配: 期望 5, 实际 %d", team.MaxPlayers)
	}

	if team.CreatorID != "creator1" {
		t.Errorf("创建者ID不匹配: 期望 creator1, 实际 %s", team.CreatorID)
	}

	if team.ChannelID != "channel1" {
		t.Errorf("频道ID不匹配: 期望 channel1, 实际 %s", team.ChannelID)
	}

	// 检查团队是否添加到管理器
	retrievedTeam, exists := manager.GetTeam("team1")
	if !exists {
		t.Errorf("团队未添加到管理器")
	}

	if retrievedTeam != team {
		t.Errorf("检索到的团队不是同一个实例")
	}
}

func TestManager_GetTeam(t *testing.T) {
	manager := NewManager()

	// 创建一个团队
	originalTeam := manager.CreateTeam("team1", "LOL", 5, "creator1", "channel1")

	tests := []struct {
		name     string
		teamID   string
		expected bool
	}{
		{
			name:     "存在的团队",
			teamID:   "team1",
			expected: true,
		},
		{
			name:     "不存在的团队",
			teamID:   "team2",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			team, exists := manager.GetTeam(tt.teamID)

			if exists != tt.expected {
				t.Errorf("期望存在性 %v, 实际 %v", tt.expected, exists)
			}

			if tt.expected && team != originalTeam {
				t.Errorf("返回的团队不是预期的团队")
			}
		})
	}
}

func TestManager_GetTeamByChannel(t *testing.T) {
	manager := NewManager()

	// 创建一些团队
	team1 := manager.CreateTeam("team1", "LOL", 5, "creator1", "channel1")
	manager.CreateTeam("team2", "Valorant", 5, "creator2", "channel2")

	tests := []struct {
		name        string
		channelID   string
		expectFound bool
		expectedID  string
	}{
		{
			name:        "存在的频道",
			channelID:   "channel1",
			expectFound: true,
			expectedID:  "team1",
		},
		{
			name:        "不存在的频道",
			channelID:   "channel3",
			expectFound: false,
			expectedID:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			team, found := manager.GetTeamByChannel(tt.channelID)

			if found != tt.expectFound {
				t.Errorf("期望找到 %v, 实际 %v", tt.expectFound, found)
			}

			if tt.expectFound {
				if team.ID != tt.expectedID {
					t.Errorf("期望团队ID %s, 实际 %s", tt.expectedID, team.ID)
				}
				if team != team1 {
					t.Errorf("返回的团队不是预期的团队")
				}
			}
		})
	}
}

func TestManager_RemoveTeam(t *testing.T) {
	manager := NewManager()

	// 创建一个团队
	manager.CreateTeam("team1", "LOL", 5, "creator1", "channel1")

	// 验证团队存在
	_, exists := manager.GetTeam("team1")
	if !exists {
		t.Errorf("团队创建失败")
	}

	// 移除团队
	manager.RemoveTeam("team1")

	// 验证团队已移除
	_, exists = manager.GetTeam("team1")
	if exists {
		t.Errorf("团队未成功移除")
	}
}

func TestManager_GetAllTeams(t *testing.T) {
	manager := NewManager()

	// 创建多个团队
	manager.CreateTeam("team1", "LOL", 5, "creator1", "channel1")
	manager.CreateTeam("team2", "Valorant", 5, "creator2", "channel2")
	manager.CreateTeam("team3", "CS2", 5, "creator3", "channel3")

	teams := manager.GetAllTeams()

	if len(teams) != 3 {
		t.Errorf("期望 3 个团队, 实际 %d", len(teams))
	}

	// 检查团队ID
	teamIDs := make(map[string]bool)
	for _, team := range teams {
		teamIDs[team.ID] = true
	}

	expectedIDs := []string{"team1", "team2", "team3"}
	for _, id := range expectedIDs {
		if !teamIDs[id] {
			t.Errorf("缺少团队 %s", id)
		}
	}
}

func TestManager_GetExpiredTeams(t *testing.T) {
	manager := NewManager()

	// 创建团队并设置不同的最后活动时间
	team1 := manager.CreateTeam("team1", "LOL", 5, "creator1", "channel1")
	team2 := manager.CreateTeam("team2", "Valorant", 5, "creator2", "channel2")
	team3 := manager.CreateTeam("team3", "CS2", 5, "creator3", "channel3")

	// 设置过期时间
	now := time.Now()
	team1.LastActivity = now.Add(-2 * time.Hour)    // 2小时前
	team2.LastActivity = now.Add(-30 * time.Minute) // 30分钟前
	team3.LastActivity = now.Add(-5 * time.Minute)  // 5分钟前

	// 获取1小时前的过期团队
	timeout := 1 * time.Hour
	expiredTeams := manager.GetExpiredTeams(timeout)

	if len(expiredTeams) != 1 {
		t.Errorf("期望 1 个过期团队, 实际 %d", len(expiredTeams))
	}

	if len(expiredTeams) > 0 && expiredTeams[0].ID != "team1" {
		t.Errorf("期望过期团队是 team1, 实际 %s", expiredTeams[0].ID)
	}
}

func TestManager_GetTeamCount(t *testing.T) {
	manager := NewManager()

	// 初始应该是0
	if count := manager.GetTeamCount(); count != 0 {
		t.Errorf("初始团队数量应该是0, 实际 %d", count)
	}

	// 添加团队
	manager.CreateTeam("team1", "LOL", 5, "creator1", "channel1")
	if count := manager.GetTeamCount(); count != 1 {
		t.Errorf("添加1个团队后数量应该是1, 实际 %d", count)
	}

	manager.CreateTeam("team2", "Valorant", 5, "creator2", "channel2")
	if count := manager.GetTeamCount(); count != 2 {
		t.Errorf("添加2个团队后数量应该是2, 实际 %d", count)
	}

	// 移除团队
	manager.RemoveTeam("team1")
	if count := manager.GetTeamCount(); count != 1 {
		t.Errorf("移除1个团队后数量应该是1, 实际 %d", count)
	}
}

// 并发安全测试
func TestManager_ConcurrentAccess(t *testing.T) {
	manager := NewManager()

	// 启动多个goroutine并发操作
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			teamID := fmt.Sprintf("team%d", id)

			// 创建团队
			manager.CreateTeam(teamID, "LOL", 5, "creator", "channel")

			// 获取团队
			_, _ = manager.GetTeam(teamID)

			// 获取所有团队
			_ = manager.GetAllTeams()

			// 移除团队
			manager.RemoveTeam(teamID)
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 最终应该没有团队
	if count := manager.GetTeamCount(); count != 0 {
		t.Errorf("并发测试后应该没有团队, 实际 %d", count)
	}
}

// TestUserChannelTracking 测试用户频道追踪功能
func TestUserChannelTracking(t *testing.T) {
	manager := NewManager()

	userID := "user123"
	channelID := "channel456"

	// 测试获取不存在的用户频道
	_, exists := manager.GetUserChannel(userID)
	assert.False(t, exists, "不存在的用户不应该有频道记录")

	// 测试设置用户频道
	manager.SetUserChannel(userID, channelID)
	retrievedChannelID, exists := manager.GetUserChannel(userID)
	assert.True(t, exists, "设置后应该能获取到用户频道")
	assert.Equal(t, channelID, retrievedChannelID, "频道ID应该匹配")

	// 测试移除用户频道
	manager.RemoveUserChannel(userID)
	_, exists = manager.GetUserChannel(userID)
	assert.False(t, exists, "移除后不应该有频道记录")
}

// TestCreateTeamWithUserChannelTracking 测试创建团队时的用户频道追踪
func TestCreateTeamWithUserChannelTracking(t *testing.T) {
	manager := NewManager()

	userID := "user123"
	channelID := "channel456"

	// 创建团队
	team := manager.CreateTeam("team1", "LOL", 5, userID, channelID)

	// 验证团队创建
	assert.NotNil(t, team, "团队应该创建成功")
	assert.Equal(t, userID, team.CreatorID, "创建者ID应该匹配")
	assert.Equal(t, channelID, team.ChannelID, "频道ID应该匹配")

	// 验证用户频道记录
	retrievedChannelID, exists := manager.GetUserChannel(userID)
	assert.True(t, exists, "创建团队后应该有用户频道记录")
	assert.Equal(t, channelID, retrievedChannelID, "用户频道记录应该匹配")
}

// TestRemoveTeamWithUserChannelTracking 测试删除团队时清理用户频道记录
func TestRemoveTeamWithUserChannelTracking(t *testing.T) {
	manager := NewManager()

	userID := "user123"
	channelID := "channel456"

	// 创建团队
	manager.CreateTeam("team1", "LOL", 5, userID, channelID)

	// 验证用户频道记录存在
	_, exists := manager.GetUserChannel(userID)
	assert.True(t, exists, "创建团队后应该有用户频道记录")

	// 删除团队
	manager.RemoveTeam("team1")

	// 验证用户频道记录被清理
	_, exists = manager.GetUserChannel(userID)
	assert.False(t, exists, "删除团队后应该清理用户频道记录")

	// 验证团队被删除
	_, exists = manager.GetTeam("team1")
	assert.False(t, exists, "团队应该被删除")
}
