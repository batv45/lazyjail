package socket

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/batv45/lazyjail/internal/f2b"
	"github.com/batv45/lazyjail/internal/f2b/fake"
	"github.com/batv45/lazyjail/internal/f2b/proto"
)

// Unix socket paths are limited to ~108 bytes, so avoid long t.TempDir() names.
func newBackend(t *testing.T) (*Backend, *fake.Server) {
	t.Helper()
	dir, err := os.MkdirTemp("", "lj")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	srv, err := fake.Start(filepath.Join(dir, "f.sock"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	return New(srv.Path, 2*time.Second), srv
}

var ctx = context.Background()

func TestPingVersionLogTarget(t *testing.T) {
	b, srv := newBackend(t)
	if err := b.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	if v, err := b.Version(ctx); err != nil || v != "1.0.2" {
		t.Fatal(v, err)
	}
	if lt, err := b.LogTarget(ctx); err != nil || lt != "/var/log/fail2ban.log" {
		t.Fatal(lt, err)
	}
	srv.SetRawReply(mustReply(t, 42))
	if err := b.Ping(ctx); err == nil {
		t.Fatal("ping with odd reply should fail")
	}
	srv.SetRawReply(mustReply(t, 42))
	if _, err := b.Version(ctx); err == nil {
		t.Fatal("version with odd reply should fail")
	}
	srv.SetRawReply(mustReply(t, 42))
	if _, err := b.LogTarget(ctx); err == nil {
		t.Fatal("logtarget with odd reply should fail")
	}
}

func TestJailsAndStatus(t *testing.T) {
	b, srv := newBackend(t)
	jails, err := b.Jails(ctx)
	if err != nil || !reflect.DeepEqual(jails, []string{"nginx-http-auth", "sshd"}) {
		t.Fatal(jails, err)
	}
	if err := b.Ban(ctx, "sshd", "192.0.2.7"); err != nil {
		t.Fatal(err)
	}
	st, err := b.JailStatus(ctx, "sshd")
	if err != nil {
		t.Fatal(err)
	}
	want := f2b.JailStatus{Name: "sshd", CurrentlyBanned: 1, TotalBanned: 1,
		FileList: []string{"/var/log/auth.log"}, BannedIPs: []string{"192.0.2.7"}}
	if !reflect.DeepEqual(st, want) {
		t.Fatalf("got %+v want %+v", st, want)
	}
	srv.SetJail("empty", &fake.Jail{Banned: map[string]time.Time{}})
	srv.DeleteJail("sshd")
	srv.DeleteJail("nginx-http-auth")
	jails, _ = b.Jails(ctx)
	if !reflect.DeepEqual(jails, []string{"empty"}) {
		t.Fatal(jails)
	}
	srv.DeleteJail("empty")
	if jails, err = b.Jails(ctx); err != nil || len(jails) != 0 {
		t.Fatal(jails, err)
	}
	var ce *f2b.CommandError
	if _, err := b.JailStatus(ctx, "gone"); !errors.As(err, &ce) || !strings.Contains(ce.Msg, "gone") {
		t.Fatalf("want CommandError, got %v", err)
	}
}

func TestBanUnbanIgnore(t *testing.T) {
	b, srv := newBackend(t)
	for _, j := range []string{"sshd", "nginx-http-auth"} {
		if err := b.Ban(ctx, j, "192.0.2.9"); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.Unban(ctx, "sshd", "192.0.2.9"); err != nil {
		t.Fatal(err)
	}
	if got := srv.Banned("sshd"); len(got) != 0 {
		t.Fatal(got)
	}
	if err := b.Unban(ctx, "", "192.0.2.9"); err != nil {
		t.Fatal(err)
	}
	if got := srv.Banned("nginx-http-auth"); len(got) != 0 {
		t.Fatal(got)
	}
	if err := b.Unban(ctx, "sshd", "192.0.2.9"); err == nil {
		t.Fatal("unban of unbanned IP should fail")
	}

	if err := b.AddIgnoreIP(ctx, "sshd", "192.0.2.0/24"); err != nil {
		t.Fatal(err)
	}
	cfg, err := b.JailConfig(ctx, "sshd")
	if err != nil || !reflect.DeepEqual(cfg.IgnoreIPs, []string{"192.0.2.0/24"}) {
		t.Fatalf("%+v %v", cfg, err)
	}
	if err := b.DelIgnoreIP(ctx, "sshd", "192.0.2.0/24"); err != nil {
		t.Fatal(err)
	}
	if err := b.DelIgnoreIP(ctx, "sshd", "192.0.2.0/24"); err == nil {
		t.Fatal("deleting a missing entry should fail")
	}
	if err := b.StopJail(ctx, "sshd"); err != nil {
		t.Fatal(err)
	}
	if jails, _ := b.Jails(ctx); !reflect.DeepEqual(jails, []string{"nginx-http-auth"}) {
		t.Fatal(jails)
	}
	if err := b.StopJail(ctx, "sshd"); err == nil {
		t.Fatal("stopping a stopped jail should fail")
	}
}

func TestInputValidation(t *testing.T) {
	b, srv := newBackend(t)
	calls := map[string]error{
		"ban bad ip":     b.Ban(ctx, "sshd", "1.2.3.4; reboot"),
		"ban bad jail":   b.Ban(ctx, "-x", "1.2.3.4"),
		"unban bad ip":   b.Unban(ctx, "sshd", "nope"),
		"unban bad jail": b.Unban(ctx, "a b", "1.2.3.4"),
		"stop bad jail":  b.StopJail(ctx, ""),
		"add bad":        b.AddIgnoreIP(ctx, "sshd", "1.2.3.4/99"),
		"del bad jail":   b.DelIgnoreIP(ctx, "x y", "1.2.3.4"),
	}
	_, calls["status bad jail"] = b.JailStatus(ctx, "a;b")
	_, calls["banned bad jail"] = b.BannedWithTime(ctx, "")
	_, calls["config bad jail"] = b.JailConfig(ctx, "..;")
	for name, err := range calls {
		if err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if n := len(srv.Commands()); n != 0 {
		t.Fatalf("invalid input must not reach the server, got %d commands", n)
	}
}

func TestBannedWithTime(t *testing.T) {
	b, _ := newBackend(t)
	if err := b.Ban(ctx, "sshd", "192.0.2.1"); err != nil {
		t.Fatal(err)
	}
	if err := b.Ban(ctx, "sshd", "2001:db8::5"); err != nil {
		t.Fatal(err)
	}
	got, err := b.BannedWithTime(ctx, "sshd")
	if err != nil || len(got) != 2 {
		t.Fatal(got, err)
	}
	if got[0].IP != "192.0.2.1" || got[0].BannedAt.IsZero() {
		t.Fatalf("%+v", got[0])
	}
	if d := got[0].ExpiresAt.Sub(got[0].BannedAt); d != 600*time.Second {
		t.Fatalf("ban lasts %v", d)
	}
}

func TestBannedWithTimeFallbacks(t *testing.T) {
	// Old version: caps say no --with-time, bare IPs with zero times.
	b, srv := newBackend(t)
	srv.SetVersion("0.9.7")
	_ = b.Ban(ctx, "sshd", "192.0.2.1")
	got, err := b.BannedWithTime(ctx, "sshd")
	if err != nil || len(got) != 1 || got[0].IP != "192.0.2.1" || !got[0].BannedAt.IsZero() {
		t.Fatal(got, err)
	}
	// New version string but the server rejects the flag: same fallback.
	b, srv = newBackend(t)
	srv.SetNoWithTime(true)
	_ = b.Ban(ctx, "sshd", "192.0.2.2")
	got, err = b.BannedWithTime(ctx, "sshd")
	if err != nil || len(got) != 1 || got[0].IP != "192.0.2.2" {
		t.Fatal(got, err)
	}
	// Unknown jail still surfaces as an error.
	if _, err := b.BannedWithTime(ctx, "gone"); err == nil {
		t.Fatal("unknown jail should fail")
	}
}

func TestParseBanLines(t *testing.T) {
	got := parseBanLines([]string{
		"192.0.2.1 \t2026-11-25 20:14:16 + 600 = 2026-11-25 20:24:16",
		"192.0.2.2 \t2026-11-25 20:14:16 + -1 = 9999-12-31 23:59:59",
		"192.0.2.3 something unexpected",
		"",
	})
	if len(got) != 3 {
		t.Fatal(got)
	}
	if got[0].ExpiresAt.Sub(got[0].BannedAt) != 10*time.Minute {
		t.Fatalf("%+v", got[0])
	}
	if !got[1].ExpiresAt.IsZero() || got[1].BannedAt.IsZero() {
		t.Fatalf("permanent ban: %+v", got[1])
	}
	if got[2].IP != "192.0.2.3" || !got[2].BannedAt.IsZero() {
		t.Fatalf("%+v", got[2])
	}
}

func TestJailConfig(t *testing.T) {
	b, _ := newBackend(t)
	cfg, err := b.JailConfig(ctx, "nginx-http-auth")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BanTime != time.Hour || cfg.FindTime != 10*time.Minute || cfg.MaxRetry != 5 {
		t.Fatalf("%+v", cfg)
	}
	if !reflect.DeepEqual(cfg.LogPaths, []string{"/var/log/nginx/error.log"}) || len(cfg.FailRegex) != 1 ||
		!reflect.DeepEqual(cfg.Actions, []string{"iptables-multiport"}) {
		t.Fatalf("%+v", cfg)
	}
	if _, err := b.JailConfig(ctx, "gone"); err == nil {
		t.Fatal("unknown jail should fail")
	}
}

func TestCapsCached(t *testing.T) {
	b, srv := newBackend(t)
	srv.SetVersion("0.11.2")
	c, err := b.Caps(ctx)
	if err != nil || !c.Known || !c.WithTime {
		t.Fatal(c, err)
	}
	srv.SetVersion("0.9.0")
	if c2, _ := b.Caps(ctx); c2 != c {
		t.Fatal("caps should be cached")
	}
	b2 := New(filepath.Join(t.TempDir(), "missing"), time.Second)
	if _, err := b2.Caps(ctx); !errors.Is(err, f2b.ErrNotRunning) {
		t.Fatal(err)
	}
}

func TestServerException(t *testing.T) {
	b, srv := newBackend(t)
	srv.FailNext(errors.New("something broke"))
	var ce *f2b.CommandError
	if err := b.Ping(ctx); !errors.As(err, &ce) || ce.Msg != "something broke" {
		t.Fatal(err)
	}
	srv.FailNext(errors.New("Invalid command"))
	err := b.Ping(ctx)
	if !errors.Is(err, f2b.ErrUnsupported) || !errors.As(err, &ce) {
		t.Fatal(err)
	}
}

func TestTransportErrors(t *testing.T) {
	missing := New(filepath.Join(t.TempDir(), "nope.sock"), time.Second)
	if err := missing.Ping(ctx); !errors.Is(err, f2b.ErrNotRunning) {
		t.Fatalf("missing socket: %v", err)
	}

	// A socket file nobody listens on: connection refused.
	b, srv := newBackend(t)
	stale := srv.Path + ".stale"
	ln, err := net.Listen("unix", stale)
	if err != nil {
		t.Fatal(err)
	}
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	_ = ln.Close()
	if err := New(stale, time.Second).Ping(ctx); !errors.Is(err, f2b.ErrNotRunning) {
		t.Fatalf("stale socket: %v", err)
	}

	// Hung server: the 5 s default is replaced by the short test timeout.
	srv.SetHang(true)
	fast := New(srv.Path, 100*time.Millisecond)
	start := time.Now()
	if err := fast.Ping(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("hang: %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("timeout not honoured")
	}
	srv.SetHang(false)

	// A caller deadline wins over the default.
	srv.SetDelay(300 * time.Millisecond)
	dctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if err := b.Ping(dctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("ctx deadline: %v", err)
	}
	srv.SetDelay(0)

	// Garbage reply.
	srv.SetRawReply([]byte("not a pickle<F2B_END_COMMAND>"))
	if err := b.Ping(ctx); err == nil {
		t.Fatal("garbage reply should fail")
	}
	// Server closes without answering.
	srv.SetRawReply([]byte("half"))
	if err := b.Ping(ctx); err == nil {
		t.Fatal("truncated reply should fail")
	}
}

func TestPermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores socket permissions")
	}
	b, srv := newBackend(t)
	if err := os.Chmod(srv.Path, 0); err != nil {
		t.Fatal(err)
	}
	if err := b.Ping(ctx); !errors.Is(err, f2b.ErrPermission) {
		t.Fatalf("got %v", err)
	}
}

func TestDiscover(t *testing.T) {
	if got := Discover(); got != Paths[0] && got != Paths[1] {
		t.Fatal(got)
	}
	if New("", 0).Path() == "" {
		t.Fatal("empty path should be discovered")
	}
}

// mustReply is a well-formed reply carrying a value the typed accessors reject.
func mustReply(t *testing.T, v int) []byte {
	t.Helper()
	b, err := proto.EncodeReply(v, nil)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
