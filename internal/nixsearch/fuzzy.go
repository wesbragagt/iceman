package nixsearch

import (
	"sort"
	"strings"
)

const (
	scoreMatch      = 16
	scoreContiguous = 12
	scoreWordStart  = 8
	scoreGapPenalty = 1
	scoreExact      = 200
	scorePrefix     = 60
)

// FuzzyMatch scores query against target the way fzy/fzf do: every query
// character must appear in target in order (case-insensitive). Contiguous runs
// and matches near the start of the target score higher. ok is false when the
// query is not a subsequence of the target.
func FuzzyMatch(query, target string) (int, bool) {
	if query == "" {
		return 0, true
	}
	q := strings.ToLower(query)
	t := strings.ToLower(target)

	score := 0
	ti := 0
	prev := -2
	for qi := 0; qi < len(q); qi++ {
		idx := strings.IndexByte(t[ti:], q[qi])
		if idx < 0 {
			return 0, false
		}
		pos := ti + idx
		score += scoreMatch
		if pos == prev+1 {
			score += scoreContiguous
		}
		if pos == 0 || isBoundary(t[pos-1]) {
			score += scoreWordStart
		}
		// Later matches are worth less, so earlier hits float to the top.
		score -= pos * scoreGapPenalty
		prev = pos
		ti = pos + 1
	}

	if t == q {
		score += scoreExact
	} else if strings.HasPrefix(t, q) {
		score += scorePrefix
	}
	// Shorter targets are a tighter fit for the same query.
	score -= len(t) / 8
	return score, true
}

func isBoundary(c byte) bool {
	return c == '-' || c == '_' || c == '.' || c == ' ' || c == '/'
}

// MatchPositions returns the byte indices in target where query's characters
// matched, in the same greedy left-to-right scan FuzzyMatch uses, so a caller
// can highlight the matched characters. Returns nil if query doesn't match.
func MatchPositions(query, target string) []int {
	if query == "" {
		return nil
	}
	q := strings.ToLower(query)
	t := strings.ToLower(target)

	positions := make([]int, 0, len(q))
	ti := 0
	for qi := 0; qi < len(q); qi++ {
		idx := strings.IndexByte(t[ti:], q[qi])
		if idx < 0 {
			return nil
		}
		pos := ti + idx
		positions = append(positions, pos)
		ti = pos + 1
	}
	return positions
}

// RankPackages returns the packages matching query, best first. A name match
// always outranks a description-only match; descriptions only contribute a
// small tiebreak. limit <= 0 means no limit.
func RankPackages(query string, pkgs []Package, limit int) []Package {
	type scored struct {
		pkg   Package
		score int
	}
	var out []scored
	for _, p := range pkgs {
		nameScore, nameOK := FuzzyMatch(query, p.PName)
		descScore, descOK := FuzzyMatch(query, p.Description)

		var total int
		switch {
		case nameOK:
			// Shorter names win ties; the description is only a faint nudge so
			// it can never reorder two comparable name matches.
			total = 1_000_000 + nameScore*8 - len(p.PName)
			if descOK {
				total += descScore / 64
			}
		case descOK && query != "":
			total = descScore
		default:
			continue
		}
		out = append(out, scored{p, total})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return out[i].pkg.AttrPath < out[j].pkg.AttrPath
	})

	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	res := make([]Package, len(out))
	for i, s := range out {
		res[i] = s.pkg
	}
	return res
}
