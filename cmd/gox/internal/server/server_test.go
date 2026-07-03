package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	jsonrpc2 "github.com/doors-dev/gox/internal/jsonrpc"
	"github.com/doors-dev/gox/internal/lsp"
)

type testListener struct {
	conns  chan io.ReadWriteCloser
	closed chan struct{}
	once   sync.Once
	count  atomic.Int32
}

func newTestListener() *testListener {
	return &testListener{
		conns:  make(chan io.ReadWriteCloser, 1),
		closed: make(chan struct{}),
	}
}

func (l *testListener) Accept() (io.ReadWriteCloser, error) {
	select {
	case <-l.closed:
		return nil, errors.New("listener closed")
	case conn, ok := <-l.conns:
		if !ok {
			return nil, errors.New("listener closed")
		}
		return conn, nil
	}
}

func (l *testListener) Close() error {
	l.once.Do(func() {
		l.count.Add(1)
		close(l.closed)
		close(l.conns)
	})
	return nil
}

type testDialer struct {
	rwc io.ReadWriteCloser
	err error
}

func (d testDialer) Dial(context.Context) (io.ReadWriteCloser, error) {
	if d.err != nil {
		return nil, d.err
	}
	return d.rwc, nil
}

type spyRWC struct {
	closed chan struct{}
	once   sync.Once
}

func newSpyRWC() *spyRWC {
	return &spyRWC{closed: make(chan struct{})}
}

func (r *spyRWC) Read([]byte) (int, error)    { return 0, io.EOF }
func (r *spyRWC) Write(p []byte) (int, error) { return len(p), nil }
func (r *spyRWC) Close() error {
	r.once.Do(func() { close(r.closed) })
	return nil
}

func TestServerWaitReturnsAfterDialError(t *testing.T) {
	listener := newTestListener()
	client := newSpyRWC()
	listener.conns <- client

	srv := NewServer(listener, testDialer{err: errors.New("boom")}, 0)

	done := make(chan struct{})
	go func() {
		srv.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop after dial error")
	}

	select {
	case <-client.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("client connection was not closed")
	}

	select {
	case <-listener.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("listener was not closed")
	}
	if listener.count.Load() != 1 {
		t.Fatalf("listener Close() count = %d, want 1", listener.count.Load())
	}
}

func TestServerIdleKillTimerStopsServer(t *testing.T) {
	listener := newTestListener()
	srv := NewServer(listener, testDialer{}, 20*time.Millisecond)

	done := make(chan struct{})
	go func() {
		srv.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("idle server did not stop after timeout")
	}

	if listener.count.Load() != 1 {
		t.Fatalf("listener Close() count = %d, want 1", listener.count.Load())
	}
}

func TestServerConnectDisconnectLifecycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv := &server{
		ctx:         ctx,
		cancel:      cancel,
		killTimeout: 50 * time.Millisecond,
	}

	if !srv.setKillTimer() {
		t.Fatal("setKillTimer() = false, want true")
	}
	if srv.killTimer == nil {
		t.Fatal("killTimer = nil after setKillTimer")
	}
	if !srv.onConnect() {
		t.Fatal("onConnect() = false, want true")
	}
	if srv.killTimer != nil {
		t.Fatal("killTimer should be cleared after onConnect")
	}

	srv.onDisconnect()
	if srv.killTimer == nil {
		t.Fatal("killTimer = nil after onDisconnect")
	}
	srv.killTimer.Stop()

	noTimeout := &server{}
	if noTimeout.setKillTimer() {
		t.Fatal("setKillTimer() = true for zero timeout, want false")
	}
}

func TestServerWaitReturnsAfterConnectedClientCloses(t *testing.T) {
	listener := newTestListener()
	serverClient, clientPeer := net.Pipe()
	serverGopls, goplsPeer := net.Pipe()
	defer clientPeer.Close()
	defer goplsPeer.Close()

	listener.conns <- serverClient
	srv := NewServer(listener, testDialer{rwc: serverGopls}, 0)

	if err := clientPeer.Close(); err != nil {
		t.Fatalf("client close error = %v", err)
	}

	done := make(chan struct{})
	go func() {
		srv.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("connected server did not stop after client close")
	}

	if listener.count.Load() != 1 {
		t.Fatalf("listener Close() count = %d, want 1", listener.count.Load())
	}
}

