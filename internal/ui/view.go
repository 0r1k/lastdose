package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"lastdose/internal/achievements"
	"lastdose/internal/art"
	"lastdose/internal/store"
	"lastdose/internal/tz"
)

// View renders the current screen, with modal overlays taking precedence.
func (m Model) View() string {
	if m.finale != "" {
		return m.place(m.viewFinale())
	}
	if m.overlaysAllowed() {
		if h, idx, ok := m.popup(); ok {
			return m.place(m.viewPopup(h, idx))
		}
	}

	var body string
	switch m.screen {
	case scrSync:
		body = m.viewSync()
	case scrTZ:
		body = m.viewTZ()
	case scrTZInput:
		body = m.viewTZInput()
	case scrHabit:
		body = m.viewHabit()
	case scrConfirmStart, scrConfirmRelapse:
		body = m.viewConfirm()
	case scrDash:
		body = m.viewDash()
	case scrGallery:
		body = m.viewGallery()
	}

	parts := []string{m.viewHeader()}
	if m.synced {
		if st := m.opt.Clock.Status(); st.Offset.Abs() >= time.Minute {
			parts = append(parts, sDim.Render(fmt.Sprintf(
				"System clock is off by %s. Ignored: LastDose runs on network time.", short(st.Offset.Abs()))))
		}
	}
	if m.offline {
		parts = append(parts, sBad.Render("! Time servers unreachable, counting on the internal clock until they are back."))
	}
	if m.warn != "" {
		parts = append(parts, sBad.Render("! "+m.warn))
	}
	if m.saveErr != "" {
		parts = append(parts, sBad.Render("! Could not save: "+m.saveErr))
	}
	parts = append(parts, "", body)
	return lipgloss.NewStyle().Padding(1, 2).Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
}

func (m Model) place(s string) string {
	return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, s)
}

func (m Model) viewHeader() string {
	left := sTitle.Render("LAST DOSE") + sDim.Render("  :: freedom counter")
	right := ""
	if m.synced {
		zone := m.tzName
		if zone == "" {
			zone = "UTC"
		}
		loc := m.loc
		if m.tzName == "" {
			loc = time.UTC
		}
		right = sDim.Render(zone+"  ") + sText.Render(m.now.In(loc).Format(dateTimeLayout))
	}
	gap := m.w - 4 - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		gap = 2
	}
	return left + strings.Repeat(" ", gap) + right
}

func help(keys ...string) string {
	return strings.Join(keys, "   ")
}

func (m Model) owl(hat int) string {
	return sOwl.Render(strings.Join(art.Owl(m.frame, hat), "\n"))
}

// ---------- network sync ----------

