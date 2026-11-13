package server_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	_ "unsafe"

	"github.com/doors-dev/gox/cmd/gox/internal/processor"
)

// goxVersion is the version gox writes into generated headers. Test builds
// leave it at "(devel)", which turns off the newer-GoX check.
//
//go:linkname goxVersion github.com/doors-dev/gox/internal/assembler.version
var goxVersion string

func setGoxVersion(t *testing.T, version string) {
	old := goxVersion
	goxVersion = version
	t.Cleanup(func() {
		goxVersion = old
	})
}

func generate(t *testing.T, ws unsavedWorkspace, targetPath string, source string) string {
	t.Helper()
	writeFile(t, ws.existing.Path, source)
	if err := processor.Generate([]string{ws.existing.Path}, false, true, false); err != nil {
		t.Fatal(err)
	}
	return readFile(t, targetPath)
}

const reloadedLongSource = `package pkg

func /*@existing_def*/Existing() string {
	return "existing"
}

func Padding() {}

func /*@long_def*/AVeryLongFunctionNameThatOutrunsEveryOldGeneratedLine() string {
	return "long"
}
`

const reloadedLongSignature = "func AVeryLongFunctionNameThatOutrunsEveryOldGeneratedLine() string"

// A checkout changes a .gox and its .x.go while neither is open. Opening the
// .x.go then gives gopls the new code and leaves the buffer as it is.
func TestLSPTargetOpenedAloneAfterCheckout(t *testing.T) {
	ws := newUnsavedWorkspace(t)
	h := ws.h
	targetPath := filepath.Join(ws.root, "pkg", "existing.x.go")
	waitForFileContents(t, targetPath)

	writeFile(t, ws.existing.Path, parseMarked(t, ws.existing.Path, reloadedLongSource).Text)
	if err := processor.Generate([]string{ws.existing.Path}, false, true, false); err != nil {
		t.Fatal(err)
	}
	text := readFile(t, targetPath)
	start := h.checkpoint()
	targetURI := h.openFileWithText(targetPath, "go", text)
	h.waitForHover(targetURI, positionOfSubstring(t, text, "AVeryLong"), reloadedLongSignature)
	if h.bufferText(targetURI) != text {
		t.Fatalf("the .x.go buffer was changed:\n%s\nrecorded events:\n%s", h.bufferText(targetURI), h.dumpEventsAfter(start))
	}
	if strings.Contains(h.dumpEventsAfter(start), "workspace/applyEdit") {
		t.Fatalf("the .x.go buffer got an edit\nrecorded events:\n%s", h.dumpEventsAfter(start))
	}
}

// A rename from a .go file onto a .gox that changed on disk and is not open
// is refused once; the retry edits the file on disk.
func TestLSPRenameOntoSourceChangedOnDisk(t *testing.T) {
	ws := newUnsavedWorkspace(t)
	h := ws.h
	use := writeMarkedFile(t, filepath.Join(ws.root, "pkg", "use.go"), "package pkg\n\nfunc Use() string {\n\treturn /*@existing_use*/Existing()\n}\n")
	useURI := h.openFile(use.Path, "go")
	h.waitForHover(useURI, use.Marker("existing_use"), "func Existing() string")

	shifted := strings.Replace(ws.existing.Text, "func Existing", "// a\n// b\nvar Extra = 1\n\nfunc Existing", 1)
	writeFile(t, ws.existing.Path, shifted)
	params := map[string]any{
		"textDocument": map[string]any{"uri": useURI},
		"position":     use.Marker("existing_use"),
		"newName":      "Renamed",
	}
	start := h.checkpoint()
	if res := h.call("textDocument/rename", params); res.Err == nil {
		t.Fatalf("rename onto a changed .gox was not refused: %s", compactJSON(res.Result))
	}
	h.waitForEventAfter(start, "notification", "window/showMessage", func(params json.RawMessage) bool {
		return strings.Contains(string(params), "existing.gox changed on disk")
	})

	edit := h.callRaw("textDocument/rename", params)
	if err := h.applyWorkspaceEdit(mustJSON[map[string]any](t, edit)); err != nil {
		t.Fatalf("apply rename %s: %v", compactJSON(edit), err)
	}
	if got, want := readFile(t, ws.existing.Path), strings.Replace(shifted, "func Existing", "func Renamed", 1); got != want {
		t.Fatalf("renamed .gox:\n%s\nwant:\n%s", got, want)
	}
	if !strings.Contains(h.bufferText(useURI), "return Renamed()") {
		t.Fatalf("use.go buffer:\n%s", h.bufferText(useURI))
	}
}

