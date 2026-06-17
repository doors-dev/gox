package assembler

import (
	"github.com/doors-dev/gox/internal/catalog/grammer"
	"github.com/doors-dev/gox/internal/common"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func scanGoSource(coll collector, root *tree_sitter.Node) {
	scanner := goScanner{
		root: root,
		coll: coll,
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

func scanGoSnippetWithSiblings(coll collector, start *tree_sitter.Node, until tree_sitter.Point, include bool) {
	scanner := goScanner{
		root: start,
		coll: coll,
		scanSiblings: &scanSiblings{
			until:   common.NewTSPos(until),
			include: include,
		},
	}
	scanner.scan()
}

type scanSiblings struct {
	until   common.Pos
	include bool
}

func (s *scanSiblings) cont(end tree_sitter.Point) bool {
	pos := common.NewTSPos(end)
	compare := s.until.Compare(pos)
	if compare == 1 {
		return true
	}
	if compare == 0 {
		return s.include
	}
	return false
}

type goScanner struct {
	root         *tree_sitter.Node
	beg          tree_sitter.Point
	cursor       *tree_sitter.TreeCursor
	coll         collector
	scanSiblings *scanSiblings
}

func (g *goScanner) portal(node *tree_sitter.Node) {
	g.coll.portal(g.beg, node.StartPosition())
	g.beg = node.EndPosition()
}

func (g *goScanner) process() bool {
	node := g.cursor.Node()
	kind := node.Kind()
	switch kind {
	case grammer.GOX_ELEMENT:
		g.portal(node)
		scanElement(g.coll, node)
	case grammer.GOX_EXPRESSION:
		g.portal(node)
		scanExpression(g.coll, node)
	case grammer.GOX_ELEM_FUNC_DEC:
		g.portal(node)
		scanElemDec(g.coll, node)
	case grammer.GOX_ELEM_METH_DEC:
		g.portal(node)
		scanElemDec(g.coll, node)
	case grammer.GOX_ELEM_FUNC_TYPE:
		g.portal(node)
		scanElemType(g.coll, node)
	case grammer.GOX_ELEM_FUNC_LIT:
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
		if g.scanSiblings == nil {
			break
		}
		next := g.root.NextSibling()
		if next == nil {
			break
		}
		if !g.scanSiblings.cont(next.EndPosition()) {
			break
		}
		g.root = next
	}
	g.coll.portal(g.beg, g.root.EndPosition())
}
