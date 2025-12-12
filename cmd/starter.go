package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/doors-dev/gox/cmd/internal/dialers"
	"github.com/doors-dev/gox/cmd/internal/listeners"
	"github.com/doors-dev/gox/cmd/internal/server"
)

type starter struct{}

func (s starter) Default() error {
	listener := listeners.NewStdioListener()
	dialer := dialers.NewCmdDialer("gopls")
	log.Printf("Gox LSP daemon: listening on stdio...")
	server := server.NewServer(listener, dialer, time.Second)
	server.Wait()
	return nil
}

func (s starter) Serve(network string, address string, timeout time.Duration) error {
	sockPath := filepath.Join(os.TempDir(), fmt.Sprintf("gox_%d.sock", os.Getpid()))
	cmd := exec.Command("gopls", "serve", "-listen", "unix;"+sockPath)
	if err := cmd.Start(); err != nil {
		return err
	}
	defer cmd.Process.Signal(os.Kill)
	listener, err := listeners.NewNetListener(network, address)
	if err != nil {
		return err
	}
	dialer := dialers.NewNetDialer("unix", sockPath)
	server := server.NewServer(listener, dialer, timeout)
	log.Printf("Gox LSP daemon: listening on %s network, address %s...", network, address)
	defer log.Printf("Gox LSP daemon: exiting")
	server.Wait()
	return nil
}
