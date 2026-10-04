// Package ui is the Bubble Tea interface of the tracker.
package ui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"lastdose/internal/achievements"
	"lastdose/internal/netclock"
	"lastdose/internal/store"
	"lastdose/internal/tz"
)

// Options configures the UI.
type Options struct {
	Store *store.Store
	// Clock is the only source of time; the system clock is never trusted.
	Clock *netclock.Clock
	// LookupTZ detects the time zone online; nil skips online detection.
	LookupTZ func(ctx context.Context) (string, error)
	// FinalMessage is shown in the modal after the last achievement.
	FinalMessage string
	// PreviewFinal opens the final modal right away without saving anything.
	PreviewFinal bool
}

type screen int

const (
	scrSync screen = iota
	scrTZ
	scrTZInput
	scrHabit
	scrConfirmStart
	scrDash
	scrGallery
	scrConfirmRelapse
)

const (
	frameEvery = 125 * time.Millisecond
	// saveEvery bounds how stale last_seen can get if the process is killed
	// with SIGKILL. The timer itself never depends on it: elapsed time is
	// always now - started_at.
	saveEvery = 10 * time.Second
	// retryEvery is the pause between failed attempts to reach time servers.
	retryEvery = 10 * time.Second
	// resyncEvery corrects drift; resyncMin throttles resyncs triggered by
	// clock jumps or suspend.
	resyncEvery = 10 * time.Minute
	resyncMin   = 30 * time.Second
	syncTimeout = 10 * time.Second
)

type tickMsg struct{}

func tick() tea.Cmd {
	return tea.Tick(frameEvery, func(time.Time) tea.Msg { return tickMsg{} })
}

type syncMsg struct {
	err     error
	zone    string
	zoneErr error
}

type tzOption struct {
	zone string
	tag  string // "detected online", "system", or empty
}

// manualTZ marks the "enter by hand" row of the zone picker.
const manualTZ = "\x00manual"

// Model is the root Bubble Tea model.
type Model struct {
	opt   Options
	st    *store.Store
	now   time.Time
	frame int
	w, h  int

	screen screen
	// back is where the time zone picker returns to; scrTZ means first run.
	back screen

	// Network time state.
	synced      bool
	syncing     bool
	offline     bool
	retryAt     time.Time
	lastSyncTry time.Time

	tzName    string
	loc       *time.Location
	netTZ     string
	tzOptions []tzOption
	tzCursor  int
	tzInput   textinput.Model
	tzErr     string

	habitCursor int
	pending     store.Habit
	// startFrom is where cancelling the start dialog returns to.
	startFrom     screen
	active        store.Habit
	yes           bool
	galleryCursor int

	// finale is the habit whose final modal is open, if any.
	finale       store.Habit
	previewFinal bool

	notice   string
	flash    string
	warn     string
	saveErr  string
	lastSave time.Time
}

// New builds the model. Nothing is shown or saved until the network clock
// syncs, because there is no trusted time before that.
func New(opt Options) Model {
	m := Model{
		opt:     opt,
		st:      opt.Store,
		w:       80,
		h:       24,
		screen:  scrSync,
		back:    scrTZ,
		loc:     time.Local,
		syncing: true,
	}
	m.tzInput = textinput.New()
	m.tzInput.CharLimit = 64
	m.tzInput.Width = 40
	return m
}

// Init starts the animation clock and the first network sync.
func (m Model) Init() tea.Cmd { return tea.Batch(tick(), m.syncCmd()) }

// syncCmd syncs the clock; before the first success it also looks up the
// time zone online, in parallel.
func (m Model) syncCmd() tea.Cmd {
	clock, lookup := m.opt.Clock, m.opt.LookupTZ
	if m.synced {
		lookup = nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), syncTimeout)
		defer cancel()
		var msg syncMsg
		var wg sync.WaitGroup
		if lookup != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				msg.zone, msg.zoneErr = lookup(ctx)
			}()
		}
		msg.err = clock.Sync(ctx)
		wg.Wait()
		return msg
	}
}

func (m *Model) trustedNow() time.Time {
	if t, ok := m.opt.Clock.Now(); ok {
		return t
	}
	return m.now
}

