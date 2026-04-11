package command

import (
	"log/slog"
	"testing"
	"time"
)

func TestGetLogLevel(t *testing.T) {
	cases := []struct {
		input string
		want  slog.Level
	}{
		{input: "debug", want: slog.LevelDebug},
		{input: "warn", want: slog.LevelWarn},
		{input: "error", want: slog.LevelError},
		{input: "INFO", want: slog.LevelInfo},
	}
	for _, tc := range cases {
		if got := getLogLevel(tc.input); got != tc.want {
			t.Fatalf("getLogLevel(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestParseServeArgsAndHelpers(t *testing.T) {
	args, err := parseServeArgs([]string{
		"-listen", "unix;/tmp/gox.sock",
		"-listen.timeout", "5s",
		"-log", "/tmp/gox.log",
		"-log.level", "debug",
		"-gopls", "/custom/gopls",
	})
	if err != nil {
		t.Fatalf("parseServeArgs() error = %v", err)
	}

	level, file, enabled := args.LoggerInfo()
	if !enabled || level != slog.LevelDebug || file != "/tmp/gox.log" {
		t.Fatalf("LoggerInfo() = (%v, %q, %v)", level, file, enabled)
	}
	if got := args.Gopls(); got != "/custom/gopls" {
		t.Fatalf("Gopls() = %q, want /custom/gopls", got)
	}
	if got := args.Timeout(); got != 5*time.Second {
		t.Fatalf("Timeout() = %v, want 5s", got)
	}
	network, address, ok := args.Socket()
	if !ok || network != "unix" || address != "/tmp/gox.sock" {
		t.Fatalf("Socket() = (%q, %q, %v)", network, address, ok)
	}
}

func TestServeArgsDefaultsAndListenParsing(t *testing.T) {
	args, err := parseServeArgs(nil)
	if err != nil {
		t.Fatalf("parseServeArgs(nil) error = %v", err)
	}
	if _, _, enabled := args.LoggerInfo(); enabled {
		t.Fatal("LoggerInfo() enabled = true, want false")
	}
	if args.Gopls() != "gopls" {
		t.Fatalf("Gopls() = %q, want gopls", args.Gopls())
	}
	if args.Timeout() != 0 {
		t.Fatalf("Timeout() = %v, want 0", args.Timeout())
	}
	if _, _, ok := args.Socket(); ok {
		t.Fatal("Socket() ok = true, want false")
	}

	network, address := parseListenArg("127.0.0.1:3737")
	if network != "tcp" || address != "127.0.0.1:3737" {
		t.Fatalf("parseListenArg(tcp) = (%q, %q)", network, address)
	}
	network, address = parseListenArg("unix;/tmp/demo.sock")
	if network != "unix" || address != "/tmp/demo.sock" {
		t.Fatalf("parseListenArg(unix) = (%q, %q)", network, address)
	}
}
