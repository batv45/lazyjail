// Package fake is an in-memory fail2ban server on a unix socket. It speaks the
// real wire protocol, so the socket Backend is tested end to end without fail2ban.
package fake

import (
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	ogórek "github.com/kisielk/og-rek"

	"github.com/batv45/lazyjail/internal/f2b/proto"
)

// Jail is the mutable state of one fake jail.
type Jail struct {
	CurrentlyFailed, TotalFailed, TotalBanned int
	FileList                                  []string
	Banned                                    map[string]time.Time
	BanTime, FindTime, MaxRetry               int
	FailRegex, IgnoreRegex, IgnoreIP, Actions []string
}

// invalidCommand is fail2ban's own (capitalised) reply to an unknown command.
const invalidCommand = replyError("Invalid command")

type replyError string

func (e replyError) Error() string { return string(e) }

// Server is a fake fail2ban. Create it with Start; fields are guarded by mu.
type Server struct {
	Path string

	mu         sync.Mutex
	version    string
	logTarget  string
	jails      map[string]*Jail
	noWithTime bool
	failNext   error
	delay      time.Duration
	hang       bool
	rawReply   []byte
	ln         net.Listener
	done       chan struct{}
	closeOnce  sync.Once
	wg         sync.WaitGroup
	commands   [][]string
}

// Start listens on path and serves until Close.
func Start(path string) (*Server, error) {
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	s := &Server{
		Path:      path,
		version:   "1.0.2",
		logTarget: "/var/log/fail2ban.log",
		ln:        ln,
		done:      make(chan struct{}),
		jails: map[string]*Jail{
			"sshd": newJail("/var/log/auth.log"),
			"nginx-http-auth": func() *Jail {
				j := newJail("/var/log/nginx/error.log")
				j.BanTime = 3600
				return j
			}(),
		},
	}
	s.wg.Add(1)
	go s.serve()
	return s, nil
}

func newJail(logpath string) *Jail {
	return &Jail{
		FileList: []string{logpath}, Banned: map[string]time.Time{},
		BanTime: 600, FindTime: 600, MaxRetry: 5,
		FailRegex: []string{`^Failed password for <HOST>$`}, Actions: []string{"iptables-multiport"},
	}
}

// Close stops the server and removes the socket.
func (s *Server) Close() {
	s.closeOnce.Do(func() {
		close(s.done)
		_ = s.ln.Close()
		s.wg.Wait()
		_ = os.Remove(s.Path)
	})
}

// SetVersion changes the version reply.
func (s *Server) SetVersion(v string) { s.lock(func() { s.version = v }) }

// SetNoWithTime makes `banip --with-time` fail like an old server.
func (s *Server) SetNoWithTime(b bool) { s.lock(func() { s.noWithTime = b }) }

// FailNext makes the next command answer with an exception.
func (s *Server) FailNext(err error) { s.lock(func() { s.failNext = err }) }

// SetDelay delays every reply.
func (s *Server) SetDelay(d time.Duration) { s.lock(func() { s.delay = d }) }

// SetHang makes the server read the command and never answer.
func (s *Server) SetHang(b bool) { s.lock(func() { s.hang = b }) }

// SetRawReply replaces the next reply with raw bytes (protocol fault injection).
func (s *Server) SetRawReply(b []byte) { s.lock(func() { s.rawReply = b }) }

// SetJail adds or replaces a jail.
func (s *Server) SetJail(name string, j *Jail) { s.lock(func() { s.jails[name] = j }) }

// DeleteJail removes a jail.
func (s *Server) DeleteJail(name string) { s.lock(func() { delete(s.jails, name) }) }

// Banned returns the banned IPs of a jail, sorted.
func (s *Server) Banned(jail string) []string {
	var out []string
	s.lock(func() {
		if j := s.jails[jail]; j != nil {
			out = sortedKeys(j.Banned)
		}
	})
	return out
}

// Commands returns every command received so far.
func (s *Server) Commands() [][]string {
	var out [][]string
	s.lock(func() { out = append(out, s.commands...) })
	return out
}

func (s *Server) lock(f func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f()
}

func (s *Server) serve() {
	defer s.wg.Done()
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer func() { _ = c.Close() }()
			s.handle(c)
		}()
	}
}

func (s *Server) handle(c net.Conn) {
	frame, err := proto.ReadFrame(c, proto.MaxReply)
	if err != nil {
		return
	}
	cmd, err := proto.DecodeCommand(frame)
	if err != nil {
		return
	}
	s.mu.Lock()
	s.commands = append(s.commands, cmd)
	hang, delay, raw, fail := s.hang, s.delay, s.rawReply, s.failNext
	s.rawReply, s.failNext = nil, nil
	s.mu.Unlock()
	if hang {
		select {
		case <-s.done:
		case <-time.After(time.Minute):
		}
		return
	}
	time.Sleep(delay)
	if raw != nil {
		_, _ = c.Write(raw)
		return
	}
	var res interface{}
	if fail == nil {
		s.mu.Lock()
		res, fail = s.exec(cmd)
		s.mu.Unlock()
	}
	out, err := proto.EncodeReply(res, fail)
	if err != nil {
		return
	}
	_, _ = c.Write(out)
}

func sortedKeys(m map[string]time.Time) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func strs(in []string) []interface{} {
	out := make([]interface{}, len(in))
	for i, s := range in {
		out[i] = s
	}
	return out
}