func TestResponseStateIssueRespondAndCancelAll(t *testing.T) {
	state := newResponseState()

	firstCh := make(chan lsp.Response, 1)
	secondCh := make(chan lsp.Response, 1)
	firstID := state.issueRequest(func(r lsp.Response) { firstCh <- r }, lsp.Origin{Role: lsp.Client, ID: jsonrpc2.StringID("a")})
	secondID := state.issueRequest(func(r lsp.Response) { secondCh <- r }, lsp.Origin{})

	if firstID != 1 || secondID != 2 {
		t.Fatalf("issued ids = (%d, %d), want (1, 2)", firstID, secondID)
	}
	if !state.respond(firstID, lsp.Response{Result: []byte(`"ok"`)}) {
		t.Fatal("respond(firstID) = false, want true")
	}
	if state.respond(firstID, lsp.Response{}) {
		t.Fatal("second respond(firstID) = true, want false")
	}

	select {
	case resp := <-firstCh:
		if string(resp.Result) != `"ok"` {
			t.Fatalf("first response = %s, want \"ok\"", resp.Result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first callback")
	}

	state.cancelAll()
	select {
	case resp := <-secondCh:
		if !errors.Is(resp.Err, context.Canceled) {
			t.Fatalf("cancelAll() err = %v, want context.Canceled", resp.Err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for canceled callback")
	}

	if len(state.requests) != 0 {
		t.Fatalf("requests len = %d, want 0", len(state.requests))
	}
	if len(state.origins) != 0 {
		t.Fatalf("origins len = %d, want 0", len(state.origins))
	}
}

func TestBridgeTranslatesCancelRequestID(t *testing.T) {
	clientServer, clientPeer := net.Pipe()
	goplsServer, goplsPeer := net.Pipe()
	defer clientPeer.Close()
	defer goplsPeer.Close()

	b := newBridge(context.Background(), clientServer, goplsServer)
	defer b.cancel()
	go b.run()

	framer := jsonrpc2.HeaderFramer()
	clientWriter := framer.Writer(clientPeer)
	goplsReader := framer.Reader(goplsPeer)

	goplsCh := make(chan *jsonrpc2.Request, 4)
	go func() {
		for {
			msg, err := goplsReader.Read(context.Background())
			if err != nil {
				close(goplsCh)
				return
			}
			if req, ok := msg.(*jsonrpc2.Request); ok {
				goplsCh <- req
			}
		}
	}()

	writeClient := func(msg jsonrpc2.Message, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("marshal message: %v", err)
		}
		if err := clientWriter.Write(context.Background(), msg); err != nil {
			t.Fatalf("client write error = %v", err)
		}
	}
	readGopls := func() *jsonrpc2.Request {
		t.Helper()
		select {
		case req, ok := <-goplsCh:
			if !ok {
				t.Fatal("gopls connection closed unexpectedly")
			}
			return req
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for gopls-bound message")
			return nil
		}
	}

	writeClient(jsonrpc2.NewCall(jsonrpc2.StringID("abc"), "test/echo", map[string]string{"query": "View"}))
	forwarded := readGopls()
	if forwarded.Method != "test/echo" {
		t.Fatalf("forwarded method = %q, want test/echo", forwarded.Method)
	}
	bridgeID, ok := forwarded.ID.Raw().(int64)
	if !ok {
		t.Fatalf("forwarded id = %#v, want int64", forwarded.ID.Raw())
	}
	if raw := forwarded.ID.Raw(); raw == "abc" {
		t.Fatal("forwarded request kept the client id")
	}

	writeClient(jsonrpc2.NewNotification("$/cancelRequest", map[string]any{"id": "abc"}))
	cancelReq := readGopls()
	if cancelReq.Method != "$/cancelRequest" || cancelReq.ID.IsValid() {
		t.Fatalf("gopls message = %#v, want $/cancelRequest notification", cancelReq)
	}
	var cancelParams struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(cancelReq.Params, &cancelParams); err != nil {
		t.Fatalf("decode cancel params: %v", err)
	}
	if cancelParams.ID != bridgeID {
		t.Fatalf("cancel id = %d, want translated bridge id %d", cancelParams.ID, bridgeID)
	}

	writeClient(jsonrpc2.NewNotification("$/cancelRequest", map[string]any{"id": "unknown"}))
	writeClient(jsonrpc2.NewNotification("test/marker", map[string]string{}))
	next := readGopls()
	if next.Method != "test/marker" {
		t.Fatalf("gopls message after unknown cancel = %q, want test/marker (unknown cancel must be dropped)", next.Method)
	}
}

func TestBridgeCallNotifyAndClosingErrors(t *testing.T) {
	clientServer, clientPeer := net.Pipe()
	goplsServer, goplsPeer := net.Pipe()
	defer clientPeer.Close()
	defer goplsPeer.Close()

	b := newBridge(context.Background(), clientServer, goplsServer)
	defer b.cancel()

	clientReader := jsonrpc2.HeaderFramer().Reader(clientPeer)
	goplsReader := jsonrpc2.HeaderFramer().Reader(goplsPeer)

	clientReadCh := make(chan *jsonrpc2.Request, 1)
	go func() {
		msg, err := clientReader.Read(context.Background())
		if err != nil {
			t.Errorf("client Read() error = %v", err)
			return
		}
		req, ok := msg.(*jsonrpc2.Request)
		if !ok {
			t.Errorf("client message = %T, want *jsonrpc2.Request", msg)
			return
		}
		clientReadCh <- req
	}()

	b.Call(lsp.Client, lsp.Request{
		Method: "workspace/symbol",
		Params: json.RawMessage(`{"query":"Card"}`),
	}, func(lsp.Response) {})

	select {
	case req := <-clientReadCh:
		if req.Method != "workspace/symbol" || !req.ID.IsValid() {
			t.Fatalf("client request = %#v", req)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for client-bound call")
	}

	goplsReadCh := make(chan *jsonrpc2.Request, 1)
	go func() {
		msg, err := goplsReader.Read(context.Background())
		if err != nil {
			t.Errorf("gopls Read() error = %v", err)
			return
		}
		req, ok := msg.(*jsonrpc2.Request)
		if !ok {
			t.Errorf("gopls message = %T, want *jsonrpc2.Request", msg)
			return
		}
		goplsReadCh <- req
	}()

	if err := b.Notify(lsp.Gopls, lsp.Request{
		Method: "initialized",
		Params: json.RawMessage(`{}`),
	}); err != nil {
		t.Fatalf("Notify() error = %v", err)
	}

	select {
	case req := <-goplsReadCh:
		if req.Method != "initialized" || req.ID.IsValid() {
			t.Fatalf("gopls request = %#v", req)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for gopls-bound notification")
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	closingBridge := &bridge{ctx: canceledCtx}
	if err := closingBridge.write(lsp.Client, nil); !errors.Is(err, jsonrpc2.ErrClientClosing) {
		t.Fatalf("write(client) error = %v, want ErrClientClosing", err)
	}
	if err := closingBridge.write(lsp.Gopls, nil); !errors.Is(err, jsonrpc2.ErrServerClosing) {
		t.Fatalf("write(gopls) error = %v, want ErrServerClosing", err)
	}
}
