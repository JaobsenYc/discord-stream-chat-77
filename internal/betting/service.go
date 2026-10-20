package betting

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/database"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// Service 竞猜服务
type Service struct {
	betRepo    *database.BetRepository
	userRepo   *database.UserRepository
	calculator *Calculator
	session    *discordgo.Session
}

// NewService 创建竞猜服务
func NewService(db *database.DB, session *discordgo.Session) *Service {
	return &Service{
		betRepo:    database.NewBetRepository(db),
		userRepo:   database.NewUserRepository(db),
		calculator: NewCalculator(),
		session:    session,
	}
}

// CreateBetRequest 创建竞猜请求
type CreateBetRequest struct {
	Name             *string
	GuildID          string
	ChannelID        string
	CreatorID        uuid.UUID
	CreatorDiscordID string
	TeamAName        string
	TeamBName        string
}

// PlaceBetResult 下注结果
type PlaceBetResult struct {
	Success         bool   `json:"success"`
	Message         string `json:"message"`
	RemainingPoints int    `json:"remaining_points"`
	EstimatedPayout int    `json:"estimated_payout"`
}

// SettlementResult 结算结果
type SettlementResult struct {
	Success           bool                      `json:"success"`
	Message           string                    `json:"message"`
	WinnerTeam        string                    `json:"winner_team"`
	TotalPool         int                       `json:"total_pool"`
	Commission        int                       `json:"commission"`         // 抽水金额（奖池管理费）
	DistributablePool int                       `json:"distributable_pool"` // 可分配奖池
	WinnerCount       int                       `json:"winner_count"`
	LoserCount        int                       `json:"loser_count"`
	Winners           []database.BetParticipant `json:"winners"`
	Losers            []database.BetParticipant `json:"losers"`        // 失败者列表
	Payouts           map[uuid.UUID]int         `json:"payouts"`       // 每个获胜者的赔付金额
	SystemPayout      int                       `json:"system_payout"` // 系统账户获得/失去的积分
}

// CreateBet 创建竞猜
func (s *Service) CreateBet(req CreateBetRequest) (*database.Bet, error) {
	// 不再限制语音频道和用户创建数量，允许任意文字频道创建多个竞猜

	// 创建竞猜记录
	bet := &database.Bet{
		ID:                uuid.New(),
		Name:              req.Name,
		GuildID:           req.GuildID,
		ChannelID:         req.ChannelID,
		CreatorID:         req.CreatorID,
		CreatorDiscordID:  req.CreatorDiscordID,
		TeamAName:         req.TeamAName,
		TeamBName:         req.TeamBName,
		Status:            database.BetStatusBetting,
		TeamATotalBet:     0,
		TeamBTotalBet:     0,
		TotalParticipants: 0,
		CreatedAt:         time.Now(),
		ExpiresAt:         time.Now().Add(7 * 24 * time.Hour), // 7天后过期
		UpdatedAt:         time.Now(),
	}

	// 保存到数据库
	err := s.betRepo.Create(bet)
	if err != nil {
		return nil, fmt.Errorf("创建竞猜失败: %w", err)
	}

	logger.Infof("创建竞猜: %s (%s vs %s), 创建者: %s", bet.ID, bet.TeamAName, bet.TeamBName, req.CreatorDiscordID)
	return bet, nil
}

