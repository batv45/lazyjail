package validate

import "testing"

func TestIP(t *testing.T) {
	ok := map[string]string{
		"192.0.2.1": "192.0.2.1", " 10.0.0.1 ": "10.0.0.1", "2001:db8::1": "2001:db8::1",
		"2001:DB8:0:0:0:0:0:1": "2001:db8::1",
	}
	for in, want := range ok {
		got, err := IP(in)
		if err != nil || got != want {
			t.Errorf("IP(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "1.2.3", "1.2.3.4/24", "-h", "1.2.3.4; rm", "example.com", "1.2.3.256"} {
		if _, err := IP(in); err == nil {
			t.Errorf("IP(%q) should fail", in)
		}
	}
}

func TestIPOrCIDR(t *testing.T) {
	ok := map[string]string{"192.0.2.0/24": "192.0.2.0/24", "192.0.2.5": "192.0.2.5", "2001:db8::/32": "2001:db8::/32"}
	for in, want := range ok {
		got, err := IPOrCIDR(in)
		if err != nil || got != want {
			t.Errorf("IPOrCIDR(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "1.2.3.4/33", "1.2.3.4/", "x/24", "-1/8"} {
		if _, err := IPOrCIDR(in); err == nil {
			t.Errorf("IPOrCIDR(%q) should fail", in)
		}
	}
}

func TestJail(t *testing.T) {
	for _, in := range []string{"sshd", "nginx-http-auth", "a.b_c", "J1"} {
		if err := Jail(in); err != nil {
			t.Errorf("Jail(%q): %v", in, err)
		}
	}
	for _, in := range []string{"", "-x", "--all", "a b", "a;b", "a/b", "ä"} {
		if err := Jail(in); err == nil {
			t.Errorf("Jail(%q) should fail", in)
		}
	}
}
