// Package socket implements the socket half of f2b.Backend: everything that
// does not need fail2ban-client to read config files.
package socket

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/batv45/lazyjail/internal/f2b"
	"github.com/batv45/lazyjail/internal/f2b/proto"
)

// DefaultTimeout applies when the caller's context has no deadline.
const DefaultTimeout = 5 * time.Second

// Paths are the fail2ban socket locations, in lookup order.
var Paths = []string{"/var/run/fail2ban/fail2ban.sock", "/run/fail2ban/fail2ban.sock"}

// Discover returns the first existing default socket, or the first default if none exists.
func Discover() string {
	for _, p := range Paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return Paths[0]
}

// Client sends one command per connection.
type Client struct {
	Path    string
	Timeout time.Duration
}

// Call sends cmd and returns the decoded result. A non-zero reply becomes
// *f2b.CommandError; transport failures wrap the f2b sentinel errors.
func (c *Client) Call(ctx context.Context, cmd ...string) (interface{}, error) {
	if _, ok := ctx.Deadline(); !ok {
		t := c.Timeout
		if t == 0 {
			t = DefaultTimeout
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t)
		defer cancel()
	}
	req, err := proto.EncodeCommand(cmd)
	if err != nil {
		return nil, err
	}
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", c.Path)
	if err != nil {
		return nil, mapDialError(c.Path, err)
	}
	defer func() { _ = conn.Close() }()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	if _, err := conn.Write(req); err != nil {
		return nil, fmt.Errorf("fail2ban socket write: %w", err)
	}
	frame, err := proto.ReadFrame(conn, proto.MaxReply)
	if err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) || ctx.Err() != nil {
			return nil, fmt.Errorf("fail2ban socket: %w", context.DeadlineExceeded)
		}
		return nil, fmt.Errorf("fail2ban socket read: %w", err)
	}
	code, res, err := proto.DecodeReply(frame)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		msg, _ := res.(string)
		ce := &f2b.CommandError{Msg: msg}
		if msg == "Invalid command" {
			return nil, fmt.Errorf("%w: %w", f2b.ErrUnsupported, ce)
		}
		return nil, ce
	}
	return res, nil
}

func mapDialError(path string, err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, syscall.ECONNREFUSED):
		return fmt.Errorf("%s: %w", path, f2b.ErrNotRunning)
	case errors.Is(err, fs.ErrPermission):
		return fmt.Errorf("%s: %w", path, f2b.ErrPermission)
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("fail2ban socket %s: %w", path, context.DeadlineExceeded)
	}
	return fmt.Errorf("fail2ban socket %s: %w", path, err)
}
