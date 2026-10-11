package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/doors-dev/gox/internal/common"
	"github.com/doors-dev/gox/internal/docpath"
	"github.com/doors-dev/gox/internal/workspace"
)

const (
	docModelTick   = 250 * time.Millisecond
	docModelSettle = 3 * docModelTick
	docModelWait   = 10 * time.Second
	docModelPoll   = 10 * time.Millisecond
)

type docModelBridge struct {
	mu            sync.Mutex
	notifications []recordedNotify
	calls         []recordedCall
}

func (b *docModelBridge) Call(role Role, call Request, callback Callback) {
	b.mu.Lock()
	b.calls = append(b.calls, recordedCall{role: role, req: call})
	b.mu.Unlock()
	go callback(Response{Result: json.RawMessage(`{"applied":true}`)})
}

func (b *docModelBridge) Notify(role Role, notification Request) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.notifications = append(b.notifications, recordedNotify{role: role, req: notification})
	return nil
}

func (b *docModelBridge) snapshot() []recordedNotify {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]recordedNotify(nil), b.notifications...)
}

type docModelEnv struct {
	t      *testing.T
	root   string
	source string
	target string
	bridge *docModelBridge
	router Router
}

type docModelState struct {
	doc          workspace.Doc
	err          error
	sourceOpened bool
	content      string
}

func docModelTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func newDocModelEnv(t *testing.T, dir ...string) *docModelEnv {
	t.Helper()
	root := docModelTempDir(t)
	pkg := filepath.Join(append([]string{root}, dir...)...)
	return &docModelEnv{
		t:      t,
		root:   root,
		source: filepath.Join(pkg, "view.gox"),
		target: filepath.Join(pkg, "view.x.go"),
	}
}

func docModelURI(path string) string {
	return string(docpath.URIFromPath(path))
}

func docModelNotifURI(n recordedNotify) string {
	var payload struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(n.req.Params, &payload); err != nil {
		return ""
	}
	return payload.TextDocument.URI
}

func docModelExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (e *docModelEnv) start(workspaces ...string) {
	e.t.Helper()
	e.bridge = &docModelBridge{}
	e.router = NewRouter(e.bridge)
	e.t.Cleanup(e.router.Stop)
	e.router.session.setEnc(common.UTF8)
	uris := make([]string, 0, len(workspaces))
	for _, dir := range workspaces {
		uris = append(uris, docModelURI(dir))
	}
	ensureRouterWorkspaces(e.router, uris...)
}

