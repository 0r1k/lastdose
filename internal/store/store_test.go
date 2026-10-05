package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lastdose/internal/secret"
)

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "state.json")
	s, err := Open(path, nil)
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

	s2, err := Open(path, nil)
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

func TestHandEditedStartIsDiscarded(t *testing.T) {
	key, _ := secret.NewKey()
	path := filepath.Join(t.TempDir(), "state.json")
	s, _ := Open(path, key)
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if err := s.Update(func(st *State) {
		st.Habits[Smoking] = &HabitState{StartedAt: start}
		st.Habits[Alcohol] = &HabitState{StartedAt: start}
	}); err != nil {
		t.Fatal(err)
	}

	// An honest reopen keeps everything.
	if s2, _ := Open(path, key); len(s2.Tampered()) != 0 {
		t.Fatalf("untouched file reported as tampered: %v", s2.Tampered())
	}

	// Move the smoking start six years back by hand.
	b, _ := os.ReadFile(path)
	edited := strings.Replace(string(b), "2026-10-01", "2020-10-01", 1)
	if edited == string(b) {
		t.Fatal("test setup: date not found")
	}
	os.WriteFile(path, []byte(edited), 0o644)

	s3, _ := Open(path, key)
	if got := s3.Tampered(); len(got) != 1 {
		t.Fatalf("tampered = %v, want exactly one habit", got)
	}
	s3.View(func(st *State) {
		if len(st.Habits) != 1 {
			t.Fatalf("edited habit must be discarded, have %d habits", len(st.Habits))
		}
	})

	// A different key (another build) cannot vouch for this file either.
	other, _ := secret.NewKey()
	if s4, _ := Open(path, other); len(s4.Tampered()) == 0 {
		t.Fatal("signature from another key accepted")
	}
}
