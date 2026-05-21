package lsp

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/doors-dev/gox/internal/common"
	"github.com/doors-dev/gox/internal/docpath"
	"github.com/doors-dev/gox/internal/workspace"
)

type recordedNotify struct {
	role Role
	req  Request
}

type recordedCall struct {
	role Role
	req  Request
}

type bridgeStub struct {
	callResp      Response
	notifications []recordedNotify
	calls         []recordedCall
}

func (b *bridgeStub) Call(role Role, call Request, callback Callback) {
	b.calls = append(b.calls, recordedCall{role: role, req: call})
	callback(b.callResp)
}

func (b *bridgeStub) Notify(role Role, notification Request) error {
	b.notifications = append(b.notifications, recordedNotify{role: role, req: notification})
	return nil
}

type asyncBridgeStub struct {
	callResp      Response
	notifications []recordedNotify
	calls         []recordedCall
}

func (b *asyncBridgeStub) Call(role Role, call Request, callback Callback) {
	b.calls = append(b.calls, recordedCall{role: role, req: call})
	go callback(b.callResp)
}

func (b *asyncBridgeStub) Notify(role Role, notification Request) error {
	b.notifications = append(b.notifications, recordedNotify{role: role, req: notification})
	return nil
}

func TestSessionBridgeAndWorkspaceHelpers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "view.gox")
	if err := os.WriteFile(path, []byte(lspHelperSource), 0o644); err != nil {
		t.Fatal(err)
	}
	dirURI := string(docpath.URIFromPath(dir))
	fileURI := string(docpath.URIFromPath(path))

	bridge := &bridgeStub{}
	sess := &session{
		bridge:   bridge,
		manager:  workspace.NewManager(),
		encoding: common.UTF16,
	}

	if sess.man() == nil {
		t.Fatal("man() returned nil")
	}
	if sess.enc() != common.UTF16 {
		t.Fatalf("enc() = %v, want UTF16", sess.enc())
	}
	sess.setEnc(common.UTF8)
	if sess.enc() != common.UTF8 {
		t.Fatalf("enc() after setEnc = %v, want UTF8", sess.enc())
	}

	sess.ensureWorkspaces([]string{dirURI})
	if doc, kind := sess.man().Doc(fileURI); doc == nil || kind != workspace.KindSource {
		t.Fatalf("Doc() after ensureWorkspaces = (%v, %v), want loaded source doc", doc, kind)
	}
	sess.removeWorkspace(dirURI)
	if doc, kind := sess.man().Doc(fileURI); doc != nil || kind != workspace.KindSource {
		t.Fatalf("Doc() after removeWorkspace = (%v, %v), want nil source doc", doc, kind)
	}
	sess.addWorkspace(dirURI)
	if doc, kind := sess.man().Doc(fileURI); doc == nil || kind != workspace.KindSource {
		t.Fatalf("Doc() after addWorkspace = (%v, %v), want loaded source doc", doc, kind)
	}

	params := mustJSONNode(t, `{"message":"hello"}`)
	sess.notifGo(didOpen, params)
	sess.notifClient(showMessage, params)
	if len(bridge.notifications) != 2 {
		t.Fatalf("notification count = %d, want 2", len(bridge.notifications))
	}
	if bridge.notifications[0].role != Gopls || bridge.notifications[0].req.Method != string(didOpen) {
		t.Fatalf("notifGo = %#v", bridge.notifications[0])
	}
	if bridge.notifications[1].role != Client || bridge.notifications[1].req.Method != string(showMessage) {
		t.Fatalf("notifClient = %#v", bridge.notifications[1])
	}

	bridge.notifications = nil
	sess.showInfo("info")
	sess.showWarn("warn")
	sess.showError("boom")
	sess.logError("logged")
	if len(bridge.notifications) != 4 {
		t.Fatalf("show/log notifications = %d, want 4", len(bridge.notifications))
	}
	assertNotifPayload(t, bridge.notifications[0], Client, showMessage, "info", 3)
	assertNotifPayload(t, bridge.notifications[1], Client, showMessage, "warn", 2)
	assertNotifPayload(t, bridge.notifications[2], Client, showMessage, "boom", 1)
	assertNotifPayload(t, bridge.notifications[3], Client, logMessage, "logged", 1)

	bridge.notifications = nil
	bridge.calls = nil
	bridge.callResp = Response{Result: json.RawMessage(`{"ok":true}`)}
	handled := false
	sess.callClientHandle(documentSymbol, params, func(r Response) {
		handled = true
		if string(r.Result) != `{"ok":true}` {
			t.Fatalf("callback result = %s", r.Result)
		}
	})
	if !handled {
		t.Fatal("callClientHandle callback was not invoked")
	}
	if len(bridge.calls) != 1 {
		t.Fatalf("call count = %d, want 1", len(bridge.calls))
	}
	if bridge.calls[0].role != Client || bridge.calls[0].req.Method != string(documentSymbol) {
		t.Fatalf("callClientHandle request = %#v", bridge.calls[0])
	}
	if len(bridge.notifications) != 0 {
		t.Fatalf("unexpected notifications on successful call: %#v", bridge.notifications)
	}

	bridge.notifications = nil
	bridge.calls = nil
	bridge.callResp = Response{Err: errors.New("boom")}
	handled = false
	sess.callClientHandle(documentSymbol, params, func(r Response) {
		handled = true
		if !errors.Is(r.Err, bridge.callResp.Err) {
			t.Fatalf("callback error = %v, want %v", r.Err, bridge.callResp.Err)
		}
	})
	if !handled {
		t.Fatal("callClientHandle error callback was not invoked")
	}
	if len(bridge.notifications) != 1 {
		t.Fatalf("notifications after error call = %d, want 1", len(bridge.notifications))
	}
	assertNotifPayload(t, bridge.notifications[0], Client, logMessage, "Call client result error: [method=textDocument/documentSymbol, error=boom]", 1)

	bridge.calls = nil
	bridge.callResp = Response{}
	sess.callClient(documentSymbol, params)
	if len(bridge.calls) != 1 || bridge.calls[0].req.Method != string(documentSymbol) {
		t.Fatalf("callClient() calls = %#v", bridge.calls)
	}
}

