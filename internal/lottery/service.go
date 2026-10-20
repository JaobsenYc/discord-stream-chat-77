package lottery

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/jaobsenyc/discord-lfg-bot-go/internal/achievement"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/database"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

// Service 抽奖服务
type Service struct {
	repo           *database.Repository
	achievementSvc *achievement.Service
	db             *database.DB
	session        *discordgo.Session
}

// NewService 创建抽奖服务
func NewService(db *database.DB, achievementSvc *achievement.Service) *Service {
	return &Service{
		repo:           database.NewRepository(db),
		achievementSvc: achievementSvc,
		db:             db,
	}
}

// NewServiceWithSession 创建带Discord会话的抽奖服务
func NewServiceWithSession(db *database.DB, achievementSvc *achievement.Service, session *discordgo.Session) *Service {
	return &Service{
		repo:           database.NewRepository(db),
		achievementSvc: achievementSvc,
		db:             db,
		session:        session,
	}
}

// CreateLotteryRequest 创建抽奖请求
type CreateLotteryRequest struct {
	Name                 string
	Description          *string
	GuildID              string
	ChannelID            string
	CreatorID            uuid.UUID
	MaxParticipants      int
	CostPoints           int
	PrizeDescription     string
	DurationMinutes      int
	WinnerCount          int // 新增：中奖人数
	RequiredAchievements []uuid.UUID
}

// ParticipationResult 参与结果
type ParticipationResult struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	UserPoints int    `json:"user_points"`
}

// LotteryResult 抽奖结果
type LotteryResult struct {
	Success           bool                          `json:"success"`
	Winners           []database.User               `json:"winners"`
	Participants      []database.LotteryParticipant `json:"participants"`
	TotalParticipants int                           `json:"total_participants"`
	WinnerCount       int                           `json:"winner_count"`
	Message           string                        `json:"message"`
}