// begin runs once, after the first network sync succeeded.
func (m *Model) begin(msg syncMsg) {
	m.synced = true
	m.now = m.trustedNow()
	m.lastSave = m.now

	if msg.zoneErr == nil {
		m.netTZ = msg.zone
	}
	m.buildTZOptions()

	var lastSeen time.Time
	m.st.View(func(s *store.State) {
		m.tzName = s.Timezone
		lastSeen = s.LastSeen
	})
	if m.tzName != "" {
		if loc, err := tz.Load(m.tzName); err == nil {
			m.loc = loc
		} else {
			m.warn = fmt.Sprintf("Saved time zone %q is unknown, please pick one again.", m.tzName)
			m.tzName = ""
		}
	}
	if !lastSeen.IsZero() {
		if away := m.now.Sub(lastSeen); away >= time.Minute {
			m.notice = fmt.Sprintf("Last opened %s, %s ago. Every second of it counts.",
				lastSeen.In(m.loc).Format(dateHMLayout), short(away))
		}
	}
	m.touch()

	switch tracked := m.tracked(); {
	case m.tzName == "":
		m.screen = scrTZ
	case len(tracked) == 0:
		m.screen = scrHabit
	default:
		m.screen = scrDash
		m.active = tracked[0]
	}

	if m.opt.PreviewFinal {
		m.previewFinal = true
		m.finale = store.Smoking
		if m.active != "" {
			m.finale = m.active
		}
	}
}

func (m *Model) buildTZOptions() {
	seen := map[string]bool{}
	add := func(zone, tag string) {
		if zone == "" || seen[zone] {
			return
		}
		seen[zone] = true
		m.tzOptions = append(m.tzOptions, tzOption{zone, tag})
	}
	system := tz.Detect()
	if m.netTZ != "" && m.netTZ == system {
		add(m.netTZ, "detected online, same as system")
	}
	add(m.netTZ, "detected online")
	add(system, "system")
	for _, z := range tz.Common {
		add(z, "")
	}
	m.tzInput.Placeholder = m.tzOptions[0].zone
	m.tzOptions = append(m.tzOptions, tzOption{zone: manualTZ})
}

func (m *Model) touch() {
	if !m.synced {
		return
	}
	m.lastSave = m.now
	if err := m.st.Touch(m.now); err != nil {
		m.saveErr = err.Error()
	} else {
		m.saveErr = ""
	}
}

func (m *Model) update(f func(s *store.State)) {
	if err := m.st.Update(f); err != nil {
		m.saveErr = err.Error()
	} else {
		m.saveErr = ""
	}
}

func (m Model) quit() (tea.Model, tea.Cmd) {
	m.now = m.trustedNow()
	m.touch()
	return m, tea.Quit
}

// tracked lists habits currently being tracked, in display order.
func (m *Model) tracked() []store.Habit {
	var out []store.Habit
	m.st.View(func(s *store.State) {
		for _, h := range store.Habits {
			if s.Habits[h] != nil {
				out = append(out, h)
			}
		}
	})
	return out
}

// habit returns a copy of the habit state, or false if it is not tracked.
func (m *Model) habit(h store.Habit) (store.HabitState, bool) {
	var hs store.HabitState
	var ok bool
	m.st.View(func(s *store.State) {
		if p := s.Habits[h]; p != nil {
			hs, ok = *p, true
		}
	})
	return hs, ok
}

// popup returns the oldest unlocked achievement the user has not seen yet.
func (m *Model) popup() (store.Habit, int, bool) {
	for _, h := range m.tracked() {
		hs, _ := m.habit(h)
		if n := achievements.Unlocked(hs.Elapsed(m.now)); hs.Acknowledged < n {
			return h, hs.Acknowledged, true
		}
	}
	return "", 0, false
}

func (m *Model) pendingCount() int {
	total := 0
	for _, h := range m.tracked() {
		hs, _ := m.habit(h)
		if n := achievements.Unlocked(hs.Elapsed(m.now)) - hs.Acknowledged; n > 0 {
			total += n
		}
	}
	return total
}

// checkFinale opens the final modal for a habit that has acknowledged the
// last achievement but has not seen the congratulation yet.
func (m *Model) checkFinale() {
	if m.finale != "" {
		return
	}
	for _, h := range m.tracked() {
		hs, _ := m.habit(h)
		if hs.Acknowledged >= len(achievements.All) && !hs.FinalShown {
			m.finale = h
			return
		}
	}
}

