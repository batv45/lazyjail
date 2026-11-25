package proto

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/2404_" + name + ".pickle")
	if err != nil {
		t.Fatal(err)
	}
	f, err := ReadFrame(bytes.NewReader(b), MaxReply)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return f
}

func TestCommandRoundTrip(t *testing.T) {
	want := []string{"set", "sshd", "banip", "192.0.2.1"}
	b, err := EncodeCommand(want)
	if err != nil {
		t.Fatal(err)
	}
	f, err := ReadFrame(bytes.NewReader(b), MaxReply)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeCommand(f)
	if err != nil || strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestReplyRoundTrip(t *testing.T) {
	b, _ := EncodeReply([]interface{}{"a", "b"}, nil)
	f, _ := ReadFrame(bytes.NewReader(b), MaxReply)
	code, res, err := DecodeReply(f)
	if err != nil || code != 0 {
		t.Fatal(code, err)
	}
	if s, _ := Strings(res); strings.Join(s, ",") != "a,b" {
		t.Fatalf("got %v", res)
	}
	b, _ = EncodeReply(nil, errors.New("nope"))
	f, _ = ReadFrame(bytes.NewReader(b), MaxReply)
	code, res, err = DecodeReply(f)
	if err != nil || code != 1 || res != "nope" {
		t.Fatalf("got %d %v %v", code, res, err)
	}
}

// Real replies recorded from fail2ban 1.0.2 on ubuntu:24.04.
func TestDecodeRecordedReplies(t *testing.T) {
	code, res, err := DecodeReply(fixture(t, "ping"))
	if err != nil || code != 0 || res != "pong" {
		t.Fatal(code, res, err)
	}
	_, res, _ = DecodeReply(fixture(t, "version"))
	if s, _ := Str(res); s != "1.0.2" {
		t.Fatal(res)
	}
	_, res, _ = DecodeReply(fixture(t, "bantime"))
	if n, err := Int(res); err != nil || n != 600 {
		t.Fatal(res, err)
	}
	_, res, _ = DecodeReply(fixture(t, "status"))
	pairs, err := Pairs(res)
	if err != nil || len(pairs) != 2 || pairs[1].Key != "Jail list" || pairs[1].Value != "sshd" {
		t.Fatal(pairs, err)
	}
	_, res, _ = DecodeReply(fixture(t, "status_sshd"))
	pairs, err = Pairs(res)
	if err != nil || len(pairs) != 2 || pairs[0].Key != "Filter" {
		t.Fatal(pairs, err)
	}
	// str subclasses (IPAddr) arrive as builtins.str(...) calls.
	_, res, _ = DecodeReply(fixture(t, "ignoreip_after"))
	if s, err := Strings(res); err != nil || len(s) != 1 || s[0] != "192.0.2.0/24" {
		t.Fatal(s, err)
	}
	_, res, _ = DecodeReply(fixture(t, "failregex"))
	if s, err := Strings(res); err != nil || len(s) < 3 {
		t.Fatal(len(s), err)
	}
}

func TestDecodeRecordedExceptions(t *testing.T) {
	code, res, err := DecodeReply(fixture(t, "err_unknown_jail"))
	if err != nil || code != 1 || res != "UnknownJailException: nosuchjail" {
		t.Fatal(code, res, err)
	}
	code, res, err = DecodeReply(fixture(t, "err_unknown_cmd"))
	if err != nil || code != 1 || res != "Invalid command" {
		t.Fatal(code, res, err)
	}
}

func TestReadFrameLimitAndEOF(t *testing.T) {
	if _, err := ReadFrame(strings.NewReader(strings.Repeat("x", 100)), 10); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("got %v", err)
	}
	if _, err := ReadFrame(strings.NewReader("partial"), 100); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("got %v", err)
	}
	if _, err := ReadFrame(errReader{}, 100); err == nil || err == io.ErrUnexpectedEOF {
		t.Fatalf("got %v", err)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestDecodeErrors(t *testing.T) {
	if _, _, err := DecodeReply([]byte("garbage")); err == nil {
		t.Error("garbage should fail")
	}
	b, _ := EncodeCommand([]string{"x"}) // a list, not a tuple
	f, _ := ReadFrame(bytes.NewReader(b), MaxReply)
	if _, _, err := DecodeReply(f); err == nil {
		t.Error("non-tuple reply should fail")
	}
	if _, err := DecodeCommand([]byte("garbage")); err == nil {
		t.Error("garbage command should fail")
	}
}

func TestHelpers(t *testing.T) {
	if _, err := Strings(42); err == nil {
		t.Error("Strings(int)")
	}
	if _, err := Strings([]interface{}{1}); err == nil {
		t.Error("Strings([1])")
	}
	if s, err := Strings(nil); err != nil || s != nil {
		t.Error("Strings(nil)")
	}
	if _, err := Int("x"); err == nil {
		t.Error("Int(string)")
	}
	if n, _ := Int(int64(7)); n != 7 {
		t.Error("Int(int64)")
	}
	if _, err := Pairs("x"); err == nil {
		t.Error("Pairs(string)")
	}
	if _, err := Pairs([]interface{}{"x"}); err == nil {
		t.Error("Pairs([x])")
	}
	if got := ExceptionText(42); got != "42" {
		t.Error(got)
	}
}

// fail2ban 0.11.2 (ubuntu:22.04) answers in the same shape as 1.0.2.
func TestDecodeRecordedReplies0112(t *testing.T) {
	for _, name := range []string{"ping", "version", "status", "status_sshd", "banip_withtime_after", "logtarget", "ignoreip_after"} {
		b, err := os.ReadFile("testdata/2204_" + name + ".pickle")
		if err != nil {
			t.Fatal(err)
		}
		f, err := ReadFrame(bytes.NewReader(b), MaxReply)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if code, _, err := DecodeReply(f); err != nil || code != 0 {
			t.Fatalf("%s: code %d, %v", name, code, err)
		}
	}
	b, _ := os.ReadFile("testdata/2204_err_unknown_jail.pickle")
	f, _ := ReadFrame(bytes.NewReader(b), MaxReply)
	if code, res, err := DecodeReply(f); err != nil || code != 1 || res != "UnknownJailException: nosuchjail" {
		t.Fatal(code, res, err)
	}
}
