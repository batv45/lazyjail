// Package mock is an in-memory f2b.Backend used by --demo and the UI tests.
package mock

import (
	"context"
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/batv45/lazyjail/internal/f2b"
)

type jail struct {
	running bool
	total   int
	failed  int
	bans    []f2b.BanEntry
	cfg     f2b.JailConfig
}

// Backend is a fake fail2ban with realistic data. It is safe for concurrent use.
type Backend struct {
	mu    sync.Mutex
	order []string
	jails map[string]*jail
	err   error
}

// New returns a Backend with four jails; timestamps are relative to now.
func New(now time.Time) *Backend {
	b := &Backend{jails: map[string]*jail{}}
	add := func(name string, running bool, failed, total int, bans []f2b.BanEntry, cfg f2b.JailConfig) {
		b.order = append(b.order, name)
		b.jails[name] = &jail{running: running, failed: failed, total: total, bans: bans, cfg: cfg}
	}
	ban := func(ip string, ago, left time.Duration) f2b.BanEntry {
		return f2b.BanEntry{IP: ip, BannedAt: now.Add(-ago), ExpiresAt: now.Add(left)}
	}
	local := []string{"127.0.0.1/8", "::1"}
	add("sshd", true, 2, 41, []f2b.BanEntry{
		ban("185.220.101.4", 8*time.Minute, 2*time.Minute),
		ban("45.95.147.12", 15*time.Minute, 55*time.Minute),
		ban("2001:db8::7", 30*time.Minute, 4*time.Hour),
	}, f2b.JailConfig{
		BanTime: 10 * time.Minute, FindTime: 10 * time.Minute, MaxRetry: 5,
		LogPaths: []string{"/var/log/auth.log"}, IgnoreIPs: local, Actions: []string{"iptables-multiport"},
		FailRegex: []string{"^Failed password for .* from <HOST>"},
	})
	add("nginx-http-auth", true, 0, 12, []f2b.BanEntry{
		ban("203.0.113.50", 5*time.Minute, 25*time.Minute),
		ban("198.51.100.23", 20*time.Minute, 10*time.Minute),
	}, f2b.JailConfig{
		BanTime: 30 * time.Minute, FindTime: 10 * time.Minute, MaxRetry: 3,
		LogPaths: []string{"/var/log/nginx/error.log"}, IgnoreIPs: local, Actions: []string{"iptables-multiport"},
		FailRegex: []string{"user \"\\S+\":? (?:password mismatch|was not found)"},
	})
	add("recidive", true, 0, 4, []f2b.BanEntry{
		ban("192.0.2.99", 2*time.Hour, 6*24*time.Hour),
	}, f2b.JailConfig{
		BanTime: 7 * 24 * time.Hour, FindTime: 24 * time.Hour, MaxRetry: 3,
		LogPaths: []string{"/var/log/fail2ban.log"}, IgnoreIPs: local, Actions: []string{"iptables-allports"},
		FailRegex: []string{"\\[(?!recidive)\\S+\\]: Ban <HOST>"},
	})
	add("postfix", true, 1, 7, nil, f2b.JailConfig{
		BanTime: time.Hour, FindTime: 10 * time.Minute, MaxRetry: 5,
		LogPaths: []string{"/var/log/mail.log"}, IgnoreIPs: local, Actions: []string{"iptables-multiport"},
		FailRegex: []string{"warning: \\S+\\[<HOST>\\]: SASL \\S+ authentication failed"},
	})
	return b
}

// SetError makes every following call fail with err (nil restores normal behaviour).
func (b *Backend) SetError(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.err = err
}

func (b *Backend) get(jailName string) (*jail, error) {
	if b.err != nil {
		return nil, b.err
	}
	j, ok := b.jails[jailName]
	if !ok || !j.running {
		return nil, &f2b.CommandError{Msg: fmt.Sprintf("UnknownJailException('%s')", jailName)}
	}
	return j, nil
}

func (b *Backend) Ping(context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.err
}

func (b *Backend) Version(context.Context) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return "1.1.0", b.err
}

func (b *Backend) Jails(context.Context) ([]string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.err != nil {
		return nil, b.err
	}
	var names []string
	for _, n := range b.order {
		if b.jails[n].running {
			names = append(names, n)
		}
	}
	return names, nil
}