// A stale .x.go opened while its .gox is never opened: gopls gets the
// generated code, not the buffer text.
func TestLSPStaleTargetOpenedAlone(t *testing.T) {
	ws := newUnsavedWorkspace(t)
	targetPath := filepath.Join(ws.root, "pkg", "existing.x.go")
	ws.h.openFile(ws.existing.Path, "gox")
	stale := waitForFileContents(t, targetPath)
	ws.h.close()
	writeFile(t, ws.existing.Path, parseMarked(t, ws.existing.Path, reloadedLongSource).Text)

	h := newSingleFolderHarness(t, ws.root)
	targetURI := h.openFileWithText(targetPath, "go", stale)
	text := h.waitForBufferText(targetURI, func(text string) bool {
		return strings.Contains(text, "AVeryLong")
	}, "synced with the generated code")
	h.waitForHover(targetURI, positionOfSubstring(t, text, "AVeryLong"), reloadedLongSignature)
}

// A .gox reopened with unsaved text while its .x.go buffer stays open: gopls
// gets the code generated from that text.
func TestLSPSourceReopenedWithUnsavedTextWhileTargetOpen(t *testing.T) {
	ws := newUnsavedWorkspace(t)
	h := ws.h
	targetPath := filepath.Join(ws.root, "pkg", "existing.x.go")
	sourceURI := h.openFile(ws.existing.Path, "gox")
	h.waitForHover(sourceURI, ws.existing.Marker("existing_def"), "func Existing() string")
	waitForFileContents(t, targetPath)
	h.openFile(targetPath, "go")
	h.closeFile(ws.existing.Path)

	reopened := parseMarked(t, ws.existing.Path, reloadedLongSource)
	h.openFileWithText(ws.existing.Path, "gox", reopened.Text)
	h.waitForHover(sourceURI, reopened.Marker("long_def"), reloadedLongSignature)
}

func withOlderHeader(t *testing.T, text string) string {
	t.Helper()
	header, body, ok := strings.Cut(text, "\n")
	if !ok || !strings.Contains(header, "Managed by GoX") {
		t.Fatalf("no GoX header in:\n%s", text)
	}
	return "// Managed by GoX v0.3.3\n" + body
}

// A committed .x.go whose header names an older GoX but is otherwise up to
// date, opened while its .gox is not: the buffer is left as it is.
func TestLSPTargetWithOlderHeaderOpenedAlone(t *testing.T) {
	ws := newUnsavedWorkspace(t)
	h := ws.h
	targetPath := filepath.Join(ws.root, "pkg", "existing.x.go")
	text := withOlderHeader(t, waitForFileContents(t, targetPath))
	writeFile(t, targetPath, text)

	start := h.checkpoint()
	targetURI := h.openFileWithText(targetPath, "go", text)
	h.waitForHover(targetURI, positionOfSubstring(t, text, "Existing"), "func Existing() string")
	if h.bufferText(targetURI) != text || strings.Contains(h.dumpEventsAfter(start), "workspace/applyEdit") {
		t.Fatalf("the .x.go buffer got an edit:\n%s\nrecorded events:\n%s", h.bufferText(targetURI), h.dumpEventsAfter(start))
	}
}

// changeFileRanged sends newText as one ranged change covering only the part
// that differs, as VS Code does when it reloads a buffer from disk.
func (h *lspHarness) changeFileRanged(path string, newText string) {
	h.t.Helper()
	uri := uriForPath(path)
	old := h.bufferText(uri)
	p := 0
	for p < len(old) && p < len(newText) && old[p] == newText[p] {
		p++
	}
	s := 0
	for s < len(old)-p && s < len(newText)-p && old[len(old)-1-s] == newText[len(newText)-1-s] {
		s++
	}
	h.patchFile(uri, unsavedEdit{
		Range: lspRange{Start: positionAtOffset(old, p), End: positionAtOffset(old, len(old)-s)},
		Text:  newText[p : len(newText)-s],
	})
}

