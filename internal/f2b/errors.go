package f2b

import "errors"

// Sentinel errors every Backend implementation wraps, so the UI can pick the right screen.
var (
	ErrNotInstalled = errors.New("fail2ban is not installed")
	ErrNotRunning   = errors.New("fail2ban server is not running")
	ErrPermission   = errors.New("permission denied for fail2ban socket")
	ErrUnsupported  = errors.New("operation not supported by this fail2ban version")
)

// CommandError is a non-zero reply from fail2ban; Msg is the exception text.
type CommandError struct {
	Msg string
}

func (e *CommandError) Error() string { return e.Msg }
