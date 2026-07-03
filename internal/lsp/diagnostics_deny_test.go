package lsp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/doors-dev/gox/internal/common"
	"github.com/doors-dev/gox/internal/docpath"
	"github.com/doors-dev/gox/internal/workspace"
)

func deniedAndKeptDiagnostics(rangeJSON string) string {
	return fmt.Sprintf(`[
		{"range":%s,"severity":3,"code":"QF1003","source":"staticcheck","message":"could use tagged switch on state.action"},
		{"range":%s,"severity":3,"message":"could use tagged switch on state.mode"},
		{"range":%s,"severity":2,"code":1003,"source":"gopls","message":"unused variable x"},
		{"range":%s,"severity":2,"code":"SA4006","source":"staticcheck","message":"this value is never used"}
	]`, rangeJSON, rangeJSON, rangeJSON, rangeJSON)
}

func assertKeptDiagnostics(t *testing.T, items Json, wantRange common.Range) {
	t.Helper()
	arr, err := items.ArrayUseNode()
	if err != nil {
		t.Fatalf("ArrayUseNode() error = %v", err)
	}
	if len(arr) != 2 {
		t.Fatalf("diagnostics kept = %d, want 2", len(arr))
	}
	wantMessages := []string{"unused variable x", "this value is never used"}
	for i, want := range wantMessages {
		if got := mustString(t, arr[i].Get("message")); got != want {
			t.Fatalf("diagnostics[%d].message = %q, want %q", i, got, want)
		}
		gotRange, err := jsonPos.intoRange(arr[i].Get("range"))
		if err != nil || gotRange != wantRange {
			t.Fatalf("diagnostics[%d].range = %v, err = %v, want %v", i, gotRange, err, wantRange)
		}
	}
}

func TestPublishDiagnosticsDropsDeniedOnSourceConversion(t *testing.T) {
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

	doc, kind := router.session.man().Doc(string(docpath.URIFromPath(path)))
	if doc == nil || kind != workspace.KindSource {
		t.Fatalf("Doc() = (%v, %v), want source doc", doc, kind)
	}
	srcRange := sourceRangeForLast(t, path, "name")
	targetRange := mustTargetRange(t, doc, srcRange)
	rangeJSON := jsonFromNode(t, jsonPos.fromRange(targetRange))

	params := []byte(fmt.Sprintf(`{"uri":%q,"diagnostics":%s}`, doc.TargetFile().URI(), deniedAndKeptDiagnostics(rangeJSON)))
	router.Notification(Gopls, Request{Method: string(publishDiagnostics), Params: params})

	if len(bridge.notifications) != 1 {
		t.Fatalf("notifications = %#v, want single publishDiagnostics", bridge.notifications)
	}
	got := bridge.notifications[0]
	if got.role != Client || got.req.Method != string(publishDiagnostics) {
		t.Fatalf("notification = %#v", got)
	}
	payload := mustJSONNode(t, string(got.req.Params))
	if uri := mustString(t, payload.Get("uri")); uri != doc.SourceFile().URI() {
		t.Fatalf("uri = %q, want %q", uri, doc.SourceFile().URI())
	}
	assertKeptDiagnostics(t, payload.Get("diagnostics"), srcRange)
}

func TestPublishDiagnosticsPlainGoForwardedUntouched(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "main.go")
	if err := os.WriteFile(path, []byte("package demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bridge := &bridgeStub{}
	router := NewRouter(bridge)
	t.Cleanup(router.Stop)
	router.session.setEnc(common.UTF8)
	ensureRouterWorkspaces(router, string(docpath.URIFromPath(root)))

	rangeJSON := `{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}`
	params := []byte(fmt.Sprintf(`{"uri":%q,"diagnostics":%s}`, string(docpath.URIFromPath(path)), deniedAndKeptDiagnostics(rangeJSON)))
	router.Notification(Gopls, Request{Method: string(publishDiagnostics), Params: params})

	if len(bridge.notifications) != 1 {
		t.Fatalf("notifications = %#v, want single forwarded publishDiagnostics", bridge.notifications)
	}
	got := bridge.notifications[0]
	if got.role != Client || got.req.Method != string(publishDiagnostics) {
		t.Fatalf("notification = %#v", got)
	}
	if string(got.req.Params) != string(params) {
		t.Fatalf("forwarded params were modified:\n got: %s\nwant: %s", got.req.Params, params)
	}
}

func TestDiagnosticPullDropsDeniedOnSourceConversion(t *testing.T) {
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

	doc, kind := router.session.man().Doc(string(docpath.URIFromPath(path)))
	if doc == nil || kind != workspace.KindSource {
		t.Fatalf("Doc() = (%v, %v), want source doc", doc, kind)
	}
	srcRange := sourceRangeForLast(t, path, "name")
	targetRange := mustTargetRange(t, doc, srcRange)
	rangeJSON := jsonFromNode(t, jsonPos.fromRange(targetRange))

	bridge.callResp = Response{Result: json.RawMessage(fmt.Sprintf(`{
		"kind":"full",
		"items":%s,
		"relatedDocuments":{%q:{"kind":"full","items":%s}}
	}`, deniedAndKeptDiagnostics(rangeJSON), doc.TargetFile().URI(), deniedAndKeptDiagnostics(rangeJSON)))}

	params, err := json.Marshal(map[string]any{
		"textDocument": map[string]any{"uri": string(docpath.URIFromPath(path))},
	})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan Response, 1)
	router.Call(Client, Request{Method: string(diagnostic), Params: params}, func(r Response) {
		done <- r
	})
	var resp Response
	select {
	case resp = <-done:
	case <-time.After(time.Second):
		t.Fatal("diagnostic callback was not invoked")
	}
	if resp.Err != nil {
		t.Fatalf("diagnostic response error: %v", resp.Err)
	}
	if len(bridge.calls) != 1 || bridge.calls[0].role != Gopls || bridge.calls[0].req.Method != string(diagnostic) {
		t.Fatalf("proxied calls = %#v", bridge.calls)
	}
	result := mustJSONNode(t, string(resp.Result))
	assertKeptDiagnostics(t, result.Get("items"), srcRange)
	related := result.Get("relatedDocuments").Get(doc.SourceFile().URI())
	if !related.Exists() {
		t.Fatalf("relatedDocuments missing source uri: %s", resp.Result)
	}
	assertKeptDiagnostics(t, related.Get("items"), srcRange)
}
