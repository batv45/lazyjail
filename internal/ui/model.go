// Package ui is the Bubble Tea front end of lazyjail. It only talks to f2b.Backend.
package ui

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/batv45/lazyjail/internal/f2b"
)

// Options configure the UI.
type Options struct {
	ReadOnly    bool
	Demo        bool
	NoColor     bool
	Refresh     time.Duration    // polling interval; 0 disables
	Host        string           // shown in the header
	SSHClientIP string           // from SSH_CONNECTION; banning it needs a second confirmation
	Now         func() time.Time // clock, injectable for tests
}

type mode int

const (
	modeMain mode = iota
	modeHelp
	modeSearch
	modeBan
	modeConfirm
)

type focus int

const (
	focusJails focus = iota
	focusBans
)

type jailRow struct {
	Name    string
	Running bool
	Failed  int
	Banned  int
}

type confirm struct {
	prompt string
	steps  int // y presses still needed
	warn   bool
	run    func() error
	done   string
}

// Model is the root Bubble Tea model.
type Model struct {
	be   f2b.Backend
	opts Options
	keys keyMap
	th   Theme
	help help.Model

	width, height int
	mode          mode
	focus         focus

	version string
	jails   []jailRow
	known   []string // every jail name seen so far, to show stopped jails
	jailSel int
	bans    []f2b.BanEntry
	cfg     f2b.JailConfig
	banSel  int
	checked map[string]bool

	jailFilter, banFilter string
	input                 textinput.Model
	inputErr              string
	conf                  confirm

	loaded bool
	fatal  error // not installed / not running / permission: replaces the main screen
	warn   string
	toast  string
}

// New returns the root model.
func New(be f2b.Backend, o Options) Model {
	if o.Now == nil {
		o.Now = time.Now
	}
	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 64
	return Model{
		be: be, opts: o, keys: newKeyMap(), th: NewTheme(o.NoColor), help: help.New(),
		width: 100, height: 30, checked: map[string]bool{}, input: ti,
	}
}

type dataMsg struct {
	version string
	jails   []jailRow
	bans    []f2b.BanEntry
	cfg     f2b.JailConfig
	err     error
}

type actionMsg struct {
	text string
	err  error
}

type tickMsg struct{}

// Init loads the first snapshot and starts polling.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.load(), m.tick())
}

func (m Model) tick() tea.Cmd {
	if m.opts.Refresh <= 0 {
		return nil
	}
	return tea.Tick(m.opts.Refresh, func(time.Time) tea.Msg { return tickMsg{} })
}

func ctx5() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

// load fetches one consistent snapshot. Stopped jails are the known names that Jails no longer reports.
func (m Model) load() tea.Cmd {
	be, known, want := m.be, append([]string(nil), m.known...), m.selectedName()
	return func() tea.Msg {
		ctx, cancel := ctx5()
		defer cancel()
		var d dataMsg
		if d.err = be.Ping(ctx); d.err != nil {
			return d
		}
		if d.version, d.err = be.Version(ctx); d.err != nil {
			return d
		}
		names, err := be.Jails(ctx)
		if err != nil {
			d.err = err
			return d
		}
		running := map[string]bool{}
		for _, n := range names {
			running[n] = true
			st, err := be.JailStatus(ctx, n)
			if err != nil {
				d.err = err
				return d
			}
			d.jails = append(d.jails, jailRow{Name: n, Running: true, Failed: st.CurrentlyFailed, Banned: st.CurrentlyBanned})
		}
		for _, n := range known {
			if !running[n] {
				d.jails = append(d.jails, jailRow{Name: n})
			}
		}
		sel := want
		if !running[sel] {
			sel = ""
			if len(names) > 0 {
				sel = names[0]
			}
		}
		if sel != "" {
			if d.bans, d.err = be.BannedWithTime(ctx, sel); d.err != nil {
				return d
			}
			d.cfg, d.err = be.JailConfig(ctx, sel)
		}
		return d
	}
}

func (m Model) selectedName() string {
	if j, ok := m.selectedJail(); ok {
		return j.Name
	}
	return ""
}

func (m Model) visibleJails() []jailRow {
	return filterRows(m.jails, m.jailFilter, func(j jailRow) string { return j.Name })
}

func (m Model) visibleBans() []f2b.BanEntry {
	return filterRows(m.bans, m.banFilter, func(b f2b.BanEntry) string { return b.IP })
}

func filterRows[T any](rows []T, q string, name func(T) string) []T {
	if q == "" {
		return rows
	}
	var out []T
	for _, r := range rows {
		if strings.Contains(strings.ToLower(name(r)), strings.ToLower(q)) {
			out = append(out, r)
		}
	}
	return out
}

