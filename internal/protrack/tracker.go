package protrack

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/rank"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/riotwatch"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// action 一次轮询后对某个账号要做的动作
type action int

const (
	actionNone   action = iota // 状态不变
	actionStart                // 开了新一局单双排:开局播报
	actionResume               // 刚结束的那局又查得到(spectator 抖动):静默恢复,不重复播报
	actionEnd                  // 对局结束:结束播报
	actionSwitch               // 上一局已结束且已开新局(多见于 bot 停机期间):先结束播报再开局播报
)

// decide 根据库中的对局状态和本次 spectator 查询结果决定动作。
// 只关注单双排(420),进入其他队列视同单双排对局已结束;查询出错(非 404)时保持原状,下一轮重试。
func decide(currentGameID, lastGameID *int64, game *riotwatch.ActiveGame, err error) action {
	inSoloDuo := err == nil && game != nil && riotwatch.IsSoloDuoRanked(game.GameQueueConfigID)
	if !inSoloDuo {
		notInGame := errors.Is(err, riotwatch.ErrNotFound) || (err == nil && game != nil)
		if notInGame && currentGameID != nil {
			return actionEnd
		}
		return actionNone
	}

	switch {
	case currentGameID != nil && *currentGameID == game.GameID:
		return actionNone
	case currentGameID != nil:
		return actionSwitch
	case lastGameID != nil && *lastGameID == game.GameID:
		return actionResume
	default:
		return actionStart
	}
}

// Tracker 定时轮询追踪中的账号并播报开局/结束
type Tracker struct {
	service       *Service
	session       *discordgo.Session
	interval      time.Duration
	mentionRoleID string
	opggRegion    string

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewTracker 创建轮询器
func NewTracker(service *Service, session *discordgo.Session, interval time.Duration, mentionRoleID, riotRegion string) *Tracker {
	ctx, cancel := context.WithCancel(context.Background())
	return &Tracker{
		service:       service,
		session:       session,
		interval:      interval,
		mentionRoleID: mentionRoleID,
		opggRegion:    rank.ToOpggRegion(riotRegion),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start 启动轮询
func (t *Tracker) Start() {
	t.wg.Add(1)
	go t.pollLoop()
}

// Stop 停止轮询并等待退出
func (t *Tracker) Stop() {
	t.cancel()
	t.wg.Wait()
	logger.Info("职业选手追踪器已停止")
}

func (t *Tracker) pollLoop() {
	defer t.wg.Done()

	// 启动后立即检查一次,补发停机期间结束的对局
	t.checkAll()

	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			t.checkAll()
		}
	}
}

// checkAll 逐个检查追踪中的账号;请求速率由 riotClient 的限流器控制
func (t *Tracker) checkAll() {
	accounts, err := t.service.repo.ListTracked(t.ctx)
	if err != nil {
		if t.ctx.Err() == nil {
			logger.Errorf("职业选手追踪: %v", err)
		}
		return
	}

	for i := range accounts {
		if t.ctx.Err() != nil {
			return
		}
		err := t.checkAccount(&accounts[i])
		if errors.Is(err, riotwatch.ErrRateLimited) {
			logger.Warn("职业选手追踪: Riot API 限流,跳过本轮剩余账号")
			return
		}
		if err != nil && t.ctx.Err() == nil {
			logger.Warnf("职业选手追踪: %v", err)
		}
	}
}

func (t *Tracker) checkAccount(a *ProAccount) error {
	if err := t.service.ensurePUUID(t.ctx, a); err != nil {
		return err
	}

	game, err := t.service.riotClient.GetActiveGame(t.ctx, *a.RiotPUUID)
	if errors.Is(err, riotwatch.ErrRateLimited) {
		return err
	}

	repo := t.service.repo
	switch decide(a.CurrentGameID, a.LastGameID, game, err) {
	case actionStart:
		return t.startGame(a, game, true)

	case actionResume:
		return t.startGame(a, game, false)

	case actionEnd:
		if err := repo.ClearGame(t.ctx, a.ID); err != nil {
			return err
		}
		t.sendEnd(a)

	case actionSwitch:
		if err := repo.ClearGame(t.ctx, a.ID); err != nil {
			return err
		}
		t.sendEnd(a)
		return t.startGame(a, game, true)

	default:
		if err != nil && !errors.Is(err, riotwatch.ErrNotFound) {
			return fmt.Errorf("查询 %s 对局状态失败: %w", a.RiotID(), err)
		}
		return repo.TouchChecked(t.ctx, a.ID)
	}
	return nil
}

// startGame 记录进行中的对局,announce 为 true 时播报开局
func (t *Tracker) startGame(a *ProAccount, game *riotwatch.ActiveGame, announce bool) error {
	startedAt, championID := gameDetails(game, *a.RiotPUUID)
	if err := t.service.repo.SetInGame(t.ctx, a.ID, game.GameID, startedAt, championID); err != nil {
		return err
	}
	if announce {
		t.send(a, startMessage(a, t.service.ChampionName(championID), startedAt, rank.OpggURL(t.opggRegion, a.RiotGameName, a.RiotTagLine)))
	}
	return nil
}

// sendEnd 播报 a 当前记录的对局已结束
func (t *Tracker) sendEnd(a *ProAccount) {
	championName := "未知"
	if a.CurrentChampionID != nil {
		championName = t.service.ChampionName(*a.CurrentChampionID)
	}
	var duration time.Duration
	if a.CurrentGameStartedAt != nil {
		duration = time.Since(*a.CurrentGameStartedAt)
	}
	t.send(a, endMessage(a, championName, duration, rank.OpggURL(t.opggRegion, a.RiotGameName, a.RiotTagLine), t.mentionRoleID))
}

func (t *Tracker) send(a *ProAccount, msg *discordgo.MessageSend) {
	if a.TrackChannelID == nil || *a.TrackChannelID == "" {
		logger.Warnf("职业选手追踪: %s 没有播报频道,跳过播报", a.RiotID())
		return
	}
	if _, err := t.session.ChannelMessageSendComplex(*a.TrackChannelID, msg); err != nil {
		logger.Errorf("职业选手追踪: 发送 %s 的播报失败: %v", a.RiotID(), err)
	}
}

// gameDetails 取对局开始时间和该玩家的英雄;加载阶段 gameStartTime 为 0,此时用当前时间
func gameDetails(game *riotwatch.ActiveGame, puuid string) (time.Time, int64) {
	startedAt := time.Now()
	if game.GameStartTime > 0 {
		startedAt = time.UnixMilli(game.GameStartTime)
	}
	var championID int64
	for _, p := range game.Participants {
		if p.PUUID == puuid {
			championID = p.ChampionID
			break
		}
	}
	return startedAt, championID
}