// A checkout changes a .gox and its .x.go while only the .x.go is open, and
// the editor reloads that buffer from disk with a didChange. gopls gets the
// new code and the buffer gets no edit.
func TestLSPTargetBufferReloadedAfterCheckout(t *testing.T) {
	setup := func(t *testing.T) (unsavedWorkspace, string, string) {
		ws := newUnsavedWorkspace(t)
		targetPath := filepath.Join(ws.root, "pkg", "existing.x.go")
		before := waitForFileContents(t, targetPath)
		targetURI := ws.h.openFileWithText(targetPath, "go", before)
		ws.h.waitForHover(targetURI, positionOfSubstring(t, before, "Existing"), "func Existing() string")
		return ws, targetPath, targetURI
	}
	assertNoEdit := func(t *testing.T, h *lspHarness, uri string, want string, start int) {
		t.Helper()
		if h.bufferText(uri) != want || strings.Contains(h.dumpEventsAfter(start), "workspace/applyEdit") {
			t.Fatalf("the .x.go buffer got an edit:\n%s\nrecorded events:\n%s", h.bufferText(uri), h.dumpEventsAfter(start))
		}
	}
	for _, tc := range []struct {
		name   string
		older  bool
		ranged bool
	}{{"same header", false, false}, {"older header", true, false}, {"ranged change, older header", true, true}} {
		t.Run(tc.name, func(t *testing.T) {
			ws, targetPath, targetURI := setup(t)
			h := ws.h
			writeFile(t, ws.existing.Path, parseMarked(t, ws.existing.Path, reloadedLongSource).Text)
			if err := processor.Generate([]string{ws.existing.Path}, false, true, false); err != nil {
				t.Fatal(err)
			}
			text := readFile(t, targetPath)
			if tc.older {
				text = withOlderHeader(t, text)
				writeFile(t, targetPath, text)
			}
			start := h.checkpoint()
			if tc.ranged {
				h.changeFileRanged(targetPath, text)
			} else {
				h.changeFile(targetPath, text)
			}
			h.waitForHover(targetURI, positionOfSubstring(t, text, "AVeryLong"), reloadedLongSignature)
			assertNoEdit(t, h, targetURI, text, start)
			if readFile(t, targetPath) != text {
				t.Fatalf("the .x.go file was rewritten:\n%s", readFile(t, targetPath))
			}
		})
	}

	// The committed .x.go was generated from an older .gox. gox rewrites the
	// file, and the editor's next reload brings the buffer to what gopls has.
	t.Run("stale committed file", func(t *testing.T) {
		ws, targetPath, targetURI := setup(t)
		h := ws.h
		source := parseMarked(t, ws.existing.Path, reloadedLongSource).Text
		writeFile(t, ws.existing.Path, strings.Replace(source, "AVeryLongFunctionNameThatOutrunsEveryOldGeneratedLine", "Stale", 1))
		if err := processor.Generate([]string{ws.existing.Path}, false, true, false); err != nil {
			t.Fatal(err)
		}
		stale := readFile(t, targetPath)
		writeFile(t, ws.existing.Path, source)

		start := h.checkpoint()
		h.changeFileRanged(targetPath, stale)
		written := waitForFileTextContains(t, targetPath, "AVeryLong")
		if written == stale || strings.Contains(written, "Stale") {
			t.Fatalf("the .x.go file was not rewritten:\n%s", written)
		}
		h.waitForHover(targetURI, positionOfSubstring(t, written, "AVeryLong"), reloadedLongSignature)
		assertNoEdit(t, h, targetURI, stale, start)

		h.changeFileRanged(targetPath, written)
		h.waitForHover(targetURI, positionOfSubstring(t, written, "AVeryLong"), reloadedLongSignature)
		h.waitForHover(targetURI, positionOfSubstring(t, written, "Existing"), "func Existing() string")
		assertNoEdit(t, h, targetURI, written, start)
		if readFile(t, targetPath) != written {
			t.Fatalf("the .x.go file changed again:\n%s", readFile(t, targetPath))
		}
	})

	// gox cannot rewrite a stale read-only .x.go, so it syncs the buffer.
	t.Run("read-only stale file", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root can write a read-only file")
		}
		ws, targetPath, targetURI := setup(t)
		h := ws.h
		source := parseMarked(t, ws.existing.Path, reloadedLongSource).Text
		text := generate(t, ws, targetPath, source)
		stale := generate(t, ws, targetPath, strings.Replace(source, "AVeryLongFunctionNameThatOutrunsEveryOldGeneratedLine", "Stale", 1))
		writeFile(t, ws.existing.Path, source)
		if err := os.Chmod(targetPath, 0o444); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			os.Chmod(targetPath, 0o644)
		})

		start := h.checkpoint()
		h.changeFileRanged(targetPath, stale)
		got := h.waitForBufferText(targetURI, func(text string) bool {
			return strings.Contains(text, "AVeryLong")
		}, "synced with the generated code")
		if got != text {
			t.Fatalf("the .x.go buffer:\n%s\nwant:\n%s\nrecorded events:\n%s", got, text, h.dumpEventsAfter(start))
		}
		h.waitForHover(targetURI, positionOfSubstring(t, text, "AVeryLong"), reloadedLongSignature)
		if readFile(t, targetPath) != stale {
			t.Fatalf("the read-only .x.go file was rewritten:\n%s", readFile(t, targetPath))
		}
	})

	// A read-only .x.go that differs from the generated code only in its
	// line endings is up to date, so the buffer gets no edit.
	t.Run("read-only CRLF file, up to date", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root can write a read-only file")
		}
		ws, targetPath, targetURI := setup(t)
		h := ws.h
		crlf := strings.ReplaceAll(generate(t, ws, targetPath, parseMarked(t, ws.existing.Path, reloadedLongSource).Text), "\n", "\r\n")
		writeFile(t, targetPath, crlf)
		if err := os.Chmod(targetPath, 0o444); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			os.Chmod(targetPath, 0o644)
		})

		start := h.checkpoint()
		h.changeFileRanged(targetPath, crlf)
		h.waitForHover(targetURI, positionOfSubstring(t, crlf, "AVeryLong"), reloadedLongSignature)
		assertNoEdit(t, h, targetURI, crlf, start)
		if readFile(t, targetPath) != crlf {
			t.Fatalf("the read-only .x.go file was rewritten:\n%q", readFile(t, targetPath))
		}
	})

	// A stale .x.go from a higher GoX minor version is neither rewritten nor
	// pushed into the buffer, so saving the buffer cannot downgrade it. One
	// from a newer patch release is rewritten like any stale file.
	for _, tc := range []struct {
		name    string
		header  string
		rewrite bool
	}{{"higher minor GoX header, stale body", "v0.5.0", false}, {"higher patch GoX header, stale body", "v0.4.1", true}} {
		t.Run(tc.name, func(t *testing.T) {
			setGoxVersion(t, "v0.4.0")
			ws, targetPath, targetURI := setup(t)
			h := ws.h
			source := parseMarked(t, ws.existing.Path, reloadedLongSource).Text
			text := generate(t, ws, targetPath, source)
			header, body, _ := strings.Cut(generate(t, ws, targetPath, strings.Replace(source, "AVeryLongFunctionNameThatOutrunsEveryOldGeneratedLine", "Stale", 1)), "\n")
			if header != "// Managed by GoX v0.4.0" {
				t.Fatalf("generated header %q", header)
			}
			newer := "// Managed by GoX " + tc.header + "\n" + body
			writeFile(t, targetPath, newer)
			writeFile(t, ws.existing.Path, source)

			start := h.checkpoint()
			h.changeFileRanged(targetPath, newer)
			h.waitForHover(targetURI, positionOfSubstring(t, text, "AVeryLong"), reloadedLongSignature)
			assertNoEdit(t, h, targetURI, newer, start)
			want := newer
			if tc.rewrite {
				want = text
			}
			if got := readFile(t, targetPath); got != want {
				t.Fatalf("the .x.go file:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// The .x.go was opened before a workspace folder covered it, so gox does not
// track the buffer. After the folder is added, a checkout changes the .gox and
// its .x.go, and the editor reloads the buffer with a didChange. gopls gets
// the new code and the buffer gets no edit.
func TestLSPUntrackedTargetBufferReloadedAfterCheckout(t *testing.T) {
	root := filepath.Join(t.TempDir(), "main")
	writeFile(t, filepath.Join(root, "go.mod"), singleFolderGoMod)
	existing := writeMarkedFile(t, filepath.Join(root, "pkg", "existing.gox"), unsavedExistingSource)
	if err := processor.Generate([]string{existing.Path}, false, true, false); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(root, "pkg", "existing.x.go")
	before := readFile(t, targetPath)
	other := filepath.Join(t.TempDir(), "other")
	writeFile(t, filepath.Join(other, "go.mod"), "module example.com/other\n\ngo 1.25.1\n")
	h := newSingleFolderHarness(t, other)
	targetURI := h.openFileWithText(targetPath, "go", before)
	h.waitForHover(targetURI, positionOfSubstring(t, before, "Existing"), "func Existing() string")
	added := h.checkpoint()
	h.changeWorkspaceFolders([]workspaceFolder{{URI: uriForPath(root), Name: "main"}}, nil)
	h.waitForEventAfter(added, "notification", "textDocument/publishDiagnostics", func(raw json.RawMessage) bool {
		var params publishDiagnosticsParams
		return json.Unmarshal(raw, &params) == nil && params.URI == uriForPath(existing.Path)
	})

	writeFile(t, existing.Path, parseMarked(t, existing.Path, reloadedLongSource).Text)
	if err := processor.Generate([]string{existing.Path}, false, true, false); err != nil {
		t.Fatal(err)
	}
	text := readFile(t, targetPath)
	start := h.checkpoint()
	h.changeFileRanged(targetPath, text)
	h.waitForHover(targetURI, positionOfSubstring(t, text, "AVeryLong"), reloadedLongSignature)
	if h.bufferText(targetURI) != text || strings.Contains(h.dumpEventsAfter(start), "workspace/applyEdit") {
		t.Fatalf("the .x.go buffer got an edit:\n%s\nrecorded events:\n%s", h.bufferText(targetURI), h.dumpEventsAfter(start))
	}
	if readFile(t, targetPath) != text {
		t.Fatalf("the .x.go file was rewritten:\n%s", readFile(t, targetPath))
	}
}

// The same untracked buffer, but the checkout leaves a stale read-only .x.go
// that gox cannot rewrite. gopls still gets the new code, and neither the
// buffer nor the file is changed.
func TestLSPUntrackedTargetBufferReloadedAfterCheckoutReadOnly(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write a read-only file")
	}
	root := filepath.Join(t.TempDir(), "main")
	writeFile(t, filepath.Join(root, "go.mod"), singleFolderGoMod)
	existing := writeMarkedFile(t, filepath.Join(root, "pkg", "existing.gox"), unsavedExistingSource)
	if err := processor.Generate([]string{existing.Path}, false, true, false); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(root, "pkg", "existing.x.go")
	before := readFile(t, targetPath)
	other := filepath.Join(t.TempDir(), "other")
	writeFile(t, filepath.Join(other, "go.mod"), "module example.com/other\n\ngo 1.25.1\n")
	h := newSingleFolderHarness(t, other)
	targetURI := h.openFileWithText(targetPath, "go", before)
	h.waitForHover(targetURI, positionOfSubstring(t, before, "Existing"), "func Existing() string")
	added := h.checkpoint()
	h.changeWorkspaceFolders([]workspaceFolder{{URI: uriForPath(root), Name: "main"}}, nil)
	h.waitForEventAfter(added, "notification", "textDocument/publishDiagnostics", func(raw json.RawMessage) bool {
		var params publishDiagnosticsParams
		return json.Unmarshal(raw, &params) == nil && params.URI == uriForPath(existing.Path)
	})

	ws := unsavedWorkspace{root: root, h: h, existing: existing}
	source := parseMarked(t, existing.Path, reloadedLongSource).Text
	text := generate(t, ws, targetPath, source)
	stale := generate(t, ws, targetPath, strings.Replace(source, "AVeryLongFunctionNameThatOutrunsEveryOldGeneratedLine", "Stale", 1))
	writeFile(t, existing.Path, source)
	if err := os.Chmod(targetPath, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Chmod(targetPath, 0o644)
	})

	start := h.checkpoint()
	h.changeFileRanged(targetPath, stale)
	h.waitForHover(targetURI, positionOfSubstring(t, text, "AVeryLong"), reloadedLongSignature)
	if h.bufferText(targetURI) != stale || strings.Contains(h.dumpEventsAfter(start), "workspace/applyEdit") {
		t.Fatalf("the .x.go buffer got an edit:\n%s\nrecorded events:\n%s", h.bufferText(targetURI), h.dumpEventsAfter(start))
	}
	if readFile(t, targetPath) != stale {
		t.Fatalf("the read-only .x.go file was rewritten:\n%s", readFile(t, targetPath))
	}
}

// A checkout changes a .gox and its .x.go while only the .x.go is open, and a
// request from another file maps a location through the .gox before the
// editor reloads the buffer. gopls gets the new code; the buffer gets an edit
// only when gox could not rewrite a stale file that no higher GoX minor
// version wrote.
func TestLSPOpenTargetReloadedByRequest(t *testing.T) {
	setup := func(t *testing.T) (unsavedWorkspace, string, string, map[string]any) {
		setGoxVersion(t, "v0.4.0")
		ws := newUnsavedWorkspace(t)
		targetPath := filepath.Join(ws.root, "pkg", "existing.x.go")
		before := waitForFileContents(t, targetPath)
		targetURI := ws.h.openFileWithText(targetPath, "go", before)
		use := writeMarkedFile(t, filepath.Join(ws.root, "pkg", "use.go"), "package pkg\n\nfunc Use() string {\n\treturn /*@existing_use*/Existing()\n}\n")
		useURI := ws.h.openFile(use.Path, "go")
		ws.h.waitForHover(useURI, use.Marker("existing_use"), "func Existing() string")
		params := map[string]any{
			"textDocument": map[string]any{"uri": useURI},
			"position":     use.Marker("existing_use"),
		}
		return ws, targetPath, targetURI, params
	}
	define := func(t *testing.T, h *lspHarness, params map[string]any) {
		t.Helper()
		res := h.call("textDocument/definition", params)
		if res.Err != nil || !strings.Contains(string(res.Result), "existing.gox") {
			t.Fatalf("definition: %s, error %v", compactJSON(res.Result), res.Err)
		}
	}
	for _, tc := range []struct {
		name    string
		header  string
		stale   bool
		rewrite bool
	}{
		{"higher minor GoX header, stale body", "v0.5.0", true, false},
		{"higher patch GoX header, stale body", "v0.4.1", true, true},
		{"older header, same body", "v0.3.3", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws, targetPath, targetURI, params := setup(t)
			h := ws.h
			before := h.bufferText(targetURI)
			source := parseMarked(t, ws.existing.Path, reloadedLongSource).Text
			text := generate(t, ws, targetPath, source)
			_, body, _ := strings.Cut(text, "\n")
			if tc.stale {
				_, body, _ = strings.Cut(generate(t, ws, targetPath, strings.Replace(source, "AVeryLongFunctionNameThatOutrunsEveryOldGeneratedLine", "Stale", 1)), "\n")
				writeFile(t, ws.existing.Path, source)
			}
			disk := "// Managed by GoX " + tc.header + "\n" + body
			writeFile(t, targetPath, disk)

			start := h.checkpoint()
			define(t, h, params)
			h.waitForHover(targetURI, positionOfSubstring(t, text, "AVeryLong"), reloadedLongSignature)
			if h.bufferText(targetURI) != before || strings.Contains(h.dumpEventsAfter(start), "workspace/applyEdit") {
				t.Fatalf("the .x.go buffer got an edit:\n%s\nrecorded events:\n%s", h.bufferText(targetURI), h.dumpEventsAfter(start))
			}
			want := disk
			if tc.rewrite {
				want = text
			}
			if got := readFile(t, targetPath); got != want {
				t.Fatalf("the .x.go file:\n%s\nwant:\n%s", got, want)
			}
		})
	}

	t.Run("read-only stale file", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root can write a read-only file")
		}
		ws, targetPath, targetURI, params := setup(t)
		h := ws.h
		source := parseMarked(t, ws.existing.Path, reloadedLongSource).Text
		text := generate(t, ws, targetPath, source)
		stale := generate(t, ws, targetPath, strings.Replace(source, "AVeryLongFunctionNameThatOutrunsEveryOldGeneratedLine", "Stale", 1))
		writeFile(t, ws.existing.Path, source)
		if err := os.Chmod(targetPath, 0o444); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			os.Chmod(targetPath, 0o644)
		})

		start := h.checkpoint()
		define(t, h, params)
		got := h.waitForBufferText(targetURI, func(buffer string) bool {
			return strings.Contains(buffer, "AVeryLong")
		}, "synced with the generated code")
		if got != text {
			t.Fatalf("the .x.go buffer:\n%s\nwant:\n%s\nrecorded events:\n%s", got, text, h.dumpEventsAfter(start))
		}
		h.waitForHover(targetURI, positionOfSubstring(t, text, "AVeryLong"), reloadedLongSignature)
		if readFile(t, targetPath) != stale {
			t.Fatalf("the read-only .x.go file was rewritten:\n%s", readFile(t, targetPath))
		}
	})
}

