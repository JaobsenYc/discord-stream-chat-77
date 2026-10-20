package bot

import (
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// handleVoiceStateUpdate 处理语音状态更新事件
func (b *Bot) handleVoiceStateUpdate(s *discordgo.Session, vs *discordgo.VoiceStateUpdate) {
	// 语音活动跟踪
	b.trackVoiceActivity(vs)

	// 优先处理临时房间功能
	if b.config.TempRoomEnabled {
		b.tempRoomManager.HandleVoiceStateUpdate(vs)
	}

	// 通知频道管理器成员变化，用于空闲状态跟踪
	if vs.ChannelID != "" {
		// 有人加入频道
		b.channelManager.OnChannelMemberChange(vs.ChannelID)
	}
	if vs.BeforeUpdate != nil && vs.BeforeUpdate.ChannelID != "" && vs.BeforeUpdate.ChannelID != vs.ChannelID {
		// 有人离开原频道
		b.channelManager.OnChannelMemberChange(vs.BeforeUpdate.ChannelID)
	}

	// 检查是否是我们管理的频道（优先检查当前频道）
	teamInfo, exists := b.teamManager.GetTeamByChannel(vs.ChannelID)
	if !exists {
		// 如果当前不在LFG频道，检查是否是从LFG频道离开
		if vs.BeforeUpdate != nil && vs.BeforeUpdate.ChannelID != "" {
			if oldTeamInfo, exists := b.teamManager.GetTeamByChannel(vs.BeforeUpdate.ChannelID); exists {
				// 处理从LFG频道离开的情况
				if oldTeamInfo.HasPlayer(vs.UserID) {
					if err := oldTeamInfo.RemovePlayer(vs.UserID); err != nil {
						logger.Warnf("移除玩家失败: %v", err)
					} else {
						// 更新频道名称
						if err := b.channelManager.UpdateChannelName(b.ctx, oldTeamInfo.ChannelID, oldTeamInfo); err != nil {
							logger.Errorf("更新频道名称失败: %v", err)
						}
					}
				}
			}
		}
		return
	}

	// 获取用户信息
	member, err := s.GuildMember(b.config.GuildID, vs.UserID)
	if err != nil {
		logger.Errorf("获取用户信息失败: %v", err)
		return
	}

	// 处理用户加入频道
	if vs.ChannelID != "" && !teamInfo.HasPlayer(vs.UserID) {
		if err := teamInfo.AddPlayer(vs.UserID, member.User.Username); err != nil {
			logger.Warnf("添加玩家失败: %v", err)
			return
		}

		// 更新频道名称
		if err := b.channelManager.UpdateChannelName(b.ctx, vs.ChannelID, teamInfo); err != nil {
			logger.Errorf("更新频道名称失败: %v", err)
		}

		// 检查团队是否满员
		if teamInfo.IsFull() {
			// 发送满员通知
			if err := b.channelManager.SendTeamFullNotification(b.ctx, vs.ChannelID, teamInfo); err != nil {
				logger.Errorf("发送满员通知失败: %v", err)
			}
		}
	}

	// 处理用户离开频道 (vs.ChannelID 为空表示离开)
	if vs.ChannelID == "" && teamInfo.HasPlayer(vs.UserID) {
		if err := teamInfo.RemovePlayer(vs.UserID); err != nil {
			logger.Warnf("移除玩家失败: %v", err)
			return
		}

		// 更新频道名称
		if err := b.channelManager.UpdateChannelName(b.ctx, teamInfo.ChannelID, teamInfo); err != nil {
			logger.Errorf("更新频道名称失败: %v", err)
		}

		// 如果团队为空且是创建者离开，立即删除团队和频道
		if teamInfo.IsEmpty() && vs.UserID == teamInfo.CreatorID {
			logger.Infof("团队创建者离开，删除团队和频道: %s", teamInfo.Game)
			if err := b.channelManager.DeleteChannel(b.ctx, teamInfo.ChannelID); err != nil {
				logger.Errorf("删除频道失败: %v", err)
			}
			b.teamManager.RemoveTeam(teamInfo.ID)
		} else if teamInfo.IsEmpty() {
			// 非创建者离开导致团队为空，等待一段时间后删除
			go func() {
				// 用可取消的等待代替裸 Sleep，避免关机后仍对已关闭的会话操作
				select {
				case <-time.After(1 * time.Minute):
				case <-b.ctx.Done():
					return
				}
				// 重新检查团队是否还为空
				if currentTeam, exists := b.teamManager.GetTeam(teamInfo.ID); exists && currentTeam.IsEmpty() {
					logger.Infof("团队长时间为空，删除团队和频道: %s", currentTeam.Game)
					if err := b.channelManager.DeleteChannel(b.ctx, currentTeam.ChannelID); err != nil {
						logger.Errorf("删除空频道失败: %v", err)
					}
					b.teamManager.RemoveTeam(teamInfo.ID)
				}
			}()
		}
	}
}

// trackVoiceActivity 跟踪用户语音活动
func (b *Bot) trackVoiceActivity(vs *discordgo.VoiceStateUpdate) {
	if b.repository == nil {
		return // 没有数据库连接，跳过跟踪
	}

	// 获取用户信息
	member, err := b.session.GuildMember(b.config.GuildID, vs.UserID)
	if err != nil {
		logger.Errorf("获取用户信息失败: %v", err)
		return
	}

	// 确保用户在数据库中存在
	user, err := b.repository.User.GetOrCreate(vs.UserID, member.User.Username, nil, nil)
	if err != nil {
		logger.Errorf("创建或获取用户失败: %v", err)
		return
	}

	// 处理语音状态变化
	if vs.BeforeUpdate != nil {
		// 用户之前在语音频道中
		if vs.BeforeUpdate.ChannelID != "" {
			// 先获取活跃会话信息以计算时长
			activeSession, err := b.repository.Voice.GetActiveSession(user.ID, b.config.GuildID)
			if err != nil {
				logger.Errorf("获取活跃会话失败: %v", err)
			}

			// 用户离开了之前的频道
			err = b.repository.Voice.LeaveVoiceChannel(user.ID, b.config.GuildID)
			if err != nil {
				logger.Errorf("记录用户离开语音频道失败: %v", err)
			} else {
				logger.Debugf("用户 %s 离开语音频道: %s", member.User.Username, vs.BeforeUpdate.ChannelID)

				// 触发语音成就系统（异步）
				if activeSession != nil {
					go func() {
						// 计算会话时长（秒）
						duration := time.Since(activeSession.JoinedAt)
						durationSeconds := int(duration.Seconds())

						err = b.achievementService.OnVoiceActivity(user.ID, vs.UserID, b.config.GuildID, durationSeconds)
						if err != nil {
							logger.Errorf("更新语音成就失败: %v", err)
						}
					}()
				}
			}
		}
	}

	// 用户加入新的语音频道
	if vs.ChannelID != "" {
		// 获取频道信息
		channel, err := b.session.Channel(vs.ChannelID)
		if err != nil {
			logger.Errorf("获取频道信息失败: %v", err)
			return
		}

		// 记录用户加入语音频道
		_, err = b.repository.Voice.JoinVoiceChannel(
			user.ID,
			vs.UserID,
			b.config.GuildID,
			vs.ChannelID,
			channel.Name,
		)
		if err != nil {
			logger.Errorf("记录用户加入语音频道失败: %v", err)
		} else {
			logger.Debugf("用户 %s 加入语音频道: %s (%s)", member.User.Username, channel.Name, vs.ChannelID)
		}
	}
}
