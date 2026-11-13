package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// assertOverlayLegal replays gox's gopls traffic for the target: didChange,
// didSave and didClose need an open overlay, and the overlay must end open
// iff wantOpen.
func (e *docModelEnv) assertOverlayLegal(step string, wantOpen bool) {
	e.t.Helper()
	uri := docModelURI(e.target)
	open := false
	for _, n := range e.bridge.snapshot() {
		if n.role != Gopls || docModelNotifURI(n) != uri {
			continue
		}
		switch method(n.req.Method) {
		case didOpen:
			open = true
		case didClose, didChange, didSave:
			if !open {
				e.t.Fatalf("%s: gopls got %s for the target without an overlay; traffic: %v", step, n.req.Method, e.traffic())
			}
			if method(n.req.Method) == didClose {
				open = false
			}
		}
	}
	if open != wantOpen {
		e.t.Fatalf("%s: gopls overlay open=%v, want %v; traffic: %v", step, open, wantOpen, e.traffic())
	}
}

func (e *docModelEnv) save(path string) {
	e.t.Helper()
	e.notify(didSave, map[string]any{"textDocument": map[string]any{"uri": docModelURI(path)}})
}

// lastGoplsText returns the generated text of gox's last didOpen or didChange
// of the target.
func (e *docModelEnv) lastGoplsText() string {
	e.t.Helper()
	text := ""
	for _, n := range e.bridge.snapshot() {
		if n.role != Gopls || docModelNotifURI(n) != docModelURI(e.target) {
			continue
		}
		var p struct {
			TextDocument struct {
				Text string `json:"text"`
			} `json:"textDocument"`
			ContentChanges []struct {
				Text string `json:"text"`
			} `json:"contentChanges"`
		}
		_ = json.Unmarshal(n.req.Params, &p)
		switch method(n.req.Method) {
		case didOpen:
			text = p.TextDocument.Text
		case didChange:
			text = p.ContentChanges[len(p.ContentChanges)-1].Text
		}
	}
	return text
}

func (e *docModelEnv) clientMessage(fragment string) bool {
	for _, line := range e.traffic() {
		if strings.HasPrefix(line, "client ") && strings.Contains(line, fragment) {
			return true
		}
	}
	return false
}

// titleLine returns the position of "Title" in the helper source.
func titleLine() (int, int) {
	for i, l := range strings.Split(lspHelperSource, "\n") {
		if c := strings.Index(l, "Title string"); c >= 0 {
			return i, c
		}
	}
	panic("Title not in helper source")
}

// insertField adds a field before Title, as an unsaved ranged edit.
func (e *docModelEnv) insertField(version int, field string) {
	line, col := titleLine()
	e.changeRange(e.source, version, [2]int{line, col}, [2]int{line, col}, field+" string\n\t")
}

// Removing the folder of an open .gox keeps its document and overlay
// until the buffers close; from then on it is not served, even before the
// ticker releases it.
func TestDocModelFolderRemovedKeepsOpenDocument(t *testing.T) {
	t.Parallel()
	for _, targetFirst := range []bool{false, true} {
		env := newDocModelEnv(t)
		env.write(env.source, lspHelperSource)
		env.start(env.root)

		env.open(env.source, lspHelperSource)
		env.openTarget()
		env.insertField(2, "First")
		env.folders(nil, []string{env.root})
		env.insertField(3, "Second")
		if text := env.lastGoplsText(); !strings.Contains(text, "First string") || !strings.Contains(text, "Second string") {
			t.Fatalf("edit after the folder removal did not reach gopls from the buffer text:\n%s\ntraffic: %v", text, env.traffic())
		}
		if env.clientMessage("not part of the current workspace") {
			t.Fatalf("open buffer reported as outside the workspace; traffic: %v", env.traffic())
		}
		first, second := env.target, env.source
		if !targetFirst {
			first, second = second, first
		}
		env.close(first)
		env.assertOverlayLegal("first buffer closed", true)
		env.close(second)
		env.assertOverlayLegal("both buffers closed", false)
		if state := env.state(env.source); state != nil {
			t.Fatal("document of a removed folder is still served after its buffers closed")
		}
	}
}

