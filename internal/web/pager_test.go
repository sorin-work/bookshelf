package web

import (
	"strconv"
	"testing"
)

func TestPager(t *testing.T) {
	link := func(n int) string { return "/x?page=" + strconv.Itoa(n) }
	tests := []struct {
		page, total int
		prev, next  string
	}{
		{1, 0, "", ""},
		{1, 10, "", ""},
		{1, 11, "", "/x?page=2"},
		{2, 11, "/x?page=1", ""},
		{2, 30, "/x?page=1", "/x?page=3"},
	}
	for _, tt := range tests {
		p := newPager(tt.page, 10, tt.total, link)
		if p.PrevURL != tt.prev || p.NextURL != tt.next {
			t.Errorf("page %d of %d: %+v", tt.page, tt.total, p)
		}
	}
}
