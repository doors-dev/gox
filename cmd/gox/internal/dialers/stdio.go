package dialers

import (
	"context"
	"io"
	"os"
	"os/exec"

	"github.com/doors-dev/gox/cmd/gox/internal/common"
	"github.com/doors-dev/gox/cmd/gox/internal/server"
)

func NewCmdDialer(command string, args ...string) server.Dialer {
	return &cmdDialer{
		command: command,
		args:    args,
	}
}

type cmdDialer struct {
	command string
	args    []string
}

func (s cmdDialer) Dial(context.Context) (io.ReadWriteCloser, error) {
	cmd := exec.Command(s.command, s.args...)
	goplsIn, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	goplsOut, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return common.RWClose(
		goplsOut,
		goplsIn,
		func() error {
			cmd.Process.Signal(os.Kill)
			goplsOut.Close()
			goplsIn.Close()
			cmd.Wait()
			return nil
		},
	), nil
}