func (m Model) selectedJail() (jailRow, bool) {
	v := m.visibleJails()
	if m.jailSel < 0 || m.jailSel >= len(v) {
		return jailRow{}, false
	}
	return v[m.jailSel], true
}

// targets are the checked IPs, or the one under the cursor.
func (m Model) targets() []string {
	var out []string
	for _, b := range m.visibleBans() {
		if m.checked[b.IP] {
			out = append(out, b.IP)
		}
	}
	if len(out) == 0 {
		if v := m.visibleBans(); m.banSel >= 0 && m.banSel < len(v) {
			out = append(out, v[m.banSel].IP)
		}
	}
	return out
}

func classify(err error) bool {
	return errors.Is(err, f2b.ErrNotInstalled) || errors.Is(err, f2b.ErrNotRunning) || errors.Is(err, f2b.ErrPermission)
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tickMsg:
		return m, tea.Batch(m.load(), m.tick())
	case dataMsg:
		return m.onData(msg), nil
	case actionMsg:
		m.mode = modeMain
		if msg.err != nil {
			m.toast = "hata: " + msg.err.Error()
		} else {
			m.toast = msg.text
		}
		return m, m.load()
	case tea.KeyMsg:
		return m.onKey(msg)
	}
	return m, nil
}

func (m Model) onData(d dataMsg) Model {
	if d.err != nil {
		if classify(d.err) {
			m.fatal = d.err
		} else {
			m.warn = d.err.Error()
		}
		return m
	}
	m.fatal, m.warn, m.loaded = nil, "", true
	m.version, m.jails, m.bans, m.cfg = d.version, d.jails, d.bans, d.cfg
	prev := m.selectedName()
	for _, j := range d.jails {
		if !contains(m.known, j.Name) {
			m.known = append(m.known, j.Name)
		}
	}
	for i, j := range m.visibleJails() {
		if j.Name == prev {
			m.jailSel = i
		}
	}
	m.jailSel = clamp(m.jailSel, len(m.visibleJails()))
	m.banSel = clamp(m.banSel, len(m.visibleBans()))
	return m
}

func (m Model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeHelp:
		m.mode = modeMain
		return m, nil
	case modeConfirm:
		return m.onConfirmKey(msg)
	case modeBan:
		return m.onBanKey(msg)
	case modeSearch:
		return m.onSearchKey(msg)
	}
	m.toast = ""
	if m.fatal != nil {
		switch s := msg.String(); {
		case s == "r" || s == "f5" || s == "ctrl+r":
			return m, m.load()
		case s == "q" || s == "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}
	k := m.keys
	if m.opts.ReadOnly && k.isWrite(msg) {
		m.toast = "salt okunur mod: yazma işlemleri kapalı (--read-only)"
		return m, nil
	}
	switch {
	case msg.String() == "ctrl+c" || msg.String() == "q":
		return m, tea.Quit
	case msg.String() == "esc":
		if m.focus == focusJails {
			m.jailFilter = ""
		} else {
			m.banFilter = ""
		}
	case keyIs(msg, k.Help):
		m.mode = modeHelp
	case keyIs(msg, k.Refresh):
		return m, m.load()
	case keyIs(msg, k.Focus):
		m.focus = 1 - m.focus
	case keyIs(msg, k.Down):
		return m.move(1)
	case keyIs(msg, k.Up):
		return m.move(-1)
	case keyIs(msg, k.Search):
		m.mode = modeSearch
		m.input.Reset()
		m.input.SetValue(m.currentFilter())
		m.input.Focus()
	case keyIs(msg, k.Select):
		if m.focus == focusBans {
			if v := m.visibleBans(); m.banSel < len(v) {
				ip := v[m.banSel].IP
				if m.checked[ip] {
					delete(m.checked, ip)
				} else {
					m.checked[ip] = true
				}
			}
		}
	case keyIs(msg, k.Ban):
		if _, ok := m.selectedJail(); !ok {
			m.toast = "önce çalışan bir jail seçin"
			return m, nil
		}
		m.mode = modeBan
		m.inputErr = ""
		m.input.Reset()
		m.input.Focus()
	case keyIs(msg, k.Unban), keyIs(msg, k.UnbanAll):
		return m.askUnban(keyIs(msg, k.UnbanAll))
	case keyIs(msg, k.Toggle):
		return m.askToggle()
	case keyIs(msg, k.Reload):
		if j, ok := m.selectedJail(); ok && j.Running {
			return m.ask(fmt.Sprintf("%s reload edilsin mi?", j.Name), 1, false, func() error {
				ctx, c := ctx5()
				defer c()
				return m.be.Reload(ctx, j.Name)
			}, "reload edildi: "+j.Name), nil
		}
	case keyIs(msg, k.ReloadAll):
		return m.ask("TÜM jail'ler reload edilsin mi?", 1, false, func() error {
			ctx, c := ctx5()
			defer c()
			return m.be.Reload(ctx, "")
		}, "tüm jail'ler reload edildi"), nil
	}
	return m, nil
}

func (m Model) currentFilter() string {
	if m.focus == focusJails {
		return m.jailFilter
	}
	return m.banFilter
}

func (m Model) move(d int) (tea.Model, tea.Cmd) {
	if m.focus == focusBans {
		m.banSel = clamp(m.banSel+d, len(m.visibleBans()))
		return m, nil
	}
	old := m.jailSel
	m.jailSel = clamp(m.jailSel+d, len(m.visibleJails()))
	if m.jailSel != old {
		m.banSel = 0
		m.checked = map[string]bool{}
		return m, m.load()
	}
	return m, nil
}

func (m Model) onSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.mode = modeMain
		return m, nil
	case "esc":
		m.input.SetValue("")
		m.mode = modeMain
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.focus == focusJails {
		m.jailFilter = m.input.Value()
		m.jailSel = clamp(m.jailSel, len(m.visibleJails()))
	} else {
		m.banFilter = m.input.Value()
		m.banSel = clamp(m.banSel, len(m.visibleBans()))
	}
	return m, cmd
}

