package domain

import "strings"

// leadingArticles are skipped when sorting titles, the way library shelves
// file "The Hobbit" under H.
var leadingArticles = []string{"the ", "a ", "an "}

// SortTitle is the key a title is sorted by: lower case, without a
// leading article.
func SortTitle(title string) string {
	t := strings.ToLower(strings.TrimSpace(title))
	for _, a := range leadingArticles {
		if strings.HasPrefix(t, a) && len(t) > len(a) {
			return strings.TrimSpace(t[len(a):])
		}
	}
	return t
}
