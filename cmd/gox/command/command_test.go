package command

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
)

type stubStarter struct {
	defaultCalled bool
	formatArgs    GenericArgs
	generateArgs  GenericArgs
	serveArgs     ServeArgs
	defaultErr    error
	formatErr     error
	generateErr   error
	serveErr      error
}

func (s *stubStarter) Default() error {
	s.defaultCalled = true
	return s.defaultErr
}

func (s *stubStarter) Serve(args ServeArgs) error {
	s.serveArgs = args
	return s.serveErr
}

func (s *stubStarter) Format(args GenericArgs) error {
	s.formatArgs = args
	return s.formatErr
}

func (s *stubStarter) Generate(args GenericArgs) error {
	s.generateArgs = args
	return s.generateErr
}

func TestExecuteDispatch(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })

	t.Run("default", func(t *testing.T) {
		os.Args = []string{"gox"}
		starter := &stubStarter{}
		cmdErr, runErr := Execute(starter)
		if cmdErr != nil || runErr != nil {
			t.Fatalf("Execute() = (%v, %v)", cmdErr, runErr)
		}
		if !starter.defaultCalled {
			t.Fatal("Default() was not called")
		}
	})

	t.Run("format", func(t *testing.T) {
		os.Args = []string{"gox", "fmt", "-no-ignore", "./internal"}
		starter := &stubStarter{}
		cmdErr, runErr := Execute(starter)
		if cmdErr != nil || runErr != nil {
			t.Fatalf("Execute() = (%v, %v)", cmdErr, runErr)
		}
		if !slices.Equal(starter.formatArgs.Paths(), []string{"./internal"}) || !starter.formatArgs.NoIgnore() {
			t.Fatalf("Format args = %#v", starter.formatArgs)
		}
	})

	t.Run("generate", func(t *testing.T) {
		os.Args = []string{"gox", "gen", "-force", "./views", "./cmd/..."}
		starter := &stubStarter{}
		cmdErr, runErr := Execute(starter)
		if cmdErr != nil || runErr != nil {
			t.Fatalf("Execute() = (%v, %v)", cmdErr, runErr)
		}
		if !slices.Equal(starter.generateArgs.Paths(), []string{"./views", "./cmd/..."}) || !starter.generateArgs.Force() {
			t.Fatalf("Generate args = %#v", starter.generateArgs)
		}
	})

	t.Run("serve", func(t *testing.T) {
		os.Args = []string{"gox", "srv", "-listen", "unix;/tmp/gox.sock", "-gopls", "/tmp/gopls"}
		starter := &stubStarter{}
		cmdErr, runErr := Execute(starter)
		if cmdErr != nil || runErr != nil {
			t.Fatalf("Execute() = (%v, %v)", cmdErr, runErr)
		}
		if got := starter.serveArgs.Gopls(); got != "/tmp/gopls" {
			t.Fatalf("ServeArgs.Gopls() = %q", got)
		}
		network, address, ok := starter.serveArgs.Socket()
		if !ok || network != "unix" || address != "/tmp/gox.sock" {
			t.Fatalf("ServeArgs.Socket() = (%q, %q, %v)", network, address, ok)
		}
	})

	t.Run("unknown", func(t *testing.T) {
		os.Args = []string{"gox", "wat"}
		starter := &stubStarter{}
		cmdErr, runErr := Execute(starter)
		if runErr != nil {
			t.Fatalf("runErr = %v, want nil", runErr)
		}
		want := "Unknown command: wat\n\n" + help
		if cmdErr == nil || cmdErr.Error() != want {
			t.Fatalf("cmdErr = %v, want %q", cmdErr, want)
		}
	})

	t.Run("accepts multiple paths", func(t *testing.T) {
		os.Args = []string{"gox", "fmt", "./one", "./two"}
		starter := &stubStarter{}
		cmdErr, runErr := Execute(starter)
		if cmdErr != nil || runErr != nil {
			t.Fatalf("Execute() = (%v, %v)", cmdErr, runErr)
		}
		if !slices.Equal(starter.formatArgs.Paths(), []string{"./one", "./two"}) {
			t.Fatalf("Format paths = %v", starter.formatArgs.Paths())
		}
	})

	t.Run("flag parse error", func(t *testing.T) {
		os.Args = []string{"gox", "fmt", "-bogus"}
		starter := &stubStarter{}
		cmdErr, runErr := Execute(starter)
		if runErr != nil {
			t.Fatalf("runErr = %v, want nil", runErr)
		}
		if cmdErr == nil || !strings.HasPrefix(cmdErr.Error(), "Could not parse format arguments:") {
			t.Fatalf("cmdErr = %v, want a parse error", cmdErr)
		}
	})

	t.Run("run error", func(t *testing.T) {
		os.Args = []string{"gox", "gen"}
		starter := &stubStarter{generateErr: errors.New("boom")}
		cmdErr, runErr := Execute(starter)
		if cmdErr != nil || runErr == nil || runErr.Error() != "boom" {
			t.Fatalf("Execute() = (%v, %v)", cmdErr, runErr)
		}
	})
}
