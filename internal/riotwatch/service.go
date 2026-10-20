package riotwatch

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/betting"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/database"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// BetAnnouncer 发布竞猜面板消息。用小接口而非依赖 betting.CommandHandler
// 具体类型，避免业务层反向耦合表现层(由 betting.CommandHandler 实现)。
type BetAnnouncer interface {
	SendBetMessage(channelID string, bet *database.Bet) error
}

// Service Riot Watch 业务逻辑层
type Service struct {
	repo              *Repository
	riotClient        *RiotAPIClient
	bettingService    *betting.Service
	bettingCmdHandler BetAnnouncer
	session           *discordgo.Session
	db                *database.DB
}

// NewService 创建新的 Service
func NewService(
	db *database.DB,
	riotClient *RiotAPIClient,
	bettingService *betting.Service,
	bettingCmdHandler BetAnnouncer,
	session *discordgo.Session,
) *Service {
	return &Service{
		repo:              NewRepository(db),
		riotClient:        riotClient,
		bettingService:    bettingService,
		bettingCmdHandler: bettingCmdHandler,
		session:           session,
		db:                db,
	}
}

// AddBinding 添加新的频道-Riot账号绑定
func (s *Service) AddBinding(
	ctx context.Context,
	guildID, channelID string,
	creatorID uuid.UUID,
	creatorDiscordID string,
	riotID, riotTag string,
	deathThreshold float64,
) (*Binding, error) {
	// 1. 验证 Riot ID 是否存在
	account, err := s.riotClient.GetAccountByRiotID(ctx, riotID, riotTag)
	if err != nil {
		if IsNotFound(err) {
			return nil, fmt.Errorf("Riot ID 不存在: %s#%s", riotID, riotTag)
		}
		return nil, fmt.Errorf("验证 Riot ID 失败: %w", err)
	}

	// 2. 检查是否已存在相同绑定（包括已停用的）
	existing, err := s.repo.GetByChannelAndPUUIDIncludeInactive(guildID, channelID, account.PUUID)
	if err != nil {
		return nil, fmt.Errorf("检查绑定失败: %w", err)
	}

	// 如果存在已停用的绑定，重新激活它
	if existing != nil {
		if existing.IsActive {
			return nil, fmt.Errorf("该 Riot 账号已绑定到此频道")
		}
		// 重新激活已停用的绑定，同时更新玩家名称（可能改名了）和阈值
		if err := s.repo.ReactivateBinding(existing.ID, account.GameName, account.TagLine, deathThreshold); err != nil {
			return nil, fmt.Errorf("重新激活绑定失败: %w", err)
		}
		existing.IsActive = true
		existing.RiotID = account.GameName
		existing.RiotTag = account.TagLine
		existing.DeathThreshold = deathThreshold
		logger.Infof("已重新激活 Riot Watch 绑定: %s#%s -> 频道 %s (阈值: %g)", account.GameName, account.TagLine, channelID, deathThreshold)
		return existing, nil
	}

	// 3. 创建新绑定
	binding := &Binding{
		ID:                 uuid.New(),
		GuildID:            guildID,
		ChannelID:          channelID,
		CreatedBy:          creatorID,
		CreatedByDiscordID: creatorDiscordID,
		RiotID:             account.GameName, // 使用 API 返回的正确名称
		RiotTag:            account.TagLine,
		RiotPUUID:          &account.PUUID,
		DeathThreshold:     deathThreshold,
		IsActive:           true,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	if err := s.repo.Create(binding); err != nil {
		return nil, fmt.Errorf("创建绑定失败: %w", err)
	}

	logger.Infof("已添加 Riot Watch 绑定: %s#%s -> 频道 %s", riotID, riotTag, channelID)
	return binding, nil
}

// RemoveBinding 移除绑定
func (s *Service) RemoveBinding(guildID, channelID string, riotID, riotTag string) error {
	// 先通过 Riot ID 获取 PUUID
	ctx := context.Background()
	account, err := s.riotClient.GetAccountByRiotID(ctx, riotID, riotTag)
	if err != nil {
		return fmt.Errorf("无法验证 Riot ID: %w", err)
	}

	binding, err := s.repo.GetByChannelAndPUUID(guildID, channelID, account.PUUID)
	if err != nil {
		return fmt.Errorf("查找绑定失败: %w", err)
	}
	if binding == nil {
		return fmt.Errorf("未找到该绑定")
	}

	// 如果正在追踪游戏，先取消竞猜
	if binding.CurrentGameID != nil {
		s.cancelActiveBets(binding)
	}

	if err := s.repo.SetInactive(binding.ID); err != nil {
		return fmt.Errorf("移除绑定失败: %w", err)
	}

	logger.Infof("已移除 Riot Watch 绑定: %s#%s <- 频道 %s", riotID, riotTag, channelID)
	return nil
}

// GetBindingsByChannel 获取频道的所有绑定
func (s *Service) GetBindingsByChannel(guildID, channelID string) ([]Binding, error) {
	return s.repo.GetBindingsByChannel(guildID, channelID)
}

// CheckAndCreateBets 检查玩家是否开始游戏并创建竞猜
func (s *Service) CheckAndCreateBets(ctx context.Context, binding *Binding) error {
	if binding.RiotPUUID == nil {
		return fmt.Errorf("绑定缺少 PUUID")
	}

	// 已经在追踪游戏，跳过
	if binding.CurrentGameID != nil {
		return nil
	}

	// 检查是否在游戏中
	game, err := s.riotClient.GetActiveGame(ctx, *binding.RiotPUUID)
	if err != nil {
		if IsNotFound(err) {
			// 不在游戏中，更新检查时间
			s.repo.UpdateLastChecked(binding.ID)
			return nil
		}
		if IsRateLimited(err) {
			logger.Warnf("Riot API 限流，跳过绑定 %s", binding.ID)
			return nil
		}
		// 401/403 是持久性错误（API Key 失效/无权限），与临时故障区分开，用 ERROR 级别提示运维处理
		if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrForbidden) {
			logger.Errorf("Riot API Key 无效或无权限，请检查 RIOT_API_KEY 是否过期 (绑定 %s): %v", binding.ID, err)
			return nil
		}
		// 其他临时错误（502、503、网络超时等）：Riot API 服务暂时不可用，跳过本次检查
		logger.Warnf("Riot API 临时错误，跳过本次检查 (绑定 %s): %v", binding.ID, err)
		return nil
	}

	// 只处理单双排游戏
	if !IsSoloDuoRanked(game.GameQueueConfigID) {
		s.repo.UpdateLastChecked(binding.ID)
		return nil
	}

	// 创建竞猜
	return s.createBetsForGame(ctx, binding, game)
}

