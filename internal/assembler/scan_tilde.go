package assembler

import (
	"github.com/doors-dev/gox/internal/catalog/grammer"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func scanTildeComment(coll collector, root *tree_sitter.Node) {
	comment := root.ChildByFieldName("comment")
	coll.cr()
	coll.append(p(comment))
}

func scanTildeBlock(coll collector, root *tree_sitter.Node) {
	root = root.ChildByFieldName("body")
	if root == nil {
		return
	}
	coll.cr()
	scanGoSnippet(coll, root)
}

func scanTilde(coll collector, root *tree_sitter.Node) {
	value := root.ChildByFieldName("value")
	kind := value.Kind()
	coll.cr()
	switch kind {
	case grammer.GOX_TILDE_IF:
		scanTildeIf(coll, value)
	case grammer.GOX_TILDE_FOR:
		scanTildeFor(coll, value)
	case grammer.GOX_SINGLE_ARG:
		coll.append(r("__e = __c.Any("))
		scanValue(coll, value)
		coll.append(r("); " + ERR_CHECK))
	case grammer.GOX_MULTI_ARG:
		coll.append(r("__e = __c.Many("))
		scanValue(coll, value)
		coll.append(r("); " + ERR_CHECK))
	}
}

func scanTildeFor(coll collector, root *tree_sitter.Node) {
	setup := root.ChildByFieldName("setup")
	body := root.ChildByFieldName("body")
	if body != nil {
		scanGoSnippetWithSiblings(coll, setup, body.StartPosition())
	} else {
		scanGoSnippet(coll, setup)
	}
	coll.append(r(" {"))
	coll.indentBeg()
	scanContent(coll, body)
	coll.indentEnd()
	coll.cr()
	coll.append(r("}"))
}

func scanTildeIf(coll collector, root *tree_sitter.Node) {
	setup := root.ChildByFieldName("setup")
	cons := root.ChildByFieldName("consequence")
	if cons != nil {
		scanGoSnippetWithSiblings(coll, setup, cons.StartPosition())
	} else {
		scanGoSnippet(coll, setup)
	}
	coll.append(r(" {"))
	coll.indentBeg()
	scanContent(coll, cons)
	coll.indentEnd()
	coll.cr()
	coll.append(r("}"))
	alternative := root.ChildByFieldName("alternative")
	if alternative == nil {
		return
	}
	coll.append(r(" else "))
	if alternative.Kind() == grammer.GOX_TILDE_IF {
		scanTildeIf(coll, alternative)
		return
	}
	coll.append(r(" {"))
	coll.indentBeg()
	scanContent(coll, alternative)
	coll.indentEnd()
	coll.cr()
	coll.append(r("}"))

}
