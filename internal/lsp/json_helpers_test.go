package lsp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	symbolcatalog "github.com/doors-dev/gox/internal/catalog/symbol"
	"github.com/doors-dev/gox/internal/common"
	"github.com/doors-dev/gox/internal/docpath"
	"github.com/doors-dev/gox/internal/workspace"
)

const lspHelperSource = `package demo

import "github.com/doors-dev/gox"

type Card struct {
	Title string
}

elem View(name string) {
	<div class="card">~(name)</div>
}
`

func TestJSONInitHelpers(t *testing.T) {
	j := mustJSONNode(t, `{
		"clientInfo":{"name":"Codex [GOX_EXT]"},
		"capabilities":{
			"foldingRangeProvider":true,
			"semanticTokensProvider":true,
			"general":{"positionEncodings":["utf-16","utf-8"]}
		},
		"workspaceFolders":[{"uri":"file:///a"},{"uri":"file:///b"}]
	}`)

	if !jsonInit.isVscodeExtension(j) {
		t.Fatal("isVscodeExtension() = false, want true")
	}
	jsonInit.prepeareForVscodeExtension(j)
	if j.Get("capabilities").Get("foldingRangeProvider").Exists() {
		t.Fatal("foldingRangeProvider was not removed")
	}
	if j.Get("capabilities").Get("semanticTokensProvider").Exists() {
		t.Fatal("semanticTokensProvider was not removed")
	}

	dirs, err := jsonInit.getWorkspaceDirs(j)
	if err != nil {
		t.Fatalf("getWorkspaceDirs() error = %v", err)
	}
	if len(dirs) != 2 || dirs[0] != "file:///a" || dirs[1] != "file:///b" {
		t.Fatalf("workspace dirs = %#v", dirs)
	}

	enc, err := jsonInit.readEncoding(mustJSONNode(t, `{"capabilities":{"positionEncoding":"utf-8"}}`))
	if err != nil || enc != common.UTF8 {
		t.Fatalf("readEncoding(utf-8) = (%v, %v)", enc, err)
	}
	if _, err := jsonInit.readEncoding(mustJSONNode(t, `{"capabilities":{"positionEncoding":"utf-32"}}`)); err == nil {
		t.Fatal("readEncoding(utf-32) error = nil, want error")
	}

	if err := jsonInit.setEncodings(j); err != nil {
		t.Fatalf("setEncodings() error = %v", err)
	}
	positionEncoding := j.Get("capabilities").Get("general").Get("positionEncoding")
	values, err := positionEncoding.ArrayUseNode()
	if err != nil || len(values) != 1 {
		t.Fatalf("positionEncoding = %#v, err = %v", positionEncoding, err)
	}
	gotEncoding, _ := values[0].String()
	if gotEncoding != "utf-8" {
		t.Fatalf("positionEncoding[0] = %q, want utf-8", gotEncoding)
	}

	serverCaps := mustJSONNode(t, `{"capabilities":{"completionProvider":{"triggerCharacters":["."]}}}`)
	if err := jsonInit.insertCompletionTriggers(serverCaps); err != nil {
		t.Fatalf("insertCompletionTriggers() error = %v", err)
	}
	triggers, err := serverCaps.Get("capabilities").Get("completionProvider").Get("triggerCharacters").ArrayUseNode()
	if err != nil || len(triggers) != 4 {
		t.Fatalf("triggerCharacters = %#v, err = %v", triggers, err)
	}

	added, removed, err := jsonInit.getWorkspaceChanges(mustJSONNode(t, `{
		"event":{
			"added":[{"uri":"file:///one"}],
			"removed":[{"uri":"file:///two"}]
		}
	}`))
	if err != nil {
		t.Fatalf("getWorkspaceChanges() error = %v", err)
	}
	if len(added) != 1 || len(removed) != 1 || added[0] != "file:///one" || removed[0] != "file:///two" {
		t.Fatalf("workspace changes = added:%#v removed:%#v", added, removed)
	}
}