func (e *docModelEnv) write(path string, content string) {
	e.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

func (e *docModelEnv) notify(m method, params any) {
	e.t.Helper()
	data, err := json.Marshal(params)
	if err != nil {
		e.t.Fatal(err)
	}
	e.router.Notification(Client, Request{Method: string(m), Params: data})
}

func (e *docModelEnv) open(path string, text string) {
	e.t.Helper()
	lang := "gox"
	if strings.HasSuffix(path, ".go") {
		lang = "go"
	}
	e.notify(didOpen, map[string]any{
		"textDocument": map[string]any{
			"uri":        docModelURI(path),
			"languageId": lang,
			"version":    1,
			"text":       text,
		},
	})
}

func (e *docModelEnv) openTarget() {
	e.t.Helper()
	e.open(e.target, e.waitTarget(""))
}

func (e *docModelEnv) close(path string) {
	e.t.Helper()
	e.notify(didClose, map[string]any{
		"textDocument": map[string]any{"uri": docModelURI(path)},
	})
}

func (e *docModelEnv) changeFull(path string, version int, text string) {
	e.t.Helper()
	e.notify(didChange, map[string]any{
		"textDocument":   map[string]any{"uri": docModelURI(path), "version": version},
		"contentChanges": []map[string]any{{"text": text}},
	})
}

func (e *docModelEnv) changeRange(path string, version int, start [2]int, end [2]int, text string) {
	e.t.Helper()
	e.notify(didChange, map[string]any{
		"textDocument": map[string]any{"uri": docModelURI(path), "version": version},
		"contentChanges": []map[string]any{{
			"range": map[string]any{
				"start": map[string]any{"line": start[0], "character": start[1]},
				"end":   map[string]any{"line": end[0], "character": end[1]},
			},
			"text": text,
		}},
	})
}

func (e *docModelEnv) folders(added []string, removed []string) {
	e.t.Helper()
	conv := func(paths []string) []map[string]any {
		out := make([]map[string]any, 0, len(paths))
		for _, p := range paths {
			out = append(out, map[string]any{"uri": docModelURI(p), "name": filepath.Base(p)})
		}
		return out
	}
	e.notify(didChangeWorkspaceFolders, map[string]any{
		"event": map[string]any{"added": conv(added), "removed": conv(removed)},
	})
}

func (e *docModelEnv) gopls(m method, path string) []Request {
	uri := docModelURI(path)
	var found []Request
	for _, n := range e.bridge.snapshot() {
		if n.role == Gopls && n.req.Method == string(m) && docModelNotifURI(n) == uri {
			found = append(found, n.req)
		}
	}
	return found
}

func (e *docModelEnv) traffic() []string {
	var out []string
	for _, n := range e.bridge.snapshot() {
		if n.role == Gopls {
			out = append(out, "gopls "+n.req.Method+" "+docModelNotifURI(n))
			continue
		}
		var payload struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(n.req.Params, &payload); err == nil && payload.Message != "" {
			out = append(out, "client "+n.req.Method+" "+payload.Message)
		}
	}
	return out
}

func (e *docModelEnv) assertOverlay(step string, wantOpens int, wantCloses int) {
	e.t.Helper()
	opens := len(e.gopls(didOpen, e.target))
	closes := len(e.gopls(didClose, e.target))
	if opens != wantOpens || closes != wantCloses {
		e.t.Fatalf("%s: gopls received didOpen=%d didClose=%d for the target, want didOpen=%d didClose=%d; traffic: %v",
			step, opens, closes, wantOpens, wantCloses, e.traffic())
	}
	if n := len(e.gopls(didOpen, e.source)) + len(e.gopls(didClose, e.source)); n != 0 {
		e.t.Fatalf("%s: gopls received %d open/close notifications with the source URI; traffic: %v", step, n, e.traffic())
	}
}

func (e *docModelEnv) sync() {
	man := e.router.session.man()
	man.Lock()
	man.Unlock()
}

func (e *docModelEnv) waitFor(what string, cond func() bool) {
	e.t.Helper()
	deadline := time.Now().Add(docModelWait)
	for !cond() {
		if time.Now().After(deadline) {
			e.t.Fatalf("timed out waiting for %s; traffic: %v", what, e.traffic())
		}
		time.Sleep(docModelPoll)
	}
	e.sync()
}

func (e *docModelEnv) settle() {
	time.Sleep(docModelSettle)
	e.sync()
}

func (e *docModelEnv) waitTarget(fragment string) string {
	e.t.Helper()
	var content string
	e.waitFor("the target on disk containing "+fragment, func() bool {
		data, err := os.ReadFile(e.target)
		content = string(data)
		return err == nil && len(data) != 0 && strings.Contains(content, fragment)
	})
	return content
}

func (e *docModelEnv) waitTargetGone() {
	e.t.Helper()
	e.waitFor("the target to be removed from disk", func() bool { return !docModelExists(e.target) })
}

func (e *docModelEnv) removeSource() {
	e.t.Helper()
	e.waitTarget("")
	if err := os.Remove(e.source); err != nil {
		e.t.Fatal(err)
	}
	e.waitTargetGone()
}

func (e *docModelEnv) state(path string) *docModelState {
	man := e.router.session.man()
	man.Lock()
	defer man.Unlock()
	doc, _ := man.Doc(docModelURI(path))
	if doc == nil {
		return nil
	}
	state := &docModelState{doc: doc, err: doc.Err(), sourceOpened: doc.SourceIsOpened()}
	if state.err == nil {
		state.content = doc.TargetContent()
	}
	return state
}

func (e *docModelEnv) healthyState(step string) *docModelState {
	e.t.Helper()
	state := e.state(e.source)
	if state == nil {
		e.t.Fatalf("%s: no document is tracked for the source; traffic: %v", step, e.traffic())
	}
	if state.err != nil {
		e.t.Fatalf("%s: document carries an error: %v", step, state.err)
	}
	return state
}

func (e *docModelEnv) forceDocumentError() {
	e.t.Helper()
	e.changeRange(e.source, 2, [2]int{5, 0}, [2]int{1, 0}, "junk")
	state := e.state(e.source)
	if state == nil {
		e.t.Fatal("document is no longer tracked after an invalid edit")
	}
	if state.err == nil {
		e.t.Fatal("an edit with an inverted range did not put the document into its error state")
	}
}

func docModelEdited() string {
	return strings.Replace(lspHelperSource, "Title string", "Title string\n\tBody string", 1)
}

func TestDocModelCloseSourceAfterFileDeleted(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t)
	env.write(env.source, lspHelperSource)
	env.start(env.root)

	env.open(env.source, lspHelperSource)
	env.assertOverlay("after source open", 1, 0)

	env.removeSource()
	env.settle()
	env.assertOverlay("file deleted, source buffer still open", 1, 0)

	env.close(env.source)
	env.assertOverlay("after source close", 1, 1)
	env.settle()
	env.assertOverlay("after the document was dropped", 1, 1)
}

