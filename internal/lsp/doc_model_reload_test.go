package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/doors-dev/gox/internal/workspace"
)

func docModelShifted() string {
	return strings.Replace(lspHelperSource, "type Card struct", "// a\n// b\nvar Extra = 1\n\ntype Card struct", 1)
}

func docModelGenerated(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(docModelTempDir(t), "view.gox")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	file, _ := workspace.NewFile(path)
	doc := workspace.NewDoc(file)
	defer doc.Close()
	if err := doc.Load(); err != nil {
		t.Fatal(err)
	}
	doc.Parse()
	doc.Assemble()
	return doc.TargetContent()
}

func docModelTitleRange(text string) map[string]any {
	for i, l := range strings.Split(text, "\n") {
		if c := strings.Index(l, "Title string"); c >= 0 {
			return map[string]any{
				"start": map[string]any{"line": i, "character": c},
				"end":   map[string]any{"line": i, "character": c + len("Title")},
			}
		}
	}
	panic("Title not in text")
}

func (e *docModelEnv) call(role Role, m method, params any) Response {
	e.t.Helper()
	data, err := json.Marshal(params)
	if err != nil {
		e.t.Fatal(err)
	}
	done := make(chan Response, 1)
	e.router.Call(role, Request{Method: string(m), Params: data}, func(r Response) {
		done <- r
	})
	select {
	case r := <-done:
		e.sync()
		return r
	case <-time.After(docModelWait):
		e.t.Fatalf("%s was not answered; traffic: %v", m, e.traffic())
		return Response{}
	}
}

func (e *docModelEnv) watchedChanges() int {
	n := 0
	for _, note := range e.bridge.snapshot() {
		if note.role != Gopls || note.req.Method != string(didChangeWatchedFiles) {
			continue
		}
		var p struct {
			Changes []struct {
				URI  string `json:"uri"`
				Type int    `json:"type"`
			} `json:"changes"`
		}
		if json.Unmarshal(note.req.Params, &p) == nil && len(p.Changes) == 1 && p.Changes[0].URI == docModelURI(e.target) && p.Changes[0].Type == 2 {
			n++
		}
	}
	return n
}

func (e *docModelEnv) targetEdit(text string) map[string]any {
	return map[string]any{"documentChanges": []map[string]any{{
		"textDocument": map[string]any{"uri": docModelURI(e.target), "version": 1},
		"edits":        []map[string]any{{"range": docModelTitleRange(text), "newText": "Name"}},
	}}}
}

func (e *docModelEnv) targetChanges(text string) map[string]any {
	return map[string]any{"changes": map[string]any{
		docModelURI(e.target): []map[string]any{{"range": docModelTitleRange(text), "newText": "Name"}},
	}}
}

func (e *docModelEnv) assertTitleOnSource(step string, uri string, ran json.RawMessage) {
	e.t.Helper()
	var got, want struct {
		Start, End struct{ Line, Character int }
	}
	data, _ := json.Marshal(docModelTitleRange(docModelShifted()))
	json.Unmarshal(data, &want)
	if json.Unmarshal(ran, &got) != nil || uri != docModelURI(e.source) || got != want {
		e.t.Fatalf("%s: %s %s is not Title in the file on disk", step, uri, ran)
	}
}

func (e *docModelEnv) assertEditOnSource(step string, raw json.RawMessage) {
	e.t.Helper()
	type textEdit struct {
		Range json.RawMessage `json:"range"`
	}
	var edit struct {
		Changes         map[string][]textEdit `json:"changes"`
		DocumentChanges []struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Edits []textEdit `json:"edits"`
		} `json:"documentChanges"`
	}
	if err := json.Unmarshal(raw, &edit); err != nil {
		e.t.Fatalf("%s: unexpected edit %s", step, raw)
	}
	for uri, edits := range edit.Changes {
		if len(edit.Changes) != 1 || len(edits) != 1 {
			e.t.Fatalf("%s: unexpected edit %s", step, raw)
		}
		e.assertTitleOnSource(step, uri, edits[0].Range)
		return
	}
	if len(edit.DocumentChanges) != 1 || len(edit.DocumentChanges[0].Edits) != 1 {
		e.t.Fatalf("%s: unexpected edit %s", step, raw)
	}
	e.assertTitleOnSource(step, edit.DocumentChanges[0].TextDocument.URI, edit.DocumentChanges[0].Edits[0].Range)
}

func (e *docModelEnv) setResult(result any) {
	e.t.Helper()
	data, err := json.Marshal(result)
	if err != nil {
		e.t.Fatal(err)
	}
	e.bridge.mu.Lock()
	e.bridge.result = data
	e.bridge.mu.Unlock()
}

