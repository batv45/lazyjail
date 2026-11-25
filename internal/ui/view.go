package ui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/batv45/lazyjail/internal/f2b"
)

const (
	minWidth   = 60
	wideWidth  = 100
	logBoxRows = 4
)

// View implements tea.Model.
func (m Model) View() string {
	switch {
	case m.width < minWidth:
		return m.th.Warn.Render(fmt.Sprintf("Terminal çok dar (%d sütun). En az %d sütun gerekli.", m.width, minWidth))
	case m.fatal != nil:
		return m.fatalView()
	case m.mode == modeHelp:
		return m.helpView()
	}
	header := m.headerView()
	body := m.bodyView()
	logBox := m.box("Log", []string{
		m.th.Dim.Render("canlı log akışı sonraki sürümde (TEK-452: log ekranı)"),
	}, m.width, logBoxRows, false)
	return lipgloss.JoinVertical(lipgloss.Left, header, body, logBox, m.statusLine(), m.footer())
}

func (m Model) headerView() string {
	parts := []string{"lazyjail"}
	if m.version != "" {
		parts = append(parts, "fail2ban "+m.version)
	}
	if m.opts.Host != "" {
		parts = append(parts, "host: "+m.opts.Host)
	}
	if m.opts.Demo {
		parts = append(parts, m.th.Warn.Render("DEMO (sahte veri)"))
	}
	if m.opts.ReadOnly {
		parts = append(parts, m.th.Warn.Render("salt okunur"))
	}
	if m.opts.Refresh > 0 {
		parts = append(parts, fmt.Sprintf("⟳ %s", m.opts.Refresh))
	}
	return m.th.Title.Render(" " + strings.Join(parts, " ─ "))
}

func (m Model) bodyView() string {
	h := m.height - 1 - logBoxRows - 2
	if h < 12 {
		h = 12
	}
	if m.width >= wideWidth {
		lw := 28
		return lipgloss.JoinHorizontal(lipgloss.Top, m.jailsBox(lw, h), m.detailView(m.width-lw, h))
	}
	jh := len(m.visibleJails()) + 3
	if jh > h/3+1 {
		jh = h/3 + 1
	}
	if jh < 5 {
		jh = 5
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.jailsBox(m.width, jh), m.detailView(m.width, h-jh))
}

func (m Model) jailsBox(w, h int) string {
	rows := m.visibleJails()
	lines := []string{m.th.Title.Render(fmt.Sprintf("Jails (%d)", len(m.jails))) + m.filterTag(m.jailFilter)}
	if !m.loaded && m.warn == "" {
		lines = append(lines, m.th.Dim.Render("yükleniyor…"))
	}
	start := window(m.jailSel, len(rows), h-3)
	for i := start; i < len(rows) && len(lines) < h-2; i++ {
		j := rows[i]
		dot, st := "●", m.th.Active
		counts := fmt.Sprintf("%2d %2d", j.Banned, j.Failed)
		if !j.Running {
			dot, st, counts = "○", m.th.Dim, " —  —"
		}
		line := fmt.Sprintf("%s %-*s %s", st.Render(dot), w-14, ansi.Truncate(j.Name, w-14, "…"), counts)
		lines = append(lines, m.cursor(line, i == m.jailSel, m.focus == focusJails))
	}
	return m.box("", lines, w, h, m.focus == focusJails)
}

func (m Model) detailView(w, h int) string {
	name := m.selectedName()
	if name == "" {
		return m.box("", []string{m.th.Dim.Render("çalışan jail yok")}, w, h, false)
	}
	cfgH := 8
	if h < 18 {
		cfgH = 6
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.bansBox(name, w, h-cfgH), m.cfgBox(w, cfgH))
}

func (m Model) bansBox(name string, w, h int) string {
	rows := m.visibleBans()
	title := fmt.Sprintf("%s ─ banlı IP'ler (%d)", name, len(m.bans))
	lines := []string{m.th.Title.Render(title) + m.filterTag(m.banFilter), m.th.Dim.Render(fmt.Sprintf("  %-22s %-17s %s", "IP", "Banlandı", "Süre"))}
	if len(rows) == 0 {
		lines = append(lines, m.th.Dim.Render("banlı IP yok"))
	}
	now := m.opts.Now()
	start := window(m.banSel, len(rows), h-4)
	for i := start; i < len(rows) && len(lines) < h-2; i++ {
		b := rows[i]
		mark := " "
		if m.checked[b.IP] {
			mark = m.th.Checked.Render("✓")
		}
		line := fmt.Sprintf("%s %-22s %-17s %s", mark, b.IP, fmtTime(b.BannedAt), left(b, now))
		lines = append(lines, m.cursor(line, i == m.banSel, m.focus == focusBans))
	}
	return m.box("", lines, w, h, m.focus == focusBans)
}