func TestDidSaveRejectsSourceOutsideWorkspace(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	inPath := filepath.Join(root, "view.gox")
	outPath := filepath.Join(outside, "view.gox")
	if err := os.WriteFile(inPath, []byte(lspHelperSource), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outPath, []byte(lspHelperSource), 0o644); err != nil {
		t.Fatal(err)
	}

	bridge := &bridgeStub{}
	router := NewRouter(bridge)
	router.session.setEnc(common.UTF8)
	router.session.ensureWorkspaces([]string{string(docpath.URIFromPath(root))})

	params, err := json.Marshal(map[string]any{
		"textDocument": map[string]any{
			"uri": string(docpath.URIFromPath(outPath)),
		},
		"text": lspHelperSource,
	})
	if err != nil {
		t.Fatal(err)
	}
	router.Notification(Client, Request{
		Method: string(didSave),
		Params: params,
	})

	for _, notification := range bridge.notifications {
		if notification.role == Gopls && notification.req.Method == string(didSave) {
			t.Fatalf("outside-workspace didSave was forwarded to gopls: %#v", notification)
		}
	}
	if len(bridge.notifications) != 2 {
		t.Fatalf("notifications = %d, want log + show error", len(bridge.notifications))
	}
	assertNotifPayload(t, bridge.notifications[0], Client, logMessage, "Error \"response\" to notification: [method=textDocument/didSave, msg=This file is not part of the current workspace., error=JSON RPC unknown error]", 1)
	assertNotifPayload(t, bridge.notifications[1], Client, showMessage, "This file is not part of the current workspace.", 1)
}

func TestDidSaveAcceptsSourceWhenWorkspaceURIHasTrailingSlash(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "view.gox")
	if err := os.WriteFile(path, []byte(lspHelperSource), 0o644); err != nil {
		t.Fatal(err)
	}

	bridge := &bridgeStub{}
	router := NewRouter(bridge)
	router.session.setEnc(common.UTF8)
	router.session.ensureWorkspaces([]string{string(docpath.URIFromPath(root)) + "/"})

	params, err := json.Marshal(map[string]any{
		"textDocument": map[string]any{
			"uri": string(docpath.URIFromPath(path)),
		},
		"text": lspHelperSource,
	})
	if err != nil {
		t.Fatal(err)
	}
	router.Notification(Client, Request{
		Method: string(didSave),
		Params: params,
	})

	if len(bridge.notifications) != 1 {
		t.Fatalf("notifications = %d, want forwarded didSave only: %#v", len(bridge.notifications), bridge.notifications)
	}
	got := bridge.notifications[0]
	if got.role != Gopls || got.req.Method != string(didSave) {
		t.Fatalf("didSave was not forwarded to gopls: %#v", got)
	}
	if string(got.req.Params) == string(params) {
		t.Fatalf("didSave params were not mapped through the generated target: %s", got.req.Params)
	}
	if !strings.Contains(string(got.req.Params), ".x.go") {
		t.Fatalf("didSave params do not target generated file: %s", got.req.Params)
	}
}

