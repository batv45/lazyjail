package real

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/batv45/lazyjail/internal/f2b"
	"github.com/batv45/lazyjail/internal/f2b/fake"
)

var ctx = context.Background()

func TestCombinedBackend(t *testing.T) {
	dir, _ := os.MkdirTemp("", "lj")
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	srv, err := fake.Start(filepath.Join(dir, "f.sock"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)

	// Fake fail2ban-client: records argv; fake fail2ban-regex: canned summary.
	client := filepath.Join(dir, "client")
	_ = os.WriteFile(client, []byte("#!/bin/sh\necho \"$@\" > "+dir+"/client.args\n"), 0o755)
	regex := filepath.Join(dir, "regex")
	_ = os.WriteFile(regex, []byte("#!/bin/sh\necho 'Lines: 3 lines, 0 ignored, 2 matched, 1 missed'\n"), 0o755)

	var b f2b.Backend = New(Config{Socket: srv.Path, ClientBin: client, RegexBin: regex, SocketTime: time.Second})
	if err := b.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	if err := b.Ban(ctx, "sshd", "192.0.2.4"); err != nil {
		t.Fatal(err)
	}
	if got := srv.Banned("sshd"); len(got) != 1 {
		t.Fatal(got)
	}
	if err := b.Reload(ctx, "sshd"); err != nil {
		t.Fatal(err)
	}
	args, _ := os.ReadFile(dir + "/client.args")
	if !strings.Contains(string(args), "-s "+srv.Path+" -- reload sshd") {
		t.Fatalf("client args %q", args)
	}
	if err := b.StartJail(ctx, "sshd"); err != nil {
		t.Fatal(err)
	}
	res, err := b.TestRegex(ctx, "line", "re")
	if err != nil || res.Matched != 2 || res.Lines != 3 {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestPingNotInstalledVsNotRunning(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "none.sock")

	err := New(Config{Socket: missing, ClientBin: "/nonexistent/fail2ban-client"}).Ping(ctx)
	if !errors.Is(err, f2b.ErrNotInstalled) || !errors.Is(err, f2b.ErrNotRunning) {
		t.Fatalf("not installed: %v", err)
	}

	// With a client binary present but no server it is "not running" only.
	bin := filepath.Join(t.TempDir(), "client")
	_ = os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755)
	err = New(Config{Socket: missing, ClientBin: bin}).Ping(ctx)
	if !errors.Is(err, f2b.ErrNotRunning) || errors.Is(err, f2b.ErrNotInstalled) {
		t.Fatalf("not running: %v", err)
	}

	// Default binary name: result depends on the host PATH, but must stay a sentinel.
	if err := New(Config{Socket: missing}).Ping(ctx); !errors.Is(err, f2b.ErrNotRunning) {
		t.Fatalf("default: %v", err)
	}
}

func TestStartJailFallsBackToReload(t *testing.T) {
	dir, _ := os.MkdirTemp("", "lj")
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	srv, err := fake.Start(filepath.Join(dir, "f.sock"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	log := filepath.Join(dir, "log")
	// Like fail2ban 1.0.2: `start` of a removed jail fails, a plain `reload` works.
	client := filepath.Join(dir, "client")
	_ = os.WriteFile(client, []byte(`#!/bin/sh
echo "$@" >> `+log+`
case "$*" in
*"-- start"*) echo "ERROR   NOK: ('sshd',)"; echo "Sorry but the jail 'sshd' does not exist"; exit 255;;
esac
`), 0o755)
	b := New(Config{Socket: srv.Path, ClientBin: client})

	_ = b.StopJail(ctx, "sshd")
	srv.SetJail("sshd", &fake.Jail{Banned: map[string]time.Time{}}) // what the reload brings back
	if err := b.StartJail(ctx, "sshd"); err != nil {
		t.Fatal(err)
	}
	calls, _ := os.ReadFile(log)
	if !strings.Contains(string(calls), "start sshd") || !strings.HasSuffix(strings.TrimSpace(string(calls)), "-- reload") {
		t.Fatalf("calls:\n%s", calls)
	}

	// Disabled in config: the reload does not bring it back.
	srv.DeleteJail("sshd")
	var ce *f2b.CommandError
	if err := b.StartJail(ctx, "sshd"); !errors.As(err, &ce) || !strings.Contains(ce.Msg, "not defined or not enabled") {
		t.Fatalf("got %v", err)
	}

	// Other start errors pass through untouched; invalid names never run.
	_ = os.WriteFile(client, []byte("#!/bin/sh\necho 'ERROR boom'; exit 1\n"), 0o755)
	if err := b.StartJail(ctx, "sshd"); !errors.As(err, &ce) || ce.Msg != "boom" {
		t.Fatalf("got %v", err)
	}
	if err := b.StartJail(ctx, "-x"); err == nil {
		t.Fatal("invalid name")
	}

	// Reload failure and a dead socket during the check surface as errors.
	_ = os.WriteFile(client, []byte("#!/bin/sh\necho 'ERROR Sorry but the jail does not exist'; exit 1\n"), 0o755)
	if err := b.StartJail(ctx, "sshd"); err == nil {
		t.Fatal("failing reload should fail StartJail")
	}
	_ = os.WriteFile(client, []byte(`#!/bin/sh
case "$*" in *"-- start"*) echo "ERROR x"; echo "jail does not exist"; exit 1;; esac
`), 0o755)
	srv.Close()
	srv2 := New(Config{Socket: srv.Path, ClientBin: client, SocketTime: time.Second})
	if err := srv2.StartJail(ctx, "sshd"); !errors.Is(err, f2b.ErrNotRunning) {
		t.Fatalf("got %v", err)
	}
}
