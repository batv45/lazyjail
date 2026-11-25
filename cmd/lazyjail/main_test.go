package main

import (
	"bytes"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRunVersion(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"--version"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, errOut.String())
	}
	if got, want := out.String(), "lazyjail "+version+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRunWithoutDemoIsNotReady(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run(nil, &out, &errOut); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "--demo") {
		t.Fatalf("stderr %q should mention --demo", errOut.String())
	}
}

func TestRunDemoStartsUI(t *testing.T) {
	var started tea.Model
	old := runUI
	runUI = func(m tea.Model) error { started = m; return nil }
	defer func() { runUI = old }()
	var out, errOut bytes.Buffer
	if code := run([]string{"--demo", "--read-only", "--no-color"}, &out, &errOut); code != 0 {
		t.Fatalf("exit code = %d (stderr: %q)", code, errOut.String())
	}
	if started == nil {
		t.Fatal("UI was not started")
	}
}

func TestSudoReexec(t *testing.T) {
	oldEuid, oldExec, oldLook := geteuid, execSudo, lookupSudo
	defer func() { geteuid, execSudo, lookupSudo = oldEuid, oldExec, oldLook }()
	geteuid = func() int { return 1000 }
	lookupSudo = func() (string, error) { return "/usr/bin/sudo", nil }
	var gotPath string
	var gotArgs []string
	execSudo = func(path string, argv []string, _ []string) error {
		gotPath, gotArgs = path, argv
		return nil
	}
	var out, errOut bytes.Buffer
	run([]string{"--sudo", "--read-only"}, &out, &errOut)
	if gotPath != "/usr/bin/sudo" || len(gotArgs) < 5 || gotArgs[0] != "sudo" || gotArgs[1] != "--" || gotArgs[3] != "--sudo" {
		t.Fatalf("exec = %q %q", gotPath, gotArgs)
	}
}

func TestSudoSkippedWhenRoot(t *testing.T) {
	oldEuid, oldExec := geteuid, execSudo
	defer func() { geteuid, execSudo = oldEuid, oldExec }()
	geteuid = func() int { return 0 }
	execSudo = func(string, []string, []string) error { t.Fatal("must not exec as root"); return nil }
	var out, errOut bytes.Buffer
	if code := run([]string{"--sudo"}, &out, &errOut); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestSSHClientIP(t *testing.T) {
	if got := sshClientIP("203.0.113.5 5555 10.0.0.1 22"); got != "203.0.113.5" {
		t.Fatalf("got %q", got)
	}
	if got := sshClientIP(""); got != "" {
		t.Fatalf("got %q", got)
	}
}
