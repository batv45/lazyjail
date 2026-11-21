// Package f2b defines the backend contract between the lazyjail UI and fail2ban.
//
// The socket implementation (fail2ban unix socket, pickle protocol), the exec
// implementation (fail2ban-client / fail2ban-regex) and the test doubles all
// satisfy Backend. See docs/PLAN.md §3.2 for which operation goes through which path.
package f2b

import (
	"context"
	"time"
)

// JailStatus is the runtime state of one jail as reported by `status <jail>`.
type JailStatus struct {
	Name            string
	CurrentlyFailed int
	TotalFailed     int
	CurrentlyBanned int
	TotalBanned     int
	FileList        []string
	BannedIPs       []string
}

// BanEntry is a banned IP. BannedAt and ExpiresAt are zero when the server
// does not provide timestamps (`--with-time` unsupported).
type BanEntry struct {
	IP        string
	BannedAt  time.Time
	ExpiresAt time.Time
}

// JailConfig holds the effective configuration of one jail.
type JailConfig struct {
	BanTime     time.Duration
	FindTime    time.Duration
	MaxRetry    int
	LogPaths    []string
	FailRegex   []string
	IgnoreRegex []string
	IgnoreIPs   []string
	Actions     []string
}

// RegexResult is the outcome of testing a filter against log lines.
type RegexResult struct {
	Lines        int
	Matched      int
	Missed       int
	MatchedLines []string
	MissedLines  []string
}

// Backend is everything the UI needs from fail2ban. Every call takes a context;
// implementations apply a default timeout (5 s) when the caller sets none.
type Backend interface {
	Ping(ctx context.Context) error
	Version(ctx context.Context) (string, error)
	Jails(ctx context.Context) ([]string, error)
	JailStatus(ctx context.Context, jail string) (JailStatus, error)
	BannedWithTime(ctx context.Context, jail string) ([]BanEntry, error)
	JailConfig(ctx context.Context, jail string) (JailConfig, error)
	Ban(ctx context.Context, jail, ip string) error
	// Unban with an empty jail unbans the IP from every jail.
	Unban(ctx context.Context, jail, ip string) error
	// StartJail and Reload go through fail2ban-client, because they read config files.
	StartJail(ctx context.Context, jail string) error
	StopJail(ctx context.Context, jail string) error
	// Reload with an empty jail reloads all jails.
	Reload(ctx context.Context, jail string) error
	AddIgnoreIP(ctx context.Context, jail, ipOrCIDR string) error
	DelIgnoreIP(ctx context.Context, jail, ipOrCIDR string) error
	LogTarget(ctx context.Context) (string, error)
	// TestRegex runs fail2ban-regex with a log file or a single line against a filter name or regex.
	TestRegex(ctx context.Context, logOrLine, filterOrRegex string) (RegexResult, error)
}
