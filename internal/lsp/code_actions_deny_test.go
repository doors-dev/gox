package lsp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/doors-dev/gox/internal/common"
	"github.com/doors-dev/gox/internal/docpath"
	"github.com/doors-dev/gox/internal/workspace"
)

func deniedAndKeptCodeActions(editURI string, editRange string) string {
	return fmt.Sprintf(`[
		{"title":"Declare missing methods of demo.Renderer","kind":"quickfix","edit":{"changes":{%q:[{"range":%s,"newText":"stub"}]}}},
		{"title":"Declare missing method Card.Render","kind":"quickfix","edit":{"changes":{%q:[{"range":%s,"newText":"stub"}]}}},
		{"title":"Add struct tags","kind":"refactor.rewrite.addTags","command":{"title":"Add struct tags","command":"gopls.modify_tags","arguments":[]}},
		{"title":"Remove struct tags","kind":"refactor.rewrite.removeTags","command":{"title":"Remove struct tags","command":"gopls.modify_tags","arguments":[]}},
		{"title":"Fill in struct fields","kind":"quickfix","edit":{"changes":{%q:[{"range":%s,"newText":"filled"}]}}}
	]`, editURI, editRange, editURI, editRange, editURI, editRange)
}

func newCodeActionRouter(t *testing.T, fileName string, source string) (Router, *asyncBridgeStub, string) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, fileName)
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	bridge := &asyncBridgeStub{}
	router := NewRouter(bridge)
	t.Cleanup(router.Stop)
	router.session.setEnc(common.UTF8)
	ensureRouterWorkspaces(router, string(docpath.URIFromPath(root)))
	return router, bridge, path
}

func mustSourceDoc(t *testing.T, router Router, path string) workspace.Doc {
	t.Helper()
	doc, kind := router.session.man().Doc(string(docpath.URIFromPath(path)))
	if doc == nil || kind != workspace.KindSource {
		t.Fatalf("Doc() = (%v, %v), want source doc", doc, kind)
	}
	return doc
}

func unmappableTargetRange(t *testing.T, doc workspace.Doc) common.Range {
	t.Helper()
	lines := strings.Split(doc.TargetContent(), "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		ran := common.NewRange(common.NewPos(i, 0), common.NewPos(i, 1))
		if _, ok := doc.SourceRange(common.UTF8, ran, workspace.Strict); !ok {
			return ran
		}
	}
	t.Fatal("no unmappable target range found")
	return common.NoRange()
}

func callRouter(t *testing.T, router Router, m method, params []byte) Response {
	t.Helper()
	done := make(chan Response, 1)
	router.Call(Client, Request{Method: string(m), Params: params}, func(r Response) {
		done <- r
	})
	select {
	case resp := <-done:
		return resp
	case <-time.After(time.Second):
		t.Fatalf("%s callback was not invoked", m)
		return Response{}
	}
}

func codeActionParams(t *testing.T, uri string, rangeJSON string) []byte {
	t.Helper()
	return []byte(fmt.Sprintf(`{
		"textDocument":{"uri":%q},
		"range":%s,
		"context":{"only":["quickfix","source.organizeImports"],"diagnostics":[]}
	}`, uri, rangeJSON))
}

func mustActionArray(t *testing.T, resp Response, want int) []Json {
	t.Helper()
	if resp.Err != nil {
		t.Fatalf("codeAction response error: %v", resp.Err)
	}
	result := mustJSONNode(t, string(resp.Result))
	arr, err := result.ArrayUseNode()
	if err != nil {
		t.Fatalf("ArrayUseNode() error = %v", err)
	}
	if len(arr) != want {
		t.Fatalf("actions kept = %d, want %d: %s", len(arr), want, resp.Result)
	}
	nodes := make([]Json, 0, len(arr))
	for i := range arr {
		nodes = append(nodes, &arr[i])
	}
	return nodes
}

func TestCodeActionsDeniedFamiliesFilteredOnSource(t *testing.T) {
	router, bridge, path := newCodeActionRouter(t, "view.gox", lspHelperSource)
	doc := mustSourceDoc(t, router, path)
	srcRange := sourceRangeForLast(t, path, "name")
	targetRange := mustTargetRange(t, doc, srcRange)
	bridge.callResp = Response{Result: json.RawMessage(deniedAndKeptCodeActions(
		doc.TargetFile().URI(),
		jsonFromNode(t, jsonPos.fromRange(targetRange)),
	))}

	resp := callRouter(t, router, codeAction, codeActionParams(t, doc.SourceFile().URI(), jsonFromNode(t, jsonPos.fromRange(srcRange))))

	if len(bridge.calls) != 1 || bridge.calls[0].role != Gopls || bridge.calls[0].req.Method != string(codeAction) {
		t.Fatalf("proxied calls = %#v", bridge.calls)
	}
	proxied := mustJSONNode(t, string(bridge.calls[0].req.Params))
	if got := mustString(t, proxied.Get("textDocument").Get("uri")); got != doc.TargetFile().URI() {
		t.Fatalf("proxied uri = %q, want %q", got, doc.TargetFile().URI())
	}
	actions := mustActionArray(t, resp, 1)
	if got := mustString(t, actions[0].Get("title")); got != "Fill in struct fields" {
		t.Fatalf("kept action title = %q, want Fill in struct fields", got)
	}
	edits := actions[0].Get("edit").Get("changes").Get(doc.SourceFile().URI())
	if !edits.Exists() {
		t.Fatalf("kept action edit is not keyed by the source uri: %s", resp.Result)
	}
	gotRange, err := jsonPos.intoRange(mustArrayIndex(t, edits, 0).Get("range"))
	if err != nil || gotRange != srcRange {
		t.Fatalf("kept action edit range = %v, err = %v, want %v", gotRange, err, srcRange)
	}
}