// CreateLottery 创建抽奖
func (s *Service) CreateLottery(req CreateLotteryRequest) (*database.Lottery, error) {
	// 验证创建者是否存在
	_, err := s.repo.User.GetByID(req.CreatorID)
	if err != nil {
		return nil, fmt.Errorf("创建者不存在: %w", err)
	}

	// 创建抽奖记录
	lottery := &database.Lottery{
		ID:               uuid.New(),
		Name:             req.Name,
		Description:      req.Description,
		GuildID:          req.GuildID,
		ChannelID:        req.ChannelID,
		CreatedBy:        req.CreatorID,
		MaxParticipants:  req.MaxParticipants,
		CostPoints:       req.CostPoints,
		PrizeDescription: req.PrizeDescription,
		StartTime:        time.Now(),
		EndTime:          time.Now().Add(time.Duration(req.DurationMinutes) * time.Minute),
		Status:           database.LotteryStatusActive,
		WinnerCount:      req.WinnerCount,
		Winners:          database.LotteryWinners{}, // 初始化为空数组
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// 保存抽奖记录
	err = s.repo.Lottery.Create(lottery)
	if err != nil {
		return nil, fmt.Errorf("创建抽奖失败: %w", err)
	}

	// 添加成就要求
	if len(req.RequiredAchievements) > 0 {
		err = s.repo.Lottery.AddAchievementRequirements(lottery.ID, req.RequiredAchievements)
		if err != nil {
			return nil, fmt.Errorf("添加成就要求失败: %w", err)
		}
	}

	return lottery, nil
}

// ParticipateInLottery 参与抽奖
func (s *Service) ParticipateInLottery(lotteryID, userID uuid.UUID, discordID, guildID string) (*ParticipationResult, error) {
	// 获取抽奖信息
	lottery, err := s.repo.Lottery.GetByID(lotteryID)
	if err != nil {
		return &ParticipationResult{
			Success: false,
			Message: "抽奖不存在",
		}, nil
	}

	// 检查抽奖状态
	if lottery.Status != database.LotteryStatusActive {
		return &ParticipationResult{
			Success: false,
			Message: "抽奖已结束",
		}, nil
	}

	// 检查是否已过期
	if time.Now().After(lottery.EndTime) {
		return &ParticipationResult{
			Success: false,
			Message: "抽奖已过期",
		}, nil
	}

	// 检查用户是否已参与
	hasParticipated, err := s.repo.Lottery.HasUserParticipated(lotteryID, userID)
	if err != nil {
		return nil, fmt.Errorf("检查参与状态失败: %w", err)
	}
	if hasParticipated {
		return &ParticipationResult{
			Success: false,
			Message: "您已经参与过此抽奖",
		}, nil
	}

	// 检查参与人数限制
	if lottery.MaxParticipants > 0 {
		participantCount, err := s.repo.Lottery.GetParticipantCount(lotteryID)
		if err != nil {
			return nil, fmt.Errorf("获取参与人数失败: %w", err)
		}
		if participantCount >= lottery.MaxParticipants {
			return &ParticipationResult{
				Success: false,
				Message: "抽奖人数已满",
			}, nil
		}
	}

	// 检查成就要求
	if s.achievementSvc != nil {
		isEligible, missingAchievements, err := s.achievementSvc.CheckLotteryEligibility(userID, lotteryID)
		if err != nil {
			return nil, fmt.Errorf("检查成就要求失败: %w", err)
		}
		if !isEligible {
			message := "您还未达成参与条件，需要完成以下成就:\n"
			for _, achievement := range missingAchievements {
				message += fmt.Sprintf("• %s\n", achievement)
			}
			return &ParticipationResult{
				Success: false,
				Message: message,
			}, nil
		}
	}

	// 检查积分要求
	var userPoints *database.UserPoints
	if lottery.CostPoints > 0 {
		userPoints, err = s.repo.User.GetUserPoints(userID, guildID)
		if err != nil {
			return nil, fmt.Errorf("获取用户积分失败: %w", err)
		}
		if userPoints.AvailablePoints < lottery.CostPoints {
			return &ParticipationResult{
				Success:    false,
				Message:    fmt.Sprintf("积分不足，需要 %d 积分，您只有 %d 积分", lottery.CostPoints, userPoints.AvailablePoints),
				UserPoints: userPoints.AvailablePoints,
			}, nil
		}
	}

	// 执行参与逻辑
	participant := &database.LotteryParticipant{
		ID:             uuid.New(),
		LotteryID:      lotteryID,
		UserID:         userID,
		DiscordID:      discordID,
		PointsSpent:    lottery.CostPoints,
		ParticipatedAt: time.Now(),
	}

	err = s.repo.Lottery.AddParticipant(participant)
	if err != nil {
		return nil, fmt.Errorf("添加参与者失败: %w", err)
	}

	// 扣除积分
	if lottery.CostPoints > 0 {
		err = s.repo.User.SpendPoints(
			userID, guildID, lottery.CostPoints,
			database.TransactionTypeLotteryParticipation,
			fmt.Sprintf("参与抽奖: %s", lottery.Name),
			&lotteryID, stringPtr("lottery"),
		)
		if err != nil {
			return nil, fmt.Errorf("扣除积分失败: %w", err)
		}
		userPoints.AvailablePoints -= lottery.CostPoints
	}

	// 触发成就更新
	if s.achievementSvc != nil {
		err = s.achievementSvc.OnLotteryParticipation(userID, discordID, guildID)
		if err != nil {
			// 成就更新失败不影响抽奖参与
			fmt.Printf("更新抽奖参与成就失败: %v\n", err)
		}
	}

	var remainingPoints int
	if userPoints != nil {
		remainingPoints = userPoints.AvailablePoints
	}

	return &ParticipationResult{
		Success:    true,
		Message:    "成功参与抽奖！",
		UserPoints: remainingPoints,
	}, nil
}

// DrawLottery 执行开奖
func (s *Service) DrawLottery(lotteryID uuid.UUID, guildID string) (*LotteryResult, error) {
	// 获取抽奖信息
	lottery, err := s.repo.Lottery.GetByID(lotteryID)
	if err != nil {
		return nil, fmt.Errorf("抽奖不存在: %w", err)
	}

	// 原子认领开奖权：定时开奖与手动开奖并发触发时只有一方成功，防止重复开奖
	claimed, err := s.repo.Lottery.TryMarkEnded(lotteryID)
	if err != nil {
		return nil, fmt.Errorf("更新抽奖状态失败: %w", err)
	}
	if !claimed {
		return &LotteryResult{
			Success: false,
			Message: "抽奖已结束",
		}, nil
	}

	// 获取所有参与者
	participants, err := s.repo.Lottery.GetParticipants(lotteryID)
	if err != nil {
		return nil, fmt.Errorf("获取参与者失败: %w", err)
	}

	if len(participants) == 0 {
		// 没有参与者，取消抽奖
		err = s.repo.Lottery.UpdateStatus(lotteryID, database.LotteryStatusCancelled)
		if err != nil {
			return nil, fmt.Errorf("更新抽奖状态失败: %w", err)
		}
		return &LotteryResult{
			Success:           false,
			Participants:      participants,
			TotalParticipants: 0,
			Message:           "没有参与者，抽奖已取消",
		}, nil
	}

	// 随机选择获奖者下标（人数超过参与者时按参与者数封顶）
	winnerIndices := pickWinnerIndices(len(participants), lottery.WinnerCount)

	winners := make([]database.User, 0, len(winnerIndices))
	winnerIDs := make(map[uuid.UUID]bool) // 用于快速查找

	for _, idx := range winnerIndices {
		winnerParticipant := participants[idx]

		// 获取获奖者信息
		winner, err := s.repo.User.GetByID(winnerParticipant.UserID)
		if err != nil {
			return nil, fmt.Errorf("获取获奖者信息失败: %w", err)
		}

		winners = append(winners, *winner)
		winnerIDs[winner.ID] = true

		// 添加到Winners JSON数组
		lottery.Winners.AddWinner(winner.ID.String(), winner.DiscordID, winner.Username)
	}

	// 更新抽奖状态
	now := time.Now()
	lottery.Status = database.LotteryStatusEnded
	lottery.WinnerSelectedAt = &now
	lottery.UpdatedAt = now

	err = s.repo.Lottery.Update(lottery)
	if err != nil {
		return nil, fmt.Errorf("更新抽奖状态失败: %w", err)
	}

	// 触发中奖成就
	if s.achievementSvc != nil {
		// 为获奖者更新中奖成就
		for _, winner := range winners {
			err = s.achievementSvc.OnLotteryWin(winner.ID, winner.DiscordID, guildID, true)
			if err != nil {
				fmt.Printf("更新中奖成就失败: %v\n", err)
			}
		}

		// 为其他参与者更新未中奖状态（用于连胜重置）
		for _, participant := range participants {
			if !winnerIDs[participant.UserID] {
				err = s.achievementSvc.OnLotteryWin(participant.UserID, participant.DiscordID, guildID, false)
				if err != nil {
					fmt.Printf("更新未中奖状态失败: %v\n", err)
				}
			}
		}
	}

	// 构造获奖者名单消息
	var message string
	if len(winners) == 1 {
		message = fmt.Sprintf("恭喜 %s 获得奖品: %s", winners[0].Username, lottery.PrizeDescription)
	} else {
		winnerNames := make([]string, len(winners))
		for i, winner := range winners {
			winnerNames[i] = winner.Username
		}
		message = fmt.Sprintf("恭喜 %s 获得奖品: %s", strings.Join(winnerNames, ", "), lottery.PrizeDescription)
	}

	return &LotteryResult{
		Success:           true,
		Winners:           winners,
		Participants:      participants,
		TotalParticipants: len(participants),
		WinnerCount:       len(winners),
		Message:           message,
	}, nil
}

// GetActiveLotteries 获取活跃的抽奖
func (s *Service) GetActiveLotteries(guildID string) ([]database.Lottery, error) {
	return s.repo.Lottery.GetActiveByGuild(guildID)
}

// GetLotteryDetails 获取抽奖详情
func (s *Service) GetLotteryDetails(lotteryID uuid.UUID) (*LotteryDetailResponse, error) {
	lottery, err := s.repo.Lottery.GetByID(lotteryID)
	if err != nil {
		return nil, fmt.Errorf("获取抽奖信息失败: %w", err)
	}

	participants, err := s.repo.Lottery.GetParticipants(lotteryID)
	if err != nil {
		return nil, fmt.Errorf("获取参与者失败: %w", err)
	}

	requirements, err := s.repo.Lottery.GetAchievementRequirements(lotteryID)
	if err != nil {
		return nil, fmt.Errorf("获取成就要求失败: %w", err)
	}

	// 获取获奖者信息（从Winners JSON数组中获取第一个）
	var winner *database.User
	if len(lottery.Winners) > 0 {
		// 使用第一个获奖者作为主要获奖者（向后兼容）
		firstWinner := lottery.Winners[0]
		if firstWinnerUUID, err := uuid.Parse(firstWinner.UserID); err == nil {
			winner, err = s.repo.User.GetByID(firstWinnerUUID)
			if err != nil {
				fmt.Printf("获取获奖者信息失败: %v\n", err)
			}
		}
	}

	return &LotteryDetailResponse{
		Lottery:                 lottery,
		Participants:            participants,
		ParticipantCount:        len(participants),
		AchievementRequirements: requirements,
		Winner:                  winner,
		TimeRemaining:           time.Until(lottery.EndTime),
		HasEnded:                lottery.Status != database.LotteryStatusActive || time.Now().After(lottery.EndTime),
	}, nil
}

// CheckExpiredLotteries 检查并处理过期的抽奖
func (s *Service) CheckExpiredLotteries(guildID string) error {
	expiredLotteries, err := s.repo.Lottery.GetExpiredLotteries(guildID)
	if err != nil {
		return fmt.Errorf("获取过期抽奖失败: %w", err)
	}

	for _, lottery := range expiredLotteries {
		// 自动开奖
		result, err := s.DrawLottery(lottery.ID, guildID)
		if err != nil {
			fmt.Printf("自动开奖失败 %s: %v\n", lottery.Name, err)
			continue
		}

		fmt.Printf("自动开奖完成: %s, 结果: %s\n", lottery.Name, result.Message)
	}

	return nil
}

// LotteryDetailResponse 抽奖详情响应
type LotteryDetailResponse struct {
	Lottery                 *database.Lottery                        `json:"lottery"`
	Participants            []database.LotteryParticipant            `json:"participants"`
	ParticipantCount        int                                      `json:"participant_count"`
	AchievementRequirements []database.LotteryAchievementRequirement `json:"achievement_requirements"`
	Winner                  *database.User                           `json:"winner,omitempty"`
	TimeRemaining           time.Duration                            `json:"time_remaining"`
	HasEnded                bool                                     `json:"has_ended"`
}

// FormatLotteryList 格式化抽奖列表为Discord嵌入消息
func (s *Service) FormatLotteryList(lotteries []database.Lottery) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:       "🎲 当前抽奖活动",
		Color:       0x9b59b6, // 紫色
		Description: "参与抽奖赢取丰厚奖励！",
	}

	if len(lotteries) == 0 {
		embed.Description = "暂无进行中的抽奖活动"
		return embed
	}

	var fieldValue string
	for i, lottery := range lotteries {
		if i >= 10 { // 限制显示数量
			break
		}

		timeRemaining := time.Until(lottery.EndTime)
		timeStr := formatDuration(timeRemaining)

		fieldValue += fmt.Sprintf("**%s**\n", lottery.Name)
		fieldValue += fmt.Sprintf("🎁 奖品: %s\n", lottery.PrizeDescription)

		if lottery.CostPoints > 0 {
			fieldValue += fmt.Sprintf("💰 参与费用: %d 积分\n", lottery.CostPoints)
		} else {
			fieldValue += "💰 参与费用: 免费\n"
		}

		if lottery.MaxParticipants > 0 {
			participantCount, _ := s.repo.Lottery.GetParticipantCount(lottery.ID)
			fieldValue += fmt.Sprintf("👥 参与人数: %d/%d\n", participantCount, lottery.MaxParticipants)
		}

		fieldValue += fmt.Sprintf("⏰ 剩余时间: %s\n", timeStr)
		fieldValue += fmt.Sprintf("🆔 ID: `%s`\n\n", lottery.ID.String()[:8])
	}

	embed.Fields = []*discordgo.MessageEmbedField{
		{
			Name:   "📋 抽奖列表",
			Value:  fieldValue,
			Inline: false,
		},
	}

	embed.Footer = &discordgo.MessageEmbedFooter{
		Text: "使用 /lottery join <ID> 参与抽奖",
	}

	return embed
}

