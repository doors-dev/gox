package assembler

import tree_sitter "github.com/tree-sitter/go-tree-sitter"

func scanElement(coll collector, root *tree_sitter.Node, inCode bool) {
	if inCode {
		coll.append(
			r("gox.Elem(func(ctx gox.Context, __c gox.Cursor) (__e error) {"),
		)
		coll.indentRef(root.StartPosition())
	} else {
		coll.cr()
		coll.append(
			r("__e = __c.WriteElem(gox.Elem(func(ctx gox.Context, __c gox.Cursor) (__e error) {"),
		)
	}
	coll.indentBeg()
	coll.append(r("__c.Noop(ctx)"))
	coll.cr()
	scanContent(coll, root)
	coll.indentEnd()
	if inCode {
		coll.cr()
		coll.append(
			r("return })"),
		)
		coll.indentEnd()
	} else {
		coll.cr()
		coll.append(
			r("return })); " + ERR_CHECK),
		)
	}
}

func scanElemDec(coll collector, root *tree_sitter.Node) {
	coll.indentRef(root.StartPosition())
	coll.append(r("func "))
	name := root.ChildByFieldName("name")
	if name == nil {
		return
	}
	receiver := root.ChildByFieldName("receiver")
	body := root.ChildByFieldName("body")
	start := name.StartPosition()
	if receiver != nil {
		start = receiver.StartPosition()
	}
	params := root.ChildByFieldName("parameters")
	if params == nil {
		return
	}
	end := params.EndPosition()
	coll.portal(start, end)
	coll.append(r(" gox.Elem"))
	if body == nil {
		return
	}
	coll.append(r(" {"))
	coll.indentBeg()
	coll.cr()
	coll.append(r("return "), r("gox.Elem(func(ctx gox.Context, __c gox.Cursor) (__e error) {"))
	coll.indentBeg()
	coll.cr()
	coll.append(r("__c.Noop(ctx)"))
	scanContent(coll, body)
	coll.indentEnd()
	coll.cr()
	coll.append(
		r("return })"),
	)
	coll.indentEnd()
	coll.cr()
	coll.append(r("}"))
	coll.indentEnd()
}

func scanElemLit(coll collector, root *tree_sitter.Node) {
	params := root.ChildByFieldName("parameters")
	body := root.ChildByFieldName("body")
	coll.indentRef(root.StartPosition())
	coll.append(r("func"), p(params), r(" gox.Elem {"))
	coll.indentBeg()
	coll.cr()
	coll.append(r("return "), r("gox.Elem(func(ctx gox.Context, __c gox.Cursor) (__e error) {"))
	coll.indentBeg()
	coll.cr()
	coll.append(r("__c.Noop(ctx)"))
	scanContent(coll, body)
	coll.indentEnd()
	coll.cr()
	coll.append(
		r("return })"),
	)
	coll.indentEnd()
	coll.cr()
	coll.append(r("}"))
	coll.indentEnd()
}

func scanElemType(coll collector, root *tree_sitter.Node) {
	params := root.ChildByFieldName("parameters")
	coll.append(r("func"), p(params), r(" gox.Elem"))
}
