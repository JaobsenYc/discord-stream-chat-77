package betting

import (
	"context"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// Monitor 竞猜监控器
type Monitor struct {
	service *Service
	session *discordgo.Session
	guildID string

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewMonitor 创建监控器
func NewMonitor(service *Service, session *discordgo.Session, guildID string) *Monitor {
	ctx, cancel := context.WithCancel(context.Background())

	return &Monitor{
		service: service,
		session: session,
		guildID: guildID,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start 启动监控
func (m *Monitor) Start() {
	// 启动定时检查过期竞猜（7天无人结算自动流盘）
	m.wg.Add(1)
	go m.checkExpiredBetsLoop()
}

// Stop 停止监控并等待后台任务退出
func (m *Monitor) Stop() {
	m.cancel()
	m.wg.Wait()
	logger.Info("竞猜监控器已停止")
}

// checkExpiredBetsLoop 定时检查过期竞猜
func (m *Monitor) checkExpiredBetsLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if err := m.service.CheckExpiredBets(m.guildID); err != nil {
				logger.Errorf("检查过期竞猜失败: %v", err)
			}
		}
	}
}