// FormatLotteryDetails 格式化抽奖详情为Discord嵌入消息
func (s *Service) FormatLotteryDetails(details *LotteryDetailResponse) *discordgo.MessageEmbed {
	lottery := details.Lottery

	var color int
	switch lottery.Status {
	case database.LotteryStatusActive:
		color = 0x2ecc71 // 绿色
	case database.LotteryStatusEnded:
		color = 0xe74c3c // 红色
	case database.LotteryStatusCancelled:
		color = 0x95a5a6 // 灰色
	default:
		color = 0x3498db // 蓝色
	}

	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("🎲 %s", lottery.Name),
		Color: color,
	}

	// 基本信息
	description := fmt.Sprintf("🎁 **奖品**: %s\n", lottery.PrizeDescription)
	if lottery.Description != nil && *lottery.Description != "" {
		description += fmt.Sprintf("📝 **描述**: %s\n", *lottery.Description)
	}
	description += fmt.Sprintf("📅 **开始时间**: %s\n", lottery.StartTime.Format("2006-01-02 15:04:05"))
	description += fmt.Sprintf("📅 **结束时间**: %s\n", lottery.EndTime.Format("2006-01-02 15:04:05"))

	if lottery.CostPoints > 0 {
		description += fmt.Sprintf("💰 **参与费用**: %d 积分\n", lottery.CostPoints)
	} else {
		description += "💰 **参与费用**: 免费\n"
	}

	if lottery.MaxParticipants > 0 {
		description += fmt.Sprintf("👥 **人数限制**: %d/%d\n", details.ParticipantCount, lottery.MaxParticipants)
	} else {
		description += fmt.Sprintf("👥 **参与人数**: %d (无限制)\n", details.ParticipantCount)
	}

	// 状态信息
	switch lottery.Status {
	case database.LotteryStatusActive:
		if details.HasEnded {
			description += "⏰ **状态**: 已到期，等待开奖\n"
		} else {
			timeRemaining := formatDuration(details.TimeRemaining)
			description += fmt.Sprintf("⏰ **剩余时间**: %s\n", timeRemaining)
		}
	case database.LotteryStatusEnded:
		if details.Winner != nil {
			description += fmt.Sprintf("🏆 **获奖者**: %s\n", details.Winner.Username)
			if lottery.WinnerSelectedAt != nil {
				description += fmt.Sprintf("📅 **开奖时间**: %s\n", lottery.WinnerSelectedAt.Format("2006-01-02 15:04:05"))
			}
		}
	case database.LotteryStatusCancelled:
		description += "❌ **状态**: 已取消\n"
	}

	embed.Description = description

	// 成就要求
	if len(details.AchievementRequirements) > 0 {
		var achievementTexts []string
		achievementRepo := database.NewAchievementRepository(s.db)

		// 一次性查询所有成就，提高效率
		achievements, err := achievementRepo.GetAllAchievements()
		if err != nil {
			// 如果查询失败，回退到显示ID
			for _, req := range details.AchievementRequirements {
				achievementTexts = append(achievementTexts, fmt.Sprintf("成就ID: %s", req.AchievementID.String()[:8]))
			}
		} else {
			// 创建成就ID到成就对象的映射
			achievementMap := make(map[string]database.Achievement)
			for _, achievement := range achievements {
				achievementMap[achievement.ID.String()] = achievement
			}

			// 查找每个要求的成就
			for _, req := range details.AchievementRequirements {
				if foundAchievement, exists := achievementMap[req.AchievementID.String()]; exists {
					// 显示成就名称、描述和目标值
					if foundAchievement.TargetValue > 0 {
						achievementTexts = append(achievementTexts, fmt.Sprintf("**%s** (需要达到%d)\n%s",
							foundAchievement.Name, foundAchievement.TargetValue, foundAchievement.Description))
					} else {
						achievementTexts = append(achievementTexts, fmt.Sprintf("**%s**\n%s",
							foundAchievement.Name, foundAchievement.Description))
					}
				} else {
					// 成就未找到，显示ID
					achievementTexts = append(achievementTexts, fmt.Sprintf("成就ID: %s", req.AchievementID.String()[:8]))
				}
			}
		}

		achievementField := "需要完成以下成就才能参与:\n\n"
		for i, text := range achievementTexts {
			achievementField += fmt.Sprintf("%d. %s\n", i+1, text)
			if i < len(achievementTexts)-1 {
				achievementField += "\n"
			}
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "🏅 参与条件",
			Value:  achievementField,
			Inline: false,
		})
	}

	// 参与者列表（如果不是太多）
	if details.ParticipantCount > 0 && details.ParticipantCount <= 20 {
		participantField := ""
		for i, participant := range details.Participants {
			if i >= 20 { // 限制显示数量
				participantField += "...\n"
				break
			}
			participantField += fmt.Sprintf("• <@%s>\n", participant.DiscordID)
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "👥 参与者",
			Value:  participantField,
			Inline: true,
		})
	}

	return embed
}

