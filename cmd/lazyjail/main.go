// Command lazyjail is a keyboard-driven terminal UI for fail2ban.
package main

import (
	"flag"
	"fmt"
	"io"
	"net/netip"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/batv45/lazyjail/internal/f2b"
	"github.com/batv45/lazyjail/internal/f2b/mock"
	"github.com/batv45/lazyjail/internal/ui"
)

// version is overridden at build time with -ldflags "-X main.version=...".
var version = "dev"

// Seams for tests.
var (
	geteuid    = os.Geteuid
	execSudo   = syscall.Exec
	runUI      = runProgram
	lookupSudo = func() (string, error) { return exec.LookPath("sudo") }
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run parses args and returns the process exit code.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lazyjail", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showVersion := fs.Bool("version", false, "print version and exit")
	demo := fs.Bool("demo", false, "run with fake in-memory data (no fail2ban, no root needed)")
	readOnly := fs.Bool("read-only", false, "disable ban/unban/start/stop/reload")
	sudo := fs.Bool("sudo", false, "re-run under sudo when not root")
	_ = fs.String("socket", "", "fail2ban socket path (used by the real backend)")
	refresh := fs.Duration("refresh", 2*time.Second, "refresh interval (0 disables)")
	noColor := fs.Bool("no-color", false, "disable colors (also honors NO_COLOR)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		_, _ = fmt.Fprintln(stdout, "lazyjail", version)
		return 0
	}
	if *sudo && !*demo && geteuid() != 0 {
		path, err := lookupSudo()
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "lazyjail: sudo bulunamadı:", err)
			return 1
		}
		exe, err := os.Executable()
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "lazyjail:", err)
			return 1
		}
		// Same TTY, sudo asks for the password itself.
		err = execSudo(path, append([]string{"sudo", "--", exe}, args...), os.Environ())
		_, _ = fmt.Fprintln(stderr, "lazyjail: sudo çalıştırılamadı:", err)
		return 1
	}
	if !*demo {
		_, _ = fmt.Fprintln(stderr, "lazyjail: gerçek fail2ban bağlantısı henüz hazır değil; --demo ile deneyin (bkz. TEK-452)")
		return 2
	}
	host, _ := os.Hostname()
	be := f2b.Backend(mock.New(time.Now()))
	opts := ui.Options{
		ReadOnly: *readOnly, Demo: true, Refresh: *refresh, Host: host,
		NoColor: *noColor || os.Getenv("NO_COLOR") != "", SSHClientIP: sshClientIP(os.Getenv("SSH_CONNECTION")),
	}
	if err := runUI(ui.New(be, opts)); err != nil {
		_, _ = fmt.Fprintln(stderr, "lazyjail:", err)
		return 1
	}
	return 0
}

func runProgram(m tea.Model) error {
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

// sshClientIP extracts the client address from SSH_CONNECTION ("cip cport sip sport").
func sshClientIP(v string) string {
	f := strings.Fields(v)
	if len(f) < 1 {
		return ""
	}
	a, err := netip.ParseAddr(f[0])
	if err != nil {
		return ""
	}
	return a.String()
}
