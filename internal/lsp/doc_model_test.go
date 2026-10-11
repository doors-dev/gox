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
)

const docModelEviction = 3 * time.Second

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

type docModelRig struct {
	t      *testing.T
	bridge *docModelBridge
	router Router
}

func newDocModelRig(t *testing.T, roots ...string) *docModelRig {
	t.Helper()
	bridge := &docModelBridge{}
	router := NewRouter(bridge)
	t.Cleanup(router.Stop)
	router.session.setEnc(common.UTF8)
	uris := make([]string, 0, len(roots))
	for _, root := range roots {
		uris = append(uris, docModelURI(root))
	}
	ensureRouterWorkspaces(router, uris...)
	return &docModelRig{t: t, bridge: bridge, router: router}
}

func docModelURI(path string) string {
	return string(docpath.URIFromPath(path))
}

func docModelWrite(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (r *docModelRig) send(m method, params any) {
	r.t.Helper()
	data, err := json.Marshal(params)
	if err != nil {
		r.t.Fatal(err)
	}
	r.router.Notification(Client, Request{Method: string(m), Params: data})
}

func (r *docModelRig) open(path string, lang string, version int, text string) {
	r.t.Helper()
	r.send(didOpen, map[string]any{
		"textDocument": map[string]any{
			"uri":        docModelURI(path),
			"languageId": lang,
			"version":    version,
			"text":       text,
		},
	})
}

func (r *docModelRig) close(path string) {
	r.t.Helper()
	r.send(didClose, map[string]any{
		"textDocument": map[string]any{"uri": docModelURI(path)},
	})
}

func (r *docModelRig) changeFull(path string, version int, text string) {
	r.t.Helper()
	r.send(didChange, map[string]any{
		"textDocument":   map[string]any{"uri": docModelURI(path), "version": version},
		"contentChanges": []map[string]any{{"text": text}},
	})
}

func (r *docModelRig) changeRange(path string, version int, line int, char int, text string) {
	r.t.Helper()
	pos := map[string]any{"line": line, "character": char}
	r.send(didChange, map[string]any{
		"textDocument": map[string]any{"uri": docModelURI(path), "version": version},
		"contentChanges": []map[string]any{{
			"range": map[string]any{"start": pos, "end": pos},
			"text":  text,
		}},
	})
}

func (r *docModelRig) folders(added []string, removed []string) {
	r.t.Helper()
	conv := func(paths []string) []map[string]any {
		out := make([]map[string]any, 0, len(paths))
		for _, p := range paths {
			out = append(out, map[string]any{"uri": docModelURI(p), "name": filepath.Base(p)})
		}
		return out
	}
	r.send(didChangeWorkspaceFolders, map[string]any{
		"event": map[string]any{"added": conv(added), "removed": conv(removed)},
	})
}

func (r *docModelRig) goplsNotifs(m method, path string) []Request {
	uri := docModelURI(path)
	var found []Request
	for _, n := range r.bridge.snapshot() {
		if n.role != Gopls || n.req.Method != string(m) {
			continue
		}
		var payload struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		if err := json.Unmarshal(n.req.Params, &payload); err != nil {
			continue
		}
		if payload.TextDocument.URI == uri {
			found = append(found, n.req)
		}
	}
	return found
}

func (r *docModelRig) clientErrors() []string {
	var found []string
	for _, n := range r.bridge.snapshot() {
		if n.role != Client {
			continue
		}
		var payload struct {
			Message string `json:"message"`
			Type    int    `json:"type"`
		}
		if err := json.Unmarshal(n.req.Params, &payload); err != nil || payload.Message == "" {
			continue
		}
		found = append(found, payload.Message)
	}
	return found
}

func (r *docModelRig) waitCount(m method, path string, want int) int {
	r.t.Helper()
	deadline := time.Now().Add(docModelEviction)
	got := 0
	for time.Now().Before(deadline) {
		got = len(r.goplsNotifs(m, path))
		if got >= want {
			return got
		}
		time.Sleep(20 * time.Millisecond)
	}
	return got
}

func (r *docModelRig) settle() {
	time.Sleep(900 * time.Millisecond)
}

func (r *docModelRig) tracked(path string) (opened bool, exists bool) {
	man := r.router.session.man()
	man.Lock()
	defer man.Unlock()
	doc, _ := man.Doc(docModelURI(path))
	if doc == nil || doc.Err() != nil {
		return false, false
	}
	return doc.SourceIsOpened(), true
}

func waitGone(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(docModelEviction)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s is still on disk", path)
}

func waitPresent(t *testing.T, path string) string {
	t.Helper()
	deadline := time.Now().Add(docModelEviction)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(path); err == nil && len(data) != 0 {
			return string(data)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s never appeared on disk", path)
	return ""
}

func TestDocModelGhostTargetCloseWithSiblingIsIgnored(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "pkg", "view.gox")
	target := filepath.Join(root, "pkg", "view.x.go")
	docModelWrite(t, source, lspHelperSource)
	docModelWrite(t, filepath.Join(root, "pkg", "other.gox"), "package demo\n")
	r := newDocModelRig(t, root)

	r.open(target, "go", 1, waitPresent(t, target))
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	if got := r.waitCount(didClose, target, 1); got != 1 {
		t.Fatalf("ghost didClose reached gopls %d times, want 1", got)
	}
	r.settle()
	r.close(target)
	r.settle()
	if got := len(r.goplsNotifs(didClose, target)); got != 1 {
		t.Fatalf("gopls received %d didClose for the ghost target, want exactly 1", got)
	}
}

func TestDocModelSourceCloseAfterParentFolderAddedStillReachesGopls(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	source := filepath.Join(sub, "view.gox")
	target := filepath.Join(sub, "view.x.go")
	docModelWrite(t, source, lspHelperSource)
	r := newDocModelRig(t, sub)

	r.open(source, "gox", 1, lspHelperSource)
	if got := len(r.goplsNotifs(didOpen, target)); got != 1 {
		t.Fatalf("didOpen reached gopls %d times, want 1", got)
	}
	r.folders([]string{root}, nil)
	r.close(source)
	if got := len(r.goplsNotifs(didClose, target)); got != 1 {
		t.Fatalf("gopls received %d didClose after the source buffer closed, want 1 (overlay leaks: S closed, T closed)", got)
	}
}

func TestDocModelSourceCloseAfterFolderReAddedStillReachesGopls(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "view.gox")
	target := filepath.Join(root, "view.x.go")
	docModelWrite(t, source, lspHelperSource)
	r := newDocModelRig(t, root)

	r.open(source, "gox", 1, lspHelperSource)
	r.folders(nil, []string{root})
	r.folders([]string{root}, nil)
	r.close(source)
	if got := len(r.goplsNotifs(didClose, target)); got != 1 {
		t.Fatalf("gopls received %d didClose after the source buffer closed, want 1 (overlay leaks: S closed, T closed)", got)
	}
}