// FormatLotteryResult 格式化抽奖结果为Discord嵌入消息
func (s *Service) FormatLotteryResult(result *LotteryResult, lottery *database.Lottery) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:       "🎉 抽奖结果",
		Description: result.Message,
		Color:       0xf39c12, // 橙色
	}

	if result.Success && len(result.Winners) > 0 {
		embed.Color = 0x2ecc71 // 绿色

		// 构建获奖者字段
		var winnerValue string
		if len(result.Winners) == 1 {
			winnerValue = fmt.Sprintf("<@%s>", result.Winners[0].DiscordID)
		} else {
			winnerMentions := make([]string, len(result.Winners))
			for i, winner := range result.Winners {
				winnerMentions[i] = fmt.Sprintf("<@%s>", winner.DiscordID)
			}
			winnerValue = strings.Join(winnerMentions, "\n")
		}

		embed.Fields = []*discordgo.MessageEmbedField{
			{
				Name:   fmt.Sprintf("🏆 获奖者 (%d人)", len(result.Winners)),
				Value:  winnerValue,
				Inline: false, // 多人时不内联显示
			},
			{
				Name:   "🎁 奖品",
				Value:  lottery.PrizeDescription,
				Inline: true,
			},
			{
				Name:   "👥 总参与人数",
				Value:  fmt.Sprintf("%d", result.TotalParticipants),
				Inline: true,
			},
		}
	}

	return embed
}

