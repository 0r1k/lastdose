package ui

import (
	"github.com/charmbracelet/lipgloss"

	"lastdose/internal/achievements"
	"lastdose/internal/art"
)

var (
	colPrimary = lipgloss.Color("#00ADD8")
	colOwl     = lipgloss.Color("#E0C097")
	colAccent  = lipgloss.Color("#F5C242")
	colDim     = lipgloss.Color("#7A7F8C")
	colText    = lipgloss.Color("#D8DEE9")
	colGood    = lipgloss.Color("#98C379")
	colBad     = lipgloss.Color("#E06C75")

	// Plain-ASCII box borders to match the art.
	asciiBorder = lipgloss.Border{
		Top: "-", Bottom: "-", Left: "|", Right: "|",
		TopLeft: "+", TopRight: "+", BottomLeft: "+", BottomRight: "+",
	}
	finaleBorder = lipgloss.Border{
		Top: "=", Bottom: "=", Left: "#", Right: "#",
		TopLeft: "#", TopRight: "#", BottomLeft: "#", BottomRight: "#",
	}

	sTitle  = lipgloss.NewStyle().Bold(true).Foreground(colPrimary)
	sAccent = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	sDim    = lipgloss.NewStyle().Foreground(colDim)
	sText   = lipgloss.NewStyle().Foreground(colText)
	sGood   = lipgloss.NewStyle().Foreground(colGood)
	sBad    = lipgloss.NewStyle().Bold(true).Foreground(colBad)
	sOwl    = lipgloss.NewStyle().Foreground(colOwl)
	sBig    = lipgloss.NewStyle().Bold(true).Foreground(colPrimary)
	sKey    = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	sSel    = lipgloss.NewStyle().Bold(true).Foreground(colPrimary)
	sTabOn  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#1E1E2E")).Background(colPrimary).Padding(0, 1)
	sTabOff = lipgloss.NewStyle().Foreground(colDim).Padding(0, 1)
	sPanel  = lipgloss.NewStyle().Border(asciiBorder).BorderForeground(colDim).Padding(0, 2)
	sButton = lipgloss.NewStyle().Foreground(colDim).Padding(0, 2)
	sBtnOn  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#1E1E2E")).Background(colAccent).Padding(0, 2)
)

// rainbow cycles for the final achievement's border.
var rainbow = []lipgloss.Color{"#FF5F5F", "#FFAF5F", "#FFD75F", "#87D787", "#5FD7FF", "#878BFF", "#D787FF"}

// tierColor gives bronze, silver and gold tiers; the final badge shimmers.
func tierColor(idx, frame int) lipgloss.Color {
	switch {
	case idx == len(achievements.All)-1:
		return rainbow[(frame/2)%len(rainbow)]
	case idx >= 8:
		return "#FFD700"
	case idx >= 4:
		return "#C0C8D0"
	}
	return "#CD7F32"
}

func styler(st lipgloss.Style) func(string) string {
	return func(s string) string { return st.Render(s) }
}

func badgeStyles(idx, frame int, lit bool) art.Styles {
	if !lit {
		return art.Styles{
			Border: styler(sDim), Figure: styler(sDim),
			Title: styler(sDim), Text: styler(sDim),
		}
	}
	return art.Styles{
		Border: styler(lipgloss.NewStyle().Bold(true).Foreground(tierColor(idx, frame))),
		Figure: styler(sOwl),
		Title:  styler(sAccent),
		Text:   styler(sText),
	}
}

func key(k, desc string) string { return sKey.Render(k) + " " + sDim.Render(desc) }