// Closing a .gox without saving brings the open .x.go buffer back to the code
// generated from the file, which is up to date on disk.
func TestLSPDiscardedSourceSyncsOpenTarget(t *testing.T) {
	ws := newUnsavedWorkspace(t)
	h := ws.h
	targetPath := filepath.Join(ws.root, "pkg", "existing.x.go")
	sourceURI := h.openFile(ws.existing.Path, "gox")
	h.waitForHover(sourceURI, ws.existing.Marker("existing_def"), "func Existing() string")
	text := waitForFileContents(t, targetPath)
	targetURI := h.openFileWithText(targetPath, "go", text)
	h.changeFile(ws.existing.Path, parseMarked(t, ws.existing.Path, reloadedLongSource).Text)
	h.waitForBufferText(targetURI, func(buffer string) bool {
		return strings.Contains(buffer, "AVeryLong")
	}, "the code generated from the unsaved .gox")

	start := h.checkpoint()
	h.closeFile(ws.existing.Path)
	got := h.waitForBufferText(targetURI, func(buffer string) bool {
		return !strings.Contains(buffer, "AVeryLong")
	}, "synced with the .gox file")
	if got != text {
		t.Fatalf("the .x.go buffer:\n%s\nwant:\n%s\nrecorded events:\n%s", got, text, h.dumpEventsAfter(start))
	}
	if readFile(t, targetPath) != text {
		t.Fatalf("the .x.go file changed:\n%s", readFile(t, targetPath))
	}
}

