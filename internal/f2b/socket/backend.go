package socket

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/batv45/lazyjail/internal/f2b"
	"github.com/batv45/lazyjail/internal/f2b/proto"
	"github.com/batv45/lazyjail/internal/validate"
)

// Backend is the socket part of f2b.Backend. StartJail, Reload and TestRegex
// live in package execbk because they read fail2ban config files.
type Backend struct {
	c *Client

	mu   sync.Mutex
	caps *f2b.Caps
}

// New returns a Backend talking to the socket at path (empty = Discover).
func New(path string, timeout time.Duration) *Backend {
	if path == "" {
		path = Discover()
	}
	return &Backend{c: &Client{Path: path, Timeout: timeout}}
}

// Path is the socket path in use.
func (b *Backend) Path() string { return b.c.Path }

func (b *Backend) Ping(ctx context.Context) error {
	res, err := b.c.Call(ctx, "ping")
	if err != nil {
		return err
	}
	if s, _ := proto.Str(res); s != "pong" {
		return fmt.Errorf("unexpected ping reply %v", res)
	}
	return nil
}

func (b *Backend) Version(ctx context.Context) (string, error) {
	res, err := b.c.Call(ctx, "version")
	if err != nil {
		return "", err
	}
	s, ok := proto.Str(res)
	if !ok {
		return "", fmt.Errorf("unexpected version reply %T", res)
	}
	return s, nil
}

// Caps returns the capabilities of the connected server; the result is cached
// after the first successful version query.
func (b *Backend) Caps(ctx context.Context) (f2b.Caps, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.caps != nil {
		return *b.caps, nil
	}
	v, err := b.Version(ctx)
	if err != nil {
		return f2b.Caps{}, err
	}
	c := f2b.CapsFor(v)
	b.caps = &c
	return c, nil
}

func (b *Backend) Jails(ctx context.Context) ([]string, error) {
	res, err := b.c.Call(ctx, "status")
	if err != nil {
		return nil, err
	}
	pairs, err := proto.Pairs(res)
	if err != nil {
		return nil, fmt.Errorf("status: %w", err)
	}
	for _, p := range pairs {
		if p.Key != "Jail list" {
			continue
		}
		s, _ := proto.Str(p.Value)
		var out []string
		for _, n := range strings.Split(s, ",") {
			if n = strings.TrimSpace(n); n != "" {
				out = append(out, n)
			}
		}
		return out, nil
	}
	return nil, errors.New("status reply has no jail list")
}

func (b *Backend) JailStatus(ctx context.Context, jail string) (f2b.JailStatus, error) {
	st := f2b.JailStatus{Name: jail}
	if err := validate.Jail(jail); err != nil {
		return st, err
	}
	res, err := b.c.Call(ctx, "status", jail)
	if err != nil {
		return st, err
	}
	sections, err := proto.Pairs(res)
	if err != nil {
		return st, fmt.Errorf("status %s: %w", jail, err)
	}
	for _, sec := range sections {
		pairs, err := proto.Pairs(sec.Value)
		if err != nil {
			return st, fmt.Errorf("status %s %s: %w", jail, sec.Key, err)
		}
		for _, p := range pairs {
			switch p.Key {
			case "Currently failed":
				st.CurrentlyFailed, _ = proto.Int(p.Value)
			case "Total failed":
				st.TotalFailed, _ = proto.Int(p.Value)
			case "Currently banned":
				st.CurrentlyBanned, _ = proto.Int(p.Value)
			case "Total banned":
				st.TotalBanned, _ = proto.Int(p.Value)
			case "File list":
				st.FileList, _ = proto.Strings(p.Value)
			case "Banned IP list":
				st.BannedIPs, _ = proto.Strings(p.Value)
			}
		}
	}
	return st, nil
}

// withTimeRe matches "<ip> \t<start> + <bantime> = <end>" lines of `banip --with-time`.
var withTimeRe = regexp.MustCompile(`^(\S+)\s+(\d{4}-\d\d-\d\d \d\d:\d\d:\d\d) \+ (-?\d+) = (\S+ \S+)$`)

// BannedWithTime lists bans with timestamps. Servers without --with-time
// return the bare IPs from `status <jail>` with zero times.
func (b *Backend) BannedWithTime(ctx context.Context, jail string) ([]f2b.BanEntry, error) {
	if err := validate.Jail(jail); err != nil {
		return nil, err
	}
	caps, err := b.Caps(ctx)
	if err != nil {
		return nil, err
	}
	if caps.WithTime {
		res, err := b.c.Call(ctx, "get", jail, "banip", "--with-time")
		if err == nil {
			lines, err := proto.Strings(res)
			if err != nil {
				return nil, fmt.Errorf("banip: %w", err)
			}
			return parseBanLines(lines), nil
		}
		if !errors.Is(err, f2b.ErrUnsupported) {
			return nil, err
		}
	}
	st, err := b.JailStatus(ctx, jail)
	if err != nil {
		return nil, err
	}
	out := make([]f2b.BanEntry, len(st.BannedIPs))
	for i, ip := range st.BannedIPs {
		out[i] = f2b.BanEntry{IP: ip}
	}
	return out, nil
}

