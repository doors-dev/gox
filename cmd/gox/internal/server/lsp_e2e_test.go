package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/doors-dev/gox/cmd/gox/internal/dialers"
	serverpkg "github.com/doors-dev/gox/cmd/gox/internal/server"
	"github.com/doors-dev/gox/internal/docpath"
	jsonrpc2 "github.com/doors-dev/gox/internal/jsonrpc"
)

const (
	lspCallTimeout  = 30 * time.Second
	lspWaitTimeout  = 20 * time.Second
	lspPollInterval = 50 * time.Millisecond
)

type workspaceFolder struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

type lspPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type lspRange struct {
	Start lspPosition `json:"start"`
	End   lspPosition `json:"end"`
}

type lspDiagnostic struct {
	Range   lspRange `json:"range"`
	Message string   `json:"message"`
}

type publishDiagnosticsParams struct {
	URI         string          `json:"uri"`
	Diagnostics []lspDiagnostic `json:"diagnostics"`
}

type documentDiagnosticReport struct {
	Kind  string          `json:"kind"`
	Items []lspDiagnostic `json:"items"`
}

type documentLink struct {
	Range  lspRange `json:"range"`
	Target string   `json:"target"`
}

type openDocument struct {
	Path       string
	URI        string
	LanguageID string
	Text       string
	Version    int
}

type callResult struct {
	Result json.RawMessage
	Err    error
}

type recordedEvent struct {
	Kind   string
	Method string
	Params json.RawMessage
}

type eventMatcher struct {
	Kind   string
	Method string
	Pred   func(json.RawMessage) bool
}

type scriptListener struct {
	conns  chan io.ReadWriteCloser
	closed chan struct{}
	once   sync.Once
}

func newScriptListener() *scriptListener {
	return &scriptListener{
		conns:  make(chan io.ReadWriteCloser),
		closed: make(chan struct{}),
	}
}

func (l *scriptListener) Accept() (io.ReadWriteCloser, error) {
	select {
	case <-l.closed:
		return nil, errors.New("listener closed")
	case conn, ok := <-l.conns:
		if !ok {
			return nil, errors.New("listener closed")
		}
		return conn, nil
	}
}

func (l *scriptListener) Close() error {
	l.once.Do(func() {
		close(l.closed)
		close(l.conns)
	})
	return nil
}

func (l *scriptListener) Connect(t *testing.T) io.ReadWriteCloser {
	t.Helper()
	serverConn, clientConn := net.Pipe()
	select {
	case <-l.closed:
		t.Fatal("listener closed before the test client connected")
	case l.conns <- serverConn:
	}
	return clientConn
}

type lspHarness struct {
	t        *testing.T
	server   serverpkg.Server
	conn     io.ReadWriteCloser
	reader   jsonrpc2.Reader
	writer   jsonrpc2.Writer
	listener *scriptListener

	writeMu sync.Mutex

	nextID   int64
	pending  map[int64]chan callResult
	pendingM sync.Mutex

	events  []recordedEvent
	eventsM sync.Mutex

	docs  map[string]*openDocument
	docsM sync.Mutex

	folders  []workspaceFolder
	foldersM sync.Mutex

	readErr  error
	readErrM sync.Mutex

	closeOnce sync.Once
}

