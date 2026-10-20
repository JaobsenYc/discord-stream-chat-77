package protrack

import (
	"strings"
	"testing"
	"time"

	"github.com/jaobsenyc/discord-lfg-bot-go/internal/rank"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{0, "未知"},
		{-time.Second, "未知"},
		{65 * time.Second, "1:05"},
		{31*time.Minute + 7*time.Second + 400*time.Millisecond, "31:07"},
		{time.Hour + 2*time.Minute + 3*time.Second, "1:02:03"},
	}
	for _, tt := range tests {
		if got := formatDuration(tt.in); got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestJoinLines(t *testing.T) {
	a, b, c := strings.Repeat("a", 10), strings.Repeat("b", 10), strings.Repeat("c", 10)
	lines := []string{a, b, c}
	full := a + "\n" + b + "\n" + c // 32 字节

	tests := []struct {
		limit int
		want  string
	}{
		{100, full},
		{32, full}, // 恰好放得下时不截断
		{31, a + "\n…还有 2 条"},
		{26, a + "\n…还有 2 条"},
		{25, "…还有 3 条"},
	}
	for _, tt := range tests {
		got := joinLines(lines, tt.limit)
		if got != tt.want {
			t.Errorf("joinLines(limit=%d) = %q, want %q", tt.limit, got, tt.want)
		}
		if len(got) > tt.limit {
			t.Errorf("limit=%d 结果长度 %d 超过上限", tt.limit, len(got))
		}
	}
}

func TestDisplayName(t *testing.T) {
	team := "T1"
	a := &ProAccount{ProName: "Faker", Team: &team, RiotGameName: "Hide in bush", RiotTagLine: "NYKR"}
	if got := a.DisplayName(); got != "Faker (T1)" {
		t.Errorf("DisplayName() = %q", got)
	}
	if got := a.RiotID(); got != "Hide in bush#NYKR" {
		t.Errorf("RiotID() = %q", got)
	}
	a.Team = nil
	if got := a.DisplayName(); got != "Faker" {
		t.Errorf("无战队时 DisplayName() = %q", got)
	}
}

func TestEndMessageMention(t *testing.T) {
	a := &ProAccount{ProName: "Zeus", RiotGameName: "Austin", RiotTagLine: "lll"}

	msg := endMessage(a, "格温", 25*time.Minute, "https://example.com", "")
	if strings.Contains(msg.Content, "<@&") || len(msg.AllowedMentions.Roles) != 0 {
		t.Errorf("未配置身份组时不应 @, got %q", msg.Content)
	}

	msg = endMessage(a, "格温", 25*time.Minute, "https://example.com", "123")
	if !strings.HasPrefix(msg.Content, "<@&123> ") {
		t.Errorf("应 @ 身份组, got %q", msg.Content)
	}
	if len(msg.AllowedMentions.Roles) != 1 || msg.AllowedMentions.Roles[0] != "123" {
		t.Errorf("只应放行配置的身份组, got %v", msg.AllowedMentions.Roles)
	}
}

func TestRiotIDHelpers(t *testing.T) {
	name, tag, err := rank.ParseRiotID(" Hide in bush #NYKR\u2069")
	if err != nil || name != "Hide in bush" || tag != "NYKR" {
		t.Errorf("ParseRiotID = %q, %q, %v", name, tag, err)
	}

	if _, _, err := rank.ParseRiotID("Faker"); err == nil {
		t.Error("缺少 # 时应报错")
	}

	got := rank.OpggURL(rank.ToOpggRegion("na1"), "Hide in bush", "NYKR")
	if got != "https://www.op.gg/summoners/NA/Hide%20in%20bush-NYKR" {
		t.Errorf("OpggURL = %q", got)
	}
}
