package updater

import (
	"strconv"
	"strings"
)

func compareVersions(a, b string) int {
	aParts := numericVersionParts(a)
	bParts := numericVersionParts(b)
	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}
	for i := 0; i < maxLen; i++ {
		av, bv := 0, 0
		if i < len(aParts) {
			av = aParts[i]
		}
		if i < len(bParts) {
			bv = bParts[i]
		}
		if av > bv {
			return 1
		}
		if av < bv {
			return -1
		}
	}
	return 0
}

func numericVersionParts(v string) []int {
	v = strings.TrimSpace(strings.TrimPrefix(v, "v"))
	v = strings.SplitN(v, "-", 2)[0]
	parts := strings.Split(v, ".")
	nums := make([]int, 0, len(parts))
	for _, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil {
			nums = append(nums, 0)
			continue
		}
		nums = append(nums, n)
	}
	return nums
}
