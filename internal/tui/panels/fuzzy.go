package panels

import "strings"

// fuzzyScore ranks how well a query matches a target as a subsequence,
// reporting false when a query character is missing. Higher is better:
// consecutive characters and matches at word starts score up, while skipped
// characters and long targets score down — so "gu" finds "get user" before
// "generate-upload-url". Comparison is per rune, so non-Latin names work.
func fuzzyScore(target, query string) (int, bool) {
	if query == "" {
		return 0, true
	}
	t := []rune(strings.ToLower(target))

	score, at := 0, 0
	prev := -1
	for _, want := range strings.ToLower(query) {
		found := false
		for ; at < len(t); at++ {
			if t[at] != want {
				continue
			}
			score += 10
			if prev >= 0 {
				if gap := at - prev - 1; gap == 0 {
					score += 15
				} else {
					score -= min(gap, 10)
				}
			}
			if at == 0 || isBoundary(t[at-1]) {
				score += 12
			}
			prev = at
			at++
			found = true
			break
		}
		if !found {
			return 0, false
		}
	}
	// Prefer shorter targets when matches are otherwise equal.
	return score - len(t)/8, true
}

func isBoundary(r rune) bool {
	switch r {
	case ' ', '-', '_', '/', '.', ':':
		return true
	}
	return false
}
