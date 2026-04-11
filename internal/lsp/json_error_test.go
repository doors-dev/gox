package lsp

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/doors-dev/gox/internal/common"
	"github.com/doors-dev/gox/internal/docpath"
	"github.com/doors-dev/gox/internal/workspace"
)

func TestJSONInitErrorCases(t *testing.T) {
	jsonInit.prepeareForVscodeExtension(mustJSONNode(t, `{}`))

	for _, src := range []string{
		`{}`,
		`{"clientInfo":{}}`,
		`{"clientInfo":{"name":123}}`,
	} {
		if jsonInit.isVscodeExtension(mustJSONNode(t, src)) {
			t.Fatalf("isVscodeExtension(%s) = true, want false", src)
		}
	}

	if _, err := jsonInit.getWorkspaceDirs(mustJSONNode(t, `{}`)); err == nil {
		t.Fatal("getWorkspaceDirs() error = nil, want missing workspace folders")
	}
	if _, err := jsonInit.getWorkspaceDirsFromArray(mustJSONNode(t, `{}`)); err == nil {
		t.Fatal("getWorkspaceDirsFromArray(object) error = nil")
	}
	if _, err := jsonInit.getWorkspaceDirsFromArray(mustJSONNode(t, `[{}]`)); err == nil {
		t.Fatal("getWorkspaceDirsFromArray(missing uri) error = nil")
	}
	if _, err := jsonInit.getWorkspaceDirsFromArray(mustJSONNode(t, `[{"uri":{}}]`)); err == nil {
		t.Fatal("getWorkspaceDirsFromArray(invalid uri) error = nil")
	}

	if enc, err := jsonInit.readEncoding(mustJSONNode(t, `{}`)); err == nil || enc != common.UTF16 {
		t.Fatalf("readEncoding(missing capabilities) = (%v, %v), want UTF16 + error", enc, err)
	}
	if enc, err := jsonInit.readEncoding(mustJSONNode(t, `{"capabilities":{"positionEncoding":123}}`)); err == nil || enc != common.UTF16 {
		t.Fatalf("readEncoding(invalid type) = (%v, %v), want UTF16 + error", enc, err)
	}
	if enc, err := jsonInit.readEncoding(mustJSONNode(t, `{"capabilities":{"positionEncoding":"utf-16"}}`)); err != nil || enc != common.UTF16 {
		t.Fatalf("readEncoding(utf-16) = (%v, %v)", enc, err)
	}
	if _, err := jsonInit.readEncoding(mustJSONNode(t, `{"capabilities":{"positionEncoding":"utf-32"}}`)); err == nil {
		t.Fatal("readEncoding(utf-32) error = nil")
	}

	if err := jsonInit.setEncodings(mustJSONNode(t, `{}`)); err != nil {
		t.Fatalf("setEncodings(no capabilities) error = %v", err)
	}
	if err := jsonInit.setEncodings(mustJSONNode(t, `{"capabilities":{"general":{"positionEncodings":123}}}`)); err != nil {
		t.Fatalf("setEncodings(non-array) error = %v", err)
	}
	utf16Only := mustJSONNode(t, `{"capabilities":{"general":{"positionEncodings":["utf-16"]}}}`)
	if err := jsonInit.setEncodings(utf16Only); err != nil {
		t.Fatalf("setEncodings(utf-16 only) error = %v", err)
	}
	values, err := utf16Only.Get("capabilities").Get("general").Get("positionEncoding").ArrayUseNode()
	if err != nil || len(values) != 1 || mustString(t, &values[0]) != "utf-16" {
		t.Fatalf("positionEncoding after utf-16 only = %#v, err = %v", values, err)
	}
	if err := jsonInit.setEncodings(mustJSONNode(t, `{"capabilities":{"general":{"positionEncodings":["utf-32"]}}}`)); err == nil {
		t.Fatal("setEncodings(unsupported only) error = nil")
	}

	if err := jsonInit.insertCompletionTriggers(mustJSONNode(t, `{}`)); err == nil {
		t.Fatal("insertCompletionTriggers(missing capabilities) error = nil")
	}
	if err := jsonInit.insertCompletionTriggers(mustJSONNode(t, `{"capabilities":{}}`)); err == nil {
		t.Fatal("insertCompletionTriggers(missing provider) error = nil")
	}
	if err := jsonInit.insertCompletionTriggers(mustJSONNode(t, `{"capabilities":{"completionProvider":{}}}`)); err == nil {
		t.Fatal("insertCompletionTriggers(missing triggers) error = nil")
	}

	if _, _, err := jsonInit.getWorkspaceChanges(mustJSONNode(t, `{}`)); err == nil {
		t.Fatal("getWorkspaceChanges(missing event) error = nil")
	}
	if _, _, err := jsonInit.getWorkspaceChanges(mustJSONNode(t, `{"event":{"added":[{"uri":{}}]}}`)); err == nil {
		t.Fatal("getWorkspaceChanges(invalid added uri) error = nil")
	}
	if _, _, err := jsonInit.getWorkspaceChanges(mustJSONNode(t, `{"event":{"removed":[{"uri":{}}]}}`)); err == nil {
		t.Fatal("getWorkspaceChanges(invalid removed uri) error = nil")
	}
}

