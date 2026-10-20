package betting

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/database"
)

// makeBet 构造测试用竞猜
func makeBet(teamATotal, teamBTotal int) *database.Bet {
	return &database.Bet{
		ID:            uuid.New(),
		TeamATotalBet: teamATotal,
		TeamBTotalBet: teamBTotal,
	}
}

// makeParticipants 按下注额列表构造参与者
func makeParticipants(team string, amounts ...int) []database.BetParticipant {
	participants := make([]database.BetParticipant, 0, len(amounts))
	for _, amount := range amounts {
		participants = append(participants, database.BetParticipant{
			ID:         uuid.New(),
			UserID:     uuid.New(),
			TeamChoice: team,
			BetAmount:  amount,
		})
	}
	return participants
}

func TestCalculatePayouts_DocumentedExample(t *testing.T) {
	// 对应 CalculatePayouts 注释中的示例：A队1000（500/300/200），B队500，A胜
	calc := NewCalculator()
	winners := makeParticipants(database.TeamChoiceA, 500, 300, 200)
	losers := makeParticipants(database.TeamChoiceB, 300, 200)
	bet := makeBet(1000, 500)

	result, err := calc.CalculatePayouts(bet, winners, losers, database.TeamChoiceA)
	if err != nil {
		t.Fatalf("计算赔付失败: %v", err)
	}

	if result.TotalPool != 1500 {
		t.Errorf("总奖池错误: expected 1500, got %d", result.TotalPool)
	}
	if result.Commission != 75 {
		t.Errorf("抽水错误: expected 75, got %d", result.Commission)
	}
	if result.DistributablePool != 1425 {
		t.Errorf("可分配奖池错误: expected 1425, got %d", result.DistributablePool)
	}

	total := 0
	for _, winner := range winners {
		payout := result.Payouts[winner.UserID]
		if payout < 0 {
			t.Errorf("下注 %d 的获胜者赔付为负数: %d", winner.BetAmount, payout)
		}
		total += payout
	}
	if total != result.DistributablePool {
		t.Errorf("赔付总额与可分配奖池不符: expected %d, got %d", result.DistributablePool, total)
	}
}

func TestCalculatePayouts_ManySmallWinners_NoNegativePayout(t *testing.T) {
	// 回归用例：大量小额获胜者时，舍入余差不能把任何人的赔付算成负数
	calc := NewCalculator()

	amounts := make([]int, 30)
	for i := range amounts {
		amounts[i] = 3 // 30 人各下注 3
	}
	winners := makeParticipants(database.TeamChoiceA, amounts...)
	losers := makeParticipants(database.TeamChoiceB, 7)
	bet := makeBet(90, 7)

	result, err := calc.CalculatePayouts(bet, winners, losers, database.TeamChoiceA)
	if err != nil {
		t.Fatalf("计算赔付失败: %v", err)
	}

	total := 0
	for _, winner := range winners {
		payout := result.Payouts[winner.UserID]
		if payout < 0 {
			t.Fatalf("出现负数赔付: %d", payout)
		}
		total += payout
	}
	if total != result.DistributablePool {
		t.Errorf("赔付总额与可分配奖池不符: expected %d, got %d", result.DistributablePool, total)
	}
}

func TestCalculatePayouts_NoLosers_RefundOnly(t *testing.T) {
	// 单边下注：无失败方时全额退还本金、不抽水
	calc := NewCalculator()
	winners := makeParticipants(database.TeamChoiceA, 100, 200)
	bet := makeBet(300, 0)

	result, err := calc.CalculatePayouts(bet, winners, nil, database.TeamChoiceA)
	if err != nil {
		t.Fatalf("计算赔付失败: %v", err)
	}

	if result.Commission != 0 {
		t.Errorf("无失败方时不应抽水: got %d", result.Commission)
	}
	for _, winner := range winners {
		if result.Payouts[winner.UserID] != winner.BetAmount {
			t.Errorf("应退还本金 %d, got %d", winner.BetAmount, result.Payouts[winner.UserID])
		}
	}
}

func TestCalculatePayouts_InvalidInput(t *testing.T) {
	calc := NewCalculator()

	if _, err := calc.CalculatePayouts(nil, makeParticipants(database.TeamChoiceA, 100), nil, database.TeamChoiceA); err == nil {
		t.Error("bet 为空应返回错误")
	}
	if _, err := calc.CalculatePayouts(makeBet(100, 100), nil, nil, database.TeamChoiceA); err == nil {
		t.Error("获胜方无参与者应返回错误")
	}
	if _, err := calc.CalculatePayouts(makeBet(100, 100), makeParticipants(database.TeamChoiceA, 100), nil, "invalid"); err == nil {
		t.Error("无效获胜队伍应返回错误")
	}
}
