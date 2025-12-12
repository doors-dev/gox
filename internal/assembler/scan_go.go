package assembler

import (
	"github.com/doors-dev/gox/internal/common"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func scanGoSource(coll collector, root *tree_sitter.Node) {
	scanner := goScanner{
		root: root,
		coll: coll,
		/*
			importGox:     importGox,
		*/
	}
	scanner.scan()
}

func scanGoSnippet(coll collector, root *tree_sitter.Node) {
	scanner := goScanner{
		root: root,
		coll: coll,
	}
	scanner.scan()
}

func scanGoSnippetWithSiblings(coll collector, start *tree_sitter.Node, until tree_sitter.Point) {
	scanner := goScanner{
		root:          start,
		coll:          coll,
		siblingsUntil: common.NewTSPos(until),
		scanSiblings:  true,
	}
	scanner.scan()
}

type goScanner struct {
	root          *tree_sitter.Node
	scanSiblings  bool
	beg           tree_sitter.Point
	cursor        *tree_sitter.TreeCursor
	importGox     bool
	importContext bool
	coll          collector
	siblingsUntil common.Pos
}

func (g *goScanner) portal(node *tree_sitter.Node) {
	g.coll.portal(g.beg, node.StartPosition())
	g.beg = node.EndPosition()
}

func (g *goScanner) process() bool {
	node := g.cursor.Node()
	kind := node.Kind()
	switch kind {
	case PACKAGE:
		if !g.importGox {
			return false
		}
		g.coll.portal(g.beg, node.EndPosition())
		g.beg = node.EndPosition()
		if g.importGox {
			g.importGox = false
			g.coll.cr()
			g.coll.append(r(`import "github.com/doors-dev/gox"`))
		}
		return true
	case GOX_ELEMENT:
		g.portal(node)
		scanElement(g.coll, node, true)
	case GOX_ELEM_FUNC_DEC:
		g.portal(node)
		scanElemDec(g.coll, node)
	case GOX_ELEM_METH_DEC:
		g.portal(node)
		scanElemDec(g.coll, node)
	case GOX_ELEM_FUNC_TYPE:
		g.portal(node)
		scanElemType(g.coll, node)
	case GOX_ELEM_FUNC_LIT:
		g.portal(node)
		scanElemLit(g.coll, node)
	default:
		return false
	}
	return true
}

func (g *goScanner) scanNode() {
	if g.process() {
		return
	}
	if !g.cursor.GotoFirstChild() {
		return
	}
	g.scanNode()
	for g.cursor.GotoNextSibling() {
		g.scanNode()
	}
	g.cursor.GotoParent()
}

func (g *goScanner) scan() {
	g.beg = g.root.StartPosition()
	for {
		g.cursor = g.root.Walk()
		g.scanNode()
		g.cursor.Close()
		if !g.scanSiblings {
			break
		}
		next := g.root.NextNamedSibling()
		if next == nil {
			break
		}
		newEndPos := next.EndPosition()
		if g.siblingsUntil.Compare(common.NewTSPos(newEndPos)) == -1 {
			break
		}
		g.root = next
	}
	g.coll.portal(g.beg, g.root.EndPosition())
}