func TestDidSaveSurvivesWorkspaceURIChangingToTrailingSlash(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "view.gox")
	if err := os.WriteFile(path, []byte(lspHelperSource), 0o644); err != nil {
		t.Fatal(err)
	}

	bridge := &bridgeStub{}
	router := NewRouter(bridge)
	router.session.setEnc(common.UTF8)
	router.session.ensureWorkspaces([]string{string(docpath.URIFromPath(root))})

	params, err := json.Marshal(map[string]any{
		"textDocument": map[string]any{
			"uri": string(docpath.URIFromPath(path)),
		},
		"text": lspHelperSource,
	})
	if err != nil {
		t.Fatal(err)
	}
	router.Notification(Client, Request{
		Method: string(didSave),
		Params: params,
	})
	if len(bridge.notifications) != 1 || bridge.notifications[0].role != Gopls {
		t.Fatalf("initial didSave did not forward to gopls: %#v", bridge.notifications)
	}

	bridge.notifications = nil
	router.session.ensureWorkspaces([]string{string(docpath.URIFromPath(root)) + "/"})
	router.Notification(Client, Request{
		Method: string(didSave),
		Params: params,
	})
	if len(bridge.notifications) != 1 || bridge.notifications[0].role != Gopls {
		t.Fatalf("didSave after workspace URI mutation did not forward to gopls: %#v", bridge.notifications)
	}
}

func TestDidSaveSurvivesWorkspaceFoldersRefreshWithTrailingSlash(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "view.gox")
	if err := os.WriteFile(path, []byte(lspHelperSource), 0o644); err != nil {
		t.Fatal(err)
	}

	bridge := &asyncBridgeStub{}
	router := NewRouter(bridge)
	router.session.setEnc(common.UTF8)
	router.session.ensureWorkspaces([]string{string(docpath.URIFromPath(root))})

	bridge.callResp = Response{Result: json.RawMessage(`[{"uri":` + strconv.Quote(string(docpath.URIFromPath(root))+"/") + `,"name":"root"}]`)}
	done := make(chan Response, 1)
	router.Call(Gopls, Request{
		Method: string(workspaceFolders),
		Params: json.RawMessage(`{}`),
	}, func(r Response) {
		done <- r
	})
	select {
	case r := <-done:
		if r.Err != nil {
			t.Fatalf("workspaceFolders response error: %v", r.Err)
		}
	case <-time.After(time.Second):
		t.Fatal("workspaceFolders callback was not invoked")
	}

	params, err := json.Marshal(map[string]any{
		"textDocument": map[string]any{
			"uri": string(docpath.URIFromPath(path)),
		},
		"text": lspHelperSource,
	})
	if err != nil {
		t.Fatal(err)
	}

	bridge.notifications = nil
	router.Notification(Client, Request{
		Method: string(didSave),
		Params: params,
	})
	if len(bridge.notifications) != 1 || bridge.notifications[0].role != Gopls {
		t.Fatalf("didSave after workspaceFolders refresh did not forward to gopls: %#v", bridge.notifications)
	}
}

func assertNotifPayload(t *testing.T, got recordedNotify, wantRole Role, wantMethod method, wantMessage string, wantType int) {
	t.Helper()
	if got.role != wantRole {
		t.Fatalf("notification role = %v, want %v", got.role, wantRole)
	}
	if got.req.Method != string(wantMethod) {
		t.Fatalf("notification method = %q, want %q", got.req.Method, wantMethod)
	}
	var payload struct {
		Message string `json:"message"`
		Type    int    `json:"type"`
	}
	if err := json.Unmarshal(got.req.Params, &payload); err != nil {
		t.Fatalf("Unmarshal(%s): %v", got.req.Params, err)
	}
	if payload.Message != wantMessage {
		t.Fatalf("payload message = %q, want %q", payload.Message, wantMessage)
	}
	if payload.Type != wantType {
		t.Fatalf("payload type = %d, want %d", payload.Type, wantType)
	}
}