func TestJSONDocAndGeneratorHelpers(t *testing.T) {
	man, doc := makeManagedDoc(t, lspHelperSource)
	srcRange := sourceRangeFor(t, doc.SourceFile().Path(), "Card struct")

	j := mustJSONNode(t, fmt.Sprintf(`{
		"textDocument":{
			"uri":%q,
			"text":"hello",
			"languageId":"gox",
			"version":1
		}
	}`, doc.SourceFile().URI()))

	text, err := jsonDoc.getText(j)
	if err != nil || text != "hello" {
		t.Fatalf("getText() = (%q, %v)", text, err)
	}
	gotDoc, kind, err := jsonDoc.get(man, j, false)
	if err != nil || kind != workspace.KindSource || gotDoc.SourceFile().URI() != doc.SourceFile().URI() {
		t.Fatalf("get() = (%v, %v, %v)", gotDoc, kind, err)
	}
	jsonDoc.setVersion(j, 7)
	version, err := jsonDoc.getVersion(j)
	if err != nil || version != 7 {
		t.Fatalf("getVersion() = (%d, %v)", version, err)
	}
	jsonDoc.setAsTarget(j, doc)
	if got := j.Get("textDocument").Get("uri"); mustString(t, got) != doc.TargetFile().URI() {
		t.Fatalf("target uri = %q, want %q", mustString(t, got), doc.TargetFile().URI())
	}
	if got := j.Get("textDocument").Get("languageId"); mustString(t, got) != "go" {
		t.Fatalf("languageId = %q, want go", mustString(t, got))
	}
	if got := j.Get("textDocument").Get("text"); mustString(t, got) != doc.TargetContent() {
		t.Fatal("textDocument.text was not replaced with target content")
	}

	fromNode := mustJSONNode(t, fmt.Sprintf(`{"from":{"uri":%q}}`, doc.TargetFile().URI()))
	gotDoc, kind, err = jsonDoc.get(man, fromNode, false)
	if err != nil || kind != workspace.KindTarget || gotDoc.SourceFile().URI() != doc.SourceFile().URI() {
		t.Fatalf("get(from) = (%v, %v, %v)", gotDoc, kind, err)
	}
	jsonDoc.setAsSource(fromNode, doc)
	if got := fromNode.Get("from").Get("uri"); mustString(t, got) != doc.SourceFile().URI() {
		t.Fatalf("from.uri = %q, want %q", mustString(t, got), doc.SourceFile().URI())
	}

	if arr := jsonGenerator.newEmptyArray(); arr.TypeSafe() != ast.V_ARRAY {
		t.Fatalf("newEmptyArray() type = %v, want array", arr.TypeSafe())
	}
	if nul := jsonGenerator.newNull(); nul.TypeSafe() != ast.V_NULL {
		t.Fatalf("newNull() type = %v, want null", nul.TypeSafe())
	}
	noTokens := jsonGenerator.newNoSemanticTokens()
	items, err := noTokens.Get("data").ArrayUseNode()
	if err != nil || len(items) != 0 {
		t.Fatalf("newNoSemanticTokens() data = %#v, err = %v", items, err)
	}

	edit := jsonGenerator.newTextEdits(srcRange, "renamed")
	edits, err := edit.ArrayUseNode()
	if err != nil || len(edits) != 1 {
		t.Fatalf("newTextEdits() = %#v, err = %v", edits, err)
	}
	if got := edits[0].Get("newText"); mustString(t, got) != "renamed" {
		t.Fatalf("newText = %q, want renamed", mustString(t, got))
	}
	if empty, err := jsonGenerator.newTextEdits(common.NoRange(), "ignored").ArrayUseNode(); err != nil || len(empty) != 0 {
		t.Fatalf("newTextEdits(NoRange) = %#v, err = %v", empty, err)
	}

	symbols := jsonGenerator.newSymbols([]workspace.Symbol{{
		Kind:  symbolcatalog.Struct,
		Name:  "Card",
		Range: srcRange,
		Symbols: []workspace.Symbol{{
			Kind:  symbolcatalog.Field,
			Name:  "Title",
			Range: srcRange,
		}},
	}})
	symbolArr, err := symbols.ArrayUseNode()
	if err != nil || len(symbolArr) != 1 {
		t.Fatalf("newSymbols() = %#v, err = %v", symbolArr, err)
	}
	children, err := symbolArr[0].Get("children").ArrayUseNode()
	if err != nil || len(children) != 1 {
		t.Fatalf("symbol children = %#v, err = %v", children, err)
	}

	hover := jsonGenerator.newHover(srcRange, "hover text")
	if got := hover.Get("contents").Get("value"); mustString(t, got) != "hover text" {
		t.Fatalf("hover value = %q, want hover text", mustString(t, got))
	}
	insert := jsonGenerator.newInsertEdit(doc.SourceFile().URI(), srcRange.Beg(), "x", "insert")
	if got := insert.Get("label"); mustString(t, got) != "insert" {
		t.Fatalf("insert label = %q, want insert", mustString(t, got))
	}
	update := jsonGenerator.newUpdateEdit(doc.SourceFile().URI(), "body", "update")
	if got := update.Get("label"); mustString(t, got) != "update" {
		t.Fatalf("update label = %q, want update", mustString(t, got))
	}

	completions := jsonGenerator.newCompletions([]workspace.Completion{{
		Range: srcRange,
		Text:  "Card",
		Label: "Card",
	}}, true)
	if got := completions.Get("isIncomplete"); mustBool(t, got) {
		t.Fatal("isIncomplete = true, want false")
	}
	itemsNode, err := completions.Get("items").ArrayUseNode()
	if err != nil || len(itemsNode) != 1 {
		t.Fatalf("completion items = %#v, err = %v", itemsNode, err)
	}
	if got := itemsNode[0].Get("label"); mustString(t, got) != "Card" {
		t.Fatalf("completion label = %q, want Card", mustString(t, got))
	}
}