func TestDocModelTargetCloseAfterFolderAddedStillReachesGopls(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	source := filepath.Join(root, "view.gox")
	target := filepath.Join(root, "view.x.go")
	docModelWrite(t, source, lspHelperSource)
	seed := newDocModelRig(t, root)
	generated := waitPresent(t, target)
	seed.router.Stop()

	r := newDocModelRig(t, other)
	r.open(target, "go", 1, generated)
	if got := len(r.goplsNotifs(didOpen, target)); got != 1 {
		t.Fatalf("target didOpen reached gopls %d times, want 1", got)
	}
	r.folders([]string{root}, nil)
	r.close(target)
	if got := len(r.goplsNotifs(didClose, target)); got != 1 {
		t.Fatalf("gopls received %d didClose after the target buffer closed, want 1 (overlay leaks)", got)
	}
}

func TestDocModelBufferCreatedDocumentLifecycle(t *testing.T) {
	root := t.TempDir()
	docModelWrite(t, filepath.Join(root, "go.mod"), "module example.com/x\n")
	source := filepath.Join(root, "fresh", "view.gox")
	target := filepath.Join(root, "fresh", "view.x.go")
	r := newDocModelRig(t, root)

	r.open(source, "gox", 1, lspHelperSource)
	opens := r.goplsNotifs(didOpen, target)
	if len(opens) != 1 {
		t.Fatalf("didOpen reached gopls %d times, want 1; client messages: %v", len(opens), r.clientErrors())
	}
	if !strings.Contains(string(opens[0].Params), "package demo") {
		t.Fatalf("gopls overlay does not carry the buffer text: %s", opens[0].Params)
	}
	r.settle()
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("generated target exists on disk for an unsaved buffer: %v", err)
	}
	if opened, exists := r.tracked(source); !exists || !opened {
		t.Fatalf("buffer-created document was dropped while its buffer is open (exists=%v opened=%v)", exists, opened)
	}
	edited := strings.Replace(lspHelperSource, "Title string", "Title string\n\tBody string", 1)
	r.changeFull(source, 2, edited)
	changes := r.goplsNotifs(didChange, target)
	if len(changes) != 1 || !strings.Contains(string(changes[0].Params), "Body string") {
		t.Fatalf("edit of a buffer-created document did not reach gopls: %v; client messages: %v", changes, r.clientErrors())
	}

	docModelWrite(t, source, edited)
	r.send(didSave, map[string]any{"textDocument": map[string]any{"uri": docModelURI(source)}})
	if got := waitPresent(t, target); !strings.Contains(got, "Body string") {
		t.Fatalf("saved target does not hold the buffer content:\n%s", got)
	}

	r.close(source)
	if got := len(r.goplsNotifs(didClose, target)); got != 1 {
		t.Fatalf("gopls received %d didClose, want 1", got)
	}
	r.settle()
	if _, exists := r.tracked(source); !exists {
		t.Fatal("document for an on-disk source vanished after close")
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("target of an on-disk source is missing: %v", err)
	}
}

