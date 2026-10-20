package betting

import (
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/database"
)

// 抽水比例常量
const (
	CommissionRate = 0.05 // 5% 奖池管理费（抽水）
)

// PayoutResult 赔付结果
type PayoutResult struct {
	TotalPool         int               // 原始总奖池
	Commission        int               // 抽水金额（奖池管理费）
	DistributablePool int               // 可分配奖池（扣除抽水后）
	WinnerCount       int               // 获胜者数量
	LoserCount        int               // 失败者数量
	Payouts           map[uuid.UUID]int // 用户ID -> 赔付金额（包含本金）
	WinnerTeam        string            // 获胜队伍
}

// Calculator 收益计算器
type Calculator struct{}

// NewCalculator 创建收益计算器
func NewCalculator() *Calculator {
	return &Calculator{}
}

// CalculatePayouts 计算赔付
//
// 算法说明：
// 1. 总奖池 = 获胜方本金 + 失败方全部下注
// 2. 抽水金额 = 总奖池 × 5%（奖池管理费）
// 3. 可分配奖池 = 总奖池 - 抽水金额
// 4. 每个获胜者的收益 = 可分配奖池 × (该用户下注额 / 获胜方总下注额)
//
// 示例：
// A队下注: 1000积分 (3人: 500, 300, 200)
// B队下注: 500积分 (2人: 300, 200)
// A队赢，总奖池 = 1500积分
// 抽水 = 1500 × 5% = 75积分
// 可分配奖池 = 1500 - 75 = 1425积分
//
// 分配:
// - 用户1（下注500）: 1425 × (500/1000) = 712积分
// - 用户2（下注300）: 1425 × (300/1000) = 428积分
// - 用户3（下注200）: 1425 × (200/1000) = 285积分
func (c *Calculator) CalculatePayouts(
	bet *database.Bet,
	winnerParticipants []database.BetParticipant,
	loserParticipants []database.BetParticipant,
	winnerTeam string,
) (*PayoutResult, error) {

	// 验证参数
	if bet == nil {
		return nil, fmt.Errorf("竞猜信息不能为空")
	}

	if len(winnerParticipants) == 0 {
		return nil, fmt.Errorf("获胜方没有参与者")
	}

	// 计算总下注额
	var winnerTotalBet, loserTotalBet int
	if winnerTeam == database.TeamChoiceA {
		winnerTotalBet = bet.TeamATotalBet
		loserTotalBet = bet.TeamBTotalBet
	} else if winnerTeam == database.TeamChoiceB {
		winnerTotalBet = bet.TeamBTotalBet
		loserTotalBet = bet.TeamATotalBet
	} else {
		return nil, fmt.Errorf("无效的获胜队伍: %s", winnerTeam)
	}

	// 总奖池 = 获胜方本金 + 失败方全部下注
	totalPool := winnerTotalBet + loserTotalBet

	// 如果没有失败方，每个人都拿回本金（不抽水）
	if loserTotalBet == 0 {
		payouts := make(map[uuid.UUID]int)
		for _, winner := range winnerParticipants {
			payouts[winner.UserID] = winner.BetAmount
		}
		return &PayoutResult{
			TotalPool:         totalPool,
			Commission:        0,
			DistributablePool: totalPool,
			WinnerCount:       len(winnerParticipants),
			LoserCount:        len(loserParticipants),
			Payouts:           payouts,
			WinnerTeam:        winnerTeam,
		}, nil
	}

	// 计算抽水金额（5% 奖池管理费）
	commission := int(math.Floor(float64(totalPool) * CommissionRate))
	distributablePool := totalPool - commission

	// 计算每个获胜者的赔付：先按比例向下取整，再把舍入余数从首位起每人 +1 分摊，
	// 保证每人赔付非负且总额精确（若把余差全塞给最后一人，人数多时可能算出负数赔付）
	payouts := make(map[uuid.UUID]int)
	totalPaidOut := 0

	for _, winner := range winnerParticipants {
		ratio := float64(winner.BetAmount) / float64(winnerTotalBet)
		payout := int(math.Floor(float64(distributablePool) * ratio))
		payouts[winner.UserID] = payout
		totalPaidOut += payout
	}

	remainder := distributablePool - totalPaidOut
	for i := 0; i < len(winnerParticipants) && remainder > 0; i++ {
		payouts[winnerParticipants[i].UserID]++
		totalPaidOut++
		remainder--
	}

	// 验证总额是否正确（防止舍入误差导致积分丢失或增多）
	if totalPaidOut != distributablePool {
		return nil, fmt.Errorf("赔付总额不匹配: 计算 %d, 应为 %d", totalPaidOut, distributablePool)
	}

	return &PayoutResult{
		TotalPool:         totalPool,
		Commission:        commission,
		DistributablePool: distributablePool,
		WinnerCount:       len(winnerParticipants),
		LoserCount:        len(loserParticipants),
		Payouts:           payouts,
		WinnerTeam:        winnerTeam,
	}, nil
}

// CalculateOdds 计算赔率（用于显示）
// 赔率 = (对方下注总额 + 本金) / 本金
func (c *Calculator) CalculateOdds(myBet, opponentBet int) float64 {
	if myBet == 0 {
		return 0
	}
	return float64(myBet+opponentBet) / float64(myBet)
}

// EstimatePayout 估算收益（用于显示）
// 返回用户下注后可能获得的积分
func (c *Calculator) EstimatePayout(userBet, teamTotalBet, opponentTotalBet int) int {
	if teamTotalBet == 0 {
		return 0
	}

	totalPool := teamTotalBet + opponentTotalBet
	ratio := float64(userBet) / float64(teamTotalBet)
	return int(math.Round(float64(totalPool) * ratio))
}

// FormatOdds 格式化赔率显示
func (c *Calculator) FormatOdds(odds float64) string {
	return fmt.Sprintf("1:%.2f", odds)
}

// CalculateROI 计算投资回报率
// ROI = (收益 - 本金) / 本金 * 100%
func (c *Calculator) CalculateROI(payout, betAmount int) float64 {
	if betAmount == 0 {
		return 0
	}
	return float64(payout-betAmount) / float64(betAmount) * 100
}
