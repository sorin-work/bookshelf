package store

import (
	"context"
	"errors"
	"testing"

	"github.com/sorin-work/bookshelf/internal/domain"
)

func TestFirstUserIsLibrarianAndCards(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	today := domain.DateOf(now)
	first, err := s.CreateUser(ctx, NewUser{Email: "a@example.com", Name: "A", Role: RoleMember, CardExpires: today}, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.CreateUser(ctx, NewUser{Email: "b@example.com", Name: "B", CardExpires: today}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !first.IsLibrarian() || second.IsLibrarian() {
		t.Errorf("roles %s, %s", first.Role, second.Role)
	}
	if first.CardNumber != domain.CardNumber(first.ID) || second.CardNumber == first.CardNumber {
		t.Errorf("cards %s, %s", first.CardNumber, second.CardNumber)
	}
	if _, err := s.CreateUser(ctx, NewUser{Email: "A@EXAMPLE.com", Name: "dup"}, now); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate email: %v", err)
	}
	byCard, err := s.UserByCard(ctx, second.CardNumber)
	if err != nil || byCard.ID != second.ID {
		t.Errorf("by card: %+v, %v", byCard, err)
	}
}

func TestListMembers(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for _, u := range []NewUser{
		{Email: "ioana@example.com", Name: "Ioana Marin"},
		{Email: "mihai@example.com", Name: "Mihai Popa"},
		{Email: "ana.pop@example.com", Name: "Ana"},
	} {
		if _, err := s.CreateUser(ctx, u, now); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		q    string
		want int
	}{
		{"", 3},
		{"pop", 2}, // name and email
		{"MARIN", 1},
		{"2000 0000", 3},
		{"nobody", 0},
	}
	for _, tt := range tests {
		got, total, err := s.ListMembers(ctx, MemberFilter{Query: tt.q})
		if err != nil {
			t.Fatal(err)
		}
		if total != tt.want || len(got) != tt.want {
			t.Errorf("%q: %d results, want %d", tt.q, total, tt.want)
		}
	}
	page, total, _ := s.ListMembers(ctx, MemberFilter{Limit: 2, Offset: 2})
	if total != 3 || len(page) != 1 || page[0].Name != "Mihai Popa" {
		t.Errorf("second page %+v", page)
	}
}

func TestSetActive(t *testing.T) {
	f := newHoldFixture(t)
	ctx := context.Background()
	h, err := f.s.PlaceHold(ctx, f.ana.ID, f.titleID, f.today, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.s.SetActive(ctx, f.member.ID, false, f.today, now); !errors.Is(err, ErrHasLoans) {
		t.Errorf("closing with a loan out: %v", err)
	}
	if err := f.s.SetActive(ctx, f.ana.ID, false, f.today, now); err != nil {
		t.Fatal(err)
	}
	hold, _ := f.s.HoldByID(ctx, h.ID)
	if hold.Status != domain.HoldCancelled {
		t.Errorf("hold %s after closing the account", hold.Status)
	}
	u, _ := f.s.UserByID(ctx, f.ana.ID)
	if u.Active || u.CanLogIn() {
		t.Errorf("account still active: %+v", u)
	}
	if err := f.s.SetActive(ctx, f.ana.ID, true, f.today, now); err != nil {
		t.Fatal(err)
	}
	if err := f.s.SetActive(ctx, 999, true, f.today, now); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing member: %v", err)
	}
}
