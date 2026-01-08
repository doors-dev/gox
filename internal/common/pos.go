package common

import (
	"fmt"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func NoPos() Pos {
	return NewPos(-1, -1)
}

func NewPos(line, column int) Pos {
	return Pos{line, column}
}

func NewTSPos(point tree_sitter.Point) Pos {
	return NewPos(int(point.Row), int(point.Column))
}

type Pos [2]int

func (p Pos) String() string {
	return fmt.Sprintf("%d:%d", p.Line(), p.Column())
}

func (p Pos) TS() tree_sitter.Point {
	return tree_sitter.Point{Row: uint(p.Line()), Column: uint(p.Column())}
}

func (c Pos) Compare(p Pos) int {
	switch true {
	case c[0] < p[0]:
		return -1
	case c[0] > p[0]:
		return 1
	case c[0] == p[0] && c[1] < p[1]:
		return -1
	case c[0] == p[0] && c[1] > p[1]:
		return 1
	default:
		return 0
	}
}

func (p Pos) Advance() Pos {
	return [2]int{p.Line(), p.Column() + 1}
}

func (p Pos) IsValid() bool {
	return p[0] >= 0 && p[1] >= 0
}

func (p Pos) Line() int {
	return p[0]
}

func (p Pos) Column() int {
	return p[1]
}

func NoRange() Range {
	return NewRange(NoPos(), NoPos())
}

func NewRange(beg Pos, end Pos) Range {
	return Range{beg, end}
}

func NewTSRange(ran tree_sitter.Range) Range {
	return NewRange(NewTSPos(ran.StartPoint), NewTSPos(ran.EndPoint))
}

type Range [2]Pos

func (r Range) IsValid() bool {
	return r[0].IsValid() && r[1].IsValid()
}

func (r Range) CountLines() int {
	return r.End().Line() - r.Beg().Line() + 1
}

func (r Range) IsCursor() bool {
	return r.Beg().Line() == r.End().Line() && r.Beg().Column() == r.End().Column()
}

func (r Range) String() string {
	return fmt.Sprintf("%s-%s", r.Beg(), r.End())
}

func (r Range) TS() tree_sitter.Range {
	return tree_sitter.Range{
		StartPoint: r.Beg().TS(),
		EndPoint:   r.End().TS(),
	}
}

func (r Range) Beg() Pos {
	return r[0]
}

func (r Range) End() Pos {
	return r[1]
}

func (r Range) Lines() []int {
	lines := make([]int, 1+r.End().Line()-r.Beg().Line())
	k := 0
	for i := r.Beg().Line(); i <= r.End().Line(); i++ {
		lines[k] = i
		k += 1
	}
	return lines
}

func (r Range) Contains(pos Pos, includeEnd bool) bool {
	if pos.Line() < r.Beg().Line() {
		return false
	}
	if pos.Line() > r.End().Line() {
		return false
	}
	if pos.Line() == r.Beg().Line() && pos.Column() < r.Beg().Column() {
		return false
	}
	if pos.Line() == r.End().Line() && pos.Column() >= r.End().Column() {
		if includeEnd && pos.Column() == r.End().Column() {
			return true
		}
		return false
	}
	return true
}
