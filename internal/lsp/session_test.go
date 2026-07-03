package lsp

import (
	"encoding/json"
	"errors"
	"fmt"
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

	t.Cleanup(sess.man().StopAll)
	sess.man().Lock()
	sess.ensureWorkspaces([]string{dirURI})
	ensuredDoc, ensuredKind := sess.man().Doc(fileURI)
	sess.removeWorkspace(dirURI)
	removedDoc, removedKind := sess.man().Doc(fileURI)
	sess.addWorkspace(dirURI)
	addedDoc, addedKind := sess.man().Doc(fileURI)
	sess.man().Unlock()
	if ensuredDoc == nil || ensuredKind != workspace.KindSource {
		t.Fatalf("Doc() after ensureWorkspaces = (%v, %v), want loaded source doc", ensuredDoc, ensuredKind)
	}
	if removedDoc != nil || removedKind != workspace.KindSource {
		t.Fatalf("Doc() after removeWorkspace = (%v, %v), want nil source doc", removedDoc, removedKind)
	}
	if addedDoc == nil || addedKind != workspace.KindSource {
		t.Fatalf("Doc() after addWorkspace = (%v, %v), want loaded source doc", addedDoc, addedKind)
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
	t.Cleanup(router.Stop)
	router.session.setEnc(common.UTF8)
	ensureRouterWorkspaces(router, string(docpath.URIFromPath(root)))

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
	t.Cleanup(router.Stop)
	router.session.setEnc(common.UTF8)
	ensureRouterWorkspaces(router, string(docpath.URIFromPath(root))+"/")

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
	t.Cleanup(router.Stop)
	router.session.setEnc(common.UTF8)
	ensureRouterWorkspaces(router, string(docpath.URIFromPath(root)))

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
	ensureRouterWorkspaces(router, string(docpath.URIFromPath(root))+"/")
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
	t.Cleanup(router.Stop)
	router.session.setEnc(common.UTF8)
	ensureRouterWorkspaces(router, string(docpath.URIFromPath(root)))

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

func TestApplyEditConvertsChangesMap(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "view.gox")
	if err := os.WriteFile(path, []byte(lspHelperSource), 0o644); err != nil {
		t.Fatal(err)
	}

	bridge := &asyncBridgeStub{callResp: Response{Result: json.RawMessage(`{"applied":true}`)}}
	router := NewRouter(bridge)
	router.session.setEnc(common.UTF8)
	router.session.ensureWorkspaces([]string{string(docpath.URIFromPath(root))})

	fileURI := string(docpath.URIFromPath(path))
	doc, kind := router.session.man().Doc(fileURI)
	if doc == nil || kind != workspace.KindSource {
		t.Fatalf("Doc(%s) = (%v, %v), want source doc", fileURI, doc, kind)
	}
	firstSrc := sourceRangeFor(t, path, "Card struct")
	secondSrc := sourceRangeForLast(t, path, "name")
	firstTarget := mustTargetRange(t, doc, firstSrc)
	secondTarget := mustTargetRange(t, doc, secondSrc)

	params := []byte(fmt.Sprintf(`{
		"edit":{
			"changes":{
				%q:[
					{"range":%s,"newText":"one"},
					{"range":%s,"newText":"two"}
				]
			}
		}
	}`, fileURI, jsonFromNode(t, jsonPos.fromRange(firstSrc)), jsonFromNode(t, jsonPos.fromRange(secondSrc))))

	done := make(chan Response, 1)
	router.Call(Gopls, Request{Method: string(applyEdit), Params: params}, func(r Response) {
		done <- r
	})
	var resp Response
	select {
	case resp = <-done:
	case <-time.After(time.Second):
		t.Fatal("applyEdit callback was not invoked")
	}
	if resp.Err != nil {
		t.Fatalf("applyEdit response error: %v", resp.Err)
	}
	if len(bridge.calls) != 1 || bridge.calls[0].role != Client || bridge.calls[0].req.Method != string(applyEdit) {
		t.Fatalf("proxied calls = %#v", bridge.calls)
	}
	proxied := mustJSONNode(t, string(bridge.calls[0].req.Params))
	edits := proxied.Get("edit").Get("changes").Get(doc.TargetFile().URI())
	if !edits.Exists() {
		t.Fatalf("changes are not keyed by the target uri: %s", bridge.calls[0].req.Params)
	}
	if arr, err := edits.ArrayUseNode(); err != nil || len(arr) != 2 {
		t.Fatalf("edits = %#v, err = %v, want 2 edits", arr, err)
	}
	gotFirst, err := jsonPos.intoRange(mustArrayIndex(t, edits, 0).Get("range"))
	if err != nil || gotFirst != firstTarget {
		t.Fatalf("edits[0].range = %v, err = %v, want %v", gotFirst, err, firstTarget)
	}
	gotSecond, err := jsonPos.intoRange(mustArrayIndex(t, edits, 1).Get("range"))
	if err != nil || gotSecond != secondTarget {
		t.Fatalf("edits[1].range = %v, err = %v, want %v", gotSecond, err, secondTarget)
	}
}

func TestWorkspaceFoldersForwardsAbsentParams(t *testing.T) {
	root := t.TempDir()
	bridge := &asyncBridgeStub{}
	router := NewRouter(bridge)
	router.session.setEnc(common.UTF8)
	bridge.callResp = Response{Result: json.RawMessage(`[{"uri":` + strconv.Quote(string(docpath.URIFromPath(root))) + `,"name":"root"}]`)}

	done := make(chan Response, 1)
	router.Call(Gopls, Request{Method: string(workspaceFolders)}, func(r Response) {
		done <- r
	})
	var resp Response
	select {
	case resp = <-done:
	case <-time.After(time.Second):
		t.Fatal("workspaceFolders callback was not invoked")
	}
	if resp.Err != nil {
		t.Fatalf("workspaceFolders response error: %v", resp.Err)
	}
	if len(bridge.calls) != 1 || bridge.calls[0].role != Client || bridge.calls[0].req.Method != string(workspaceFolders) {
		t.Fatalf("forwarded calls = %#v", bridge.calls)
	}
	if string(bridge.calls[0].req.Params) != "null" {
		t.Fatalf("forwarded params = %q, want null", bridge.calls[0].req.Params)
	}
	if len(bridge.notifications) != 0 {
		t.Fatalf("unexpected notifications: %#v", bridge.notifications)
	}
	if !strings.Contains(string(resp.Result), "root") {
		t.Fatalf("response result = %s", resp.Result)
	}
}

func TestRouterStopStopsManager(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "view.gox")
	if err := os.WriteFile(path, []byte(lspHelperSource), 0o644); err != nil {
		t.Fatal(err)
	}

	router := NewRouter(&bridgeStub{})
	ensureRouterWorkspaces(router, string(docpath.URIFromPath(root)))
	man := router.session.man()
	man.Lock()
	doc, kind := man.Doc(string(docpath.URIFromPath(path)))
	roots := man.RootsLocked()
	man.Unlock()
	if doc == nil || kind != workspace.KindSource || len(roots) != 1 {
		t.Fatalf("before Stop: doc = %v, kind = %v, roots = %v", doc, kind, roots)
	}

	router.Stop()
	router.Stop()

	man.Lock()
	doc, kind = man.Doc(string(docpath.URIFromPath(path)))
	roots = man.RootsLocked()
	man.Unlock()
	if doc != nil || kind != workspace.KindSource || len(roots) != 0 {
		t.Fatalf("after Stop: doc = %v, kind = %v, roots = %v", doc, kind, roots)
	}
}

