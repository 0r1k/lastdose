// Package achievements describes the milestone ladder: 24h, then every
// previous milestone doubled, with enough steps to cover more than 5 years.
package achievements

import "time"

// BaseStep is the first milestone; every next one is the previous doubled.
const BaseStep = 24 * time.Hour

// Achievement is a single milestone on the ladder.
type Achievement struct {
	Index     int
	Name      string
	Motto     string
	Threshold time.Duration
}

var defs = []struct{ name, motto string }{
	{"Day One", "Not one day. Day one."},
	{"Second Wind", "The hardest step is the one you take again."},
	{"Through the Fog", "Clarity is not given. It is earned, hour by hour."},
	{"Forged in Silence", "Discipline is quiet. Its results are loud."},
	{"Master of Mornings", "Every morning is a vote for who you are becoming."},
	{"One Moon Clean", "A full cycle of the moon. A full cycle of you."},
	{"Roots Run Deep", "What grows slowly stands strong."},
	{"The Quiet Strength", "Strength is not loud. It is steady."},
	{"New Default", "It is no longer a fight. It is simply who you are."},
	{"The Stranger I Was", "You barely remember the one who needed it."},
	{"The Long Road Home", "Miles behind you. Home ahead."},
	{"Last Dose", "The last one was the last one. Forever."},
}

// All is the full ladder: 24h * 2^i. The last step is 2048 days (~5.6 years),
// which covers the required 5-year horizon with margin.
var All = func() []Achievement {
	out := make([]Achievement, len(defs))
	for i, d := range defs {
		out[i] = Achievement{
			Index:     i,
			Name:      d.name,
			Motto:     d.motto,
			Threshold: BaseStep << i,
		}
	}
	return out
}()

// Unlocked returns how many achievements are reached after elapsed time.
func Unlocked(elapsed time.Duration) int {
	n := 0
	for _, a := range All {
		if elapsed >= a.Threshold {
			n++
		}
	}
	return n
}

// Next returns the next locked achievement, or false if all are unlocked.
func Next(elapsed time.Duration) (Achievement, bool) {
	n := Unlocked(elapsed)
	if n >= len(All) {
		return Achievement{}, false
	}
	return All[n], true
}

// Progress returns 0..1 progress from the previous milestone to the next one.
func Progress(elapsed time.Duration) float64 {
	n := Unlocked(elapsed)
	if n >= len(All) {
		return 1
	}
	var from time.Duration
	if n > 0 {
		from = All[n-1].Threshold
	}
	to := All[n].Threshold
	p := float64(elapsed-from) / float64(to-from)
	if p < 0 {
		return 0
	}
	return p
}
