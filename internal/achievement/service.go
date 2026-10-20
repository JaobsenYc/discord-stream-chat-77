package achievement

import (
	"fmt"
	"log"
	"strconv"

	"github.com/jaobsenyc/discord-lfg-bot-go/internal/database"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

// Service 成就服务
type Service struct {
	repo *database.AchievementRepository
	db   *database.DB
}

// NewService 创建成就服务
func NewService(db *database.DB) *Service {
	return &Service{
		repo: database.NewAchievementRepository(db),
		db:   db,
	}
}

// ============== 事件处理器 ==============

// OnCheckIn 处理签到事件
func (s *Service) OnCheckIn(userID uuid.UUID, discordID, guildID string, consecutiveDays int) error {
	metadata := map[string]interface{}{
		"consecutive_days": consecutiveDays,
		"guild_id":         guildID,
	}

	achievements, err := s.repo.UpdateProgress(userID, database.AchievementTypeCheckIn, 1, metadata)
	if err != nil {
		return fmt.Errorf("failed to update checkin achievement progress: %w", err)
	}

	if len(achievements) > 0 {
		err = s.repo.ProcessRewards(userID, guildID, achievements)
		if err != nil {
			log.Printf("Failed to process achievement rewards for user %s: %v", discordID, err)
		}

		// 发送成就解锁通知
		s.sendAchievementNotifications(discordID, guildID, achievements)
	}

	return nil
}

// OnVoiceActivity 处理语音活动事件
func (s *Service) OnVoiceActivity(userID uuid.UUID, discordID, guildID string, durationSeconds int) error {
	metadata := map[string]interface{}{
		"guild_id": guildID,
	}

	achievements, err := s.repo.UpdateProgress(userID, database.AchievementTypeVoice, durationSeconds, metadata)
	if err != nil {
		return fmt.Errorf("failed to update voice achievement progress: %w", err)
	}

	if len(achievements) > 0 {
		err = s.repo.ProcessRewards(userID, guildID, achievements)
		if err != nil {
			log.Printf("Failed to process achievement rewards for user %s: %v", discordID, err)
		}

		s.sendAchievementNotifications(discordID, guildID, achievements)
	}

	return nil
}

// OnMessageSent 处理消息发送事件
func (s *Service) OnMessageSent(userID uuid.UUID, discordID, guildID string) error {
	metadata := map[string]interface{}{
		"guild_id": guildID,
	}

	achievements, err := s.repo.UpdateProgress(userID, database.AchievementTypeMessage, 1, metadata)
	if err != nil {
		return fmt.Errorf("failed to update message achievement progress: %w", err)
	}

	if len(achievements) > 0 {
		err = s.repo.ProcessRewards(userID, guildID, achievements)
		if err != nil {
			log.Printf("Failed to process achievement rewards for user %s: %v", discordID, err)
		}

		s.sendAchievementNotifications(discordID, guildID, achievements)
	}

	return nil
}

// OnLotteryParticipation 处理抽奖参与事件
func (s *Service) OnLotteryParticipation(userID uuid.UUID, discordID, guildID string) error {
	metadata := map[string]interface{}{
		"guild_id": guildID,
	}

	achievements, err := s.repo.UpdateProgress(userID, database.AchievementTypeLottery, 1, metadata)
	if err != nil {
		return fmt.Errorf("failed to update lottery achievement progress: %w", err)
	}

	if len(achievements) > 0 {
		err = s.repo.ProcessRewards(userID, guildID, achievements)
		if err != nil {
			log.Printf("Failed to process achievement rewards for user %s: %v", discordID, err)
		}

		s.sendAchievementNotifications(discordID, guildID, achievements)
	}

	return nil
}

// OnLotteryWin 处理抽奖中奖事件
func (s *Service) OnLotteryWin(userID uuid.UUID, discordID, guildID string, isWin bool) error {
	metadata := map[string]interface{}{
		"guild_id": guildID,
		"is_win":   isWin,
	}

	achievements, err := s.repo.UpdateProgress(userID, database.AchievementTypeLotteryWin, 1, metadata)
	if err != nil {
		return fmt.Errorf("failed to update lottery win achievement progress: %w", err)
	}

	if len(achievements) > 0 {
		err = s.repo.ProcessRewards(userID, guildID, achievements)
		if err != nil {
			log.Printf("Failed to process achievement rewards for user %s: %v", discordID, err)
		}

		s.sendAchievementNotifications(discordID, guildID, achievements)
	}

	return nil
}

// OnDailyActivity 处理每日活跃事件
func (s *Service) OnDailyActivity(userID uuid.UUID, discordID, guildID string) error {
	metadata := map[string]interface{}{
		"guild_id": guildID,
	}

	achievements, err := s.repo.UpdateProgress(userID, database.AchievementTypeActivity, 1, metadata)
	if err != nil {
		return fmt.Errorf("failed to update activity achievement progress: %w", err)
	}

	if len(achievements) > 0 {
		err = s.repo.ProcessRewards(userID, guildID, achievements)
		if err != nil {
			log.Printf("Failed to process achievement rewards for user %s: %v", discordID, err)
		}

		s.sendAchievementNotifications(discordID, guildID, achievements)
	}

	return nil
}

// OnTeamCreated 处理组队创建事件
func (s *Service) OnTeamCreated(userID uuid.UUID, discordID, guildID string, totalTeamsCreated int) error {
	metadata := map[string]interface{}{
		"guild_id":            guildID,
		"total_teams_created": totalTeamsCreated,
	}

	achievements, err := s.repo.UpdateProgress(userID, database.AchievementTypeTeamCreated, totalTeamsCreated, metadata)
	if err != nil {
		return fmt.Errorf("failed to update team created achievement progress: %w", err)
	}

	if len(achievements) > 0 {
		err = s.repo.ProcessRewards(userID, guildID, achievements)
		if err != nil {
			log.Printf("Failed to process achievement rewards for user %s: %v", discordID, err)
		}

		s.sendAchievementNotifications(discordID, guildID, achievements)
	}

	return nil
}

// ============== 查询功能 ==============

// GetUserAchievements 获取用户成就列表
func (s *Service) GetUserAchievements(userID uuid.UUID, category string) ([]database.AchievementWithProgress, error) {
	if category != "" {
		return s.repo.GetUserAchievementsByCategory(userID, category)
	}
	return s.repo.GetUserAchievements(userID)
}

// GetUserAchievementSummary 获取用户成就总览
func (s *Service) GetUserAchievementSummary(userID uuid.UUID) (*database.UserAchievementSummary, error) {
	return s.repo.GetUserAchievementSummary(userID)
}

// GetAchievementLeaderboard 获取成就排行榜
func (s *Service) GetAchievementLeaderboard(limit int) ([]database.AchievementLeaderboard, error) {
	return s.repo.GetAchievementLeaderboard(limit)
}

// CheckLotteryEligibility 检查抽奖资格
func (s *Service) CheckLotteryEligibility(userID uuid.UUID, lotteryID uuid.UUID) (bool, []string, error) {
	return s.repo.CheckLotteryEligibility(userID, lotteryID)
}

// ============== Discord 消息格式化 ==============

// FormatUserAchievements 格式化用户成就为Discord嵌入消息
func (s *Service) FormatUserAchievements(achievements []database.AchievementWithProgress, username string) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("🏅 %s 的成就进度", username),
		Color: 0x84c318, // 茶色主题
	}

	// 按分类分组
	categories := make(map[string][]database.AchievementWithProgress)
	for _, achievement := range achievements {
		categories[achievement.Category] = append(categories[achievement.Category], achievement)
	}

	// 为每个分类创建字段
	categoryNames := map[string]string{
		"checkin":  "🫖 签到成就",
		"voice":    "🎤 语音成就",
		"message":  "💬 社交成就",
		"lottery":  "🎲 抽奖成就",
		"team":     "🍵 组队成就",
		"activity": "☕ 活跃成就",
	}

	for category, categoryAchievements := range categories {
		categoryName, ok := categoryNames[category]
		if !ok {
			categoryName = fmt.Sprintf("📋 %s", category)
		}

		var fieldValue string
		for _, achievement := range categoryAchievements {
			icon := "🔒"
			status := ""
			progressBar := s.createProgressBar(achievement.ProgressPercent)

			if achievement.IsUnlocked {
				icon = "✅"
				status = "已完成"
			} else {
				status = fmt.Sprintf("%d/%d", achievement.UserProgress, achievement.TargetValue)
			}

			fieldValue += fmt.Sprintf("%s **%s %s**\n%s %s\n%s\n\n",
				*achievement.IconEmoji, achievement.Name, icon,
				progressBar, status,
				achievement.Description)
		}

		if fieldValue != "" {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   categoryName,
				Value:  fieldValue,
				Inline: false,
			})
		}
	}

	if len(embed.Fields) == 0 {
		embed.Description = "暂无成就记录，开始你的茶道之旅吧！"
	}

	embed.Footer = &discordgo.MessageEmbedFooter{
		Text: "继续努力，解锁更多成就获得奖励！",
	}

	return embed
}

