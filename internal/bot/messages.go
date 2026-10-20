package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/parser"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/role"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/team"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// handleMessageCreate 处理消息创建事件
func (b *Bot) handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// 忽略机器人自己的消息
	if m.Author.ID == s.State.User.ID {
		return
	}

	// 忽略私信
	if m.GuildID != b.config.GuildID {
		logger.Debugf("忽略非目标服务器消息: %s != %s", m.GuildID, b.config.GuildID)
		return
	}

	logger.Debugf("收到消息: 用户=%s, 频道=%s, 内容=%s", m.Author.Username, m.ChannelID, m.Content)

	// 检查是否是被监听的频道
	if !b.config.IsMonitoredChannel(m.ChannelID) {
		logger.Debugf("频道 %s 不在监听列表中", m.ChannelID)
		return
	}

	logger.Debugf("频道 %s 在监听列表中，继续处理消息", m.ChannelID)

	content := strings.TrimSpace(m.Content)

	// 检查是否是帮助命令
	if strings.HasPrefix(content, b.config.BotPrefix+"help") || content == "!帮助" {
		b.sendHelpMessage(m.ChannelID)
		return
	}

	// 检查是否是状态命令
	if strings.HasPrefix(content, b.config.BotPrefix+"status") || content == "!状态" {
		b.sendStatusMessage(m.ChannelID)
		return
	}

	// 检查是否是临时房间状态命令
	if strings.HasPrefix(content, b.config.BotPrefix+"temproom") || content == "!临时房间" {
		b.sendTempRoomStatusMessage(m.ChannelID)
		return
	}

	// 触发消息成就系统（异步）
	if b.repository != nil && b.achievementService != nil {
		go func() {
			user, err := b.repository.User.GetOrCreate(m.Author.ID, m.Author.Username, nil, nil)
			if err == nil {
				err = b.achievementService.OnMessageSent(user.ID, m.Author.ID, m.GuildID)
				if err != nil {
					logger.Errorf("更新消息成就失败: %v", err)
				}
			}
		}()
	}

	// 尝试解析LFG消息
	lfgMsg, err := parser.Parse(content)
	if err != nil {
		// 不是LFG消息，忽略
		return
	}

	logger.Infof("收到LFG消息: %s (频道: %s)", content, m.ChannelID)

	// 处理LFG请求
	if err := b.handleLFGRequest(m, lfgMsg); err != nil {
		logger.Errorf("处理LFG请求失败: %v", err)
		b.sendErrorMessage(m.ChannelID, err.Error())
	}
}

