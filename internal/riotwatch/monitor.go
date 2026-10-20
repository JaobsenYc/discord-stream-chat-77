package riotwatch

import (
	"context"
	"sync"
	"time"

	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// Monitor Riot Watch 后台监控器
type Monitor struct {
	service *Service
	guildID string

	// 轮询间隔
	pollInterval     time.Duration // 检查新游戏的间隔
	checkEndInterval time.Duration // 检查游戏结束的间隔

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewMonitor 创建新的监控器
func NewMonitor(service *Service, guildID string) *Monitor {
	ctx, cancel := context.WithCancel(context.Background())

	return &Monitor{
		service:          service,
		guildID:          guildID,
		pollInterval:     30 * time.Second, // 每 30 秒检查新游戏
		checkEndInterval: 15 * time.Second, // 每 15 秒检查游戏结束
		ctx:              ctx,
		cancel:           cancel,
	}
}

// Start 启动监控器
func (m *Monitor) Start() {
	logger.Info("Riot Watch 监控器启动")

	m.wg.Add(2)
	go m.pollNewGames()
	go m.checkGameEnds()
}

// Stop 停止监控器
func (m *Monitor) Stop() {
	logger.Info("Riot Watch 监控器停止中...")
	m.cancel()
	m.wg.Wait()
	logger.Info("Riot Watch 监控器已停止")
}

// pollNewGames 轮询检查新游戏
func (m *Monitor) pollNewGames() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	// 启动时立即检查一次
	m.checkAllBindingsForNewGames()

	for {
		select {
		case <-m.ctx.Done():
			logger.Debug("pollNewGames 循环退出")
			return
		case <-ticker.C:
			m.checkAllBindingsForNewGames()
		}
	}
}

// checkAllBindingsForNewGames 检查所有绑定是否有新游戏
func (m *Monitor) checkAllBindingsForNewGames() {
	bindings, err := m.service.GetRepository().GetAllActiveBindings()
	if err != nil {
		logger.Errorf("获取活跃绑定失败: %v", err)
		return
	}

	if len(bindings) == 0 {
		return
	}

	logger.Debugf("检查 %d 个绑定的游戏状态", len(bindings))

	for _, binding := range bindings {
		// 跳过已在追踪游戏的绑定
		if binding.CurrentGameID != nil {
			continue
		}

		// 检查是否开始新游戏
		if err := m.service.CheckAndCreateBets(m.ctx, &binding); err != nil {
			logger.Errorf("检查绑定 %s 失败: %v", binding.ID, err)
		}

		// 请求间隔，避免触发限流
		select {
		case <-m.ctx.Done():
			return
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// checkGameEnds 检查游戏是否结束
func (m *Monitor) checkGameEnds() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.checkEndInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			logger.Debug("checkGameEnds 循环退出")
			return
		case <-ticker.C:
			m.checkAllGamesForEnd()
		}
	}
}

// checkAllGamesForEnd 检查所有正在追踪的游戏是否结束
func (m *Monitor) checkAllGamesForEnd() {
	bindings, err := m.service.GetRepository().GetBindingsInGame()
	if err != nil {
		logger.Errorf("获取游戏中绑定失败: %v", err)
		return
	}

	if len(bindings) == 0 {
		return
	}

	logger.Debugf("检查 %d 个游戏是否结束", len(bindings))

	for _, binding := range bindings {
		if err := m.service.CheckAndSettleBets(m.ctx, &binding); err != nil {
			logger.Errorf("检查游戏结束失败 (绑定 %s): %v", binding.ID, err)
		}

		// 请求间隔
		select {
		case <-m.ctx.Done():
			return
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// SetPollInterval 设置轮询间隔（用于测试）
func (m *Monitor) SetPollInterval(d time.Duration) {
	m.pollInterval = d
}

// SetCheckEndInterval 设置检查结束间隔（用于测试）
func (m *Monitor) SetCheckEndInterval(d time.Duration) {
	m.checkEndInterval = d
}