// PlaceBet 下注
func (s *Service) PlaceBet(betID, userID uuid.UUID, discordID, guildID, team string, amount int) (*PlaceBetResult, error) {
	// 获取竞猜信息
	bet, err := s.betRepo.GetByID(betID)
	if err != nil {
		return nil, fmt.Errorf("获取竞猜失败: %w", err)
	}
	if bet == nil {
		return &PlaceBetResult{
			Success: false,
			Message: "竞猜不存在",
		}, nil
	}

	// 检查竞猜状态
	if bet.Status != database.BetStatusBetting {
		return &PlaceBetResult{
			Success: false,
			Message: "竞猜已锁定，无法下注",
		}, nil
	}

	// 检查是否已过期
	if time.Now().After(bet.ExpiresAt) {
		return &PlaceBetResult{
			Success: false,
			Message: "竞猜已过期",
		}, nil
	}

	// 检查用户是否已下注
	hasParticipated, err := s.betRepo.HasUserParticipated(betID, userID)
	if err != nil {
		return nil, fmt.Errorf("检查参与状态失败: %w", err)
	}
	if hasParticipated {
		return &PlaceBetResult{
			Success: false,
			Message: "您已经下注过了",
		}, nil
	}

	// 验证队伍选择
	if team != database.TeamChoiceA && team != database.TeamChoiceB {
		return &PlaceBetResult{
			Success: false,
			Message: "无效的队伍选择",
		}, nil
	}

	// 验证下注金额
	if amount < database.MinBetAmount || amount > database.MaxBetAmount {
		return &PlaceBetResult{
			Success: false,
			Message: fmt.Sprintf("下注金额必须在%d-%d之间", database.MinBetAmount, database.MaxBetAmount),
		}, nil
	}

	// 检查用户积分
	userPoints, err := s.userRepo.GetUserPoints(userID, guildID)
	if err != nil {
		return nil, fmt.Errorf("获取用户积分失败: %w", err)
	}
	if userPoints.AvailablePoints < amount {
		return &PlaceBetResult{
			Success:         false,
			Message:         fmt.Sprintf("积分不足，您只有 %d 积分", userPoints.AvailablePoints),
			RemainingPoints: userPoints.AvailablePoints,
		}, nil
	}

	// 创建参与记录
	participant := &database.BetParticipant{
		ID:         uuid.New(),
		BetID:      betID,
		UserID:     userID,
		DiscordID:  discordID,
		TeamChoice: team,
		BetAmount:  amount,
		PlacedAt:   time.Now(),
	}

	// 保存参与记录
	err = s.betRepo.AddParticipant(participant)
	if err != nil {
		return nil, fmt.Errorf("添加参与记录失败: %w", err)
	}

	// 扣除积分
	err = s.userRepo.SpendPoints(
		userID, guildID, amount,
		"bet_participation",
		fmt.Sprintf("参与竞猜: %s vs %s", bet.TeamAName, bet.TeamBName),
		&betID, stringPtr("bet"),
	)
	if err != nil {
		return nil, fmt.Errorf("扣除积分失败: %w", err)
	}

	// 更新竞猜统计
	err = s.betRepo.IncrementBetStats(betID, team, amount)
	if err != nil {
		return nil, fmt.Errorf("更新竞猜统计失败: %w", err)
	}

	// 重新获取更新后的竞猜信息
	bet, err = s.betRepo.GetByID(betID)
	if err != nil {
		return nil, fmt.Errorf("获取竞猜失败: %w", err)
	}

	// 计算估算收益
	var teamTotal, opponentTotal int
	if team == database.TeamChoiceA {
		teamTotal = bet.TeamATotalBet
		opponentTotal = bet.TeamBTotalBet
	} else {
		teamTotal = bet.TeamBTotalBet
		opponentTotal = bet.TeamATotalBet
	}
	estimatedPayout := s.calculator.EstimatePayout(amount, teamTotal, opponentTotal)

	logger.Infof("用户 %s 下注 %d 积分给 %s队", discordID, amount, team)

	return &PlaceBetResult{
		Success:         true,
		Message:         "下注成功！",
		RemainingPoints: userPoints.AvailablePoints - amount,
		EstimatedPayout: estimatedPayout,
	}, nil
}

