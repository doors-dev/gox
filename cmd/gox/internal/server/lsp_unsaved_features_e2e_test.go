package server_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func unsavedInit(t *testing.T, root string) *lspHarness {
	t.Helper()
	folder := workspaceFolder{URI: uriForPath(root), Name: "main"}
	h := newLSPHarness(t, []workspaceFolder{folder})
	h.callRaw("initialize", map[string]any{
		"processId": nil,
		"rootUri":   folder.URI,
		"capabilities": map[string]any{
			"workspace": map[string]any{
				"applyEdit":        true,
				"workspaceFolders": true,
			},
			"textDocument": map[string]any{
				"publishDiagnostics": map[string]any{},
				"rename":             map[string]any{"prepareSupport": true},
			},
			"window": map[string]any{},
		},
		"workspaceFolders": []workspaceFolder{folder},
	})
	h.notify("initialized", map[string]any{})
	return h
}

func unsavedWaitHover(t *testing.T, h *lspHarness, uri string, pos lspPosition, needle string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	var last callResult
	for time.Now().Before(deadline) {
		last = h.call("textDocument/hover", map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"position":     pos,
		})
		if last.Err == nil && strings.Contains(string(last.Result), needle) {
			return
		}
		time.Sleep(lspPollInterval)
	}
	t.Fatalf("hover on %s never returned %q: result=%s err=%v", uri, needle, compactJSON(last.Result), last.Err)
}

