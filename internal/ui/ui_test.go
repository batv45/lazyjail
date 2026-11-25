package ui

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/batv45/lazyjail/internal/f2b"
	"github.com/batv45/lazyjail/internal/f2b/mock"
)

var update = flag.Bool("update", false, "rewrite golden files")

var fixedNow = time.Date(2026, 11, 21, 19, 5, 0, 0, time.UTC)

func newModel(t *testing.T, o Options) (Model, *mock.Backend) {
	t.Helper()
	be := mock.New(fixedNow)
	o.Now = func() time.Time { return fixedNow }
	o.NoColor = true
	m := New(be, o)
	m.width, m.height = 100, 30
	return settle(m, m.Init()), be
}

// settle runs commands (and the messages they produce) until none are left.
func settle(m Model, cmd tea.Cmd) Model {
	for cmd != nil {
		msg := cmd()
		if b, ok := msg.(tea.BatchMsg); ok {
			for _, c := range b {
				m = settle(m, c)
			}
			return m
		}
		var next tea.Model
		next, cmd = m.Update(msg)
		m = next.(Model)
	}
	return m
}

func press(m Model, keys ...string) Model {
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "space":
			msg = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		next, cmd := m.Update(msg)
		m = settle(next.(Model), cmd)
	}
	return m
}

