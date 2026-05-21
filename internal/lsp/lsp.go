package lsp

import (
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/doors-dev/gox/internal/common"
	"github.com/doors-dev/gox/internal/jsonrpc"
	"github.com/doors-dev/gox/internal/workspace"
)

type Json = *ast.Node

type Role string

const (
	Client Role = "client"
	Gopls  Role = "gopls"
)

func (r Role) Revert() Role {
	switch r {
	case Client:
		return Gopls
	case Gopls:
		return Client
	}
	panic("Unknoen role")
}

type Response struct {
	Err    error
	Result json.RawMessage
}

type Request struct {
	Method string
	Params json.RawMessage
}

type Bridge interface {
	Call(role Role, call Request, callback Callback)
	Notify(role Role, notification Request) error
}

type Callback func(Response)

type Router = *router

type router struct {
	session      *session
	clientNotifs map[method]onNotif
	serverNotifs map[method]onNotif
	clientCalls  map[method]onCall
	serverCalls  map[method]onCall
}

type caller interface {
	forward()
	method() method
	proxy(params Json, handler func(res Json))
	res(result Json)
	err(err *common.Err)
}

type notifier interface {
	forward()
	method() method
	notify(params Json)
	err(err *common.Err)
}

type request struct {
	data     json.RawMessage
	cb       Callback
	sess     *session
	m        method
	role     Role
	locker   sync.Locker
	logAttrs []any
}

func (c *request) method() method {
	return c.m
}

func (c *request) notify(params Json) {
	data, err := params.MarshalJSON()
	if err != nil {
		panic("param marshaling error - should not happen")
	}
	c.sess.bridge.Notify(c.role.Revert(), Request{
		Method: string(c.m),
		Params: data,
	})
}

func (c *request) res(result Json) {
	data, err := result.MarshalJSON()
	if err != nil {
		panic("param marshaling error - should not happen")
	}
	c.cb(Response{Result: data})
}

func (c *request) err(err *common.Err) {
	attrs := []any{"method", c.m, "msg", err.Msg, "error", err.Wire.Error()}
	attrs = append(attrs, c.logAttrs...)
	attrs = append(attrs, "roots", c.sess.rootsLocked())
	if c.isCall() {
		attrs = append(attrs, "role", c.role)
		slog.Error("Error response to call", attrs...)
		c.sess.logError("Error response to call: [method=" + string(c.m) + ", role=" + string(c.role) + ", msg=" + err.Msg + ", error=" + err.Wire.Error() + "]")
		c.cb(Response{Err: err.Wire})
		c.sess.showError(err.Error())
	} else {
		slog.Error("Error \"response\" to notification", attrs...)
		c.sess.logError("Error \"response\" to notification: [method=" + string(c.m) + ", msg=" + err.Msg + ", error=" + err.Wire.Error() + "]")
		c.sess.showError(err.Error())
	}
}

func (c *request) isCall() bool {
	return c.cb != nil
}

func (c *request) forward() {
	if c.isCall() {
		c.sess.bridge.Call(c.role.Revert(), Request{
			Method: string(c.m),
			Params: c.data,
		}, c.cb)
	} else {
		c.sess.bridge.Notify(c.role.Revert(), Request{
			Method: string(c.m),
			Params: c.data,
		})
	}
}

func (c *request) proxy(params Json, handler func(res Json)) {
	data, err := params.MarshalJSON()
	if err != nil {
		panic("param marshaling error - should not happen")
	}
	c.sess.bridge.Call(c.role.Revert(), Request{
		Method: string(c.m),
		Params: data,
	}, func(r Response) {
		c.locker.Lock()
		defer c.locker.Unlock()
		if r.Err != nil {
			attrs := []any{"method", c.m, "from", c.role.Revert(), "error", r.Err.Error()}
			attrs = append(attrs, c.logAttrs...)
			slog.Error("Got error response to call", attrs...)
			c.cb(r)
			return
		}
		node, err := sonic.Get(r.Result)
		if err != nil {
			attrs := []any{"method", c.m, "from", c.role.Revert(), "error", err.Error()}
			attrs = append(attrs, c.logAttrs...)
			slog.Error("Response parsing error", attrs...)
			c.sess.logError("Response parsing error: [method=" + string(c.m) + ", from=" + string(c.role.Revert()) + ", error=" + err.Error() + "]")
			c.cb(r)
			return
		}
		handler(&node)
	})
}

type onNotif func(n notifier, j Json)
type onCall func(c caller, j Json)

