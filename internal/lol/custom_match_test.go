package lol

import (
	"fmt"
	"testing"
	"time"
)

func TestManager_CreateMatch(t *testing.T) {
	manager := NewManager(nil, "test-guild", "test-category", 30*time.Minute)

	// 测试创建普通内战
	match, err := manager.CreateMatch("test-channel", "user1", "测试内战", false, 0)
	if err != nil {
		t.Fatalf("创建内战失败: %v", err)
	}

	if match.ID == "" {
		t.Error("内战ID不能为空")
	}

	if match.CreatorID != "user1" {
		t.Errorf("创建者ID错误: expected user1, got %s", match.CreatorID)
	}

	if match.Name != "测试内战" {
		t.Errorf("内战名称错误: expected 测试内战, got %s", match.Name)
	}

	if match.IsSpyMode {
		t.Error("普通内战不应该是内鬼局")
	}

	if match.MaxPlayers != 10 {
		t.Errorf("最大玩家数错误: expected 10, got %d", match.MaxPlayers)
	}

	if len(match.Participants) != 1 {
		t.Errorf("参与者数量错误: expected 1, got %d", len(match.Participants))
	}

	if match.Status != StatusRecruiting {
		t.Errorf("内战状态错误: expected %d, got %d", StatusRecruiting, match.Status)
	}

	// 测试重复创建
	_, err = manager.CreateMatch("test-channel", "user1", "重复内战", false, 0)
	if err == nil {
		t.Error("应该阻止重复创建内战")
	}

	// 测试创建内鬼局
	manager2 := NewManager(nil, "test-guild", "test-category", 30*time.Minute)
	spyMatch, err := manager2.CreateMatch("test-channel", "user2", "内鬼局测试", true, 2)
	if err != nil {
		t.Fatalf("创建内鬼局失败: %v", err)
	}

	if !spyMatch.IsSpyMode {
		t.Error("应该是内鬼局")
	}

	if spyMatch.SpyCount != 2 {
		t.Errorf("内鬼数量错误: expected 2, got %d", spyMatch.SpyCount)
	}

	// 测试默认名称
	manager3 := NewManager(nil, "test-guild", "test-category", 30*time.Minute)
	match3, err := manager3.CreateMatch("test-channel", "user4", "", false, 0)
	if err != nil {
		t.Fatalf("创建默认名称内战失败: %v", err)
	}
	if match3.Name != "LOL内战" {
		t.Errorf("默认名称错误: expected LOL内战, got %s", match3.Name)
	}

	// 测试内鬼局默认名称
	spyMatch2, err := manager3.CreateMatch("test-channel", "user5", "", true, 1)
	if err != nil {
		t.Fatalf("创建内鬼局默认名称失败: %v", err)
	}
	if spyMatch2.Name != "LOL内鬼局" {
		t.Errorf("内鬼局默认名称错误: expected LOL内鬼局, got %s", spyMatch2.Name)
	}
}

func TestManager_JoinMatch(t *testing.T) {
	manager := NewManager(nil, "test-guild", "test-category", 30*time.Minute)

	// 创建内战
	match, _ := manager.CreateMatch("test-channel", "user1", "加入测试", false, 0)

	// 测试加入内战
	err := manager.JoinMatch(match.ID, "user2")
	if err != nil {
		t.Fatalf("加入内战失败: %v", err)
	}

	// GetMatch 返回快照，需重新获取最新状态
	updated, _ := manager.GetMatch(match.ID)
	if len(updated.Participants) != 2 {
		t.Errorf("参与者数量错误: expected 2, got %d", len(updated.Participants))
	}

	// 测试重复加入
	err = manager.JoinMatch(match.ID, "user2")
	if err == nil {
		t.Error("应该阻止重复加入")
	}

	// 测试加入不存在的内战
	err = manager.JoinMatch("nonexistent", "user3")
	if err == nil {
		t.Error("应该阻止加入不存在的内战")
	}
}

func TestManager_LeaveMatch(t *testing.T) {
	manager := NewManager(nil, "test-guild", "test-category", 30*time.Minute)

	// 创建内战并加入用户
	match, _ := manager.CreateMatch("test-channel", "user1", "离开测试", false, 0)
	manager.JoinMatch(match.ID, "user2")

	// 测试普通用户离开
	err, cancelled := manager.LeaveMatch(match.ID, "user2")
	if err != nil {
		t.Fatalf("离开内战失败: %v", err)
	}

	if cancelled {
		t.Error("普通用户离开不应该取消内战")
	}

	// GetMatch 返回快照，需重新获取最新状态
	updated, _ := manager.GetMatch(match.ID)
	if len(updated.Participants) != 1 {
		t.Errorf("参与者数量错误: expected 1, got %d", len(updated.Participants))
	}

	// 测试创建者离开（应该取消内战）
	err, cancelled = manager.LeaveMatch(match.ID, "user1")
	if err != nil {
		t.Fatalf("创建者离开失败: %v", err)
	}

	if !cancelled {
		t.Error("创建者离开应该取消内战")
	}

	updated, exists := manager.GetMatch(match.ID)
	if exists && updated.Status != StatusCancelled {
		t.Errorf("内战状态错误: expected %d, got %d", StatusCancelled, updated.Status)
	}
}

