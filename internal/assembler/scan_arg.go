package assembler

import tree_sitter "github.com/tree-sitter/go-tree-sitter"

func scanExpression(coll collector, value *tree_sitter.Node) {
	coll.append(r("func() any "))
	if value.ChildByFieldName("body") == nil {
		coll.append(r("{ return nil }"))
	} else {
		scanGoSnippetWithSiblings(coll, value.Child(0), value.EndPosition(), true)
	}
	coll.append(r("()"))
}
