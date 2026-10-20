package bot

import (
	"fmt"
	"strings"
	"time"

	"github.com/jaobsenyc/discord-lfg-bot-go/internal/lottery"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// startBackgroundTasks 启动 Bot 自身的后台周期任务,统一挂在 b.ctx / b.wg 上,
// Shutdown 时取消并等待全部退出
func (b *Bot) startBackgroundTasks() {
	b.wg.Add(3)
	go b.startCleanupTask()
	go b.startVoiceCleanupTask()
	go b.startLotteryExpiryTask()
}

// startCleanupTask 启动清理任务
func (b *Bot) startCleanupTask() {
	defer b.wg.Done()

	ticker := time.NewTicker(5 * time.Minute) // 每5分钟检查一次
	defer ticker.Stop()

	logger.Info("启动频道清理任务")

	for {
		select {
		case <-b.ctx.Done():
			logger.Info("停止频道清理任务")
			return
		case <-ticker.C:
			if err := b.channelManager.CleanupExpiredChannels(b.ctx, b.teamManager, b.config.ChannelCleanupTimeout); err != nil {
				logger.Errorf("清理过期频道失败: %v", err)
			}
		}
	}
}

// startVoiceCleanupTask 启动语音会话清理任务
func (b *Bot) startVoiceCleanupTask() {
	defer b.wg.Done()

	if b.repository == nil {
		return
	}

	ticker := time.NewTicker(1 * time.Hour) // 每小时清理一次
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			// 清理超过24小时的未结束会话
			if err := b.repository.Voice.CleanupInactiveSessions(24 * time.Hour); err != nil {
				logger.Errorf("清理语音会话失败: %v", err)
			} else {
				logger.Debug("语音会话清理完成")
			}
		}
	}
}

// startLotteryExpiryTask 启动抽奖过期检查任务
func (b *Bot) startLotteryExpiryTask() {
	defer b.wg.Done()

	if b.repository == nil {
		return
	}

	ticker := time.NewTicker(5 * time.Minute) // 每5分钟检查一次
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			b.processExpiredLotteries()
		}
	}
}

// processExpiredLotteries 检查并开奖全部过期抽奖
func (b *Bot) processExpiredLotteries() {
	expiredLotteries, err := b.repository.Lottery.GetExpiredLotteries(b.config.GuildID)
	if err != nil {
		logger.Errorf("获取过期抽奖失败: %v", err)
		return
	}

	for _, expiredLottery := range expiredLotteries {
		logger.Infof("处理过期抽奖: %s", expiredLottery.ID)

		// 创建抽奖服务实例进行开奖
		lotteryService := lottery.NewService(b.repository.DB(), b.achievementService)
		result, err := lotteryService.DrawLottery(expiredLottery.ID, expiredLottery.GuildID)
		if err != nil {
			logger.Errorf("抽奖 %s 开奖失败: %v", expiredLottery.ID, err)
			continue
		}

		// 发送开奖通知到原频道
		var message string
		if result.Success && len(result.Winners) > 0 {
			// 触发中奖成就（为所有获奖者）
			go func() {
				for _, winner := range result.Winners {
					err := b.achievementService.OnLotteryWin(winner.ID, winner.DiscordID, expiredLottery.GuildID, true)
					if err != nil {
						logger.Errorf("触发中奖成就失败: %v", err)
					}
				}
			}()

			// 构建获奖者消息
			if len(result.Winners) == 1 {
				winner := result.Winners[0]
				user, err := b.session.User(winner.DiscordID)
				if err != nil {
					logger.Warnf("获取中奖用户信息失败: %v", err)
					message = fmt.Sprintf("🎉 抽奖 `%s` 已结束！\n🏆 获奖者：<@%s>\n🎁 奖品：%s",
						expiredLottery.Name, winner.DiscordID, expiredLottery.PrizeDescription)
				} else {
					message = fmt.Sprintf("🎉 抽奖 `%s` 已结束！\n🏆 获奖者：%s (<@%s>)\n🎁 奖品：%s",
						expiredLottery.Name, user.Username, winner.DiscordID, expiredLottery.PrizeDescription)
				}
			} else {
				// 多人中奖
				winnerMentions := make([]string, len(result.Winners))
				for i, winner := range result.Winners {
					winnerMentions[i] = fmt.Sprintf("<@%s>", winner.DiscordID)
				}
				message = fmt.Sprintf("🎉 抽奖 `%s` 已结束！\n🏆 获奖者 (%d人)：%s\n🎁 奖品：%s",
					expiredLottery.Name, len(result.Winners), strings.Join(winnerMentions, " "), expiredLottery.PrizeDescription)
			}
		} else {
			message = fmt.Sprintf("😔 抽奖 `%s` 已结束，但没有有效参与者。\n抽奖已自动取消。", expiredLottery.Name)
		}

		// 发送通知消息
		_, err = b.session.ChannelMessageSend(expiredLottery.ChannelID, message)
		if err != nil {
			logger.Errorf("发送抽奖结果通知失败: %v", err)
		}
	}

	if len(expiredLotteries) > 0 {
		logger.Infof("处理了 %d 个过期抽奖", len(expiredLotteries))
	}
}
