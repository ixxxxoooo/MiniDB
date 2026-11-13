package schemaindex

import (
	"sort"
	"strings"
)

// Suggest 给一个不存在的表名推荐最可能想要的表：先看拼写相近的，再看检索相关的。
func (s *Searcher) Suggest(name string, n int) []string {
	name = strings.ToLower(strings.TrimSpace(name))
	type cand struct {
		name string
		dist int
	}
	var near []cand
	for _, t := range s.schema.Tables {
		low := strings.ToLower(t.Name)
		d := editDistance(name, low)
		if d <= 2 || strings.Contains(low, name) || strings.Contains(name, low) {
			near = append(near, cand{t.Name, d})
		}
	}
	sort.SliceStable(near, func(i, j int) bool { return near[i].dist < near[j].dist })
	var out []string
	seen := map[string]bool{}
	for _, c := range near {
		if !seen[c.name] && len(out) < n {
			seen[c.name] = true
			out = append(out, c.name)
		}
	}
	for _, h := range s.Tables([]string{name}, n) {
		if !seen[h.Name] && len(out) < n {
			seen[h.Name] = true
			out = append(out, h.Name)
		}
	}
	return out
}

// editDistance 两个字符串的 Levenshtein 距离。
func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}
