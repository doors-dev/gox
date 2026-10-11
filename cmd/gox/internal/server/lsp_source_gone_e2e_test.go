package server_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	watchedFileCreated = 1
	watchedFileDeleted = 3

	sourceGoneRemovalTimeout = 5 * time.Second
	sourceGoneSettleWindow   = 2 * time.Second
	sourceGoneSettleTimeout  = 10 * time.Second
)

const sourceGoneScrollSource = `package pkg

func Scroll() string {
	return "scroll"
}
`

const sourceGoneUserSource = `package pkg

func UseScroll() string {
	return Scroll()
}
`

type watchedFileEvent struct {
	URI  string `json:"uri"`
	Type int    `json:"type"`
}

type sourceGoneFixture struct {
	h             *lspHarness
	root          string
	oldSourcePath string
	oldTargetPath string
	newSourcePath string
	newTargetPath string
	oldSourceURI  string
	oldTargetURI  string
	newSourceURI  string
	newTargetURI  string
}

func newSourceGoneFixture(t *testing.T) *sourceGoneFixture {
	t.Helper()
	root := filepath.Join(t.TempDir(), "main")
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/probe\n\ngo 1.25.1\n")
	writeFile(t, filepath.Join(root, "pkg", "user.go"), sourceGoneUserSource)
	f := &sourceGoneFixture{
		root:          root,
		oldSourcePath: filepath.Join(root, "pkg", "scroll.gox"),
		oldTargetPath: filepath.Join(root, "pkg", "scroll.x.go"),
		newSourcePath: filepath.Join(root, "pkg", "scroll2.gox"),
		newTargetPath: filepath.Join(root, "pkg", "scroll2.x.go"),
	}
	writeFile(t, f.oldSourcePath, sourceGoneScrollSource)
	f.oldSourceURI = uriForPath(f.oldSourcePath)
	f.oldTargetURI = uriForPath(f.oldTargetPath)
	f.newSourceURI = uriForPath(f.newSourcePath)
	f.newTargetURI = uriForPath(f.newTargetPath)

	folder := workspaceFolder{URI: uriForPath(root), Name: "main"}
	f.h = newLSPHarness(t, []workspaceFolder{folder})
	f.h.callRaw("initialize", map[string]any{
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
	f.h.notify("initialized", map[string]any{})
	return f
}

func (f *sourceGoneFixture) changeWatchedFiles(events ...watchedFileEvent) {
	f.h.t.Helper()
	f.h.notify("workspace/didChangeWatchedFiles", map[string]any{"changes": events})
}

func (f *sourceGoneFixture) waitForDiagnosticsAfter(start int, uri string) {
	f.h.t.Helper()
	f.h.waitForEventAfter(start, "notification", "textDocument/publishDiagnostics", func(raw json.RawMessage) bool {
		var params publishDiagnosticsParams
		return json.Unmarshal(raw, &params) == nil && params.URI == uri
	})
}

func (f *sourceGoneFixture) latestDiagnosticsAfter(start int, uri string) publishDiagnosticsParams {
	f.h.eventsM.Lock()
	defer f.h.eventsM.Unlock()
	for i := len(f.h.events) - 1; i >= start; i-- {
		ev := f.h.events[i]
		if ev.Kind != "notification" || ev.Method != "textDocument/publishDiagnostics" {
			continue
		}
		var params publishDiagnosticsParams
		if err := json.Unmarshal(ev.Params, &params); err != nil {
			continue
		}
		if params.URI == uri {
			return params
		}
	}
	return publishDiagnosticsParams{}
}

func (f *sourceGoneFixture) renameSource(t *testing.T) {
	t.Helper()
	if err := os.Rename(f.oldSourcePath, f.newSourcePath); err != nil {
		t.Fatalf("rename source: %v", err)
	}
}

func (f *sourceGoneFixture) notifySourceRenamed() {
	f.h.t.Helper()
	f.changeWatchedFiles(
		watchedFileEvent{URI: f.oldSourceURI, Type: watchedFileDeleted},
		watchedFileEvent{URI: f.newSourceURI, Type: watchedFileCreated},
	)
}

func (f *sourceGoneFixture) waitForOldTargetRemoved(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(sourceGoneRemovalTimeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(f.oldTargetPath); os.IsNotExist(err) {
			f.changeWatchedFiles(watchedFileEvent{URI: f.oldTargetURI, Type: watchedFileDeleted})
			return
		}
		time.Sleep(lspPollInterval)
	}
	t.Fatalf("old generated target was not removed within %s: %s", sourceGoneRemovalTimeout, f.oldTargetPath)
}

func (f *sourceGoneFixture) scrollPosition(t *testing.T, uri string) lspPosition {
	t.Helper()
	return positionOfSubstring(t, f.h.bufferText(uri), "Scroll()")
}

func (f *sourceGoneFixture) assertNewSourceIsClean(t *testing.T) {
	t.Helper()
	h := f.h

	start := h.checkpoint()
	h.openFile(f.newSourcePath, "gox")
	waitForFileContents(t, f.newTargetPath)
	f.changeWatchedFiles(watchedFileEvent{URI: f.newTargetURI, Type: watchedFileCreated})
	f.waitForDiagnosticsAfter(start, f.newSourceURI)

	deadline := time.Now().Add(sourceGoneSettleTimeout)
	var cleanSince time.Time
	var latest publishDiagnosticsParams
	for time.Now().Before(deadline) {
		latest = f.latestDiagnosticsAfter(start, f.newSourceURI)
		if len(sourceGoneRedeclared(latest)) != 0 {
			cleanSince = time.Time{}
		} else if cleanSince.IsZero() {
			cleanSince = time.Now()
		} else if time.Since(cleanSince) > sourceGoneSettleWindow {
			break
		}
		time.Sleep(lspPollInterval)
	}
	if dups := sourceGoneRedeclared(latest); len(dups) != 0 {
		t.Errorf("gopls reports duplicate declarations in the renamed file %s: %v", f.newSourceURI, dups)
	}

	refs := h.call("textDocument/references", map[string]any{
		"textDocument": map[string]any{"uri": f.newSourceURI},
		"position":     f.scrollPosition(t, f.newSourceURI),
		"context":      map[string]any{"includeDeclaration": true},
	})
	if refs.Err != nil {
		t.Errorf("references on the renamed file failed: %v", refs.Err)
	}
	if !strings.Contains(string(refs.Result), "scroll2.gox") {
		t.Errorf("references did not return the declaration in the renamed file: %s", compactJSON(refs.Result))
	}
	if !strings.Contains(string(refs.Result), "user.go") {
		t.Errorf("references did not return the usage in user.go: %s", compactJSON(refs.Result))
	}
	sourceGoneAssertNoStaleLocation(t, "references", refs.Result)

	symbols := h.call("workspace/symbol", map[string]any{"query": "Scroll"})
	if symbols.Err != nil {
		t.Errorf("workspace/symbol failed: %v", symbols.Err)
	}
	if !strings.Contains(string(symbols.Result), "scroll2.") {
		t.Errorf("workspace/symbol did not return the symbol in the renamed file: %s", compactJSON(symbols.Result))
	}
	sourceGoneAssertNoStaleLocation(t, "workspace/symbol", symbols.Result)

	if _, err := os.Stat(f.oldTargetPath); !os.IsNotExist(err) {
		t.Errorf("old generated target is on disk: %s (stat err: %v)", f.oldTargetPath, err)
	}
}

func sourceGoneRedeclared(params publishDiagnosticsParams) []string {
	var found []string
	for _, diag := range params.Diagnostics {
		if strings.Contains(diag.Message, "redeclared") {
			found = append(found, diag.Message)
		}
	}
	return found
}

func sourceGoneAssertNoStaleLocation(t *testing.T, method string, raw json.RawMessage) {
	t.Helper()
	for _, stale := range []string{"/scroll.gox", "/scroll.x.go"} {
		if strings.Contains(string(raw), stale) {
			t.Errorf("%s returned a stale location in %s: %s", method, stale, compactJSON(raw))
		}
	}
}

func TestLSPSourceRenamedCloseBeforeTickerNotices(t *testing.T) {
	f := newSourceGoneFixture(t)
	h := f.h

	h.openFile(f.oldSourcePath, "gox")
	f.waitForDiagnosticsAfter(0, f.oldSourceURI)
	waitForFileContents(t, f.oldTargetPath)

	f.renameSource(t)
	h.closeFile(f.oldSourcePath)
	f.notifySourceRenamed()
	f.waitForOldTargetRemoved(t)

	f.assertNewSourceIsClean(t)
}

func TestLSPSourceRenamedCloseAfterTickerNoticed(t *testing.T) {
	f := newSourceGoneFixture(t)
	h := f.h

	h.openFile(f.oldSourcePath, "gox")
	f.waitForDiagnosticsAfter(0, f.oldSourceURI)
	waitForFileContents(t, f.oldTargetPath)

	f.renameSource(t)
	f.notifySourceRenamed()
	f.waitForOldTargetRemoved(t)
	h.closeFile(f.oldSourcePath)

	f.assertNewSourceIsClean(t)
}

func TestLSPSourceRenamedOnlyTargetBufferOpenNeverClosed(t *testing.T) {
	f := newSourceGoneFixture(t)
	h := f.h

	hover := h.call("textDocument/hover", map[string]any{
		"textDocument": map[string]any{"uri": f.oldSourceURI},
		"position":     positionOfSubstring(t, sourceGoneScrollSource, "Scroll()"),
	})
	if hover.Err != nil {
		t.Fatalf("hover on the unopened source failed: %v", hover.Err)
	}
	waitForFileContents(t, f.oldTargetPath)

	h.openFile(f.oldTargetPath, "go")
	f.waitForDiagnosticsAfter(0, f.oldTargetURI)

	f.renameSource(t)
	f.notifySourceRenamed()
	f.waitForOldTargetRemoved(t)

	f.assertNewSourceIsClean(t)
}

func TestLSPSourceGoneOpenBufferKeepsDocument(t *testing.T) {
	f := newSourceGoneFixture(t)
	h := f.h

	h.openFile(f.oldSourcePath, "gox")
	f.waitForDiagnosticsAfter(0, f.oldSourceURI)
	waitForFileContents(t, f.oldTargetPath)

	f.renameSource(t)
	f.notifySourceRenamed()
	f.waitForOldTargetRemoved(t)
	time.Sleep(time.Second)

	position := f.scrollPosition(t, f.oldSourceURI)
	hover := h.call("textDocument/hover", map[string]any{
		"textDocument": map[string]any{"uri": f.oldSourceURI},
		"position":     position,
	})
	if hover.Err != nil {
		t.Fatalf("hover on the still-open old buffer failed: %v", hover.Err)
	}
	if !strings.Contains(string(hover.Result), "func Scroll() string") {
		t.Errorf("hover on the still-open old buffer returned no signature: %s", compactJSON(hover.Result))
	}

	definition := h.call("textDocument/definition", map[string]any{
		"textDocument": map[string]any{"uri": f.oldSourceURI},
		"position":     position,
	})
	if definition.Err != nil {
		t.Fatalf("definition on the still-open old buffer failed: %v", definition.Err)
	}
	if !strings.Contains(string(definition.Result), "/scroll.gox") {
		t.Errorf("definition on the still-open old buffer did not resolve into it: %s", compactJSON(definition.Result))
	}
}
