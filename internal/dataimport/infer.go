package dataimport

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Kind 是按样本推断出的列类型。
type Kind string

const (
	KindInt   Kind = "int"
	KindFloat Kind = "float"
	KindText  Kind = "text"
)

// inferSample 是 InferKinds 默认看多少行。
const inferSample = 2000

// InferKinds 读前 limit 行（<=0 取默认值），给每列推断类型：
// 空值不参与判断；全是整数得 int，全是数字得 float，其余为 text。
// 有前导零的整数（如 007）当作文本，它们多半是编号。
func InferKinds(path string, opts Options, limit int) ([]Kind, error) {
	if limit <= 0 {
		limit = inferSample
	}
	r, err := Open(path, opts)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	n := len(r.Columns())
	kinds := make([]Kind, n)
	seen := make([]bool, n)
	for i := 0; i < limit; i++ {
		row, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		for c := 0; c < n && c < len(row); c++ {
			if row[c] == nil {
				continue
			}
			s := strings.TrimSpace(fmt.Sprint(row[c]))
			if s == "" {
				continue
			}
			k := kindOf(s)
			switch {
			case !seen[c]:
				kinds[c], seen[c] = k, true
			case kinds[c] == KindText || k == KindText:
				kinds[c] = KindText
			case kinds[c] == KindFloat || k == KindFloat:
				kinds[c] = KindFloat
			}
		}
	}
	for c := range kinds {
		if !seen[c] {
			kinds[c] = KindText
		}
	}
	return kinds, nil
}

func kindOf(s string) Kind {
	digits := strings.TrimLeft(s, "+-")
	if digits == "" || strings.Trim(digits, "0123456789.eE+-") != "" {
		return KindText
	}
	if _, err := strconv.ParseInt(s, 10, 64); err == nil {
		if len(digits) > 1 && digits[0] == '0' || s[0] == '+' {
			return KindText
		}
		return KindInt
	}
	if len(digits) > 1 && digits[0] == '0' && digits[1] != '.' {
		return KindText
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return KindFloat
	}
	return KindText
}
