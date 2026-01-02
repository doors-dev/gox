package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"

	jsonrpc2 "github.com/doors-dev/gox/internal/jsonrpc"
	"github.com/doors-dev/gox/internal/lsp"
	"github.com/doors-dev/gox/cmd/gox/internal/server/conn"
)

func newBridge(ctx context.Context, client io.ReadWriteCloser, gopls io.ReadWriteCloser) *bridge {
	ctx, cancel := context.WithCancel(ctx)
	b := &bridge{
		ctx:           ctx,
		cancel:        cancel,
		client:        conn.NewConn(ctx, client),
		gopls:         conn.NewConn(ctx, gopls),
		responseState: newResponseState(),
	}
	b.router = lsp.NewRouter(b)
	return b
}

type bridge struct {
	ctx           context.Context
	router        lsp.Router
	cancel        context.CancelFunc
	client        *conn.Conn
	gopls         *conn.Conn
	responseState *responseState
	mu            sync.Mutex
	wg            sync.WaitGroup
}

func (b *bridge) run() {
	defer b.responseState.cancelAll()
	for {
		select {
		case m, ok := <-b.client.Read():
			if !ok {
				b.close(b.gopls)
				return
			}
			b.incoming(lsp.Client, m)
		case m, ok := <-b.gopls.Read():
			if !ok {
				b.close(b.client)
				return
			}
			b.incoming(lsp.Gopls, m)
		}
	}
}

func (b *bridge) incoming(role lsp.Role, message jsonrpc2.Message) {
	switch m := message.(type) {
	case *jsonrpc2.Request:
		if m.ID.IsValid() {
			b.incomingCall(role, m.ID, m.Method, m.Params)
		} else {
			b.incomingNotification(role, m.Method, m.Params)
		}
	case *jsonrpc2.Response:
		b.incomingResponse(role, m.ID, m.Result, m.Error)
	}
}

func (b *bridge) incomingResponse(role lsp.Role, d jsonrpc2.ID, result json.RawMessage, err error) {
	numId, ok := d.Raw().(int64)
	if !ok {
		slog.Error("Got non number response from " + string(role))
	}
	ok = b.responseState.respond(numId, lsp.Response{
		Err:    err,
		Result: result,
	})
	if !ok {
		slog.Warn("Response by id not found " + fmt.Sprint(numId))
	}
}

func (b *bridge) incomingCall(role lsp.Role, id jsonrpc2.ID, method string, params json.RawMessage) {
	b.router.Call(role, lsp.Request{
		Method: method,
		Params: params,
	}, func(resp lsp.Response) {
		if errors.Is(resp.Err, context.Canceled) {
			return
		}
		b.write(role, &jsonrpc2.Response{
			Result: resp.Result,
			Error:  resp.Err,
			ID:     id,
		})
	})
}

func (b *bridge) incomingNotification(role lsp.Role, method string, params json.RawMessage) {
	b.router.Notification(role, lsp.Request{
		Method: method,
		Params: params,
	})
}

func (b *bridge) Call(role lsp.Role, call lsp.Request, callback lsp.Callback) {
	id := b.responseState.issueRequest(callback)
	err := b.write(role, &jsonrpc2.Request{
		ID:     jsonrpc2.Int64ID(id),
		Method: call.Method,
		Params: call.Params,
	})
	if err != nil {
		b.responseState.respond(id, lsp.Response{Err: err})
		return
	}
}

func (b *bridge) Notify(role lsp.Role, notification lsp.Request) error {
	return b.write(role, &jsonrpc2.Request{
		Method: notification.Method,
		Params: notification.Params,
	})
}

func (b *bridge) write(role lsp.Role, message jsonrpc2.Message) error {
	switch role {
	case lsp.Client:
		if b.ctx.Err() != nil {
			return jsonrpc2.ErrClientClosing
		}
		if err := b.client.Write(message); err != nil {
			return jsonrpc2.ErrClientClosing
		}
	case lsp.Gopls:
		if b.ctx.Err() != nil {
			return jsonrpc2.ErrServerClosing
		}
		if err := b.gopls.Write(message); err != nil {
			return jsonrpc2.ErrServerClosing
		}
	}
	return nil
}

func (b *bridge) close(conn *conn.Conn) {
	b.cancel()
	for {
		_, ok := <-conn.Read()
		if ok {
			continue
		}
		break
	}
}