// createBetsForGame 为游戏创建两个竞猜
func (s *Service) createBetsForGame(ctx context.Context, binding *Binding, game *ActiveGame) error {
	// Match ID 格式: {REGION}_gameId，区域代码需要大写 (如 NA1_5458765734)
	gameIDStr := fmt.Sprintf("%s_%d", strings.ToUpper(s.riotClient.region), game.GameID)
	playerName := fmt.Sprintf("%s#%s", binding.RiotID, binding.RiotTag)

	// 使用绑定创建者的身份来创建竞猜
	creatorID := binding.CreatedBy
	creatorDiscordID := binding.CreatedByDiscordID

	// 1. 创建输赢竞猜
	winBetName := fmt.Sprintf("🎮 %s 本局输赢", playerName)
	winBet, err := s.bettingService.CreateBet(betting.CreateBetRequest{
		Name:             &winBetName,
		GuildID:          binding.GuildID,
		ChannelID:        binding.ChannelID,
		CreatorID:        creatorID,
		CreatorDiscordID: creatorDiscordID,
		TeamAName:        "🏆 赢",
		TeamBName:        "💀 输",
	})
	if err != nil {
		return fmt.Errorf("创建输赢竞猜失败: %w", err)
	}

	// 2. 创建死亡数竞猜
	deathBetName := fmt.Sprintf("💀 %s 本局死亡数", playerName)
	deathBet, err := s.bettingService.CreateBet(betting.CreateBetRequest{
		Name:             &deathBetName,
		GuildID:          binding.GuildID,
		ChannelID:        binding.ChannelID,
		CreatorID:        creatorID,
		CreatorDiscordID: creatorDiscordID,
		TeamAName:        fmt.Sprintf("<%g 次", binding.DeathThreshold),
		TeamBName:        fmt.Sprintf(">%g 次", binding.DeathThreshold),
	})
	if err != nil {
		// 回滚：取消已创建的输赢竞猜
		s.bettingService.CancelBet(winBet.ID, creatorID)
		return fmt.Errorf("创建死亡数竞猜失败: %w", err)
	}

	// 3. 更新绑定状态
	if err := s.repo.UpdateGameState(binding.ID, &gameIDStr, &winBet.ID, &deathBet.ID); err != nil {
		// 回滚：取消两个竞猜
		s.bettingService.CancelBet(winBet.ID, creatorID)
		s.bettingService.CancelBet(deathBet.ID, creatorID)
		return fmt.Errorf("更新游戏状态失败: %w", err)
	}

	// 4. 发送游戏开始通知
	s.sendGameStartEmbed(binding, game)

	// 5. 发送带下注按钮的竞猜消息
	if err := s.bettingCmdHandler.SendBetMessage(binding.ChannelID, winBet); err != nil {
		logger.Errorf("发送输赢竞猜消息失败: %v", err)
	}
	if err := s.bettingCmdHandler.SendBetMessage(binding.ChannelID, deathBet); err != nil {
		logger.Errorf("发送死亡数竞猜消息失败: %v", err)
	}

	logger.Infof("已为 %s 创建自动竞猜 (游戏ID: %d)", playerName, game.GameID)
	return nil
}