// A stale .x.go opened while its .gox is not is synced, unless a higher GoX
// minor version wrote it: then neither the buffer nor the file is downgraded,
// and gopls still gets the new code.
func TestLSPTargetWithNewerHeaderOpenedAlone(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header string
		synced bool
	}{{"higher minor", "v0.5.0", false}, {"higher patch", "v0.4.1", true}} {
		t.Run(tc.name, func(t *testing.T) {
			setGoxVersion(t, "v0.4.0")
			ws := newUnsavedWorkspace(t)
			h := ws.h
			targetPath := filepath.Join(ws.root, "pkg", "existing.x.go")
			waitForFileContents(t, targetPath)
			source := parseMarked(t, ws.existing.Path, reloadedLongSource).Text
			text := generate(t, ws, targetPath, source)
			_, body, _ := strings.Cut(generate(t, ws, targetPath, strings.Replace(source, "AVeryLongFunctionNameThatOutrunsEveryOldGeneratedLine", "Stale", 1)), "\n")
			newer := "// Managed by GoX " + tc.header + "\n" + body
			writeFile(t, targetPath, newer)
			writeFile(t, ws.existing.Path, source)

			start := h.checkpoint()
			targetURI := h.openFileWithText(targetPath, "go", newer)
			if tc.synced {
				if got := h.waitForBufferText(targetURI, func(buffer string) bool {
					return buffer != newer
				}, "synced with the generated code"); got != text {
					t.Fatalf("the .x.go buffer:\n%s\nwant:\n%s", got, text)
				}
				if got := readFile(t, targetPath); got != text {
					t.Fatalf("the .x.go file:\n%s\nwant:\n%s", got, text)
				}
				return
			}
			h.waitForHover(targetURI, positionOfSubstring(t, text, "AVeryLong"), reloadedLongSignature)
			if h.bufferText(targetURI) != newer || strings.Contains(h.dumpEventsAfter(start), "workspace/applyEdit") {
				t.Fatalf("the .x.go buffer got an edit:\n%s\nrecorded events:\n%s", h.bufferText(targetURI), h.dumpEventsAfter(start))
			}
			if readFile(t, targetPath) != newer {
				t.Fatalf("the .x.go file was rewritten:\n%s", readFile(t, targetPath))
			}
		})
	}
}