// LockBetting 锁定下注（开始比赛）
func (s *Service) LockBetting(betID, userID uuid.UUID) error {
	// 获取竞猜信息
	bet, err := s.betRepo.GetByID(betID)
	if err != nil {
		return fmt.Errorf("获取竞猜失败: %w", err)
	}
	if bet == nil {
		return fmt.Errorf("竞猜不存在")
	}

	// 验证是否为创建者
	if bet.CreatorID != userID {
		return fmt.Errorf("只有创建者可以锁定竞猜")
	}

	// 检查竞猜状态
	if bet.Status != database.BetStatusBetting {
		return fmt.Errorf("竞猜已锁定或已结束")
	}

	// 检查是否有参与者
	if bet.TotalParticipants == 0 {
		return fmt.Errorf("没有参与者，无法锁定")
	}

	// 系统对冲：当两边下注不平衡时，系统在较少的一边补齐差额的 80%
	// 例如：A队 2000，B队 100 → 差额 1900 × 0.8 = 1520，系统在B队下注 1520
	if bet.TeamATotalBet > bet.TeamBTotalBet {
		// A队下注更多，系统在B队对冲
		difference := bet.TeamATotalBet - bet.TeamBTotalBet
		hedgeAmount := int(float64(difference) * database.SystemHedgeRatio)
		if hedgeAmount > 0 {
			err = s.placeSystemHedgeBet(bet, database.TeamChoiceB, hedgeAmount)
			if err != nil {
				// 对冲失败不阻止锁定，只记录日志
				logger.Warnf("系统对冲失败: %v", err)
			} else {
				// 重新获取更新后的竞猜信息
				bet, err = s.betRepo.GetByID(betID)
				if err != nil {
					return fmt.Errorf("获取竞猜失败: %w", err)
				}
			}
		}
	} else if bet.TeamBTotalBet > bet.TeamATotalBet {
		// B队下注更多，系统在A队对冲
		difference := bet.TeamBTotalBet - bet.TeamATotalBet
		hedgeAmount := int(float64(difference) * database.SystemHedgeRatio)
		if hedgeAmount > 0 {
			err = s.placeSystemHedgeBet(bet, database.TeamChoiceA, hedgeAmount)
			if err != nil {
				// 对冲失败不阻止锁定，只记录日志
				logger.Warnf("系统对冲失败: %v", err)
			} else {
				// 重新获取更新后的竞猜信息
				bet, err = s.betRepo.GetByID(betID)
				if err != nil {
					return fmt.Errorf("获取竞猜失败: %w", err)
				}
			}
		}
	}

	// 检查两队是否都有下注
	if bet.TeamATotalBet == 0 || bet.TeamBTotalBet == 0 {
		return fmt.Errorf("两队都需要有下注才能锁定")
	}

	// 更新状态
	now := time.Now()
	bet.Status = database.BetStatusLocked
	bet.LockedAt = &now
	err = s.betRepo.Update(bet)
	if err != nil {
		return fmt.Errorf("更新竞猜状态失败: %w", err)
	}

	logger.Infof("竞猜 %s 已锁定，创建者: %s", betID, bet.CreatorDiscordID)
	return nil
}

// AutoLockResult 自动锁定结果
type AutoLockResult struct {
	Locked    bool   // 是否成功锁定
	Cancelled bool   // 是否取消
	Message   string // 结果消息
}

