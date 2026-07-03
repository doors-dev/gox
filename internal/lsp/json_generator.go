package lsp

import (
	"math"
	"sort"
	"strings"

	"github.com/bytedance/sonic/ast"
	"github.com/doors-dev/gox/internal/common"
	"github.com/doors-dev/gox/internal/workspace"
)

var jsonGenerator jsonGeneratorDriver

type jsonGeneratorDriver struct{}

func (r jsonGeneratorDriver) newEmptyArray() Json {
	node := ast.NewArray(nil)
	return &node
}

func (r jsonGeneratorDriver) newNull() Json {
	node := ast.NewNull()
	return &node
}

func (r jsonGeneratorDriver) newSymbols(symbols []workspace.Symbol) Json {
	arr := make([]ast.Node, 0, len(symbols))
	for _, s := range symbols {
		sym := ast.NewObject(nil)
		sym.Set("kind", ast.NewAny(int(s.Kind)))
		sym.Set("name", ast.NewString(s.Name))
		ran := jsonPos.fromRange(s.Range)
		sym.Set("range", ran)
		sym.Set("selectionRange", ran)
		if len(s.Symbols) > 0 {
			sym.Set("children", *r.newSymbols(s.Symbols))
		}
		arr = append(arr, sym)
	}
	node := ast.NewArray(arr)
	return &node
}

func (r jsonGeneratorDriver) newNoSemanticTokens() Json {
	pair := ast.NewPair("data", ast.NewArray([]ast.Node{}))
	node := ast.NewObject([]ast.Pair{pair})
	return &node
}

func (r jsonGeneratorDriver) newTextEdits(rang common.Range, content string) Json {
	if !rang.IsValid() {
		return r.newEmptyArray()
	}
	newText := ast.NewPair("newText", ast.NewString(content))
	ran := ast.NewPair("range", jsonPos.fromRange(rang))
	arr := ast.NewArray([]ast.Node{ast.NewObject([]ast.Pair{newText, ran})})
	return &arr
}

func (r jsonGeneratorDriver) newEdit(uri string, ran common.Range, content string, message string) Json {
	messageNode := ast.NewPair("label", ast.NewString(message))
	rang := ast.NewPair("range", jsonPos.fromRange(ran))
	text := ast.NewPair("newText", ast.NewString(content))
	textEdit := ast.NewObject([]ast.Pair{rang, text})
	docEdits := ast.NewPair(uri, ast.NewArray([]ast.Node{textEdit}))
	changes := ast.NewPair("changes", ast.NewObject([]ast.Pair{docEdits}))
	edit := ast.NewPair("edit", ast.NewObject([]ast.Pair{changes}))
	node := ast.NewObject([]ast.Pair{messageNode, edit})
	return &node
}
func (r jsonGeneratorDriver) newInsertEdit(uri string, pos common.Pos, content string, message string) Json {
	return r.newEdit(uri, common.NewRange(
		pos,
		pos,
	), content, message)
}

func (r jsonGeneratorDriver) newUpdateEdit(uri string, content string, message string) Json {
	return r.newEdit(uri, common.NewRange(
		common.NewPos(0, 0),
		common.NewPos(int(math.MaxInt32), 0),
	), content, message)
}

func (r jsonGeneratorDriver) newCodeActions(actions []workspace.CodeAction) Json {
	arr := make([]ast.Node, 0, len(actions))
	for _, a := range actions {
		arr = append(arr, r.newCodeActionNode(a))
	}
	node := ast.NewArray(arr)
	return &node
}

func (r jsonGeneratorDriver) appendCodeActions(res Json, actions []workspace.CodeAction) {
	if len(actions) == 0 {
		return
	}
	var arr []ast.Node
	if res != nil && res.Exists() && res.TypeSafe() == ast.V_ARRAY {
		if existing, err := res.ArrayUseNode(); err == nil {
			arr = append(arr, existing...)
		}
	}
	for _, a := range actions {
		arr = append(arr, r.newCodeActionNode(a))
	}
	node := ast.NewArray(arr)
	*res = node
}

func (r jsonGeneratorDriver) newCodeActionNode(a workspace.CodeAction) ast.Node {
	var docChanges []ast.Node
	if a.CreateFile != nil {
		docChanges = append(docChanges, ast.NewObject([]ast.Pair{
			ast.NewPair("kind", ast.NewString("create")),
			ast.NewPair("uri", ast.NewString(a.CreateFile.URI)),
		}))
		zero := common.NewPos(0, 0)
		content := r.textEditNode(common.NewRange(zero, zero), a.CreateFile.Text)
		docChanges = append(docChanges, r.textDocumentEditNode(a.CreateFile.URI, []ast.Node{content}))
	}
	if len(a.Edits) > 0 {
		ordered := make([]workspace.ActionEdit, len(a.Edits))
		copy(ordered, a.Edits)
		sort.SliceStable(ordered, func(i, j int) bool {
			return ordered[i].Range.Beg().Compare(ordered[j].Range.Beg()) > 0
		})
		edits := make([]ast.Node, 0, len(ordered))
		for _, e := range ordered {
			edits = append(edits, r.textEditNode(e.Range, e.NewText))
		}
		docChanges = append(docChanges, r.textDocumentEditNode(a.URI, edits))
	}
	edit := ast.NewObject([]ast.Pair{
		ast.NewPair("documentChanges", ast.NewArray(docChanges)),
	})
	return ast.NewObject([]ast.Pair{
		ast.NewPair("title", ast.NewString(a.Title)),
		ast.NewPair("kind", ast.NewString(a.Kind)),
		ast.NewPair("edit", edit),
	})
}

