// Package store persists tracker state as JSON. Elapsed time is always derived
// from the stored start moment, so nothing has to run in the background:
// reopening the app just computes now - start. All times stored here come
// from the network clock, never from the system clock.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"lastdose/internal/secret"
)

// Habit identifies a tracked bad habit.
type Habit string

const (
	Smoking Habit = "smoking"
	Alcohol Habit = "alcohol"
)

// Habits is the display order of supported habits.
var Habits = []Habit{Smoking, Alcohol}

// HabitState is the tracking state of one habit.
type HabitState struct {
	StartedAt time.Time `json:"started_at"`
	// Acknowledged is how many unlock popups the user has already seen.
	Acknowledged int  `json:"acknowledged"`
	FinalShown   bool `json:"final_shown"`
	// Sig signs the habit and its start moment, so the start cannot be moved
	// back by editing the file. Empty in builds without the release key.
	Sig string `json:"sig,omitempty"`
}

// Elapsed returns time clean at now, never negative.
func (h *HabitState) Elapsed(now time.Time) time.Duration {
	d := now.Sub(h.StartedAt)
	if d < 0 {
		return 0
	}
	return d
}

// State is everything written to disk.
type State struct {
	Timezone string                `json:"timezone"`
	Habits   map[Habit]*HabitState `json:"habits"`
	// LastSeen is the last moment the app was known to be running.
	LastSeen time.Time `json:"last_seen"`
}

// Store guards State with a mutex so signal handlers can flush it safely.
type Store struct {
	mu    sync.Mutex
	path  string
	state State
	// key signs habits; nil disables signing and verification.
	key      []byte
	tampered []Habit
}

// DefaultPath returns $XDG_CONFIG_HOME/lastdose/state.json (or the OS analogue).
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "lastdose", "state.json"), nil
}

// Open loads state from path; a missing file yields empty state. With a key,
// habits whose signature does not match are discarded, see Tampered.
func Open(path string, key []byte) (*Store, error) {
	s := &Store{path: path, key: key}
	b, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		if err := json.Unmarshal(b, &s.state); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	}
	if s.state.Habits == nil {
		s.state.Habits = map[Habit]*HabitState{}
	}
	if key != nil {
		for _, h := range Habits {
			hs := s.state.Habits[h]
			if hs != nil && !secret.Verify(key, sigData(h, hs), hs.Sig) {
				delete(s.state.Habits, h)
				s.tampered = append(s.tampered, h)
			}
		}
	}
	return s, nil
}

func sigData(h Habit, hs *HabitState) string {
	return fmt.Sprintf("%s|%d", h, hs.StartedAt.UnixNano())
}

// Tampered lists habits dropped on Open because the file was edited by hand.
func (s *Store) Tampered() []Habit { return s.tampered }

// Path returns the file backing the store.
func (s *Store) Path() string { return s.path }

// View runs f with read access to the state.
func (s *Store) View(f func(st *State)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f(&s.state)
}

// Update mutates state and saves it immediately.
func (s *Store) Update(f func(st *State)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f(&s.state)
	return s.saveLocked()
}

// Touch records now as the last moment the app was alive and saves.
func (s *Store) Touch(now time.Time) error {
	return s.Update(func(st *State) { st.LastSeen = now.UTC() })
}

func (s *Store) saveLocked() error {
	if s.key != nil {
		for h, hs := range s.state.Habits {
			hs.Sig = secret.Sign(s.key, sigData(h, hs))
		}
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	// Write-then-rename so a crash mid-write never corrupts the file.
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".state-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}