// FormatAchievementSummary 格式化成就总览
func (s *Service) FormatAchievementSummary(summary *database.UserAchievementSummary) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("🏆 %s 的成就总览", summary.Username),
		Color: 0x84c318,
	}

	// 完成率进度条
	completionBar := s.createProgressBar(summary.CompletionRate)

	embed.Fields = []*discordgo.MessageEmbedField{
		{
			Name: "📊 完成统计",
			Value: fmt.Sprintf("%s %.1f%%\n已解锁: %d/%d 个成就",
				completionBar, summary.CompletionRate, summary.UnlockedCount, summary.TotalAchievements),
			Inline: true,
		},
		{
			Name:   "💰 成就奖励",
			Value:  fmt.Sprintf("累计获得: %d 积分", summary.TotalPoints),
			Inline: true,
		},
	}

	if summary.LastUnlocked != nil {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "🕒 最近解锁",
			Value:  fmt.Sprintf("<t:%d:R>", summary.LastUnlocked.Unix()),
			Inline: true,
		})
	}

	// 根据完成率添加评价
	var evaluation string
	switch {
	case summary.CompletionRate >= 90:
		evaluation = "🌟 茶道宗师！你已经达到了炉火纯青的境界！"
	case summary.CompletionRate >= 70:
		evaluation = "🍃 茶香四溢！你是真正的茶道爱好者！"
	case summary.CompletionRate >= 50:
		evaluation = "🫖 茶韵初显，继续努力修炼茶道！"
	case summary.CompletionRate >= 30:
		evaluation = "🌱 初窥茶道门径，前路还很长！"
	default:
		evaluation = "🔰 茶道新手，开始你的品茶之旅吧！"
	}

	embed.Description = evaluation

	return embed
}