// 辅助函数

// formatDuration 格式化时间间隔
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "已结束"
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 24 {
		days := hours / 24
		hours = hours % 24
		return fmt.Sprintf("%d天%d小时%d分钟", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%d小时%d分钟", hours, minutes)
	} else {
		return fmt.Sprintf("%d分钟", minutes)
	}
}

// stringPtr 返回字符串指针
func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// SendLotteryAnnouncement 发送抽奖公告到指定频道
func (s *Service) SendLotteryAnnouncement(lottery *database.Lottery, announcementChannelID string) error {
	if s.session == nil {
		return fmt.Errorf("Discord会话未设置")
	}

	// 获取成就要求
	requirements, err := s.repo.Lottery.GetAchievementRequirements(lottery.ID)
	if err != nil {
		return fmt.Errorf("获取成就要求失败: %w", err)
	}

	// 构建嵌入消息
	embed := s.CreateLotteryAnnouncementEmbed(lottery, requirements)

	// 构建参与按钮
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					CustomID: fmt.Sprintf("lottery_join_%s", lottery.ID.String()),
					Label:    "🎲 参与抽奖",
					Style:    discordgo.PrimaryButton,
				},
				discordgo.Button{
					CustomID: fmt.Sprintf("lottery_info_%s", lottery.ID.String()),
					Label:    "📊 查看详情",
					Style:    discordgo.SecondaryButton,
				},
			},
		},
	}

	// 发送消息
	_, err = s.session.ChannelMessageSendComplex(announcementChannelID, &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: components,
	})

	if err != nil {
		return fmt.Errorf("发送抽奖公告失败: %w", err)
	}

	return nil
}

