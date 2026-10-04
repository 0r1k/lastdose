package ui

import (
	"fmt"
	"strings"
	"time"

	"lastdose/internal/store"
)

const (
	dateTimeLayout = "Jan 02, 2006 15:04:05"
	dateLayout     = "Jan 02, 2006"
	dateHMLayout   = "Jan 02, 2006 15:04"
)

// plural returns "1 day", "2 days".
func plural(n int64, unit string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, unit)
	}
	return fmt.Sprintf("%d %ss", n, unit)
}

func split(d time.Duration) (days, h, m, s int64) {
	if d < 0 {
		d = 0
	}
	total := int64(d / time.Second)
	return total / 86400, total % 86400 / 3600, total % 3600 / 60, total % 60
}

// clock formats d as "3d 04:12:55" for the big seven-segment timer.
func clock(d time.Duration) string {
	days, h, m, s := split(d)
	return fmt.Sprintf("%dd %02d:%02d:%02d", days, h, m, s)
}

// compact formats d as "3d 04:12:55", dropping the days when zero.
func compact(d time.Duration) string {
	days, h, m, s := split(d)
	if days > 0 {
		return fmt.Sprintf("%dd %02d:%02d:%02d", days, h, m, s)
	}
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// short formats d coarsely, e.g. "2d 3h" or "15m".
func short(d time.Duration) string {
	days, h, m, s := split(d)
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, h)
	case h > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	case m > 0:
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%ds", s)
}

// words spells d out: "3 days 4 hours 12 minutes 55 seconds".
func words(d time.Duration) string {
	days, h, m, s := split(d)
	var parts []string
	if days > 0 {
		parts = append(parts, plural(days, "day"))
	}
	parts = append(parts, plural(h, "hour"), plural(m, "minute"), plural(s, "second"))
	return strings.Join(parts, " ")
}

// milestone names a threshold: "48 hours", "512 days (~1.4 yrs)".
func milestone(d time.Duration) string {
	hours := int64(d / time.Hour)
	if hours < 72 {
		return plural(hours, "hour")
	}
	days := hours / 24
	s := plural(days, "day")
	if days >= 365 {
		s += fmt.Sprintf(" (~%.1f yrs)", float64(days)/365.25)
	}
	return s
}

// milestoneUpper is the badge caption: "24 HOURS", "4 DAYS".
func milestoneUpper(d time.Duration) string {
	hours := int64(d / time.Hour)
	if hours < 72 {
		return fmt.Sprintf("%d HOURS", hours)
	}
	return fmt.Sprintf("%d DAYS", hours/24)
}

func habitTitle(h store.Habit) string {
	switch h {
	case store.Smoking:
		return "Smoking"
	case store.Alcohol:
		return "Alcohol"
	}
	return string(h)
}

// habitFree is the phrase shown above the timer.
func habitFree(h store.Habit) string {
	switch h {
	case store.Smoking:
		return "Smoke-free for"
	case store.Alcohol:
		return "Alcohol-free for"
	}
	return "Clean for"
}

func zoneLabel(name string, loc *time.Location, now time.Time) string {
	_, off := now.In(loc).Zone()
	sign := '+'
	if off < 0 {
		sign, off = '-', -off
	}
	return fmt.Sprintf("%s (UTC%c%02d:%02d)", name, sign, off/3600, off%3600/60)
}
