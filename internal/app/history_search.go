package app

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// HistorySearch 是历史搜索的条件。
type HistorySearch struct {
	Text      string // 搜索词；Regex 为真时是正则
	Regex     bool   // 把 Text 当正则表达式
	Case      bool   // 区分大小写（默认不区分）
	Word      bool   // 整词匹配
	Favorites bool   // 只要收藏的
	Limit     int    // 最多返回几条，<=0 不限
}

// ErrHistoryPattern 表示搜索用的正则不合法。
var ErrHistoryPattern = errors.New("invalid history search pattern")

// compile 把搜索条件编译成一个匹配器；搜索词为空返回 nil（全部匹配）。正则只编译一次，不在每条历史上重编。
func (s HistorySearch) compile() (*regexp.Regexp, error) {
	text := strings.TrimSpace(s.Text)
	if text == "" {
		return nil, nil
	}
	pat := text
	if !s.Regex {
		pat = regexp.QuoteMeta(text)
	}
	if s.Word {
		pat = `\b(?:` + pat + `)\b`
	}
	if !s.Case {
		pat = "(?i)" + pat
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrHistoryPattern, err)
	}
	return re, nil
}

// Search 按条件搜索连接的历史，新的在前。扫描该连接的全部历史，再按 Limit 截断，
// 所以收藏过滤、正则搜索都不会因为只看最近 N 条而漏掉旧的记录。
func (h *HistoryService) Search(connID string, s HistorySearch) ([]HistoryItem, error) {
	re, err := s.compile()
	if err != nil {
		return nil, err
	}
	items, err := h.store.ListPrefix(historyBucket, connID+"\x00", true, func() interface{} { return &HistoryItem{} })
	if err != nil {
		return nil, err
	}
	var out []HistoryItem
	for _, it := range items {
		item := *it.(*HistoryItem)
		if s.Favorites && !item.Favorite {
			continue
		}
		if re != nil && !re.MatchString(item.SQL) && !re.MatchString(item.Name) {
			continue
		}
		out = append(out, item)
		if s.Limit > 0 && len(out) >= s.Limit {
			break
		}
	}
	return out, nil
}
