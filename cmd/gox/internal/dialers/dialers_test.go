package dialers

import (
	"context"
	"net"
	"runtime"
	"testing"
	"time"
)

func TestNetDialerConnectsAndRespectsContext(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer listener.Close()

	accepted := make(chan struct{}, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			conn.Close()
			accepted <- struct{}{}
		}
	}()

	conn, err := NewNetDialer("tcp", listener.Addr().String()).Dial(context.Background())
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	conn.Close()
	select {
	case <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("listener did not accept the dialed connection")
	}

	closedListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen(closed target) error = %v", err)
	}
	closedAddr := closedListener.Addr().String()
	closedListener.Close()
	cancelCtx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()
	if _, err := NewNetDialer("tcp", closedAddr).Dial(cancelCtx); err == nil {
		t.Fatal("Dial(canceled) error = nil, want error")
	}
}

func TestCmdDialerStartsProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/cat")
	}
	conn, err := NewCmdDialer("/bin/cat").Dial(context.Background())
	if err != nil {
		t.Fatalf("Dial(/bin/cat) error = %v", err)
	}
	if conn == nil {
		t.Fatal("Dial(/bin/cat) returned nil conn")
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if _, err := NewCmdDialer("/definitely/missing/gopls").Dial(context.Background()); err == nil {
		t.Fatal("Dial(missing command) error = nil, want error")
	}
}