// A gopls edit on a .gox that changed on disk is refused and the .gox
// reloaded; the next edit, computed on the regenerated code, maps onto the
// file.
func TestDocModelGoplsEditOnSourceChangedOnDisk(t *testing.T) {
	t.Parallel()
	for _, shape := range []string{"documentChanges", "changes"} {
		t.Run(shape, func(t *testing.T) {
			env := newDocModelEnv(t)
			env.write(env.source, lspHelperSource)
			env.start(env.root)
			edit := env.targetEdit
			if shape == "changes" {
				edit = env.targetChanges
			}
			stale := env.waitTarget("")
			env.write(env.source, docModelShifted())

			calls := len(env.bridge.callSnapshot())
			r := env.call(Gopls, applyEdit, map[string]any{"edit": edit(stale)})
			var res struct {
				Applied       bool   `json:"applied"`
				FailureReason string `json:"failureReason"`
			}
			if r.Err != nil || json.Unmarshal(r.Result, &res) != nil || res.Applied || !strings.Contains(res.FailureReason, "view.gox changed on disk") {
				t.Fatalf("edit on a changed .gox: response %s, error %v", r.Result, r.Err)
			}
			if len(env.bridge.callSnapshot()) != calls {
				t.Fatalf("refused edit was forwarded to the client; traffic: %v", env.traffic())
			}
			if env.watchedChanges() != 1 {
				t.Fatalf("gopls was not told the .x.go changed; traffic: %v", env.traffic())
			}

			r = env.call(Gopls, applyEdit, map[string]any{"edit": edit(env.waitTarget("var Extra"))})
			sent := env.bridge.callSnapshot()
			if r.Err != nil || len(sent) != calls+1 {
				t.Fatalf("retried edit was not forwarded: %v; traffic: %v", r.Err, env.traffic())
			}
			var params struct {
				Edit json.RawMessage `json:"edit"`
			}
			json.Unmarshal(sent[calls].req.Params, &params)
			env.assertEditOnSource("retried edit", params.Edit)
		})
	}
}

// Client requests whose gopls edit touches a .gox that changed on disk fail
// with a retry message (code actions come back as an empty list, without a
// message); the retry gets the edits on the file.
func TestDocModelClientEditOnSourceChangedOnDisk(t *testing.T) {
	t.Parallel()
	for _, m := range []method{rename, codeAction, resolveCodeAction} {
		t.Run(string(m), func(t *testing.T) {
			env := newDocModelEnv(t)
			env.write(env.source, lspHelperSource)
			env.start(env.root)
			stale := env.waitTarget("")
			var params any = map[string]any{
				"textDocument": map[string]any{"uri": docModelURI(filepath.Join(env.root, "use.go"))},
				"position":     map[string]any{"line": 0, "character": 0},
				"newName":      "Name",
			}
			if m == resolveCodeAction {
				params = map[string]any{"title": "fix", "data": map[string]any{}}
			}
			respond := func(text string) {
				switch m {
				case rename:
					env.setResult(env.targetEdit(text))
				case codeAction:
					action := map[string]any{"title": "fix", "kind": "quickfix", "edit": env.targetEdit(text)}
					env.setResult([]any{action, action})
				case resolveCodeAction:
					env.setResult(map[string]any{"title": "fix", "edit": env.targetEdit(text)})
				}
			}
			env.write(env.source, docModelShifted())

			respond(stale)
			r := env.call(Client, m, params)
			if m == codeAction {
				if r.Err != nil || string(r.Result) != "[]" || env.clientMessage("changed on disk") {
					t.Fatalf("code action on a changed .gox was not dropped silently: %s, error %v; traffic: %v", r.Result, r.Err, env.traffic())
				}
			} else if r.Err == nil || !env.clientMessage("view.gox changed on disk") {
				t.Fatalf("edit on a changed .gox was not refused with a retry message: %s; traffic: %v", r.Result, env.traffic())
			}
			if env.watchedChanges() != 1 {
				t.Fatalf("gopls was not told the .x.go changed; traffic: %v", env.traffic())
			}

			respond(env.waitTarget("var Extra"))
			r = env.call(Client, m, params)
			if r.Err != nil {
				t.Fatalf("retry failed: %v; traffic: %v", r.Err, env.traffic())
			}
			var action struct {
				Edit json.RawMessage `json:"edit"`
			}
			switch m {
			case rename:
				action.Edit = r.Result
			case codeAction:
				var actions []struct {
					Edit json.RawMessage `json:"edit"`
				}
				if json.Unmarshal(r.Result, &actions) != nil || len(actions) != 2 {
					t.Fatalf("retry returned %s", r.Result)
				}
				env.assertEditOnSource("retry", actions[0].Edit)
				action = actions[1]
			case resolveCodeAction:
				json.Unmarshal(r.Result, &action)
			}
			env.assertEditOnSource("retry", action.Edit)
		})
	}
}

