// Package real is the production f2b.Backend: socket for everything it can do,
// fail2ban-client / fail2ban-regex for the rest (docs/PLAN.md §3.2).
package real

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/batv45/lazyjail/internal/f2b"
	"github.com/batv45/lazyjail/internal/f2b/execbk"
	"github.com/batv45/lazyjail/internal/f2b/socket"
)

// Config selects the socket and the tools; zero values use the defaults.
type Config struct {
	Socket      string        // empty = auto-discover
	ClientBin   string        // fail2ban-client
	RegexBin    string        // fail2ban-regex
	SocketTime  time.Duration // per socket call, default 5 s
	ExecTimeout time.Duration // per exec call, default 30 s
}

// Backend combines the socket and exec halves.
type Backend struct {
	*socket.Backend
	*execbk.Exec
}

var _ f2b.Backend = (*Backend)(nil)

// New builds a Backend from cfg.
func New(cfg Config) *Backend {
	sb := socket.New(cfg.Socket, cfg.SocketTime)
	return &Backend{
		Backend: sb,
		Exec: &execbk.Exec{
			Client: cfg.ClientBin, Regex: cfg.RegexBin,
			Socket: sb.Path(), Timeout: cfg.ExecTimeout,
		},
	}
}

// Ping adds one distinction the socket cannot make: a missing socket with no
// fail2ban-client installed means "not installed", not "not running".
func (b *Backend) Ping(ctx context.Context) error {
	err := b.Backend.Ping(ctx)
	if errors.Is(err, f2b.ErrNotRunning) {
		bin := b.Client
		if bin == "" {
			bin = "fail2ban-client"
		}
		if _, lerr := exec.LookPath(bin); lerr != nil {
			return fmt.Errorf("%w: %w", f2b.ErrNotInstalled, err)
		}
	}
	return err
}

// StartJail starts a jail. fail2ban removes a jail from the server on `stop`, and
// `start <jail>` then fails with "does not exist": only a full reload re-reads the
// config and re-adds it. In that case reload and check the jail came back, so a
// jail that is disabled in the config is reported instead of silently skipped.
func (b *Backend) StartJail(ctx context.Context, jail string) error {
	err := b.Exec.StartJail(ctx, jail)
	var ce *f2b.CommandError
	if !errors.As(err, &ce) || !strings.Contains(ce.Msg, "does not exist") {
		return err
	}
	if rerr := b.Reload(ctx, ""); rerr != nil {
		return rerr
	}
	jails, jerr := b.Jails(ctx)
	if jerr != nil {
		return jerr
	}
	for _, j := range jails {
		if j == jail {
			return nil
		}
	}
	return &f2b.CommandError{Msg: fmt.Sprintf("jail %q is not defined or not enabled in the fail2ban config", jail)}
}