// AutoLockThreshold 自动封盘的游戏时长阈值（秒）
const AutoLockThreshold = 3 * 60 // 3 分钟

// CheckAndSettleBets 检查游戏是否结束并结算竞猜
func (s *Service) CheckAndSettleBets(ctx context.Context, binding *Binding) error {
	if binding.CurrentGameID == nil || binding.RiotPUUID == nil {
		return nil
	}

	// 检查是否还在游戏中
	game, err := s.riotClient.GetActiveGame(ctx, *binding.RiotPUUID)
	if err == nil && game != nil {
		// 仍在游戏中，检查是否需要自动封盘
		s.checkAndAutoLockBets(binding, game)
		return nil
	}

	// 只有明确返回 NotFound 才认为游戏结束
	// 其他错误（如 API 超时、502等）应该跳过本次检查
	if err != nil && !IsNotFound(err) {
		logger.Debugf("检查游戏状态时出错，稍后重试: %v", err)
		return nil
	}

	// 游戏已结束（GetActiveGame 返回 NotFound），获取对局结果
	// 等待一小段时间让 Riot 处理数据
	matchID := *binding.CurrentGameID
	match, err := s.riotClient.GetMatch(ctx, matchID)
	if err != nil {
		if IsNotFound(err) {
			// 对局数据可能还没准备好，稍后重试
			logger.Debugf("对局数据尚未准备好: %s", matchID)
			return nil
		}
		return fmt.Errorf("获取对局结果失败: %w", err)
	}

	// 找到玩家结果
	var playerResult *MatchParticipant
	for i := range match.Info.Participants {
		if match.Info.Participants[i].PUUID == *binding.RiotPUUID {
			playerResult = &match.Info.Participants[i]
			break
		}
	}

	if playerResult == nil {
		// 对局数据中不包含该玩家（可能是重开局或数据异常），取消竞猜并清除游戏状态
		playerName := fmt.Sprintf("%s#%s", binding.RiotID, binding.RiotTag)
		logger.Warnf("对局 %s 中未找到玩家 %s，取消竞猜并清除游戏状态", matchID, playerName)
		s.cancelActiveBets(binding)
		if err := s.repo.ClearGameState(binding.ID); err != nil {
			logger.Errorf("清除游戏状态失败: %v", err)
		}
		return nil
	}

	// 结算竞猜
	return s.settleBets(binding, playerResult)
}

