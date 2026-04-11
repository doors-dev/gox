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

	var server io.ReadWriteCloser
	select {
	case server = <-accepted:
		defer server.Close()
	case <-time.After(2 * time.Second):
		t.Fatal("Accept() did not return in time")
	}

	// Verify the accepted conn is wired to the dialed client by
	// round-tripping bytes in both directions.
	if _, err := client.Write([]byte("ping")); err != nil {
		t.Fatalf("client Write() error = %v", err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(server, buf); err != nil {
		t.Fatalf("server ReadFull() error = %v", err)
	}
	if string(buf) != "ping" {
		t.Fatalf("server read %q, want %q", buf, "ping")
	}
	if _, err := server.Write([]byte("pong")); err != nil {
		t.Fatalf("server Write() error = %v", err)
	}
	if _, err := io.ReadFull(client, buf); err != nil {
		t.Fatalf("client ReadFull() error = %v", err)
	}
	if string(buf) != "pong" {
		t.Fatalf("client read %q, want %q", buf, "pong")
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