func (m Model) cfgBox(w, h int) string {
	c := m.cfg
	lines := []string{
		m.th.Title.Render("Config (salt okunur)"),
		fmt.Sprintf("bantime %s  findtime %s  maxretry %d", dur(c.BanTime), dur(c.FindTime), c.MaxRetry),
		"logpath " + strings.Join(c.LogPaths, " "),
		"ignoreip " + strings.Join(c.IgnoreIPs, " "),
		"actions " + strings.Join(c.Actions, " "),
	}
	if len(c.FailRegex) > 0 {
		lines = append(lines, "failregex "+c.FailRegex[0])
	}
	return m.box("", lines, w, h, false)
}

// box draws a bordered panel of exactly w×h cells. Lines are cut to fit, never wrapped.
func (m Model) box(_ string, lines []string, w, h int, focused bool) string {
	st := m.th.Border
	if focused {
		st = m.th.BorderFocus
	}
	inner := w - 2
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, inner-2, "…")
	}
	if len(lines) > h-2 {
		lines = lines[:h-2]
	}
	return st.Width(inner).Height(h - 2).Render(strings.Join(lines, "\n"))
}

func (m Model) cursor(line string, selected, focused bool) string {
	if !selected {
		return line
	}
	if focused {
		return m.th.Cursor.Render(ansi.Strip(line))
	}
	return m.th.Title.Render(ansi.Strip(line))
}

func (m Model) filterTag(f string) string {
	if f == "" {
		return ""
	}
	return m.th.Warn.Render("  /" + f)
}

func (m Model) statusLine() string {
	switch {
	case m.mode == modeSearch:
		return " /" + m.input.View()
	case m.mode == modeBan:
		j, _ := m.selectedJail()
		s := fmt.Sprintf(" Ban IP (%s): %s  [enter: onay, esc: iptal]", j.Name, m.input.View())
		if m.inputErr != "" {
			s += " " + m.th.Err.Render(m.inputErr)
		}
		return s
	case m.mode == modeConfirm:
		st := m.th.Warn
		if m.conf.warn {
			st = m.th.Err
		}
		return " " + st.Render(m.conf.prompt+"[y/N]")
	case m.warn != "":
		return " " + m.th.Err.Render("⚠ "+m.warn+" (son iyi veri gösteriliyor)")
	case m.toast != "":
		return " " + m.th.Active.Render(m.toast)
	}
	return " "
}

func (m Model) footer() string {
	return " " + m.help.ShortHelpView(m.keys.ShortHelp())
}

func (m Model) helpView() string {
	lines := []string{m.th.Title.Render("lazyjail — kısayollar"), ""}
	for _, g := range m.keys.FullHelp() {
		for _, b := range g {
			lines = append(lines, fmt.Sprintf("  %-10s %s", b.Help().Key, b.Help().Desc))
		}
		lines = append(lines, "")
	}
	lines = append(lines, m.th.Dim.Render("Herhangi bir tuş: geri dön"))
	return m.box("", lines, m.width, minInt(len(lines)+2, m.height), true)
}

func (m Model) fatalView() string {
	var title, hint string
	switch {
	case isErr(m.fatal, f2b.ErrNotInstalled):
		title, hint = "fail2ban kurulu değil", "Kurulum: sudo apt install fail2ban"
	case isErr(m.fatal, f2b.ErrNotRunning):
		title, hint = "fail2ban çalışmıyor", "Başlatın: sudo systemctl start fail2ban"
	default:
		title, hint = "Socket'e erişim yok", "sudo lazyjail ile ya da lazyjail --sudo ile çalıştırın. (Denemek için: lazyjail --demo)"
	}
	lines := []string{m.th.Err.Render(title), "", m.fatal.Error(), "", hint, "", m.th.Dim.Render("r: tekrar dene   q: çık")}
	return m.box("", lines, m.width, minInt(len(lines)+2, m.height), true)
}

func isErr(err, target error) bool { return errors.Is(err, target) }

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("2006-01-02 15:04")
}

func left(b f2b.BanEntry, now time.Time) string {
	if b.ExpiresAt.IsZero() {
		return "—"
	}
	d := b.ExpiresAt.Sub(now)
	if d <= 0 {
		return "süresi doldu"
	}
	return "kalan " + dur(d.Round(time.Minute))
}

func dur(d time.Duration) string {
	switch {
	case d <= 0:
		return "—"
	case d%(24*time.Hour) == 0:
		return fmt.Sprintf("%dd", d/(24*time.Hour))
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", d/time.Hour)
	case d%time.Minute == 0:
		return fmt.Sprintf("%dm", d/time.Minute)
	}
	return d.String()
}

// window returns the first visible row so that sel stays on screen.
func window(sel, n, size int) int {
	if size < 1 || n <= size {
		return 0
	}
	start := sel - size + 1
	if start < 0 {
		start = 0
	}
	if start > n-size {
		start = n - size
	}
	return start
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