func (b *Backend) JailStatus(_ context.Context, name string) (f2b.JailStatus, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	j, err := b.get(name)
	if err != nil {
		return f2b.JailStatus{}, err
	}
	st := f2b.JailStatus{
		Name: name, CurrentlyFailed: j.failed, TotalFailed: j.total * 9,
		CurrentlyBanned: len(j.bans), TotalBanned: j.total, FileList: j.cfg.LogPaths,
	}
	for _, e := range j.bans {
		st.BannedIPs = append(st.BannedIPs, e.IP)
	}
	return st, nil
}

func (b *Backend) BannedWithTime(_ context.Context, name string) ([]f2b.BanEntry, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	j, err := b.get(name)
	if err != nil {
		return nil, err
	}
	return append([]f2b.BanEntry(nil), j.bans...), nil
}

func (b *Backend) JailConfig(_ context.Context, name string) (f2b.JailConfig, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	j, err := b.get(name)
	if err != nil {
		return f2b.JailConfig{}, err
	}
	return j.cfg, nil
}

func (b *Backend) Ban(_ context.Context, name, ip string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	j, err := b.get(name)
	if err != nil {
		return err
	}
	if _, perr := netip.ParseAddr(ip); perr != nil {
		return &f2b.CommandError{Msg: "Invalid IP address: " + ip}
	}
	for _, e := range j.bans {
		if e.IP == ip {
			return nil
		}
	}
	now := time.Now()
	j.bans = append(j.bans, f2b.BanEntry{IP: ip, BannedAt: now, ExpiresAt: now.Add(j.cfg.BanTime)})
	j.total++
	return nil
}

func (b *Backend) Unban(_ context.Context, name, ip string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.err != nil {
		return b.err
	}
	targets := []string{name}
	if name == "" {
		targets = b.order
	}
	for _, n := range targets {
		j, err := b.get(n)
		if err != nil {
			if name == "" {
				continue
			}
			return err
		}
		kept := j.bans[:0]
		for _, e := range j.bans {
			if e.IP != ip {
				kept = append(kept, e)
			}
		}
		j.bans = kept
	}
	return nil
}

func (b *Backend) StartJail(_ context.Context, name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.err != nil {
		return b.err
	}
	j, ok := b.jails[name]
	if !ok {
		return &f2b.CommandError{Msg: fmt.Sprintf("jail %q not found in configuration", name)}
	}
	j.running = true
	return nil
}

func (b *Backend) StopJail(_ context.Context, name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	j, err := b.get(name)
	if err != nil {
		return err
	}
	j.running = false
	j.bans = nil
	return nil
}

func (b *Backend) Reload(context.Context, string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.err
}

func (b *Backend) AddIgnoreIP(_ context.Context, name, ip string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	j, err := b.get(name)
	if err != nil {
		return err
	}
	j.cfg.IgnoreIPs = append(j.cfg.IgnoreIPs, ip)
	return nil
}

func (b *Backend) DelIgnoreIP(_ context.Context, name, ip string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	j, err := b.get(name)
	if err != nil {
		return err
	}
	kept := j.cfg.IgnoreIPs[:0]
	for _, v := range j.cfg.IgnoreIPs {
		if v != ip {
			kept = append(kept, v)
		}
	}
	j.cfg.IgnoreIPs = kept
	return nil
}

func (b *Backend) LogTarget(context.Context) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return "/var/log/fail2ban.log", b.err
}

func (b *Backend) TestRegex(_ context.Context, logOrLine, filterOrRegex string) (f2b.RegexResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.err != nil {
		return f2b.RegexResult{}, b.err
	}
	lines := strings.Split(logOrLine, "\n")
	res := f2b.RegexResult{Lines: len(lines)}
	for _, l := range lines {
		if strings.Contains(l, strings.TrimSpace(filterOrRegex)) {
			res.Matched++
			res.MatchedLines = append(res.MatchedLines, l)
		} else {
			res.Missed++
			res.MissedLines = append(res.MissedLines, l)
		}
	}
	sort.Strings(res.MatchedLines)
	return res, nil
}

var _ f2b.Backend = (*Backend)(nil)