// handleLegacyTextCommand 处理 ! 前缀的历史文本命令(角色管理等)
func (b *Bot) handleLegacyTextCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	// 本处理器只服务 ! 前缀的文本命令；普通聊天直接返回，
	// 避免对每条消息都发起 GuildMember REST 调用打满限流桶
	if !strings.HasPrefix(m.Content, "!") {
		return
	}

	// 创建角色管理器
	roleManager := role.NewManager(s, b.config.GuildID, b.config)

	// 检查是否是管理员
	isAdmin, err := roleManager.IsAdmin(m.Author.ID)
	if err != nil {
		logger.Errorf("检查管理员权限失败: %v", err)
		s.ChannelMessageSend(m.ChannelID, "❌ 检查权限失败，请稍后重试。")
		return
	}

	// 检查是否在允许的频道中
	isAdminChannel := roleManager.IsAdminChannel(m.ChannelID)

	// 处理创建角色命令
	if strings.HasPrefix(m.Content, "!create-role ") {
		if !isAdmin {
			s.ChannelMessageSend(m.ChannelID, "❌ 只有管理员才能创建角色！")
			return
		}

		if !isAdminChannel {
			s.ChannelMessageSend(m.ChannelID, "❌ 管理员命令只能在指定频道中使用！")
			return
		}

		// 解析命令参数
		parts := strings.Fields(m.Content)
		if len(parts) < 3 {
			s.ChannelMessageSend(m.ChannelID, "❌ 命令格式错误！正确格式：!create-role 角色名 颜色 [权限]")
			return
		}

		// 获取角色名和颜色
		roleName := parts[1]
		color := parts[2]

		// 设置默认权限为0
		permissions := int64(0)

		// 如果提供了权限参数，则解析它
		if len(parts) > 3 {
			permStr := parts[3]
			perm, err := strconv.ParseInt(permStr, 10, 64)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "❌ 权限值格式错误！")
				return
			}
			permissions = perm
		}

		// 创建角色
		_, err = roleManager.CreateRole(roleName, color, permissions)
		if err != nil {
			logger.Errorf("创建角色失败: %v", err)
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("❌ 创建角色失败：%v", err))
			return
		}

		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("✅ 角色创建成功：%s", roleName))
		return
	}

	// 处理删除角色命令
	if strings.HasPrefix(m.Content, "!delete-role ") {
		if !isAdmin {
			s.ChannelMessageSend(m.ChannelID, "❌ 只有管理员才能删除角色！")
			return
		}

		if !isAdminChannel {
			s.ChannelMessageSend(m.ChannelID, "❌ 管理员命令只能在指定频道中使用！")
			return
		}

		// 解析命令参数
		parts := strings.Fields(m.Content)
		if len(parts) != 2 {
			s.ChannelMessageSend(m.ChannelID, "❌ 命令格式错误！正确格式：!delete-role 角色名")
			return
		}

		roleName := parts[1]

		// 删除角色
		err := roleManager.DeleteRole(roleName)
		if err != nil {
			logger.Errorf("删除角色失败: %v", err)
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("❌ 删除角色失败：%v", err))
			return
		}

		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("✅ 角色删除成功：%s", roleName))
		return
	}

	// 旧的文本命令已迁移到slash命令
	// 使用 /channel cleanup 替代 !test-cleanup 和 !cleanup-channels
	// 使用 /channel status 替代 !teams-status
	if strings.HasPrefix(m.Content, "!test-cleanup") || strings.HasPrefix(m.Content, "!cleanup-channels") || strings.HasPrefix(m.Content, "!teams-status") {
		s.ChannelMessageSend(m.ChannelID, "ℹ️ 这些命令已迁移到slash命令！\n\n"+
			"🆕 **新的命令**:\n"+
			"• `/channel cleanup` - 清理空闲频道\n"+
			"• `/channel cleanup test_mode:true` - 测试清理（10秒超时）\n"+
			"• `/channel status` - 查看频道状态\n"+
			"• `/channel status detailed:true` - 查看详细状态")
		return
	}
}

