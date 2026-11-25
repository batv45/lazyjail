// Package validate checks every user- or server-supplied value before it
// reaches fail2ban (socket command or exec argument).
package validate

import (
	"fmt"
	"net/netip"
	"regexp"
	"strings"
)

var jailRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// IP accepts a single IPv4/IPv6 address and returns its canonical text form.
func IP(s string) (string, error) {
	a, err := netip.ParseAddr(strings.TrimSpace(s))
	if err != nil {
		return "", fmt.Errorf("invalid IP address %q", s)
	}
	return a.String(), nil
}

// IPOrCIDR accepts an address or a CIDR network and returns the canonical form.
func IPOrCIDR(s string) (string, error) {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "/") {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			return "", fmt.Errorf("invalid CIDR %q", s)
		}
		return p.String(), nil
	}
	return IP(s)
}

// Jail accepts [A-Za-z0-9._-]+ and rejects a leading '-' so a name can never
// be read as an option by fail2ban-client.
func Jail(s string) error {
	if !jailRe.MatchString(s) || strings.HasPrefix(s, "-") {
		return fmt.Errorf("invalid jail name %q", s)
	}
	return nil
}
