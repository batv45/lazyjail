// Package proto speaks the fail2ban unix socket wire format: a pickled command
// list followed by "<F2B_END_COMMAND>", answered by a pickled (code, result)
// tuple with the same terminator. Only string lists are ever pickled by us;
// replies are decoded by og-rek, which never executes code.
package proto

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	ogórek "github.com/kisielk/og-rek"
)

// EndMarker terminates every command and reply.
const EndMarker = "<F2B_END_COMMAND>"

// MaxReply is the largest reply (in bytes) a client accepts.
const MaxReply = 16 << 20

// ErrTooLarge is returned when a frame exceeds the limit.
var ErrTooLarge = errors.New("fail2ban reply exceeds size limit")

// EncodeCommand pickles cmd as a list of strings and appends the end marker.
func EncodeCommand(cmd []string) ([]byte, error) {
	items := make([]interface{}, len(cmd))
	for i, s := range cmd {
		items[i] = s
	}
	return encode(items)
}

// EncodeReply builds a server reply; used by the fake server. An error result
// is sent as a Python exception object, as fail2ban does.
func EncodeReply(result interface{}, failure error) ([]byte, error) {
	if failure != nil {
		exc := ogórek.Call{
			Callable: ogórek.Class{Module: "builtins", Name: "Exception"},
			Args:     ogórek.Tuple{failure.Error()},
		}
		return encode(ogórek.Tuple{1, exc})
	}
	return encode(ogórek.Tuple{0, result})
}

func encode(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := ogórek.NewEncoderWithConfig(&buf, &ogórek.EncoderConfig{Protocol: 2})
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("pickle encode: %w", err)
	}
	buf.WriteString(EndMarker)
	return buf.Bytes(), nil
}

// ReadFrame reads from r until the end marker and returns the payload without it.
func ReadFrame(r io.Reader, limit int) ([]byte, error) {
	var buf []byte
	chunk := make([]byte, 32*1024)
	for {
		n, err := r.Read(chunk)
		buf = append(buf, chunk[:n]...)
		if bytes.HasSuffix(buf, []byte(EndMarker)) {
			return buf[:len(buf)-len(EndMarker)], nil
		}
		if len(buf) > limit {
			return nil, ErrTooLarge
		}
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return nil, err
		}
	}
}

func decode(payload []byte) (interface{}, error) {
	v, err := ogórek.NewDecoder(bytes.NewReader(payload)).Decode()
	if err != nil {
		return nil, fmt.Errorf("pickle decode: %w", err)
	}
	return v, nil
}

// DecodeCommand unpickles a command list; used by the fake server.
func DecodeCommand(payload []byte) ([]string, error) {
	v, err := decode(payload)
	if err != nil {
		return nil, err
	}
	return Strings(v)
}

// DecodeReply unpickles a (code, result) tuple. A non-zero code is returned as
// code with the exception rendered by ExceptionText in result.
func DecodeReply(payload []byte) (int, interface{}, error) {
	v, err := decode(payload)
	if err != nil {
		return 0, nil, err
	}
	t, ok := v.(ogórek.Tuple)
	if !ok || len(t) != 2 {
		return 0, nil, fmt.Errorf("unexpected reply shape %T", v)
	}
	code, err := Int(t[0])
	if err != nil {
		return 0, nil, fmt.Errorf("reply code: %w", err)
	}
	if code != 0 {
		return code, ExceptionText(t[1]), nil
	}
	return 0, t[1], nil
}

// ExceptionText renders a pickled Python exception (or any value) as text.
// builtins exceptions give their message, others "Name: message".
func ExceptionText(v interface{}) string {
	c, ok := v.(ogórek.Call)
	if !ok {
		return fmt.Sprint(v)
	}
	var parts []string
	for _, a := range c.Args {
		parts = append(parts, fmt.Sprint(a))
	}
	msg := strings.Join(parts, ", ")
	cls := c.Callable
	if cls.Module == "builtins" || cls.Name == "" {
		return msg
	}
	if msg == "" {
		return cls.Name
	}
	return cls.Name + ": " + msg
}

// Str returns v as a string. Python str subclasses (fail2ban's IPAddr) arrive
// as builtins.str(...) calls and are unwrapped.
func Str(v interface{}) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case ogórek.Call:
		if len(x.Args) == 1 {
			return Str(x.Args[0])
		}
	}
	return "", false
}

// Strings converts a pickled list or tuple of strings.
func Strings(v interface{}) ([]string, error) {
	var items []interface{}
	switch x := v.(type) {
	case []interface{}:
		items = x
	case ogórek.Tuple:
		items = x
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("expected list, got %T", v)
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		s, ok := Str(it)
		if !ok {
			return nil, fmt.Errorf("expected string item, got %T", it)
		}
		out = append(out, s)
	}
	return out, nil
}

// Int converts a pickled integer.
func Int(v interface{}) (int, error) {
	switch x := v.(type) {
	case int:
		return x, nil
	case int64:
		return int(x), nil
	}
	return 0, fmt.Errorf("expected integer, got %T", v)
}

// Pair is one (name, value) entry of a status reply.
type Pair struct {
	Key   string
	Value interface{}
}

// Pairs converts a list of (name, value) tuples as found in `status` replies.
func Pairs(v interface{}) ([]Pair, error) {
	items, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected pair list, got %T", v)
	}
	out := make([]Pair, 0, len(items))
	for _, it := range items {
		t, ok := it.(ogórek.Tuple)
		if !ok || len(t) != 2 {
			return nil, fmt.Errorf("expected (name, value) tuple, got %T", it)
		}
		k, ok := Str(t[0])
		if !ok {
			return nil, fmt.Errorf("pair key is %T", t[0])
		}
		out = append(out, Pair{k, t[1]})
	}
	return out, nil
}