func (m Model) viewSync() string {
	var b strings.Builder
	spinner := string(`|/-\`[m.frame/2%4])
	if m.syncing {
		b.WriteString(sAccent.Render(spinner+" Syncing with network time servers...") + "\n\n")
	} else {
		b.WriteString(sBad.Render("Could not reach any time server.") + "\n")
		b.WriteString(sDim.Render("Check your internet connection.") + "\n\n")
		wait := time.Until(m.retryAt).Round(time.Second)
		b.WriteString(sText.Render(fmt.Sprintf("Retrying in %s.", max(wait, 0))) + "\n\n")
	}
	b.WriteString(sDim.Render("LastDose never trusts the computer clock: time comes from\nNTP servers, so changing the system date cannot fake progress.\nAn internet connection is required to start.") + "\n\n")
	keys := []string{}
	if !m.syncing {
		keys = append(keys, key("r", "retry now"))
	}
	b.WriteString(help(append(keys, key("q", "quit"))...))
	return lipgloss.JoinHorizontal(lipgloss.Top, b.String(), "    ", m.owl(-1))
}

// ---------- time zone ----------

func (m Model) viewTZ() string {
	var b strings.Builder
	b.WriteString(sAccent.Render("Choose your time zone") + "\n")
	b.WriteString(sDim.Render("Time is counted in absolute network time; the zone only affects how dates are shown.") + "\n\n")
	for i, o := range m.tzOptions {
		cursor, st := "  ", sText
		if i == m.tzCursor {
			cursor, st = sSel.Render("> "), sSel
		}
		if o.zone == manualTZ {
			b.WriteString(cursor + st.Render("Enter manually...") + "\n")
			continue
		}
		loc, err := tz.Load(o.zone)
		label := st.Render(fmt.Sprintf("%-38s", zoneLabel(o.zone, loc, m.now)))
		if err == nil {
			label += sDim.Render("  " + m.now.In(loc).Format("15:04"))
		}
		if o.tag != "" {
			label += sDim.Render("  " + o.tag)
		}
		if i == 0 {
			label += sGood.Render("  [default]")
		}
		if o.zone == m.tzName {
			label += sAccent.Render("  *current")
		}
		b.WriteString(cursor + label + "\n")
	}
	b.WriteString("\n")
	if m.netTZ == "" {
		b.WriteString(sDim.Render("Online detection failed, the system zone is the default.") + "\n")
	}
	if m.tzErr != "" {
		b.WriteString(sBad.Render(m.tzErr) + "\n")
	}
	keys := []string{key("j/k", "move"), key("enter", "select")}
	if m.back != scrTZ {
		keys = append(keys, key("esc", "back"))
	}
	b.WriteString(help(append(keys, key("q", "quit"))...))
	return b.String()
}

func (m Model) viewTZInput() string {
	var b strings.Builder
	b.WriteString(sAccent.Render("Enter an IANA time zone") + "\n")
	b.WriteString(sDim.Render("For example: Europe/Berlin, Asia/Tokyo, America/New_York. Empty = "+m.tzInput.Placeholder) + "\n\n")
	b.WriteString("  " + m.tzInput.View() + "\n\n")
	if m.tzErr != "" {
		b.WriteString(sBad.Render(m.tzErr) + "\n\n")
	}
	b.WriteString(help(key("enter", "save"), key("esc", "back")))
	return b.String()
}

// ---------- habit choice & confirmations ----------

func (m Model) viewHabit() string {
	var b strings.Builder
	b.WriteString(sAccent.Render("Which habit are you quitting?") + "\n\n")
	for i, h := range store.Habits {
		cursor, st := "  ", sText
		if i == m.habitCursor {
			cursor, st = sSel.Render("> "), sSel
		}
		status := sDim.Render("not tracked")
		if hs, ok := m.habit(h); ok {
			status = sGood.Render("clean for " + compact(hs.Elapsed(m.now)))
		}
		b.WriteString(cursor + st.Render(fmt.Sprintf("%-10s", habitTitle(h))) + "  " + status + "\n")
	}
	b.WriteString("\n")
	keys := []string{key("j/k", "move"), key("enter", "select")}
	if len(m.tracked()) > 0 {
		keys = append(keys, key("esc", "back"))
	}
	b.WriteString(help(append(keys, key("t", "time zone"), key("q", "quit"))...))
	out := lipgloss.JoinHorizontal(lipgloss.Top, b.String(), "    ", m.owl(-1))
	if m.flash != "" {
		out = lipgloss.JoinVertical(lipgloss.Left, sBad.Render(m.flash), "", out)
	}
	return out
}

func (m Model) buttons(yes, no string) string {
	y, n := sButton, sBtnOn
	if m.yes {
		y, n = sBtnOn, sButton
	}
	return y.Render(yes) + "  " + n.Render(no)
}

func (m Model) viewConfirm() string {
	var title, text, yes, no string
	switch m.screen {
	case scrConfirmStart:
		title = "Start the counter: " + habitTitle(m.pending)
		text = fmt.Sprintf("The timer starts this very second:\n%s  (%s)\n\nClosing the app does not stop it: time is always\ncounted from this starting moment.",
			sAccent.Render(m.now.In(m.loc).Format(dateTimeLayout)), m.tzName)
		yes, no = "Start", "Cancel"
	case scrConfirmRelapse:
		hs, _ := m.habit(m.active)
		unlocked := achievements.Unlocked(hs.Elapsed(m.now))
		title = "Relapsed? " + habitTitle(m.active)
		text = fmt.Sprintf("This wipes everything for this habit:\n  - the counter (%s) goes back to zero\n  - every badge is taken away (%d/%d earned)\n\nNothing is kept. You will have to start the counter again.",
			sAccent.Render(compact(hs.Elapsed(m.now))), unlocked, len(achievements.All))
		yes, no = "Yes, I relapsed", "No, go back"
	}
	panel := sPanel.Render(lipgloss.JoinVertical(lipgloss.Left,
		sAccent.Render(title), "", sText.Render(text), "", m.buttons(yes, no)))
	hint := help(key("<-/->", "choose"), key("enter", "ok"), key("y/n", "yes/no"), key("esc", "cancel"))
	return lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.JoinVertical(lipgloss.Left, panel, "", hint), "    ", m.owl(-1))
}

// ---------- dashboard ----------

func (m Model) viewTabs() string {
	var tabs []string
	for _, h := range store.Habits {
		title := habitTitle(h)
		if _, ok := m.habit(h); !ok {
			title += " (off)"
		}
		st := sTabOff
		if h == m.active {
			st = sTabOn
		}
		tabs = append(tabs, st.Render(title))
	}
	return strings.Join(tabs, " ") + sDim.Render("   <- tab ->")
}

// viewIdleTab is the dashboard of a habit that is not tracked yet.
func (m Model) viewIdleTab() string {
	text := lipgloss.JoinVertical(lipgloss.Left,
		sAccent.Render("Not tracking "+strings.ToLower(habitTitle(m.active))+" yet."),
		"",
		sText.Render("Press enter to start the counter right now."),
		sDim.Render("Time is taken from the network, not from this computer."),
	)
	return lipgloss.JoinVertical(lipgloss.Left,
		m.viewTabs(),
		"",
		lipgloss.JoinHorizontal(lipgloss.Top, text, "    ", m.owl(-1)),
		"",
		help(key("tab", "habit"), key("enter", "start"), key("t", "time zone"), key("q", "quit")),
	)
}

func (m Model) progressBar(p float64, width int) string {
	if p > 1 {
		p = 1
	}
	filled := int(p * float64(width))
	bar := strings.Repeat("#", filled)
	rest := width - filled
	if rest > 0 {
		bar += string(`|/-\`[m.frame/2%4])
		rest--
	}
	return sDim.Render("[") + sGood.Render(bar) + sDim.Render(strings.Repeat(".", rest)+"]") +
		sText.Render(fmt.Sprintf(" %5.1f%%", p*100))
}

func (m Model) viewDash() string {
	hs, ok := m.habit(m.active)
	if !ok {
		return m.viewIdleTab()
	}
	elapsed := hs.Elapsed(m.now)
	unlocked := achievements.Unlocked(elapsed)

	var b strings.Builder
	b.WriteString(sDim.Render(habitFree(m.active)) + "\n")
	b.WriteString(sBig.Render(strings.Join(art.BigText(clock(elapsed)), "\n")) + "\n")
	b.WriteString(sText.Render(words(elapsed)) + "\n\n")
	b.WriteString(sDim.Render("Started:  ") + sText.Render(hs.StartedAt.In(m.loc).Format(dateTimeLayout)) + "\n\n")

	if next, ok := achievements.Next(elapsed); ok {
		b.WriteString(sDim.Render("Next badge: ") + sAccent.Render(next.Name) +
			sDim.Render("  ("+milestone(next.Threshold)+")") + "\n")
		b.WriteString(m.progressBar(achievements.Progress(elapsed), 34) + "\n")
		at := hs.StartedAt.Add(next.Threshold)
		b.WriteString(sText.Render(compact(next.Threshold-elapsed)) +
			sDim.Render(" to go, unlocks "+at.In(m.loc).Format(dateHMLayout)) + "\n\n")
	} else {
		b.WriteString(sAccent.Render("Every badge collected. You walked the whole road.") + "\n\n\n\n")
	}

	var row strings.Builder
	for i := range achievements.All {
		if i < unlocked {
			row.WriteString(lipgloss.NewStyle().Bold(true).Foreground(tierColor(i, m.frame)).Render("[*]"))
		} else {
			row.WriteString(sDim.Render("[ ]"))
		}
	}
	b.WriteString(sDim.Render(fmt.Sprintf("Badges %2d/%d  ", unlocked, len(achievements.All))) + row.String())

	rank := "Rookie"
	if unlocked > 0 {
		rank = achievements.All[unlocked-1].Name
	}
	right := lipgloss.JoinVertical(lipgloss.Center,
		m.owl(unlocked-1),
		sAccent.Render("~ "+rank+" ~"),
	)

	parts := []string{m.viewTabs(), ""}
	if m.notice != "" {
		parts = append(parts, sGood.Render(m.notice), "")
	}
	parts = append(parts,
		lipgloss.JoinHorizontal(lipgloss.Top, b.String(), "    ", right),
		"",
		help(key("tab", "habit"), key("a", "badges"), key("t", "time zone"),
			key("r", "relapsed"), key("q", "quit")),
	)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// ---------- gallery ----------

func (m Model) badge(a achievements.Achievement, unlocked, party bool) string {
	var lines []string
	if unlocked {
		fig := art.Owl(m.frame, a.Index)
		if party {
			fig = art.Party(m.frame, a.Index)
		}
		lines = art.Badge(fig, a.Name, milestoneUpper(a.Threshold), a.Motto, m.frame, true, badgeStyles(a.Index, m.frame, true))
	} else {
		lines = art.Badge(art.Sleeping(m.frame), a.Name, "LOCKED  -  "+milestoneUpper(a.Threshold),
			"Still asleep. Keep going.", 0, false, badgeStyles(a.Index, m.frame, false))
	}
	return strings.Join(lines, "\n")
}

func (m Model) viewGallery() string {
	hs, _ := m.habit(m.active)
	elapsed := hs.Elapsed(m.now)
	unlocked := achievements.Unlocked(elapsed)

	wide := m.w >= 116
	var b strings.Builder
	b.WriteString(sAccent.Render("Badges: "+habitTitle(m.active)) + sDim.Render(fmt.Sprintf("  %d/%d", unlocked, len(achievements.All))) + "\n\n")
	for _, a := range achievements.All {
		got := a.Index < unlocked
		cursor := "  "
		if a.Index == m.galleryCursor {
			cursor = sSel.Render("> ")
		}
		mark, st := sDim.Render("[ ]"), sDim
		if got {
			mark = lipgloss.NewStyle().Bold(true).Foreground(tierColor(a.Index, m.frame)).Render("[*]")
			st = sText
		}
		if a.Index == m.galleryCursor {
			st = sSel
		}
		row := cursor + mark + " " + st.Render(fmt.Sprintf("%-20s", a.Name))
		if wide {
			row += " " + sDim.Render(fmt.Sprintf("%-20s", milestone(a.Threshold))) + " " + m.badgeStatus(hs, a, elapsed)
		}
		b.WriteString(row + "\n")
	}
	if !wide {
		// Narrow terminals only fit the list next to the badge, so the
		// selected milestone's details go under the list.
		a := achievements.All[m.galleryCursor]
		b.WriteString("\n" + sDim.Render(milestone(a.Threshold)) + "\n" + m.badgeStatus(hs, a, elapsed) + "\n")
	}
	b.WriteString("\n" + help(key("j/k", "browse"), key("esc", "back")))

	a := achievements.All[m.galleryCursor]
	return lipgloss.JoinHorizontal(lipgloss.Top, b.String(), "  ", m.badge(a, a.Index < unlocked, false))
}

func (m Model) badgeStatus(hs store.HabitState, a achievements.Achievement, elapsed time.Duration) string {
	if elapsed >= a.Threshold {
		return sGood.Render(hs.StartedAt.Add(a.Threshold).In(m.loc).Format(dateLayout))
	}
	return sDim.Render("in " + short(a.Threshold-elapsed))
}

// ---------- overlays ----------

func (m Model) viewPopup(h store.Habit, idx int) string {
	a := achievements.All[idx]
	hs, _ := m.habit(h)
	bannerStyle := sAccent
	if m.frame/3%2 == 1 {
		bannerStyle = sBig
	}
	when := hs.StartedAt.Add(a.Threshold).In(m.loc).Format(dateHMLayout)
	keys := []string{key("enter", "accept")}
	if rest := m.pendingCount() - 1; rest > 0 {
		keys = append(keys, key("s", fmt.Sprintf("accept all (%d more)", rest)))
	}
	return lipgloss.JoinVertical(lipgloss.Center,
		bannerStyle.Render("*** ACHIEVEMENT UNLOCKED ***"),
		sDim.Render(habitTitle(h)+"  ·  "+milestone(a.Threshold)+"  ·  "+when),
		"",
		m.badge(a, true, true),
		"",
		help(keys...),
	)
}

func (m Model) viewFinale() string {
	var elapsed time.Duration
	if hs, ok := m.habit(m.finale); ok {
		elapsed = hs.Elapsed(m.now)
	}
	color := rainbow[(m.frame/2)%len(rainbow)]
	content := lipgloss.JoinVertical(lipgloss.Center,
		sOwl.Render(strings.Join(art.Party(m.frame, art.HatCount-1), "\n")),
		lipgloss.NewStyle().Bold(true).Foreground(color).Render("~ L A S T   D O S E ~"),
		sDim.Render(habitTitle(m.finale)+"  ·  "+compact(elapsed)),
		"",
		sText.Width(60).Align(lipgloss.Center).Render(m.opt.FinalMessage),
		"",
		help(key("enter", "close")),
	)
	return lipgloss.NewStyle().
		Border(finaleBorder).
		BorderForeground(color).
		Padding(1, 4).
		Render(content)
}
