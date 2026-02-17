package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/doors-dev/gox/cmd/gox/command"
	"github.com/doors-dev/gox/cmd/gox/internal/common"
	"github.com/doors-dev/gox/cmd/gox/internal/dialers"
	"github.com/doors-dev/gox/cmd/gox/internal/listeners"
	"github.com/doors-dev/gox/cmd/gox/internal/processor"
	"github.com/doors-dev/gox/cmd/gox/internal/server"
	"github.com/gofrs/flock"
)

func initLogger(file string, level slog.Level, enable bool) {
	if !enable {
		h := slog.NewTextHandler(io.Discard, nil)
		slog.SetDefault(slog.New(h))
		return
	}
	f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		panic(err)
	}
	h := slog.NewTextHandler(f, &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(h))
}

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

func (s starter) Host(args command.HostArgs) error {
	lockPath, err := args.LockPath()
	if err != nil {
		return nil
	}
	serveArgs, err := args.ServeArgs()
	if err != nil {
		return nil
	}
	lock := flock.New(lockPath)
	locked, err := lock.TryLock()
	if err != nil {
		return err
	}
	if !locked {
		return nil
	}
	defer lock.Unlock()
	return s.Serve(serveArgs)
}

func (s starter) Serve(args command.ServeArgs) error {
	level, file, enabled := args.LoggerInfo()
	initLogger(file, level, enabled)
	var listener server.Listener
	var dialer server.Dialer
	network, address, ok := args.Socket()
	if ok {
		sockPath, err := common.SocketPath(fmt.Sprintf("gopls_%d", os.Getpid()))
		if err != nil {
			return err
		}
		goArgs := []string{"serve", "-listen", "unix;" + sockPath}
		if args.Timeout() != 0 {
			timeout := args.Timeout() + time.Second
			goArgs = append(goArgs, "-listen.timeout", timeout.String())
		}
		cmd := exec.Command(args.Gopls(), goArgs...)
		if err := cmd.Start(); err != nil {
			return err
		}
		defer cmd.Process.Signal(os.Kill)
		listener, err = listeners.NewNetListener(network, address)
		if err != nil {
			return err
		}
		dialer = dialers.NewNetDialer("unix", sockPath)
		log.Printf("Gox LSP daemon: listening on %s network, address %s", network, address)
		defer log.Printf("Gox LSP daemon: exiting")
	} else {
		log.Printf("Gox LSP daemon: listening on stdio")
		listener = listeners.NewStdioListener()
		dialer = dialers.NewCmdDialer(args.Gopls())
	}
	server := server.NewServer(listener, dialer, args.Timeout())
	slog.Info("...")
	server.Wait()
	return nil
}

func appendLine(filename, line string) error {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(line + "\n")
	return err
}


func (s starter) Client(args command.ClientArgs) error {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	appendLine("/tmp/gox.pwd", dir)
	transport, address, err := args.Socket()
	if err != nil {
		return err
	}
	command, commandArgs := args.HostCommand()
	var goxConn io.ReadWriteCloser
	var clientConn io.ReadWriteCloser
	for range 3 {
		if err = common.Spawn(command, commandArgs...); err != nil {
			return err
		}
		dialer := dialers.NewNetDialer(transport, address)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		goxConn, err = dialer.Dial(ctx)
		cancel()
		if err != nil {
			continue
		}
		listener := listeners.NewStdioListener()
		clientConn, err = listener.Accept()
		if err != nil {
			goxConn.Close()
		}
		break
	}
	if err != nil {
		return err
	}
	wg := sync.WaitGroup{}
	wg.Go(func() {
		io.Copy(goxConn, clientConn)
	})
	wg.Go(func() {
		io.Copy(clientConn, goxConn)
	})
	wg.Wait()
	goxConn.Close()
	clientConn.Close()
	return nil
}
