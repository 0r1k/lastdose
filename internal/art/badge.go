package art

import "strings"

// Styles colors parts of a badge. Nil funcs leave text as is, so the art
// package stays independent from any terminal styling library.
type Styles struct {
	Border, Figure, Title, Text func(string) string
}

func apply(f func(string) string, s string) string {
	if f == nil {
		return s
	}
	return f(s)
}

// BadgeInner is the inner width of a badge in columns.
const BadgeInner = 38

// Badge frames figure and captions with a border. When lit, the border is a
// running marquee of lights driven by frame; otherwise it is a plain box.
func Badge(fig []string, title, subtitle, motto string, frame int, lit bool, st Styles) []string {
	var inner []string
	add := func(s string) { inner = append(inner, Center(s, BadgeInner)) }

	add("")
	// The figure is centered as a block so its columns stay aligned.
	inner = append(inner, CenterBlock(fig, BadgeInner)...)
	add("")
	add("~ " + strings.ToUpper(title) + " ~")
	add(subtitle)
	add("")
	for _, l := range Wrap(motto, BadgeInner-4) {
		add(l)
	}
	add("")

	w, h := BadgeInner+2, len(inner)+2
	border := perimeter(w, h, frame, lit)

	out := make([]string, 0, h)
	out = append(out, apply(st.Border, string(border[0])))
	for i, l := range inner {
		style := st.Text
		switch {
		case i >= 1 && i <= len(fig):
			style = st.Figure
		case i == len(fig)+2:
			style = st.Title
		}
		line := apply(st.Border, string(border[i+1][0])) +
			apply(style, l) +
			apply(st.Border, string(border[i+1][w-1]))
		out = append(out, line)
	}
	out = append(out, apply(st.Border, string(border[h-1])))
	return out
}

// perimeter returns an h x w grid where only border cells are meaningful.
func perimeter(w, h, frame int, lit bool) [][]byte {
	g := make([][]byte, h)
	for y := range g {
		g[y] = []byte(strings.Repeat(" ", w))
	}
	if !lit {
		for x := 0; x < w; x++ {
			g[0][x], g[h-1][x] = '-', '-'
		}
		for y := 0; y < h; y++ {
			g[y][0], g[y][w-1] = ':', ':'
		}
		g[0][0], g[0][w-1], g[h-1][0], g[h-1][w-1] = '+', '+', '+', '+'
		return g
	}
	// Walk the border clockwise and paint a moving light pattern.
	const lights = "*-.-+-.-"
	pos := 0
	paint := func(x, y int) {
		g[y][x] = lights[mod(pos-frame, len(lights))]
		pos++
	}
	for x := 0; x < w; x++ {
		paint(x, 0)
	}
	for y := 1; y < h; y++ {
		paint(w-1, y)
	}
	for x := w - 2; x >= 0; x-- {
		paint(x, h-1)
	}
	for y := h - 2; y > 0; y-- {
		paint(0, y)
	}
	return g
}

// Center pads s with spaces to width w, trimming trailing spaces first so
// fixed-width art stays visually centered.
func Center(s string, w int) string {
	s = strings.TrimRight(s, " ")
	if len(s) >= w {
		return s[:w]
	}
	left := (w - len(s)) / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", w-len(s)-left)
}

// CenterBlock centers a block of lines as a whole, keeping their alignment.
// Trailing padding counts toward the block width, so animation frames padded
// to a fixed canvas never jitter sideways.
func CenterBlock(lines []string, w int) []string {
	max := 0
	for _, l := range lines {
		if n := len(l); n > max {
			max = n
		}
	}
	left := (w - max) / 2
	if left < 0 {
		left = 0
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		l = strings.Repeat(" ", left) + strings.TrimRight(l, " ")
		if len(l) < w {
			l += strings.Repeat(" ", w-len(l))
		}
		out[i] = l
	}
	return out
}

// Wrap breaks text into lines of at most width runes on word boundaries.
func Wrap(text string, width int) []string {
	var lines []string
	for _, para := range strings.Split(text, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		cur := words[0]
		for _, w := range words[1:] {
			if len([]rune(cur))+1+len([]rune(w)) > width {
				lines = append(lines, cur)
				cur = w
				continue
			}
			cur += " " + w
		}
		lines = append(lines, cur)
	}
	return lines
}
