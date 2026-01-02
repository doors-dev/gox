package lsp

import (
	"log/slog"
	"math"

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

func (r jsonGeneratorDriver) newUpdateEdit(uri string, content string, message string) Json {
	messageNode := ast.NewPair("label", ast.NewString(message))
	rang := ast.NewPair("range", jsonPos.fromRange(common.NewRange(
		common.NewPos(0, 0),
		common.NewPos(int(math.MaxInt32), 0),
	)))
	text := ast.NewPair("newText", ast.NewString(content))
	textEdit := ast.NewObject([]ast.Pair{rang, text})
	docEdits := ast.NewPair(uri, ast.NewArray([]ast.Node{textEdit}))
	changes := ast.NewPair("changes", ast.NewObject([]ast.Pair{docEdits}))
	edit := ast.NewPair("edit", ast.NewObject([]ast.Pair{changes}))
	node := ast.NewObject([]ast.Pair{messageNode, edit})
	d, _ := node.MarshalJSON()
	slog.Info("new update edit", "node", string(d))
	return &node
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
