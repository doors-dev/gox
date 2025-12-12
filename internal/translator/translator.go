package translator

import (
	"github.com/doors-dev/gox/internal/common"
)

type Translator = *translator

func NewTranslator() Translator {
	return &translator{}
}

type translator struct {
	sourcePortals portals
	targetPortals portals
}

func (t Translator) Append(source, target common.Range) {
	p := Portal{
		self:  source,
		other: target,
	}
	t.sourcePortals.append(p)
	t.targetPortals.append(p.Reverse())
}

func (t Translator) ApproximateConvertToTarget(source common.Pos) (common.Pos, bool) {
	p, ok := t.sourcePortals.findClosest(source)
	if !ok {
		return common.NoPos(), false
	}
	return p.Convert(source), true
}

func (d Translator) ApproximateConvertToSource(target common.Pos) (common.Pos, bool) {
	p, ok := d.targetPortals.findClosest(target)
	if !ok {
		return common.NoPos(), false
	}
	return p.Convert(target), true
}

func (t Translator) ConvertToSource(pos common.Pos, includeEnd bool) (common.Pos, bool) {
	p, ok := t.TargetPos(pos, includeEnd)
	if !ok {
		return common.NoPos(), false
	}
	return p.Convert(pos), true
}

func (t Translator) ConvertToTarget(pos common.Pos, includeEnd bool) (common.Pos, bool) {
	p, ok := t.SourcePos(pos, includeEnd)
	if !ok {
		return common.NoPos(), false
	}
	return p.Convert(pos), true
}

func (t Translator) RangeConvertToTarget(source common.Range) (common.Range, bool) {
	p, ok := t.SourcePos(source.Beg(), true)
	if !ok {
		return common.NoRange(), false
	}
	if !p.self.Contains(source.End(), true) {
		return common.NoRange(), false
	}
	return common.NewRange(p.Convert(source.Beg()), p.Convert(source.End())), true
}

func (t Translator) RangeConvertToSource(target common.Range) (common.Range, bool) {
	p, ok := t.TargetPos(target.Beg(), true)
	if !ok {
		return common.NoRange(), false
	}
	if !p.self.Contains(target.End(), true) {
		return common.NoRange(), false
	}
	return common.NewRange(p.Convert(target.Beg()), p.Convert(target.End())), true
}

/*
func (t Translator) SourceRange(r common.Range) []Portal {
	return t.sourcePortals.findRange(r)
}

func (t Translator) TargetRange(r common.Range) []Portal {
	return t.targetPortals.findRange(r)
} */

func (t Translator) SourcePos(pos common.Pos, includeEnd bool) (Portal, bool) {
	return t.sourcePortals.findPos(pos, includeEnd)
}

func (t Translator) TargetPos(pos common.Pos, includeEnd bool) (Portal, bool) {
	return t.targetPortals.findPos(pos, includeEnd)
}
