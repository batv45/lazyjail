// Package execbk runs fail2ban-client and fail2ban-regex for the operations
// that read fail2ban config files (start, reload) or parse filters (regex test).
// Arguments are passed as an argv slice, never through a shell.
package execbk

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/batv45/lazyjail/internal/f2b"
	"github.com/batv45/lazyjail/internal/validate"
)

// DefaultTimeout is longer than the socket's: reload parses every config file.
const DefaultTimeout = 30 * time.Second

// Exec runs the fail2ban command line tools.
type Exec struct {
	Client  string // fail2ban-client binary; empty = "fail2ban-client"
	Regex   string // fail2ban-regex binary; empty = "fail2ban-regex"
	Socket  string // passed as -s to fail2ban-client when set
	Timeout time.Duration
}

func (e *Exec) client() string {
	if e.Client == "" {
		return "fail2ban-client"
	}
	return e.Client
}

func (e *Exec) regex() string {
	if e.Regex == "" {
		return "fail2ban-regex"
	}
	return e.Regex
}

// StartJail starts (and loads, if needed) a jail.
func (e *Exec) StartJail(ctx context.Context, jail string) error {
	if err := validate.Jail(jail); err != nil {
		return err
	}
	_, err := e.run(ctx, e.client(), e.clientArgs("start", jail)...)
	return err
}

// Reload reloads one jail, or all jails when jail is empty.
func (e *Exec) Reload(ctx context.Context, jail string) error {
	args := []string{"reload"}
	if jail != "" {
		if err := validate.Jail(jail); err != nil {
			return err
		}
		args = append(args, jail)
	}
	_, err := e.run(ctx, e.client(), e.clientArgs(args[0], args[1:]...)...)
	return err
}

// clientArgs puts "--" before the command so no later value is parsed as an option.
func (e *Exec) clientArgs(cmd string, rest ...string) []string {
	var a []string
	if e.Socket != "" {
		a = append(a, "-s", e.Socket)
	}
	a = append(a, "--", cmd)
	return append(a, rest...)
}

var (
	linesRe = regexp.MustCompile(`Lines: (\d+) lines, (\d+) ignored, (\d+) matched, (\d+) missed`)
	// logTimeRe strips fail2ban's "2026-11-25 20:19:09,696 fail2ban [47]: " log prefix.
	logTimeRe = regexp.MustCompile(`^\d{4}-\d\d-\d\d \d\d:\d\d:\d\d,\d+ \S+\s+\[\d+\]: `)
)

// TestRegex runs fail2ban-regex with a log file or a single line against a
// filter name or a regex. Both arguments go in as plain argv values after "--".
func (e *Exec) TestRegex(ctx context.Context, logOrLine, filterOrRegex string) (f2b.RegexResult, error) {
	var res f2b.RegexResult
	if logOrLine == "" || filterOrRegex == "" {
		return res, errors.New("log/line and filter/regex are both required")
	}
	if strings.ContainsRune(logOrLine+filterOrRegex, 0) {
		return res, errors.New("NUL byte in argument")
	}
	out, err := e.run(ctx, e.regex(), "--print-all-matched", "--print-all-missed", "--", logOrLine, filterOrRegex)
	if err != nil {
		return res, err
	}
	m := linesRe.FindStringSubmatch(out)
	if m == nil {
		return res, errors.New("fail2ban-regex: no result summary in output")
	}
	res.Lines, _ = strconv.Atoi(m[1])
	res.Matched, _ = strconv.Atoi(m[3])
	res.Missed, _ = strconv.Atoi(m[4])
	res.MatchedLines = section(out, "|- Matched line(s):")
	res.MissedLines = section(out, "|- Missed line(s):")
	return res, nil
}

// section returns the "|  <line>" entries under header, up to the closing "`-".
func section(out, header string) []string {
	i := strings.Index(out, header)
	if i < 0 {
		return nil
	}
	var lines []string
	for _, l := range strings.Split(out[i+len(header):], "\n")[1:] {
		if strings.HasPrefix(l, "`-") {
			break
		}
		lines = append(lines, strings.TrimPrefix(l, "|  "))
	}
	return lines
}

// run executes name with args and returns its combined output. A non-zero exit becomes a
// *f2b.CommandError carrying the tool's error text.
func (e *Exec) run(ctx context.Context, name string, args ...string) (string, error) {
	if _, ok := ctx.Deadline(); !ok {
		t := e.Timeout
		if t == 0 {
			t = DefaultTimeout
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t)
		defer cancel()
	}
	// One buffer for both streams keeps the tool's own line order.
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout, cmd.Stderr = &out, &out
	cmd.Env = append(cmd.Environ(), "LC_ALL=C")
	err := cmd.Run()
	if err == nil {
		return out.String(), nil
	}
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("%s: %w", name, f2b.ErrNotInstalled)
	}
	if ctx.Err() != nil {
		return "", fmt.Errorf("%s: %w", name, ctx.Err())
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		msg := errorText(out.String())
		ce := &f2b.CommandError{Msg: msg}
		switch {
		case strings.Contains(msg, "Permission denied"):
			return "", fmt.Errorf("%w: %w", f2b.ErrPermission, ce)
		case strings.Contains(msg, "Could not find server"), strings.Contains(msg, "Unable to contact server"):
			return "", fmt.Errorf("%w: %w", f2b.ErrNotRunning, ce)
		}
		return "", ce
	}
	return "", fmt.Errorf("%s: %w", name, err)
}

// errorText picks the ERROR entries of a tool's output (with their unprefixed
// continuation lines), else its last non-empty line.
func errorText(out string) string {
	var errs []string
	last := ""
	for _, raw := range strings.Split(out, "\n") {
		l := strings.TrimSpace(raw)
		if l == "" {
			continue
		}
		isLog := logTimeRe.MatchString(l)
		l = strings.TrimSpace(logTimeRe.ReplaceAllString(l, ""))
		last = l
		switch {
		case strings.HasPrefix(l, "ERROR"):
			l = strings.TrimSpace(strings.TrimPrefix(l, "ERROR"))
			errs = append(errs, strings.TrimSpace(strings.TrimPrefix(l, ":")))
		case len(errs) > 0 && !isLog && !strings.HasPrefix(l, "WARNING"):
			errs[len(errs)-1] += " " + l
		}
	}
	if len(errs) > 0 {
		return strings.Join(errs, "; ")
	}
	if last == "" {
		return "command failed"
	}
	return last
}