func newLSPHarness(t *testing.T, folders []workspaceFolder) *lspHarness {
	t.Helper()

	goplsPath, err := exec.LookPath("gopls")
	if err != nil {
		t.Skip("gopls is not installed")
	}

	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("create fake home: %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "xdg-cache"))
	t.Setenv("GOCACHE", filepath.Join(home, "go-build"))

	listener := newScriptListener()
	server := serverpkg.NewServer(listener, dialers.NewCmdDialer(goplsPath), 0)
	conn := listener.Connect(t)

	framer := jsonrpc2.HeaderFramer()
	h := &lspHarness{
		t:        t,
		server:   server,
		conn:     conn,
		reader:   framer.Reader(conn),
		writer:   framer.Writer(conn),
		listener: listener,
		pending:  make(map[int64]chan callResult),
		docs:     make(map[string]*openDocument),
		folders:  append([]workspaceFolder(nil), folders...),
	}
	go h.readLoop()

	t.Cleanup(func() {
		h.close()
	})

	return h
}

func (h *lspHarness) close() {
	h.closeOnce.Do(func() {
		_ = h.conn.Close()
		done := make(chan struct{})
		go func() {
			h.server.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			h.t.Fatalf("server did not stop in time\nrecorded events:\n%s", h.dumpEvents())
		}
	})
}

func (h *lspHarness) readLoop() {
	for {
		msg, err := h.reader.Read(context.Background())
		if err != nil {
			h.setReadErr(err)
			return
		}
		switch m := msg.(type) {
		case *jsonrpc2.Response:
			h.handleResponse(m)
		case *jsonrpc2.Request:
			if m.ID.IsValid() {
				h.recordEvent(recordedEvent{Kind: "call", Method: m.Method, Params: cloneRaw(m.Params)})
				h.handleServerCall(m)
				continue
			}
			h.recordEvent(recordedEvent{Kind: "notification", Method: m.Method, Params: cloneRaw(m.Params)})
		}
	}
}

func (h *lspHarness) setReadErr(err error) {
	h.readErrM.Lock()
	defer h.readErrM.Unlock()
	if h.readErr == nil {
		h.readErr = err
	}
}

func (h *lspHarness) getReadErr() error {
	h.readErrM.Lock()
	defer h.readErrM.Unlock()
	return h.readErr
}

func (h *lspHarness) recordEvent(ev recordedEvent) {
	h.eventsM.Lock()
	defer h.eventsM.Unlock()
	h.events = append(h.events, ev)
}

func (h *lspHarness) checkpoint() int {
	h.eventsM.Lock()
	defer h.eventsM.Unlock()
	return len(h.events)
}

func (h *lspHarness) dumpEvents() string {
	h.eventsM.Lock()
	defer h.eventsM.Unlock()
	if len(h.events) == 0 {
		return "(no events)"
	}
	var b strings.Builder
	for i, ev := range h.events {
		fmt.Fprintf(&b, "%03d %s %s %s\n", i, ev.Kind, ev.Method, compactJSON(ev.Params))
	}
	return b.String()
}

func (h *lspHarness) dumpEventsAfter(start int) string {
	h.eventsM.Lock()
	defer h.eventsM.Unlock()
	if start >= len(h.events) {
		return "(no new events)"
	}
	var b strings.Builder
	for i := start; i < len(h.events); i++ {
		ev := h.events[i]
		fmt.Fprintf(&b, "%03d %s %s %s\n", i, ev.Kind, ev.Method, compactJSON(ev.Params))
	}
	return b.String()
}

func (h *lspHarness) waitForEventAfter(start int, kind string, method string, pred func(json.RawMessage) bool) json.RawMessage {
	h.t.Helper()
	deadline := time.Now().Add(lspWaitTimeout)
	for time.Now().Before(deadline) {
		h.eventsM.Lock()
		for i := start; i < len(h.events); i++ {
			ev := h.events[i]
			if ev.Kind != kind || ev.Method != method {
				continue
			}
			if pred != nil && !pred(ev.Params) {
				continue
			}
			params := cloneRaw(ev.Params)
			h.eventsM.Unlock()
			return params
		}
		h.eventsM.Unlock()

		if err := h.getReadErr(); err != nil && !errors.Is(err, io.EOF) {
			h.t.Fatalf("LSP peer stopped while waiting for %s %s: %v\nrecorded events:\n%s", kind, method, err, h.dumpEventsAfter(start))
		}
		time.Sleep(lspPollInterval)
	}
	h.t.Fatalf("timed out waiting for %s %s\nrecorded events:\n%s", kind, method, h.dumpEventsAfter(start))
	return nil
}

func (h *lspHarness) waitForAnyEventAfter(start int, matchers ...eventMatcher) recordedEvent {
	h.t.Helper()
	deadline := time.Now().Add(lspWaitTimeout)
	for time.Now().Before(deadline) {
		h.eventsM.Lock()
		for i := start; i < len(h.events); i++ {
			ev := h.events[i]
			for _, matcher := range matchers {
				if ev.Kind != matcher.Kind || ev.Method != matcher.Method {
					continue
				}
				if matcher.Pred != nil && !matcher.Pred(ev.Params) {
					continue
				}
				found := recordedEvent{
					Kind:   ev.Kind,
					Method: ev.Method,
					Params: cloneRaw(ev.Params),
				}
				h.eventsM.Unlock()
				return found
			}
		}
		h.eventsM.Unlock()

		if err := h.getReadErr(); err != nil && !errors.Is(err, io.EOF) {
			h.t.Fatalf("LSP peer stopped while waiting for events: %v\nrecorded events:\n%s", err, h.dumpEventsAfter(start))
		}
		time.Sleep(lspPollInterval)
	}
	h.t.Fatalf("timed out waiting for expected events\nrecorded events:\n%s", h.dumpEventsAfter(start))
	return recordedEvent{}
}

func (h *lspHarness) handleResponse(resp *jsonrpc2.Response) {
	id, ok := resp.ID.Raw().(int64)
	if !ok {
		return
	}
	h.pendingM.Lock()
	ch := h.pending[id]
	delete(h.pending, id)
	h.pendingM.Unlock()
	if ch == nil {
		return
	}
	ch <- callResult{Result: cloneRaw(resp.Result), Err: resp.Error}
}

func (h *lspHarness) handleServerCall(req *jsonrpc2.Request) {
	switch req.Method {
	case "workspace/applyEdit":
		var params map[string]any
		if err := json.Unmarshal(req.Params, &params); err != nil {
			h.reply(req.ID, nil, err)
			return
		}
		result := map[string]any{"applied": true}
		edit, _ := params["edit"].(map[string]any)
		if err := h.applyWorkspaceEdit(edit); err != nil {
			result["applied"] = false
			result["failureReason"] = err.Error()
		}
		h.reply(req.ID, result, nil)
	case "workspace/configuration":
		var params struct {
			Items []map[string]any `json:"items"`
		}
		_ = json.Unmarshal(req.Params, &params)
		results := make([]any, len(params.Items))
		h.reply(req.ID, results, nil)
	case "workspace/workspaceFolders":
		h.foldersM.Lock()
		folders := append([]workspaceFolder(nil), h.folders...)
		h.foldersM.Unlock()
		h.reply(req.ID, folders, nil)
	case "client/registerCapability", "client/unregisterCapability", "window/workDoneProgress/create":
		h.reply(req.ID, nil, nil)
	case "window/showDocument":
		h.reply(req.ID, map[string]any{"success": true}, nil)
	case "window/showMessageRequest":
		h.reply(req.ID, nil, nil)
	default:
		h.reply(req.ID, nil, nil)
	}
}

func (h *lspHarness) reply(id jsonrpc2.ID, result any, err error) {
	msg, marshalErr := jsonrpc2.NewResponse(id, result, err)
	if marshalErr != nil {
		h.t.Fatalf("marshal response: %v", marshalErr)
	}
	if writeErr := h.writeMessage(msg); writeErr != nil && !errors.Is(writeErr, io.EOF) {
		h.t.Fatalf("write response: %v", writeErr)
	}
}

func (h *lspHarness) writeMessage(msg jsonrpc2.Message) error {
	h.writeMu.Lock()
	defer h.writeMu.Unlock()
	return h.writer.Write(context.Background(), msg)
}

func (h *lspHarness) notify(method string, params any) {
	h.t.Helper()
	msg, err := jsonrpc2.NewNotification(method, params)
	if err != nil {
		h.t.Fatalf("marshal notification %s: %v", method, err)
	}
	if err := h.writeMessage(msg); err != nil {
		h.t.Fatalf("write notification %s: %v", method, err)
	}
}

func (h *lspHarness) callRaw(method string, params any) json.RawMessage {
	h.t.Helper()
	id := atomic.AddInt64(&h.nextID, 1)
	msg, err := jsonrpc2.NewCall(jsonrpc2.Int64ID(id), method, params)
	if err != nil {
		h.t.Fatalf("marshal call %s: %v", method, err)
	}
	respCh := make(chan callResult, 1)
	h.pendingM.Lock()
	h.pending[id] = respCh
	h.pendingM.Unlock()
	if err := h.writeMessage(msg); err != nil {
		h.t.Fatalf("write call %s: %v", method, err)
	}
	select {
	case resp := <-respCh:
		if resp.Err != nil {
			h.t.Fatalf("call %s failed: %v\nrecorded events:\n%s", method, resp.Err, h.dumpEvents())
		}
		return resp.Result
	case <-time.After(lspCallTimeout):
		h.t.Fatalf("timed out waiting for response to %s\nrecorded events:\n%s", method, h.dumpEvents())
		return nil
	}
}

func (h *lspHarness) openFile(path string, languageID string) string {
	h.t.Helper()
	text := readFile(h.t, path)
	return h.openFileWithText(path, languageID, text)
}

func (h *lspHarness) openFileWithText(path string, languageID string, text string) string {
	h.t.Helper()
	uri := uriForPath(path)
	h.docsM.Lock()
	h.docs[uri] = &openDocument{
		Path:       path,
		URI:        uri,
		LanguageID: languageID,
		Text:       text,
		Version:    1,
	}
	h.docsM.Unlock()
	h.notify("textDocument/didOpen", map[string]any{
		"textDocument": map[string]any{
			"uri":        uri,
			"languageId": languageID,
			"version":    1,
			"text":       text,
		},
	})
	return uri
}

func (h *lspHarness) changeFile(path string, newText string) {
	h.t.Helper()
	uri := uriForPath(path)
	doc := h.mustOpenDoc(uri)
	doc.Version++
	doc.Text = newText
	h.notify("textDocument/didChange", map[string]any{
		"textDocument": map[string]any{
			"uri":     uri,
			"version": doc.Version,
		},
		"contentChanges": []map[string]any{{
			"text": newText,
		}},
	})
}

func (h *lspHarness) saveFile(path string) {
	h.t.Helper()
	uri := uriForPath(path)
	doc := h.mustOpenDoc(uri)
	writeFile(h.t, path, doc.Text)
	h.notify("textDocument/didSave", map[string]any{
		"textDocument": map[string]any{
			"uri": uri,
		},
		"text": doc.Text,
	})
}

func (h *lspHarness) closeFile(path string) {
	h.t.Helper()
	uri := uriForPath(path)
	h.notify("textDocument/didClose", map[string]any{
		"textDocument": map[string]any{
			"uri": uri,
		},
	})
	h.docsM.Lock()
	delete(h.docs, uri)
	h.docsM.Unlock()
}

func (h *lspHarness) setBufferText(uri string, text string) {
	h.t.Helper()
	doc := h.mustOpenDoc(uri)
	doc.Version++
	doc.Text = text
}

func (h *lspHarness) bufferText(uri string) string {
	h.t.Helper()
	h.docsM.Lock()
	defer h.docsM.Unlock()
	doc := h.docs[uri]
	if doc == nil {
		h.t.Fatalf("document is not open: %s", uri)
	}
	return doc.Text
}

func (h *lspHarness) mustOpenDoc(uri string) *openDocument {
	h.t.Helper()
	h.docsM.Lock()
	defer h.docsM.Unlock()
	doc := h.docs[uri]
	if doc == nil {
		h.t.Fatalf("document is not open: %s", uri)
	}
	return doc
}

func (h *lspHarness) changeWorkspaceFolders(added []workspaceFolder, removed []workspaceFolder) {
	h.t.Helper()
	h.foldersM.Lock()
	next := make([]workspaceFolder, 0, len(h.folders)+len(added))
	for _, existing := range h.folders {
		keep := true
		for _, folder := range removed {
			if folder.URI == existing.URI {
				keep = false
				break
			}
		}
		if keep {
			next = append(next, existing)
		}
	}
	next = append(next, added...)
	h.folders = next
	h.foldersM.Unlock()

	h.notify("workspace/didChangeWorkspaceFolders", map[string]any{
		"event": map[string]any{
			"added":   added,
			"removed": removed,
		},
	})
}

func (h *lspHarness) applyWorkspaceEdit(edit map[string]any) error {
	if len(edit) == 0 {
		return nil
	}
	if changes, ok := edit["changes"].(map[string]any); ok {
		for uri, rawEdits := range changes {
			edits, err := decodeTextEdits(rawEdits)
			if err != nil {
				return err
			}
			if err := h.applyTextEdits(uri, edits); err != nil {
				return err
			}
		}
	}
	if documentChanges, ok := edit["documentChanges"].([]any); ok {
		for _, rawChange := range documentChanges {
			change, ok := rawChange.(map[string]any)
			if !ok {
				continue
			}
			if _, ok := change["kind"]; ok {
				continue
			}
			textDocument, ok := change["textDocument"].(map[string]any)
			if !ok {
				continue
			}
			uri, _ := textDocument["uri"].(string)
			edits, err := decodeTextEdits(change["edits"])
			if err != nil {
				return err
			}
			if err := h.applyTextEdits(uri, edits); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *lspHarness) applyTextEdits(uri string, edits []textEdit) error {
	if len(edits) == 0 {
		return nil
	}

	path, err := pathForURI(uri)
	if err != nil {
		return err
	}

	h.docsM.Lock()
	doc := h.docs[uri]
	var text string
	if doc != nil {
		text = doc.Text
	} else {
		text = readFile(h.t, path)
	}
	h.docsM.Unlock()

	type editWithOffsets struct {
		Start   int
		End     int
		NewText string
	}
	converted := make([]editWithOffsets, 0, len(edits))
	for _, edit := range edits {
		start, err := offsetForPosition(text, edit.Range.Start)
		if err != nil {
			return err
		}
		end, err := offsetForPosition(text, edit.Range.End)
		if err != nil {
			return err
		}
		converted = append(converted, editWithOffsets{
			Start:   start,
			End:     end,
			NewText: edit.NewText,
		})
	}
	sort.Slice(converted, func(i, j int) bool {
		if converted[i].Start == converted[j].Start {
			return converted[i].End > converted[j].End
		}
		return converted[i].Start > converted[j].Start
	})
	for _, edit := range converted {
		if edit.Start > edit.End || edit.Start < 0 || edit.End > len(text) {
			return fmt.Errorf("invalid edit range %d:%d for %s", edit.Start, edit.End, uri)
		}
		text = text[:edit.Start] + edit.NewText + text[edit.End:]
	}

	h.docsM.Lock()
	defer h.docsM.Unlock()
	if doc != nil {
		doc.Text = text
		doc.Version++
		return nil
	}
	writeFile(h.t, path, text)
	return nil
}

type textEdit struct {
	Range   lspRange `json:"range"`
	NewText string   `json:"newText"`
}

func decodeTextEdits(raw any) ([]textEdit, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var edits []textEdit
	if err := json.Unmarshal(data, &edits); err != nil {
		return nil, err
	}
	return edits, nil
}

func offsetForPosition(text string, pos lspPosition) (int, error) {
	if pos.Line < 0 || pos.Character < 0 {
		return 0, fmt.Errorf("invalid position %+v", pos)
	}
	offset := 0
	line := 0
	for {
		if line == pos.Line {
			lineEnd := offset
			for lineEnd < len(text) && text[lineEnd] != '\n' {
				lineEnd++
			}
			char := pos.Character
			if char > lineEnd-offset {
				char = lineEnd - offset
			}
			return offset + char, nil
		}
		if offset >= len(text) {
			return len(text), nil
		}
		nextNewline := strings.IndexByte(text[offset:], '\n')
		if nextNewline < 0 {
			return len(text), nil
		}
		offset += nextNewline + 1
		line++
	}
}

func TestLSPServerE2EWithRealGopls(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixture := newFixture(t, repoRoot)
	h := newLSPHarness(t, []workspaceFolder{fixture.mainFolder})

	initializeResult := mustJSON[map[string]any](t, h.callRaw("initialize", map[string]any{
		"processId": nil,
		"rootUri":   fixture.mainFolder.URI,
		"capabilities": map[string]any{
			"workspace": map[string]any{
				"applyEdit": true,
				"workspaceEdit": map[string]any{
					"documentChanges": true,
				},
				"workspaceFolders": true,
			},
			"textDocument": map[string]any{
				"completion": map[string]any{
					"completionItem": map[string]any{
						"snippetSupport": true,
					},
				},
				"codeAction": map[string]any{
					"resolveSupport": map[string]any{
						"properties": []string{"edit", "command"},
					},
				},
				"publishDiagnostics": map[string]any{},
			},
			"window": map[string]any{},
		},
		"clientInfo": map[string]any{
			"name": "codex-e2e",
		},
		"workspaceFolders": []workspaceFolder{fixture.mainFolder},
	}))
	h.notify("initialized", map[string]any{})

	t.Run("initialize adds GoX completion triggers", func(t *testing.T) {
		h.t = t
		capabilities, ok := initializeResult["capabilities"].(map[string]any)
		if !ok {
			t.Fatalf("initialize result missing capabilities: %s", compactJSON(mustMarshalJSON(t, initializeResult)))
		}
		completionProvider, ok := capabilities["completionProvider"].(map[string]any)
		if !ok {
			t.Fatalf("initialize result missing completion provider: %s", compactJSON(mustMarshalJSON(t, initializeResult)))
		}
		var triggers []string
		data := mustMarshalJSON(t, completionProvider["triggerCharacters"])
		if err := json.Unmarshal(data, &triggers); err != nil {
			t.Fatalf("decode trigger characters: %v", err)
		}
		for _, trigger := range []string{"<", "/", "~"} {
			if !containsString(triggers, trigger) {
				t.Fatalf("initialize result did not include completion trigger %q: %v", trigger, triggers)
			}
		}
	})

	viewURI := h.openFile(fixture.view.Path, "gox")

	t.Run("source document calls are routed end-to-end", func(t *testing.T) {
		run := func(name string, fn func(t *testing.T)) {
			t.Run(name, func(t *testing.T) {
				h.t = t
				fn(t)
			})
		}

		run("hover", func(t *testing.T) {
			hover := h.callRaw("textDocument/hover", map[string]any{
				"textDocument": map[string]any{"uri": viewURI},
				"position":     fixture.view.Marker("template_label_use"),
			})
			assertRawContains(t, hover, "label")
		})

		run("completion", func(t *testing.T) {
			completion := h.callRaw("textDocument/completion", map[string]any{
				"textDocument": map[string]any{"uri": viewURI},
				"position":     fixture.view.Marker("complete_method"),
				"context": map[string]any{
					"triggerKind":      2,
					"triggerCharacter": ".",
				},
			})
			assertRawContains(t, completion, "NameValue")
		})

		run("definition", func(t *testing.T) {
			definition := h.callRaw("textDocument/definition", map[string]any{
				"textDocument": map[string]any{"uri": viewURI},
				"position":     fixture.view.Marker("template_label_use"),
			})
			assertRawContains(t, definition, viewURI)
		})

		run("references", func(t *testing.T) {
			references := h.callRaw("textDocument/references", map[string]any{
				"textDocument": map[string]any{"uri": viewURI},
				"position":     fixture.view.Marker("label_def"),
				"context": map[string]any{
					"includeDeclaration": true,
				},
			})
			assertRawContains(t, references, viewURI)
			if strings.Count(string(references), viewURI) < 2 {
				t.Fatalf("expected multiple source references, got %s", compactJSON(references))
			}
		})

		run("type definition", func(t *testing.T) {
			typeDefinition := h.callRaw("textDocument/typeDefinition", map[string]any{
				"textDocument": map[string]any{"uri": viewURI},
				"position":     fixture.view.Marker("helper_sig"),
			})
			assertRawContains(t, typeDefinition, viewURI)
		})

		run("implementation", func(t *testing.T) {
			implementation := h.callRaw("textDocument/implementation", map[string]any{
				"textDocument": map[string]any{"uri": viewURI},
				"position":     fixture.view.Marker("iface_ref"),
			})
			assertRawContains(t, implementation, viewURI)
		})

		run("prepare rename", func(t *testing.T) {
			prepareRename := h.callRaw("textDocument/prepareRename", map[string]any{
				"textDocument": map[string]any{"uri": viewURI},
				"position":     fixture.view.Marker("helper_use"),
			})
			assertRawContains(t, prepareRename, "\"range\"")
		})

		run("rename", func(t *testing.T) {
			rename := h.callRaw("textDocument/rename", map[string]any{
				"textDocument": map[string]any{"uri": viewURI},
				"position":     fixture.view.Marker("helper_use"),
				"newName":      "helperRenamed",
			})
			assertRawContains(t, rename, viewURI)
			assertRawContains(t, rename, "helperRenamed")
		})

		run("prepare type hierarchy", func(t *testing.T) {
			prepareTypeHierarchy := h.callRaw("textDocument/prepareTypeHierarchy", map[string]any{
				"textDocument": map[string]any{"uri": viewURI},
				"position":     fixture.view.Marker("iface_def"),
			})
			typeItems := mustJSON[[]map[string]any](t, prepareTypeHierarchy)
			if len(typeItems) == 0 {
				t.Fatalf("prepare type hierarchy returned no items: %s", compactJSON(prepareTypeHierarchy))
			}
			assertRawContains(t, prepareTypeHierarchy, viewURI)
		})

		run("type hierarchy subtypes", func(t *testing.T) {
			prepareTypeHierarchy := h.callRaw("textDocument/prepareTypeHierarchy", map[string]any{
				"textDocument": map[string]any{"uri": viewURI},
				"position":     fixture.view.Marker("iface_def"),
			})
			typeItems := mustJSON[[]map[string]any](t, prepareTypeHierarchy)
			if len(typeItems) == 0 {
				t.Fatalf("prepare type hierarchy returned no items: %s", compactJSON(prepareTypeHierarchy))
			}
			subtypes := h.callRaw("typeHierarchy/subtypes", map[string]any{
				"item": typeItems[0],
			})
			assertRawContains(t, subtypes, viewURI)
		})

		run("type hierarchy supertypes", func(t *testing.T) {
			personHierarchy := h.callRaw("textDocument/prepareTypeHierarchy", map[string]any{
				"textDocument": map[string]any{"uri": viewURI},
				"position":     fixture.view.Marker("person_type"),
			})
			personItems := mustJSON[[]map[string]any](t, personHierarchy)
			if len(personItems) == 0 {
				t.Fatalf("prepare type hierarchy for Person returned no items: %s", compactJSON(personHierarchy))
			}
			supertypes := h.callRaw("typeHierarchy/supertypes", map[string]any{
				"item": personItems[0],
			})
			assertRawContains(t, supertypes, viewURI)
		})

		run("document highlight", func(t *testing.T) {
			documentHighlight := h.callRaw("textDocument/documentHighlight", map[string]any{
				"textDocument": map[string]any{"uri": viewURI},
				"position":     fixture.view.Marker("template_label_use"),
			})
			assertNonEmptyArray(t, documentHighlight)
		})

		run("source code lens", func(t *testing.T) {
			codeLens := h.callRaw("textDocument/codeLens", map[string]any{
				"textDocument": map[string]any{"uri": viewURI},
			})
			assertValidJSON(t, codeLens)
		})

		run("source document link", func(t *testing.T) {
			rawLinks := h.callRaw("textDocument/documentLink", map[string]any{
				"textDocument": map[string]any{"uri": viewURI},
			})
			links := mustJSON[[]documentLink](t, rawLinks)
			if len(links) == 0 {
				t.Fatalf("expected at least one source document link, got %s", compactJSON(rawLinks))
			}
			assertRawContains(t, rawLinks, "https://example.com/view-docs")

			targetURI := uriForPath(strings.TrimSuffix(fixture.view.Path, ".gox") + ".x.go")
			rawTargetLinks := h.callRaw("textDocument/documentLink", map[string]any{
				"textDocument": map[string]any{"uri": targetURI},
			})
			targetLinks := mustJSON[[]documentLink](t, rawTargetLinks)
			if len(targetLinks) == 0 {
				t.Fatalf("expected at least one target document link, got %s", compactJSON(rawTargetLinks))
			}
			if rangesEqual(links[0].Range, targetLinks[0].Range) {
				t.Fatalf(
					"expected source document link range to differ from raw target range; source=%+v target=%+v",
					links[0].Range,
					targetLinks[0].Range,
				)
			}
		})

		run("document symbol", func(t *testing.T) {
			documentSymbol := h.callRaw("textDocument/documentSymbol", map[string]any{
				"textDocument": map[string]any{"uri": viewURI},
			})
			assertRawContains(t, documentSymbol, "View")
			assertRawContains(t, documentSymbol, "helper")
			assertRawContains(t, documentSymbol, "Namer")
		})

		run("prepare call hierarchy", func(t *testing.T) {
			prepareCallHierarchy := h.callRaw("textDocument/prepareCallHierarchy", map[string]any{
				"textDocument": map[string]any{"uri": viewURI},
				"position":     fixture.view.Marker("helper_def"),
			})
			callItems := mustJSON[[]map[string]any](t, prepareCallHierarchy)
			if len(callItems) == 0 {
				t.Fatalf("prepare call hierarchy returned no items: %s", compactJSON(prepareCallHierarchy))
			}
			assertRawContains(t, prepareCallHierarchy, viewURI)
		})

		run("call hierarchy incoming calls", func(t *testing.T) {
			prepareCallHierarchy := h.callRaw("textDocument/prepareCallHierarchy", map[string]any{
				"textDocument": map[string]any{"uri": viewURI},
				"position":     fixture.view.Marker("helper_def"),
			})
			callItems := mustJSON[[]map[string]any](t, prepareCallHierarchy)
			if len(callItems) == 0 {
				t.Fatalf("prepare call hierarchy returned no items: %s", compactJSON(prepareCallHierarchy))
			}
			incoming := h.callRaw("callHierarchy/incomingCalls", map[string]any{
				"item": callItems[0],
			})
			assertRawContains(t, incoming, viewURI)
		})

		run("call hierarchy outgoing calls", func(t *testing.T) {
			prepareCallHierarchy := h.callRaw("textDocument/prepareCallHierarchy", map[string]any{
				"textDocument": map[string]any{"uri": viewURI},
				"position":     fixture.view.Marker("helper_def"),
			})
			callItems := mustJSON[[]map[string]any](t, prepareCallHierarchy)
			if len(callItems) == 0 {
				t.Fatalf("prepare call hierarchy returned no items: %s", compactJSON(prepareCallHierarchy))
			}
			outgoing := h.callRaw("callHierarchy/outgoingCalls", map[string]any{
				"item": callItems[0],
			})
			assertRawContains(t, outgoing, viewURI)
		})

		run("semantic tokens full", func(t *testing.T) {
			semanticTokensFull := h.callRaw("textDocument/semanticTokens/full", map[string]any{
				"textDocument": map[string]any{"uri": viewURI},
			})
			assertRawContains(t, semanticTokensFull, "\"data\":[]")
		})

		run("semantic tokens range", func(t *testing.T) {
			semanticTokensRange := h.callRaw("textDocument/semanticTokens/range", map[string]any{
				"textDocument": map[string]any{"uri": viewURI},
				"range":        fixture.rangeAt("template_label_use", len("label")),
			})
			assertRawContains(t, semanticTokensRange, "\"data\":[]")
		})

		run("folding range", func(t *testing.T) {
			foldingRange := h.callRaw("textDocument/foldingRange", map[string]any{
				"textDocument": map[string]any{"uri": viewURI},
			})
			assertRawEquals(t, foldingRange, "[]")
		})

		run("formatting", func(t *testing.T) {
			formatting := h.callRaw("textDocument/formatting", map[string]any{
				"textDocument": map[string]any{"uri": uriForPath(fixture.format.Path)},
				"options": map[string]any{
					"tabSize":      4,
					"insertSpaces": true,
				},
			})
			assertNonEmptyArray(t, formatting)
			assertRawContains(t, formatting, "\"newText\"")
		})

		run("inlay hints", func(t *testing.T) {
			inlayHints := h.callRaw("textDocument/inlayHint", map[string]any{
				"textDocument": map[string]any{"uri": viewURI},
				"range":        fixture.rangeAt("view_def", len("View")),
			})
			assertValidJSON(t, inlayHints)
		})

		run("selection range", func(t *testing.T) {
			selectionRanges := h.callRaw("textDocument/selectionRange", map[string]any{
				"textDocument": map[string]any{"uri": viewURI},
				"positions":    []lspPosition{fixture.view.Marker("template_label_use")},
			})
			assertValidJSON(t, selectionRanges)
		})

		run("signature help", func(t *testing.T) {
			signatureHelp := h.callRaw("textDocument/signatureHelp", map[string]any{
				"textDocument": map[string]any{"uri": viewURI},
				"position":     fixture.view.Marker("helper_sig"),
			})
			assertRawContains(t, signatureHelp, "helper")
		})

		run("workspace symbol", func(t *testing.T) {
			workspaceSymbol := h.callRaw("workspace/symbol", map[string]any{
				"query": "View",
			})
			assertRawContains(t, workspaceSymbol, viewURI)
		})
	})

	t.Run("regular Go files still proxy through GoX to real gopls", func(t *testing.T) {
		h.t = t
		testURI := h.openFile(fixture.testGo.Path, "go")
		_ = testURI
		codeLens := h.callRaw("textDocument/codeLens", map[string]any{
			"textDocument": map[string]any{"uri": uriForPath(fixture.testGo.Path)},
		})
		assertValidJSON(t, codeLens)

		documentLinks := h.callRaw("textDocument/documentLink", map[string]any{
			"textDocument": map[string]any{"uri": uriForPath(fixture.links.Path)},
		})
		assertRawContains(t, documentLinks, "https://example.com/docs")
	})

	importURI := h.openFile(fixture.addImport.Path, "go")

	var addImportDiagnostics publishDiagnosticsParams
	t.Run("code actions and resolve work through the real gopls session", func(t *testing.T) {
		h.t = t
		addImportDiagnostics = waitForDiagnostics(t, h, importURI)

		codeAction := h.callRaw("textDocument/codeAction", map[string]any{
			"textDocument": map[string]any{"uri": importURI},
			"range":        addImportDiagnostics.Diagnostics[0].Range,
			"context": map[string]any{
				"diagnostics": addImportDiagnostics.Diagnostics,
			},
		})
		assertNonEmptyArray(t, codeAction)

		actions := mustJSON[[]map[string]any](t, codeAction)
		if len(actions) == 0 {
			t.Fatalf("code action result was empty: %s", compactJSON(codeAction))
		}
		resolved := h.callRaw("codeAction/resolve", actions[0])
		assertValidJSON(t, resolved)
	})

	t.Run("real gopls can call workspace/applyEdit back through GoX", func(t *testing.T) {
		h.t = t
		checkpoint := h.checkpoint()
		_ = h.callRaw("workspace/executeCommand", map[string]any{
			"command": "gopls.add_import",
			"arguments": []map[string]any{{
				"ImportPath": "fmt",
				"URI":        importURI,
			}},
		})
		params := h.waitForEventAfter(checkpoint, "call", "workspace/applyEdit", func(raw json.RawMessage) bool {
			return strings.Contains(string(raw), importURI)
		})
		assertRawContains(t, params, importURI)
		if !strings.Contains(h.bufferText(importURI), "import \"fmt\"") {
			t.Fatalf("workspace/applyEdit did not update the open buffer:\n%s", h.bufferText(importURI))
		}
	})

	brokenURI := h.openFile(fixture.broken.Path, "gox")

	t.Run("diagnostics are published back to the source .gox URI", func(t *testing.T) {
		run := func(name string, fn func(t *testing.T)) {
			t.Run(name, func(t *testing.T) {
				h.t = t
				fn(t)
			})
		}

		run("publish diagnostics", func(t *testing.T) {
			diagnostics := waitForDiagnostics(t, h, brokenURI)
			if len(diagnostics.Diagnostics) == 0 {
				t.Fatalf("expected diagnostics for %s", brokenURI)
			}
			if !strings.Contains(diagnostics.Diagnostics[0].Message, "missingSymbol") {
				t.Fatalf("unexpected diagnostic message: %+v", diagnostics.Diagnostics[0])
			}
		})

		run("pull diagnostics", func(t *testing.T) {
			diagnosticResult := h.callRaw("textDocument/diagnostic", map[string]any{
				"textDocument": map[string]any{"uri": brokenURI},
			})
			report := mustJSON[documentDiagnosticReport](t, diagnosticResult)
			if len(report.Items) == 0 {
				t.Fatalf("expected pull diagnostics for %s, got %s", brokenURI, compactJSON(diagnosticResult))
			}
			if !strings.Contains(report.Items[0].Message, "missingSymbol") {
				t.Fatalf("unexpected pull diagnostic message: %+v", report.Items[0])
			}
			assertRangeContainsPosition(t, report.Items[0].Range, fixture.broken.Marker("missing_symbol"))
		})
	})

	targetPath := strings.TrimSuffix(fixture.view.Path, ".gox") + ".x.go"
	targetURI := uriForPath(targetPath)

	t.Run("source and target lifecycle notifications stay in sync", func(t *testing.T) {
		run := func(name string, fn func(t *testing.T)) {
			t.Run(name, func(t *testing.T) {
				h.t = t
				fn(t)
			})
		}

		run("opening target while source is open refreshes generated buffer", func(t *testing.T) {
			targetText := waitForFileContents(t, targetPath)
			openTargetCheckpoint := h.checkpoint()
			h.openFileWithText(targetPath, "go", targetText+"\n// stale\n")
			h.waitForEventAfter(openTargetCheckpoint, "call", "workspace/applyEdit", func(raw json.RawMessage) bool {
				return strings.Contains(string(raw), targetURI)
			})
			if h.bufferText(targetURI) != targetText {
				t.Fatalf("target open sync did not restore generated text")
			}
		})

		run("source didChange refreshes generated target buffer", func(t *testing.T) {
			changeTargetCheckpoint := h.checkpoint()
			updatedView := strings.Replace(fixture.view.Text, "class=\"badge\"", "class=\"badge2\"", 1)
			h.changeFile(fixture.view.Path, updatedView)
			event := h.waitForAnyEventAfter(
				changeTargetCheckpoint,
				eventMatcher{
					Kind:   "call",
					Method: "workspace/applyEdit",
					Pred: func(raw json.RawMessage) bool {
						return strings.Contains(string(raw), targetURI)
					},
				},
				eventMatcher{
					Kind:   "notification",
					Method: "window/showMessage",
					Pred: func(raw json.RawMessage) bool {
						return strings.Contains(string(raw), "The range field is missing.")
					},
				},
			)
			if event.Method != "workspace/applyEdit" {
				t.Fatalf("source didChange failed before refreshing the target buffer: %s", compactJSON(event.Params))
			}
			if !strings.Contains(h.bufferText(targetURI), "badge2") {
				t.Fatalf("source change did not refresh the generated target buffer:\n%s", h.bufferText(targetURI))
			}
		})

		run("source didSave writes the refreshed generated file", func(t *testing.T) {
			if !strings.Contains(h.bufferText(targetURI), "badge2") {
				t.Fatalf("source change never refreshed the target buffer, so save could not persist the new generated text:\n%s", h.bufferText(targetURI))
			}
			h.saveFile(fixture.view.Path)
			savedTarget := waitForFileTextContains(t, targetPath, "badge2")
			if !strings.Contains(savedTarget, "badge2") {
				t.Fatalf("source save did not write the refreshed generated file:\n%s", savedTarget)
			}
		})

		run("target didChange and didSave restore generated contents", func(t *testing.T) {
			warnCheckpoint := h.checkpoint()
			h.changeFile(targetPath, "package probe\n\n// hacked by test\n")
			h.saveFile(targetPath)
			h.waitForEventAfter(warnCheckpoint, "notification", "window/showMessage", func(raw json.RawMessage) bool {
				return strings.Contains(string(raw), "Generated file changes were reverted on save")
			})
			restoredTarget := waitForFileContents(t, targetPath)
			if strings.Contains(restoredTarget, "hacked by test") {
				t.Fatalf("target save did not restore generated contents:\n%s", restoredTarget)
			}
		})

		run("closing source and target lets target reopen proxy directly", func(t *testing.T) {
			h.closeFile(fixture.view.Path)
			h.closeFile(targetPath)

			reopenTargetText := waitForFileContents(t, targetPath)
			h.openFileWithText(targetPath, "go", reopenTargetText)
			hover := h.callRaw("textDocument/hover", map[string]any{
				"textDocument": map[string]any{"uri": targetURI},
				"position": positionOfSubstring(
					t,
					reopenTargetText,
					"label(value)",
				),
			})
			assertRawContains(t, hover, "label")
			h.closeFile(targetPath)
		})
	})

	t.Run("workspace folder changes propagate through the live session", func(t *testing.T) {
		h.t = t
		h.changeWorkspaceFolders([]workspaceFolder{fixture.extraFolder}, nil)
		waitForWorkspaceSymbol(t, h, "Extra", fixture.extraFolder.URI)
		h.changeWorkspaceFolders(nil, []workspaceFolder{fixture.extraFolder})
	})
}

type fixtureFile struct {
	Path    string
	URI     string
	Text    string
	Markers map[string]lspPosition
}

func (f fixtureFile) Marker(name string) lspPosition {
	pos, ok := f.Markers[name]
	if !ok {
		panic("missing marker: " + name)
	}
	return pos
}

type lspFixture struct {
	mainFolder  workspaceFolder
	extraFolder workspaceFolder
	view        fixtureFile
	format      fixtureFile
	broken      fixtureFile
	links       fixtureFile
	testGo      fixtureFile
	addImport   fixtureFile
}

func newFixture(t *testing.T, repoRoot string) lspFixture {
	t.Helper()

	base := t.TempDir()
	mainRoot := filepath.Join(base, "main")
	extraRoot := filepath.Join(base, "extra")

	writeFile(t, filepath.Join(mainRoot, "go.mod"), fmt.Sprintf(`module example.com/probe

go 1.25.1

require github.com/doors-dev/gox v0.0.0

replace github.com/doors-dev/gox => %s
`, repoRoot))

	view := writeMarkedFile(t, filepath.Join(mainRoot, "view.gox"), `package probe

// /*@view_doc_link*/https://example.com/view-docs

import "github.com/doors-dev/gox"

type /*@person_type*/Person struct {
	Name string
}

type /*@iface_def*/Namer interface {
	/*@iface_method_def*/NameValue() string
}

func (p Person) /*@impl_method_def*/NameValue() string {
	return p.Name
}

func /*@label_def*/label(name string) string {
	return name
}

func /*@helper_def*/helper(name string) string {
	return /*@helper_call*/label(name)
}

func /*@view_def*/View(person /*@iface_ref*/Namer) gox.Elem {
	value := /*@helper_use*/helper(/*@helper_sig*/person./*@complete_method*/NameValue())
	return <div class="badge">~(/*@template_label_use*/label(value))</div>
}
`)

	format := writeMarkedFile(t, filepath.Join(mainRoot, "format.gox"), `package probe

import "github.com/doors-dev/gox"

func Messy( person Namer ) gox.Elem { return <div>~( label( person.NameValue() ) )</div> }
`)

	links := writeMarkedFile(t, filepath.Join(mainRoot, "links.go"), `package probe

// https://example.com/docs
func LinkDoc() {}
`)

	testGo := writeMarkedFile(t, filepath.Join(mainRoot, "view_test.go"), `package probe

import "testing"

func TestSmoke(t *testing.T) {}
`)

	broken := writeMarkedFile(t, filepath.Join(mainRoot, "broken", "broken.gox"), `package broken

import "github.com/doors-dev/gox"

func Broken() gox.Elem {
	return <div>~(/*@missing_symbol*/missingSymbol)</div>
}
`)

	addImport := writeMarkedFile(t, filepath.Join(mainRoot, "importpkg", "addimport.go"), `package importpkg

func AddImportTarget() {
	fmt.Println("hi")
}
`)

	writeFile(t, filepath.Join(extraRoot, "go.mod"), `module example.com/extra

go 1.25.1
`)
	writeFile(t, filepath.Join(extraRoot, "extra.go"), `package extra

func Extra() {}
`)

	return lspFixture{
		mainFolder: workspaceFolder{
			URI:  uriForPath(mainRoot),
			Name: "main",
		},
		extraFolder: workspaceFolder{
			URI:  uriForPath(extraRoot),
			Name: "extra",
		},
		view:      view,
		format:    format,
		broken:    broken,
		links:     links,
		testGo:    testGo,
		addImport: addImport,
	}
}

func (f lspFixture) rangeAt(marker string, width int) lspRange {
	start := f.view.Marker(marker)
	return lspRange{
		Start: start,
		End: lspPosition{
			Line:      start.Line,
			Character: start.Character + width,
		},
	}
}

func waitForDiagnostics(t *testing.T, h *lspHarness, uri string) publishDiagnosticsParams {
	t.Helper()
	params := h.waitForEventAfter(0, "notification", "textDocument/publishDiagnostics", func(raw json.RawMessage) bool {
		var notification publishDiagnosticsParams
		if err := json.Unmarshal(raw, &notification); err != nil {
			return false
		}
		return notification.URI == uri && len(notification.Diagnostics) > 0
	})
	return mustJSON[publishDiagnosticsParams](t, params)
}

func waitForWorkspaceSymbol(t *testing.T, h *lspHarness, query string, folderURI string) {
	t.Helper()
	deadline := time.Now().Add(lspWaitTimeout)
	for time.Now().Before(deadline) {
		result := h.callRaw("workspace/symbol", map[string]any{"query": query})
		if strings.Contains(string(result), query) && strings.Contains(string(result), folderURI) {
			return
		}
		time.Sleep(lspPollInterval)
	}
	t.Fatalf("workspace/symbol never returned %q for %s", query, folderURI)
}

func waitForFileContents(t *testing.T, path string) string {
	t.Helper()
	deadline := time.Now().Add(lspWaitTimeout)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data)
		}
		time.Sleep(lspPollInterval)
	}
	t.Fatalf("timed out waiting for file %s", path)
	return ""
}

