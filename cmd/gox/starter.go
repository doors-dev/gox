package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/doors-dev/gox/cmd/gox/command"
	"github.com/doors-dev/gox/cmd/gox/internal/dialers"
	"github.com/doors-dev/gox/cmd/gox/internal/listeners"
	"github.com/doors-dev/gox/cmd/gox/internal/processor"
	"github.com/doors-dev/gox/cmd/gox/internal/server"
)

type starter struct{}

func (s starter) Format(args command.GenericArgs) error {
	return processor.Format(args.Path(), args.NoIgnore())
}

func (s starter) Generate(args command.GenericArgs) error {
	return processor.Generate(args.Path(), args.NoIgnore())
}

func (s starter) Default() error {
	return s.Serve(command.ServeArgs{})
}

func (s starter) Serve(args command.ServeArgs) error {
	var listener server.Listener
	var dialer server.Dialer
	network, address, ok := args.Socket()
	if ok {
		sockPath := filepath.Join(os.TempDir(), fmt.Sprintf("gox_%d.sock", os.Getpid()))
		cmd := exec.Command(args.Gopls(), "serve", "-listen", "unix;"+sockPath)
		defer cmd.Process.Signal(os.Kill)
		var err error
		listener, err = listeners.NewNetListener(network, address)
		if err != nil {
			return err
		}
		dialer = dialers.NewNetDialer("unix", sockPath)
		log.Printf("Gox LSP daemon: listening on %s network, address %s...", network, address)
		defer log.Printf("Gox LSP daemon: exiting")
	} else {
		log.Printf("Gox LSP daemon: listening on stdio...")
		listener = listeners.NewStdioListener()
		dialer = dialers.NewCmdDialer(args.Gopls())
	}
	server := server.NewServer(listener, dialer, args.Timeout())
	server.Wait()
	return nil
}