// Adding a parent folder keeps the open document, its buffer text and
// its open state.
func TestDocModelParentFolderAddedKeepsBuffer(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t, "sub")
	env.write(env.source, lspHelperSource)
	env.start(filepath.Join(env.root, "sub"))

	env.open(env.source, lspHelperSource)
	env.insertField(2, "First")
	env.openTarget()
	before := env.state(env.source).doc
	env.folders([]string{env.root}, nil)
	if after := env.state(env.source); after == nil || after.doc != before || !after.sourceOpened {
		t.Fatalf("the parent folder replaced the open document: %+v", after)
	}
	env.insertField(3, "Second")
	if text := env.lastGoplsText(); !strings.Contains(text, "First string") || !strings.Contains(text, "Second string") {
		t.Fatalf("gopls did not get the buffer text after the parent folder was added:\n%s", text)
	}
	env.close(env.target)
	env.assertOverlayLegal("target closed", true)
	env.close(env.source)
	env.assertOverlayLegal("source closed", false)
}

// One event replacing a folder by its child keeps the child served.
func TestDocModelFolderReplacedByChildInOneEvent(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t, "sub")
	env.write(env.source, lspHelperSource)
	env.start(env.root)

	env.open(env.source, lspHelperSource)
	env.folders([]string{filepath.Join(env.root, "sub")}, []string{env.root})
	if roots := env.router.session.rootsLocked(); len(roots) != 1 || roots[0] != filepath.Join(env.root, "sub") {
		t.Fatalf("roots = %v, want the child folder", roots)
	}
	env.insertField(2, "First")
	if !strings.Contains(env.lastGoplsText(), "First string") {
		t.Fatalf("edit did not reach gopls; traffic: %v", env.traffic())
	}
	env.close(env.source)
	env.assertOverlayLegal("source closed", false)
}

// After the ghost closed the overlay, the .gox comes back on disk; the
// editor's later saves (unchanged, then edited) of the .x.go buffer must not
// reach gopls. (Its close does: gox cannot tell it from the close of a .x.go
// it passed to gopls as is, and gopls drops it.)
func TestDocModelGhostThenSourceRestored(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t)
	env.write(env.source, lspHelperSource)
	env.start(env.root)

	env.openTarget()
	if err := os.Remove(env.source); err != nil {
		t.Fatal(err)
	}
	env.waitFor("the ghost close", func() bool {
		return len(env.gopls(didClose, env.target)) == 1
	})
	env.write(env.source, lspHelperSource)
	env.save(env.target)
	env.write(env.target, "package demo\n")
	env.save(env.target)
	if data, _ := os.ReadFile(env.target); string(data) == "package demo\n" {
		t.Fatal("the edited .x.go was not reverted on save")
	}
	env.assertOverlayLegal("ghosted target saved", false)
}

// A .gox opened outside every folder stays untracked once a folder
// covers it: its edits are refused and gopls gets nothing for it.
func TestDocModelSourceOpenedOutsideThenCovered(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t)
	env.write(env.source, lspHelperSource)
	env.start(docModelTempDir(t))

	env.open(env.source, lspHelperSource)
	env.insertField(2, "Unsaved")
	env.folders([]string{env.root}, nil)
	diskTarget := env.waitTarget("")
	env.insertField(3, "Second")
	env.save(env.source)
	env.close(env.source)
	env.settle()
	env.assertOverlayLegal("untracked buffer edited, saved and closed", false)
	if state := env.state(env.source); state == nil || strings.Contains(state.content, "Second") {
		t.Fatal("an edit of an untracked buffer was applied to the document")
	}
	if data, _ := os.ReadFile(env.target); string(data) != diskTarget {
		t.Fatal("saving an untracked buffer rewrote the .x.go")
	}
	if !env.clientMessage("Close and reopen the file") {
		t.Fatalf("the refused edit was not reported; traffic: %v", env.traffic())
	}
}

