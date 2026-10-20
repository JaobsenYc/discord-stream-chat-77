// Package parser 解析 Discord / 游戏 LFG 消息（5=1、4缺1、lfg 等）。
package parser

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

//
// ===== 对外结构与错误 =====
//

var ErrUnrecognized = errors.New("消息无法识别为 LFG 格式")

type LFGMessage struct {
	MaxPlayers     int
	CurrentPlayers int
	OriginalText   string
}

//
// ===== 内部常量 / 工具 =====
//

// 中文数字映射
var cnNum = map[string]int{
	"一": 1, "二": 2, "两": 2, "三": 3, "四": 4, "五": 5,
	"六": 6, "七": 7, "八": 8, "九": 9, "十": 10,
}

func toInt(s string) (int, bool) {
	if n, ok := cnNum[s]; ok {
		return n, true
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}

// 两数字→cur/max 通用封装
func calc(curHint, maxHint int, mode parseMode) (cur, max int) {
	switch mode {
	case modePairSlash, modePairDash:
		return curHint, maxHint
	case modePairEqual:
		// a = b  => 可能是  a 人队 / a=b（剩 b） / a=b（等 b）
		switch {
		case curHint == maxHint:
			return curHint, curHint + maxHint
		case curHint > maxHint:
			return maxHint, curHint
		default:
			return curHint, maxHint
		}
	case modeNeedOnly: // 缺 X  / 等 X
		return 1, 1 + maxHint
	case modeNeedSplit: // X 缺 Y / X 等 Y
		cur = curHint - maxHint
		if cur < 0 {
			cur = 0
		}
		return cur, curHint
	case modeWait: // X 等 Y  (已 X 人等 Y 人)
		return curHint, curHint + maxHint
	default:
		return 0, 0
	}
}

//
// ===== 正则与规则表 =====
//

// 统一的「数字」片段（支持阿拉伯 + 中文）
// (?P<n>...) 语法不被 Go 原生支持；因此只用捕获顺序，不用命名组。
const num = `(\d+|一|二|两|三|四|五|六|七|八|九|十)`

// 各种语义标签，用来告诉 parser 应怎样解释两个数字
type parseMode int

const (
	modePairSlash parseMode = iota
	modePairDash
	modePairEqual
	modeNeedSplit
	modeNeedOnly
	modeWait
	modeKeyword
)

// rule 精简为：正则 + 模式标签
type rule struct {
	re   *regexp.Regexp
	mode parseMode
}

// 前缀片段：允许游戏名等任意前缀，但前缀不得以 `-`、数字或空白结尾，
// 防止 "-1=5" 里的负号被当作前缀吞掉后误解析成 1=5
const prefix = `^(?:.*[^-\d\s])?\s*`

// 规则初始化（包级变量会在 init 前执行一次）
var rules = []rule{
	// 带前缀的格式（游戏名称 + 数字格式）- 优先匹配
	{regexp.MustCompile(prefix + num + `\s*=\s*` + num + `\s*$`), modePairEqual},
	{regexp.MustCompile(prefix + num + `\s*/\s*` + num + `\s*$`), modePairSlash},
	{regexp.MustCompile(prefix + num + `\s*-\s*` + num + `\s*$`), modePairDash},

	// 纯数字格式（保持向后兼容）
	{regexp.MustCompile(`^\s*` + num + `\s*=\s*` + num + `\s*$`), modePairEqual},
	{regexp.MustCompile(`^\s*` + num + `\s*/\s*` + num + `\s*$`), modePairSlash},
	{regexp.MustCompile(`^\s*` + num + `\s*-\s*` + num + `\s*$`), modePairDash},

	// X人Y人
	{regexp.MustCompile(`^\s*` + num + `\s*(?:人|个)\s*` + num + `\s*(?:人|个)?\s*$`), modePairSlash},

	// 带前缀的缺人格式 - X人缺Y / X缺Y 优先于 缺Y
	{regexp.MustCompile(`.*?` + num + `\s*(?:人|个)?\s*(?:缺|等)\s*` + num + `\s*$`), modeNeedSplit},
	{regexp.MustCompile(`.*?(?:缺|等|少)\s*` + num + `\s*$`), modeNeedOnly},

	// X缺Y / X等Y   => 已知总人数 & 缺口
	{regexp.MustCompile(`^\s*` + num + `\s*(?:缺|等)\s*` + num + `\s*$`), modeNeedSplit},

	// 缺X / 等X / 少X
	{regexp.MustCompile(`^\s*(?:缺|等|少)\s*` + num + `\s*$`), modeNeedOnly},

	// X等Y（“已有X人等Y人”）  -> 与上条语义重复，可合并
	{regexp.MustCompile(`^\s*` + num + `\s*等\s*` + num + `\s*$`), modeWait},

	// 纯文本寻组
	{regexp.MustCompile(`(?i)(寻组|找队友|组队|lfg|looking for group|求组|来人|缺人|招人)`), modeKeyword},
}

//
// ===== 核心入口 =====
//

// Parse 解析 LFG 字符串
func Parse(message string) (*LFGMessage, error) {
	orig := strings.TrimSpace(message)
	if orig == "" {
		return nil, fmt.Errorf("消息为空")
	}
	if msg := match(orig); msg != nil {
		msg.OriginalText = orig
		return msg, nil
	}

	// 二次：全角 → 半角 + 去噪音
	normed := normalize(orig)
	if normed != orig {
		if msg := match(normed); msg != nil {
			msg.OriginalText = orig
			return msg, nil
		}
	}

	return nil, ErrUnrecognized
}

// match 遍历 rule，命中即返回 LFGMessage；未命中返回 nil
func match(s string) *LFGMessage {
	for _, r := range rules {
		m := r.re.FindStringSubmatch(s)
		if len(m) == 0 {
			continue
		}

		switch r.mode {
		case modeKeyword:
			return &LFGMessage{CurrentPlayers: 1, MaxPlayers: 5}
		default:
			// 规则都会捕获两个数字；有的可能只用到其中一个
			n1, ok1 := toInt(m[1])
			n2, ok2 := 0, true
			if len(m) > 2 {
				n2, ok2 = toInt(m[2])
			}
			if !ok1 || !ok2 {
				continue
			}

			// 对于只有一个数字的 modeNeedOnly，数字应该作为 maxHint
			var cur, max int
			if r.mode == modeNeedOnly && len(m) == 2 {
				cur, max = calc(0, n1, r.mode)
			} else {
				cur, max = calc(n1, n2, r.mode)
			}

			if max > 0 && cur >= 0 && cur <= max {
				return &LFGMessage{CurrentPlayers: cur, MaxPlayers: max}
			}
		}
	}
	return nil
}

//
// ===== normalize 与 util =====
//

func normalize(s string) string {
	s = halfWidth(s)
	var out []rune
	for _, r := range s {
		switch {
		case unicode.IsNumber(r), unicode.IsLetter(r),
			strings.ContainsRune("=/- ", r),
			unicode.Is(unicode.Han, r):
			out = append(out, r)
		default:
			out = append(out, ' ')
		}
	}
	res := strings.Join(strings.Fields(string(out)), " ")
	// 去“人/个”等无信息字
	return strings.NewReplacer("人", "", "个", "").Replace(res)
}

func halfWidth(s string) string {
	t := transform.Chain(norm.NFKC)
	r, _, err := transform.String(t, s)
	if err != nil {
		return s
	}
	return r
}

//
// ===== 示例 =====
//

func SupportedExamples() []string {
	return []string{
		"5=1   → 五人队，已 1 人（还差 4）",
		"英雄联盟 4=1  → 四人队，已 1 人",
		"美服排位4=1  → 四人队，已 1 人",
		"4缺1  → 四人队，已 3 人",
		"缺2   → 当前 1 人，还差 2",
		"1/5   → 已 1 人，总 5 人",
		"lfg   → 文字寻组（默认 5 人队）",
	}
}
