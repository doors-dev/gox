package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/doors-dev/gox/internal/common"
	"github.com/doors-dev/gox/internal/docpath"
)

const strayCloseLspSource = `package demo

import "github.com/doors-dev/gox"

elem View() {
	<div class="card">hi</my-crad></div>
}
`

func strayCloseWorkspace(t *testing.T, bridge Bridge) (Router, string) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "view.gox")
	if err := os.WriteFile(path, []byte(strayCloseLspSource), 0o644); err != nil {
		t.Fatal(err)
	}
	router := NewRouter(bridge)
	t.Cleanup(router.Stop)
	router.session.setEnc(common.UTF8)
	ensureRouterWorkspaces(router, string(docpath.URIFromPath(root)))
	return router, path
}

func assertStrayCloseWarning(t *testing.T, items Json, path string) {
	t.Helper()
	arr, err := items.ArrayUseNode()
	if err != nil {
		t.Fatalf("ArrayUseNode() error = %v", err)
	}
	if len(arr) != 1 {
		t.Fatalf("diagnostics = %d, want the stray close tag warning", len(arr))
	}
	warning := arr[0]
	if got := mustString(t, warning.Get("message")); got != "`</my-crad>` does not match an open element and is ignored." {
		t.Fatalf("warning message = %q", got)
	}
	if got := mustInt64(t, warning.Get("severity")); got != 2 {
		t.Fatalf("warning severity = %d, want 2", got)
	}
	if got := mustString(t, warning.Get("source")); got != "gox" {
		t.Fatalf("warning source = %q, want gox", got)
	}
	got, err := jsonPos.intoRange(warning.Get("range"))
	if want := sourceRangeFor(t, path, "</my-crad>"); err != nil || got != want {
		t.Fatalf("warning range = %v, err = %v, want %v", got, err, want)
	}
}

func TestPublishDiagnosticsIncludesStrayCloseTags(t *testing.T) {
	bridge := &bridgeStub{}
	router, path := strayCloseWorkspace(t, bridge)
	doc, _ := router.session.man().Doc(string(docpath.URIFromPath(path)))
	params, err := json.Marshal(map[string]any{"uri": doc.TargetFile().URI(), "diagnostics": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	router.Notification(Gopls, Request{Method: string(publishDiagnostics), Params: params})

	if len(bridge.notifications) != 1 || bridge.notifications[0].req.Method != string(publishDiagnostics) {
		t.Fatalf("notifications = %#v, want single publishDiagnostics", bridge.notifications)
	}
	payload := mustJSONNode(t, string(bridge.notifications[0].req.Params))
	assertStrayCloseWarning(t, payload.Get("diagnostics"), path)
}

func TestDiagnosticPullIncludesStrayCloseTags(t *testing.T) {
	bridge := &asyncBridgeStub{callResp: Response{Result: json.RawMessage(`{"kind":"full","items":[]}`)}}
	router, path := strayCloseWorkspace(t, bridge)
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
	assertStrayCloseWarning(t, mustJSONNode(t, string(resp.Result)).Get("items"), path)
}