// AutoLockBetting 自动锁定下注（用于游戏开始后自动封盘）
// 与 LockBetting 不同，此方法会处理没有人下注或只有一边有人下注的情况
func (s *Service) AutoLockBetting(betID, userID uuid.UUID) (*AutoLockResult, error) {
	// 获取竞猜信息
	bet, err := s.betRepo.GetByID(betID)
	if err != nil {
		return nil, fmt.Errorf("获取竞猜失败: %w", err)
	}
	if bet == nil {
		return nil, fmt.Errorf("竞猜不存在")
	}

	// 验证是否为创建者
	if bet.CreatorID != userID {
		return nil, fmt.Errorf("只有创建者可以锁定竞猜")
	}

	// 检查竞猜状态
	if bet.Status != database.BetStatusBetting {
		return &AutoLockResult{
			Locked:  false,
			Message: "竞猜已锁定或已结束",
		}, nil
	}

	// 情况1: 没有任何人下注 -> 取消竞猜
	if bet.TotalParticipants == 0 {
		err = s.CancelBet(betID, userID)
		if err != nil {
			return nil, fmt.Errorf("取消竞猜失败: %w", err)
		}
		return &AutoLockResult{
			Cancelled: true,
			Message:   "无人下注，竞猜已取消",
		}, nil
	}

	// 情况2: 只有一边有人下注 -> 系统对冲 60%
	const oneSideHedgeRatio = 0.6 // 只有一边下注时的对冲比例
	if bet.TeamATotalBet == 0 && bet.TeamBTotalBet > 0 {
		// B队有下注，A队没有，系统在A队对冲 60%
		hedgeAmount := int(float64(bet.TeamBTotalBet) * oneSideHedgeRatio)
		if hedgeAmount > 0 {
			err = s.placeSystemHedgeBet(bet, database.TeamChoiceA, hedgeAmount)
			if err != nil {
				logger.Warnf("系统对冲失败: %v", err)
				// 对冲失败，取消竞猜并退款
				err = s.CancelBet(betID, userID)
				if err != nil {
					return nil, fmt.Errorf("取消竞猜失败: %w", err)
				}
				return &AutoLockResult{
					Cancelled: true,
					Message:   "系统对冲失败，竞猜已取消并退款",
				}, nil
			}
			bet, _ = s.betRepo.GetByID(betID)
		}
	} else if bet.TeamBTotalBet == 0 && bet.TeamATotalBet > 0 {
		// A队有下注，B队没有，系统在B队对冲 60%
		hedgeAmount := int(float64(bet.TeamATotalBet) * oneSideHedgeRatio)
		if hedgeAmount > 0 {
			err = s.placeSystemHedgeBet(bet, database.TeamChoiceB, hedgeAmount)
			if err != nil {
				logger.Warnf("系统对冲失败: %v", err)
				// 对冲失败，取消竞猜并退款
				err = s.CancelBet(betID, userID)
				if err != nil {
					return nil, fmt.Errorf("取消竞猜失败: %w", err)
				}
				return &AutoLockResult{
					Cancelled: true,
					Message:   "系统对冲失败，竞猜已取消并退款",
				}, nil
			}
			bet, _ = s.betRepo.GetByID(betID)
		}
	} else if bet.TeamATotalBet > 0 && bet.TeamBTotalBet > 0 {
		// 情况3: 两边都有下注但不平衡 -> 系统补齐差额的 80%
		if bet.TeamATotalBet > bet.TeamBTotalBet {
			difference := bet.TeamATotalBet - bet.TeamBTotalBet
			hedgeAmount := int(float64(difference) * database.SystemHedgeRatio)
			if hedgeAmount > 0 {
				err = s.placeSystemHedgeBet(bet, database.TeamChoiceB, hedgeAmount)
				if err != nil {
					logger.Warnf("系统对冲失败: %v", err)
				} else {
					bet, _ = s.betRepo.GetByID(betID)
				}
			}
		} else if bet.TeamBTotalBet > bet.TeamATotalBet {
			difference := bet.TeamBTotalBet - bet.TeamATotalBet
			hedgeAmount := int(float64(difference) * database.SystemHedgeRatio)
			if hedgeAmount > 0 {
				err = s.placeSystemHedgeBet(bet, database.TeamChoiceA, hedgeAmount)
				if err != nil {
					logger.Warnf("系统对冲失败: %v", err)
				} else {
					bet, _ = s.betRepo.GetByID(betID)
				}
			}
		}
	}

	// 更新状态为已锁定
	now := time.Now()
	bet.Status = database.BetStatusLocked
	bet.LockedAt = &now
	err = s.betRepo.Update(bet)
	if err != nil {
		return nil, fmt.Errorf("更新竞猜状态失败: %w", err)
	}

	logger.Infof("竞猜 %s 已自动锁定，创建者: %s", betID, bet.CreatorDiscordID)
	return &AutoLockResult{
		Locked:  true,
		Message: "竞猜已自动锁定",
	}, nil
}

