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
	for _, child := range children {
		name := child.Kind()
		switch name {
		case grammer.GOX_ELEMENT:
			scanElement(coll, &child, false)
		case grammer.GOX_HEAD:
			scanHead(coll, &child)
		case grammer.GOX_RAW_HEAD:
			scanRawHead(coll, &child)
		case grammer.GOX_SCRIPT_HEAD:
			scanHead(coll, &child)
		case grammer.GOX_STYLE_HEAD:
			scanHead(coll, &child)
		case grammer.GOX_VOID_HEAD:
			scanVoidHead(coll, &child)
		case grammer.GOX_SELF_CLOSING_HEAD:
			scanSelfClosingHead(coll, &child)
		case grammer.GOX_ERRONEOUS_CLOSE_HEAD:
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
		case grammer.GOX_TILDE_COMMENT:
			scanComment(coll, &child) 
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
	coll.append(r("__e = __c.WriteText(ctx, "), t(root), r("); "+ERR_CHECK))
}

func scanRaw(coll collector, root *tree_sitter.Node) {
	coll.cr()
	coll.append(r("__e = __c.WriteRaw(ctx, "), s(root), r("); "+ERR_CHECK))
}

func scanVoidHead(coll collector, root *tree_sitter.Node) {
	coll.cr()
	name := root.ChildByFieldName("name")
	coll.append(r("__e = __c.HeadInitVoid(ctx, "), s(name), r("); "+ERR_CHECK))
	coll.indentFake()
	scanAttributes(coll, root)
	coll.indentEnd()
	coll.cr()
	coll.append(r("__e = __c.HeadSubmit(ctx); " + ERR_CHECK))
}

func scanSelfClosingHead(coll collector, root *tree_sitter.Node) {
	name := root.ChildByFieldName("name")
	if name == nil {
		return
	}
	coll.cr()
	coll.append(r("__e = __c.HeadInit(ctx, "), s(name), r("); "+ERR_CHECK))
	coll.indentFake()
	scanAttributes(coll, root)
	coll.cr()
	coll.append(r("__e = __c.HeadSubmit(ctx); " + ERR_CHECK))
	coll.indentEnd()
	coll.cr()
	coll.append(r("__e = __c.HeadClose(ctx); " + ERR_CHECK))
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
	coll.append(r("__e = __c.HeadInit(ctx, "), s(name), r("); "+ERR_CHECK))
	coll.indentFake()
	scanAttributes(coll, open)
	coll.cr()
	coll.append(r("__e = __c.HeadSubmit(ctx); " + ERR_CHECK))
	scanContent(coll, root)
	coll.indentEnd()
	coll.cr()
	coll.append(r("__e = __c.HeadClose(ctx); " + ERR_CHECK))
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
			coll.cr()
			coll.append(r("__e = __c.AttrMod"))
			scanGoSnippet(coll, &child)
			coll.cr()
			coll.append(r("; "), r(ERR_CHECK))
		}
	}
}

func scanValue(coll collector, root *tree_sitter.Node, string bool) {
	kind := root.Kind()
	if kind == grammer.GOX_FUNC {
		body := root.ChildByFieldName("body")
		if string {
			coll.append(r("func() string "))
		} else {
			coll.append(r("func() any "))
		}
		scanGoSnippet(coll, body)
		coll.append(r("()"))
		return
	}
	scanGoSnippet(coll, root)
}