func TestJSONChangesAndPositionHelpers(t *testing.T) {
	man, doc := makeManagedDoc(t, lspHelperSource)
	srcRange := sourceRangeForLast(t, doc.SourceFile().Path(), "name")
	targetRange := mustTargetRange(t, doc, srcRange)
	sourcePos := srcRange.Beg()
	targetPos := mustTargetPos(t, doc, sourcePos)

	changesNode := mustJSONNode(t, fmt.Sprintf(`{
		"contentChanges":[
			{"text":"full"},
			{"range":%s,"text":"patch"}
		]
	}`, jsonFromNode(t, jsonPos.fromRange(srcRange))))
	changes, err := jsonChanges.getChanges(changesNode)
	if err != nil {
		t.Fatalf("getChanges() error = %v", err)
	}
	if len(changes) != 2 || changes[0].Range.IsValid() || changes[1].Range != srcRange {
		t.Fatalf("changes = %#v", changes)
	}
	jsonChanges.setUpdate(changesNode, "updated")
	updatedText := mustString(t, mustArrayIndex(t, changesNode.Get("contentChanges"), 0).Get("text"))
	if updatedText != "updated" {
		t.Fatalf("contentChanges[0].text = %q, want updated", updatedText)
	}

	completionsNode := mustJSONNode(t, fmt.Sprintf(`{
		"items":[
			{"label":"name","textEdit":{"range":%s,"newText":"name"}}
		]
	}`, jsonFromNode(t, jsonPos.fromRange(targetRange))))
	jsonChanges.convertCompletions(common.UTF8, doc, completionsNode)
	gotCompletionRange, err := jsonPos.intoRange(mustArrayIndex(t, completionsNode.Get("items"), 0).Get("textEdit").Get("range"))
	if err != nil || gotCompletionRange != srcRange {
		t.Fatalf("completion range = %v, err = %v", gotCompletionRange, err)
	}

	inlayHints := mustJSONNode(t, fmt.Sprintf(`[
		{"position":%s,"textEdits":[{"range":%s,"newText":"renamed"}]}
	]`, jsonFromNode(t, jsonPos.fromPos(sourcePos)), jsonFromNode(t, jsonPos.fromRange(srcRange))))
	if err := jsonChanges.convertInlayHints(common.UTF8, doc, inlayHints); err != nil {
		t.Fatalf("convertInlayHints() error = %v", err)
	}
	gotPos, err := jsonPos.intoPos(mustArrayIndex(t, inlayHints, 0).Get("position"))
	if err != nil || gotPos != targetPos {
		t.Fatalf("inlay hint pos = %v, err = %v", gotPos, err)
	}
	gotHintRange, err := jsonPos.intoRange(mustArrayIndex(t, mustArrayIndex(t, inlayHints, 0).Get("textEdits"), 0).Get("range"))
	if err != nil || gotHintRange != targetRange {
		t.Fatalf("inlay hint range = %v, err = %v", gotHintRange, err)
	}

	editNode := mustJSONNode(t, fmt.Sprintf(`{
		"documentChanges":[
			{
				"textDocument":{"uri":%q},
				"edits":[{"range":%s,"newText":"renamed"}]
			}
		]
	}`, doc.SourceFile().URI(), jsonFromNode(t, jsonPos.fromRange(srcRange))))
	if err := jsonChanges.convertEdit(man, common.UTF8, editNode); err != nil {
		t.Fatalf("convertEdit() error = %v", err)
	}
	firstChange := mustArrayIndex(t, editNode.Get("documentChanges"), 0)
	if got := firstChange.Get("textDocument").Get("uri"); mustString(t, got) != doc.TargetFile().URI() {
		t.Fatalf("textDocument.uri = %q, want %q", mustString(t, got), doc.TargetFile().URI())
	}
	gotEditRange, err := jsonPos.intoRange(mustArrayIndex(t, firstChange.Get("edits"), 0).Get("range"))
	if err != nil || gotEditRange != targetRange {
		t.Fatalf("edit range = %v, err = %v", gotEditRange, err)
	}

	toTarget := mustJSONNode(t, fmt.Sprintf(`{"diagnostics":[{"range":%s,"message":"x"}]}`, jsonFromNode(t, jsonPos.fromRange(srcRange))))
	if err := jsonPos.convertDiagnosticsToTarget(man, common.UTF8, doc, toTarget); err != nil {
		t.Fatalf("convertDiagnosticsToTarget() error = %v", err)
	}
	gotTargetRange, err := jsonPos.intoRange(mustArrayIndex(t, toTarget.Get("diagnostics"), 0).Get("range"))
	if err != nil || gotTargetRange != targetRange {
		t.Fatalf("target diagnostic range = %v, err = %v", gotTargetRange, err)
	}

	toSource := mustJSONNode(t, fmt.Sprintf(`{
		"items":[{"range":%s,"message":"x"}],
		"relatedDocuments":{
			%q:{"items":[{"range":%s,"message":"rel"}]}
		}
	}`, jsonFromNode(t, jsonPos.fromRange(targetRange)), doc.TargetFile().URI(), jsonFromNode(t, jsonPos.fromRange(targetRange))))
	if err := jsonPos.convertDiagnosticsToSource(man, common.UTF8, doc, toSource); err != nil {
		t.Fatalf("convertDiagnosticsToSource() error = %v", err)
	}
	gotSourceRange, err := jsonPos.intoRange(mustArrayIndex(t, toSource.Get("items"), 0).Get("range"))
	if err != nil || gotSourceRange != srcRange {
		t.Fatalf("source diagnostic range = %v, err = %v", gotSourceRange, err)
	}
	if !toSource.Get("relatedDocuments").Get(doc.SourceFile().URI()).Exists() {
		t.Fatalf("relatedDocuments missing source uri %q", doc.SourceFile().URI())
	}

	itemNode := mustJSONNode(t, fmt.Sprintf(`{"item":{"range":%s,"selectionRange":%s}}`,
		jsonFromNode(t, jsonPos.fromRange(srcRange)),
		jsonFromNode(t, jsonPos.fromRange(srcRange)),
	))
	if err := jsonPos.convertItemRanges(common.UTF8, doc, itemNode, convertToTarget, workspace.Strict); err != nil {
		t.Fatalf("convertItemRanges() error = %v", err)
	}
	gotItemRange, err := jsonPos.intoRange(itemNode.Get("item").Get("range"))
	if err != nil || gotItemRange != targetRange {
		t.Fatalf("item range = %v, err = %v", gotItemRange, err)
	}

	locations := mustJSONNode(t, fmt.Sprintf(`{
		"uri":%q,
		"range":%s,
		"selectionRange":%s,
		"originSelectionRange":%s
	}`, doc.TargetFile().URI(),
		jsonFromNode(t, jsonPos.fromRange(targetRange)),
		jsonFromNode(t, jsonPos.fromRange(targetRange)),
		jsonFromNode(t, jsonPos.fromRange(targetRange)),
	))
	if err := jsonPos.convertLocations(man, common.UTF8, doc, locations); err != nil {
		t.Fatalf("convertLocations() error = %v", err)
	}
	if got := locations.Get("uri"); mustString(t, got) != doc.SourceFile().URI() {
		t.Fatalf("location uri = %q, want %q", mustString(t, got), doc.SourceFile().URI())
	}
	gotLocationRange, err := jsonPos.intoRange(locations.Get("range"))
	if err != nil || gotLocationRange != srcRange {
		t.Fatalf("location range = %v, err = %v", gotLocationRange, err)
	}

	outgoing := mustJSONNode(t, fmt.Sprintf(`[
		{
			"to":{"uri":%q,"range":%s,"selectionRange":%s},
			"fromRanges":[%s]
		}
	]`, doc.TargetFile().URI(),
		jsonFromNode(t, jsonPos.fromRange(targetRange)),
		jsonFromNode(t, jsonPos.fromRange(targetRange)),
		jsonFromNode(t, jsonPos.fromRange(targetRange)),
	))
	if err := jsonPos.convertOutgoingCalls(man, common.UTF8, doc, outgoing); err != nil {
		t.Fatalf("convertOutgoingCalls() error = %v", err)
	}
	if got := mustArrayIndex(t, outgoing, 0).Get("to").Get("uri"); mustString(t, got) != doc.SourceFile().URI() {
		t.Fatalf("outgoing to.uri = %q, want %q", mustString(t, got), doc.SourceFile().URI())
	}
	gotFromRange, err := jsonPos.intoRange(mustArrayIndex(t, mustArrayIndex(t, outgoing, 0).Get("fromRanges"), 0))
	if err != nil || gotFromRange != srcRange {
		t.Fatalf("fromRanges[0] = %v, err = %v", gotFromRange, err)
	}

	incoming := mustJSONNode(t, fmt.Sprintf(`[
		{
			"from":{"uri":%q,"range":%s,"selectionRange":%s},
			"fromRanges":[%s]
		}
	]`, doc.TargetFile().URI(),
		jsonFromNode(t, jsonPos.fromRange(targetRange)),
		jsonFromNode(t, jsonPos.fromRange(targetRange)),
		jsonFromNode(t, jsonPos.fromRange(targetRange)),
	))
	if err := jsonPos.convertCalls(man, common.UTF8, incoming); err != nil {
		t.Fatalf("convertCalls() error = %v", err)
	}
	if got := mustArrayIndex(t, incoming, 0).Get("from").Get("uri"); mustString(t, got) != doc.SourceFile().URI() {
		t.Fatalf("incoming from.uri = %q, want %q", mustString(t, got), doc.SourceFile().URI())
	}
	gotIncomingRange, err := jsonPos.intoRange(mustArrayIndex(t, mustArrayIndex(t, incoming, 0).Get("fromRanges"), 0))
	if err != nil || gotIncomingRange != srcRange {
		t.Fatalf("incoming fromRanges[0] = %v, err = %v", gotIncomingRange, err)
	}

	posToSource := mustJSONNode(t, fmt.Sprintf(`{"position":%s}`, jsonFromNode(t, jsonPos.fromPos(targetPos))))
	if err := jsonPos.convertPosToSource(common.UTF8, doc, posToSource, workspace.Strict); err != nil {
		t.Fatalf("convertPosToSource() error = %v", err)
	}
	gotSourcePos, err := jsonPos.intoPos(posToSource.Get("position"))
	if err != nil || gotSourcePos != sourcePos {
		t.Fatalf("source pos = %v, err = %v", gotSourcePos, err)
	}

	posToTarget := mustJSONNode(t, fmt.Sprintf(`{"position":%s}`, jsonFromNode(t, jsonPos.fromPos(sourcePos))))
	if err := jsonPos.convertPosToTarget(common.UTF8, doc, posToTarget, workspace.Strict); err != nil {
		t.Fatalf("convertPosToTarget() error = %v", err)
	}
	gotTargetPosFromDirect, err := jsonPos.intoPos(posToTarget.Get("position"))
	if err != nil || gotTargetPosFromDirect != targetPos {
		t.Fatalf("target pos = %v, err = %v", gotTargetPosFromDirect, err)
	}

	allToSource := mustJSONNode(t, fmt.Sprintf(`{
		"position":%s,
		"range":%s,
		"selectionRange":%s,
		"fromRanges":[%s]
	}`,
		jsonFromNode(t, jsonPos.fromPos(targetPos)),
		jsonFromNode(t, jsonPos.fromRange(targetRange)),
		jsonFromNode(t, jsonPos.fromRange(targetRange)),
		jsonFromNode(t, jsonPos.fromRange(targetRange)),
	))
	if err := jsonPos.convertAllToSource(common.UTF8, doc, allToSource, workspace.Strict); err != nil {
		t.Fatalf("convertAllToSource() error = %v", err)
	}
	gotAllPos, err := jsonPos.intoPos(allToSource.Get("position"))
	if err != nil || gotAllPos != sourcePos {
		t.Fatalf("all position = %v, err = %v", gotAllPos, err)
	}
	gotAllRange, err := jsonPos.intoRange(allToSource.Get("range"))
	if err != nil || gotAllRange != srcRange {
		t.Fatalf("all range = %v, err = %v", gotAllRange, err)
	}

	rangeNode := mustJSONNode(t, `{"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}`)
	jsonPos.setRange(rangeNode, srcRange)
	gotSetRange, err := jsonPos.intoRange(rangeNode.Get("range"))
	if err != nil || gotSetRange != srcRange {
		t.Fatalf("setRange() produced %v, err = %v", gotSetRange, err)
	}
	jsonPos.unsetRange(rangeNode)
	if rangeNode.Get("range").Exists() {
		t.Fatal("unsetRange() did not remove range")
	}

	posNode := mustJSONNode(t, `{"position":{"line":0,"character":0}}`)
	jsonPos.setPos(posNode, sourcePos)
	gotSetPos, err := jsonPos.intoPos(posNode.Get("position"))
	if err != nil || gotSetPos != sourcePos {
		t.Fatalf("setPos() produced %v, err = %v", gotSetPos, err)
	}
}

