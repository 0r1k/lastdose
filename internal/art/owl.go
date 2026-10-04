// Package art draws Ray the owl, the mascot, plus achievement badges and big
// digits in plain 7-bit ASCII. Animation is frame based: callers pass a frame counter
// that grows with time and get back the lines to print.
package art

import "strings"

// eyes is one eye drawn 6 columns wide; both eyes always match.
type eyes string

const (
	eyesOpen  eyes = "( () )"
	eyesLeft  eyes = "(()  )"
	eyesRight eyes = "(  ())"
	eyesHappy eyes = "( ^^ )"
	eyesShut  eyes = "( -- )"
)

// Name is the mascot's name.
const Name = "Ray"

const (
	// Width is the canvas width every frame is padded to.
	Width = 29
	// Height is the number of lines in every frame.
	Height = 14
	// headCenter is the column hats are centered on.
	headCenter = 13
)

// wing positions.
const (
	wingsFolded = iota
	wingsUp
	wingsOut
)

type pose struct {
	eyes  eyes
	wings int
	fly   bool
	zzz   int // sleeping "z" bubble phase, 0 = none
}

// owl returns the bird without hat, talons or branch: 9 lines.
func owl(p pose) []string {
	e := string(p.eyes)
	lines := []string{
		`      ,_          _,`,
		"      | `-.____.-' |",
		`     /  .--.  .--.  \`,
		`    |  ` + e + e + `  |`,
		"    |   `--'\\/`--'   |",
		`     \            /`,
		"     /`\\\\\\\\\\\\\\\\\\\\\\\\'\\",
		`    /  |\\\\\\\\\\|  \`,
		"     `-|__\\\\\\\\\\\\__|-'",
	}
	switch p.wings {
	case wingsUp:
		lines[4] = ` \\ ` + lines[4][4:] + ` //`
		lines[5] = `  \\ ` + lines[5][5:] + ` //`
		lines[6] = `   \\` + lines[6][5:] + `//`
		lines[7] = `    |` + lines[7][5:]
		lines[7] = lines[7][:len(lines[7])-1] + `|`
	case wingsOut:
		lines[6] = `  __/` + lines[6][6:] + `__`
		lines[7] = ` /__/  ` + lines[7][7:len(lines[7])-1] + `\__\`
	}
	switch p.zzz {
	case 1:
		lines[1] += "  z"
	case 2:
		lines[0] += "    Z"
		lines[1] += "  z"
	case 3:
		lines[0] += "      Z"
	}
	return lines
}

const (
	perch      = `   ________((( )))________`
	branchTop  = `   _______________________`
	talonsAir  = `           (( ))`
	branchBase = `  (_______________________(@)`
)

// hats are drawn above the head, one per achievement. Pure ASCII.
var hats = [][2]string{
	{`.\|/.`, `  |  `},     // Day One: a sprout
	{`~ ~ ~`, ` ~ ~ ~`},    // Second Wind: wind
	{`\ | /`, `- O -`},     // Through the Fog: the sun
	{`.=====.`, `|_|`},     // Forged in Silence: an anvil
	{`) )`, `c[_]`},        // Master of Mornings: coffee
	{` _ `, `( (`},         // One Moon Clean: crescent
	{` ^ `, `/|\`},         // Roots Run Deep: a tree
	{``, `O===O`},          // The Quiet Strength: a dumbbell
	{`.-^-.`, `|___|`},     // New Default: graduation cap
	{`  ___  `, `_/___\_`}, // The Stranger I Was: fedora
	{`.-~-~-.`, `'-~-~-'`}, // The Long Road Home: halo
	{`.^.^.^.`, `|=====|`}, // Last Dose: crown
}

// HatCount is the number of distinct hats.
var HatCount = len(hats)

func centerAt(s string, col int) string {
	start := col - len(s)/2
	if start < 0 {
		start = 0
	}
	return strings.Repeat(" ", start) + s
}

// figure assembles hat + owl + branch on a fixed canvas. When flying, the
// owl and its hat lift one line off the branch.
func figure(p pose, hat int) []string {
	top := []string{"", ""}
	if hat >= 0 && hat < len(hats) {
		top = []string{centerAt(hats[hat][0], headCenter), centerAt(hats[hat][1], headCenter)}
	}
	bird := append(top, owl(p)...)
	var out []string
	if p.fly {
		out = append(bird, talonsAir, branchTop, branchBase)
	} else {
		out = append([]string{""}, bird...)
		out = append(out, perch, branchBase)
	}
	return pad(out, Width)
}

func pad(lines []string, w int) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		if len(l) < w {
			l += strings.Repeat(" ", w-len(l))
		}
		out[i] = l
	}
	return out
}

var (
	idle    = pose{eyes: eyesOpen}
	blink   = pose{eyes: eyesShut}
	left    = pose{eyes: eyesLeft}
	right   = pose{eyes: eyesRight}
	flapUp  = pose{eyes: eyesHappy, wings: wingsUp}
	flapOut = pose{eyes: eyesHappy, wings: wingsOut}
	hopUp   = pose{eyes: eyesHappy, wings: wingsUp, fly: true}
	hopOut  = pose{eyes: eyesHappy, wings: wingsOut, fly: true}
	land    = pose{eyes: eyesHappy}
)

func rep(p pose, n int) []pose {
	out := make([]pose, n)
	for i := range out {
		out[i] = p
	}
	return out
}

func seq(parts ...[]pose) []pose {
	var out []pose
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// Idle loop: look around, blink, stretch the wings now and then.
var idleSeq = seq(
	rep(idle, 10), rep(blink, 1), rep(idle, 6),
	rep(left, 5), rep(idle, 2), rep(right, 5), rep(idle, 4),
	rep(blink, 1), rep(idle, 3),
	rep(flapOut, 2), rep(flapUp, 2), rep(flapOut, 2), rep(flapUp, 2), rep(flapOut, 2),
	rep(idle, 6), rep(blink, 1), rep(idle, 2),
)

// Celebration loop: the owl takes off from the branch, flapping, and lands.
var partySeq = seq(
	rep(flapOut, 2), rep(hopUp, 2), rep(hopOut, 2), rep(hopUp, 2), rep(hopOut, 2),
	rep(flapUp, 2), rep(land, 2),
	rep(flapOut, 2), rep(flapUp, 2), rep(land, 2),
)

var sleepSeq = seq(rep(pose{eyes: eyesShut, zzz: 1}, 5), rep(pose{eyes: eyesShut, zzz: 2}, 5), rep(pose{eyes: eyesShut, zzz: 3}, 5))

// Owl returns an idle-animation frame wearing the given hat (-1 = none).
func Owl(frame, hat int) []string { return figure(idleSeq[mod(frame, len(idleSeq))], hat) }

// Party returns a celebration frame wearing the given hat.
func Party(frame, hat int) []string { return figure(partySeq[mod(frame, len(partySeq))], hat) }

// Sleeping returns a dozing owl, used for locked achievements.
func Sleeping(frame int) []string { return figure(sleepSeq[mod(frame, len(sleepSeq))], -1) }

func mod(a, n int) int { return ((a % n) + n) % n }
