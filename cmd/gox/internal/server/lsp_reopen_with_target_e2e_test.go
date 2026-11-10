package server_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// The editor reloads a .gox buffer (didClose + didOpen with new text) after the
// file changed on disk, while the .x.go buffer stays open. gopls must see the
// regenerated code, or positions mapped from the new source land in the old
// generated text ("column is beyond end of line").
func TestLSPSourceReopenedWithNewTextWhileTargetOpen(t *testing.T) {
	ws := newUnsavedWorkspace(t)
	h := ws.h
	targetPath := filepath.Join(ws.root, "pkg", "existing.x.go")

	sourceURI := h.openFile(ws.existing.Path, "gox")
	h.waitForHover(sourceURI, ws.existing.Marker("existing_def"), "func Existing() string")
	waitForFileContents(t, targetPath)
	h.openFile(targetPath, "go")

	reloaded := parseMarked(t, ws.existing.Path, `package pkg

func /*@existing_def*/Existing() string {
	return "existing"
}

func Padding() {}

func /*@long_def*/AVeryLongFunctionNameThatOutrunsEveryOldGeneratedLine() string {
	return "long"
}
`)
	writeFile(t, ws.existing.Path, reloaded.Text)
	h.closeFile(ws.existing.Path)
	h.openFileWithText(ws.existing.Path, "gox", reloaded.Text)

	h.waitForHover(sourceURI, reloaded.Marker("long_def"), "func AVeryLongFunctionNameThatOutrunsEveryOldGeneratedLine() string")
}

// A stale .x.go on disk (e.g. the .gox changed while no server ran) opened
// before its .gox: gopls must get the generated code, not the buffer text,
// even though opening the .gox then brings no new source text.
func TestLSPStaleTargetOpenedBeforeSource(t *testing.T) {
	ws := newUnsavedWorkspace(t)
	targetPath := filepath.Join(ws.root, "pkg", "existing.x.go")
	ws.h.openFile(ws.existing.Path, "gox")
	stale := waitForFileContents(t, targetPath)
	ws.h.close()

	current := parseMarked(t, ws.existing.Path, `package pkg

func /*@existing_def*/Existing() string {
	return "existing"
}

func Padding() {}

func /*@long_def*/AVeryLongFunctionNameThatOutrunsEveryOldGeneratedLine() string {
	return "long"
}
`)
	writeFile(t, ws.existing.Path, current.Text)

	h := newSingleFolderHarness(t, ws.root)
	targetURI := h.openFileWithText(targetPath, "go", stale)
	sourceURI := h.openFile(ws.existing.Path, "gox")

	h.waitForHover(sourceURI, current.Marker("long_def"), "func AVeryLongFunctionNameThatOutrunsEveryOldGeneratedLine() string")
	h.waitForBufferText(targetURI, func(text string) bool {
		return strings.Contains(text, "AVeryLongFunctionNameThatOutrunsEveryOldGeneratedLine")
	}, "synced with the generated code")
}
