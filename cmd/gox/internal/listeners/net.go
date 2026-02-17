package listeners

import (
	"io"
	"net"
	"os"

	"github.com/doors-dev/gox/cmd/gox/internal/server"
)

func NewNetListener(network string, address string) (server.Listener, error) {
	if network == "unix" {
		os.Remove(address)
	}
	l, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}
	return &netListener{
		l: l,
	}, nil
}

type netListener struct {
	l net.Listener
}

func (n netListener) Accept() (io.ReadWriteCloser, error) {
	return n.l.Accept()
}

func (n netListener) Close() error {
	return n.l.Close()
}
