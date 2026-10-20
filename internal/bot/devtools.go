package bot

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// validateChannels 验证配置的频道是否存在和可访问
func (b *Bot) validateChannels() error {
	logger.Info("开始验证配置的频道...")

	// 验证服务器是否可访问
	guild, err := b.session.Guild(b.config.GuildID)
	if err != nil {
		return fmt.Errorf("无法访问服务器 %s: %w", b.config.GuildID, err)
	}
	logger.Infof("成功连接到服务器: %s", guild.Name)

	// 验证默认分类
	if b.config.DefaultCategoryID != "" {
		if _, err := b.session.Channel(b.config.DefaultCategoryID); err != nil {
			logger.Warnf("默认分类 %s 不存在或无法访问: %v", b.config.DefaultCategoryID, err)
		} else {
			logger.Infof("默认分类验证成功: %s", b.config.DefaultCategoryID)
		}
	}

	// 验证频道映射
	for i, mapping := range b.config.ChannelMappings {
		logger.Infof("验证频道映射 %d: %s -> %s (%s)", i+1, mapping.TextChannelID, mapping.VoiceCategoryID, mapping.GameName)

		// 验证文字频道
		textChannel, err := b.session.Channel(mapping.TextChannelID)
		if err != nil {
			logger.Warnf("文字频道 %s 不存在或无法访问: %v", mapping.TextChannelID, err)
			continue
		}
		logger.Infof("文字频道验证成功: %s (#%s)", textChannel.Name, textChannel.Name)

		// 验证语音分类
		voiceCategory, err := b.session.Channel(mapping.VoiceCategoryID)
		if err != nil {
			logger.Warnf("语音分类 %s 不存在或无法访问: %v", mapping.VoiceCategoryID, err)
			continue
		}
		logger.Infof("语音分类验证成功: %s", voiceCategory.Name)
	}

	logger.Info("频道验证完成")
	return nil
}

// cleanupTextChannels 清空所有监听的文字频道消息(仅开发环境启动时使用)
func (b *Bot) cleanupTextChannels() error {
	logger.Info("开始清理所有文字频道消息...")

	// 获取所有监听的文字频道
	textChannels := b.config.GetAllMonitoredTextChannels()

	for _, channelID := range textChannels {
		logger.Infof("清理文字频道消息: %s", channelID)

		// 批量删除消息
		if err := b.bulkDeleteMessages(channelID); err != nil {
			logger.Warnf("清理文字频道 %s 消息失败: %v", channelID, err)
		}
	}

	logger.Info("所有文字频道消息清理完成")
	return nil
}

// bulkDeleteMessages 批量删除频道消息
func (b *Bot) bulkDeleteMessages(channelID string) error {
	// 先发送一条通知消息
	_, err := b.session.ChannelMessageSend(channelID, "🧹 开发环境启动：正在清理频道消息...")
	if err != nil {
		return fmt.Errorf("发送通知消息失败: %w", err)
	}

	// 获取最近的消息
	messages, err := b.session.ChannelMessages(channelID, 100, "", "", "")
	if err != nil {
		return fmt.Errorf("获取频道消息失败: %w", err)
	}

	// 如果没有消息，直接返回
	if len(messages) == 0 {
		return nil
	}

	// 收集消息ID
	messageIDs := make([]string, 0, len(messages))
	twoWeeksAgo := time.Now().Add(-14 * 24 * time.Hour)

	for _, msg := range messages {
		// 检查消息是否在两周内（Discord API限制）
		// 直接使用消息时间戳
		if msg.Timestamp.After(twoWeeksAgo) {
			messageIDs = append(messageIDs, msg.ID)
		}
	}

	// 批量删除消息
	if len(messageIDs) > 0 {
		err = b.session.ChannelMessagesBulkDelete(channelID, messageIDs)
		if err != nil {
			// 如果批量删除失败，尝试单条删除
			logger.Warnf("批量删除消息失败，尝试单条删除: %v", err)
			for _, msgID := range messageIDs {
				if err := b.session.ChannelMessageDelete(channelID, msgID); err != nil {
					logger.Warnf("删除消息 %s 失败: %v", msgID, err)
				}
				// 添加短暂延迟，避免触发Discord速率限制
				time.Sleep(200 * time.Millisecond)
			}
		}
	}

	// 发送清理完成消息
	_, err = b.session.ChannelMessageSend(channelID, "✅ 频道消息已清理完成！")
	if err != nil {
		logger.Warnf("发送清理完成消息失败: %v", err)
	}

	return nil
}

// cleanupVoiceChannels 清空所有监听的语音频道(仅开发环境启动时使用)
func (b *Bot) cleanupVoiceChannels() error {
	logger.Info("开始清理所有语音频道...")

	// 获取所有监听的语音分类ID
	voiceCategories := b.config.GetAllMonitoredVoiceChannels()

	for _, categoryID := range voiceCategories {
		logger.Infof("清理语音分类下的频道: %s", categoryID)

		// 清理分类下的语音频道
		if err := b.cleanupChannelsInCategory(categoryID); err != nil {
			logger.Warnf("清理语音分类 %s 下的频道失败: %v", categoryID, err)
		}
	}

	logger.Info("所有语音频道清理完成")
	return nil
}

// cleanupChannelsInCategory 清理指定分类下的所有语音频道
func (b *Bot) cleanupChannelsInCategory(categoryID string) error {
	// 获取服务器所有频道
	channels, err := b.session.GuildChannels(b.config.GuildID)
	if err != nil {
		return fmt.Errorf("获取频道列表失败: %w", err)
	}

	// 找到该分类下的所有语音频道
	var voiceChannelsToDelete []string
	for _, channel := range channels {
		// 检查是否是语音频道且属于指定分类
		if channel.Type == discordgo.ChannelTypeGuildVoice && channel.ParentID == categoryID {
			voiceChannelsToDelete = append(voiceChannelsToDelete, channel.ID)
		}
	}

	logger.Infof("在分类 %s 下找到 %d 个语音频道需要清理", categoryID, len(voiceChannelsToDelete))

	// 逐个删除语音频道
	for _, channelID := range voiceChannelsToDelete {
		channel, err := b.session.Channel(channelID)
		if err != nil {
			logger.Warnf("获取频道 %s 信息失败: %v", channelID, err)
			continue
		}

		logger.Infof("删除语音频道: %s (%s)", channel.Name, channelID)

		_, err = b.session.ChannelDelete(channelID)
		if err != nil {
			logger.Warnf("删除语音频道 %s 失败: %v", channelID, err)
		}

		// 添加短暂延迟，避免触发Discord速率限制
		time.Sleep(500 * time.Millisecond)
	}

	return nil
}
