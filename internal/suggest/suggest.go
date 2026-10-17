// Package suggest finds the option a user most likely meant to type.
package suggest

import "strings"

// maxDistance is how many edits a typo may have to still get a suggestion.
const maxDistance = 2

// Closest returns the option nearest to input, ignoring case, if it is at most
// two edits away. Among equally near options the first one wins.
func Closest(input string, options []string) (string, bool) {
	input = strings.ToLower(input)
	if input == "" {
		return "", false
	}

	best, bestDist := "", maxDistance+1
	for _, option := range options {
		if d := distance(input, strings.ToLower(option)); d < bestDist {
			best, bestDist = option, d
		}
	}
	return best, bestDist <= maxDistance
}

// distance is the Damerau-Levenshtein distance (optimal string alignment), so
// that a swap of two neighbouring letters counts as one typo.
func distance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	rows := make([][]int, len(ra)+1)
	for i := range rows {
		rows[i] = make([]int, len(rb)+1)
		rows[i][0] = i
	}
	for j := range rows[0] {
		rows[0][j] = j
	}

	for i := 1; i <= len(ra); i++ {
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			d := min(rows[i-1][j]+1, rows[i][j-1]+1, rows[i-1][j-1]+cost)
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				d = min(d, rows[i-2][j-2]+1)
			}
			rows[i][j] = d
		}
	}
	return rows[len(ra)][len(rb)]
}