func TestDocModelBufferCreatedThenClosedUnsavedIsDropped(t *testing.T) {
	root := t.TempDir()
	docModelWrite(t, filepath.Join(root, "keep.gox"), "package demo\n")
	source := filepath.Join(root, "view.gox")
	target := filepath.Join(root, "view.x.go")
	r := newDocModelRig(t, root)

	r.open(source, "gox", 1, lspHelperSource)
	r.settle()
	r.close(source)
	if got := len(r.goplsNotifs(didClose, target)); got != 1 {
		t.Fatalf("gopls received %d didClose, want 1", got)
	}
	r.settle()
	if _, exists := r.tracked(source); exists {
		t.Fatal("document survives with no file and no buffer")
	}
	if got := len(r.goplsNotifs(didClose, target)); got != 1 {
		t.Fatalf("gopls received %d didClose after drop, want 1", got)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target exists for a never-saved source: %v", err)
	}
	r.close(source)
	r.close(target)
	if got := len(r.goplsNotifs(didClose, target)); got != 1 {
		t.Fatalf("stray didClose reached gopls: %d", got)
	}
}

func TestDocModelKeptDocumentSurvivesSourceRemoval(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "view.gox")
	target := filepath.Join(root, "view.x.go")
	docModelWrite(t, source, lspHelperSource)
	docModelWrite(t, filepath.Join(root, "other.gox"), "package demo\n")
	r := newDocModelRig(t, root)

	r.open(source, "gox", 1, lspHelperSource)
	generated := waitPresent(t, target)
	r.open(target, "go", 1, generated)
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	waitGone(t, target)
	r.settle()
	if opened, exists := r.tracked(source); !exists || !opened {
		t.Fatalf("document dropped while its source buffer is open (exists=%v opened=%v)", exists, opened)
	}
	if got := len(r.goplsNotifs(didClose, target)); got != 0 {
		t.Fatalf("gopls received %d didClose while the source buffer is open", got)
	}
	edited := strings.Replace(lspHelperSource, "Title string", "Title string\n\tBody string", 1)
	r.changeFull(source, 2, edited)
	if changes := r.goplsNotifs(didChange, target); len(changes) != 1 {
		t.Fatalf("edit of a kept document did not reach gopls; client messages: %v", r.clientErrors())
	}
	r.settle()
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target on disk while the source file is absent: %v", err)
	}

	r.close(source)
	if got := r.waitCount(didClose, target, 1); got != 1 {
		t.Fatalf("gopls received %d didClose after source closed with ghost target open, want 1", got)
	}
	r.settle()
	r.close(target)
	r.settle()
	if got := len(r.goplsNotifs(didClose, target)); got != 1 {
		t.Fatalf("gopls received %d didClose in total, want exactly 1", got)
	}
}

func TestDocModelKeptDocumentSourceReappears(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "view.gox")
	target := filepath.Join(root, "view.x.go")
	docModelWrite(t, source, lspHelperSource)
	r := newDocModelRig(t, root)

	r.open(source, "gox", 1, lspHelperSource)
	waitPresent(t, target)
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	waitGone(t, target)
	edited := strings.Replace(lspHelperSource, "Title string", "Title string\n\tBody string", 1)
	r.changeFull(source, 2, edited)
	docModelWrite(t, source, "package demo\n")
	if got := waitPresent(t, target); !strings.Contains(got, "Body string") {
		t.Fatalf("target was not rewritten from the buffer:\n%s", got)
	}
	r.close(source)
	if got := len(r.goplsNotifs(didClose, target)); got != 1 {
		t.Fatalf("gopls received %d didClose, want 1", got)
	}
	if got := len(r.goplsNotifs(didOpen, target)); got != 1 {
		t.Fatalf("gopls received %d didOpen, want 1", got)
	}
}

func TestDocModelOpenCloseChurnAgainstTicker(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "view.gox")
	target := filepath.Join(root, "view.x.go")
	ghostSource := filepath.Join(root, "ghost.gox")
	ghostTarget := filepath.Join(root, "ghost.x.go")
	r := newDocModelRig(t, root)

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
				_ = os.WriteFile(source, []byte(lspHelperSource), 0o644)
				_ = os.WriteFile(ghostSource, []byte(lspHelperSource), 0o644)
			} else {
				_ = os.Remove(source)
				_ = os.Remove(ghostSource)
			}
			time.Sleep(60 * time.Millisecond)
		}
	}()
	deadline := time.Now().Add(2500 * time.Millisecond)
	version := 1
	for time.Now().Before(deadline) {
		r.open(source, "gox", version, lspHelperSource)
		r.open(ghostTarget, "go", version, "package demo\n")
		version++
		r.changeFull(source, version, lspHelperSource+"\n// x\n")
		r.open(target, "go", version, "package demo\n")
		r.close(target)
		r.close(source)
		r.close(ghostTarget)
		time.Sleep(15 * time.Millisecond)
	}
	close(stop)
	<-done
	_ = os.Remove(source)
	_ = os.Remove(ghostSource)
	r.settle()
	opens := len(r.goplsNotifs(didOpen, target))
	closes := len(r.goplsNotifs(didClose, target))
	if opens != closes {
		t.Fatalf("unbalanced gopls overlay for %s: %d didOpen vs %d didClose", target, opens, closes)
	}
}