func mustJSONNode(t *testing.T, src string) Json {
	t.Helper()
	node, err := sonic.Get([]byte(src))
	if err != nil {
		t.Fatalf("sonic.Get(%s): %v", src, err)
	}
	return &node
}

func makeManagedDoc(t *testing.T, src string) (workspace.Manager, workspace.Doc) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "view.gox")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
	man := workspace.NewManager()
	man.EnsureWorkspaces([]string{string(docpath.URIFromPath(dir))})
	doc, kind := man.Doc(string(docpath.URIFromPath(path)))
	if kind != workspace.KindSource || doc == nil {
		t.Fatalf("man.Doc(%s) = (%v, %v), want source doc", path, doc, kind)
	}
	if err := doc.Err(); err != nil {
		t.Fatalf("doc.Err() = %v", err)
	}
	return man, doc
}

func sourceRangeFor(t *testing.T, path string, needle string) common.Range {
	t.Helper()
	return sourceRangeForIndex(t, path, strings.Index, needle)
}

func sourceRangeForLast(t *testing.T, path string, needle string) common.Range {
	t.Helper()
	return sourceRangeForIndex(t, path, strings.LastIndex, needle)
}

func sourceRangeForIndex(t *testing.T, path string, find func(string, string) int, needle string) common.Range {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	index := find(string(data), needle)
	if index < 0 {
		t.Fatalf("%q not found in %s", needle, path)
	}
	start := byteOffsetToPos(data, index)
	end := byteOffsetToPos(data, index+len(needle))
	return common.NewRange(start, end)
}

