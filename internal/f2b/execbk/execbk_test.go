package execbk

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/batv45/lazyjail/internal/f2b"
)

var ctx = context.Background()

// script writes an executable that records its argv to <dir>/args and prints
// the contents of outFile (or nothing), then exits with code.
func script(t *testing.T, outFile string, code int) (bin, argsFile string) {
	t.Helper()
	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args")
	body := "#!/bin/sh\nfor a in \"$@\"; do printf '%s\\n' \"$a\"; done > '" + argsFile + "'\n"
	if outFile != "" {
		abs, _ := filepath.Abs(outFile)
		body += "cat '" + abs + "'\n"
	}
	body += "exit " + string(rune('0'+code)) + "\n"
	bin = filepath.Join(dir, "tool")
	if err := os.WriteFile(bin, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, argsFile
}

func argv(t *testing.T, f string) []string {
	t.Helper()
	b, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

func equal(a, b []string) bool { return strings.Join(a, "\x00") == strings.Join(b, "\x00") }

func TestStartAndReloadArgv(t *testing.T) {
	bin, args := script(t, "", 0)
	e := &Exec{Client: bin, Socket: "/run/x.sock"}
	if err := e.StartJail(ctx, "sshd"); err != nil {
		t.Fatal(err)
	}
	if got := argv(t, args); !equal(got, []string{"-s", "/run/x.sock", "--", "start", "sshd"}) {
		t.Fatal(got)
	}
	if err := e.Reload(ctx, "sshd"); err != nil {
		t.Fatal(err)
	}
	if got := argv(t, args); !equal(got, []string{"-s", "/run/x.sock", "--", "reload", "sshd"}) {
		t.Fatal(got)
	}
	if err := e.Reload(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if got := argv(t, args); !equal(got, []string{"-s", "/run/x.sock", "--", "reload"}) {
		t.Fatal(got)
	}
	// Without a socket no -s is passed.
	e.Socket = ""
	_ = e.Reload(ctx, "")
	if got := argv(t, args); !equal(got, []string{"--", "reload"}) {
		t.Fatal(got)
	}
}

func TestInvalidJailNeverRuns(t *testing.T) {
	bin, args := script(t, "", 0)
	e := &Exec{Client: bin}
	for _, j := range []string{"", "-x", "a b", "a;rm -rf /"} {
		if err := e.StartJail(ctx, j); err == nil {
			t.Errorf("StartJail(%q) should fail", j)
		}
	}
	if err := e.Reload(ctx, "--all"); err == nil {
		t.Error("Reload(--all) should fail")
	}
	if _, err := os.Stat(args); err == nil {
		t.Fatal("the tool must not have been run")
	}
}

func TestClientFailures(t *testing.T) {
	bin, _ := script(t, "testdata/reload_bad.txt", 2)
	e := &Exec{Client: bin}
	err := e.Reload(ctx, "nosuch")
	var ce *f2b.CommandError
	if !errors.As(err, &ce) || ce.Msg != "Failed during configuration: No section: 'nosuch'" {
		t.Fatalf("got %v", err)
	}

	for out, want := range map[string]error{
		"ERROR Permission denied: '/run/fail2ban/fail2ban.sock'": f2b.ErrPermission,
		"ERROR   Could not find server":                          f2b.ErrNotRunning,
		"ERROR  Unable to contact server. Is it running?":        f2b.ErrNotRunning,
	} {
		f := filepath.Join(t.TempDir(), "out")
		_ = os.WriteFile(f, []byte(out+"\n"), 0o644)
		bin, _ := script(t, f, 1)
		if err := (&Exec{Client: bin}).Reload(ctx, ""); !errors.Is(err, want) {
			t.Errorf("%q: got %v want %v", out, err, want)
		}
	}

	bin, _ = script(t, "", 1) // no output at all
	if err := (&Exec{Client: bin}).Reload(ctx, ""); err == nil || !strings.Contains(err.Error(), "command failed") {
		t.Fatalf("got %v", err)
	}
}

func TestMissingBinary(t *testing.T) {
	e := &Exec{Client: "/nonexistent/fail2ban-client", Regex: "/nonexistent/fail2ban-regex"}
	if err := e.Reload(ctx, ""); !errors.Is(err, f2b.ErrNotInstalled) {
		t.Fatal(err)
	}
	if _, err := e.TestRegex(ctx, "line", "re"); !errors.Is(err, f2b.ErrNotInstalled) {
		t.Fatal(err)
	}
	// Defaults resolve through PATH; the binary name itself is what matters.
	if (&Exec{}).client() != "fail2ban-client" || (&Exec{}).regex() != "fail2ban-regex" {
		t.Fatal("default binary names")
	}
}

func TestTimeout(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "slow")
	_ = os.WriteFile(bin, []byte("#!/bin/sh\nexec sleep 5\n"), 0o755)
	e := &Exec{Client: bin, Timeout: 100 * time.Millisecond}
	start := time.Now()
	if err := e.Reload(ctx, ""); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("timeout not enforced")
	}
}

func TestTestRegexFile(t *testing.T) {
	bin, args := script(t, "testdata/regex_file.txt", 0)
	e := &Exec{Regex: bin}
	res, err := e.TestRegex(ctx, "/tmp/t.log", "sshd")
	if err != nil {
		t.Fatal(err)
	}
	if res.Lines != 2 || res.Matched != 1 || res.Missed != 1 {
		t.Fatalf("%+v", res)
	}
	if len(res.MatchedLines) != 1 || !strings.Contains(res.MatchedLines[0], "Failed password for root") {
		t.Fatalf("%q", res.MatchedLines)
	}
	if len(res.MissedLines) != 1 || !strings.HasSuffix(res.MissedLines[0], "unrelated line") {
		t.Fatalf("%q", res.MissedLines)
	}
	want := []string{"--print-all-matched", "--print-all-missed", "--", "/tmp/t.log", "sshd"}
	if got := argv(t, args); !equal(got, want) {
		t.Fatal(got)
	}
}

func TestTestRegexLine(t *testing.T) {
	bin, _ := script(t, "testdata/regex_line.txt", 0)
	res, err := (&Exec{Regex: bin}).TestRegex(ctx, "x failed from 1.2.3.4", "failed from <HOST>")
	if err != nil || res.Matched != 1 || res.Missed != 0 || len(res.MissedLines) != 0 {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestTestRegexErrors(t *testing.T) {
	bin, _ := script(t, "testdata/regex_bad.txt", 255)
	_, err := (&Exec{Regex: bin}).TestRegex(ctx, "x", "(unclosed")
	var ce *f2b.CommandError
	if !errors.As(err, &ce) || !strings.Contains(ce.Msg, "Unable to compile regular expression") ||
		!strings.Contains(ce.Msg, "missing )") {
		t.Fatalf("got %v", err)
	}
	// Exit 0 but no summary: treated as an error instead of a bogus empty result.
	bin, _ = script(t, "", 0)
	if _, err := (&Exec{Regex: bin}).TestRegex(ctx, "a", "b"); err == nil {
		t.Fatal("missing summary should fail")
	}
	e := &Exec{Regex: bin}
	for _, c := range [][2]string{{"", "x"}, {"x", ""}, {"a\x00b", "x"}} {
		if _, err := e.TestRegex(ctx, c[0], c[1]); err == nil {
			t.Errorf("TestRegex(%q, %q) should fail", c[0], c[1])
		}
	}
}

// Option-looking values reach the tool only after "--".
func TestRegexArgsAfterDoubleDash(t *testing.T) {
	bin, args := script(t, "testdata/regex_line.txt", 0)
	_, err := (&Exec{Regex: bin}).TestRegex(ctx, "--help", "-v")
	if err != nil {
		t.Fatal(err)
	}
	got := argv(t, args)
	if got[2] != "--" || got[3] != "--help" || got[4] != "-v" {
		t.Fatal(got)
	}
}

func TestSectionEdgeCases(t *testing.T) {
	if section("no headers here", "|- Matched line(s):") != nil {
		t.Fatal("missing header")
	}
	got := section("|- Matched line(s):\n|  a\n|  b\n`-\n|  not mine", "|- Matched line(s):")
	if !equal(got, []string{"a", "b"}) {
		t.Fatal(got)
	}
}

func TestErrorTextContinuation(t *testing.T) {
	got := errorText("2026-11-25 20:23:24,212 fail2ban  [29]: ERROR   NOK: ('sshd',)\nSorry but the jail 'sshd' does not exist\n")
	if got != "NOK: ('sshd',) Sorry but the jail 'sshd' does not exist" {
		t.Fatal(got)
	}
	got = errorText("2026-11-25 20:23:24,212 fail2ban [29]: WARNING x\n2026-11-25 20:23:24,212 fail2ban [29]: ERROR a\n2026-11-25 20:23:24,212 fail2ban [29]: ERROR b\n")
	if got != "a; b" {
		t.Fatal(got)
	}
}
