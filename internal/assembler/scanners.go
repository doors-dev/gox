package assembler
/*
import (
	"strings"

	"github.com/doors-dev/gox/internal/doc2/reader"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type Assembler interface {
	Source() []byte
	Frame(origin *tree_sitter.Node, open string, close string) Assembler
	Open(origin *tree_sitter.Node, parts ...any)
	Close(parts ...any)
}

func code(r reader.Reader, a Assembler) {
	for {
		node := r.Next()
		if node == nil {
			break
		}
		name := node.GrammarName()
		println(name)
		switch name {
		case GOX_ELEMENT:
			r.Skip()
			//		gox(r.CurrentReader(), a)
		case GOX_FUNC_DEC:
		case GOX_METH_DEC:
		case GOX_FUNC_TYPE:
		case GOX_FUNC_LIT:
		}
	}
}

func codeElement(element *tree_sitter.Node, a Assembler) {
	a = a.Frame(element,
		"gox.Elem(func(ctx context.Context, __C gox.Cursor) (__E error) {",
		"return })",
	)
	content(element, a)
}

func content(head *tree_sitter.Node, a Assembler) {
	cursor := head.Walk()
	children := head.ChildrenByFieldName("content", cursor)
	cursor.Close()
	for _, child := range children {
		name := child.GrammarName()
		switch name {
		case GOX_ELEMENT:
		case GOX_HEAD:
		case GOX_SCRIPT_HEAD:
		case GOX_STYLE_HEAD:
		case GOX_VOID_HEAD:
		case GOX_SELF_CLOSING_HEAD:
		case GOX_ERRONEOUS_CLOSE_HEAD:
		case GOX_DOCTYPE:
		case GOX_TILDE:
		case GOX_TILDE_PROXY:
		case GOX_TILDE_COMMENT:
		case GOX_COMMENT:
		case GOX_PLAIN_TEXT:
		case GOX_RAW_TEXT:
		}
	}
}

func element(node *tree_sitter.Node, a Assembler) {
	a = a.Frame(node,
		"__E = __C.WriteElem(ctx, gox.Elem(func(ctx context.Context, __C gox.Cursor) (__E error) {",
		"return })); if __E != nil { return };",
	)
	content(node, a)
}

func anyHead(node *tree_sitter.Node, a Assembler) {
	open := node.ChildByFieldName("open")
	name := open.ChildByFieldName("name")
	a.Open(node, "__E = __C.HeadInit(ctx, ", s(name), "); if __E != nil { return };")
	defer a.Close("__E = __C.HeadClose(ctx); if __E != nil { return };")
	attributes(open, a)
	content(node, a)
}

func attributes(openHead *tree_sitter.Node, a Assembler) {
	cursor := openHead.Walk()
	children := openHead.ChildrenByFieldName("attrs", cursor)
	cursor.Close()
	for _, child := range children {
		name := child.GrammarName()
		switch name {
		case GOX_ATTR:
			attribute(&child, a)
		case GOX_ATTR_MOD:
		case COMMENT:
		}
	}
}

func attribute(attr *tree_sitter.Node, a Assembler) {
	name := attr.ChildByFieldName("name")
	attrName := name.Utf8Text(a.Source())
	if strings.EqualFold(attrName, )
} */
