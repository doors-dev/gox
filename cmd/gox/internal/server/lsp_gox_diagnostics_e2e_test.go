package server_test

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func (h *lspHarness) waitForGoxDiagnostic(uri string, want lspDiagnostic) {
	h.t.Helper()
	h.waitForEventAfter(0, "notification", "textDocument/publishDiagnostics", func(raw json.RawMessage) bool {
		var params publishDiagnosticsParams
		if json.Unmarshal(raw, &params) != nil || params.URI != uri {
			return false
		}
		for _, d := range params.Diagnostics {
			if d == want {
				return true
			}
		}
		return false
	})
}

func TestLSPDiagnosticsReportStrayCloseTagsAndMissingPackage(t *testing.T) {
	ws := newUnsavedWorkspace(t)
	h := ws.h
	stray := filepath.Join(ws.root, "pkg", "stray.gox")
	writeFile(t, stray, "package pkg\n\nelem Card() {\n\t<div>hi</my-crad></div>\n}\n")
	empty := filepath.Join(ws.root, "pkg", "empty.gox")
	writeFile(t, empty, "")
	commentOnly := filepath.Join(ws.root, "pkg", "comment.gox")
	writeFile(t, commentOnly, "// Package pkg.\n")

	h.waitForGoxDiagnostic(h.openFile(stray, "gox"), lspDiagnostic{
		Range:    lspRange{Start: lspPosition{Line: 3, Character: 8}, End: lspPosition{Line: 3, Character: 18}},
		Severity: 2,
		Message:  "`</my-crad>` does not match an open element and is ignored.",
	})
	missingPackage := lspDiagnostic{Severity: 1, Message: "Syntax error: missing package clause"}
	h.waitForGoxDiagnostic(h.openFile(empty, "gox"), missingPackage)
	h.waitForGoxDiagnostic(h.openFile(commentOnly, "gox"), missingPackage)
	h.waitForGoxDiagnostic(h.openFileWithText(filepath.Join(ws.root, "pkg", "unsaved.gox"), "gox", "elem Card() {\n\t<div></div>\n}\n"), missingPackage)
	h.assertNoErrorMessagesAfter(0, "gox diagnostics")
}