// A .x.go from a higher GoX minor version opened while its .gox is open: gopls
// has the code generated from the .gox, so the buffer is synced with it.
func TestLSPTargetWithNewerHeaderOpenedWithSource(t *testing.T) {
	setGoxVersion(t, "v0.4.0")
	ws := newUnsavedWorkspace(t)
	h := ws.h
	targetPath := filepath.Join(ws.root, "pkg", "existing.x.go")
	sourceURI := h.openFile(ws.existing.Path, "gox")
	h.waitForHover(sourceURI, ws.existing.Marker("existing_def"), "func Existing() string")
	text := waitForFileContents(t, targetPath)
	_, body, _ := strings.Cut(text, "\n")
	newer := "// Managed by GoX v0.5.0\n" + strings.Replace(body, `return "existing"`, `return "newer"`, 1)
	writeFile(t, targetPath, newer)

	targetURI := h.openFileWithText(targetPath, "go", newer)
	if got := h.waitForBufferText(targetURI, func(buffer string) bool {
		return buffer != newer
	}, "synced with the .gox file"); got != text {
		t.Fatalf("the .x.go buffer:\n%s\nwant:\n%s", got, text)
	}
	if readFile(t, targetPath) != newer {
		t.Fatalf("the .x.go file was rewritten:\n%s", readFile(t, targetPath))
	}
}

