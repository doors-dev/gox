package assembler

import (
	"github.com/doors-dev/gox/internal/catalog/grammer"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

const ERR_CHECK = "if __e != nil { return }"

func scanContent(coll collector, root *tree_sitter.Node) {
	if root == nil {
		return
	}
	cursor := root.Walk()
	children := root.ChildrenByFieldName("content", cursor)
	cursor.Close()
	proxies := make([]*tree_sitter.Node, 0)
	for _, child := range children {
		name := child.Kind()
		if name == grammer.GOX_SPACE_FILLER || name == grammer.GOX_ERRONEOUS_CLOSE_HEAD {
			continue
		}
		if name == grammer.GOX_TILDE_PROXY {
			body := child.ChildByFieldName("body")
			if body == nil {
				continue
			}
			arg := body.ChildByFieldName("arg")
			if arg == nil {
				continue
			}
			proxies = append(proxies, arg)
			continue
		}
		proxied := len(proxies) != 0
		if proxied {
			for _, proxy := range proxies {
				coll.cr()
				coll.append(r("__c.AddProxy("))
				scanValue(coll, proxy, false)
				coll.append(r(")"))
			}
			proxies = proxies[:0]
			coll.cr()
			coll.append(
				r("__e = __c.ProxyElem(gox.Elem(func(__c gox.Cursor) (__e error) {"),
			)
			coll.indentBeg()
			coll.cr()
			coll.append(r("ctx := __c.Context(); __c.Noop(ctx)"))
		}
		nonContainer := false
		switch name {
		case grammer.GOX_CONTAINER_HEAD:
			scanContainerHead(coll, &child)
		case grammer.GOX_HEAD:
			scanHead(coll, &child)
		case grammer.GOX_SCRIPT_HEAD:
			scanHead(coll, &child)
		case grammer.GOX_STYLE_HEAD:
			scanHead(coll, &child)
		case grammer.GOX_SELF_CLOSING_HEAD:
			scanSelfClosingHead(coll, &child)
		default:
			nonContainer = true
		}
		if nonContainer {
			if proxied {
				coll.cr()
				coll.append(r("__e = __c.InitContainer(); " + ERR_CHECK))
				coll.indentFake()
			}
			switch name {
			case grammer.GOX_RAW_HEAD:
				scanRawHead(coll, &child)
			case grammer.GOX_VOID_HEAD:
				scanVoidHead(coll, &child)
			case grammer.GOX_DOCTYPE:
				scanRaw(coll, &child)
			case grammer.GOX_COMMENT:
				scanRaw(coll, &child)
			case grammer.GOX_PLAIN_TEXT:
				scanPlain(coll, &child)
			case grammer.GOX_RAW_TEXT:
				scanRaw(coll, &child)
			case grammer.GOX_TILDE:
				scanTilde(coll, &child)
			}
			if proxied {
				coll.indentEnd()
				coll.cr()
				coll.append(r("__e = __c.Close(); " + ERR_CHECK))
			}
		}
		if proxied {
			coll.indentEnd()
			coll.cr()
			coll.append(
				r("return })); " + ERR_CHECK),
			)
		}
	}
}

func scanRawHead(coll collector, root *tree_sitter.Node) {
	content := root.ChildByFieldName("content")
	if content == nil {
		return
	}
	scanRaw(coll, content)
}

func scanPlain(coll collector, root *tree_sitter.Node) {
	coll.cr()
	coll.append(r("__e = __c.Text("), t(root), r("); "+ERR_CHECK))
}

func scanRaw(coll collector, root *tree_sitter.Node) {
	coll.cr()
	coll.append(r("__e = __c.Raw("), s(root), r("); "+ERR_CHECK))
}

func scanVoidHead(coll collector, root *tree_sitter.Node) {
	coll.cr()
	name := root.ChildByFieldName("name")
	coll.append(r("__e = __c.InitVoid("), s(name), r("); "+ERR_CHECK))
	coll.indentFake()
	scanAttributes(coll, root)
	coll.indentEnd()
	coll.cr()
	coll.append(r("__e = __c.Submit(); " + ERR_CHECK))
}

func scanSelfClosingHead(coll collector, root *tree_sitter.Node) {
	name := root.ChildByFieldName("name")
	if name == nil {
		return
	}
	coll.cr()
	coll.append(r("__e = __c.Init("), s(name), r("); "+ERR_CHECK))
	coll.indentFake()
	scanAttributes(coll, root)
	coll.cr()
	coll.append(r("__e = __c.Submit(); " + ERR_CHECK))
	coll.indentEnd()
	coll.cr()
	coll.append(r("__e = __c.Close(); " + ERR_CHECK))
}

func scanHead(coll collector, root *tree_sitter.Node) {
	open := root.ChildByFieldName("open")
	if open == nil {
		return
	}
	name := open.ChildByFieldName("name")
	if name == nil {
		return
	}
	coll.cr()
	coll.append(r("__e = __c.Init("), s(name), r("); "+ERR_CHECK))
	coll.indentFake()
	scanAttributes(coll, open)
	coll.cr()
	coll.append(r("__e = __c.Submit(); " + ERR_CHECK))
	scanContent(coll, root)
	coll.indentEnd()
	coll.cr()
	coll.append(r("__e = __c.Close(); " + ERR_CHECK))
}

func scanContainerHead(coll collector, root *tree_sitter.Node) {
	coll.cr()
	coll.append(r("__e = __c.InitContainer(); " + ERR_CHECK))
	coll.indentFake()
	scanContent(coll, root)
	coll.indentEnd()
	coll.cr()
	coll.append(r("__e = __c.Close(); " + ERR_CHECK))
}

func scanAttributes(coll collector, root *tree_sitter.Node) {
	cursor := root.Walk()
	children := root.ChildrenByFieldName("attrs", cursor)
	cursor.Close()
	for _, child := range children {
		kind := child.Kind()
		name := child.ChildByFieldName("name")
		value := child.ChildByFieldName("value")
		switch kind {
		case grammer.COMMENT:
			coll.cr()
			coll.append(p(&child))
		case grammer.GOX_ATTR:
			coll.cr()
			coll.append(r("__e = __c.AttrSetAny("), s(name), r(", "))
			scanValue(coll, value, false)
			coll.append(r("); "), r(ERR_CHECK))
		case grammer.GOX_LITERAL_ATTR:
			coll.cr()
			coll.append(r("__e = __c.AttrSet("), s(name), r(", "), s(value), r("); "), r(ERR_CHECK))
		case grammer.GOX_CLASS_ATTR:
			coll.cr()
			coll.append(r("__e = __c.AttrAppend(\"class\", "))
			scanValue(coll, value, true)
			coll.append(r("); "), r(ERR_CHECK))
		case grammer.GOX_CLASS_LITERAL_ATTR:
			coll.cr()
			coll.append(r("__e = __c.AttrAppend(\"class\", "), s(value), r("); "), r(ERR_CHECK))
		case grammer.GOX_BOOL_ATTR:
			coll.cr()
			if value == nil {
				coll.append(r("__e = __c.AttrSetBool("), s(name), r(", true);"), r(ERR_CHECK))
			} else {
				coll.append(r("__e = __c.AttrSetBool("), s(name), r(", "), p(value), r("); "), r(ERR_CHECK))
			}
		case grammer.GOX_ATTR_MOD:
			arg := child.ChildByFieldName("arg")
			if arg == nil {
				continue
			}
			coll.cr()
			coll.append(r("__e = __c.AttrMod("))
			scanGoSnippet(coll, arg)
			coll.append(r("); "), r(ERR_CHECK))
		}
	}
}

func scanFunc(coll collector, root *tree_sitter.Node, string bool) {
	body := root.ChildByFieldName("body")
	if string {
		coll.append(r("func() string "))
	} else {
		coll.append(r("func() any "))
	}
	scanGoSnippet(coll, body)
	coll.append(r("()"))
}

func scanValue(coll collector, root *tree_sitter.Node, string bool) {
	kind := root.Kind()
	if kind == grammer.GOX_FUNC {
		scanFunc(coll, root, string)
		return
	}
	scanGoSnippet(coll, root)
}