func ensureRouterWorkspaces(router Router, uris ...string) {
	router.session.man().Lock()
	defer router.session.man().Unlock()
	router.session.ensureWorkspaces(uris)
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

func TestSubtypesRejectsSourceOutsideWorkspace(t *testing.T) {
	outside := t.TempDir()
	path := filepath.Join(outside, "view.gox")
	if err := os.WriteFile(path, []byte(lspHelperSource), 0o644); err != nil {
		t.Fatal(err)
	}

	bridge := &bridgeStub{}
	router := NewRouter(bridge)
	router.session.setEnc(common.UTF8)

	params, err := json.Marshal(map[string]any{
		"item": map[string]any{
			"uri": string(docpath.URIFromPath(path)),
			"range": map[string]any{
				"start": map[string]any{"line": 0, "character": 0},
				"end":   map[string]any{"line": 0, "character": 1},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, m := range []method{subtypes, supertypes} {
		bridge.calls = nil
		handled := false
		var resp Response
		router.Call(Client, Request{
			Method: string(m),
			Params: params,
		}, func(r Response) {
			handled = true
			resp = r
		})
		if !handled {
			t.Fatalf("%s callback was not invoked", m)
		}
		if resp.Err == nil {
			t.Fatalf("%s error = nil, want out-of-workspace error", m)
		}
		if len(bridge.calls) != 0 {
			t.Fatalf("out-of-workspace %s was forwarded: %#v", m, bridge.calls)
		}
	}
}

func TestDidChangeRejectsPresentInvalidRange(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "view.gox")
	if err := os.WriteFile(path, []byte(lspHelperSource), 0o644); err != nil {
		t.Fatal(err)
	}

	bridge := &bridgeStub{}
	router := NewRouter(bridge)
	router.session.setEnc(common.UTF8)
	router.session.ensureWorkspaces([]string{string(docpath.URIFromPath(root))})

	uri := string(docpath.URIFromPath(path))
	doc, kind := router.session.man().Doc(uri)
	if kind != workspace.KindSource || doc == nil {
		t.Fatalf("Doc() = (%v, %v), want source doc", doc, kind)
	}
	targetBefore := doc.TargetContent()

	params, err := json.Marshal(map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"contentChanges": []map[string]any{
			{
				"range": map[string]any{
					"start": map[string]any{"line": -1, "character": 0},
					"end":   map[string]any{"line": 0, "character": 0},
				},
				"text": "junk fragment",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	router.Notification(Client, Request{Method: string(didChange), Params: params})

	for _, notification := range bridge.notifications {
		if notification.role == Gopls {
			t.Fatalf("invalid didChange was forwarded to gopls: %#v", notification)
		}
	}
	if doc.Err() != nil {
		t.Fatalf("doc.Err() = %v, want nil", doc.Err())
	}
	if got := doc.TargetContent(); got != targetBefore {
		t.Fatalf("target content changed after rejected didChange: %q", got)
	}
	found := false
	for _, notification := range bridge.notifications {
		if notification.req.Method == string(logMessage) && strings.Contains(string(notification.req.Params), "The edit range is invalid.") {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing invalid-range error notification: %#v", bridge.notifications)
	}
}

func TestDidChangeAbsentRangeAppliesFullUpdate(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "view.gox")
	if err := os.WriteFile(path, []byte(lspHelperSource), 0o644); err != nil {
		t.Fatal(err)
	}

	bridge := &bridgeStub{}
	router := NewRouter(bridge)
	router.session.setEnc(common.UTF8)
	router.session.ensureWorkspaces([]string{string(docpath.URIFromPath(root))})

	uri := string(docpath.URIFromPath(path))
	doc, kind := router.session.man().Doc(uri)
	if kind != workspace.KindSource || doc == nil {
		t.Fatalf("Doc() = (%v, %v), want source doc", doc, kind)
	}

	newSrc := strings.Replace(lspHelperSource, "card", "panel", 1)
	params, err := json.Marshal(map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"contentChanges": []map[string]any{
			{"text": newSrc},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	router.Notification(Client, Request{Method: string(didChange), Params: params})

	if doc.Err() != nil {
		t.Fatalf("doc.Err() = %v, want nil", doc.Err())
	}
	if !strings.Contains(doc.TargetContent(), "panel") {
		t.Fatalf("target content missing full update: %q", doc.TargetContent())
	}
	forwarded := false
	for _, notification := range bridge.notifications {
		if notification.role == Gopls && notification.req.Method == string(didChange) {
			forwarded = true
		}
	}
	if !forwarded {
		t.Fatalf("full-document didChange was not forwarded to gopls: %#v", bridge.notifications)
	}
}