// SettleBet 结算竞猜
func (s *Service) SettleBet(betID, settlerID uuid.UUID, winnerTeam string) (*SettlementResult, error) {
	// 获取竞猜信息
	bet, err := s.betRepo.GetByID(betID)
	if err != nil {
		return nil, fmt.Errorf("获取竞猜失败: %w", err)
	}
	if bet == nil {
		return nil, fmt.Errorf("竞猜不存在")
	}

	// 验证是否为创建者
	if bet.CreatorID != settlerID {
		return nil, fmt.Errorf("只有创建者可以结算竞猜")
	}

	// 检查竞猜状态
	if bet.Status != database.BetStatusLocked {
		return nil, fmt.Errorf("竞猜未锁定，无法结算")
	}

	// 验证获胜队伍
	if winnerTeam != database.TeamChoiceA && winnerTeam != database.TeamChoiceB {
		return nil, fmt.Errorf("无效的获胜队伍")
	}

	// 获取所有参与者
	winnerParticipants, err := s.betRepo.GetParticipantsByTeam(betID, winnerTeam)
	if err != nil {
		return nil, fmt.Errorf("获取获胜方参与者失败: %w", err)
	}

	var loserTeam string
	if winnerTeam == database.TeamChoiceA {
		loserTeam = database.TeamChoiceB
	} else {
		loserTeam = database.TeamChoiceA
	}
	loserParticipants, err := s.betRepo.GetParticipantsByTeam(betID, loserTeam)
	if err != nil {
		return nil, fmt.Errorf("获取失败方参与者失败: %w", err)
	}

	// 计算赔付
	payoutResult, err := s.calculator.CalculatePayouts(bet, winnerParticipants, loserParticipants, winnerTeam)
	if err != nil {
		return nil, fmt.Errorf("计算赔付失败: %w", err)
	}

	// 原子认领结算权：并发结算（双击/交互重试）时只有一方能通过，防止重复赔付
	claimed, err := s.betRepo.TryMarkSettled(betID, settlerID, winnerTeam)
	if err != nil {
		return nil, err
	}
	if !claimed {
		return nil, fmt.Errorf("竞猜未锁定，无法结算")
	}

	// 用于记录系统账户的赔付（正数表示系统赢，负数表示系统输）
	var systemPayout int

	// 发放积分给获胜者
	for _, winner := range winnerParticipants {
		payout := payoutResult.Payouts[winner.UserID]

		// 检查是否是系统账户
		if winner.DiscordID == database.SystemDiscordID {
			// 系统账户获胜，积分归入系统池
			systemPayout = payout - winner.BetAmount // 净收益
			err = s.userRepo.AddPoints(
				winner.UserID, bet.GuildID, payout,
				"system_hedge_win",
				fmt.Sprintf("系统对冲获胜: %s vs %s", bet.TeamAName, bet.TeamBName),
				&betID, stringPtr("bet"),
			)
			if err != nil {
				logger.Errorf("系统账户积分归还失败: %v", err)
			}
		} else {
			// 普通用户获胜
			err = s.userRepo.AddPoints(
				winner.UserID, bet.GuildID, payout,
				"bet_win",
				fmt.Sprintf("竞猜获胜: %s vs %s", bet.TeamAName, bet.TeamBName),
				&betID, stringPtr("bet"),
			)
			if err != nil {
				logger.Errorf("发放积分失败: %v", err)
				continue
			}
		}

		// 更新参与者赔付记录
		isWinner := true
		err = s.betRepo.UpdateParticipantPayout(winner.ID, payout, isWinner)
		if err != nil {
			logger.Errorf("更新参与者赔付失败: %v", err)
		}
	}

	// 标记失败者
	for _, loser := range loserParticipants {
		// 检查是否是系统账户输了
		if loser.DiscordID == database.SystemDiscordID {
			systemPayout = -loser.BetAmount // 系统输掉的金额
		}

		isWinner := false
		err = s.betRepo.UpdateParticipantPayout(loser.ID, 0, isWinner)
		if err != nil {
			logger.Errorf("更新参与者状态失败: %v", err)
		}
	}

	// 管理费进入系统账户
	if payoutResult.Commission > 0 {
		systemAccount, err := s.getOrCreateSystemAccount(bet.GuildID)
		if err != nil {
			logger.Errorf("获取系统账户失败: %v", err)
		} else {
			err = s.userRepo.AddPoints(
				systemAccount.UserID, bet.GuildID, payoutResult.Commission,
				"commission",
				fmt.Sprintf("竞猜管理费: %s vs %s", bet.TeamAName, bet.TeamBName),
				&betID, stringPtr("bet"),
			)
			if err != nil {
				logger.Errorf("管理费入账失败: %v", err)
			} else {
				logger.Infof("管理费 %d 积分已入账系统账户", payoutResult.Commission)
			}
		}
	}

	// 结算状态已在 TryMarkSettled 中原子写入，此处仅同步内存对象供返回值使用
	now := time.Now()
	bet.Status = database.BetStatusSettled
	bet.WinnerTeam = &winnerTeam
	bet.SettledBy = &settlerID
	bet.SettledAt = &now

	logger.Infof("竞猜 %s 已结算，获胜队伍: %s, 总奖池: %d, 抽水: %d, 实际分配: %d, 系统赔付: %d",
		betID, winnerTeam, payoutResult.TotalPool, payoutResult.Commission, payoutResult.DistributablePool, systemPayout)

	return &SettlementResult{
		Success:           true,
		Message:           "结算成功",
		WinnerTeam:        winnerTeam,
		TotalPool:         payoutResult.TotalPool,
		Commission:        payoutResult.Commission,
		DistributablePool: payoutResult.DistributablePool,
		WinnerCount:       payoutResult.WinnerCount,
		LoserCount:        payoutResult.LoserCount,
		Winners:           winnerParticipants,
		Losers:            loserParticipants,
		Payouts:           payoutResult.Payouts,
		SystemPayout:      systemPayout,
	}, nil
}

