package assembler

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func scanComment(coll collector, root *tree_sitter.Node) {
	comment := root.ChildByFieldName("comment")
	coll.cr()
	coll.append(p(comment))
}

func scanTilde(coll collector, root *tree_sitter.Node) {
	body := root.ChildByFieldName("body")
	kind := body.Kind()
	coll.cr()
	switch kind {
	case GOX_TILDE_IF:
		scanIf(coll, body)
	case GOX_TILDE_FOR:
		scanFor(coll, body)
	case GOX_TILDE_VALUE, GOX_FUNC:
		coll.append(r("__e = __c.WriteAny(ctx, "))
		scanValue(coll, body)
		coll.append(r("); " + ERR_CHECK))
	case GOX_TILDE_LITERAL_VALUE:
		coll.append(r("__e = __c.WriteText(ctx, "), s(body), r("); "+ERR_CHECK))
	case GOX_TIDE_BLOCK:
		body = body.ChildByFieldName("body")
		if body != nil {
			scanGoSnippet(coll, body)
		}
	}
}

func scanFor(coll collector, root *tree_sitter.Node) {
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

func scanIf(coll collector, root *tree_sitter.Node) {
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
	if alternative.Kind() == GOX_TILDE_IF {
		scanIf(coll, alternative)
		return
	}
	coll.append(r(" {"))
	coll.indentBeg()
	scanContent(coll, alternative)
	coll.indentEnd()
	coll.cr()
	coll.append(r("}"))

}