func TestManager_RandomTeamSplit(t *testing.T) {
	manager := NewManager(nil, "test-guild", "test-category", 30*time.Minute)

	// 创建内战并手动设置参与者（避免触发创建频道）
	match := &CustomMatch{
		ID:           "test-match",
		GuildID:      "test-guild",
		ChannelID:    "test-channel",
		CreatorID:    "user1",
		Status:       StatusRecruiting,
		Participants: make([]string, 10),
		MaxPlayers:   10,
		CategoryID:   "test-category",
	}

	// 添加10个用户
	for i := 0; i < 10; i++ {
		match.Participants[i] = fmt.Sprintf("user%d", i+1)
	}

	// 测试随机分组（直接调用内部方法，避免触发频道创建）
	err := manager.randomTeamSplit(match)
	if err != nil {
		t.Fatalf("随机分组失败: %v", err)
	}

	// 检查分组结果
	if match.BlueTeam == nil || match.RedTeam == nil {
		t.Fatal("分组后队伍不能为空")
	}

	if len(match.BlueTeam.Members) != 5 {
		t.Errorf("蓝队人数错误: expected 5, got %d", len(match.BlueTeam.Members))
	}

	if len(match.RedTeam.Members) != 5 {
		t.Errorf("红队人数错误: expected 5, got %d", len(match.RedTeam.Members))
	}

	// 检查队长
	if match.BlueTeam.Captain == "" {
		t.Error("蓝队队长不能为空")
	}

	if match.RedTeam.Captain == "" {
		t.Error("红队队长不能为空")
	}

	// 检查所有参与者都被分配
	allMembers := append(match.BlueTeam.Members, match.RedTeam.Members...)
	if len(allMembers) != 10 {
		t.Errorf("分组后总人数错误: expected 10, got %d", len(allMembers))
	}

	// 检查没有重复成员
	memberMap := make(map[string]bool)
	for _, member := range allMembers {
		if memberMap[member] {
			t.Errorf("发现重复成员: %s", member)
		}
		memberMap[member] = true
	}
}

func TestManager_GetUserMatch(t *testing.T) {
	manager := NewManager(nil, "test-guild", "test-category", 30*time.Minute)

	// 创建内战
	match, _ := manager.CreateMatch("test-channel", "user1", "用户匹配测试", false, 0)

	// 测试获取用户内战
	userMatch, exists := manager.GetUserMatch("user1")
	if !exists {
		t.Error("应该能找到用户的内战")
	}

	if userMatch.ID != match.ID {
		t.Errorf("内战ID不匹配: expected %s, got %s", match.ID, userMatch.ID)
	}

	// 测试不存在的用户
	_, exists = manager.GetUserMatch("nonexistent")
	if exists {
		t.Error("不应该找到不存在用户的内战")
	}
}

func TestManager_CompleteMatch(t *testing.T) {
	manager := NewManager(nil, "test-guild", "test-category", 30*time.Minute)

	// 创建内战
	match, _ := manager.CreateMatch("test-channel", "user1", "完成测试", false, 0)

	// 完成内战
	err := manager.CompleteMatch(match.ID)
	if err != nil {
		t.Fatalf("完成内战失败: %v", err)
	}

	// GetMatch 返回快照，需重新获取最新状态
	updated, _ := manager.GetMatch(match.ID)
	if updated.Status != StatusCompleted {
		t.Errorf("内战状态错误: expected %d, got %d", StatusCompleted, updated.Status)
	}

	if updated.CompletedAt == nil {
		t.Error("完成时间不能为空")
	}

	// 用户映射应该被清理
	_, exists := manager.GetUserMatch("user1")
	if exists {
		t.Error("完成后用户映射应该被清理")
	}
}

func TestCustomMatchStatus(t *testing.T) {
	// 测试状态常量
	if StatusRecruiting != 0 {
		t.Errorf("StatusRecruiting 应该为 0, 得到 %d", StatusRecruiting)
	}

	if StatusInProgress != 1 {
		t.Errorf("StatusInProgress 应该为 1, 得到 %d", StatusInProgress)
	}

	if StatusCompleted != 2 {
		t.Errorf("StatusCompleted 应该为 2, 得到 %d", StatusCompleted)
	}

	if StatusCancelled != 3 {
		t.Errorf("StatusCancelled 应该为 3, 得到 %d", StatusCancelled)
	}
}

func TestTeam(t *testing.T) {
	team := &Team{
		Name:           "蓝色方",
		Members:        []string{"user1", "user2", "user3", "user4", "user5"},
		Captain:        "user1",
		VoiceChannelID: "channel123",
	}

	if team.Name != "蓝色方" {
		t.Errorf("队伍名称错误: expected 蓝色方, got %s", team.Name)
	}

	if len(team.Members) != 5 {
		t.Errorf("队伍成员数量错误: expected 5, got %d", len(team.Members))
	}

	if team.Captain != "user1" {
		t.Errorf("队长错误: expected user1, got %s", team.Captain)
	}
}

