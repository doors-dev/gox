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

const spaceHintLspSource = `package demo

import "github.com/doors-dev/gox"

elem View(name string) {
	<div class="card">hi ~(name)</div>
}
`

func preservedSpaceRange(t *testing.T, path string) common.Range {
	t.Helper()
	textRange := sourceRangeFor(t, path, "hi ")
	end := textRange.End()
	return common.NewRange(common.NewPos(end.Line(), end.Column()-1), end)
}

func assertGoplsDiagWithSpaceHint(t *testing.T, items Json, goplsRange common.Range, hintRange common.Range) {
	t.Helper()
	arr, err := items.ArrayUseNode()
	if err != nil {
		t.Fatalf("ArrayUseNode() error = %v", err)
	}
	if len(arr) != 2 {
		t.Fatalf("diagnostics = %d, want gopls diagnostic plus space hint", len(arr))
	}
	if got := mustString(t, arr[0].Get("message")); got != "unused variable x" {
		t.Fatalf("diagnostics[0].message = %q, want gopls diagnostic first", got)
	}
	gotGoplsRange, err := jsonPos.intoRange(arr[0].Get("range"))
	if err != nil || gotGoplsRange != goplsRange {
		t.Fatalf("diagnostics[0].range = %v, err = %v, want %v", gotGoplsRange, err, goplsRange)
	}
	hint := arr[1]
	if got := mustString(t, hint.Get("message")); got != "This space will be preserved in the output." {
		t.Fatalf("hint message = %q", got)
	}
	if got := mustInt64(t, hint.Get("severity")); got != 4 {
		t.Fatalf("hint severity = %d, want 4", got)
	}
	if got := mustString(t, hint.Get("source")); got != "gox" {
		t.Fatalf("hint source = %q, want gox", got)
	}
	gotHintRange, err := jsonPos.intoRange(hint.Get("range"))
	if err != nil || gotHintRange != hintRange {
		t.Fatalf("hint range = %v, err = %v, want %v", gotHintRange, err, hintRange)
	}
}

func TestPublishDiagnosticsIncludesSpaceHints(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "view.gox")
	if err := os.WriteFile(path, []byte(spaceHintLspSource), 0o644); err != nil {
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

	params := []byte(fmt.Sprintf(
		`{"uri":%q,"diagnostics":[{"range":%s,"severity":2,"code":1003,"source":"gopls","message":"unused variable x"}]}`,
		doc.TargetFile().URI(), rangeJSON,
	))
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
	assertGoplsDiagWithSpaceHint(t, payload.Get("diagnostics"), srcRange, preservedSpaceRange(t, path))
}

func TestDiagnosticPullIncludesSpaceHints(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "view.gox")
	if err := os.WriteFile(path, []byte(spaceHintLspSource), 0o644); err != nil {
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

	bridge.callResp = Response{Result: json.RawMessage(fmt.Sprintf(
		`{"kind":"full","items":[{"range":%s,"severity":2,"code":1003,"source":"gopls","message":"unused variable x"}]}`,
		rangeJSON,
	))}

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
	assertGoplsDiagWithSpaceHint(t, result.Get("items"), srcRange, preservedSpaceRange(t, path))
}
