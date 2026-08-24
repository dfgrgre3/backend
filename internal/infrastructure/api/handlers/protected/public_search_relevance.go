package protected

import (
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// computeRelevance calculates a simple relevance score (0-100) based on
// how well the query matches the title and description.
// Title matches weigh more than description matches.
func computeRelevance(query, title, description string) int {
	if query == "" {
		return 0
	}

	queryLower := strings.ToLower(query)
	titleLower := strings.ToLower(title)
	descLower := strings.ToLower(description)

	score := 0

	// Exact title match → highest
	if titleLower == queryLower {
		score = 100
	} else if strings.Contains(titleLower, queryLower) {
		// Title starts with query → very high
		if strings.HasPrefix(titleLower, queryLower) {
			score = 90
		} else {
			score = 75
		}
	} else if strings.Contains(descLower, queryLower) {
		// Description match → medium
		score = 50
	} else {
		// Word-level matching for partial relevance
		queryWords := strings.Fields(queryLower)
		matchedWords := 0
		for _, word := range queryWords {
			if len(word) < 2 {
				continue
			}
			if strings.Contains(titleLower, word) {
				matchedWords++
			} else if strings.Contains(descLower, word) {
				matchedWords++
			}
		}
		if len(queryWords) > 0 {
			score = (matchedWords * 30) / len(queryWords)
		}
	}

	// Clamp to 0-100
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	if score == 0 {
		// Ensure non-zero for display when we have a result
		score = 10
	}

	return score
}

// sortResultsByRelevance sorts results in descending order of relevance
// using a simple insertion sort (results slice is small, ≤ 50 items).
func sortResultsByRelevance(results []SearchResultItem) {
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].Relevance > results[j-1].Relevance; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}
}

// Compile-time interface check to ensure the handler signature matches gin.HandlerFunc.
var _ gin.HandlerFunc = PublicSearch

// Ensure gorm is imported (used indirectly through models and db packages).
var _ = gorm.ErrRecordNotFound
