package listeners

import (
	"errors"
	"io"
	"os"
	"sync/atomic"

	"github.com/doors-dev/gox/cmd/internal/common"
	"github.com/doors-dev/gox/cmd/internal/server"
)

func NewStdioListener() server.Listener {
	return &stdioListener{
		close: make(chan struct{}),
	}
}

type stdioListener struct {
	fired atomic.Bool
	close chan struct{}
}

func (s *stdioListener) Accept() (io.ReadWriteCloser, error) {
	if s.fired.Swap(true) {
		<-s.close
		return nil, errors.New("closed")
	}
	return common.RWClose(
		os.Stdin,
		os.Stdout,
		func() error {
			os.Stdout.Close()
			os.Stdin.Close()
			return nil
		},
	), nil
}

func (s *stdioListener) Close() error {
	close(s.close)
	return nil
}