func TestDocModelCloseSourceThenDeleteFile(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t)
	env.write(env.source, lspHelperSource)
	env.start(env.root)

	env.open(env.source, lspHelperSource)
	env.assertOverlay("after source open", 1, 0)

	env.close(env.source)
	env.assertOverlay("after source close", 1, 1)

	env.removeSource()
	env.settle()
	env.assertOverlay("after file deleted", 1, 1)
}

func TestDocModelTargetOnlyGhostClosedOnceByTicker(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t)
	env.write(env.source, lspHelperSource)
	env.start(env.root)

	env.openTarget()
	env.assertOverlay("after target open", 1, 0)

	if err := os.Remove(env.source); err != nil {
		t.Fatal(err)
	}
	env.waitFor("the ticker to close the target overlay", func() bool {
		return len(env.gopls(didClose, env.target)) == 1
	})
	env.waitTargetGone()
	env.settle()
	env.assertOverlay("after the source file vanished", 1, 1)
}

func TestDocModelGhostTargetCloseWithSiblingIsIgnored(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t, "pkg")
	env.write(env.source, lspHelperSource)
	env.write(filepath.Join(env.root, "pkg", "other.gox"), "package demo\n")
	env.start(env.root)

	env.openTarget()
	if err := os.Remove(env.source); err != nil {
		t.Fatal(err)
	}
	env.waitFor("the ticker to close the target overlay", func() bool {
		return len(env.gopls(didClose, env.target)) == 1
	})
	env.settle()
	env.assertOverlay("after the source file vanished", 1, 1)
	env.close(env.target)
	env.settle()
	env.assertOverlay("after the target buffer closed", 1, 1)
}

func TestDocModelSourceAndTargetOpenCloseSourceFirst(t *testing.T) {
	t.Parallel()
	t.Run("Alone", func(t *testing.T) { testDocModelSourceAndTargetOpenCloseSourceFirst(t, false) })
	t.Run("WithSibling", func(t *testing.T) { testDocModelSourceAndTargetOpenCloseSourceFirst(t, true) })
}

func testDocModelSourceAndTargetOpenCloseSourceFirst(t *testing.T, sibling bool) {
	t.Parallel()
	env := newDocModelEnv(t)
	env.write(env.source, lspHelperSource)
	if sibling {
		env.write(filepath.Join(env.root, "other.gox"), "package demo\n")
	}
	env.start(env.root)

	env.open(env.source, lspHelperSource)
	env.openTarget()
	env.assertOverlay("after source and target open", 1, 0)

	env.removeSource()
	env.settle()
	if state := env.healthyState("file deleted, both buffers open"); !state.sourceOpened {
		t.Fatal("document is not marked source-open while its source buffer is open")
	}
	env.assertOverlay("file deleted, both buffers open", 1, 0)

	env.changeFull(env.source, 2, docModelEdited())
	if changes := env.gopls(didChange, env.target); len(changes) != 1 {
		t.Fatalf("gopls received %d didChange for an edit of a document without a source file, want 1; traffic: %v", len(changes), env.traffic())
	}
	env.settle()
	if docModelExists(env.target) {
		t.Fatal("target is on disk while the source file is absent")
	}

	env.close(env.source)
	env.waitFor("the overlay close after the source buffer closed", func() bool {
		return len(env.gopls(didClose, env.target)) >= 1
	})
	env.settle()
	env.assertOverlay("after source close", 1, 1)
	if !sibling {
		return
	}
	env.close(env.target)
	env.settle()
	env.assertOverlay("after target close", 1, 1)
}

