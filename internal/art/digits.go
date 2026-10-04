package art

// Seven-segment style glyphs, three rows high.
var glyphs = map[rune][3]string{
	'0': {" _ ", "| |", "|_|"},
	'1': {"   ", "  |", "  |"},
	'2': {" _ ", " _|", "|_ "},
	'3': {" _ ", " _|", " _|"},
	'4': {"   ", "|_|", "  |"},
	'5': {" _ ", "|_ ", " _|"},
	'6': {" _ ", "|_ ", "|_|"},
	'7': {" _ ", "  |", "  |"},
	'8': {" _ ", "|_|", "|_|"},
	'9': {" _ ", "|_|", " _|"},
	'd': {"   ", " _|", "|_|"},
	':': {" ", ".", "."},
	' ': {"  ", "  ", "  "},
}

// BigText renders digits, ':', 'd' and spaces as three-row ASCII art.
func BigText(s string) []string {
	var rows [3]string
	for _, r := range s {
		g, ok := glyphs[r]
		if !ok {
			g = glyphs[' ']
		}
		for i := range rows {
			rows[i] += g[i] + " "
		}
	}
	return rows[:]
}
