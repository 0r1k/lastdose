// LastDose is a TUI counter of time spent free from a bad habit.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	// Ship the zone database so zones resolve even without system tzdata.
	_ "time/tzdata"

	"lastdose/assets"
	"lastdose/internal/netclock"
	"lastdose/internal/store"
	"lastdose/internal/tz"
	"lastdose/internal/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "lastdose:", err)
		os.Exit(1)
	}
}

func run() error {
	defPath, err := store.DefaultPath()
	if err != nil {
		return err
	}
	dataPath := flag.String("data", defPath, "state file")
	demo := flag.Duration("demo", 0, "try the app on a throwaway state with a habit already this old, e.g. -demo 100h")
	flag.Parse()

	clock := netclock.New(netclock.DefaultSources()...)
	if *demo > 0 {
		path, cleanup, err := demoState(clock, *demo)
		if err != nil {
			return err
		}
		defer cleanup()
		*dataPath = path
	}

	st, err := store.Open(*dataPath, assets.Key())
	if err != nil {
		return err
	}

	p := tea.NewProgram(ui.New(ui.Options{
		Store:        st,
		Clock:        clock,
		LookupTZ:     tz.Online,
		FinalMessage: assets.FinalMessage,
		Demo:         *demo > 0,
	}), tea.WithAltScreen())

	// The terminal may close (SIGHUP) or the process may be asked to stop:
	// record the last moment we were alive before going down.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sigs
		touch(st, clock)
		p.Quit()
	}()

	_, runErr := p.Run()
	touch(st, clock)
	return runErr
}

// touch saves last_seen, but only with trusted network time.
func touch(st *store.Store, clock *netclock.Clock) {
	if now, ok := clock.Now(); ok {
		_ = st.Touch(now)
	}
}

// demoState creates a temporary state with smoking quit `age` ago, so badges
// and animations can be explored without touching the real progress.
func demoState(clock *netclock.Clock, age time.Duration) (string, func(), error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := clock.Sync(ctx); err != nil {
		return "", nil, err
	}
	now, _ := clock.Now()
	dir, err := os.MkdirTemp("", "lastdose-demo-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { os.RemoveAll(dir) }
	path := filepath.Join(dir, "state.json")
	st, err := store.Open(path, assets.Key())
	if err == nil {
		err = st.Update(func(s *store.State) {
			s.Timezone = tz.Detect()
			s.Habits[store.Smoking] = &store.HabitState{StartedAt: now.Add(-age).UTC()}
		})
	}
	if err != nil {
		cleanup()
		return "", nil, err
	}
	return path, cleanup, nil
}