func byteOffsetToPos(data []byte, offset int) common.Pos {
	line := 0
	col := 0
	for i := 0; i < offset; i++ {
		if data[i] == '\n' {
			line++
			col = 0
			continue
		}
		col++
	}
	return common.NewPos(line, col)
}

func mustTargetRange(t *testing.T, doc workspace.Doc, source common.Range) common.Range {
	t.Helper()
	target, ok := doc.TargetRange(common.UTF8, source, workspace.Strict)
	if !ok {
		t.Fatalf("TargetRange(%v) = !ok", source)
	}
	return target
}

func mustTargetPos(t *testing.T, doc workspace.Doc, source common.Pos) common.Pos {
	t.Helper()
	target, ok := doc.TargetPos(common.UTF8, source, workspace.Strict)
	if !ok {
		t.Fatalf("TargetPos(%v) = !ok", source)
	}
	return target
}

func mustString(t *testing.T, node Json) string {
	t.Helper()
	value, err := node.String()
	if err != nil {
		t.Fatalf("String() error = %v", err)
	}
	return value
}

func mustBool(t *testing.T, node Json) bool {
	t.Helper()
	value, err := node.Bool()
	if err != nil {
		t.Fatalf("Bool() error = %v", err)
	}
	return value
}

func mustInt64(t *testing.T, node Json) int64 {
	t.Helper()
	value, err := node.Int64()
	if err != nil {
		t.Fatalf("Int64() error = %v", err)
	}
	return value
}

func mustArrayIndex(t *testing.T, node Json, index int) *ast.Node {
	t.Helper()
	arr, err := node.ArrayUseNode()
	if err != nil {
		t.Fatalf("ArrayUseNode() error = %v", err)
	}
	if index < 0 || index >= len(arr) {
		t.Fatalf("array index %d out of bounds for len %d", index, len(arr))
	}
	return &arr[index]
}

func jsonFromNode(t *testing.T, node ast.Node) string {
	t.Helper()
	data, err := node.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}
	return compactJSON(t, data)
}

func compactJSON(t *testing.T, data []byte) string {
	t.Helper()
	var out bytes.Buffer
	if err := json.Compact(&out, data); err != nil {
		t.Fatalf("json.Compact() error = %v", err)
	}
	return out.String()
}