func waitForFileTextContains(t *testing.T, path string, needle string) string {
	t.Helper()
	deadline := time.Now().Add(lspWaitTimeout)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(data), needle) {
			return string(data)
		}
		time.Sleep(lspPollInterval)
	}
	t.Fatalf("timed out waiting for %s to contain %q", path, needle)
	return ""
}

func assertRawContains(t *testing.T, raw json.RawMessage, needle string) {
	t.Helper()
	if !strings.Contains(string(raw), needle) {
		t.Fatalf("expected %q in %s", needle, compactJSON(raw))
	}
}

func assertRawEquals(t *testing.T, raw json.RawMessage, expected string) {
	t.Helper()
	if compactJSON(raw) != expected {
		t.Fatalf("expected %s, got %s", expected, compactJSON(raw))
	}
}

func assertValidJSON(t *testing.T, raw json.RawMessage) {
	t.Helper()
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, string(raw))
	}
}

func assertNonEmptyArray(t *testing.T, raw json.RawMessage) {
	t.Helper()
	var arr []any
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Fatalf("expected array JSON: %v\n%s", err, string(raw))
	}
	if len(arr) == 0 {
		t.Fatalf("expected a non-empty array, got %s", compactJSON(raw))
	}
}

func assertRangeContainsPosition(t *testing.T, ran lspRange, pos lspPosition) {
	t.Helper()
	if !rangeContainsPosition(ran, pos) {
		t.Fatalf("expected range %+v to contain position %+v", ran, pos)
	}
}