// ask opens a confirmation. done is the toast shown on success.
func (m Model) ask(prompt string, steps int, warn bool, run func() error, done string) Model {
	m.mode = modeConfirm
	m.conf = confirm{prompt: prompt, steps: steps, warn: warn, run: run, done: done}
	return m
}

func (m Model) onConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() != "y" {
		m.mode = modeMain
		m.toast = "iptal edildi"
		return m, nil
	}
	if m.conf.steps > 1 {
		m.conf.steps--
		m.conf.prompt = "EMİN MİSİNİZ? " + m.conf.prompt
		return m, nil
	}
	run, done := m.conf.run, m.conf.done
	m.mode = modeMain
	return m, func() tea.Msg {
		if err := run(); err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{text: done}
	}
}

func (m Model) askUnban(all bool) (tea.Model, tea.Cmd) {
	j, ok := m.selectedJail()
	ips := m.targets()
	if !ok || len(ips) == 0 {
		m.toast = "unban için banlı bir IP seçin"
		return m, nil
	}
	jail, steps, where := j.Name, 1, j.Name
	if all {
		jail, steps, where = "", 2, "TÜM jail'ler"
	}
	prompt := fmt.Sprintf("Unban %s — %s? ", strings.Join(ips, ", "), where)
	m = m.ask(prompt, steps, all, func() error {
		ctx, c := ctx5()
		defer c()
		for _, ip := range ips {
			if err := m.be.Unban(ctx, jail, ip); err != nil {
				return err
			}
		}
		return nil
	}, fmt.Sprintf("unban: %s", strings.Join(ips, ", ")))
	m.checked = map[string]bool{}
	return m, nil
}

func (m Model) askToggle() (tea.Model, tea.Cmd) {
	j, ok := m.selectedJail()
	if !ok {
		return m, nil
	}
	if !j.Running {
		return m, m.act(func() error {
			ctx, c := ctx5()
			defer c()
			return m.be.StartJail(ctx, j.Name)
		}, "başlatıldı: "+j.Name)
	}
	return m.ask(fmt.Sprintf("%s durdurulsun mu?", j.Name), 1, false, func() error {
		ctx, c := ctx5()
		defer c()
		return m.be.StopJail(ctx, j.Name)
	}, "durduruldu: "+j.Name), nil
}

func (m Model) act(run func() error, done string) tea.Cmd {
	return func() tea.Msg {
		if err := run(); err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{text: done}
	}
}

func (m Model) onBanKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeMain
		return m, nil
	case "enter":
		ip := strings.TrimSpace(m.input.Value())
		addr, err := netip.ParseAddr(ip)
		if err != nil {
			m.inputErr = "geçersiz IP adresi"
			return m, nil
		}
		j, _ := m.selectedJail()
		ip = addr.String()
		run := func() error {
			ctx, c := ctx5()
			defer c()
			return m.be.Ban(ctx, j.Name, ip)
		}
		done := fmt.Sprintf("banlandı: %s (%s)", ip, j.Name)
		if m.opts.SSHClientIP != "" && ip == m.opts.SSHClientIP {
			return m.ask("DİKKAT: bu SİZİN SSH IP'niz! Yine de banlansın mı?", 2, true, run, done), nil
		}
		m.mode = modeMain
		return m, m.act(run, done)
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.inputErr = ""
	return m, cmd
}

func clamp(i, n int) int {
	if i >= n {
		i = n - 1
	}
	if i < 0 {
		i = 0
	}
	return i
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func keyIs(msg tea.KeyMsg, b ...key.Binding) bool { return key.Matches(msg, b...) }
