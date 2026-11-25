package f2b

import (
	"regexp"
	"strconv"
)

// Caps lists what the connected fail2ban version can do.
type Caps struct {
	// WithTime is true when `get <jail> banip --with-time` exists.
	WithTime bool
	// Known is false when the version string could not be parsed; features are then assumed present.
	Known bool
}

var versionRe = regexp.MustCompile(`^v?(\d+)\.(\d+)`)

// CapsFor derives Caps from a `version` reply such as "1.0.2" or "0.11.2".
// An unparseable version yields Known=false with every feature enabled, so
// lazyjail warns instead of refusing to run.
func CapsFor(version string) Caps {
	m := versionRe.FindStringSubmatch(version)
	if m == nil {
		return Caps{WithTime: true}
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	// --with-time arrived in the 0.10 series; 0.9 and older only list bare IPs.
	return Caps{Known: true, WithTime: major > 0 || minor >= 10}
}
