// Command lazyjail is a keyboard-driven terminal UI for fail2ban.
//
// This skeleton only implements --version; the TUI lands in later MVP issues.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

// version is overridden at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run parses args and returns the process exit code.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lazyjail", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showVersion := fs.Bool("version", false, "print version and exit")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		_, _ = fmt.Fprintln(stdout, "lazyjail", version)
		return 0
	}
	_, _ = fmt.Fprintln(stderr, "lazyjail: TUI henüz hazır değil; --version kullanın")
	return 2
}
