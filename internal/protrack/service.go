package protrack

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jaobsenyc/discord-lfg-bot-go/internal/database"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/rank"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/riotwatch"
)

// Service 职业选手追踪业务逻辑
type Service struct {
	repo       *Repository
	riotClient *riotwatch.RiotAPIClient
	maxTracked int
}

// NewService 创建服务;riotClient 与 riotwatch/rank 共用以共享限流
func NewService(db *database.DB, riotClient *riotwatch.RiotAPIClient, maxTracked int) *Service {
	return &Service{
		repo:       NewRepository(db),
		riotClient: riotClient,
		maxTracked: maxTracked,
	}
}

// TrackResult /trackpro add 的结果
type TrackResult struct {
	Added   []ProAccount // 本次新开始追踪的账号
	Already []ProAccount // 之前已在追踪的账号(播报频道已更新为当前频道)
}

// Track 开始追踪 target(选手名或 名字#TAG);未入库的 Riot ID 会校验后以 proName 自动入库
func (s *Service) Track(ctx context.Context, target, proName, channelID, discordID string) (*TrackResult, error) {
	accounts, err := s.resolveTarget(ctx, target, proName, true)
	if err != nil {
		return nil, err
	}

	result := &TrackResult{}
	var pending []ProAccount
	for _, a := range accounts {
		if a.IsTracked {
			result.Already = append(result.Already, a)
		} else {
			pending = append(pending, a)
		}
	}

	if len(pending) > 0 {
		count, err := s.repo.CountTracked(ctx)
		if err != nil {
			return nil, err
		}
		if count+len(pending) > s.maxTracked {
			return nil, fmt.Errorf("追踪数量已达上限(%d/%d),请先 /trackpro remove 一些选手", count, s.maxTracked)
		}
	}

	for i := range pending {
		// 预置或手动录入的账号可能还没有 PUUID,开始追踪前先确认账号存在
		if err := s.ensurePUUID(ctx, &pending[i]); err != nil {
			return nil, err
		}
	}

	for _, a := range accounts {
		if err := s.repo.SetTracked(ctx, a.ID, channelID, discordID); err != nil {
			return nil, err
		}
	}
	result.Added = pending
	return result, nil
}

// Untrack 取消追踪 target 下所有追踪中的账号
func (s *Service) Untrack(ctx context.Context, target string) ([]ProAccount, error) {
	accounts, err := s.resolveTarget(ctx, target, "", false)
	if err != nil {
		return nil, err
	}

	var removed []ProAccount
	for _, a := range accounts {
		if !a.IsTracked {
			continue
		}
		if err := s.repo.Untrack(ctx, a.ID); err != nil {
			return nil, err
		}
		removed = append(removed, a)
	}
	if len(removed) == 0 {
		return nil, ErrNotTracked
	}
	return removed, nil
}

// Register 录入或更新选手库中的账号(不开始追踪);返回账号及是否为新建
func (s *Service) Register(ctx context.Context, riotIDFull, proName string, team *string) (*ProAccount, bool, error) {
	gameName, tagLine, err := rank.ParseRiotID(riotIDFull)
	if err != nil {
		return nil, false, err
	}

	existing, err := s.repo.GetByRiotID(ctx, gameName, tagLine)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		if err := s.repo.UpdateProInfo(ctx, existing.ID, proName, team); err != nil {
			return nil, false, err
		}
		existing.ProName, existing.Team = proName, team
		return existing, false, nil
	}

	a, err := s.createAccount(ctx, gameName, tagLine, proName, team)
	if err != nil {
		return nil, false, err
	}
	return a, true, nil
}

// ListTracked 追踪中的账号
func (s *Service) ListTracked(ctx context.Context) ([]ProAccount, error) {
	return s.repo.ListTracked(ctx)
}

// ListAll 选手库全部账号
func (s *Service) ListAll(ctx context.Context) ([]ProAccount, error) {
	return s.repo.ListAll(ctx)
}

// ChampionName 英雄 ID 转中文名
func (s *Service) ChampionName(championID int64) string {
	return s.riotClient.GetChampionName(championID)
}

// resolveTarget 把命令参数解析为账号列表:
// 含 # 视为 Riot ID(create 为 true 时库中没有则校验后入库),否则按选手名查询全部账号
func (s *Service) resolveTarget(ctx context.Context, target, proName string, create bool) ([]ProAccount, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, errors.New("请输入选手名或 Riot ID (名字#TAG)")
	}

	if !strings.Contains(target, "#") {
		accounts, err := s.repo.ListByProName(ctx, target)
		if err != nil {
			return nil, err
		}
		if len(accounts) == 0 {
			return nil, ErrProNotFound
		}
		return accounts, nil
	}

	gameName, tagLine, err := rank.ParseRiotID(target)
	if err != nil {
		return nil, err
	}
	a, err := s.repo.GetByRiotID(ctx, gameName, tagLine)
	if err != nil {
		return nil, err
	}
	if a != nil {
		return []ProAccount{*a}, nil
	}
	if !create {
		return nil, ErrNotTracked
	}

	a, err = s.createAccount(ctx, gameName, tagLine, proName, nil)
	if err != nil {
		return nil, err
	}
	return []ProAccount{*a}, nil
}

// createAccount 经 Riot API 校验账号后入库;proName 为空时用游戏名代替
func (s *Service) createAccount(ctx context.Context, gameName, tagLine, proName string, team *string) (*ProAccount, error) {
	account, err := s.riotClient.GetAccountByRiotID(ctx, gameName, tagLine)
	if err != nil {
		if errors.Is(err, riotwatch.ErrNotFound) {
			return nil, fmt.Errorf("%w: %s#%s", ErrRiotIDNotFound, gameName, tagLine)
		}
		return nil, fmt.Errorf("查询 Riot 账号失败: %w", err)
	}

	if strings.TrimSpace(proName) == "" {
		proName = account.GameName
	}
	a := &ProAccount{
		ProName:      strings.TrimSpace(proName),
		Team:         team,
		RiotGameName: account.GameName,
		RiotTagLine:  account.TagLine,
		RiotPUUID:    &account.PUUID,
	}
	if err := s.repo.Create(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// ensurePUUID 账号缺少 PUUID 时经 account-v1 解析并回写
func (s *Service) ensurePUUID(ctx context.Context, a *ProAccount) error {
	if a.RiotPUUID != nil && *a.RiotPUUID != "" {
		return nil
	}

	account, err := s.riotClient.GetAccountByRiotID(ctx, a.RiotGameName, a.RiotTagLine)
	if err != nil {
		if errors.Is(err, riotwatch.ErrNotFound) {
			return fmt.Errorf("%w: %s", ErrRiotIDNotFound, a.RiotID())
		}
		return fmt.Errorf("查询 Riot 账号 %s 失败: %w", a.RiotID(), err)
	}

	if err := s.repo.SetRiotAccount(ctx, a.ID, account.PUUID, account.GameName, account.TagLine); err != nil {
		return err
	}
	a.RiotPUUID = &account.PUUID
	a.RiotGameName, a.RiotTagLine = account.GameName, account.TagLine
	return nil
}
