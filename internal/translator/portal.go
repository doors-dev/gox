package translator

import (
	"slices"

	"github.com/doors-dev/gox/internal/common"
)

type Portal struct {
	self  common.Range
	other common.Range
}

func (p Portal) String() string {
	return "self: " + p.self.String() + ", other: " + p.other.String()
}

func (p Portal) Self() common.Range {
	return p.self
}

func (p Portal) Other() common.Range {
	return p.other
}

func (p Portal) Reverse() Portal {
	return Portal{
		self:  p.other,
		other: p.self,
	}
}

func (p Portal) Convert(pos common.Pos) common.Pos {
	switch p.self.End().Compare(pos) {
	case 0:
		return p.other.End()
	case 1:
		line := pos.Line() - p.self.End().Line() + p.other.End().Line()
		column := pos.Column()
		if pos.Line() == p.self.Beg().Line() {
			column += p.other.Beg().Column() - p.self.Beg().Column()
		}
		return common.NewPos(max(0, line), max(0, column))
	default:
		line := pos.Line() - p.self.Beg().Line() + p.other.Beg().Line()
		column := pos.Column()
		if pos.Line() == p.self.End().Line() {
			column += p.other.End().Column() - p.self.End().Column()
		}
		return common.NewPos(max(0, line), max(0, column))
	}
}

type portals []Portal

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (ps portals) findClosest(pos common.Pos) (Portal, bool) {
	reg := Portal{
		self:  common.NewRange(pos, pos.Advance()),
		other: common.NoRange(),
	}
	index, ok := slices.BinarySearchFunc(ps, reg, func(a, b Portal) int {
		return a.self.Beg().Compare(b.self.Beg())
	})
	if ok {
		return (ps)[index], true
	}
	var prev *Portal
	var next *Portal
	if index != 0 {
		prev = &ps[index-1]
	}
	if len(ps) > index {
		next = &ps[index]
	}
	switch true {
	case prev == nil && next == nil:
		return Portal{}, false
	case prev == nil:
		return *next, true
	case next == nil:
		return *prev, true
	default:
		if prev.self.Contains(pos, true) {
			return *prev, true
		}
		left := prev.self.End()
		right := next.self.Beg()
		linesLeft := abs(left.Line() - pos.Line())
		linesRight := abs(right.Line() - pos.Line())
		switch true {
		case linesLeft < linesRight:
			return *prev, true
		case linesLeft > linesRight:
			return *next, true
		case linesRight == 0:
			columnsLeft := abs(left.Column() - pos.Column())
			columnsRight := abs(right.Column() - pos.Column())
			switch true {
			case columnsLeft > columnsRight:
				return *next, true
			default:
				return *prev, true
			}
		default:
			return *prev, true
		}
	}
}

func (ps portals) findPos(pos common.Pos, includeEnd bool) (Portal, bool) {
	reg := Portal{
		self:  common.NewRange(pos, pos.Advance()),
		other: common.NoRange(),
	}
	index, ok := slices.BinarySearchFunc(ps, reg, func(a, b Portal) int {
		return a.self.Beg().Compare(b.self.Beg())
	})
	if ok {
		return (ps)[index], true
	}
	if index == 0 {
		return Portal{}, false
	}
	prev := (ps)[index-1]
	if !prev.self.Contains(pos, includeEnd) {
		return Portal{}, false
	}
	return prev, true
}

func (ps *portals) append(p Portal) {
	if len(*ps) == 0 {
		*ps = append(*ps, p)
		return
	}
	last := (*ps)[len(*ps)-1]
	switch last.self.End().Compare(p.self.Beg()) {
	case 1:
		panic("region overlap, previous inculdes current begining")
	default:
		*ps = append(*ps, p)
	}
}