// FormatAchievementLeaderboard 格式化成就排行榜
func (s *Service) FormatAchievementLeaderboard(leaderboard []database.AchievementLeaderboard) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:       "🏆 成就排行榜",
		Color:       0xffd700, // 金色
		Description: "茶道高手榜 - 看看谁是最厉害的茶道大师！",
	}

	var fieldValue string
	for _, entry := range leaderboard {
		rankIcon := s.getRankIcon(entry.Rank)
		fieldValue += fmt.Sprintf("%s **第%d名** %s\n", rankIcon, entry.Rank, entry.Username)
		fieldValue += fmt.Sprintf("   完成度: %.1f%% (%d个成就)\n", entry.CompletionRate, entry.UnlockedCount)
		fieldValue += fmt.Sprintf("   奖励积分: %d\n\n", entry.TotalPoints)
	}

	if fieldValue == "" {
		fieldValue = "暂无排行数据"
	}

	embed.Fields = []*discordgo.MessageEmbedField{
		{
			Name:   "🥇 排行榜",
			Value:  fieldValue,
			Inline: false,
		},
	}

	return embed
}

// FormatAchievementUnlocked 格式化成就解锁通知
func (s *Service) FormatAchievementUnlocked(achievements []database.Achievement) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title: "🎉 成就解锁！",
		Color: 0x00ff00, // 绿色
	}

	var description string
	var totalPoints int

	for _, achievement := range achievements {
		description += fmt.Sprintf("%s **%s**\n%s\n\n",
			*achievement.IconEmoji, achievement.Name, achievement.Description)

		if achievement.RewardType == database.RewardTypePoints {
			if points, err := strconv.Atoi(achievement.RewardValue); err == nil {
				totalPoints += points
			}
		}
	}

	embed.Description = description

	if totalPoints > 0 {
		embed.Footer = &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("恭喜获得 %d 积分奖励！", totalPoints),
		}
	}

	return embed
}