// CancelBet 取消竞猜
func (s *Service) CancelBet(betID, userID uuid.UUID) error {
	// 获取竞猜信息
	bet, err := s.betRepo.GetByID(betID)
	if err != nil {
		return fmt.Errorf("获取竞猜失败: %w", err)
	}
	if bet == nil {
		return fmt.Errorf("竞猜不存在")
	}

	// 验证是否为创建者
	if bet.CreatorID != userID {
		return fmt.Errorf("只有创建者可以取消竞猜")
	}

	// 检查竞猜状态
	if bet.Status == database.BetStatusSettled {
		return fmt.Errorf("竞猜已结算，无法取消")
	}
	if bet.Status == database.BetStatusCancelled {
		return fmt.Errorf("竞猜已取消")
	}

	// 取消竞猜并退还积分（使用事务）
	err = s.betRepo.CancelBet(bet)
	if err != nil {
		return fmt.Errorf("取消竞猜失败: %w", err)
	}

	logger.Infof("竞猜 %s 已取消，创建者: %s", betID, bet.CreatorDiscordID)
	return nil
}

// GetBet 获取竞猜信息
func (s *Service) GetBet(betID uuid.UUID) (*database.Bet, error) {
	return s.betRepo.GetByID(betID)
}

// GetActiveBets 获取活跃的竞猜
func (s *Service) GetActiveBets(guildID string) ([]database.Bet, error) {
	return s.betRepo.GetActiveBets(guildID)
}

// CheckExpiredBets 检查并处理过期的竞猜
func (s *Service) CheckExpiredBets(guildID string) error {
	expiredBets, err := s.betRepo.GetExpiredBets(guildID)
	if err != nil {
		return fmt.Errorf("获取过期竞猜失败: %w", err)
	}

	for _, bet := range expiredBets {
		// 自动取消并退款
		err = s.betRepo.CancelBet(&bet)
		if err != nil {
			logger.Errorf("自动取消过期竞猜失败 %s: %v", bet.ID, err)
			continue
		}

		logger.Infof("自动取消过期竞猜: %s", bet.ID)

		// 更新Discord消息（如果有）
		if bet.MessageID != nil && *bet.MessageID != "" {
			// TODO: 更新消息显示已过期
		}
	}

	return nil
}

