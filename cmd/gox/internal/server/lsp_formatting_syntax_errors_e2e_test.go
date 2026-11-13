package server_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestLSPFormattingRefusesSyntaxErrors(t *testing.T) {
	ws := newUnsavedWorkspace(t)
	h := ws.h
	for name, src := range map[string]string{
		"missing.gox": "package pkg\n\nfunc Make(n int) []int {\n\ts := make([]int, n\n}\n\nfunc Other() int {\n\tif true {\n\t\treturn 1\n\t}\n\treturn 2\n}\n",
		"error.gox":   "package pkg\n\nfunc Make(n int) []int {\n\ts := make([]int, n\n\treturn s\n}\n",
	} {
		start := h.checkpoint()
		uri := h.openFileWithText(filepath.Join(ws.root, "pkg", name), "gox", src)
		h.waitForEventAfter(start, "notification", "textDocument/publishDiagnostics", func(raw json.RawMessage) bool {
			var params publishDiagnosticsParams
			return json.Unmarshal(raw, &params) == nil && params.URI == uri
		})

		start = h.checkpoint()
		res := h.call("textDocument/formatting", map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"options":      map[string]any{"tabSize": 4, "insertSpaces": false},
		})
		if res.Err == nil {
			t.Fatalf("formatting %s = %s, want an error", name, compactJSON(res.Result))
		}
		h.waitForEventAfter(start, "notification", "window/showMessage", func(raw json.RawMessage) bool {
			return strings.Contains(string(raw), "Could not format this file.")
		})
	}
}