// handleLFGRequest 处理LFG请求
func (b *Bot) handleLFGRequest(m *discordgo.MessageCreate, lfgMsg *parser.LFGMessage) error {
	userID := m.Author.ID
	textChannelID := m.ChannelID

	// 根据文字频道获取游戏信息和语音分类
	gameName := b.config.GetGameNameForTextChannel(textChannelID)
	voiceCategoryID := b.config.GetVoiceCategoryForTextChannel(textChannelID)

	logger.Debugf("处理LFG请求: 用户=%s, 游戏=%s, 人数=%d/%d, 分类=%s",
		m.Author.Username, gameName, lfgMsg.CurrentPlayers, lfgMsg.MaxPlayers, voiceCategoryID)

	// 检查用户是否已有频道记录
	existingChannelID, hasExistingChannel := b.teamManager.GetUserChannel(userID)

	var teamInfo *team.Team
	var channelID string
	var err error

	if hasExistingChannel {
		// 用户已有频道记录，尝试获取现有团队
		if existingTeam, exists := b.teamManager.GetTeamByChannel(existingChannelID); exists {
			logger.Infof("用户 %s 更新现有团队: %s -> %s", m.Author.Username, existingTeam.Game, gameName)

			// 更新现有团队信息
			teamInfo = existingTeam
			teamInfo.UpdateTeamInfo(gameName, lfgMsg.MaxPlayers)

			// 使用CreateOrUpdateVoiceChannel来处理频道更新
			channelID, err = b.channelManager.CreateOrUpdateVoiceChannel(b.ctx, teamInfo, existingChannelID, voiceCategoryID)
			if err != nil {
				logger.Errorf("更新语音频道失败: %v", err)
				// 清理失效记录，稍后重新创建
				b.teamManager.RemoveTeam(existingTeam.ID)
				hasExistingChannel = false
			} else {
				// 更新团队的频道ID（可能是新创建的）
				if err := b.teamManager.UpdateTeamChannel(teamInfo.ID, channelID); err != nil {
					logger.Errorf("更新团队频道失败: %v", err)
				}
			}
		} else {
			logger.Warnf("用户 %s 有频道记录但没有对应团队，清理记录", m.Author.Username)
			// 频道记录存在但没有对应团队，清理记录
			b.teamManager.RemoveUserChannel(userID)
			hasExistingChannel = false
		}
	}

	if !hasExistingChannel {
		// 生成新的团队ID
		teamID := fmt.Sprintf("%s_%d", userID, time.Now().Unix())

		// 创建新团队（先不设置频道ID）
		teamInfo = b.teamManager.CreateTeam(
			teamID,
			gameName,
			lfgMsg.MaxPlayers,
			userID,
			"", // 频道ID稍后设置
		)

		// 创建语音频道
		channelID, err = b.channelManager.CreateVoiceChannel(b.ctx, teamInfo, voiceCategoryID)
		if err != nil {
			b.teamManager.RemoveTeam(teamID)
			return fmt.Errorf("创建语音频道失败: %w", err)
		}

		// 更新团队的频道ID和用户频道映射
		if err := b.teamManager.UpdateTeamChannel(teamID, channelID); err != nil {
			logger.Errorf("更新团队频道失败: %v", err)
		}

		// 触发组队成就 - 异步处理
		if b.repository != nil && b.repository.User != nil && b.achievementService != nil {
			// 复制必要的依赖引用，避免在goroutine中访问可能变为nil的指针
			userRepo := b.repository.User
			achievementSvc := b.achievementService

			go func(discordUserID, guildID, username string) {
				logger.Debugf("开始处理用户 %s 的组队成就", discordUserID)

				// 获取或创建用户信息
				user, err := userRepo.GetOrCreate(discordUserID, username, nil, nil)
				if err != nil {
					logger.Errorf("获取或创建用户失败: %v", err)
					return
				}

				// 更新用户的组队创建统计
				err = userRepo.IncrementTeamsCreated(user.ID)
				if err != nil {
					logger.Errorf("更新用户组队创建统计失败: %v", err)
					return
				}

				// 重新获取用户信息，获取最新的 total_teams_created
				updatedUser, err := userRepo.GetByID(user.ID)
				if err != nil {
					logger.Errorf("重新获取用户信息失败: %v", err)
					return
				}

				// 触发组队成就检查
				err = achievementSvc.OnTeamCreated(updatedUser.ID, discordUserID, guildID, updatedUser.TotalTeamsCreated)
				if err != nil {
					logger.Errorf("触发组队成就失败: %v", err)
					return
				}

				logger.Debugf("用户 %s 组队成就已触发，当前总组队次数: %d", discordUserID, updatedUser.TotalTeamsCreated)
			}(userID, m.GuildID, m.Author.Username)
		}

		logger.Infof("用户 %s 创建语音频道: %s (游戏: %s)", m.Author.Username, channelID, gameName)
	}

	// 确保发起者在团队中
	if !teamInfo.HasPlayer(userID) {
		if err := teamInfo.AddPlayer(userID, m.Author.Username); err != nil {
			logger.Warnf("添加发起者到团队失败: %v", err)
		} else {
			logger.Infof("玩家 %s 加入团队 %s (%d/%d)", m.Author.Username, gameName, teamInfo.PlayerCount(), teamInfo.MaxPlayers)
		}
	} else {
		logger.Debugf("玩家 %s 已在团队 %s 中", m.Author.Username, gameName)
	}

	// 发送频道邀请
	logger.Debugf("准备发送频道邀请: 文字频道=%s, 语音频道=%s, 消息=%s", textChannelID, channelID, m.Content)
	if err := b.channelManager.SendChannelInvite(b.ctx, textChannelID, channelID, m.Content, teamInfo); err != nil {
		logger.Errorf("发送频道邀请失败: %v", err)
		// 即使邀请发送失败，也要继续完成处理
	} else {
		logger.Debugf("频道邀请发送成功")
	}

	logger.Infof("LFG请求处理完成: %s (频道: %s)", gameName, channelID)
	return nil
}