func TestDocModelSourceAndTargetOpenCloseTargetFirst(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t)
	env.write(env.source, lspHelperSource)
	env.start(env.root)

	env.open(env.source, lspHelperSource)
	env.openTarget()
	env.assertOverlay("after source and target open", 1, 0)

	env.removeSource()
	env.settle()
	env.assertOverlay("file deleted, both buffers open", 1, 0)

	env.close(env.target)
	env.assertOverlay("after target close", 1, 0)

	env.close(env.source)
	env.assertOverlay("after source close", 1, 1)
	env.settle()
	env.assertOverlay("after the document was dropped", 1, 1)
}

func TestDocModelBufferStaysAuthoritativeWhileFileGoneAndBack(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t)
	env.write(env.source, lspHelperSource)
	env.start(env.root)

	env.open(env.source, lspHelperSource)
	env.removeSource()
	env.settle()

	line, col := -1, -1
	for i, l := range strings.Split(lspHelperSource, "\n") {
		if c := strings.Index(l, "card"); c >= 0 {
			line, col = i, c
		}
	}
	if line < 0 {
		t.Fatal("marker not found in helper source")
	}
	env.changeRange(env.source, 2, [2]int{line, col}, [2]int{line, col + len("card")}, "frombuffer")

	if n := len(env.gopls(didChange, env.source)); n != 0 {
		t.Fatalf("gopls received %d didChange with the source URI", n)
	}
	changes := env.gopls(didChange, env.target)
	if len(changes) != 1 {
		t.Fatalf("gopls received %d didChange for a ranged edit of a document without a source file, want 1; traffic: %v", len(changes), env.traffic())
	}
	var payload struct {
		ContentChanges []struct {
			Text string `json:"text"`
		} `json:"contentChanges"`
	}
	if err := json.Unmarshal(changes[0].Params, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.ContentChanges) != 1 {
		t.Fatalf("forwarded didChange has %d changes, want 1: %s", len(payload.ContentChanges), changes[0].Params)
	}
	forwarded := payload.ContentChanges[0].Text
	if !strings.Contains(forwarded, "frombuffer") || strings.Contains(forwarded, `"card"`) {
		t.Fatalf("forwarded content was not generated from the edited buffer:\n%s", forwarded)
	}
	if state := env.healthyState("file deleted, source buffer edited"); forwarded != state.content {
		t.Fatal("forwarded content differs from the document target content")
	}
	if docModelExists(env.target) {
		t.Fatal("target is on disk while the source file is absent")
	}

	env.write(env.source, strings.Replace(lspHelperSource, "card", "fromdisk", 1))
	env.waitTarget("frombuffer")
	env.settle()

	state := env.healthyState("source file back on disk")
	if !state.sourceOpened {
		t.Fatal("document is not marked source-open after the file reappeared")
	}
	if !strings.Contains(state.content, "frombuffer") || strings.Contains(state.content, "fromdisk") {
		t.Fatalf("document content does not equal the buffer:\n%s", state.content)
	}
	onDisk, err := os.ReadFile(env.target)
	if err != nil {
		t.Fatal(err)
	}
	if string(onDisk) != state.content {
		t.Fatalf("target on disk was not written from the buffer:\n%s", onDisk)
	}
	env.assertOverlay("file gone and back with the source buffer open", 1, 0)

	env.close(env.source)
	env.assertOverlay("after source close", 1, 1)
	env.settle()
	env.assertOverlay("after source close and ticker", 1, 1)
}

func TestDocModelDidCloseUntrackedSource(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t)
	env.write(env.source, lspHelperSource)
	env.start(env.root)

	env.close(filepath.Join(docModelTempDir(t), "other.gox"))
	if got := env.bridge.snapshot(); len(got) != 0 {
		t.Fatalf("didClose for a source outside any workspace produced traffic: %#v", got)
	}

	emptyDir := filepath.Join(env.root, "empty")
	if err := os.Mkdir(emptyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	env.close(filepath.Join(emptyDir, "nothing.gox"))
	env.close(filepath.Join(env.root, "missingdir", "nothing.gox"))
	env.close(filepath.Join(env.root, "untracked.gox"))
	env.settle()
	if got := env.bridge.snapshot(); len(got) != 0 {
		t.Fatalf("didClose for untracked sources inside the workspace produced traffic: %#v", got)
	}
	env.assertOverlay("after closing untracked sources", 0, 0)
}

