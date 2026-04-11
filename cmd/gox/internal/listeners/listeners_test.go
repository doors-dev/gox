package listeners

import (
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"
)

func TestNetListenerAcceptAndClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gox.sock")
	listener, err := NewNetListener("unix", path)
	if err != nil {
		t.Fatalf("NewNetListener() error = %v", err)
	}
	defer listener.Close()

	accepted := make(chan io.ReadWriteCloser, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()

	client, err := net.Dial("unix", path)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer client.Close()

	select {
	case conn := <-accepted:
		defer conn.Close()
	case <-time.After(2 * time.Second):
		t.Fatal("Accept() did not return in time")
	}
}

func TestStdioListenerSingleAccept(t *testing.T) {
	listener := NewStdioListener()
	conn, err := listener.Accept()
	if err != nil {
		t.Fatalf("first Accept() error = %v", err)
	}
	if conn == nil {
		t.Fatal("first Accept() returned nil conn")
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := listener.Accept(); err == nil {
		t.Fatal("second Accept() error = nil, want error")
	}
}
