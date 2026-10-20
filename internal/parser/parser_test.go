package parser

import (
	"testing"
)

func TestParser_Parse(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		expected    *LFGMessage
	}{
		{
			name:        "标准格式 - 5=1",
			input:       "5=1",
			expectError: false,
			expected: &LFGMessage{
				MaxPlayers:     5,
				CurrentPlayers: 1,
				OriginalText:   "5=1",
			},
		},
		{
			name:        "特殊格式 - 1=1 (当前1人等1人)",
			input:       "1=1",
			expectError: false,
			expected: &LFGMessage{
				MaxPlayers:     2,
				CurrentPlayers: 1,
				OriginalText:   "1=1",
			},
		},
		{
			name:        "反向格式 - 1=5 (当前1人，队伍5人)",
			input:       "1=5",
			expectError: false,
			expected: &LFGMessage{
				MaxPlayers:     5,
				CurrentPlayers: 1,
				OriginalText:   "1=5",
			},
		},
		{
			name:        "斜杠格式 - 3/5",
			input:       "3/5",
			expectError: false,
			expected: &LFGMessage{
				MaxPlayers:     5,
				CurrentPlayers: 3,
				OriginalText:   "3/5",
			},
		},
		{
			name:        "横杠格式 - 2-5",
			input:       "2-5",
			expectError: false,
			expected: &LFGMessage{
				MaxPlayers:     5,
				CurrentPlayers: 2,
				OriginalText:   "2-5",
			},
		},
		{
			name:        "纯文字LFG - 寻组",
			input:       "寻组",
			expectError: false,
			expected: &LFGMessage{
				MaxPlayers:     5,
				CurrentPlayers: 1,
				OriginalText:   "寻组",
			},
		},
		{
			name:        "纯文字LFG - 找队友",
			input:       "找队友",
			expectError: false,
			expected: &LFGMessage{
				MaxPlayers:     5,
				CurrentPlayers: 1,
				OriginalText:   "找队友",
			},
		},
		{
			name:        "纯文字LFG - lfg",
			input:       "lfg",
			expectError: false,
			expected: &LFGMessage{
				MaxPlayers:     5,
				CurrentPlayers: 1,
				OriginalText:   "lfg",
			},
		},
		{
			name:        "纯文字LFG - 组队",
			input:       "组队",
			expectError: false,
			expected: &LFGMessage{
				MaxPlayers:     5,
				CurrentPlayers: 1,
				OriginalText:   "组队",
			},
		},
		{
			name:        "纯文字LFG - 来人",
			input:       "来人",
			expectError: false,
			expected: &LFGMessage{
				MaxPlayers:     5,
				CurrentPlayers: 1,
				OriginalText:   "来人",
			},
		},
		{
			name:        "带空格的格式",
			input:       "5 = 1",
			expectError: false,
			expected: &LFGMessage{
				MaxPlayers:     5,
				CurrentPlayers: 1,
				OriginalText:   "5 = 1",
			},
		},
		{
			name:        "全角字符",
			input:       "５＝１",
			expectError: false,
			expected: &LFGMessage{
				MaxPlayers:     5,
				CurrentPlayers: 1,
				OriginalText:   "５＝１",
			},
		},
		{
			name:        "大数字也支持 - 100=50",
			input:       "100=50",
			expectError: false,
			expected: &LFGMessage{
				MaxPlayers:     100,
				CurrentPlayers: 50,
				OriginalText:   "100=50",
			},
		},
		{
			name:        "当前人数大于最大人数也支持 - 10=5",
			input:       "10=5",
			expectError: false,
			expected: &LFGMessage{
				MaxPlayers:     10,
				CurrentPlayers: 5,
				OriginalText:   "10=5",
			},
		},
		{
			name:        "复杂格式 - 带中文单位",
			input:       "5个=1个",
			expectError: false,
			expected: &LFGMessage{
				MaxPlayers:     5,
				CurrentPlayers: 1,
				OriginalText:   "5个=1个",
			},
		},
		// 新增：带前缀的格式测试
		{
			name:        "带前缀格式 - 英雄联盟 4=1",
			input:       "英雄联盟 4=1",
			expectError: false,
			expected: &LFGMessage{
				MaxPlayers:     4,
				CurrentPlayers: 1,
				OriginalText:   "英雄联盟 4=1",
			},
		},
		{
			name:        "带前缀格式 - 美服排位4=1",
			input:       "美服排位4=1",
			expectError: false,
			expected: &LFGMessage{
				MaxPlayers:     4,
				CurrentPlayers: 1,
				OriginalText:   "美服排位4=1",
			},
		},
		{
			name:        "带前缀格式 - 单双 2=2",
			input:       "单双 2=2",
			expectError: false,
			expected: &LFGMessage{
				MaxPlayers:     4,
				CurrentPlayers: 2,
				OriginalText:   "单双 2=2",
			},
		},
		{
			name:        "缺人格式 - 缺2",
			input:       "缺2",
			expectError: false,
			expected: &LFGMessage{
				MaxPlayers:     3,
				CurrentPlayers: 1,
				OriginalText:   "缺2",
			},
		},
		{
			name:        "缺人格式 - 5人缺1",
			input:       "5人缺1",
			expectError: false,
			expected: &LFGMessage{
				MaxPlayers:     5,
				CurrentPlayers: 4,
				OriginalText:   "5人缺1",
			},
		},
		{
			name:        "缺人格式 - 4缺1",
			input:       "4缺1",
			expectError: false,
			expected: &LFGMessage{
				MaxPlayers:     4,
				CurrentPlayers: 3,
				OriginalText:   "4缺1",
			},
		},
		{
			name:        "等人格式 - 等3",
			input:       "等3",
			expectError: false,
			expected: &LFGMessage{
				MaxPlayers:     4,
				CurrentPlayers: 1,
				OriginalText:   "等3",
			},
		},
		{
			name:        "中文数字 - 缺二",
			input:       "缺二",
			expectError: false,
			expected: &LFGMessage{
				MaxPlayers:     3,
				CurrentPlayers: 1,
				OriginalText:   "缺二",
			},
		},
		{
			name:        "带前缀缺人 - 排位缺1",
			input:       "排位缺1",
			expectError: false,
			expected: &LFGMessage{
				MaxPlayers:     2,
				CurrentPlayers: 1,
				OriginalText:   "排位缺1",
			},
		},
		{
			name:        "无效格式 - 空字符串",
			input:       "",
			expectError: true,
			expected:    nil,
		},
		{
			name:        "无效格式 - 纯随机文字",
			input:       "今天天气真好",
			expectError: true,
			expected:    nil,
		},
		{
			name:        "无效格式 - 负数",
			input:       "-1=5",
			expectError: true,
			expected:    nil,
		},
		{
			name:        "无效格式 - 带游戏名前缀的负数",
			input:       "LOL -1=5",
			expectError: true,
			expected:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("期望出现错误，但没有错误")
				}
				return
			}

			if err != nil {
				t.Errorf("意外错误: %v", err)
				return
			}

			if result == nil {
				t.Errorf("结果为nil")
				return
			}

			if result.MaxPlayers != tt.expected.MaxPlayers {
				t.Errorf("MaxPlayers 不匹配: 期望 %d, 实际 %d", tt.expected.MaxPlayers, result.MaxPlayers)
			}

			if result.CurrentPlayers != tt.expected.CurrentPlayers {
				t.Errorf("CurrentPlayers 不匹配: 期望 %d, 实际 %d", tt.expected.CurrentPlayers, result.CurrentPlayers)
			}

			if result.OriginalText != tt.expected.OriginalText {
				t.Errorf("OriginalText 不匹配: 期望 %s, 实际 %s", tt.expected.OriginalText, result.OriginalText)
			}
		})
	}
}

// 基准测试
func BenchmarkParser_Parse(b *testing.B) {
	input := "5=1"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Parse(input)
	}
}
