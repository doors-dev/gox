package workspace

import (
	"log/slog"

	"github.com/doors-dev/gox/internal/assembler"
	"github.com/doors-dev/gox/internal/catalog/grammer"
	"github.com/doors-dev/gox/internal/catalog/symbol"
	"github.com/doors-dev/gox/internal/common"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type Symbol struct {
	Kind    symbol.Kind
	Name    string
	Range   common.Range
	Symbols []Symbol
}

func (d Doc) Symbols(enc common.Encoding) []Symbol {
	root := d.tree.RootNode()
	cursor := root.Walk()
	defer cursor.Close()
	ss := make([]Symbol, 0)
	for _, n := range root.Children(cursor) {
		switch n.Kind() {
		case grammer.TYPE_DECLARATION:
			spec := n.Child(1)
			if spec == nil {
				slog.Info("type declaration not found")
				continue
			}
			slog.Info("type declaration found", "kind", spec.Kind())
			nameNode := spec.ChildByFieldName("name")
			if nameNode == nil {
				continue
			}
			typ := spec.ChildByFieldName("type")
			if typ == nil {
				continue
			}
			name := nameNode.Utf8Text(d.source.Source())
			ran := d.source.FromRange(enc, common.NewTSRange(nameNode.Range()))
			s := Symbol{
				Name:  name,
				Range: ran,
			}
			switch typ.Kind() {
			case grammer.STRUCT_TYPE:
				s.Kind = symbol.Struct
				assembler.QueryFields.IterateCapture(d.source.Source(), spec, "name", func(node *tree_sitter.Node) bool {
					name := node.Utf8Text(d.source.Source())
					ran := d.source.FromRange(enc, common.NewTSRange(node.Range()))
					s.Symbols = append(s.Symbols, Symbol{
						Kind:  symbol.Field,
						Name:  name,
						Range: ran,
					})
					return false
				})
			case grammer.INTERFACE_TYPE:
				s.Kind = symbol.Interface
				assembler.QueryInterfaceMethods.IterateCapture(d.source.Source(), spec, "name", func(node *tree_sitter.Node) bool {
					name := node.Utf8Text(d.source.Source())
					ran := d.source.FromRange(enc, common.NewTSRange(node.Range()))
					s.Symbols = append(s.Symbols, Symbol{
						Kind:  symbol.Method,
						Name:  name,
						Range: ran,
					})
					return false
				})
			default:
				s.Kind = symbol.Class
			}
			ss = append(ss, s)
		case grammer.CONST_DECLARATION,
			grammer.VAR_DECLARATION,
			grammer.FUNC_DECLARATION,
			grammer.METHOD_DECLARATION,
			grammer.GOX_ELEM_FUNC_DEC,
			grammer.GOX_ELEM_METH_DEC:
			nameNode := n.ChildByFieldName("name")
			if nameNode == nil {
				continue
			}
			name := nameNode.Utf8Text(d.source.Source())
			ran := d.source.FromRange(enc, common.NewTSRange(nameNode.Range()))
			s := Symbol{
				Name:  name,
				Range: ran,
			}
			switch n.Kind() {
			case grammer.CONST_DECLARATION:
				s.Kind = symbol.Constant
			case grammer.VAR_DECLARATION:
				s.Kind = symbol.Variable
			case grammer.FUNC_DECLARATION:
				s.Kind = symbol.Function
			case grammer.METHOD_DECLARATION:
				s.Kind = symbol.Method
			case grammer.GOX_ELEM_FUNC_DEC:
				s.Kind = symbol.Function
			case grammer.GOX_ELEM_METH_DEC:
				s.Kind = symbol.Method
			}
			ss = append(ss, s)
		}
	}
	return ss
}
