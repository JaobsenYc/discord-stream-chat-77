package bot

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

// recorder 返回一个把标记写入 *string 的处理器,用于验证分发目标
func recorder(target *string, name string) handlerFunc {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		*target = name
	}
}

func TestRouterSlashDispatch(t *testing.T) {
	r := newInteractionRouter()
	var got string
	r.slashCmd(recorder(&got, "achievement"), "achievement", "achievement-summary", "achievement-admin")
	r.slashCmd(recorder(&got, "bet"), "bet")

	cases := []struct {
		name    string
		want    string
		matched bool
	}{
		{"achievement", "achievement", true},
		{"achievement-summary", "achievement", true},
		{"achievement-admin", "achievement", true},
		{"bet", "bet", true},
		{"unknown", "", false},
	}

	for _, c := range cases {
		got = ""
		h, ok := r.dispatchSlash(c.name)
		if ok != c.matched {
			t.Errorf("dispatchSlash(%q) matched=%v, want %v", c.name, ok, c.matched)
			continue
		}
		if ok {
			h(nil, nil)
			if got != c.want {
				t.Errorf("dispatchSlash(%q) 分发到 %q, want %q", c.name, got, c.want)
			}
		}
	}
}

func TestRouterComponentPrefixDispatch(t *testing.T) {
	r := newInteractionRouter()
	var got string
	r.component("lottery_", recorder(&got, "lottery"))
	r.component("bet_", recorder(&got, "betting"))
	r.component("redeem_", recorder(&got, "points"))

	cases := []struct {
		customID string
		want     string
		matched  bool
	}{
		{"lottery_join_123", "lottery", true},
		{"bet_place_win_456", "betting", true},
		{"redeem_confirm_789", "points", true},
		{"room_full_abc", "", false}, // 组队按钮走兜底,不在前缀表内
		{"lotteryX", "", false},      // 前缀必须完整含下划线
	}

	for _, c := range cases {
		got = ""
		h, ok := r.dispatchComponent(c.customID)
		if ok != c.matched {
			t.Errorf("dispatchComponent(%q) matched=%v, want %v", c.customID, ok, c.matched)
			continue
		}
		if ok {
			h(nil, nil)
			if got != c.want {
				t.Errorf("dispatchComponent(%q) 分发到 %q, want %q", c.customID, got, c.want)
			}
		}
	}
}

func TestRouterModalDispatch(t *testing.T) {
	r := newInteractionRouter()
	var got string
	r.modal("bet_", recorder(&got, "betting"))

	if h, ok := r.dispatchModal("bet_amount_123"); !ok {
		t.Fatal("dispatchModal(bet_amount_123) 未匹配")
	} else {
		h(nil, nil)
		if got != "betting" {
			t.Errorf("dispatchModal 分发到 %q, want betting", got)
		}
	}

	if _, ok := r.dispatchModal("other_modal"); ok {
		t.Error("dispatchModal(other_modal) 不应匹配")
	}
}

func TestParseTeamButtonID(t *testing.T) {
	cases := []struct {
		customID string
		action   string
		teamID   string
		ok       bool
	}{
		{"room_full_123456_1700000000", "room_full", "123456_1700000000", true},
		{"abandon_team_uid_99", "abandon_team", "uid_99", true},
		{"join_room_x", "join_room", "x", true},
		{"a_b", "", "", false}, // 不足三段
		{"nounderscore", "", "", false},
	}

	for _, c := range cases {
		action, teamID, ok := parseTeamButtonID(c.customID)
		if ok != c.ok || action != c.action || teamID != c.teamID {
			t.Errorf("parseTeamButtonID(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.customID, action, teamID, ok, c.action, c.teamID, c.ok)
		}
	}
}