func mustJSON[T any](t *testing.T, raw json.RawMessage) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, string(raw))
	}
	return out
}

func compactJSON(raw []byte) string {
	if len(raw) == 0 {
		return "null"
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return strings.TrimSpace(string(raw))
	}
	data, err := json.Marshal(value)
	if err != nil {
		return strings.TrimSpace(string(raw))
	}
	return string(data)
}

func mustMarshalJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	return data
}

func cloneRaw(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func writeMarkedFile(t *testing.T, path string, content string) fixtureFile {
	t.Helper()
	text, markers, err := stripMarkers(content)
	if err != nil {
		t.Fatalf("strip markers for %s: %v", path, err)
	}
	writeFile(t, path, text)
	return fixtureFile{
		Path:    path,
		URI:     uriForPath(path),
		Text:    text,
		Markers: markers,
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func stripMarkers(input string) (string, map[string]lspPosition, error) {
	offsets := make(map[string]int)
	var out strings.Builder
	for i := 0; i < len(input); {
		if strings.HasPrefix(input[i:], "/*@") {
			end := strings.Index(input[i:], "*/")
			if end < 0 {
				return "", nil, errors.New("unterminated marker")
			}
			name := input[i+3 : i+end]
			if _, exists := offsets[name]; exists {
				return "", nil, fmt.Errorf("duplicate marker %q", name)
			}
			offsets[name] = out.Len()
			i += end + 2
			continue
		}
		out.WriteByte(input[i])
		i++
	}
	text := out.String()
	markers := make(map[string]lspPosition, len(offsets))
	for name, offset := range offsets {
		markers[name] = positionAtOffset(text, offset)
	}
	return text, markers, nil
}

func positionAtOffset(text string, offset int) lspPosition {
	if offset < 0 {
		return lspPosition{}
	}
	line := 0
	character := 0
	for i := 0; i < offset && i < len(text); i++ {
		if text[i] == '\n' {
			line++
			character = 0
			continue
		}
		character++
	}
	return lspPosition{Line: line, Character: character}
}

func rangeContainsPosition(ran lspRange, pos lspPosition) bool {
	startOK := pos.Line > ran.Start.Line || (pos.Line == ran.Start.Line && pos.Character >= ran.Start.Character)
	endOK := pos.Line < ran.End.Line || (pos.Line == ran.End.Line && pos.Character <= ran.End.Character)
	return startOK && endOK
}

func rangesEqual(a lspRange, b lspRange) bool {
	return a.Start == b.Start && a.End == b.End
}

func positionOfSubstring(t *testing.T, text string, substring string) lspPosition {
	t.Helper()
	index := strings.Index(text, substring)
	if index < 0 {
		t.Fatalf("substring %q not found", substring)
	}
	return positionAtOffset(text, index)
}

func uriForPath(path string) string {
	return string(docpath.URIFromPath(path))
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func pathForURI(uri string) (string, error) {
	docURI, err := docpath.ParseDocumentURI(uri)
	if err != nil {
		return "", err
	}
	return docURI.Path(), nil
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate repository root")
		}
		dir = parent
	}
}
