package ui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lastdose/internal/achievements"
	"lastdose/internal/netclock"
	"lastdose/internal/store"
)

// fakeNet stands in for the network time servers.
type fakeNet struct {
	t    time.Time
	down bool
}

func (f *fakeNet) source(context.Context) (netclock.Sample, error) {
	if f.down {
		return netclock.Sample{}, errors.New("offline")
	}
	return netclock.Sample{Net: f.t, Local: time.Now(), Precise: true, Source: "fake"}, nil
}

type env struct {
	t     *testing.T
	path  string
	net   *fakeNet
	clock *netclock.Clock
}

func newEnv(t *testing.T) *env {
	f := &fakeNet{t: time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)}
	return &env{t: t, path: filepath.Join(t.TempDir(), "state.json"), net: f, clock: netclock.New(f.source)}
}

// open starts the app like a fresh launch and feeds it the sync result.
func (e *env) open() Model {
	e.t.Helper()
	st, err := store.Open(e.path)
	if err != nil {
		e.t.Fatal(err)
	}
	m := New(Options{
		Store:        st,
		Clock:        e.clock,
		LookupTZ:     func(context.Context) (string, error) { return "Europe/Berlin", nil },
		FinalMessage: "MY OWN TEXT",
	})
	return e.sync(m)
}

// sync advances the network clock to e.net.t and delivers the result.
func (e *env) sync(m Model) Model {
	e.t.Helper()
	err := e.clock.Sync(context.Background())
	msg := syncMsg{err: err}
	if !m.synced && err == nil {
		msg.zone = "Europe/Berlin"
	}
	next, _ := m.Update(msg)
	next, _ = next.(Model).Update(tickMsg{})
	return next.(Model)
}

func press(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	return m
}

func TestOfflineBlocksStart(t *testing.T) {
	e := newEnv(t)
	e.net.down = true
	m := e.open()
	if m.synced || m.screen != scrSync {
		t.Fatalf("must wait for network time, screen %v", m.screen)
	}
	if !strings.Contains(m.View(), "could not reach any time server") {
		t.Fatal("offline screen not shown")
	}
	e.net.down = false
	m = e.sync(m)
	if !m.synced || m.screen != scrTZ {
		t.Fatalf("after network is back expected tz screen, got %v", m.screen)
	}
	if m.tzOptions[0].zone != "Europe/Berlin" || m.tzOptions[0].tag != "detected online" {
		t.Fatalf("online zone must be the default: %+v", m.tzOptions[0])
	}
}

func TestFullJourney(t *testing.T) {
	e := newEnv(t)
	m := e.open()

	// Default (online) zone, then smoking, then confirm start.
	m = press(t, m, "enter")
	if m.screen != scrHabit || m.tzName != "Europe/Berlin" {
		t.Fatalf("after tz: screen %v zone %q", m.screen, m.tzName)
	}
	m = press(t, m, "enter", "enter")
	if m.screen != scrDash || m.active != store.Smoking {
		t.Fatalf("tracking did not start: screen %v active %q", m.screen, m.active)
	}
	hs, _ := m.habit(store.Smoking)
	if d := hs.StartedAt.Sub(e.net.t); d < 0 || d > time.Second {
		t.Fatalf("start must use network time: %v vs %v", hs.StartedAt, e.net.t)
	}

	// Close the app; reopen 3 days later by network time.
	e.net.t = e.net.t.Add(3*24*time.Hour + time.Minute)
	m = e.open()
	if m.screen != scrDash || !strings.Contains(m.notice, "3d 0h ago") {
		t.Fatalf("reopen: screen %v notice %q", m.screen, m.notice)
	}
	if h, idx, ok := m.popup(); !ok || h != store.Smoking || idx != 0 {
		t.Fatalf("expected Day One popup, got %v %v %v", h, idx, ok)
	}
	m = press(t, m, "enter", "enter")
	if _, _, ok := m.popup(); ok {
		t.Fatal("no popups expected after acknowledging both")
	}

	// Jump past the last milestone: accept all, then the custom finale shows.
	e.net.t = e.net.t.Add(achievements.All[len(achievements.All)-1].Threshold)
	m = e.sync(m)
	m = press(t, m, "s")
	next, _ := m.Update(tickMsg{})
	m = next.(Model)
	if m.finale != store.Smoking || !strings.Contains(m.View(), "MY OWN TEXT") {
		t.Fatal("final modal with the configured text should open after the last badge")
	}
	m = press(t, m, "enter")
	if next, _ := m.Update(tickMsg{}); next.(Model).finale != "" {
		t.Fatal("final modal must not reopen once shown")
	}
}

func TestRelapseWipesEverything(t *testing.T) {
	e := newEnv(t)
	m := press(t, e.open(), "enter", "enter", "enter")
	e.net.t = e.net.t.Add(10 * 24 * time.Hour)
	m = press(t, e.sync(m), "s")

	// Default answer is "no": a stray enter must not wipe anything.
	m = press(t, m, "r", "enter")
	if _, ok := m.habit(store.Smoking); !ok || m.screen != scrDash {
		t.Fatal("enter on the default button must cancel the relapse")
	}

	m = press(t, m, "r", "y")
	if _, ok := m.habit(store.Smoking); ok {
		t.Fatal("relapse must delete the habit entirely")
	}
	if m.screen != scrHabit || !strings.Contains(m.View(), "zero badges") {
		t.Fatalf("relapse must send the user back to start, screen %v", m.screen)
	}

	// Reopening keeps nothing; starting again begins from zero with no badges.
	m = press(t, e.open(), "enter", "enter")
	hs, _ := m.habit(store.Smoking)
	if hs.Elapsed(e.net.t) > time.Second || hs.Acknowledged != 0 {
		t.Fatalf("restart must begin from scratch: %+v", hs)
	}
	if _, _, ok := m.popup(); ok {
		t.Fatal("no badges after a relapse")
	}
}

func TestTabReachesUntrackedHabit(t *testing.T) {
	e := newEnv(t)
	m := press(t, e.open(), "enter", "enter", "enter") // tz, smoking, start
	if m.active != store.Smoking {
		t.Fatalf("active %q", m.active)
	}

	m = press(t, m, "tab")
	if m.active != store.Alcohol || !strings.Contains(m.View(), "Not tracking alcohol yet") {
		t.Fatalf("tab must switch to the untracked habit, active %q", m.active)
	}
	// Cancelling the start dialog comes back to the dashboard, not elsewhere.
	m = press(t, m, "enter", "n")
	if m.screen != scrDash {
		t.Fatalf("cancel must return to dashboard, got %v", m.screen)
	}
	m = press(t, m, "enter", "enter")
	if _, ok := m.habit(store.Alcohol); !ok || m.screen != scrDash {
		t.Fatal("enter on an untracked tab must start its counter")
	}
	m = press(t, m, "tab")
	if m.active != store.Smoking {
		t.Fatalf("tab must wrap around, active %q", m.active)
	}
}
