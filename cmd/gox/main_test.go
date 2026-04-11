package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/doors-dev/gox/cmd/gox/command"
)

const starterSample = `package demo

import "github.com/doors-dev/gox"

elem View(name string) {
	<div>~(name)</div>
}
`

func TestInitLoggerWritesToFile(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "gox.log")
	initLogger(logFile, slog.LevelInfo, true)
	slog.Info("hello from test")

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("ReadFile(log): %v", err)
	}
	if !strings.Contains(string(content), "hello from test") {
		t.Fatalf("log output missing message:\n%s", content)
	}
	beforeDisable := string(content)

	initLogger(logFile, slog.LevelInfo, false)
	slog.Info("discarded")
	content, err = os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("ReadFile(log after disable): %v", err)
	}
	if got := string(content); got != beforeDisable {
		t.Fatalf("disabled logger changed file:\nbefore:\n%s\nafter:\n%s", beforeDisable, got)
	}
}

func TestStarterFormatAndGenerateViaExecute(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })

	dir := t.TempDir()
	goFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(goFile, []byte("package main\nfunc main(){println(\"x\")}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Args = []string{"gox", "fmt", goFile}
	cmdErr, runErr := command.Execute(starter{})
	if cmdErr != nil || runErr != nil {
		t.Fatalf("format Execute() = (%v, %v)", cmdErr, runErr)
	}
	formatted, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatal(err)
	}
	want := "package main\n\nfunc main() { println(\"x\") }\n"
	if got := string(formatted); got != want {
		t.Fatalf("formatted file = %q, want %q", got, want)
	}

	goxFile := filepath.Join(dir, "view.gox")
	if err := os.WriteFile(goxFile, []byte(starterSample), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Args = []string{"gox", "gen", goxFile}
	cmdErr, runErr = command.Execute(starter{})
	if cmdErr != nil || runErr != nil {
		t.Fatalf("generate Execute() = (%v, %v)", cmdErr, runErr)
	}
	if _, err := os.Stat(filepath.Join(dir, "view.x.go")); err != nil {
		t.Fatalf("generated target missing: %v", err)
	}
}

func TestStarterServeReturnsExecError(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })

	os.Args = []string{
		"gox",
		"srv",
		"-listen", "127.0.0.1:0",
		"-gopls", "/definitely/missing/gopls",
	}
	cmdErr, runErr := command.Execute(starter{})
	if cmdErr != nil {
		t.Fatalf("cmdErr = %v, want nil", cmdErr)
	}
	if runErr == nil {
		t.Fatal("runErr = nil, want exec error")
	}
}

func TestMainRunsSuccessfulCommand(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })

	dir := t.TempDir()
	goFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(goFile, []byte("package main\nfunc main(){println(\"x\")}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Args = []string{"gox", "fmt", goFile}
	main()

	formatted, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatal(err)
	}
	want := "package main\n\nfunc main() { println(\"x\") }\n"
	if got := string(formatted); got != want {
		t.Fatalf("main() formatted file = %q, want %q", got, want)
	}
}

func TestMainExitsWithCommandError(t *testing.T) {
	result := runMainProcess(t, []string{"wat"}, nil)
	if result.exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", result.exitCode)
	}
	want := "Unknown command: wat\n\nCommands:\n  srv\t\tStarts the GoX Language Server (default)\n  gen\t\tGenerates .x.go files from .gox files and removes orphaned .x.go files\n  fmt\t\tFormats .go and .gox files\n  ver\t\tPrints the version\n"
	if result.stderr != want {
		t.Fatalf("stderr = %q, want %q", result.stderr, want)
	}
}

func TestMainExitsWithRunError(t *testing.T) {
	result := runMainProcess(t, []string{
		"srv",
		"-listen", "127.0.0.1:0",
		"-gopls", "/definitely/missing/gopls",
	}, nil)
	if result.exitCode != 1 {
		t.Fatalf("exit code = %d, want 1 (stderr=%q)", result.exitCode, result.stderr)
	}
	if !strings.Contains(result.stderr, "/definitely/missing/gopls") {
		t.Fatalf("stderr = %q, want missing gopls path", result.stderr)
	}
}

func TestMainDefaultPathReturnsWithoutGopls(t *testing.T) {
	result := runMainProcess(t, nil, map[string]string{
		"PATH": t.TempDir(),
	})
	if result.exitCode != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr=%q)", result.exitCode, result.stderr)
	}
}

func TestMainProcessHelper(t *testing.T) {
	if os.Getenv("GOX_MAIN_HELPER") != "1" {
		return
	}
	var args []string
	if err := json.Unmarshal([]byte(os.Getenv("GOX_MAIN_ARGS")), &args); err != nil {
		panic(err)
	}
	os.Args = append([]string{"gox"}, args...)
	main()
	os.Exit(0)
}

type mainProcessResult struct {
	stdout   string
	stderr   string
	exitCode int
}

func runMainProcess(t *testing.T, args []string, extraEnv map[string]string) mainProcessResult {
	t.Helper()

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("Executable() error = %v", err)
	}
	rawArgs, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("Marshal(args) error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, exe, "-test.run=TestMainProcessHelper")
	cmd.Env = append(os.Environ(), "GOX_MAIN_HELPER=1", "GOX_MAIN_ARGS="+string(rawArgs))
	for key, value := range extraEnv {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	var stdout strings.Builder
	var stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("main helper timed out\nstdout:\n%s\nstderr:\n%s", stdout.String(), stderr.String())
	}

	result := mainProcessResult{
		stdout: stdout.String(),
		stderr: stderr.String(),
	}
	if err == nil {
		return result
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Run() error = %v\nstdout:\n%s\nstderr:\n%s", err, result.stdout, result.stderr)
	}
	result.exitCode = exitErr.ExitCode()
	return result
}
