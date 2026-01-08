package assembler

import (
	"github.com/doors-dev/gox/internal/catalog/grammer"
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
	case grammer.GOX_TILDE_IF:
		scanIf(coll, body)
	case grammer.GOX_TILDE_FOR:
		scanFor(coll, body)
	case grammer.GOX_FUNC:
		coll.append(r("__e = __c.Any(ctx, "))
		scanFunc(coll, body, false)
		coll.append(r("); " + ERR_CHECK))
	case grammer.GOX_TILDE_JOB:
		arg := body.ChildByFieldName("arg")
		if arg == nil {
			return
		}
		switch arg.Kind() {
		case grammer.GOX_SINGLE_ARG:
			coll.append(r("__e = __c.Any(ctx, "))
		case grammer.GOX_MULTI_ARG:
			coll.append(r("__e = __c.Many(ctx, "))
		default:
			return
		}
		scanGoSnippet(coll, arg)
		coll.append(r("); " + ERR_CHECK))
	case grammer.GOX_TILDE_LITERAL_VALUE:
		coll.append(r("__e = __c.Text("), s(body), r("); "+ERR_CHECK))
	case grammer.GOX_TILDE_PROXY:
		arg := body.ChildByFieldName("arg")
		if arg == nil {
			return
		}
		coll.append(r("__c.Proxy("))
		scanValue(coll, arg, false)
		coll.append(r(")"))
	case grammer.GOX_TIDE_BLOCK:
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
	if alternative.Kind() == grammer.GOX_TILDE_IF {
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