// Such an untracked buffer gets no formatting: it would be built from the
// file, not from the buffer. Reopened, it gets formatting.
func TestDocModelUntrackedSourceGetsNoFormatting(t *testing.T) {
	t.Parallel()
	const src = "package demo\n\nimport \"github.com/doors-dev/gox\"\n\nfunc   Old()  int { return 1 + 2 }\n\nelem View() {\n\t<div><p>b</p></div>\n}\n"
	env := newDocModelEnv(t)
	env.write(env.source, src)
	env.start(docModelTempDir(t))

	env.open(env.source, src)
	env.folders([]string{env.root}, nil)
	env.waitTarget("")
	refusals := func() int {
		n := 0
		for _, line := range env.traffic() {
			if strings.HasPrefix(line, "client "+string(showMessage)) && strings.Contains(line, "Close and reopen the file") {
				n++
			}
		}
		return n
	}
	env.changeFull(env.source, 2, src+"\nfunc   Added()  int { return 2 }\n")
	if refusals() != 1 {
		t.Fatalf("the edit of the untracked buffer was not refused; traffic: %v", env.traffic())
	}
	format := func() Response {
		return env.call(Client, formatting, map[string]any{
			"textDocument": map[string]any{"uri": docModelURI(env.source)},
			"options":      map[string]any{"tabSize": 4, "insertSpaces": false},
		})
	}
	if r := format(); r.Err == nil || refusals() != 2 {
		t.Fatalf("formatting an untracked buffer: %s, error %v", r.Result, r.Err)
	}

	env.close(env.source)
	env.open(env.source, src)
	if r := format(); r.Err != nil || !strings.Contains(string(r.Result), "func Old() int") {
		t.Fatalf("formatting the reopened buffer: %s, error %v", r.Result, r.Err)
	}
}

func (e *docModelEnv) goplsEditVersion(version int) string {
	e.t.Helper()
	content := e.healthyState("before gopls edit").content
	line, col := -1, -1
	for i, l := range strings.Split(content, "\n") {
		if c := strings.Index(l, "Title string"); c >= 0 {
			line, col = i, c
			break
		}
	}
	data, err := json.Marshal(map[string]any{"edit": map[string]any{"documentChanges": []map[string]any{{
		"textDocument": map[string]any{"uri": docModelURI(e.target), "version": version},
		"edits": []map[string]any{{
			"range": map[string]any{
				"start": map[string]any{"line": line, "character": col},
				"end":   map[string]any{"line": line, "character": col + len("Title")},
			},
			"newText": "Name",
		}},
	}}}})
	if err != nil {
		e.t.Fatal(err)
	}
	before := len(e.bridge.callSnapshot())
	e.router.Call(Gopls, Request{Method: string(applyEdit), Params: data}, func(Response) {})
	calls := e.bridge.callSnapshot()
	if len(calls) != before+1 {
		e.t.Fatalf("gopls applyEdit was not proxied to the client; traffic: %v", e.traffic())
	}
	var p struct {
		Edit struct {
			DocumentChanges []struct {
				TextDocument struct {
					URI     string          `json:"uri"`
					Version json.RawMessage `json:"version"`
				} `json:"textDocument"`
			} `json:"documentChanges"`
		} `json:"edit"`
	}
	if err := json.Unmarshal(calls[before].req.Params, &p); err != nil || len(p.Edit.DocumentChanges) != 1 || p.Edit.DocumentChanges[0].TextDocument.URI != docModelURI(e.source) {
		e.t.Fatalf("edit was not mapped onto the source: %s", calls[before].req.Params)
	}
	return string(p.Edit.DocumentChanges[0].TextDocument.Version)
}