// strCall mimics fail2ban's IPAddr, a str subclass that pickles as builtins.str(...).
func strCall(s string) interface{} {
	return ogórek.Call{Callable: ogórek.Class{Module: "builtins", Name: "str"}, Args: ogórek.Tuple{s}}
}

func (s *Server) jail(name string) (*Jail, error) {
	j := s.jails[name]
	if j == nil {
		return nil, fmt.Errorf("UnknownJailException: %s", name)
	}
	return j, nil
}

// exec answers one command; the caller holds s.mu.
func (s *Server) exec(cmd []string) (interface{}, error) {
	if len(cmd) == 0 {
		return nil, invalidCommand
	}
	switch {
	case len(cmd) == 1 && cmd[0] == "ping":
		return "pong", nil
	case len(cmd) == 1 && cmd[0] == "version":
		return s.version, nil
	case len(cmd) == 1 && cmd[0] == "status":
		names := make([]string, 0, len(s.jails))
		for n := range s.jails {
			names = append(names, n)
		}
		sort.Strings(names)
		return []interface{}{
			ogórek.Tuple{"Number of jail", len(names)},
			ogórek.Tuple{"Jail list", strings.Join(names, ", ")},
		}, nil
	case len(cmd) >= 2 && cmd[0] == "status":
		return s.status(cmd[1])
	case len(cmd) == 2 && cmd[0] == "get" && cmd[1] == "logtarget":
		return s.logTarget, nil
	case len(cmd) == 2 && cmd[0] == "stop":
		if _, err := s.jail(cmd[1]); err != nil {
			return nil, err
		}
		delete(s.jails, cmd[1])
		return "Jail stopped", nil
	case len(cmd) == 2 && cmd[0] == "unban":
		return s.unbanAll(cmd[1]), nil
	case len(cmd) >= 3 && cmd[0] == "get":
		return s.get(cmd)
	case len(cmd) == 4 && cmd[0] == "set":
		return s.set(cmd)
	}
	return nil, invalidCommand
}

func (s *Server) status(name string) (interface{}, error) {
	j, err := s.jail(name)
	if err != nil {
		return nil, err
	}
	return []interface{}{
		ogórek.Tuple{"Filter", []interface{}{
			ogórek.Tuple{"Currently failed", j.CurrentlyFailed},
			ogórek.Tuple{"Total failed", j.TotalFailed},
			ogórek.Tuple{"File list", strs(j.FileList)},
		}},
		ogórek.Tuple{"Actions", []interface{}{
			ogórek.Tuple{"Currently banned", len(j.Banned)},
			ogórek.Tuple{"Total banned", j.TotalBanned},
			ogórek.Tuple{"Banned IP list", strs(sortedKeys(j.Banned))},
		}},
	}, nil
}

func (s *Server) get(cmd []string) (interface{}, error) {
	j, err := s.jail(cmd[1])
	if err != nil {
		return nil, err
	}
	switch cmd[2] {
	case "banip":
		if len(cmd) == 4 && cmd[3] == "--with-time" {
			if s.noWithTime {
				return nil, invalidCommand
			}
			var out []string
			for _, ip := range sortedKeys(j.Banned) {
				at := j.Banned[ip]
				out = append(out, fmt.Sprintf("%s \t%s + %d = %s", ip, at.Format("2006-01-02 15:04:05"),
					j.BanTime, at.Add(time.Duration(j.BanTime)*time.Second).Format("2006-01-02 15:04:05")))
			}
			return strs(out), nil
		}
		return strs(sortedKeys(j.Banned)), nil
	case "bantime":
		return j.BanTime, nil
	case "findtime":
		return j.FindTime, nil
	case "maxretry":
		return j.MaxRetry, nil
	case "logpath":
		return strs(j.FileList), nil
	case "failregex":
		return strs(j.FailRegex), nil
	case "ignoreregex":
		return strs(j.IgnoreRegex), nil
	case "ignoreip":
		out := make([]interface{}, len(j.IgnoreIP))
		for i, ip := range j.IgnoreIP {
			out[i] = strCall(ip)
		}
		return out, nil
	case "actions":
		return strs(j.Actions), nil
	}
	return nil, invalidCommand
}

func (s *Server) set(cmd []string) (interface{}, error) {
	j, err := s.jail(cmd[1])
	if err != nil {
		return nil, err
	}
	arg := cmd[3]
	switch cmd[2] {
	case "banip":
		if _, dup := j.Banned[arg]; !dup {
			j.Banned[arg] = time.Now()
			j.TotalBanned++
		}
		return 1, nil
	case "unbanip":
		if _, ok := j.Banned[arg]; !ok {
			return nil, fmt.Errorf("%s is not banned", arg)
		}
		delete(j.Banned, arg)
		return arg, nil
	case "addignoreip":
		for _, ip := range j.IgnoreIP {
			if ip == arg {
				return strs(j.IgnoreIP), nil
			}
		}
		j.IgnoreIP = append(j.IgnoreIP, arg)
		return strs(j.IgnoreIP), nil
	case "delignoreip":
		for i, ip := range j.IgnoreIP {
			if ip == arg {
				j.IgnoreIP = append(j.IgnoreIP[:i], j.IgnoreIP[i+1:]...)
				return strs(j.IgnoreIP), nil
			}
		}
		return nil, fmt.Errorf("%s is not in ignore list", arg)
	}
	return nil, invalidCommand
}

func (s *Server) unbanAll(ip string) interface{} {
	n := 0
	for _, j := range s.jails {
		if _, ok := j.Banned[ip]; ok {
			delete(j.Banned, ip)
			n++
		}
	}
	return strconv.Itoa(n)
}