func TestCodeActionsPlainGoPassThrough(t *testing.T) {
	router, bridge, path := newCodeActionRouter(t, "main.go", "package demo\n")
	uri := string(docpath.URIFromPath(path))
	rangeJSON := `{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}`
	bridge.callResp = Response{Result: json.RawMessage(deniedAndKeptCodeActions(uri, rangeJSON))}

	resp := callRouter(t, router, codeAction, codeActionParams(t, uri, rangeJSON))

	if len(bridge.calls) != 1 || bridge.calls[0].role != Gopls || bridge.calls[0].req.Method != string(codeAction) {
		t.Fatalf("proxied calls = %#v", bridge.calls)
	}
	proxied := mustJSONNode(t, string(bridge.calls[0].req.Params))
	if got := mustString(t, proxied.Get("textDocument").Get("uri")); got != uri {
		t.Fatalf("proxied uri = %q, want %q", got, uri)
	}
	actions := mustActionArray(t, resp, 5)
	wantTitles := []string{
		"Declare missing methods of demo.Renderer",
		"Declare missing method Card.Render",
		"Add struct tags",
		"Remove struct tags",
		"Fill in struct fields",
	}
	for i, want := range wantTitles {
		if got := mustString(t, actions[i].Get("title")); got != want {
			t.Fatalf("actions[%d].title = %q, want %q", i, got, want)
		}
	}
	edits := actions[0].Get("edit").Get("changes").Get(uri)
	if !edits.Exists() {
		t.Fatalf("plain go edit is not keyed by the original uri: %s", resp.Result)
	}
	if got := jsonFromNode(t, *mustArrayIndex(t, edits, 0).Get("range")); got != rangeJSON {
		t.Fatalf("plain go edit range = %s, want %s", got, rangeJSON)
	}
}

func TestCodeActionsSourceUnmappableEditBehaviorUnchanged(t *testing.T) {
	router, bridge, path := newCodeActionRouter(t, "view.gox", lspHelperSource)
	doc := mustSourceDoc(t, router, path)
	srcRange := sourceRangeForLast(t, path, "name")
	targetRange := mustTargetRange(t, doc, srcRange)
	badRange := unmappableTargetRange(t, doc)
	bridge.callResp = Response{Result: json.RawMessage(fmt.Sprintf(`[
		{"title":"Fill in struct fields","kind":"quickfix","edit":{"changes":{%q:[{"range":%s,"newText":"x"}]}}},
		{"title":"Organize Imports","kind":"source.organizeImports","edit":{"changes":{%q:[{"range":%s,"newText":"y"}]}}}
	]`,
		doc.TargetFile().URI(), jsonFromNode(t, jsonPos.fromRange(badRange)),
		doc.TargetFile().URI(), jsonFromNode(t, jsonPos.fromRange(targetRange)),
	))}

	resp := callRouter(t, router, codeAction, codeActionParams(t, doc.SourceFile().URI(), jsonFromNode(t, jsonPos.fromRange(srcRange))))

	actions := mustActionArray(t, resp, 1)
	if got := mustString(t, actions[0].Get("title")); got != "Organize Imports" {
		t.Fatalf("kept action title = %q, want Organize Imports", got)
	}
	gotRange, err := jsonPos.intoRange(mustArrayIndex(t, actions[0].Get("edit").Get("changes").Get(doc.SourceFile().URI()), 0).Get("range"))
	if err != nil || gotRange != srcRange {
		t.Fatalf("kept action edit range = %v, err = %v, want %v", gotRange, err, srcRange)
	}
}

func TestResolveCodeActionUnmappableEditFailsWholeResponse(t *testing.T) {
	router, bridge, path := newCodeActionRouter(t, "view.gox", lspHelperSource)
	doc := mustSourceDoc(t, router, path)
	badRange := unmappableTargetRange(t, doc)
	bridge.callResp = Response{Result: json.RawMessage(fmt.Sprintf(
		`{"title":"Fill in struct fields","kind":"quickfix","edit":{"changes":{%q:[{"range":%s,"newText":"x"}]}}}`,
		doc.TargetFile().URI(), jsonFromNode(t, jsonPos.fromRange(badRange)),
	))}

	params := []byte(`{"title":"Fill in struct fields","kind":"quickfix","data":{"id":1}}`)
	resp := callRouter(t, router, resolveCodeAction, params)

	if len(bridge.calls) != 1 || bridge.calls[0].role != Gopls || bridge.calls[0].req.Method != string(resolveCodeAction) {
		t.Fatalf("proxied calls = %#v", bridge.calls)
	}
	if resp.Err == nil {
		t.Fatalf("resolve response error = nil, want whole-response conversion error: %s", resp.Result)
	}
}
