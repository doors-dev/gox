package server_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const unsavedEvictionWait = 900 * time.Millisecond

const unsavedExistingSource = `package pkg

func /*@existing_def*/Existing() string {
	return "existing"
}
`

type unsavedWorkspace struct {
	root     string
	h        *lspHarness
	existing fixtureFile
}

type unsavedEdit struct {
	Range lspRange
	Text  string
}

func newUnsavedWorkspace(t *testing.T) unsavedWorkspace {
	t.Helper()
	root := filepath.Join(t.TempDir(), "main")
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/probe\n\ngo 1.25.1\n")
	existing := writeMarkedFile(t, filepath.Join(root, "pkg", "existing.gox"), unsavedExistingSource)
	writeFile(t, filepath.Join(root, "plain", "helper.go"), "package plain\n\nfunc Helper() string {\n\treturn \"helper\"\n}\n")
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
			},
			"window": map[string]any{},
		},
		"workspaceFolders": []workspaceFolder{folder},
	})
	h.notify("initialized", map[string]any{})
	return unsavedWorkspace{root: root, h: h, existing: existing}
}

func parseMarked(t *testing.T, path string, content string) fixtureFile {
	t.Helper()
	text, markers, err := stripMarkers(content)
	if err != nil {
		t.Fatalf("strip markers for %s: %v", path, err)
	}
	return fixtureFile{Path: path, URI: uriForPath(path), Text: text, Markers: markers}
}

func (h *lspHarness) patchFile(uri string, edits ...unsavedEdit) {
	h.t.Helper()
	doc := h.mustOpenDoc(uri)
	changes := make([]map[string]any, 0, len(edits))
	text := doc.Text
	for _, edit := range edits {
		start, err := offsetForPosition(text, edit.Range.Start)
		if err != nil {
			h.t.Fatalf("edit start: %v", err)
		}
		end, err := offsetForPosition(text, edit.Range.End)
		if err != nil {
			h.t.Fatalf("edit end: %v", err)
		}
		text = text[:start] + edit.Text + text[end:]
		changes = append(changes, map[string]any{"range": edit.Range, "text": edit.Text})
	}
	doc.Version++
	doc.Text = text
	h.notify("textDocument/didChange", map[string]any{
		"textDocument":   map[string]any{"uri": uri, "version": doc.Version},
		"contentChanges": changes,
	})
}