func TestCustomMatch(t *testing.T) {
	now := time.Now()
	match := &CustomMatch{
		ID:           "test-match",
		GuildID:      "test-guild",
		ChannelID:    "test-channel",
		CreatorID:    "user1",
		Status:       StatusRecruiting,
		Participants: []string{"user1"},
		CreatedAt:    now,
		MaxPlayers:   10,
		CategoryID:   "test-category",
	}

	if match.ID != "test-match" {
		t.Errorf("内战ID错误: expected test-match, got %s", match.ID)
	}

	if match.MaxPlayers != 10 {
		t.Errorf("最大玩家数错误: expected 10, got %d", match.MaxPlayers)
	}

	if len(match.Participants) != 1 {
		t.Errorf("参与者数量错误: expected 1, got %d", len(match.Participants))
	}
}

// Benchmark测试随机分组性能
func BenchmarkRandomTeamSplit(b *testing.B) {
	manager := NewManager(nil, "test-guild", "test-category", 30*time.Minute)

	// 准备测试数据
	match := &CustomMatch{
		Participants: make([]string, 10),
	}

	for i := 0; i < 10; i++ {
		match.Participants[i] = fmt.Sprintf("user%d", i+1)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.randomTeamSplit(match)
	}
}

func TestManager_SpyMode(t *testing.T) {
	manager := NewManager(nil, "test-guild", "test-category", 30*time.Minute)

	// 创建内鬼局
	match, err := manager.CreateMatch("test-channel", "user1", "内鬼局测试", true, 2)
	if err != nil {
		t.Fatalf("创建内鬼局失败: %v", err)
	}

	if !match.IsSpyMode {
		t.Error("应该是内鬼局")
	}

	if match.SpyCount != 2 {
		t.Errorf("内鬼数量错误: expected 2, got %d", match.SpyCount)
	}

	// 添加参与者但不满员（避免自动开始触发私信）
	for i := 2; i <= 9; i++ {
		err := manager.JoinMatch(match.ID, fmt.Sprintf("user%d", i))
		if err != nil {
			t.Fatalf("用户%d加入失败: %v", i, err)
		}
	}

	// 验证还在招募状态
	if match.Status != StatusRecruiting {
		t.Errorf("内战状态错误: expected %d, got %d", StatusRecruiting, match.Status)
	}

	// 手动设置参与者和分组进行内鬼选择测试
	match.Participants = []string{"user1", "user2", "user3", "user4", "user5", "user6", "user7", "user8", "user9", "user10"}

	// 手动分组
	match.BlueTeam = &Team{
		Name:    "蓝色方",
		Members: []string{"user1", "user2", "user3", "user4", "user5"},
		Captain: "user1",
	}
	match.RedTeam = &Team{
		Name:    "红色方",
		Members: []string{"user6", "user7", "user8", "user9", "user10"},
		Captain: "user6",
	}

	// 测试内鬼选择功能
	err = manager.selectSpies(match)
	if err != nil {
		t.Fatalf("选择内鬼失败: %v", err)
	}

	// 验证内鬼选择
	if len(match.BlueSpies) != 2 {
		t.Errorf("蓝队内鬼数量错误: expected 2, got %d", len(match.BlueSpies))
	}

	if len(match.RedSpies) != 2 {
		t.Errorf("红队内鬼数量错误: expected 2, got %d", len(match.RedSpies))
	}

	// 验证内鬼都在对应队伍中
	for _, spy := range match.BlueSpies {
		found := false
		for _, member := range match.BlueTeam.Members {
			if member == spy {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("蓝队内鬼 %s 不在蓝队中", spy)
		}
	}

	for _, spy := range match.RedSpies {
		found := false
		for _, member := range match.RedTeam.Members {
			if member == spy {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("红队内鬼 %s 不在红队中", spy)
		}
	}
}

func TestManager_RandomSelectFromTeam(t *testing.T) {
	manager := NewManager(nil, "test-guild", "test-category", 30*time.Minute)

	members := []string{"user1", "user2", "user3", "user4", "user5"}

	// 测试正常选择
	selected, err := manager.randomSelectFromTeam(members, 2)
	if err != nil {
		t.Fatalf("随机选择失败: %v", err)
	}

	if len(selected) != 2 {
		t.Errorf("选择数量错误: expected 2, got %d", len(selected))
	}

	// 验证选择的成员都在原队伍中
	for _, sel := range selected {
		found := false
		for _, mem := range members {
			if mem == sel {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("选择的成员 %s 不在原队伍中", sel)
		}
	}

	// 测试边界情况
	_, err = manager.randomSelectFromTeam(members, 0)
	if err == nil {
		t.Error("应该拒绝选择0个成员")
	}

	_, err = manager.randomSelectFromTeam(members, 6)
	if err == nil {
		t.Error("应该拒绝选择超过队伍人数的成员")
	}
}
