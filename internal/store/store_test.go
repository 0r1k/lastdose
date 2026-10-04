package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "state.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := s.Update(func(st *State) {
		st.Timezone = "Europe/Moscow"
		st.Habits[Smoking] = &HabitState{StartedAt: start, Acknowledged: 2}
	}); err != nil {
		t.Fatal(err)
	}
	seen := start.Add(time.Hour)
	if err := s.Touch(seen); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s2.View(func(st *State) {
		h := st.Habits[Smoking]
		if st.Timezone != "Europe/Moscow" || h == nil || !h.StartedAt.Equal(start) || h.Acknowledged != 2 {
			t.Fatalf("state not restored: %+v %+v", st, h)
		}
		if !st.LastSeen.Equal(seen) {
			t.Fatalf("last seen = %v, want %v", st.LastSeen, seen)
		}
	})
}

func TestElapsedNeverNegative(t *testing.T) {
	h := HabitState{StartedAt: time.Now().Add(time.Hour)}
	if e := h.Elapsed(time.Now()); e != 0 {
		t.Fatalf("elapsed = %v, want 0", e)
	}
}
