package lottery

import "testing"

func TestPickWinnerIndices(t *testing.T) {
	cases := []struct {
		name        string
		n           int
		winnerCount int
		wantLen     int
	}{
		{"正常抽取", 10, 3, 3},
		{"人数等于参与者", 5, 5, 5},
		{"人数超过参与者按参与者封顶", 3, 10, 3},
		{"单人参与", 1, 1, 1},
		{"零参与者", 0, 3, 0},
		{"零中奖名额", 10, 0, 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := pickWinnerIndices(c.n, c.winnerCount)
			if len(got) != c.wantLen {
				t.Fatalf("pickWinnerIndices(%d, %d) 返回 %d 个下标, want %d",
					c.n, c.winnerCount, len(got), c.wantLen)
			}

			// 下标必须无重复且在 [0, n) 范围内
			seen := make(map[int]bool, len(got))
			for _, idx := range got {
				if idx < 0 || idx >= c.n {
					t.Errorf("下标 %d 越界 [0, %d)", idx, c.n)
				}
				if seen[idx] {
					t.Errorf("下标 %d 重复出现", idx)
				}
				seen[idx] = true
			}
		})
	}
}

// TestPickWinnerIndicesRandomness 多次抽取应产生不同结果(防退化为固定顺序)
func TestPickWinnerIndicesRandomness(t *testing.T) {
	const n, k, rounds = 20, 5, 50
	first := pickWinnerIndices(n, k)
	for i := 0; i < rounds; i++ {
		got := pickWinnerIndices(n, k)
		for j := range got {
			if got[j] != first[j] {
				return // 出现过不同排列即可
			}
		}
	}
	t.Errorf("连续 %d 次抽取结果完全相同，随机性疑似失效", rounds)
}