func parseBanLines(lines []string) []f2b.BanEntry {
	out := make([]f2b.BanEntry, 0, len(lines))
	for _, l := range lines {
		m := withTimeRe.FindStringSubmatch(strings.TrimSpace(l))
		if m == nil {
			// Unknown format: keep the IP so the ban is still visible.
			if f := strings.Fields(l); len(f) > 0 {
				out = append(out, f2b.BanEntry{IP: f[0]})
			}
			continue
		}
		e := f2b.BanEntry{IP: m[1]}
		e.BannedAt, _ = time.ParseInLocation("2006-01-02 15:04:05", m[2], time.Local)
		if m[3] != "-1" { // -1 is a permanent ban: no expiry
			e.ExpiresAt, _ = time.ParseInLocation("2006-01-02 15:04:05", m[4], time.Local)
		}
		out = append(out, e)
	}
	return out
}

func (b *Backend) getInt(ctx context.Context, jail, key string) (int, error) {
	res, err := b.c.Call(ctx, "get", jail, key)
	if err != nil {
		return 0, err
	}
	n, err := proto.Int(res)
	if err != nil {
		return 0, fmt.Errorf("get %s %s: %w", jail, key, err)
	}
	return n, nil
}

func (b *Backend) getStrings(ctx context.Context, jail, key string) ([]string, error) {
	res, err := b.c.Call(ctx, "get", jail, key)
	if err != nil {
		return nil, err
	}
	s, err := proto.Strings(res)
	if err != nil {
		return nil, fmt.Errorf("get %s %s: %w", jail, key, err)
	}
	return s, nil
}

func (b *Backend) JailConfig(ctx context.Context, jail string) (f2b.JailConfig, error) {
	var cfg f2b.JailConfig
	if err := validate.Jail(jail); err != nil {
		return cfg, err
	}
	bt, err := b.getInt(ctx, jail, "bantime")
	if err != nil {
		return cfg, err
	}
	ft, err := b.getInt(ctx, jail, "findtime")
	if err != nil {
		return cfg, err
	}
	if cfg.MaxRetry, err = b.getInt(ctx, jail, "maxretry"); err != nil {
		return cfg, err
	}
	cfg.BanTime, cfg.FindTime = time.Duration(bt)*time.Second, time.Duration(ft)*time.Second
	for _, f := range []struct {
		key string
		dst *[]string
	}{
		{"logpath", &cfg.LogPaths}, {"failregex", &cfg.FailRegex}, {"ignoreregex", &cfg.IgnoreRegex},
		{"ignoreip", &cfg.IgnoreIPs}, {"actions", &cfg.Actions},
	} {
		if *f.dst, err = b.getStrings(ctx, jail, f.key); err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}

func (b *Backend) Ban(ctx context.Context, jail, ip string) error {
	if err := validate.Jail(jail); err != nil {
		return err
	}
	ip, err := validate.IP(ip)
	if err != nil {
		return err
	}
	_, err = b.c.Call(ctx, "set", jail, "banip", ip)
	return err
}

// Unban with an empty jail unbans the IP from every jail.
func (b *Backend) Unban(ctx context.Context, jail, ip string) error {
	ip, err := validate.IP(ip)
	if err != nil {
		return err
	}
	if jail == "" {
		_, err = b.c.Call(ctx, "unban", ip)
		return err
	}
	if err := validate.Jail(jail); err != nil {
		return err
	}
	_, err = b.c.Call(ctx, "set", jail, "unbanip", ip)
	return err
}

func (b *Backend) StopJail(ctx context.Context, jail string) error {
	if err := validate.Jail(jail); err != nil {
		return err
	}
	_, err := b.c.Call(ctx, "stop", jail)
	return err
}

func (b *Backend) AddIgnoreIP(ctx context.Context, jail, ipOrCIDR string) error {
	return b.ignoreIP(ctx, "addignoreip", jail, ipOrCIDR)
}

func (b *Backend) DelIgnoreIP(ctx context.Context, jail, ipOrCIDR string) error {
	return b.ignoreIP(ctx, "delignoreip", jail, ipOrCIDR)
}

func (b *Backend) ignoreIP(ctx context.Context, verb, jail, ipOrCIDR string) error {
	if err := validate.Jail(jail); err != nil {
		return err
	}
	v, err := validate.IPOrCIDR(ipOrCIDR)
	if err != nil {
		return err
	}
	_, err = b.c.Call(ctx, "set", jail, verb, v)
	return err
}

func (b *Backend) LogTarget(ctx context.Context) (string, error) {
	res, err := b.c.Call(ctx, "get", "logtarget")
	if err != nil {
		return "", err
	}
	s, ok := proto.Str(res)
	if !ok {
		return "", fmt.Errorf("unexpected logtarget reply %T", res)
	}
	return s, nil
}
