package protrack

import (
	"errors"
	"testing"
	"time"

	"github.com/jaobsenyc/discord-lfg-bot-go/internal/riotwatch"
)

func int64Ptr(v int64) *int64 { return &v }

func TestDecide(t *testing.T) {
	soloDuo := func(id int64) *riotwatch.ActiveGame {
		return &riotwatch.ActiveGame{GameID: id, GameQueueConfigID: riotwatch.QueueSoloDuo}
	}
	aram := &riotwatch.ActiveGame{GameID: 99, GameQueueConfigID: 450}

	tests := []struct {
		name    string
		current *int64
		last    *int64
		game    *riotwatch.ActiveGame
		err     error
		want    action
	}{
		{"空闲且不在游戏", nil, nil, nil, riotwatch.ErrNotFound, actionNone},
		{"新开单双排", nil, nil, soloDuo(1), nil, actionStart},
		{"同一局进行中", int64Ptr(1), nil, soloDuo(1), nil, actionNone},
		{"对局结束", int64Ptr(1), nil, nil, riotwatch.ErrNotFound, actionEnd},
		{"包装过的 404 也算结束", int64Ptr(1), nil, nil, errors.Join(errors.New("ctx"), riotwatch.ErrNotFound), actionEnd},
		{"结束后又查到同一局视为抖动", nil, int64Ptr(1), soloDuo(1), nil, actionResume},
		{"上一局之后开了新局", nil, int64Ptr(1), soloDuo(2), nil, actionStart},
		{"停机期间换了一局", int64Ptr(1), nil, soloDuo(2), nil, actionSwitch},
		{"进入非单双排队列视为结束", int64Ptr(1), nil, aram, nil, actionEnd},
		{"空闲时打非单双排不播报", nil, nil, aram, nil, actionNone},
		{"查询出错保持游戏中", int64Ptr(1), nil, nil, errors.New("timeout"), actionNone},
		{"限流保持游戏中", int64Ptr(1), nil, nil, riotwatch.ErrRateLimited, actionNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decide(tt.current, tt.last, tt.game, tt.err); got != tt.want {
				t.Errorf("decide() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGameDetails(t *testing.T) {
	game := &riotwatch.ActiveGame{
		GameStartTime: 1_700_000_000_000,
		Participants: []riotwatch.Participant{
			{PUUID: "other", ChampionID: 1},
			{PUUID: "me", ChampionID: 103},
		},
	}
	startedAt, championID := gameDetails(game, "me")
	if !startedAt.Equal(time.UnixMilli(1_700_000_000_000)) {
		t.Errorf("startedAt = %v", startedAt)
	}
	if championID != 103 {
		t.Errorf("championID = %d, want 103", championID)
	}

	// 加载阶段 gameStartTime 为 0,用当前时间
	game.GameStartTime = 0
	startedAt, _ = gameDetails(game, "me")
	if time.Since(startedAt) > time.Second {
		t.Errorf("加载阶段 startedAt 应接近当前时间, got %v", startedAt)
	}
}