// gopls's version of the .x.go is the .gox buffer's only while the .gox
// is open. Closing an unchanged .gox resyncs nothing.
func TestDocModelGoplsEditVersion(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t)
	env.write(env.source, lspHelperSource)
	env.start(env.root)

	env.open(env.source, lspHelperSource)
	env.openTarget()
	if v := env.goplsEditVersion(7); v != "7" {
		t.Fatalf("source open: version = %s, want 7", v)
	}
	changes, calls := len(env.gopls(didChange, env.target)), len(env.bridge.callSnapshot())
	env.close(env.source)
	if len(env.gopls(didChange, env.target)) != changes || len(env.bridge.callSnapshot()) != calls {
		t.Fatalf("closing an unchanged .gox resynced the .x.go; traffic: %v", env.traffic())
	}
	if v := env.goplsEditVersion(7); v != "null" {
		t.Fatalf("source closed, target open: version = %s, want null", v)
	}
}

// Closing a .gox without saving returns the document, gopls and the
// .x.go buffer to the file on disk.
func TestDocModelDiscardedSourceReturnsToFile(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t)
	env.write(env.source, lspHelperSource)
	env.start(env.root)

	env.open(env.source, lspHelperSource)
	env.openTarget()
	env.insertField(2, "Zapped")
	env.close(env.source)
	if text := env.lastGoplsText(); strings.Contains(text, "Zapped") {
		t.Fatalf("gopls keeps the discarded code:\n%s", text)
	}
	if state := env.state(env.source); state == nil || strings.Contains(state.content, "Zapped") {
		t.Fatal("the document keeps the discarded text")
	}
	calls := env.bridge.callSnapshot()
	if last := calls[len(calls)-1]; last.req.Method != string(applyEdit) || !strings.Contains(string(last.req.Params), "view.x.go") || strings.Contains(string(last.req.Params), "Zapped") {
		t.Fatalf(".x.go buffer was not resynced after the discarded close: %q %q %s", last.role, last.req.Method, last.req.Params)
	}
	env.save(env.target)
	if data, _ := os.ReadFile(env.target); strings.Contains(string(data), "Zapped") {
		t.Fatal("saving the .x.go wrote the discarded code")
	}
	env.close(env.target)
	env.assertOverlayLegal("all closed", false)
}

// The stored diagnostics are published to the .x.go buffer only if the
// client applied the sync edit and the buffer is still open (not closed by
// the editor, nor by the ghost).
func TestDocModelTargetOpenDiagnosticsNeedAppliedSync(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"applied", "closed", "ghosted", "refused"} {
		env := newDocModelEnv(t)
		env.write(env.source, lspHelperSource)
		env.start(env.root)
		env.open(env.source, lspHelperSource)
		diag, _ := json.Marshal(map[string]any{"uri": docModelURI(env.target), "version": 1, "diagnostics": []any{}})
		env.router.Notification(Gopls, Request{Method: string(publishDiagnostics), Params: diag})

		env.bridge.mu.Lock()
		env.bridge.hold = true
		if mode == "refused" {
			env.bridge.result = json.RawMessage(`{"applied":false}`)
		}
		env.bridge.mu.Unlock()
		env.open(env.target, "package demo\n")
		switch mode {
		case "closed":
			env.close(env.target)
		case "ghosted":
			env.close(env.source)
			if err := os.Remove(env.source); err != nil {
				t.Fatal(err)
			}
			env.waitFor("the ghost close", func() bool {
				return len(env.gopls(didClose, env.target)) == 1
			})
		}
		env.bridge.release()
		env.sync()
		published := 0
		for _, n := range env.bridge.snapshot() {
			if n.role == Client && n.req.Method == string(publishDiagnostics) && strings.Contains(string(n.req.Params), "view.x.go") {
				published++
			}
		}
		want := 0
		if mode == "applied" {
			want = 1
		}
		if published != want {
			t.Fatalf("%s: stored .x.go diagnostics published %d times, want %d", mode, published, want)
		}
	}
}