func TestDocModelDidCloseReachesGoplsDespiteStickyError(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t)
	env.write(env.source, lspHelperSource)
	env.start(env.root)

	env.open(env.source, lspHelperSource)
	env.assertOverlay("after source open", 1, 0)
	env.forceDocumentError()

	env.close(env.source)
	env.assertOverlay("after closing an errored document", 1, 1)
	if after := env.state(env.source); after == nil || after.sourceOpened {
		t.Fatalf("errored document is still marked source-open after didClose: %v", after)
	}

	env.removeSource()
	env.settle()
	env.assertOverlay("after the errored document was dropped", 1, 1)
}

func TestDocModelDidCloseErroredTargetOnlyReachesGopls(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t)
	env.write(env.source, lspHelperSource)
	env.start(env.root)

	env.open(env.source, lspHelperSource)
	env.openTarget()
	env.forceDocumentError()

	env.close(env.source)
	env.assertOverlay("errored, source closed, target open", 1, 0)
	env.close(env.target)
	env.assertOverlay("errored, both closed", 1, 1)
}

func TestDocModelUnsavedSourceOpenCreatesDocumentFromBuffer(t *testing.T) {
	t.Parallel()
	t.Run("Alone", func(t *testing.T) { testDocModelUnsavedSourceOpenCreatesDocumentFromBuffer(t, false) })
	t.Run("WithSibling", func(t *testing.T) { testDocModelUnsavedSourceOpenCreatesDocumentFromBuffer(t, true) })
}