// 辅助函数
func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// 系统账户固定 UUID（用于标识系统参与者）
// SystemUserID 系统对冲账户固定 ID(定义收敛在 database 包,避免两处漂移)
var SystemUserID = database.SystemUserID

// getOrCreateSystemAccount 获取或创建系统对冲账户
func (s *Service) getOrCreateSystemAccount(guildID string) (*database.UserPoints, error) {
	// 先尝试获取系统账户
	systemUser, err := s.userRepo.GetByDiscordID(database.SystemDiscordID)
	if err != nil {
		return nil, fmt.Errorf("查询系统账户失败: %w", err)
	}

	// 如果系统用户不存在，创建它
	if systemUser == nil {
		systemUser = &database.User{
			ID:        SystemUserID,
			DiscordID: database.SystemDiscordID,
			Username:  "系统对冲账户",
			Timezone:  "UTC",
			Language:  "zh-CN",
		}
		err = s.userRepo.Create(systemUser)
		if err != nil {
			return nil, fmt.Errorf("创建系统账户失败: %w", err)
		}
	}

	// 获取系统账户在该服务器的积分
	systemPoints, err := s.userRepo.GetUserPoints(systemUser.ID, guildID)
	if err != nil {
		return nil, fmt.Errorf("获取系统积分失败: %w", err)
	}

	// 如果是新创建的积分记录，初始化为 SystemInitialPool
	if systemPoints.TotalPoints == 0 && systemPoints.AvailablePoints == 0 {
		err = s.userRepo.AddPoints(
			systemUser.ID, guildID, database.SystemInitialPool,
			"system_init",
			"系统对冲账户初始化",
			nil, nil,
		)
		if err != nil {
			return nil, fmt.Errorf("初始化系统积分失败: %w", err)
		}
		// 重新获取更新后的积分
		systemPoints, err = s.userRepo.GetUserPoints(systemUser.ID, guildID)
		if err != nil {
			return nil, fmt.Errorf("获取系统积分失败: %w", err)
		}
	}

	return systemPoints, nil
}

// placeSystemHedgeBet 系统对冲下注
func (s *Service) placeSystemHedgeBet(bet *database.Bet, team string, amount int) error {
	// 获取系统账户
	systemPoints, err := s.getOrCreateSystemAccount(bet.GuildID)
	if err != nil {
		return err
	}

	// 检查系统余额是否足够
	if systemPoints.AvailablePoints < amount {
		return fmt.Errorf("系统账户余额不足（当前: %d, 需要: %d）", systemPoints.AvailablePoints, amount)
	}

	// 获取系统用户
	systemUser, _ := s.userRepo.GetByDiscordID(database.SystemDiscordID)

	// 创建系统参与记录
	participant := &database.BetParticipant{
		ID:         uuid.New(),
		BetID:      bet.ID,
		UserID:     systemUser.ID,
		DiscordID:  database.SystemDiscordID,
		TeamChoice: team,
		BetAmount:  amount,
		PlacedAt:   time.Now(),
	}

	// 保存参与记录
	err = s.betRepo.AddParticipant(participant)
	if err != nil {
		return fmt.Errorf("添加系统参与记录失败: %w", err)
	}

	// 扣除系统积分
	err = s.userRepo.SpendPoints(
		systemUser.ID, bet.GuildID, amount,
		"system_hedge",
		fmt.Sprintf("系统对冲下注: %s vs %s", bet.TeamAName, bet.TeamBName),
		&bet.ID, stringPtr("bet"),
	)
	if err != nil {
		return fmt.Errorf("扣除系统积分失败: %w", err)
	}

	// 更新竞猜统计
	err = s.betRepo.IncrementBetStats(bet.ID, team, amount)
	if err != nil {
		return fmt.Errorf("更新竞猜统计失败: %w", err)
	}

	logger.Infof("系统对冲下注: %d 积分 -> %s队 (竞猜: %s)", amount, team, bet.ID)
	return nil
}