// A parent folder keeps a buffer-only document (the .gox was never saved).
func TestDocModelParentFolderKeepsBufferOnlyDocument(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t, "sub")
	env.write(filepath.Join(env.root, "sub", "keep.gox"), "package demo\n")
	env.start(filepath.Join(env.root, "sub"))

	env.open(env.source, lspHelperSource)
	before := env.state(env.source)
	if before == nil || !before.sourceOpened {
		t.Fatal("buffer-only document not tracked")
	}
	env.folders([]string{env.root}, nil)
	env.insertField(2, "First")
	if after := env.state(env.source); after == nil || after.doc != before.doc || !strings.Contains(after.content, "First string") {
		t.Fatalf("parent folder lost the buffer-only document; traffic: %v", env.traffic())
	}
	if env.clientMessage("no longer exists") || env.clientMessage("not part of the current workspace") {
		t.Fatalf("error shown for an open buffer-only document; traffic: %v", env.traffic())
	}
	env.close(env.source)
	env.assertOverlayLegal("closed", false)
}

// Forgotten nested folder: folders R and R/sub, R removed: R/sub stays served.
func TestDocModelNestedFolderServedAfterParentRemoved(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t, "sub")
	env.write(env.source, lspHelperSource)
	env.start(env.root)
	env.folders([]string{filepath.Join(env.root, "sub")}, nil)
	env.folders(nil, []string{env.root})
	env.open(env.source, lspHelperSource)
	if env.clientMessage("not part of the current workspace") {
		t.Fatalf("child folder forgotten; traffic: %v", env.traffic())
	}
	env.assertOverlayLegal("open", true)
}

// Closing a .gox without saving, with no .x.go open: the document goes back
// to the file.
func TestDocModelDiscardedSourceWithoutTargetReturnsToFile(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t)
	env.write(env.source, lspHelperSource)
	env.start(env.root)
	env.open(env.source, lspHelperSource)
	env.insertField(2, "Zapped")
	env.close(env.source)
	if state := env.state(env.source); state == nil || strings.Contains(state.content, "Zapped") {
		t.Fatal("document keeps the discarded text")
	}
	env.openTarget()
	if strings.Contains(env.lastGoplsText(), "Zapped") {
		t.Fatal("a later .x.go open pushed the discarded code to gopls")
	}
	env.close(env.target)
	env.assertOverlayLegal("closed", false)
}

// With only the .gox open, removing its folder keeps the document ticked
// (deleting the .gox removes its .x.go), and its close reaches gopls.
func TestDocModelFolderRemovedSourceOnlyCloseReachesGopls(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t)
	env.write(env.source, lspHelperSource)
	env.start(env.root)
	env.open(env.source, lspHelperSource)
	env.folders(nil, []string{env.root})
	env.removeSource()
	env.close(env.source)
	env.assertOverlayLegal("closed", false)
}

// With only the .x.go open, removing its folder keeps the document (served,
// its buffer saved and closed through gox) until the buffer closes.
func TestDocModelFolderRemovedKeepsTargetOnlyDocument(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t)
	env.write(env.source, lspHelperSource)
	env.start(env.root)
	env.openTarget()
	env.folders(nil, []string{env.root})
	env.settle()
	if env.state(env.target) == nil {
		t.Fatal("document of an open .x.go buffer dropped with its folder")
	}
	env.save(env.target)
	if env.clientMessage("does not belong to the current workspace") {
		t.Fatalf("open .x.go buffer reported outside the workspace; traffic: %v", env.traffic())
	}
	env.close(env.target)
	env.assertOverlayLegal("target closed", false)
	env.folders([]string{env.root}, nil)
	env.open(env.source, lspHelperSource)
	env.assertOverlayLegal("source opened after the folder was re-added", true)
}