// checkAndAutoLockBets 检查游戏时长并自动封盘
func (s *Service) checkAndAutoLockBets(binding *Binding, game *ActiveGame) {
	// GameLength 是秒数，检查是否超过阈值
	if game.GameLength < AutoLockThreshold {
		return
	}

	playerName := fmt.Sprintf("%s#%s", binding.RiotID, binding.RiotTag)
	creatorID := binding.CreatedBy
	lockedAny := false
	cancelledAny := false

	// 自动锁定输赢竞猜
	if binding.CurrentWinBetID != nil {
		result, err := s.bettingService.AutoLockBetting(*binding.CurrentWinBetID, creatorID)
		if err != nil {
			logger.Warnf("自动封盘输赢竞猜失败: %v", err)
		} else if result.Locked {
			lockedAny = true
			logger.Infof("自动封盘输赢竞猜 (游戏时长: %d秒): %s", game.GameLength, playerName)
		} else if result.Cancelled {
			cancelledAny = true
			logger.Infof("自动取消输赢竞猜 (无人下注): %s", playerName)
		}
	}

	// 自动锁定死亡数竞猜
	if binding.CurrentDeathBetID != nil {
		result, err := s.bettingService.AutoLockBetting(*binding.CurrentDeathBetID, creatorID)
		if err != nil {
			logger.Warnf("自动封盘死亡数竞猜失败: %v", err)
		} else if result.Locked {
			lockedAny = true
			logger.Infof("自动封盘死亡数竞猜 (游戏时长: %d秒): %s", game.GameLength, playerName)
		} else if result.Cancelled {
			cancelledAny = true
			logger.Infof("自动取消死亡数竞猜 (无人下注): %s", playerName)
		}
	}

	// 发送通知
	if lockedAny {
		s.sendAutoLockEmbed(binding, game.GameLength)
	} else if cancelledAny {
		s.sendAutoCancelEmbed(binding)
	}
}

// sendAutoLockEmbed 发送自动封盘通知
func (s *Service) sendAutoLockEmbed(binding *Binding, gameLength int64) {
	playerName := fmt.Sprintf("%s#%s", binding.RiotID, binding.RiotTag)
	minutes := gameLength / 60

	embed := &discordgo.MessageEmbed{
		Title:       "🔒 竞猜已自动封盘",
		Description: fmt.Sprintf("**%s** 的游戏已进行 **%d 分钟**，竞猜已自动封盘！\n\n等待游戏结束后自动结算...", playerName, minutes),
		Color:       0xffa500, // 橙色
	}

	_, err := s.session.ChannelMessageSendEmbed(binding.ChannelID, embed)
	if err != nil {
		logger.Errorf("发送自动封盘通知失败: %v", err)
	}
}

// sendAutoCancelEmbed 发送自动取消通知
func (s *Service) sendAutoCancelEmbed(binding *Binding) {
	playerName := fmt.Sprintf("%s#%s", binding.RiotID, binding.RiotTag)

	embed := &discordgo.MessageEmbed{
		Title:       "❌ 竞猜已自动取消",
		Description: fmt.Sprintf("**%s** 的竞猜因无人下注已自动取消。", playerName),
		Color:       0x808080, // 灰色
	}

	_, err := s.session.ChannelMessageSendEmbed(binding.ChannelID, embed)
	if err != nil {
		logger.Errorf("发送自动取消通知失败: %v", err)
	}
}

