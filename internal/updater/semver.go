package updater

import (
	"strconv"
	"strings"
)

// compareVersions 按语义化版本比较 a 与 b：返回 1 / 0 / -1。
// 数字部分逐段比较；同一数字版本下，预发布版本（如 1.2.0-beta.1）低于正式版 1.2.0；
// 构建元数据（+build）忽略。
func compareVersions(a, b string) int {
	aCore, aPre := splitVersion(a)
	bCore, bPre := splitVersion(b)

	aParts := numericVersionParts(aCore)
	bParts := numericVersionParts(bCore)
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
	return comparePrerelease(aPre, bPre)
}

func splitVersion(v string) (core, pre string) {
	v = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(v), "v"))
	if i := strings.Index(v, "+"); i >= 0 {
		v = v[:i]
	}
	if i := strings.Index(v, "-"); i >= 0 {
		return v[:i], v[i+1:]
	}
	return v, ""
}

func numericVersionParts(v string) []int {
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

// comparePrerelease 按 semver 规则比较预发布标识：无预发布 > 有预发布；数字标识按数值比，
// 数字标识 < 字母标识；前缀相同时标识更多者更大。
func comparePrerelease(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return 1
	}
	if b == "" {
		return -1
	}
	aIDs := strings.Split(a, ".")
	bIDs := strings.Split(b, ".")
	for i := 0; i < len(aIDs) && i < len(bIDs); i++ {
		if c := compareIdentifier(aIDs[i], bIDs[i]); c != 0 {
			return c
		}
	}
	switch {
	case len(aIDs) > len(bIDs):
		return 1
	case len(aIDs) < len(bIDs):
		return -1
	}
	return 0
}

func compareIdentifier(a, b string) int {
	an, aErr := strconv.Atoi(a)
	bn, bErr := strconv.Atoi(b)
	switch {
	case aErr == nil && bErr == nil:
		if an > bn {
			return 1
		}
		if an < bn {
			return -1
		}
		return 0
	case aErr == nil:
		return -1
	case bErr == nil:
		return 1
	}
	return strings.Compare(a, b)
}