// CreateLotteryAnnouncementEmbed 创建抽奖公告嵌入消息
func (s *Service) CreateLotteryAnnouncementEmbed(lottery *database.Lottery, requirements []database.LotteryAchievementRequirement) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("🎉 %s", lottery.Name),
		Color: 0xf39c12, // 橙色
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "🎁 奖品",
				Value:  lottery.PrizeDescription,
				Inline: true,
			},
			{
				Name:   "🏆 中奖人数",
				Value:  fmt.Sprintf("%d人", lottery.WinnerCount),
				Inline: true,
			},
			{
				Name:   "⏰ 结束时间",
				Value:  fmt.Sprintf("<t:%d:F>", lottery.EndTime.Unix()),
				Inline: false,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("抽奖ID: %s", lottery.ID.String()[:8]),
		},
		Timestamp: lottery.CreatedAt.Format(time.RFC3339),
	}

	// 添加描述
	description := ""
	if lottery.Description != nil && *lottery.Description != "" {
		description = *lottery.Description + "\n\n"
	}

	// 添加参与条件
	var conditions []string

	if lottery.CostPoints > 0 {
		conditions = append(conditions, fmt.Sprintf("💰 参与费用: %d 积分", lottery.CostPoints))
	} else {
		conditions = append(conditions, "💰 参与费用: 免费")
	}

	if lottery.MaxParticipants > 0 {
		conditions = append(conditions, fmt.Sprintf("👥 人数限制: %d人", lottery.MaxParticipants))
	} else {
		conditions = append(conditions, "👥 人数限制: 无限制")
	}

	// 添加成就要求
	if len(requirements) > 0 {
		achievementTexts := []string{}
		achievementRepo := database.NewAchievementRepository(s.db)

		// 一次性查询所有成就，提高效率
		achievements, err := achievementRepo.GetAllAchievements()
		if err != nil {
			// 如果查询失败，回退到显示ID
			for _, req := range requirements {
				achievementTexts = append(achievementTexts, fmt.Sprintf("成就ID: %s", req.AchievementID.String()[:8]))
			}
		} else {
			// 创建成就ID到成就对象的映射
			achievementMap := make(map[string]database.Achievement)
			for _, achievement := range achievements {
				achievementMap[achievement.ID.String()] = achievement
			}

			// 查找每个要求的成就
			for _, req := range requirements {
				if foundAchievement, exists := achievementMap[req.AchievementID.String()]; exists {
					// 显示成就名称和目标值
					if foundAchievement.TargetValue > 0 {
						achievementTexts = append(achievementTexts, fmt.Sprintf("%s (需要达到%d)", foundAchievement.Name, foundAchievement.TargetValue))
					} else {
						achievementTexts = append(achievementTexts, foundAchievement.Name)
					}
				} else {
					// 成就未找到，显示ID
					achievementTexts = append(achievementTexts, fmt.Sprintf("成就ID: %s", req.AchievementID.String()[:8]))
				}
			}
		}
		conditions = append(conditions, fmt.Sprintf("🏅 成就要求: %s", strings.Join(achievementTexts, ", ")))
	}

	description += "**参与条件:**\n" + strings.Join(conditions, "\n")

	// 添加倒计时
	timeRemaining := time.Until(lottery.EndTime)
	if timeRemaining > 0 {
		description += fmt.Sprintf("\n\n⏱️ **剩余时间: %s**", formatDuration(timeRemaining))
	}

	embed.Description = description

	return embed
}

