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

const modelWait = 1500 * time.Millisecond
const modelSettle = 700 * time.Millisecond

type lockedBridgeStub struct {
	mu            sync.Mutex
	notifications []recordedNotify
	calls         []recordedCall
}

func (b *lockedBridgeStub) Call(role Role, call Request, callback Callback) {
	b.mu.Lock()
	b.calls = append(b.calls, recordedCall{role: role, req: call})
	b.mu.Unlock()
	callback(Response{Result: json.RawMessage(`{"applied":true}`)})
}

func (b *lockedBridgeStub) Notify(role Role, notification Request) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.notifications = append(b.notifications, recordedNotify{role: role, req: notification})
	return nil
}

func (b *lockedBridgeStub) snapshot() []recordedNotify {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]recordedNotify(nil), b.notifications...)
}

type modelEnv struct {
	t      *testing.T
	root   string
	router Router
	bridge *lockedBridgeStub
	source string
	target string
}

func newModelEnv(t *testing.T, withSource bool) *modelEnv {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	env := &modelEnv{
		t:      t,
		root:   root,
		source: filepath.Join(root, "view.gox"),
		target: filepath.Join(root, "view.x.go"),
		bridge: &lockedBridgeStub{},
	}
	if withSource {
		if err := os.WriteFile(env.source, []byte(lspHelperSource), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	env.router = NewRouter(env.bridge)
	t.Cleanup(env.router.Stop)
	env.router.session.setEnc(common.UTF8)
	ensureRouterWorkspaces(env.router, string(docpath.URIFromPath(root)))
	return env
}

func modelURI(path string) string {
	return string(docpath.URIFromPath(path))
}

func (e *modelEnv) notify(m method, params map[string]any) {
	e.t.Helper()
	data, err := json.Marshal(params)
	if err != nil {
		e.t.Fatal(err)
	}
	e.router.Notification(Client, Request{Method: string(m), Params: data})
}

func (e *modelEnv) open(path string, text string) {
	e.t.Helper()
	lang := "gox"
	if strings.HasSuffix(path, ".go") {
		lang = "go"
	}
	e.notify(didOpen, map[string]any{
		"textDocument": map[string]any{
			"uri":        modelURI(path),
			"languageId": lang,
			"version":    1,
			"text":       text,
		},
	})
}

func (e *modelEnv) openTarget() {
	e.t.Helper()
	content, err := os.ReadFile(e.target)
	if err != nil {
		e.t.Fatalf("generated target is missing: %v", err)
	}
	e.open(e.target, string(content))
}

func (e *modelEnv) close(path string) {
	e.t.Helper()
	e.notify(didClose, map[string]any{
		"textDocument": map[string]any{"uri": modelURI(path)},
	})
}

func notifURI(n recordedNotify) string {
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

func (e *modelEnv) goplsCount(m method, path string) int {
	uri := modelURI(path)
	count := 0
	for _, n := range e.bridge.snapshot() {
		if n.role == Gopls && n.req.Method == string(m) && notifURI(n) == uri {
			count++
		}
	}
	return count
}

func (e *modelEnv) goplsAll() []string {
	var out []string
	for _, n := range e.bridge.snapshot() {
		if n.role == Gopls {
			out = append(out, n.req.Method+" "+notifURI(n))
		}
	}
	return out
}

func (e *modelEnv) assertOverlay(step string, wantOpens int, wantCloses int) {
	e.t.Helper()
	opens := e.goplsCount(didOpen, e.target)
	closes := e.goplsCount(didClose, e.target)
	if opens != wantOpens || closes != wantCloses {
		e.t.Fatalf("%s: gopls overlay for .x.go got didOpen=%d didClose=%d, want didOpen=%d didClose=%d; gopls traffic: %v",
			step, opens, closes, wantOpens, wantCloses, e.goplsAll())
	}
	if n := e.goplsCount(didOpen, e.source) + e.goplsCount(didClose, e.source); n != 0 {
		e.t.Fatalf("%s: %d open/close notifications leaked to gopls with the .gox URI: %v", step, n, e.goplsAll())
	}
}

func (e *modelEnv) sync() {
	man := e.router.session.man()
	man.Lock()
	man.Unlock()
}

func (e *modelEnv) waitFor(what string, cond func() bool) {
	e.t.Helper()
	deadline := time.Now().Add(modelWait)
	for {
		if cond() {
			e.sync()
			return
		}
		if time.Now().After(deadline) {
			e.t.Fatalf("timed out waiting for %s; gopls traffic: %v", what, e.goplsAll())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (e *modelEnv) deleteSourceAndWaitTicker() {
	e.t.Helper()
	if !fileExists(e.target) {
		e.t.Fatalf("precondition: %s should exist while the source is on disk", e.target)
	}
	if err := os.Remove(e.source); err != nil {
		e.t.Fatal(err)
	}
	e.waitFor(".x.go removal by the ticker", func() bool { return !fileExists(e.target) })
}

func (e *modelEnv) settle() {
	time.Sleep(modelSettle)
	e.sync()
}

type docState struct {
	doc          workspace.Doc
	err          error
	sourceOpened bool
	content      string
}

func (e *modelEnv) trackedDoc() *docState {
	man := e.router.session.man()
	man.Lock()
	defer man.Unlock()
	doc, _ := man.Doc(modelURI(e.source))
	if doc == nil {
		return nil
	}
	state := &docState{doc: doc, err: doc.Err(), sourceOpened: doc.SourceIsOpened()}
	if state.err == nil {
		state.content = doc.TargetContent()
	}
	return state
}

func TestModelCloseSourceAfterFileDeleted(t *testing.T) {
	env := newModelEnv(t, true)
	env.open(env.source, lspHelperSource)
	env.assertOverlay("after source open", 1, 0)

	env.deleteSourceAndWaitTicker()
	env.settle()
	env.assertOverlay("file deleted, source buffer still open", 1, 0)

	env.close(env.source)
	env.assertOverlay("after source close", 1, 1)
	env.settle()
	env.assertOverlay("after ticker dropped the document", 1, 1)
}

func TestModelCloseSourceThenDeleteFile(t *testing.T) {
	env := newModelEnv(t, true)
	env.open(env.source, lspHelperSource)
	env.assertOverlay("after source open", 1, 0)

	env.close(env.source)
	env.assertOverlay("after source close", 1, 1)

	env.deleteSourceAndWaitTicker()
	env.settle()
	env.assertOverlay("after file deleted", 1, 1)
}

func TestModelTargetOnlyGhostClosedOnceByTicker(t *testing.T) {
	env := newModelEnv(t, true)
	env.openTarget()
	env.assertOverlay("after target open", 1, 0)

	if err := os.Remove(env.source); err != nil {
		t.Fatal(err)
	}
	env.waitFor("ticker didClose of the ghost .x.go", func() bool {
		return env.goplsCount(didClose, env.target) == 1
	})
	if fileExists(env.target) {
		t.Fatalf(".x.go still on disk after its source vanished")
	}
	env.settle()
	env.assertOverlay("after ticker ghosted the document", 1, 1)
}

func TestModelSourceAndTargetOpenCloseSourceFirst(t *testing.T) {
	env := newModelEnv(t, true)
	env.open(env.source, lspHelperSource)
	env.openTarget()
	env.assertOverlay("after source and target open", 1, 0)

	env.deleteSourceAndWaitTicker()
	env.settle()
	env.assertOverlay("file deleted, source buffer still open", 1, 0)

	env.close(env.source)
	env.waitFor("overlay close after the source buffer closed", func() bool {
		return env.goplsCount(didClose, env.target) >= 1
	})
	env.settle()
	env.assertOverlay("after source close and ticker", 1, 1)
}

func TestModelSourceAndTargetOpenCloseTargetFirst(t *testing.T) {
	env := newModelEnv(t, true)
	env.open(env.source, lspHelperSource)
	env.openTarget()
	env.assertOverlay("after source and target open", 1, 0)

	env.deleteSourceAndWaitTicker()
	env.settle()
	env.assertOverlay("file deleted, both buffers open", 1, 0)

	env.close(env.target)
	env.assertOverlay("after target close", 1, 0)

	env.close(env.source)
	env.assertOverlay("after source close", 1, 1)
	env.settle()
	env.assertOverlay("after ticker dropped the document", 1, 1)
}

func TestModelBufferStaysAuthoritativeWhileFileGoneAndBack(t *testing.T) {
	env := newModelEnv(t, true)
	env.open(env.source, lspHelperSource)
	env.deleteSourceAndWaitTicker()
	env.settle()

	lines := strings.Split(lspHelperSource, "\n")
	line, col := -1, -1
	for i, l := range lines {
		if c := strings.Index(l, "card"); c >= 0 {
			line, col = i, c
		}
	}
	if line < 0 {
		t.Fatal("marker not found in helper source")
	}
	env.notify(didChange, map[string]any{
		"textDocument": map[string]any{"uri": modelURI(env.source), "version": 2},
		"contentChanges": []map[string]any{
			{
				"range": map[string]any{
					"start": map[string]any{"line": line, "character": col},
					"end":   map[string]any{"line": line, "character": col + len("card")},
				},
				"text": "frombuffer",
			},
		},
	})

	var change *recordedNotify
	for _, n := range env.bridge.snapshot() {
		if n.role == Gopls && n.req.Method == string(didChange) {
			n := n
			change = &n
		}
	}
	if change == nil {
		t.Fatalf("ranged didChange for a deleted-but-open source was not forwarded to gopls: %v", env.goplsAll())
	}
	if got := notifURI(*change); got != modelURI(env.target) {
		t.Fatalf("didChange forwarded with uri %q, want %q", got, modelURI(env.target))
	}
	var payload struct {
		ContentChanges []struct {
			Text  string          `json:"text"`
			Range json.RawMessage `json:"range"`
		} `json:"contentChanges"`
	}
	if err := json.Unmarshal(change.req.Params, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.ContentChanges) != 1 {
		t.Fatalf("forwarded didChange has %d changes, want 1: %s", len(payload.ContentChanges), change.req.Params)
	}
	forwarded := payload.ContentChanges[0].Text
	if !strings.Contains(forwarded, "frombuffer") || strings.Contains(forwarded, `"card"`) {
		t.Fatalf("forwarded content was not generated from the patched buffer:\n%s", forwarded)
	}
	doc := env.trackedDoc()
	if doc == nil || doc.err != nil {
		t.Fatalf("document lost or errored while its buffer is open: doc=%v", doc)
	}
	if forwarded != doc.content {
		t.Fatalf("forwarded content differs from the document target content")
	}
	if fileExists(env.target) {
		t.Fatalf(".x.go exists on disk while the source file is gone")
	}

	diskSource := strings.Replace(lspHelperSource, "card", "fromdisk", 1)
	if err := os.WriteFile(env.source, []byte(diskSource), 0o644); err != nil {
		t.Fatal(err)
	}
	env.waitFor(".x.go to be recreated", func() bool { return fileExists(env.target) })
	env.settle()

	doc = env.trackedDoc()
	if doc == nil || doc.err != nil {
		t.Fatalf("document lost or errored after the source reappeared: doc=%v", doc)
	}
	if !doc.sourceOpened {
		t.Fatal("document is no longer marked as source-open after the file reappeared")
	}
	content := doc.content
	if !strings.Contains(content, "frombuffer") || strings.Contains(content, "fromdisk") {
		t.Fatalf("document content no longer equals the buffer:\n%s", content)
	}
	onDisk, err := os.ReadFile(env.target)
	if err != nil {
		t.Fatal(err)
	}
	if string(onDisk) != content {
		t.Fatalf(".x.go on disk was not written from the buffer:\n%s", onDisk)
	}
	env.assertOverlay("file gone and back with the source buffer open", 1, 0)

	env.close(env.source)
	env.settle()
	env.assertOverlay("after source close", 1, 1)
}

func TestModelDidCloseUntrackedSourceDoesNotPanic(t *testing.T) {
	env := newModelEnv(t, true)

	outsideDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(outsideDir, "other.gox")
	env.close(outside)
	for _, n := range env.bridge.snapshot() {
		if n.role == Gopls && strings.HasSuffix(notifURI(n), ".x.go") {
			t.Fatalf("didClose for a .gox outside any workspace reached gopls as a .x.go close: %s %s", n.req.Method, n.req.Params)
		}
	}
	outsideTraffic := len(env.bridge.snapshot())
	t.Logf("traffic after didClose of an out-of-workspace .gox: %v", env.goplsAll())

	emptyDir := filepath.Join(env.root, "empty")
	if err := os.Mkdir(emptyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	env.close(filepath.Join(emptyDir, "nothing.gox"))
	env.close(filepath.Join(env.root, "missingdir", "nothing.gox"))
	env.close(filepath.Join(env.root, "untracked.gox"))
	env.settle()
	after := env.bridge.snapshot()
	if len(after) != outsideTraffic {
		t.Fatalf("didClose for untracked in-workspace .gox files produced traffic: %#v", after[outsideTraffic:])
	}
	env.assertOverlay("unrelated closes", 0, 0)
}

func TestModelDidCloseOutsideWorkspaceSendsNothing(t *testing.T) {
	env := newModelEnv(t, true)
	outsideDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	env.close(filepath.Join(outsideDir, "other.gox"))
	if got := env.bridge.snapshot(); len(got) != 0 {
		t.Fatalf("didClose for a .gox outside any workspace produced traffic: %#v", got)
	}
}

func TestModelDidCloseReachesGoplsDespiteStickyError(t *testing.T) {
	env := newModelEnv(t, true)
	env.open(env.source, lspHelperSource)
	env.assertOverlay("after source open", 1, 0)

	env.notify(didChange, map[string]any{
		"textDocument": map[string]any{"uri": modelURI(env.source), "version": 2},
		"contentChanges": []map[string]any{
			{
				"range": map[string]any{
					"start": map[string]any{"line": 5, "character": 0},
					"end":   map[string]any{"line": 1, "character": 0},
				},
				"text": "junk",
			},
		},
	})
	doc := env.trackedDoc()
	if doc == nil {
		t.Fatal("document vanished after an invalid edit")
	}
	if doc.err == nil {
		t.Skip("could not force a sticky document error through client notifications")
	}

	env.close(env.source)
	env.assertOverlay("after closing an errored document", 1, 1)
	if after := env.trackedDoc(); after == nil || after.sourceOpened {
		t.Fatalf("errored document is still marked source-open after didClose: %v", after)
	}

	if err := os.Remove(env.source); err != nil {
		t.Fatal(err)
	}
	env.waitFor(".x.go removal by the ticker", func() bool { return !fileExists(env.target) })
	env.settle()
	env.assertOverlay("after the errored document was dropped", 1, 1)
}

func TestModelDidCloseErroredTargetOnlyReachesGopls(t *testing.T) {
	env := newModelEnv(t, true)
	env.open(env.source, lspHelperSource)
	env.openTarget()
	env.notify(didChange, map[string]any{
		"textDocument": map[string]any{"uri": modelURI(env.source), "version": 2},
		"contentChanges": []map[string]any{
			{
				"range": map[string]any{
					"start": map[string]any{"line": 5, "character": 0},
					"end":   map[string]any{"line": 1, "character": 0},
				},
				"text": "junk",
			},
		},
	})
	doc := env.trackedDoc()
	if doc == nil || doc.err == nil {
		t.Skip("could not force a sticky document error through client notifications")
	}
	env.close(env.source)
	env.assertOverlay("errored, source closed, target open", 1, 0)
	env.close(env.target)
	env.assertOverlay("errored, both closed", 1, 1)
}

func TestModelUnsavedSourceOpenCreatesDocumentFromBuffer(t *testing.T) {
	env := newModelEnv(t, false)
	env.open(env.source, lspHelperSource)

	var opened *recordedNotify
	for _, n := range env.bridge.snapshot() {
		if n.role == Gopls && n.req.Method == string(didOpen) && notifURI(n) == modelURI(env.target) {
			n := n
			opened = &n
		}
	}
	if opened == nil {
		t.Fatalf("didOpen for an unsaved .gox did not open the .x.go overlay in gopls: %#v", env.bridge.snapshot())
	}
	var payload struct {
		TextDocument struct {
			Text string `json:"text"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(opened.req.Params, &payload); err != nil {
		t.Fatal(err)
	}
	doc := env.trackedDoc()
	if doc == nil || doc.err != nil {
		t.Fatalf("no healthy document for an open unsaved buffer: doc=%v", doc)
	}
	if !doc.sourceOpened {
		t.Fatal("unsaved document is not marked source-open")
	}
	if payload.TextDocument.Text != doc.content || !strings.Contains(payload.TextDocument.Text, "card") {
		t.Fatalf("overlay content was not generated from the buffer text:\n%s", payload.TextDocument.Text)
	}
	env.settle()
	if fileExists(env.target) {
		t.Fatal(".x.go was written to disk for a source that was never saved")
	}
	if got := env.trackedDoc(); got == nil || got.doc != doc.doc {
		t.Fatal("ticker dropped the document of an open unsaved buffer")
	}
	env.assertOverlay("unsaved buffer open", 1, 0)

	env.close(env.source)
	env.assertOverlay("unsaved buffer closed", 1, 1)
	env.settle()
	env.assertOverlay("after ticker dropped the unsaved document", 1, 1)
	if fileExists(env.target) {
		t.Fatal(".x.go appeared on disk after the unsaved buffer closed")
	}
}