func TestLSPUnsavedAndKeptDocumentFeatures(t *testing.T) {
	root := filepath.Join(t.TempDir(), "main")
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/probe\n\ngo 1.25.1\n")
	keep := writeMarkedFile(t, filepath.Join(root, "pkg", "keep.gox"), `package pkg

func /*@keep_def*/Keep() string {
	return "keep"
}
`)
	keepTarget := filepath.Join(root, "pkg", "keep.x.go")
	draft := writeMarkedFile(t, filepath.Join(t.TempDir(), "fresh.gox"), `package pkg

func /*@fresh_def*/Fresh() string {
	return /*@keep_ref*/Keep()
}
`)
	freshPath := filepath.Join(root, "pkg", "fresh.gox")
	freshTarget := filepath.Join(root, "pkg", "fresh.x.go")
	freshText := readFile(t, draft.Path)

	h := unsavedInit(t, root)
	keepURI := h.openFile(keep.Path, "gox")
	waitForFileContents(t, keepTarget)
	freshURI := h.openFileWithText(freshPath, "gox", freshText)
	h.waitForEventAfter(0, "notification", "textDocument/publishDiagnostics", func(raw json.RawMessage) bool {
		var params publishDiagnosticsParams
		return json.Unmarshal(raw, &params) == nil && params.URI == freshURI
	})

	unsavedWaitHover(t, h, freshURI, draft.Marker("fresh_def"), "Fresh")
	unsavedWaitHover(t, h, freshURI, draft.Marker("keep_ref"), "Keep")

	def := h.call("textDocument/definition", map[string]any{
		"textDocument": map[string]any{"uri": freshURI},
		"position":     draft.Marker("keep_ref"),
	})
	if def.Err != nil || !strings.Contains(string(def.Result), "keep.gox") {
		t.Errorf("definition from an unsaved document: result=%s err=%v", compactJSON(def.Result), def.Err)
	}
	refs := h.call("textDocument/references", map[string]any{
		"textDocument": map[string]any{"uri": keepURI},
		"position":     keep.Marker("keep_def"),
		"context":      map[string]any{"includeDeclaration": true},
	})
	if refs.Err != nil || !strings.Contains(string(refs.Result), "fresh.gox") {
		t.Errorf("references do not reach the unsaved document: result=%s err=%v", compactJSON(refs.Result), refs.Err)
	}
	if strings.Contains(string(refs.Result), ".x.go") {
		t.Errorf("references leak a generated location: %s", compactJSON(refs.Result))
	}
	for _, method := range []string{"textDocument/formatting", "textDocument/documentSymbol", "textDocument/foldingRange", "textDocument/codeLens"} {
		res := h.call(method, map[string]any{
			"textDocument": map[string]any{"uri": freshURI},
			"options":      map[string]any{"tabSize": 4, "insertSpaces": false},
		})
		if res.Err != nil {
			t.Errorf("%s on an unsaved document failed: %v", method, res.Err)
		}
	}
	completion := h.call("textDocument/completion", map[string]any{
		"textDocument": map[string]any{"uri": freshURI},
		"position":     draft.Marker("keep_ref"),
	})
	if completion.Err != nil {
		t.Errorf("completion on an unsaved document failed: %v", completion.Err)
	}
	prepare := h.call("textDocument/prepareRename", map[string]any{
		"textDocument": map[string]any{"uri": freshURI},
		"position":     draft.Marker("fresh_def"),
	})
	if prepare.Err != nil {
		t.Errorf("prepareRename on an unsaved document failed: %v", prepare.Err)
	}
	rename := h.call("textDocument/rename", map[string]any{
		"textDocument": map[string]any{"uri": freshURI},
		"position":     draft.Marker("fresh_def"),
		"newName":      "Fresher",
	})
	if rename.Err != nil || !strings.Contains(string(rename.Result), "fresh.gox") {
		t.Errorf("rename on an unsaved document: result=%s err=%v", compactJSON(rename.Result), rename.Err)
	}
	if strings.Contains(string(rename.Result), ".x.go") {
		t.Errorf("rename edits a generated file: %s", compactJSON(rename.Result))
	}
	actions := h.call("textDocument/codeAction", map[string]any{
		"textDocument": map[string]any{"uri": freshURI},
		"range":        lspRange{Start: draft.Marker("fresh_def"), End: draft.Marker("fresh_def")},
		"context":      map[string]any{"diagnostics": []any{}},
	})
	if actions.Err != nil {
		t.Errorf("codeAction on an unsaved document failed: %v", actions.Err)
	}

	time.Sleep(900 * time.Millisecond)
	if _, err := os.Stat(freshTarget); !os.IsNotExist(err) {
		t.Errorf("generated target on disk for an unsaved buffer (stat err: %v)", err)
	}

	if err := os.Remove(keep.Path); err != nil {
		t.Fatal(err)
	}
	time.Sleep(900 * time.Millisecond)
	if _, err := os.Stat(keepTarget); !os.IsNotExist(err) {
		t.Errorf("generated target on disk while the source file is gone (stat err: %v)", err)
	}
	unsavedWaitHover(t, h, keepURI, keep.Marker("keep_def"), "Keep")
	def = h.call("textDocument/definition", map[string]any{
		"textDocument": map[string]any{"uri": freshURI},
		"position":     draft.Marker("keep_ref"),
	})
	if def.Err != nil || !strings.Contains(string(def.Result), "keep.gox") {
		t.Errorf("definition into a kept document: result=%s err=%v", compactJSON(def.Result), def.Err)
	}
	h.changeFile(keep.Path, "package pkg\n\nfunc Keep() string {\n\treturn \"kept\"\n}\n\nfunc Extra() int { return 1 }\n")
	unsavedWaitHover(t, h, keepURI, lspPosition{Line: 6, Character: 5}, "Extra")

	h.saveFile(freshPath)
	waitForFileTextContains(t, freshTarget, "Fresh")

	h.closeFile(keep.Path)
	time.Sleep(900 * time.Millisecond)
	gone := h.call("textDocument/hover", map[string]any{
		"textDocument": map[string]any{"uri": keepURI},
		"position":     lspPosition{Line: 2, Character: 5},
	})
	if gone.Err == nil && strings.Contains(string(gone.Result), "Keep") {
		t.Errorf("document with no file and no buffer still answers hover: %s", compactJSON(gone.Result))
	}
	if _, err := os.Stat(keepTarget); !os.IsNotExist(err) {
		t.Errorf("generated target reappeared for a dropped document (stat err: %v)", err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		res := h.call("textDocument/hover", map[string]any{
			"textDocument": map[string]any{"uri": freshURI},
			"position":     draft.Marker("keep_ref"),
		})
		if res.Err == nil && !strings.Contains(string(res.Result), "Keep") {
			break
		}
		if time.Now().After(deadline) {
			t.Errorf("gopls still resolves Keep after its document was dropped (stale overlay): result=%s err=%v", compactJSON(res.Result), res.Err)
			break
		}
		time.Sleep(lspPollInterval)
	}
}