// HandleLotteryButtonInteraction 处理抽奖按钮交互
func (s *Service) HandleLotteryButtonInteraction(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	customID := interaction.MessageComponentData().CustomID

	// 解析按钮ID
	parts := strings.Split(customID, "_")
	if len(parts) < 3 {
		s.respondToButtonInteraction(session, interaction, "无效的按钮操作", true)
		return
	}

	action := parts[1] // join 或 info
	lotteryIDStr := parts[2]

	// 解析抽奖ID
	lotteryID, err := uuid.Parse(lotteryIDStr)
	if err != nil {
		s.respondToButtonInteraction(session, interaction, "无效的抽奖ID", true)
		return
	}

	switch action {
	case "join":
		s.handleJoinButtonClick(session, interaction, lotteryID)
	case "info":
		s.handleInfoButtonClick(session, interaction, lotteryID)
	default:
		s.respondToButtonInteraction(session, interaction, "未知的操作", true)
	}
}

// handleJoinButtonClick 处理参与按钮点击
func (s *Service) handleJoinButtonClick(session *discordgo.Session, interaction *discordgo.InteractionCreate, lotteryID uuid.UUID) {
	// 获取或创建用户
	user, err := s.repo.User.GetOrCreate(
		interaction.Member.User.ID,
		interaction.Member.User.Username,
		nil, nil,
	)
	if err != nil {
		s.respondToButtonInteraction(session, interaction, "用户信息获取失败", true)
		return
	}

	// 参与抽奖
	result, err := s.ParticipateInLottery(lotteryID, user.ID, interaction.Member.User.ID, interaction.GuildID)
	if err != nil {
		s.respondToButtonInteraction(session, interaction, fmt.Sprintf("参与失败: %v", err), true)
		return
	}

	// 响应结果
	s.respondToButtonInteraction(session, interaction, result.Message, !result.Success)
}

