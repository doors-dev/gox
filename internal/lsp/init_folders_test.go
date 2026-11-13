package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/doors-dev/gox/internal/docpath"
	"github.com/doors-dev/gox/internal/workspace"
)

func TestGetWorkspaceDirsFallsBackToRoot(t *testing.T) {
	root := t.TempDir()
	rootURI := string(docpath.URIFromPath(root))
	cases := []struct {
		name   string
		params map[string]any
		want   []string
	}{
		{"folders win over root", map[string]any{"workspaceFolders": []any{map[string]any{"uri": "file:///a"}}, "rootUri": rootURI}, []string{"file:///a"}},
		{"only rootUri", map[string]any{"rootUri": rootURI}, []string{rootURI}},
		{"null folders with rootUri", map[string]any{"workspaceFolders": nil, "rootUri": rootURI, "rootPath": "/elsewhere"}, []string{rootURI}},
		{"empty folders with rootUri", map[string]any{"workspaceFolders": []any{}, "rootUri": rootURI}, []string{rootURI}},
		{"null folders and rootUri with rootPath", map[string]any{"workspaceFolders": nil, "rootUri": nil, "rootPath": root}, []string{rootURI}},
		{"only rootPath", map[string]any{"rootPath": root}, []string{rootURI}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := jsonInit.getWorkspaceDirs(mustJSONNode(t, string(mustMarshal(t, tc.params))))
			if err != nil || !slices.Equal(got, tc.want) {
				t.Fatalf("getWorkspaceDirs() = (%q, %v), want %q", got, err, tc.want)
			}
		})
	}
	for _, src := range []string{
		`{}`,
		`{"workspaceFolders":null}`,
		`{"workspaceFolders":[],"rootUri":null,"rootPath":null}`,
		`{"rootUri":"","rootPath":""}`,
	} {
		if got, err := jsonInit.getWorkspaceDirs(mustJSONNode(t, src)); err == nil {
			t.Fatalf("getWorkspaceDirs(%s) = %q, want an error", src, got)
		}
	}
	if _, err := jsonInit.getWorkspaceDirs(mustJSONNode(t, `{"workspaceFolders":{},"rootUri":"file:///a"}`)); err == nil {
		t.Fatal("getWorkspaceDirs(invalid folders) error = nil")
	}
}

func TestInitializeRootURIServesFolderAcrossNullWorkspaceFolders(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "view.gox")
	if err := os.WriteFile(path, []byte(lspHelperSource), 0o644); err != nil {
		t.Fatal(err)
	}
	bridge := &asyncBridgeStub{callResp: Response{Result: json.RawMessage(`{"capabilities":{"completionProvider":{"triggerCharacters":["."]}}}`)}}
	router := NewRouter(bridge)
	t.Cleanup(router.Stop)

	call := func(role Role, m method, params any) Response {
		t.Helper()
		done := make(chan Response, 1)
		router.Call(role, Request{Method: string(m), Params: mustMarshal(t, params)}, func(r Response) {
			done <- r
		})
		select {
		case r := <-done:
			return r
		case <-time.After(time.Second):
			t.Fatalf("%s callback was not invoked", m)
			return Response{}
		}
	}
	assertServed := func(stage string) {
		t.Helper()
		router.session.man().Lock()
		defer router.session.man().Unlock()
		if doc, kind := router.session.man().Doc(string(docpath.URIFromPath(path))); doc == nil || kind != workspace.KindSource {
			t.Fatalf("%s: Doc() = (%v, %v), want source doc", stage, doc, kind)
		}
	}

	if resp := call(Client, initialize, map[string]any{"rootUri": string(docpath.URIFromPath(root)), "capabilities": map[string]any{}}); resp.Err != nil {
		t.Fatalf("initialize error: %v", resp.Err)
	}
	assertServed("after initialize")

	bridge.callResp = Response{Result: json.RawMessage(`null`)}
	resp := call(Gopls, workspaceFolders, nil)
	if resp.Err != nil || string(resp.Result) != "null" {
		t.Fatalf("workspaceFolders response = (%s, %v), want null passed through", resp.Result, resp.Err)
	}
	assertServed("after a null workspaceFolders result")
	if len(bridge.notifications) != 0 {
		t.Fatalf("unexpected notifications: %#v", bridge.notifications)
	}
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
