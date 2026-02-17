package dialers

import (
	"context"
	"io"
	"net"
	"time"

	"github.com/doors-dev/gox/cmd/gox/internal/server"
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
	for {
		conn, err := d.DialContext(ctx, s.network, s.address)
		if err != nil {
			if ctx.Err() != nil {
				return nil, err
			}
			<-time.After(50 * time.Millisecond)
			continue
		}
		return conn, nil
	}
}
