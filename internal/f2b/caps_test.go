package f2b

import (
	"errors"
	"fmt"
	"testing"
)

func TestCapsFor(t *testing.T) {
	cases := []struct {
		in   string
		want Caps
	}{
		{"1.0.2", Caps{Known: true, WithTime: true}},
		{"1.1.0", Caps{Known: true, WithTime: true}},
		{"0.11.2", Caps{Known: true, WithTime: true}},
		{"0.10.5", Caps{Known: true, WithTime: true}},
		{"0.9.7", Caps{Known: true}},
		{"v1.0", Caps{Known: true, WithTime: true}},
		{"weird", Caps{WithTime: true}},
		{"", Caps{WithTime: true}},
	}
	for _, c := range cases {
		if got := CapsFor(c.in); got != c.want {
			t.Errorf("CapsFor(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestCommandError(t *testing.T) {
	err := fmt.Errorf("status: %w", &CommandError{Msg: "boom"})
	var ce *CommandError
	if !errors.As(err, &ce) || ce.Error() != "boom" {
		t.Fatalf("errors.As failed: %v", err)
	}
}