// handleInfoButtonClick 处理查看详情按钮点击
func (s *Service) handleInfoButtonClick(session *discordgo.Session, interaction *discordgo.InteractionCreate, lotteryID uuid.UUID) {
	// 获取抽奖详情
	details, err := s.GetLotteryDetails(lotteryID)
	if err != nil {
		s.respondToButtonInteraction(session, interaction, "获取抽奖详情失败", true)
		return
	}

	// 格式化详情并响应
	embed := s.FormatLotteryDetails(details)

	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	}

	err = session.InteractionRespond(interaction.Interaction, response)
	if err != nil {
		fmt.Printf("响应交互失败: %v\n", err)
	}
}

// respondToButtonInteraction 响应按钮交互
func (s *Service) respondToButtonInteraction(session *discordgo.Session, interaction *discordgo.InteractionCreate, message string, ephemeral bool) {
	var flags discordgo.MessageFlags
	if ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}

	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   flags,
		},
	}

	err := session.InteractionRespond(interaction.Interaction, response)
	if err != nil {
		fmt.Printf("响应交互失败: %v\n", err)
	}
}

// pickWinnerIndices 从 n 个参与者中随机选出至多 winnerCount 个不重复下标
// (Fisher-Yates 洗牌取前 N;Go 1.20+ 全局随机源自动播种)
func pickWinnerIndices(n, winnerCount int) []int {
	if winnerCount > n {
		winnerCount = n
	}
	if winnerCount < 0 {
		winnerCount = 0
	}

	indices := make([]int, n)
	for i := range indices {
		indices[i] = i
	}
	for i := n - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		indices[i], indices[j] = indices[j], indices[i]
	}
	return indices[:winnerCount]
}