func testDocModelUnsavedSourceOpenCreatesDocumentFromBuffer(t *testing.T, sibling bool) {
	t.Parallel()
	env := newDocModelEnv(t)
	if sibling {
		env.write(filepath.Join(env.root, "keep.gox"), "package demo\n")
	}
	env.start(env.root)

	env.open(env.source, lspHelperSource)
	env.assertOverlay("unsaved buffer open", 1, 0)
	var payload struct {
		TextDocument struct {
			Text string `json:"text"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(env.gopls(didOpen, env.target)[0].Params, &payload); err != nil {
		t.Fatal(err)
	}
	state := env.healthyState("unsaved buffer open")
	if !state.sourceOpened {
		t.Fatal("unsaved document is not marked source-open")
	}
	if payload.TextDocument.Text != state.content || !strings.Contains(payload.TextDocument.Text, "card") {
		t.Fatalf("overlay content was not generated from the buffer text:\n%s", payload.TextDocument.Text)
	}
	env.settle()
	if docModelExists(env.target) {
		t.Fatal("target was written to disk for a source that was never saved")
	}
	if got := env.state(env.source); got == nil || got.doc != state.doc {
		t.Fatal("document of an open unsaved buffer was dropped")
	}
	env.assertOverlay("unsaved buffer open after ticker", 1, 0)

	env.close(env.source)
	env.assertOverlay("unsaved buffer closed", 1, 1)
	env.settle()
	if got := env.state(env.source); got != nil && got.err == nil {
		t.Fatal("document is still tracked with no file and no buffer")
	}
	env.assertOverlay("after the unsaved document was dropped", 1, 1)
	if docModelExists(env.target) {
		t.Fatal("target appeared on disk after the unsaved buffer closed")
	}
	if !sibling {
		return
	}

	env.close(env.source)
	env.close(env.target)
	env.assertOverlay("after repeated closes", 1, 1)
}

func TestDocModelBufferCreatedDocumentLifecycle(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t, "fresh")
	env.write(filepath.Join(env.root, "go.mod"), "module example.com/app\n")
	env.start(env.root)

	env.open(env.source, lspHelperSource)
	env.assertOverlay("unsaved buffer open", 1, 0)
	if opened := env.gopls(didOpen, env.target)[0]; !strings.Contains(string(opened.Params), "package demo") {
		t.Fatalf("gopls overlay does not carry the buffer text: %s", opened.Params)
	}
	env.settle()
	if docModelExists(env.target) {
		t.Fatal("target was written to disk for a source that was never saved")
	}
	if state := env.healthyState("unsaved buffer open after ticker"); !state.sourceOpened {
		t.Fatal("unsaved document is not marked source-open")
	}

	edited := docModelEdited()
	env.changeFull(env.source, 2, edited)
	changes := env.gopls(didChange, env.target)
	if len(changes) != 1 || !strings.Contains(string(changes[0].Params), "Body string") {
		t.Fatalf("edit of an unsaved document did not reach gopls: %v; traffic: %v", changes, env.traffic())
	}

	env.write(env.source, edited)
	env.notify(didSave, map[string]any{"textDocument": map[string]any{"uri": docModelURI(env.source)}})
	env.waitTarget("Body string")

	env.close(env.source)
	env.assertOverlay("after source close", 1, 1)
	env.settle()
	env.healthyState("saved source closed")
	if !docModelExists(env.target) {
		t.Fatal("target of a source on disk is missing")
	}
}

func TestDocModelBufferCreatedFromBlankLinesTracksBufferText(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t)
	env.start(env.root)

	env.open(env.source, "\n\n")
	env.changeRange(env.source, 2, [2]int{2, 0}, [2]int{2, 0}, "package demo\n\nfunc Marker() {}\n")
	buffer := "\n\npackage demo\n\nfunc Marker() {}\n"

	file, ok := workspace.NewFileFromURI(docModelURI(env.source))
	if !ok {
		t.Fatal("bad uri")
	}
	reference := workspace.NewDoc(file)
	reference.InitUnsaved()
	if _, err := reference.SourceUpdate(buffer); err != nil {
		t.Fatal(err)
	}
	defer reference.Close()

	var got string
	if state := env.state(env.source); state != nil {
		got = state.content
	}
	if got != reference.TargetContent() {
		t.Fatalf("document differs from the editor buffer.\nwant target (from buffer text %q):\n%s\ngot target:\n%s\ntraffic: %v", buffer, reference.TargetContent(), got, env.traffic())
	}
}

func TestDocModelSourceCloseAfterParentFolderAddedStillReachesGopls(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t, "sub")
	env.write(env.source, lspHelperSource)
	env.start(filepath.Join(env.root, "sub"))

	env.open(env.source, lspHelperSource)
	env.assertOverlay("after source open", 1, 0)
	env.folders([]string{env.root}, nil)
	env.close(env.source)
	env.assertOverlay("source closed after the parent folder was added", 1, 1)
}

func TestDocModelSourceCloseAfterFolderReAddedStillReachesGopls(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t)
	env.write(env.source, lspHelperSource)
	env.start(env.root)

	env.open(env.source, lspHelperSource)
	env.folders(nil, []string{env.root})
	env.folders([]string{env.root}, nil)
	env.close(env.source)
	if got := len(env.gopls(didClose, env.target)); got != 1 {
		t.Fatalf("gopls received %d didClose for a source closed after its folder was removed and added again, want 1; traffic: %v", got, env.traffic())
	}
}

func TestDocModelTargetCloseAfterFolderAddedStillReachesGopls(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t)
	env.write(env.source, lspHelperSource)
	env.start(env.root)
	generated := env.waitTarget("")
	env.router.Stop()

	env.start(docModelTempDir(t))
	env.open(env.target, generated)
	env.assertOverlay("target opened outside any workspace", 1, 0)
	env.folders([]string{env.root}, nil)
	env.close(env.target)
	env.assertOverlay("target closed after its folder was added", 1, 1)
}

func TestDocModelOpenCloseChurnAgainstTicker(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t)
	env.start(env.root)

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			if i%2 == 0 {
				_ = os.WriteFile(env.source, []byte(lspHelperSource), 0o644)
			} else {
				_ = os.Remove(env.source)
			}
			time.Sleep(60 * time.Millisecond)
		}
	}()
	deadline := time.Now().Add(6 * docModelTick)
	for version := 2; time.Now().Before(deadline); version++ {
		env.open(env.source, lspHelperSource)
		env.changeFull(env.source, version, lspHelperSource+"\nfunc Extra() {}\n")
		env.open(env.target, "package demo\n")
		env.close(env.target)
		env.close(env.source)
		time.Sleep(15 * time.Millisecond)
	}
	close(stop)
	<-done
	_ = os.Remove(env.source)

	balance := func() (int, int) {
		return len(env.gopls(didOpen, env.target)), len(env.gopls(didClose, env.target))
	}
	env.waitFor("equal didOpen and didClose counts for the target", func() bool {
		opens, closes := balance()
		return opens == closes
	})
	env.settle()
	opens, closes := balance()
	if opens == 0 {
		t.Fatal("gopls received no didOpen for the target of a source that was opened repeatedly")
	}
	if opens != closes {
		t.Fatalf("gopls received %d didOpen and %d didClose for the target, want equal counts", opens, closes)
	}
	env.assertOverlay("after churn", opens, closes)
}
