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
	proxyLevel := 0
	for _, child := range children {
		name := child.Kind()
		switch name {
		case grammer.GOX_SPACE_FILLER, grammer.GOX_ERRONEOUS_CLOSE_HEAD, grammer.GOX_TILDE_COMMENT:
			continue
		case grammer.GOX_TILDE_PROXY:
			arg := child.ChildByFieldName("value")
			if arg == nil {
				continue
			}
			if arg.Kind() == grammer.GOX_SINGLE_ARG {
				proxyLevel++
				renderProxyBeg(coll, arg)
			}
			if arg.Kind() == grammer.GOX_MULTI_ARG {
				for i := range arg.ChildCount() {
					proxy := arg.Child(i)
					if !proxy.IsNamed() {
						continue
					}
					proxyLevel++
					renderProxyBeg(coll, proxy)
				}
			}
			continue
		case grammer.GOX_CONTAINER_HEAD:
			scanContainerHead(coll, &child)
		case grammer.GOX_VOID_HEAD:
			scanVoidHead(coll, &child)
		case grammer.GOX_HEAD:
			scanHead(coll, &child)
		case grammer.GOX_SCRIPT_HEAD:
			scanHead(coll, &child)
		case grammer.GOX_STYLE_HEAD:
			scanHead(coll, &child)
		case grammer.GOX_SELF_CLOSING_HEAD:
			scanSelfClosingHead(coll, &child)
		case grammer.GOX_RAW_HEAD:
			scanRawHead(coll, &child)
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
		case grammer.GOX_TILDE_BLOCK:
			scanTildeBlock(coll, &child)
		}
		for range proxyLevel {
			coll.indentEnd()
			coll.cr()
			coll.append(
				r("return })); " + ERR_CHECK),
			)
		}
		proxyLevel = 0
	}
	for range proxyLevel {
		coll.indentEnd()
		coll.cr()
		coll.append(
			r("return })); " + ERR_CHECK),
		)
	}
}

func renderProxyBeg(coll collector, proxy *tree_sitter.Node) {
	coll.cr()
	coll.append(r("__e = "))
	scanValue(coll, proxy)
	coll.append(r(".Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {"))
	coll.indentBeg()
	coll.cr()
	coll.append(r("ctx := __c.Context(); _ = ctx"))
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
			coll.append(r("__e = __c.AttrSet("), s(name), r(", "))
			if value != nil {
				scanValue(coll, value)
			} else {
				coll.append(r("true"))
			}
			coll.append(r("); "), r(ERR_CHECK))
		case grammer.GOX_ATTR_MOD:
			arg := child.ChildByFieldName("value")
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