func (r jsonGeneratorDriver) textEditNode(ran common.Range, newText string) ast.Node {
	return ast.NewObject([]ast.Pair{
		ast.NewPair("range", jsonPos.fromRange(ran)),
		ast.NewPair("newText", ast.NewString(newText)),
	})
}

func (r jsonGeneratorDriver) textDocumentEditNode(uri string, edits []ast.Node) ast.Node {
	td := ast.NewObject([]ast.Pair{
		ast.NewPair("uri", ast.NewString(uri)),
		ast.NewPair("version", ast.NewNull()),
	})
	return ast.NewObject([]ast.Pair{
		ast.NewPair("textDocument", td),
		ast.NewPair("edits", ast.NewArray(edits)),
	})
}

func (r jsonGeneratorDriver) filterActionsByOnly(actions []workspace.CodeAction, j Json) []workspace.CodeAction {
	context := j.Get("context")
	if !context.Exists() {
		return actions
	}
	only := context.Get("only")
	if !only.Exists() || only.TypeSafe() != ast.V_ARRAY {
		return actions
	}
	onlyArr, err := only.ArrayUseNode()
	if err != nil {
		return actions
	}
	allowed := make([]string, 0, len(onlyArr))
	for i := range onlyArr {
		if s, err := onlyArr[i].String(); err == nil {
			allowed = append(allowed, s)
		}
	}
	if len(allowed) == 0 {
		return actions
	}
	out := actions[:0:0]
	for _, a := range actions {
		for _, o := range allowed {
			if a.Kind == o || strings.HasPrefix(a.Kind, o+".") {
				out = append(out, a)
				break
			}
		}
	}
	return out
}

const (
	severityError = 1
	severityHint  = 4
)

func (r jsonGeneratorDriver) addSyntaxErrors(enc common.Encoding, doc workspace.Doc, j Json) {
	r.appendDiagnostics(j, doc.SyntaxErrors(enc), severityError)
}

func (r jsonGeneratorDriver) addSpaceHints(enc common.Encoding, doc workspace.Doc, j Json) {
	r.appendDiagnostics(j, doc.SpaceHints(enc), severityHint)
}

func (r jsonGeneratorDriver) appendDiagnostics(j Json, diags []workspace.SyntaxError, severity int) {
	key := "diagnostics"
	arr := j.Get(key)
	if !arr.Exists() {
		key = "items"
		arr = j.Get(key)
	}
	if !arr.Exists() {
		return
	}
	if len(diags) == 0 {
		return
	}
	existing, err := arr.ArrayUseNode()
	if err != nil {
		return
	}
	nodes := make([]ast.Node, 0, len(existing)+len(diags))
	nodes = append(nodes, existing...)
	for _, d := range diags {
		nodes = append(nodes, r.newDiagnostic(d, severity))
	}
	j.Set(key, ast.NewArray(nodes))
}

func (r jsonGeneratorDriver) newDiagnostic(e workspace.SyntaxError, severity int) ast.Node {
	return ast.NewObject([]ast.Pair{
		ast.NewPair("range", jsonPos.fromRange(e.Range)),
		ast.NewPair("severity", ast.NewAny(severity)),
		ast.NewPair("source", ast.NewString("gox")),
		ast.NewPair("message", ast.NewString(e.Message)),
	})
}

func (r jsonGeneratorDriver) newHover(ran common.Range, message string) Json {
	kind := ast.NewPair("kind", ast.NewString("plaintext"))
	value := ast.NewPair("value", ast.NewString(message))
	contents := ast.NewPair("contents", ast.NewObject([]ast.Pair{kind, value}))
	rang := ast.NewPair("range", jsonPos.fromRange(ran))
	node := ast.NewObject([]ast.Pair{rang, contents})
	return &node
}

func (r jsonGeneratorDriver) newCompletions(completions []workspace.Completion, complete bool) Json {
	inComplete := ast.NewPair("isIncomplete", ast.NewBool(!complete))
	var itemNodes = make([]ast.Node, 0, len(completions))
	for _, completion := range completions {
		rang := ast.NewPair("range", jsonPos.fromRange(completion.Range))
		label := ast.NewPair("label", ast.NewString(completion.Label))
		newText := ast.NewPair("newText", ast.NewString(completion.Text))
		kind := ast.NewPair("kind", ast.NewAny(completion.Kind()))
		editNode := ast.NewObject([]ast.Pair{rang, newText})
		textEdit := ast.NewPair("textEdit", editNode)
		itemNode := ast.NewObject([]ast.Pair{label, textEdit, kind})
		itemNodes = append(itemNodes, itemNode)
	}
	insertTextFormat := ast.NewPair("insertTextFormat", ast.NewAny(2))
	insertTextMode := ast.NewPair("insertTextMode", ast.NewAny(2))
	itemDefaults := ast.NewPair("itemDefaults", ast.NewObject([]ast.Pair{insertTextFormat, insertTextMode}))
	items := ast.NewPair("items", ast.NewArray(itemNodes))
	node := ast.NewObject([]ast.Pair{inComplete, itemDefaults, items})
	return &node
}