// Locations on a .gox that changed on disk are mapped through the reloaded
// file, without an error.
func TestDocModelLocationOnSourceChangedOnDisk(t *testing.T) {
	t.Parallel()
	for _, m := range []method{definition, prepareCallHierarchy, outgoingCalls} {
		t.Run(string(m), func(t *testing.T) {
			env := newDocModelEnv(t)
			env.write(env.source, lspHelperSource)
			env.start(env.root)
			env.waitTarget("")
			env.write(env.source, docModelShifted())

			ran := docModelTitleRange(docModelGenerated(t, docModelShifted()))
			item := map[string]any{"uri": docModelURI(env.target), "name": "Title", "kind": 8, "range": ran, "selectionRange": ran}
			params := map[string]any{
				"textDocument": map[string]any{"uri": docModelURI(filepath.Join(env.root, "use.go"))},
				"position":     map[string]any{"line": 0, "character": 0},
			}
			if m == outgoingCalls {
				env.setResult([]any{map[string]any{"to": item, "fromRanges": []any{}}})
				params = map[string]any{"item": map[string]any{"uri": docModelURI(filepath.Join(env.root, "use.go")), "name": "Use", "kind": 12, "range": ran, "selectionRange": ran}}
			} else {
				env.setResult([]any{item})
			}
			r := env.call(Client, m, params)
			var items []struct {
				URI   string          `json:"uri"`
				Range json.RawMessage `json:"range"`
				To    struct {
					URI   string          `json:"uri"`
					Range json.RawMessage `json:"range"`
				} `json:"to"`
			}
			if r.Err != nil || json.Unmarshal(r.Result, &items) != nil || len(items) != 1 {
				t.Fatalf("%s: %s, error %v; traffic: %v", m, r.Result, r.Err, env.traffic())
			}
			if m == outgoingCalls {
				env.assertTitleOnSource(string(m), items[0].To.URI, items[0].To.Range)
			} else {
				env.assertTitleOnSource(string(m), items[0].URI, items[0].Range)
			}
			if env.watchedChanges() != 1 {
				t.Fatalf("gopls was not told the .x.go changed; traffic: %v", env.traffic())
			}
		})
	}
}

// Closing a .gox without saving rewrites a .x.go written from the buffer
// (restored by the ticker, or saved from the .x.go buffer), and gopls
// rereads it.
func TestDocModelDiscardedSourceRewritesTarget(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"restored", "saved"} {
		t.Run(mode, func(t *testing.T) {
			env := newDocModelEnv(t)
			env.write(env.source, lspHelperSource)
			env.start(env.root)
			env.waitTarget("")
			env.open(env.source, docModelEdited())
			if mode == "restored" {
				if err := os.Remove(env.target); err != nil {
					t.Fatal(err)
				}
			} else {
				env.openTarget()
				env.save(env.target)
			}
			env.waitTarget("Body string")
			env.close(env.source)
			if data, _ := os.ReadFile(env.target); strings.Contains(string(data), "Body string") {
				t.Fatalf("the .x.go keeps the discarded code:\n%s", data)
			}
			if mode == "restored" && env.watchedChanges() != 1 {
				t.Fatalf("gopls was not told the .x.go changed; traffic: %v", env.traffic())
			}
			if mode == "saved" && strings.Contains(env.lastGoplsText(), "Body string") {
				t.Fatalf("gopls keeps the discarded code; traffic: %v", env.traffic())
			}
		})
	}
}

// A .gox closed without saving while its file was briefly missing keeps the
// discarded text only until it is used: opening the .x.go gives gopls the
// file's code.
func TestDocModelDiscardedTextDroppedAfterFileFlicker(t *testing.T) {
	t.Parallel()
	env := newDocModelEnv(t)
	env.write(env.source, lspHelperSource)
	env.start(env.root)
	env.waitTarget("")
	env.open(env.source, docModelEdited())
	if err := os.Remove(env.source); err != nil {
		t.Fatal(err)
	}
	env.close(env.source)
	env.write(env.source, lspHelperSource)
	env.settle()
	env.openTarget()
	if strings.Contains(env.lastGoplsText(), "Body string") {
		t.Fatalf("opening the .x.go pushed the discarded code to gopls; traffic: %v", env.traffic())
	}
	env.close(env.target)
	env.assertOverlayLegal("closed", false)
}