func (r Router) Notification(role Role, n Request) {
	r.session.man().Lock()
	defer r.session.man().Unlock()
	m := method(n.Method)
	var handler onNotif
	var ok bool
	switch role {
	case Client:
		handler, ok = r.clientNotifs[m]
	case Gopls:
		handler, ok = r.serverNotifs[m]
	default:
		panic("unknown role")
	}
	if ok {
		req := &request{
			data:     n.Params,
			sess:     r.session,
			role:     role,
			m:        m,
			locker:   r.session.man(),
			logAttrs: nil,
		}
		node, err := sonic.Get(n.Params)
		if err != nil {
			slog.Error("Notification parsing error", "method", m, "from", role, "error", err.Error(), "roots", r.session.rootsLocked())
			r.session.logError("Notification parsing error: [method=" + string(m) + ", from=" + string(role) + ", error=" + err.Error() + "]")
			req.err(common.FromErr(jsonrpc.ErrParse, err))
			return
		}
		req.logAttrs = requestLogAttrs(&node)
		slog.Debug("Notification", append([]any{"method", m, "from", role}, req.logAttrs...)...)
		handler(req, &node)
		return
	}
	slog.Debug("Notification", "method", m, "from", role)
	r.session.bridge.Notify(role.Revert(), n)
}

func (r Router) Call(role Role, call Request, cb Callback) {
	r.session.man().Lock()
	defer r.session.man().Unlock()
	m := method(call.Method)
	var handler onCall
	var ok bool
	switch role {
	case Client:
		handler, ok = r.clientCalls[m]
	case Gopls:
		handler, ok = r.serverCalls[m]
	default:
		panic("unknown role")
	}
	if ok {
		req := &request{
			sess:     r.session,
			role:     role,
			m:        m,
			data:     call.Params,
			cb:       cb,
			locker:   r.session.man(),
			logAttrs: nil,
		}
		node, err := sonic.Get(call.Params)
		if err != nil {
			slog.Error("Call parsing error", "method", m, "from", role, "error", err.Error(), "roots", r.session.rootsLocked())
			r.session.logError("Call parsing error: [method=" + string(m) + ", from=" + string(role) + ", error=" + err.Error() + "]")
			req.err(common.FromErr(jsonrpc.ErrParse, err))
			return
		}
		req.logAttrs = requestLogAttrs(&node)
		slog.Debug("Call", append([]any{"method", m, "from", role}, req.logAttrs...)...)
		handler(req, &node)
		return
	}
	slog.Debug("Call", "method", m, "from", role)
	r.session.bridge.Call(role.Revert(), call, cb)
}

func NewRouter(bridge Bridge) Router {
	b := builder{
		clientNotifs: make(map[method]onNotif),
		serverNotifs: make(map[method]onNotif),
		clientCalls:  make(map[method]onCall),
		serverCalls:  make(map[method]onCall),
	}
	return b.build(bridge)
}

type builder struct {
	clientNotifs map[method]onNotif
	serverNotifs map[method]onNotif
	clientCalls  map[method]onCall
	serverCalls  map[method]onCall
}

func (b *builder) build(br Bridge) Router {
	session := &session{
		bridge:  br,
		manager: workspace.NewManager(),
	}
	initClientCalls(session, b.clientCall)
	initClientNotifs(session, b.clientNotif)
	initServerCalls(session, b.serverCall)
	initServerNotifs(session, b.serverNotif)
	return &router{
		session:      session,
		clientNotifs: b.clientNotifs,
		serverNotifs: b.serverNotifs,
		clientCalls:  b.clientCalls,
		serverCalls:  b.serverCalls,
	}
}

func (b *builder) clientNotif(on onNotif, m ...method) {
	if len(m) == 0 {
		panic("no method")
	}
	for _, method := range m {
		_, ok := b.clientNotifs[method]
		if ok {
			panic("duplicate client notif handler")
		}
		b.clientNotifs[method] = on
	}
}

func (b *builder) clientCall(on onCall, m ...method) {
	if len(m) == 0 {
		panic("no method")
	}
	for _, method := range m {
		_, ok := b.clientCalls[method]
		if ok {
			panic("duplicate client call handler")
		}
		b.clientCalls[method] = on
	}
}

func (b *builder) serverNotif(on onNotif, m ...method) {
	if len(m) == 0 {
		panic("no method")
	}
	for _, method := range m {
		_, ok := b.serverNotifs[method]
		if ok {
			panic("duplicate server notif handler")
		}
		b.serverNotifs[method] = on
	}
}

func (b *builder) serverCall(on onCall, m ...method) {
	if len(m) == 0 {
		panic("no method")
	}
	for _, method := range m {
		_, ok := b.serverCalls[method]
		if ok {
			panic("duplicate server call handler")
		}
		b.serverCalls[method] = on
	}
}