func TestJSONDocErrorCases(t *testing.T) {
	man, doc := makeManagedDoc(t, lspHelperSource)

	for _, src := range []string{
		`{}`,
		`{"textDocument":{}}`,
	} {
		if _, err := jsonDoc.getText(mustJSONNode(t, src)); err == nil {
			t.Fatalf("getText(%s) error = nil", src)
		}
	}
	if _, err := jsonDoc.getText(mustJSONNode(t, `{"textDocument":{"text":{}}}`)); err == nil {
		t.Fatal("getText(invalid text) error = nil")
	}

	if _, _, err := jsonDoc.get(man, mustJSONNode(t, `{}`), false); err == nil {
		t.Fatal("get(missing uri) error = nil")
	}
	if gotDoc, kind, err := jsonDoc.get(man, mustJSONNode(t, `{"uri":1}`), false); err != nil || gotDoc != nil || kind != workspace.KindUnknown {
		t.Fatalf("get(non-file uri string) = (%v, %v, %v), want (nil, unknown, nil)", gotDoc, kind, err)
	}
	if gotDoc, kind, err := jsonDoc.get(man, mustJSONNode(t, fmt.Sprintf(`{"targetUri":%q}`, doc.TargetFile().URI())), false); err != nil || gotDoc == nil || kind != workspace.KindTarget {
		t.Fatalf("get(targetUri) = (%v, %v, %v)", gotDoc, kind, err)
	}
	if gotDoc, kind, err := jsonDoc.get(man, mustJSONNode(t, fmt.Sprintf(`{"item":{"uri":%q}}`, doc.TargetFile().URI())), false); err != nil || gotDoc == nil || kind != workspace.KindTarget {
		t.Fatalf("get(item.uri) = (%v, %v, %v)", gotDoc, kind, err)
	}

	targetOnlyDir := t.TempDir()
	targetOnlyPath := filepath.Join(targetOnlyDir, "ghost.x.go")
	if err := os.WriteFile(targetOnlyPath, []byte("package demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	targetOnlyMan := workspace.NewManager()
	targetOnlyMan.EnsureWorkspaces([]string{string(docpath.URIFromPath(targetOnlyDir))})
	gotDoc, kind, err := jsonDoc.get(targetOnlyMan, mustJSONNode(t, fmt.Sprintf(`{"uri":%q}`, docpath.URIFromPath(targetOnlyPath))), false)
	if err != nil || gotDoc != nil || kind != workspace.KindUnknown {
		t.Fatalf("get(target only, forEdit=false) = (%v, %v, %v)", gotDoc, kind, err)
	}

	for _, src := range []string{
		`{}`,
		`{"textDocument":{}}`,
		`{"textDocument":{"version":"x"}}`,
	} {
		if _, err := jsonDoc.getVersion(mustJSONNode(t, src)); err == nil {
			t.Fatalf("getVersion(%s) error = nil", src)
		}
	}

	uriNode := mustJSONNode(t, fmt.Sprintf(`{"uri":%q}`, doc.TargetFile().URI()))
	jsonDoc.setAsSource(uriNode, doc)
	if got := mustString(t, uriNode.Get("uri")); got != doc.SourceFile().URI() {
		t.Fatalf("setAsSource(uri) = %q, want %q", got, doc.SourceFile().URI())
	}

	targetURINode := mustJSONNode(t, fmt.Sprintf(`{"targetUri":%q}`, doc.SourceFile().URI()))
	jsonDoc.setAsTarget(targetURINode, doc)
	if got := mustString(t, targetURINode.Get("targetUri")); got != doc.TargetFile().URI() {
		t.Fatalf("setAsTarget(targetUri) = %q, want %q", got, doc.TargetFile().URI())
	}

	itemNode := mustJSONNode(t, fmt.Sprintf(`{"item":{"uri":%q}}`, doc.TargetFile().URI()))
	jsonDoc.setAsSource(itemNode, doc)
	if got := mustString(t, itemNode.Get("item").Get("uri")); got != doc.SourceFile().URI() {
		t.Fatalf("setAsSource(item.uri) = %q, want %q", got, doc.SourceFile().URI())
	}

	jsonDoc.setVersion(mustJSONNode(t, `{}`), 7)
}

func TestJSONChangesAndPositionErrorCases(t *testing.T) {
	man, doc := makeManagedDoc(t, lspHelperSource)
	srcRange := sourceRangeFor(t, doc.SourceFile().Path(), "Card struct")
	targetRange := mustTargetRange(t, doc, srcRange)

	invalidDefaults := mustJSONNode(t, `{"itemDefaults":{"position":1},"items":[{"label":"keep"}]}`)
	jsonChanges.convertCompletions(common.UTF8, doc, invalidDefaults)
	items, err := invalidDefaults.Get("items").ArrayUseNode()
	if err != nil || len(items) != 0 {
		t.Fatalf("convertCompletions(invalid defaults) items = %#v, err = %v", items, err)
	}

	if err := jsonChanges.convertInlayHints(common.UTF8, doc, mustJSONNode(t, `null`)); err != nil {
		t.Fatalf("convertInlayHints(null) error = %v", err)
	}
	if err := jsonChanges.convertInlayHints(common.UTF8, doc, mustJSONNode(t, `{"bad":{}}`)); err == nil {
		t.Fatal("convertInlayHints(object) error = nil")
	}
	if err := jsonChanges.convertInlayHints(common.UTF8, doc, mustJSONNode(t, fmt.Sprintf(`[
		{"position":%s,"textEdits":{"bad":1}}
	]`, jsonFromNode(t, jsonPos.fromPos(srcRange.Beg()))))); err == nil {
		t.Fatal("convertInlayHints(invalid textEdits) error = nil")
	}

	if err := jsonChanges.convertCodeActions(man, common.UTF8, doc, mustJSONNode(t, `null`)); err != nil {
		t.Fatalf("convertCodeActions(null) error = %v", err)
	}
	if err := jsonChanges.convertCodeActions(man, common.UTF8, doc, mustJSONNode(t, `{}`)); err == nil {
		t.Fatal("convertCodeActions(object) error = nil")
	}

	if _, err := jsonChanges.getChanges(mustJSONNode(t, `{}`)); err == nil {
		t.Fatal("getChanges(missing contentChanges) error = nil")
	}
	if changes, err := jsonChanges.getChanges(mustJSONNode(t, `{"contentChanges":[{"text":{}}]}`)); err != nil || len(changes) != 1 || changes[0].Text != "" {
		t.Fatalf("getChanges(invalid text) = (%#v, %v)", changes, err)
	}
	if _, err := jsonChanges.getChanges(mustJSONNode(t, `{"contentChanges":[{"range":{}}]}`)); err == nil {
		t.Fatal("getChanges(invalid range) error = nil")
	}
	if got := jsonChanges.getText(mustJSONNode(t, `{}`)); got != "" {
		t.Fatalf("getText(missing) = %q, want empty", got)
	}
	if got := jsonChanges.getText(mustJSONNode(t, `{"text":{}}`)); got != "" {
		t.Fatalf("getText(invalid) = %q, want empty", got)
	}

	if _, err := jsonPos.getRange(mustJSONNode(t, `{}`)); err == nil {
		t.Fatal("getRange(missing) error = nil")
	}
	if _, err := jsonPos.getPos(mustJSONNode(t, `{}`)); err == nil {
		t.Fatal("getPos(missing) error = nil")
	}
	if _, err := jsonPos.intoRange(mustJSONNode(t, `{}`)); err == nil {
		t.Fatal("intoRange(missing start/end) error = nil")
	}
	if _, err := jsonPos.intoPos(mustJSONNode(t, `{"line":"x","character":0}`)); err == nil {
		t.Fatal("intoPos(invalid line) error = nil")
	}

	tryRange := mustJSONNode(t, fmt.Sprintf(`{"position":%s,"range":{}}`, jsonFromNode(t, jsonPos.fromPos(srcRange.Beg()))))
	if err := jsonPos.convertPosTryRangeToTarget(common.UTF8, doc, tryRange, workspace.Strict); err != nil {
		t.Fatalf("convertPosTryRangeToTarget() error = %v", err)
	}
	if tryRange.Get("range").Exists() {
		t.Fatal("convertPosTryRangeToTarget() did not unset invalid range")
	}

	if err := jsonPos.convertLocations(man, common.UTF8, nil, mustJSONNode(t, `{}`)); err != nil {
		t.Fatalf("convertLocations(no uri, no origin) error = %v", err)
	}
	if err := jsonPos.convertCalls(man, common.UTF8, mustJSONNode(t, fmt.Sprintf(`[
		{"from":{"uri":%q,"range":%s,"selectionRange":%s},"fromRanges":[%s]}
	]`, doc.SourceFile().URI(),
		jsonFromNode(t, jsonPos.fromRange(srcRange)),
		jsonFromNode(t, jsonPos.fromRange(srcRange)),
		jsonFromNode(t, jsonPos.fromRange(srcRange))))); err == nil {
		t.Fatal("convertCalls(source uri) error = nil")
	}
	if err := jsonPos.convertOutgoingCalls(man, common.UTF8, doc, mustJSONNode(t, fmt.Sprintf(`[
		{"to":{"uri":%q,"range":%s,"selectionRange":%s},"fromRanges":[%s]}
	]`, doc.SourceFile().URI(),
		jsonFromNode(t, jsonPos.fromRange(srcRange)),
		jsonFromNode(t, jsonPos.fromRange(srcRange)),
		jsonFromNode(t, jsonPos.fromRange(targetRange))))); err == nil {
		t.Fatal("convertOutgoingCalls(source callee) error = nil")
	}
}