func (h *lspHarness) errorMessagesAfter(start int) []string {
	h.eventsM.Lock()
	defer h.eventsM.Unlock()
	var found []string
	for i := start; i < len(h.events); i++ {
		ev := h.events[i]
		if ev.Method != "window/showMessage" && ev.Method != "window/showMessageRequest" {
			continue
		}
		var params struct {
			Type    int    `json:"type"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(ev.Params, &params); err != nil {
			continue
		}
		if params.Type == 1 {
			found = append(found, params.Message)
		}
	}
	return found
}

func (h *lspHarness) assertNoErrorMessagesAfter(start int, stage string) {
	h.t.Helper()
	if found := h.errorMessagesAfter(start); len(found) != 0 {
		h.t.Fatalf("%s: server showed error messages: %q\nrecorded events:\n%s", stage, found, h.dumpEventsAfter(start))
	}
}

func (h *lspHarness) waitForHover(uri string, pos lspPosition, needle string) json.RawMessage {
	h.t.Helper()
	deadline := time.Now().Add(lspWaitTimeout)
	var last callResult
	for time.Now().Before(deadline) {
		last = h.call("textDocument/hover", map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"position":     pos,
		})
		if last.Err == nil && strings.Contains(string(last.Result), needle) {
			return last.Result
		}
		time.Sleep(lspPollInterval)
	}
	h.t.Fatalf("hover at %s %+v never contained %q; last result %s, last error %v\nrecorded events:\n%s", uri, pos, needle, compactJSON(last.Result), last.Err, h.dumpEvents())
	return nil
}

func (h *lspHarness) definitionLocations(uri string, pos lspPosition) []struct {
	URI   string   `json:"uri"`
	Range lspRange `json:"range"`
} {
	h.t.Helper()
	raw := h.callRaw("textDocument/definition", map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     pos,
	})
	var locations []struct {
		URI   string   `json:"uri"`
		Range lspRange `json:"range"`
	}
	if err := json.Unmarshal(raw, &locations); err != nil {
		h.t.Fatalf("decode definition %s: %v", compactJSON(raw), err)
	}
	return locations
}

func assertAbsentFor(t *testing.T, path string, duration time.Duration, stage string) {
	t.Helper()
	deadline := time.Now().Add(duration)
	for {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s: %s must not exist on disk (stat err: %v)", stage, path, err)
		}
		if !time.Now().Before(deadline) {
			return
		}
		time.Sleep(lspPollInterval)
	}
}

func unsavedSource(pkg string) string {
	return "package " + pkg + `

func /*@fresh_def*/Fresh() string {
	return "fresh"
}

func UseFresh() string {
	return /*@fresh_use*/Fresh()
}
`
}

func TestLSPUnsavedSourceBufferLifecycle(t *testing.T) {
	cases := []struct {
		name       string
		dir        string
		pkg        string
		dirOnDisk  bool
		siblingUse string
		siblingSig string
	}{
		{name: "package dir with another gox", dir: "pkg", pkg: "pkg", dirOnDisk: true, siblingUse: "Existing", siblingSig: "func Existing() string"},
		{name: "package dir without gox files", dir: "plain", pkg: "plain", dirOnDisk: true, siblingUse: "Helper", siblingSig: "func Helper() string"},
		{name: "dir missing on disk", dir: filepath.Join("fresh", "sub"), pkg: "sub"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := newUnsavedWorkspace(t)
			h := ws.h
			dir := filepath.Join(ws.root, tc.dir)
			sourcePath := filepath.Join(dir, "fresh.gox")
			targetPath := filepath.Join(dir, "fresh.x.go")
			content := unsavedSource(tc.pkg)
			if tc.siblingUse != "" {
				content += "\nfunc UseSibling() string {\n\treturn /*@sibling_use*/" + tc.siblingUse + "()\n}\n"
			}
			source := parseMarked(t, sourcePath, content)

			if _, err := os.Stat(dir); tc.dirOnDisk != (err == nil) {
				t.Fatalf("fixture dir %s presence mismatch: %v", dir, err)
			}

			start := h.checkpoint()
			uri := h.openFileWithText(sourcePath, "gox", source.Text)

			h.waitForHover(uri, source.Marker("fresh_use"), "func Fresh() string")
			if tc.siblingUse != "" {
				h.waitForHover(uri, source.Marker("sibling_use"), tc.siblingSig)
			}
			locations := h.definitionLocations(uri, source.Marker("fresh_use"))
			if len(locations) != 1 || locations[0].URI != uri || locations[0].Range.Start != source.Marker("fresh_def") {
				t.Fatalf("definition must point at the buffer declaration %+v in %s, got %+v", source.Marker("fresh_def"), uri, locations)
			}
			assertAbsentFor(t, targetPath, unsavedEvictionWait, "unsaved buffer after open")
			if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
				t.Fatalf("server must not create the source file: %v", err)
			}
			if !tc.dirOnDisk {
				if _, err := os.Stat(dir); !os.IsNotExist(err) {
					t.Fatalf("server must not create the missing directory %s: %v", dir, err)
				}
			}
			h.assertNoErrorMessagesAfter(start, "after didOpen")
			h.waitForHover(uri, source.Marker("fresh_use"), "func Fresh() string")

			def := source.Marker("fresh_def")
			use := source.Marker("fresh_use")
			width := len("Fresh")
			h.patchFile(uri,
				unsavedEdit{
					Range: lspRange{Start: use, End: lspPosition{Line: use.Line, Character: use.Character + width}},
					Text:  "Renamed",
				},
				unsavedEdit{
					Range: lspRange{Start: def, End: lspPosition{Line: def.Line, Character: def.Character + width}},
					Text:  "Renamed",
				},
			)
			h.patchFile(uri, unsavedEdit{
				Range: lspRange{Start: lspPosition{Line: def.Line}, End: lspPosition{Line: def.Line}},
				Text:  "// Renamed carries the unsavedmarker.\n",
			})
			movedUse := lspPosition{Line: use.Line + 1, Character: use.Character}
			movedDef := lspPosition{Line: def.Line + 1, Character: def.Character}

			hover := h.waitForHover(uri, movedUse, "func Renamed() string")
			assertRawContains(t, hover, "unsavedmarker")
			locations = h.definitionLocations(uri, movedUse)
			if len(locations) != 1 || locations[0].URI != uri || locations[0].Range.Start != movedDef {
				t.Fatalf("definition after ranged edits must point at %+v in %s, got %+v", movedDef, uri, locations)
			}
			assertAbsentFor(t, targetPath, unsavedEvictionWait, "unsaved buffer after edits")
			h.assertNoErrorMessagesAfter(start, "after didChange")

			h.saveFile(sourcePath)
			target := waitForFileTextContains(t, targetPath, "func Renamed() string")
			for _, needle := range []string{"package " + tc.pkg, "unsavedmarker", "return Renamed()"} {
				if !strings.Contains(target, needle) {
					t.Fatalf("generated target is missing %q:\n%s", needle, target)
				}
			}
			if strings.Contains(target, "return Fresh()") || strings.Contains(target, "func Fresh()") {
				t.Fatalf("generated target still has pre-edit content:\n%s", target)
			}
			h.waitForHover(uri, movedUse, "func Renamed() string")
			locations = h.definitionLocations(uri, movedUse)
			if len(locations) != 1 || locations[0].URI != uri || locations[0].Range.Start != movedDef {
				t.Fatalf("definition after save must point at %+v in %s, got %+v", movedDef, uri, locations)
			}
			h.assertNoErrorMessagesAfter(start, "after didSave")

			h.closeFile(sourcePath)
			existingURI := h.openFile(ws.existing.Path, "gox")
			h.waitForHover(existingURI, ws.existing.Marker("existing_def"), "func Existing() string")
			assertAbsentFor(t, filepath.Join(dir, "fresh.gox.x.go"), 0, "after close")
			time.Sleep(unsavedEvictionWait)
			if _, err := os.Stat(targetPath); err != nil {
				t.Fatalf("generated target must stay while the saved source exists: %v", err)
			}
			h.waitForHover(existingURI, ws.existing.Marker("existing_def"), "func Existing() string")
			h.assertNoErrorMessagesAfter(start, "after didClose")
		})
	}
}

func TestLSPUnsavedEmptyAndBrokenBuffersKeepServerAlive(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{name: "empty", text: ""},
		{name: "whitespace only", text: "\n\n"},
		{name: "broken go", text: "package pkg\n\nfunc Broken( {\n\treturn\n"},
		{name: "broken element", text: "package pkg\n\nfunc Broken() any {\n\treturn <div><span>\n}\n"},
		{name: "not go at all", text: "<<<<< }}}} ~( \x00 ?"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := newUnsavedWorkspace(t)
			h := ws.h
			sourcePath := filepath.Join(ws.root, "pkg", "odd.gox")
			targetPath := filepath.Join(ws.root, "pkg", "odd.x.go")

			existingURI := h.openFile(ws.existing.Path, "gox")
			h.waitForHover(existingURI, ws.existing.Marker("existing_def"), "func Existing() string")

			uri := h.openFileWithText(sourcePath, "gox", tc.text)
			h.call("textDocument/hover", map[string]any{
				"textDocument": map[string]any{"uri": uri},
				"position":     lspPosition{},
			})
			h.call("textDocument/documentSymbol", map[string]any{
				"textDocument": map[string]any{"uri": uri},
			})
			h.waitForHover(existingURI, ws.existing.Marker("existing_def"), "func Existing() string")
			assertAbsentFor(t, targetPath, unsavedEvictionWait, "odd unsaved buffer")

			valid := parseMarked(t, sourcePath, "package pkg\n\nfunc /*@odd_def*/Odd() string {\n\treturn Existing()\n}\n")
			h.changeFile(sourcePath, valid.Text)
			h.waitForHover(uri, valid.Marker("odd_def"), "func Odd() string")

			h.changeFile(sourcePath, tc.text)
			h.call("textDocument/hover", map[string]any{
				"textDocument": map[string]any{"uri": uri},
				"position":     lspPosition{},
			})
			h.closeFile(sourcePath)
			h.waitForHover(existingURI, ws.existing.Marker("existing_def"), "func Existing() string")
			assertAbsentFor(t, targetPath, unsavedEvictionWait, "odd unsaved buffer after close")
			h.waitForHover(existingURI, ws.existing.Marker("existing_def"), "func Existing() string")
		})
	}
}

func TestLSPUnsavedEmptyBufferFilledByRangedEdit(t *testing.T) {
	ws := newUnsavedWorkspace(t)
	h := ws.h
	sourcePath := filepath.Join(ws.root, "pkg", "filled.gox")
	targetPath := filepath.Join(ws.root, "pkg", "filled.x.go")

	start := h.checkpoint()
	uri := h.openFileWithText(sourcePath, "gox", "")
	filled := parseMarked(t, sourcePath, "package pkg\n\nfunc /*@filled_def*/Filled() string {\n\treturn Existing()\n}\n")
	h.patchFile(uri, unsavedEdit{Text: filled.Text})
	h.waitForHover(uri, filled.Marker("filled_def"), "func Filled() string")
	assertAbsentFor(t, targetPath, unsavedEvictionWait, "filled unsaved buffer")
	h.assertNoErrorMessagesAfter(start, "empty buffer filled by a ranged edit")

	h.saveFile(sourcePath)
	target := waitForFileTextContains(t, targetPath, "func Filled() string")
	if !strings.Contains(target, "return Existing()") {
		t.Fatalf("generated target does not match the buffer:\n%s", target)
	}
	h.closeFile(sourcePath)
	h.assertNoErrorMessagesAfter(start, "after close")
}

func TestLSPUnsavedBufferClosedWithoutSaving(t *testing.T) {
	cases := []struct {
		name      string
		dir       string
		pkg       string
		dirOnDisk bool
	}{
		{name: "package dir with another gox", dir: "pkg", pkg: "pkg", dirOnDisk: true},
		{name: "dir missing on disk", dir: filepath.Join("fresh", "sub"), pkg: "sub"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := newUnsavedWorkspace(t)
			h := ws.h
			dir := filepath.Join(ws.root, tc.dir)
			sourcePath := filepath.Join(dir, "fresh.gox")
			targetPath := filepath.Join(dir, "fresh.x.go")
			source := parseMarked(t, sourcePath, "package "+tc.pkg+"\n\nfunc /*@only_def*/UnsavedOnlySymbol() string {\n\treturn \"only\"\n}\n")

			start := h.checkpoint()
			uri := h.openFileWithText(sourcePath, "gox", source.Text)
			h.waitForHover(uri, source.Marker("only_def"), "func UnsavedOnlySymbol() string")
			symbols := h.callRaw("workspace/symbol", map[string]any{"query": "UnsavedOnlySymbol"})
			assertRawContains(t, symbols, "UnsavedOnlySymbol")

			h.closeFile(sourcePath)
			symbols = h.callRaw("workspace/symbol", map[string]any{"query": "UnsavedOnlySymbol"})
			if strings.Contains(string(symbols), "UnsavedOnlySymbol") {
				t.Fatalf("gopls still knows the closed unsaved buffer: %s", compactJSON(symbols))
			}

			existingURI := h.openFile(ws.existing.Path, "gox")
			h.waitForHover(existingURI, ws.existing.Marker("existing_def"), "func Existing() string")
			assertAbsentFor(t, targetPath, unsavedEvictionWait, "closed unsaved buffer")
			if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
				t.Fatalf("server must not create the source file: %v", err)
			}
			if !tc.dirOnDisk {
				if _, err := os.Stat(dir); !os.IsNotExist(err) {
					t.Fatalf("server must not create the missing directory %s: %v", dir, err)
				}
			}

			h.assertNoErrorMessagesAfter(start, "after closing the unsaved buffer")

			h.call("textDocument/hover", map[string]any{
				"textDocument": map[string]any{"uri": uri},
				"position":     source.Marker("only_def"),
			})
			h.closeFile(sourcePath)
			h.waitForHover(existingURI, ws.existing.Marker("existing_def"), "func Existing() string")

			reopened := h.checkpoint()
			h.openFileWithText(sourcePath, "gox", source.Text)
			h.waitForHover(uri, source.Marker("only_def"), "func UnsavedOnlySymbol() string")
			h.closeFile(sourcePath)
			assertAbsentFor(t, targetPath, unsavedEvictionWait, "reopened and closed unsaved buffer")
			h.waitForHover(existingURI, ws.existing.Marker("existing_def"), "func Existing() string")
			h.assertNoErrorMessagesAfter(reopened, "after reopening the unsaved buffer")
		})
	}
}