// sendHelpMessage 发送帮助消息
func (b *Bot) sendHelpMessage(channelID string) {
	// 获取当前频道对应的游戏名称
	gameName := b.config.GetGameNameForTextChannel(channelID)

	embed := &discordgo.MessageEmbed{
		Title:       "🎮 Discord LFG Bot 帮助",
		Description: fmt.Sprintf("**%s** 寻组机器人使用说明", gameName),
		Color:       0x0099ff,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "支持的格式",
				Value:  strings.Join(parser.SupportedExamples(), "\n• "),
				Inline: false,
			},
			{
				Name:   "使用说明",
				Value:  "• 在此频道发送人数格式消息即可自动创建语音频道\n• 机器人会自动识别游戏类型并在对应分类创建频道\n• 频道名称格式：游戏名-用户名",
				Inline: false,
			},
			{
				Name:   "斜杠命令",
				Value:  "• `/checkin daily` - 每日签到获取积分\n• `/achievement` - 查看个人成就\n• `/lottery create` - 创建抽奖\n• `/lottery join <ID>` - 参与抽奖",
				Inline: false,
			},
			{
				Name:   "其他命令",
				Value:  "• `!help` - 显示此帮助\n• `!status` - 查看当前状态\n• `!temproom` - 临时房间状态",
				Inline: false,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "发送人数格式的消息开始寻组",
		},
	}

	_, err := b.session.ChannelMessageSendEmbed(channelID, embed)
	if err != nil {
		logger.Errorf("发送帮助消息失败: %v", err)
	}
}

// sendStatusMessage 发送状态消息
func (b *Bot) sendStatusMessage(channelID string) {
	teams := b.teamManager.GetAllTeams()

	embed := &discordgo.MessageEmbed{
		Title: "🎮 当前寻组状态",
		Color: 0x00ff00,
	}

	if len(teams) == 0 {
		embed.Description = "当前没有进行中的寻组"
	} else {
		var description strings.Builder
		for _, team := range teams {
			description.WriteString(fmt.Sprintf("• %s\n", team.GetStatusString()))
		}
		embed.Description = description.String()
	}

	embed.Footer = &discordgo.MessageEmbedFooter{
		Text: fmt.Sprintf("总计 %d 个活跃团队", len(teams)),
	}

	_, err := b.session.ChannelMessageSendEmbed(channelID, embed)
	if err != nil {
		logger.Errorf("发送状态消息失败: %v", err)
	}
}

// sendErrorMessage 发送错误消息
func (b *Bot) sendErrorMessage(channelID, errorMsg string) {
	embed := &discordgo.MessageEmbed{
		Title:       "❌ 错误",
		Description: errorMsg,
		Color:       0xff0000,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "使用 !help 查看正确的使用方法",
		},
	}

	_, err := b.session.ChannelMessageSendEmbed(channelID, embed)
	if err != nil {
		logger.Errorf("发送错误消息失败: %v", err)
	}
}

// sendTempRoomStatusMessage 发送临时房间状态消息
func (b *Bot) sendTempRoomStatusMessage(channelID string) {
	if !b.config.TempRoomEnabled {
		_, err := b.session.ChannelMessageSend(channelID, "❌ 临时房间功能未启用")
		if err != nil {
			logger.Errorf("发送临时房间状态消息失败: %v", err)
		}
		return
	}

	stats := b.tempRoomManager.GetTempRoomStats()
	totalRooms := stats["total_rooms"].(int)

	embed := &discordgo.MessageEmbed{
		Title: "🏠 临时房间状态",
		Color: 0x9932cc,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "📊 当前临时房间数量",
				Value:  fmt.Sprintf("%d 个", totalRooms),
				Inline: true,
			},
			{
				Name:   "⚙️ 清理延迟",
				Value:  b.config.TempRoomCleanupDelay.String(),
				Inline: true,
			},
			{
				Name:   "🎯 触发频道",
				Value:  fmt.Sprintf("<#%s>", b.config.TempRoomTriggerChannel),
				Inline: true,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// 如果有活跃房间，显示详细信息
	if totalRooms > 0 {
		rooms := stats["rooms"].([]map[string]interface{})
		roomList := ""
		for i, room := range rooms {
			if i >= 5 { // 最多显示5个房间
				roomList += fmt.Sprintf("... 还有 %d 个房间\n", totalRooms-5)
				break
			}
			ownerName := room["owner_name"].(string)
			duration := room["duration"].(time.Duration)
			roomList += fmt.Sprintf("🎮 %s的房间 (运行 %v)\n", ownerName, duration.Round(time.Second))
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "🏠 活跃房间",
			Value:  roomList,
			Inline: false,
		})
	}

	_, err := b.session.ChannelMessageSendEmbed(channelID, embed)
	if err != nil {
		logger.Errorf("发送临时房间状态消息失败: %v", err)
	}
}