// settleBets 结算两个竞猜
func (s *Service) settleBets(binding *Binding, result *MatchParticipant) error {
	// 使用绑定创建者的身份来结算竞猜
	creatorID := binding.CreatedBy

	playerName := fmt.Sprintf("%s#%s", binding.RiotID, binding.RiotTag)
	var winBetResult, deathBetResult string
	var winSettlement, deathSettlement *betting.SettlementResult

	// 1. 结算输赢竞猜
	if binding.CurrentWinBetID != nil {
		// 先锁定竞猜
		if err := s.bettingService.LockBetting(*binding.CurrentWinBetID, creatorID); err != nil {
			logger.Warnf("锁定输赢竞猜失败: %v", err)
		}

		// 确定胜方
		winnerTeam := "A" // 赢
		if !result.Win {
			winnerTeam = "B" // 输
		}

		settlement, err := s.bettingService.SettleBet(*binding.CurrentWinBetID, creatorID, winnerTeam)
		if err != nil {
			logger.Errorf("结算输赢竞猜失败: %v", err)
		} else {
			winSettlement = settlement
		}

		if result.Win {
			winBetResult = "🏆 获胜"
		} else {
			winBetResult = "💀 落败"
		}
	}

	// 2. 结算死亡数竞猜
	if binding.CurrentDeathBetID != nil {
		// 先锁定竞猜
		if err := s.bettingService.LockBetting(*binding.CurrentDeathBetID, creatorID); err != nil {
			logger.Warnf("锁定死亡数竞猜失败: %v", err)
		}

		// 确定胜方：使用 float64 比较避免整数边界问题
		winnerTeam := "A" // 死亡数 < 阈值
		if float64(result.Deaths) > binding.DeathThreshold {
			winnerTeam = "B" // 死亡数 > 阈值
		}

		settlement, err := s.bettingService.SettleBet(*binding.CurrentDeathBetID, creatorID, winnerTeam)
		if err != nil {
			logger.Errorf("结算死亡数竞猜失败: %v", err)
		} else {
			deathSettlement = settlement
		}

		if float64(result.Deaths) < binding.DeathThreshold {
			deathBetResult = fmt.Sprintf("<%g (实际: %d)", binding.DeathThreshold, result.Deaths)
		} else {
			deathBetResult = fmt.Sprintf(">%g (实际: %d)", binding.DeathThreshold, result.Deaths)
		}
	}

	// 3. 清除游戏状态
	if err := s.repo.ClearGameState(binding.ID); err != nil {
		logger.Errorf("清除游戏状态失败: %v", err)
	}

	// 4. 发送结果通知
	s.sendGameEndEmbed(binding, result, winBetResult, deathBetResult, winSettlement, deathSettlement)

	logger.Infof("已结算 %s 的竞猜: Win=%v, Deaths=%d", playerName, result.Win, result.Deaths)
	return nil
}

// cancelActiveBets 取消活跃的竞猜
func (s *Service) cancelActiveBets(binding *Binding) {
	// 使用绑定创建者的身份来取消竞猜
	creatorID := binding.CreatedBy

	if binding.CurrentWinBetID != nil {
		if err := s.bettingService.CancelBet(*binding.CurrentWinBetID, creatorID); err != nil {
			logger.Errorf("取消输赢竞猜失败: %v", err)
		}
	}

	if binding.CurrentDeathBetID != nil {
		if err := s.bettingService.CancelBet(*binding.CurrentDeathBetID, creatorID); err != nil {
			logger.Errorf("取消死亡数竞猜失败: %v", err)
		}
	}
}

// sendGameStartEmbed 发送游戏开始通知
func (s *Service) sendGameStartEmbed(binding *Binding, game *ActiveGame) {
	playerName := fmt.Sprintf("%s#%s", binding.RiotID, binding.RiotTag)

	// 找到玩家使用的英雄
	var championID int64
	for _, p := range game.Participants {
		if binding.RiotPUUID != nil && p.PUUID == *binding.RiotPUUID {
			championID = p.ChampionID
			break
		}
	}

	// 通过 Data Dragon API 获取英雄名称
	championName := s.riotClient.GetChampionName(championID)

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("🎮 游戏开始: %s", playerName),
		Description: "检测到单双排游戏！快来下注吧！\n\n⬇️ 点击下方按钮参与竞猜 ⬇️",
		Color:       0x00ff00, // 绿色
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "🎭 英雄",
				Value:  championName,
				Inline: true,
			},
			{
				Name:   "💀 死亡阈值",
				Value:  fmt.Sprintf("%g", binding.DeathThreshold),
				Inline: true,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	_, err := s.session.ChannelMessageSendEmbed(binding.ChannelID, embed)
	if err != nil {
		logger.Errorf("发送游戏开始通知失败: %v", err)
	}
}