func (m *Model) overlaysAllowed() bool {
	return m.screen == scrDash || m.screen == scrGallery
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		return m, nil

	case syncMsg:
		m.syncing = false
		m.lastSyncTry = time.Now()
		if msg.err != nil {
			if m.synced {
				// Keep counting on the monotonic clock until the network is back.
				m.offline = true
			} else {
				m.retryAt = time.Now().Add(retryEvery)
			}
			return m, nil
		}
		m.offline = false
		if !m.synced {
			m.begin(msg)
		}
		m.now = m.trustedNow()
		return m, nil

	case tickMsg:
		m.frame++
		if !m.synced {
			if !m.syncing && !time.Now().Before(m.retryAt) {
				m.syncing = true
				return m, tea.Batch(tick(), m.syncCmd())
			}
			return m, tick()
		}
		m.now = m.trustedNow()
		if m.now.Sub(m.lastSave) >= saveEvery {
			m.touch()
		}
		if m.overlaysAllowed() {
			m.checkFinale()
		}
		since := time.Since(m.lastSyncTry)
		if !m.syncing && since >= resyncMin && (since >= resyncEvery || m.opt.Clock.NeedsResync()) {
			m.syncing = true
			return m, tea.Batch(tick(), m.syncCmd())
		}
		return m, tick()

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m.quit()
		}
		if m.finale != "" {
			return m.keyFinale(msg)
		}
		if m.overlaysAllowed() {
			if h, idx, ok := m.popup(); ok {
				return m.keyPopup(msg, h, idx)
			}
		}
		switch m.screen {
		case scrSync:
			return m.keySync(msg)
		case scrTZ:
			return m.keyTZ(msg)
		case scrTZInput:
			return m.keyTZInput(msg)
		case scrHabit:
			return m.keyHabit(msg)
		case scrConfirmStart, scrConfirmRelapse:
			return m.keyConfirm(msg)
		case scrDash:
			return m.keyDash(msg)
		case scrGallery:
			return m.keyGallery(msg)
		}
	}
	return m, nil
}

func (m Model) keySync(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "r":
		if !m.syncing {
			m.syncing = true
			return m, m.syncCmd()
		}
	case "q", "esc":
		return m.quit()
	}
	return m, nil
}

func (m Model) keyFinale(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "esc", " ":
		if !m.previewFinal {
			h := m.finale
			m.update(func(s *store.State) {
				if hs := s.Habits[h]; hs != nil {
					hs.FinalShown = true
				}
			})
		}
		m.previewFinal = false
		m.finale = ""
	case "q":
		return m.quit()
	}
	return m, nil
}

func (m Model) keyPopup(msg tea.KeyMsg, h store.Habit, idx int) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", " ":
		m.update(func(s *store.State) {
			if hs := s.Habits[h]; hs != nil && hs.Acknowledged == idx {
				hs.Acknowledged++
			}
		})
	case "s":
		// Accept every pending badge at once, e.g. after a long absence.
		now := m.now
		m.update(func(s *store.State) {
			for _, hs := range s.Habits {
				if n := achievements.Unlocked(hs.Elapsed(now)); hs.Acknowledged < n {
					hs.Acknowledged = n
				}
			}
		})
	case "q":
		return m.quit()
	default:
		return m, nil
	}
	m.active = h
	m.checkFinale()
	return m, nil
}

func (m Model) keyTZ(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(m.tzOptions)
	switch msg.String() {
	case "up", "k":
		m.tzCursor = (m.tzCursor - 1 + n) % n
	case "down", "j":
		m.tzCursor = (m.tzCursor + 1) % n
	case "enter":
		choice := m.tzOptions[m.tzCursor].zone
		if choice != manualTZ {
			m.setTZ(choice)
			return m, nil
		}
		m.screen = scrTZInput
		m.tzErr = ""
		m.tzInput.SetValue("")
		return m, m.tzInput.Focus()
	case "esc":
		if m.back != scrTZ {
			m.screen = m.back
		}
	case "q":
		return m.quit()
	}
	return m, nil
}

func (m Model) keyTZInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		name := strings.TrimSpace(m.tzInput.Value())
		if name == "" {
			name = m.tzInput.Placeholder
		}
		if _, err := tz.Load(name); err != nil {
			m.tzErr = fmt.Sprintf("Unknown time zone %q", name)
			return m, nil
		}
		m.tzInput.Blur()
		m.setTZ(name)
		return m, nil
	case "esc":
		m.tzInput.Blur()
		m.screen = scrTZ
		return m, nil
	}
	var cmd tea.Cmd
	m.tzInput, cmd = m.tzInput.Update(msg)
	m.tzErr = ""
	return m, cmd
}

