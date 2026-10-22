package service

import (
	"testing"

	"github.com/novian001/helpdesk-api/internal/models"
)

// totalPages mirrors the arithmetic used by the ticket list query. It is
// extracted here so the pagination rules can be tested without a database.
func totalPages(total, limit int) int {
	if total <= 0 || limit <= 0 {
		return 0
	}
	return (total + limit - 1) / limit
}

func TestTotalPages(t *testing.T) {
	cases := []struct {
		total, limit, want int
	}{
		{0, 20, 0},
		{1, 20, 1},
		{20, 20, 1},
		{21, 20, 2},
		{100, 20, 5},
		{7, 3, 3},
	}
	for _, c := range cases {
		if got := totalPages(c.total, c.limit); got != c.want {
			t.Fatalf("totalPages(%d, %d) = %d, want %d", c.total, c.limit, got, c.want)
		}
	}
}

func TestValidStatusAndPriority(t *testing.T) {
	for _, s := range []string{models.StatusOpen, models.StatusInProgress, models.StatusResolved, models.StatusClosed} {
		if !validStatus(s) {
			t.Fatalf("%q must be a valid status", s)
		}
	}
	if validStatus("DONE") {
		t.Fatal("DONE must not be a valid status")
	}
	for _, p := range []string{models.PriorityLow, models.PriorityMedium, models.PriorityHigh, models.PriorityCritical} {
		if !validPriority(p) {
			t.Fatalf("%q must be a valid priority", p)
		}
	}
	if validPriority("URGENT") {
		t.Fatal("URGENT must not be a valid priority")
	}
}