func typeText(m Model, s string) Model {
	for _, r := range s {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(Model)
	}
	return m
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update)", err)
	}
	if string(want) != got {
		t.Fatalf("%s differs from golden:\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestGoldenMain(t *testing.T) {
	m, _ := newModel(t, Options{Host: "pi5"})
	m.opts.Refresh = 2 * time.Second // set after settling: polling would never end
	golden(t, "main", m.View())
}

func TestGoldenHelp(t *testing.T) {
	m, _ := newModel(t, Options{})
	golden(t, "help", press(m, "?").View())
}

func TestGoldenNarrow(t *testing.T) {
	m, _ := newModel(t, Options{})
	m.width = 80
	golden(t, "narrow", m.View())
}

func TestTooNarrow(t *testing.T) {
	m, _ := newModel(t, Options{})
	m.width = 40
	if !strings.Contains(m.View(), "çok dar") {
		t.Fatal("expected narrow warning")
	}
}

func TestGoldenEmpty(t *testing.T) {
	m, be := newModel(t, Options{})
	m = press(m, "j", "j", "j") // postfix has no bans
	_ = be
	golden(t, "empty", m.View())
}

func TestGoldenErrorScreens(t *testing.T) {
	for name, err := range map[string]error{
		"permission":    fmt.Errorf("socket: %w", f2b.ErrPermission),
		"not_running":   fmt.Errorf("socket: %w", f2b.ErrNotRunning),
		"not_installed": f2b.ErrNotInstalled,
	} {
		be := mock.New(fixedNow)
		be.SetError(err)
		m := New(be, Options{Now: func() time.Time { return fixedNow }, NoColor: true})
		m = settle(m, m.Init())
		if m.fatal == nil {
			t.Fatalf("%s: no fatal screen", name)
		}
		golden(t, name, m.View())
	}
}

func TestFatalRecovers(t *testing.T) {
	be := mock.New(fixedNow)
	be.SetError(f2b.ErrNotRunning)
	m := New(be, Options{Now: func() time.Time { return fixedNow }, NoColor: true})
	m = settle(m, m.Init())
	be.SetError(nil)
	m = press(m, "r")
	if m.fatal != nil || len(m.jails) != 4 {
		t.Fatalf("did not recover: fatal=%v jails=%d", m.fatal, len(m.jails))
	}
}

func TestOtherErrorKeepsLastData(t *testing.T) {
	m, be := newModel(t, Options{})
	be.SetError(&f2b.CommandError{Msg: "boom"})
	m = press(m, "f5")
	if m.fatal != nil || m.warn != "boom" || len(m.jails) != 4 {
		t.Fatalf("fatal=%v warn=%q jails=%d", m.fatal, m.warn, len(m.jails))
	}
}

func TestReadOnlyBlocksWrites(t *testing.T) {
	m, be := newModel(t, Options{ReadOnly: true})
	m = press(m, "tab", "u")
	if !strings.Contains(m.toast, "salt okunur") || m.mode != modeMain {
		t.Fatalf("toast=%q mode=%v", m.toast, m.mode)
	}
	for _, k := range []string{"b", "U", "s", "r", "R"} {
		m = press(m, k)
		if m.mode != modeMain {
			t.Fatalf("%s opened a dialog in read-only mode", k)
		}
	}
	bans, _ := be.BannedWithTime(context.Background(), "sshd")
	if len(bans) != 3 {
		t.Fatalf("bans changed: %d", len(bans))
	}
}

func bansOf(t *testing.T, be *mock.Backend, jail string) []f2b.BanEntry {
	t.Helper()
	b, err := be.BannedWithTime(context.Background(), jail)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestUnbanNeedsConfirmation(t *testing.T) {
	m, be := newModel(t, Options{})
	m = press(m, "tab", "u")
	if m.mode != modeConfirm || !strings.Contains(m.conf.prompt, "185.220.101.4") {
		t.Fatalf("mode=%v prompt=%q", m.mode, m.conf.prompt)
	}
	m = press(m, "n")
	if len(bansOf(t, be, "sshd")) != 3 {
		t.Fatal("unban ran without confirmation")
	}
	m = press(m, "u", "y")
	if got := bansOf(t, be, "sshd"); len(got) != 2 || len(m.bans) != 2 {
		t.Fatalf("bans after unban = %d / ui %d", len(got), len(m.bans))
	}
}

func TestMultiSelectUnban(t *testing.T) {
	m, be := newModel(t, Options{})
	m = press(m, "tab", "space", "j", "space", "u", "y")
	if got := bansOf(t, be, "sshd"); len(got) != 1 || got[0].IP != "2001:db8::7" {
		t.Fatalf("remaining = %+v", got)
	}
}

func TestUnbanAllNeedsTwoConfirmations(t *testing.T) {
	m, be := newModel(t, Options{})
	_ = be.Ban(context.Background(), "nginx-http-auth", "185.220.101.4")
	m = press(m, "tab", "U", "y")
	if m.mode != modeConfirm {
		t.Fatal("second confirmation expected")
	}
	if len(bansOf(t, be, "nginx-http-auth")) != 3 {
		t.Fatal("ran after first y")
	}
	m = press(m, "y")
	if len(bansOf(t, be, "sshd")) != 2 || len(bansOf(t, be, "nginx-http-auth")) != 2 {
		t.Fatal("IP not removed from every jail")
	}
}

func TestBanFlow(t *testing.T) {
	m, be := newModel(t, Options{})
	m = press(m, "b")
	m = typeText(m, "not-an-ip")
	m = press(m, "enter")
	if m.mode != modeBan || m.inputErr == "" {
		t.Fatalf("invalid IP accepted: mode=%v err=%q", m.mode, m.inputErr)
	}
	m = press(m, "esc", "b")
	m = typeText(m, "192.0.2.200")
	m = press(m, "enter")
	found := false
	for _, b := range bansOf(t, be, "sshd") {
		found = found || b.IP == "192.0.2.200"
	}
	if !found || !strings.Contains(m.toast, "192.0.2.200") {
		t.Fatalf("not banned: toast=%q", m.toast)
	}
}

func TestBanOwnSSHIPAsksTwice(t *testing.T) {
	m, be := newModel(t, Options{SSHClientIP: "203.0.113.77"})
	m = press(m, "b")
	m = typeText(m, "203.0.113.77")
	m = press(m, "enter")
	if m.mode != modeConfirm || !m.conf.warn || m.conf.steps != 2 {
		t.Fatalf("mode=%v conf=%+v", m.mode, m.conf)
	}
	m = press(m, "y")
	if len(bansOf(t, be, "sshd")) != 3 {
		t.Fatal("banned after a single confirmation")
	}
	press(m, "y")
	if len(bansOf(t, be, "sshd")) != 4 {
		t.Fatal("not banned after two confirmations")
	}
}

func TestStopStartReload(t *testing.T) {
	m, be := newModel(t, Options{})
	m = press(m, "s")
	if m.mode != modeConfirm {
		t.Fatal("stop must be confirmed")
	}
	m = press(m, "y")
	names, _ := be.Jails(context.Background())
	if len(names) != 3 || m.jails[len(m.jails)-1].Running {
		t.Fatalf("running=%v rows=%+v", names, m.jails)
	}
	// the stopped jail is still listed (dimmed) and 's' starts it without a prompt
	m = press(m, "j", "j", "j", "s")
	if names, _ = be.Jails(context.Background()); len(names) != 4 {
		t.Fatalf("start failed: %v", names)
	}
	m = press(m, "r")
	if m.mode != modeConfirm {
		t.Fatal("reload must be confirmed")
	}
	m = press(m, "y")
	if !strings.Contains(m.toast, "reload") {
		t.Fatalf("toast=%q", m.toast)
	}
	if m = press(m, "R"); m.mode != modeConfirm || !strings.Contains(m.conf.prompt, "TÜM") {
		t.Fatalf("global reload prompt: %q", m.conf.prompt)
	}
}

func TestSearchFiltersFocusedList(t *testing.T) {
	m, _ := newModel(t, Options{})
	m = press(m, "tab", "/")
	m = typeText(m, "45.")
	m = press(m, "enter")
	if v := m.visibleBans(); len(v) != 1 || v[0].IP != "45.95.147.12" {
		t.Fatalf("visible = %+v", v)
	}
	m = press(m, "esc")
	if len(m.visibleBans()) != 3 {
		t.Fatal("esc must clear the filter")
	}
	m = press(m, "tab", "/")
	m = typeText(m, "ng")
	if v := m.visibleJails(); len(v) != 1 || v[0].Name != "nginx-http-auth" {
		t.Fatalf("jails = %+v", v)
	}
}

func TestNavigationLoadsSelectedJail(t *testing.T) {
	m, _ := newModel(t, Options{})
	m = press(m, "j")
	if m.selectedName() != "nginx-http-auth" || len(m.bans) != 2 || m.cfg.MaxRetry != 3 {
		t.Fatalf("sel=%s bans=%d cfg=%+v", m.selectedName(), len(m.bans), m.cfg)
	}
}

func TestQuit(t *testing.T) {
	m, _ := newModel(t, Options{})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("no quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("q must quit")
	}
}

func TestErrorsAreSentinelAware(t *testing.T) {
	if !classify(fmt.Errorf("x: %w", f2b.ErrPermission)) || classify(errors.New("other")) || classify(&f2b.CommandError{Msg: "m"}) {
		t.Fatal("classify")
	}
}