// A committed .x.go opened while its .gox is not gets an edit only when its
// code below the header differs from the generated code, whatever its line
// endings.
func TestLSPTargetLineEndingsOpenedAlone(t *testing.T) {
	for _, tc := range []struct {
		name  string
		crlf  bool
		stale bool
	}{{"LF, up to date", false, false}, {"LF, stale body", false, true}, {"CRLF, up to date", true, false}, {"CRLF, stale body", true, true}} {
		t.Run(tc.name, func(t *testing.T) {
			ws := newUnsavedWorkspace(t)
			h := ws.h
			targetPath := filepath.Join(ws.root, "pkg", "existing.x.go")
			text := waitForFileContents(t, targetPath)
			disk := text
			if tc.stale {
				disk = strings.Replace(disk, `return "existing"`, `return "stale"`, 1)
			}
			if tc.crlf {
				disk = strings.ReplaceAll(disk, "\n", "\r\n")
			}
			writeFile(t, targetPath, disk)

			start := h.checkpoint()
			targetURI := h.openFileWithText(targetPath, "go", disk)
			if tc.stale {
				if got := h.waitForBufferText(targetURI, func(buffer string) bool {
					return buffer != disk
				}, "synced with the generated code"); got != text {
					t.Fatalf("the .x.go buffer:\n%q\nwant:\n%q", got, text)
				}
				return
			}
			h.waitForHover(targetURI, positionOfSubstring(t, text, "Existing"), "func Existing() string")
			if h.bufferText(targetURI) != disk || strings.Contains(h.dumpEventsAfter(start), "workspace/applyEdit") {
				t.Fatalf("the .x.go buffer got an edit:\n%q\nrecorded events:\n%s", h.bufferText(targetURI), h.dumpEventsAfter(start))
			}
		})
	}
}
