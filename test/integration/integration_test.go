//go:build integration

// Package integration runs the real Backend against a real fail2ban server.
// It must run as root inside the container built from test/integration/docker
// (see `make integration`): the socket is root-only and /tmp/auth.log feeds the sshd jail.
package integration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/batv45/lazyjail/internal/f2b"
	"github.com/batv45/lazyjail/internal/f2b/real"
)

const authLog = "/tmp/auth.log"

func backend(t *testing.T) *real.Backend {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("integration tests need root and the fail2ban container (make integration)")
	}
	return real.New(real.Config{Socket: "/run/fail2ban/fail2ban.sock"})
}

func ctx(t *testing.T) context.Context {
	c, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	return c
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestServer(t *testing.T) {
	b, c := backend(t), ctx(t)
	if err := b.Ping(c); err != nil {
		t.Fatal(err)
	}
	v, err := b.Version(c)
	if err != nil || v == "" {
		t.Fatal(v, err)
	}
	t.Logf("fail2ban %s, caps %+v", v, f2b.CapsFor(v))
	jails, err := b.Jails(c)
	if err != nil || !contains(jails, "sshd") {
		t.Fatal(jails, err)
	}
	if lt, err := b.LogTarget(c); err != nil || lt == "" {
		t.Fatal(lt, err)
	}
	st, err := b.JailStatus(c, "sshd")
	if err != nil || len(st.FileList) != 1 || st.FileList[0] != authLog {
		t.Fatalf("%+v %v", st, err)
	}
	var ce *f2b.CommandError
	if _, err := b.JailStatus(c, "nosuchjail"); !errors.As(err, &ce) || !strings.Contains(ce.Msg, "nosuchjail") {
		t.Fatalf("unknown jail: %v", err)
	}
}

func TestJailConfig(t *testing.T) {
	b, c := backend(t), ctx(t)
	cfg, err := b.JailConfig(c, "sshd")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BanTime != 10*time.Minute || cfg.FindTime != 10*time.Minute || cfg.MaxRetry != 3 {
		t.Fatalf("%+v", cfg)
	}
	if len(cfg.LogPaths) != 1 || len(cfg.FailRegex) == 0 || len(cfg.Actions) != 1 || cfg.Actions[0] != "dummy" {
		t.Fatalf("%+v", cfg)
	}
}

func TestBanUnban(t *testing.T) {
	b, c := backend(t), ctx(t)
	const ip = "192.0.2.10"
	if err := b.Ban(c, "sshd", ip); err != nil {
		t.Fatal(err)
	}
	bans, err := b.BannedWithTime(c, "sshd")
	if err != nil || len(bans) != 1 || bans[0].IP != ip {
		t.Fatalf("%+v %v", bans, err)
	}
	if v, _ := b.Version(c); f2b.CapsFor(v).WithTime {
		if bans[0].BannedAt.IsZero() || bans[0].ExpiresAt.Sub(bans[0].BannedAt) != 10*time.Minute {
			t.Fatalf("times: %+v", bans[0])
		}
	}
	if err := b.Unban(c, "sshd", ip); err != nil {
		t.Fatal(err)
	}
	if bans, _ = b.BannedWithTime(c, "sshd"); len(bans) != 0 {
		t.Fatal(bans)
	}

	// Unban from every jail.
	if err := b.Ban(c, "sshd", ip); err != nil {
		t.Fatal(err)
	}
	if err := b.Unban(c, "", ip); err != nil {
		t.Fatal(err)
	}
	if st, _ := b.JailStatus(c, "sshd"); len(st.BannedIPs) != 0 {
		t.Fatalf("%+v", st)
	}

	// IPv6.
	if err := b.Ban(c, "sshd", "2001:db8::7"); err != nil {
		t.Fatal(err)
	}
	if st, _ := b.JailStatus(c, "sshd"); !contains(st.BannedIPs, "2001:db8::7") {
		t.Fatalf("%+v", st)
	}
	_ = b.Unban(c, "sshd", "2001:db8::7")
}

// A ban caused by log lines the jail's own filter matches, not by a command.
func TestBanFromLog(t *testing.T) {
	b, c := backend(t), ctx(t)
	const ip = "192.0.2.50"
	f, err := os.OpenFile(authLog, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	for i := 0; i < 4; i++ {
		_, _ = fmt.Fprintf(f, "%s testhost sshd[%d]: Failed password for root from %s port 4000%d ssh2\n",
			time.Now().Format("Jan _2 15:04:05"), 100+i, ip, i)
	}
	deadline := time.Now().Add(40 * time.Second)
	for {
		st, err := b.JailStatus(c, "sshd")
		if err != nil {
			t.Fatal(err)
		}
		if contains(st.BannedIPs, ip) {
			if st.TotalFailed < 3 || st.TotalBanned < 1 {
				t.Fatalf("counters: %+v", st)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no ban for %s after 40s: %+v", ip, st)
		}
		time.Sleep(time.Second)
	}
	if err := b.Unban(c, "sshd", ip); err != nil {
		t.Fatal(err)
	}
}

func TestIgnoreIP(t *testing.T) {
	b, c := backend(t), ctx(t)
	if err := b.AddIgnoreIP(c, "sshd", "198.51.100.0/24"); err != nil {
		t.Fatal(err)
	}
	cfg, err := b.JailConfig(c, "sshd")
	if err != nil || !contains(cfg.IgnoreIPs, "198.51.100.0/24") {
		t.Fatalf("%+v %v", cfg, err)
	}
	if err := b.DelIgnoreIP(c, "sshd", "198.51.100.0/24"); err != nil {
		t.Fatal(err)
	}
	if cfg, _ = b.JailConfig(c, "sshd"); contains(cfg.IgnoreIPs, "198.51.100.0/24") {
		t.Fatalf("%+v", cfg)
	}
}

func TestStopStartReload(t *testing.T) {
	b, c := backend(t), ctx(t)
	if err := b.StopJail(c, "sshd"); err != nil {
		t.Fatal(err)
	}
	if jails, _ := b.Jails(c); contains(jails, "sshd") {
		t.Fatalf("sshd still listed: %v", jails)
	}
	if err := b.StartJail(c, "sshd"); err != nil {
		t.Fatal(err)
	}
	if jails, _ := b.Jails(c); !contains(jails, "sshd") {
		t.Fatalf("sshd not started: %v", jails)
	}
	if err := b.Reload(c, "sshd"); err != nil {
		t.Fatal(err)
	}
	if err := b.Reload(c, ""); err != nil {
		t.Fatal(err)
	}
	var ce *f2b.CommandError
	if err := b.Reload(c, "nosuchjail"); !errors.As(err, &ce) {
		t.Fatalf("reload of an unknown jail: %v", err)
	}
	if jails, _ := b.Jails(c); !contains(jails, "sshd") {
		t.Fatalf("sshd lost after reload: %v", jails)
	}
}

func TestRegex(t *testing.T) {
	b, c := backend(t), ctx(t)
	log := "/tmp/regex-test.log"
	lines := time.Now().Format("Jan _2 15:04:05") + " h sshd[1]: Failed password for root from 192.0.2.5 port 22 ssh2\n" +
		time.Now().Format("Jan _2 15:04:05") + " h sshd[1]: unrelated\n"
	if err := os.WriteFile(log, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := b.TestRegex(c, log, "sshd")
	if err != nil || res.Lines != 2 || res.Matched != 1 || res.Missed != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	if len(res.MatchedLines) != 1 || len(res.MissedLines) != 1 {
		t.Fatalf("%+v", res)
	}
	res, err = b.TestRegex(c, "x failed from 1.2.3.4", "failed from <HOST>")
	if err != nil || res.Matched != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	var ce *f2b.CommandError
	if _, err := b.TestRegex(c, "x", "(unclosed"); !errors.As(err, &ce) {
		t.Fatalf("bad regex: %v", err)
	}
}

func TestNotRunning(t *testing.T) {
	b := real.New(real.Config{Socket: "/run/fail2ban/absent.sock"})
	if err := b.Ping(context.Background()); !errors.Is(err, f2b.ErrNotRunning) {
		t.Fatal(err)
	}
}
