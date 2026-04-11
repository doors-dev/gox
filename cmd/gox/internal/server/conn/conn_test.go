package conn

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	jsonrpc2 "github.com/doors-dev/gox/internal/jsonrpc"
)

func TestConnReadWriteAndCancel(t *testing.T) {
	serverSide, clientSide := net.Pipe()
	defer clientSide.Close()

	ctx, cancel := context.WithCancel(context.Background())
	conn := NewConn(ctx, serverSide)

	clientFramer := jsonrpc2.HeaderFramer()
	clientReader := clientFramer.Reader(clientSide)
	clientWriter := clientFramer.Writer(clientSide)

	request, err := jsonrpc2.NewNotification("workspace/symbol", map[string]string{"query": "View"})
	if err != nil {
		t.Fatalf("NewNotification() error = %v", err)
	}
	readErrCh := make(chan error, 1)
	go func() {
		msg, err := clientReader.Read(context.Background())
		if err != nil {
			readErrCh <- err
			return
		}
		req, ok := msg.(*jsonrpc2.Request)
		if !ok || req.Method != "workspace/symbol" {
			readErrCh <- io.ErrUnexpectedEOF
			return
		}
		readErrCh <- nil
	}()
	if err := conn.Write(request); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := <-readErrCh; err != nil {
		t.Fatalf("client Read() error = %v", err)
	}

	response, err := jsonrpc2.NewResponse(jsonrpc2.Int64ID(1), map[string]bool{"ok": true}, nil)
	if err != nil {
		t.Fatalf("NewResponse() error = %v", err)
	}
	if err := clientWriter.Write(context.Background(), response); err != nil {
		t.Fatalf("client Write() error = %v", err)
	}

	select {
	case incoming, ok := <-conn.Read():
		if !ok {
			t.Fatal("Read() channel closed before message arrived")
		}
		if _, ok := incoming.(*jsonrpc2.Response); !ok {
			t.Fatalf("incoming message = %#v", incoming)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for incoming message")
	}

	cancel()
	select {
	case _, ok := <-conn.Read():
		if ok {
			for {
				if _, ok := <-conn.Read(); !ok {
					break
				}
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Read() channel did not close after cancel")
	}

	if err := clientSide.Close(); err != nil && err != io.EOF {
		t.Fatalf("client close error = %v", err)
	}
}