// ============== 辅助方法 ==============

// createProgressBar 创建进度条
func (s *Service) createProgressBar(percent float64) string {
	if percent > 100 {
		percent = 100
	}
	if percent < 0 {
		percent = 0
	}

	barLength := 12
	filled := int(percent / 100 * float64(barLength))

	bar := ""
	for i := 0; i < barLength; i++ {
		if i < filled {
			bar += "▓"
		} else {
			bar += "░"
		}
	}

	return bar
}

// getRankIcon 获取排名图标
func (s *Service) getRankIcon(rank int) string {
	switch rank {
	case 1:
		return "🥇"
	case 2:
		return "🥈"
	case 3:
		return "🥉"
	default:
		return "🏅"
	}
}

// sendAchievementNotifications 发送成就解锁通知
func (s *Service) sendAchievementNotifications(discordID, guildID string, achievements []database.Achievement) {
	// 这里可以实现发送Discord消息的逻辑
	// 由于需要Discord session，这个方法可能需要在bot层调用
	log.Printf("Achievement unlocked for user %s in guild %s: %d achievements", discordID, guildID, len(achievements))
	for _, achievement := range achievements {
		log.Printf("- %s: %s", achievement.Name, achievement.Description)
	}
}

// ============== 管理功能 ==============

// ProcessPendingRewards 处理待处理的奖励
func (s *Service) ProcessPendingRewards(session *discordgo.Session, guildID string) error {
	rewards, err := s.repo.GetUnprocessedRewards(guildID)
	if err != nil {
		return fmt.Errorf("failed to get unprocessed rewards: %w", err)
	}

	for _, reward := range rewards {
		var err error
		switch reward.RewardType {
		case database.RewardTypeRole:
			err = s.processRoleReward(session, guildID, reward)
		case database.RewardTypeBadge:
			err = s.processBadgeReward(session, guildID, reward)
			// Points rewards are processed automatically
		}

		if err != nil {
			log.Printf("Failed to process reward %s: %v", reward.ID, err)
			s.repo.MarkRewardProcessed(reward.ID, false, err.Error())
		} else {
			s.repo.MarkRewardProcessed(reward.ID, true, "")
		}
	}

	return nil
}

// processRoleReward 处理角色奖励
func (s *Service) processRoleReward(session *discordgo.Session, guildID string, reward database.AchievementReward) error {
	if reward.RoleID == nil {
		return fmt.Errorf("role ID is nil")
	}

	// 获取用户ID
	userRepo := database.NewUserRepository(s.db)
	user, err := userRepo.GetByID(reward.UserID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// 添加角色
	err = session.GuildMemberRoleAdd(guildID, user.DiscordID, *reward.RoleID)
	if err != nil {
		return fmt.Errorf("failed to add role to user: %w", err)
	}

	return nil
}

// processBadgeReward 处理徽章奖励
func (s *Service) processBadgeReward(session *discordgo.Session, guildID string, reward database.AchievementReward) error {
	// 徽章奖励的具体实现取决于你的系统设计
	// 这里可以是更新用户档案、发送特殊消息等
	log.Printf("Badge reward '%s' granted to user %s", *reward.BadgeName, reward.UserID)
	return nil
}
