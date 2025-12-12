package dialers

import (
	"context"
	"io"
	"net"

	"github.com/doors-dev/gox/cmd/internal/server"
)

func NewNetDialer(network string, address string) server.Dialer {
	return &socketDialer{
		network: network,
		address: address,
	}
}

type socketDialer struct {
	network string
	address string
}

func (s socketDialer) Dial(ctx context.Context) (io.ReadWriteCloser, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, s.network, s.address)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
