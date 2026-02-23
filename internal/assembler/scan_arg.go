package assembler

import (
	"github.com/doors-dev/gox/internal/catalog/grammer"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func scanValue(coll collector, value *tree_sitter.Node) {
	switch value.Kind() {
	case grammer.GOX_FUNC:
		body := value.ChildByFieldName("body")
		coll.append(r("func() any "))
		scanGoSnippet(coll, body)
		coll.append(r("()"))
	case grammer.GOX_SINGLE_ARG:
		child := value.Child(0)
		if child != nil && child.Kind() == grammer.GOX_FUNC {
			scanValue(coll, child)
			return
		}
		scanGoSnippet(coll, value)
	case grammer.GOX_MULTI_ARG:
		scanGoSnippet(coll, value)
	default:
		scanGoSnippet(coll, value)
	}
}
