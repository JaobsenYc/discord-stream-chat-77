package protrack

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

const (
	colorGameStart = 0x2ecc71
	colorGameEnd   = 0xe74c3c
	colorInfo      = 0x3498db

	// embedDescriptionLimit 留出余量,Discord 上限为 4096
	embedDescriptionLimit = 4000
)

// startMessage 开局播报
func startMessage(a *ProAccount, championName string, startedAt time.Time, opggURL string) *discordgo.MessageSend {
	return &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{{
			Title: fmt.Sprintf("🎮 %s 开始单双排", a.DisplayName()),
			Color: colorGameStart,
			Fields: []*discordgo.MessageEmbedField{
				{Name: "账号", Value: fmt.Sprintf("[%s](%s)", a.RiotID(), opggURL), Inline: true},
				{Name: "英雄", Value: championName, Inline: true},
				{Name: "开局", Value: fmt.Sprintf("<t:%d:R>", startedAt.Unix()), Inline: true},
			},
			Timestamp: time.Now().Format(time.RFC3339),
		}},
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	}
}

// endMessage 结束播报;mentionRoleID 非空时 @ 该身份组且只放行这一个 mention
func endMessage(a *ProAccount, championName string, duration time.Duration, opggURL, mentionRoleID string) *discordgo.MessageSend {
	content := fmt.Sprintf("🏁 **%s** 单双排结束了,现在排队有机会撞车!", a.DisplayName())
	allowed := &discordgo.MessageAllowedMentions{}
	if mentionRoleID != "" {
		content = fmt.Sprintf("<@&%s> %s", mentionRoleID, content)
		allowed.Roles = []string{mentionRoleID}
	}

	return &discordgo.MessageSend{
		Content: content,
		Embeds: []*discordgo.MessageEmbed{{
			Title: fmt.Sprintf("🏁 %s 单双排结束", a.DisplayName()),
			Color: colorGameEnd,
			Fields: []*discordgo.MessageEmbedField{
				{Name: "账号", Value: fmt.Sprintf("[%s](%s)", a.RiotID(), opggURL), Inline: true},
				{Name: "英雄", Value: championName, Inline: true},
				{Name: "对局时长", Value: formatDuration(duration), Inline: true},
			},
			Timestamp: time.Now().Format(time.RFC3339),
		}},
		AllowedMentions: allowed,
	}
}

// formatDuration 格式化为 m:ss 或 h:mm:ss,未知时长返回 "未知"
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "未知"
	}
	d = d.Round(time.Second)
	h := int(d / time.Hour)
	m := int(d/time.Minute) % 60
	s := int(d/time.Second) % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// joinLines 按行拼接,超出 limit 时截断并注明剩余条数
func joinLines(lines []string, limit int) string {
	if all := strings.Join(lines, "\n"); len(all) <= limit {
		return all
	}

	var b strings.Builder
	for i, line := range lines {
		need := b.Len() + len(line)
		if i < len(lines)-1 {
			// 写入本行后仍要放得下剩余条数的提示
			need += 1 + len(fmt.Sprintf("…还有 %d 条", len(lines)-i-1))
		}
		if need > limit {
			b.WriteString(fmt.Sprintf("…还有 %d 条", len(lines)-i))
			break
		}
		b.WriteString(line)
		if i < len(lines)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}