func (m *Model) setTZ(name string) {
	loc, err := tz.Load(name)
	if err != nil {
		m.tzErr = err.Error()
		return
	}
	m.loc, m.tzName = loc, name
	m.update(func(s *store.State) { s.Timezone = name })
	if m.back != scrTZ {
		m.screen = m.back
		return
	}
	if tracked := m.tracked(); len(tracked) > 0 {
		m.active = tracked[0]
		m.screen = scrDash
	} else {
		m.screen = scrHabit
	}
}

func (m *Model) openTZ() {
	m.back = m.screen
	m.tzCursor = 0
	for i, o := range m.tzOptions {
		if o.zone == m.tzName {
			m.tzCursor = i
		}
	}
	m.screen = scrTZ
}

func (m Model) keyHabit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(store.Habits)
	switch msg.String() {
	case "up", "k":
		m.habitCursor = (m.habitCursor - 1 + n) % n
	case "down", "j":
		m.habitCursor = (m.habitCursor + 1) % n
	case "enter":
		h := store.Habits[m.habitCursor]
		if _, ok := m.habit(h); ok {
			m.active = h
			m.screen = scrDash
		} else {
			m.askStart(h)
		}
		m.flash = ""
	case "esc":
		if len(m.tracked()) > 0 {
			m.flash = ""
			m.screen = scrDash
		}
	case "t":
		m.openTZ()
	case "q":
		return m.quit()
	}
	return m, nil
}

func (m Model) keyConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "right", "h", "l", "tab", "shift+tab":
		m.yes = !m.yes
		return m, nil
	case "y":
		m.yes = true
	case "n", "esc":
		m.yes = false
	case "enter":
	case "q":
		return m.quit()
	default:
		return m, nil
	}
	// "y", "n", "esc" and "enter" all resolve the dialog.
	switch m.screen {
	case scrConfirmStart:
		if !m.yes {
			m.screen = m.startFrom
			return m, nil
		}
		h, now := m.pending, m.now
		m.update(func(s *store.State) {
			s.Habits[h] = &store.HabitState{StartedAt: now.UTC()}
		})
		m.active = h
		m.notice = ""
	case scrConfirmRelapse:
		if m.yes {
			m.relapse()
			return m, nil
		}
	}
	m.screen = scrDash
	return m, nil
}

// relapse wipes the habit completely: timer, badges, everything. Nothing is
// carried over; the user has to start the counter again from scratch.
func (m *Model) relapse() {
	h := m.active
	m.update(func(s *store.State) { delete(s.Habits, h) })
	for i, x := range store.Habits {
		if x == h {
			m.habitCursor = i
		}
	}
	m.notice = ""
	m.flash = fmt.Sprintf("%s counter wiped: zero time, zero badges. Start again when you're ready.", habitTitle(h))
	m.screen = scrHabit
}

func (m *Model) askStart(h store.Habit) {
	m.pending = h
	m.yes = true
	m.startFrom = m.screen
	m.screen = scrConfirmStart
}

// keyDash handles the dashboard. Tabs cover every habit, tracked or not; an
// untracked tab offers to start its counter.
func (m Model) keyDash(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(store.Habits)
	cur := 0
	for i, h := range store.Habits {
		if h == m.active {
			cur = i
		}
	}
	_, tracked := m.habit(m.active)
	switch msg.String() {
	case "tab", "right", "l":
		m.active = store.Habits[(cur+1)%n]
	case "shift+tab", "left", "h":
		m.active = store.Habits[(cur-1+n)%n]
	case "enter":
		if !tracked {
			m.askStart(m.active)
		}
	case "a", "g":
		if tracked {
			hs, _ := m.habit(m.active)
			m.galleryCursor = min(achievements.Unlocked(hs.Elapsed(m.now)), len(achievements.All)-1)
			m.screen = scrGallery
		}
	case "t":
		m.openTZ()
	case "r":
		if tracked {
			m.yes = false
			m.screen = scrConfirmRelapse
		}
	case "q", "esc":
		return m.quit()
	}
	return m, nil
}

func (m Model) keyGallery(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(achievements.All)
	switch msg.String() {
	case "up", "k":
		m.galleryCursor = (m.galleryCursor - 1 + n) % n
	case "down", "j":
		m.galleryCursor = (m.galleryCursor + 1) % n
	case "home":
		m.galleryCursor = 0
	case "end":
		m.galleryCursor = n - 1
	case "esc", "a", "g", "backspace":
		m.screen = scrDash
	case "q":
		return m.quit()
	}
	return m, nil
}