// sendGameEndEmbed 发送游戏结束通知
func (s *Service) sendGameEndEmbed(binding *Binding, result *MatchParticipant, winResult, deathResult string, winSettlement, deathSettlement *betting.SettlementResult) {
	playerName := fmt.Sprintf("%s#%s", binding.RiotID, binding.RiotTag)

	color := 0xff0000 // 红色 = 输
	if result.Win {
		color = 0x00ff00 // 绿色 = 赢
	}

	fields := []*discordgo.MessageEmbedField{
		{
			Name:   "🎯 对局结果",
			Value:  winResult,
			Inline: true,
		},
		{
			Name:   "💀 死亡数",
			Value:  deathResult,
			Inline: true,
		},
		{
			Name:   "📊 KDA",
			Value:  fmt.Sprintf("%d / %d / %d", result.Kills, result.Deaths, result.Assists),
			Inline: true,
		},
		{
			Name:   "🎭 英雄",
			Value:  result.ChampionName,
			Inline: true,
		},
	}

	// 添加输赢竞猜结算详情
	if winSettlement != nil {
		settlementInfo := s.formatSettlementInfo(winSettlement, "输赢")
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "🎲 输赢竞猜结算",
			Value:  settlementInfo,
			Inline: false,
		})
	}

	// 添加死亡数竞猜结算详情
	if deathSettlement != nil {
		settlementInfo := s.formatSettlementInfo(deathSettlement, "死亡数")
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "💀 死亡数竞猜结算",
			Value:  settlementInfo,
			Inline: false,
		})
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("🏁 游戏结束: %s", playerName),
		Description: "竞猜已自动结算！",
		Color:       color,
		Fields:      fields,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	_, err := s.session.ChannelMessageSendEmbed(binding.ChannelID, embed)
	if err != nil {
		logger.Errorf("发送游戏结束通知失败: %v", err)
	}
}

// formatSettlementInfo 格式化结算信息
func (s *Service) formatSettlementInfo(settlement *betting.SettlementResult, betType string) string {
	var lines []string

	// 获胜者列表 - 显示拿回的总积分
	if len(settlement.Winners) > 0 {
		lines = append(lines, "**🎉 获胜者:**")
		for _, winner := range settlement.Winners {
			payout := 0
			if settlement.Payouts != nil {
				payout = settlement.Payouts[winner.UserID]
			}

			if winner.DiscordID == database.SystemDiscordID {
				lines = append(lines, fmt.Sprintf("🤖 系统对冲: 拿回 %d积分", payout))
			} else {
				lines = append(lines, fmt.Sprintf("<@%s>: 拿回 %d积分", winner.DiscordID, payout))
			}
		}
	}

	// 失败者列表
	if len(settlement.Losers) > 0 {
		lines = append(lines, "**💔 失败者:**")
		for _, loser := range settlement.Losers {
			if loser.DiscordID == database.SystemDiscordID {
				lines = append(lines, fmt.Sprintf("🤖 系统对冲: -%d积分", loser.BetAmount))
			} else {
				lines = append(lines, fmt.Sprintf("<@%s>: -%d积分", loser.DiscordID, loser.BetAmount))
			}
		}
	}

	// 奖池信息
	poolInfo := fmt.Sprintf("💰 奖池: %d积分", settlement.TotalPool)
	if settlement.Commission > 0 {
		poolInfo += fmt.Sprintf(" (管理费 %d积分)", settlement.Commission)
	}
	lines = append(lines, poolInfo)

	if len(lines) == 0 {
		return "无参与者"
	}

	return strings.Join(lines, "\n")
}

// GetRepository 获取 Repository（供 Monitor 使用）
func (s *Service) GetRepository() *Repository {
	return s.repo
}

// ErrBindingNotFound 绑定不存在
var ErrBindingNotFound = errors.New("binding not found")
