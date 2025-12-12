package workspace

import (
	"github.com/doors-dev/gox/internal/common"
)

type ConvMode int

const (
	Strict ConvMode = iota
	Edge
	Approximate
)

func (d Doc) TargetPos(enc common.Encoding, sourcePos common.Pos, mode ConvMode) (common.Pos, bool) {
	pos := d.source.IntoPos(enc, sourcePos)
	switch mode {
	case Strict:
		pos, ok := d.translator.ConvertToTarget(pos, false)
		if !ok {
			return common.NoPos(), false
		}
		return d.target.FromPos(enc, pos), true
	case Edge:
		pos, ok := d.translator.ConvertToTarget(pos, true)
		if !ok {
			return common.NoPos(), false
		}
		return d.target.FromPos(enc, pos), true
	case Approximate:
		pos, ok := d.translator.ApproximateConvertToTarget(pos)
		if !ok {
			return common.NoPos(), false
		}
		return d.target.FromPos(enc, pos), true
	default:
		panic("invalid mode")
	}
}

func (d Doc) SourcePos(enc common.Encoding, targetPos common.Pos, mode ConvMode) (common.Pos, bool) {
	pos := d.target.IntoPos(enc, targetPos)
	switch mode {
	case Strict:
		pos, ok := d.translator.ConvertToSource(pos, false)
		if !ok {
			return common.NoPos(), false
		}
		return d.source.FromPos(enc, pos), true
	case Edge:
		pos, ok := d.translator.ConvertToSource(pos, true)
		if !ok {
			return common.NoPos(), false
		}
		return d.source.FromPos(enc, pos), true
	case Approximate:
		pos, ok := d.translator.ApproximateConvertToSource(pos)
		if !ok {
			return common.NoPos(), false
		}
		return d.source.FromPos(enc, pos), true
	default:
		panic("invalid mode")
	}
}


func (d Doc) TargetRange(enc common.Encoding, sourceRange common.Range, mode ConvMode) (common.Range, bool) {
	ran := d.source.IntoRange(enc, sourceRange)
	switch mode {
	case Strict:
		r, ok := d.translator.RangeConvertToTarget(ran)
		if !ok {
			return common.NoRange(), false
		}
		return d.target.FromRange(enc, r), true
	case Edge:
		beg, ok := d.translator.ConvertToTarget(ran.Beg(), false)
		if !ok {
			return common.NoRange(), false
		}
		end, ok := d.translator.ConvertToTarget(ran.End(), true)
		if !ok {
			return common.NoRange(), false
		}
		return d.target.FromRange(enc, common.NewRange(beg, end)), true
	case Approximate:
		beg, ok := d.translator.ApproximateConvertToTarget(ran.Beg())
		if !ok {
			return common.NoRange(), false
		}
		end, ok := d.translator.ApproximateConvertToTarget(ran.End())
		if !ok {
			return common.NoRange(), false
		}
		return d.target.FromRange(enc, common.NewRange(beg, end)), true
	default:
		panic("invalid mode")
	}
}
func (d Doc) SourceRange(enc common.Encoding, targetRange common.Range, mode ConvMode) (common.Range, bool) {
	ran := d.target.IntoRange(enc, targetRange)
	switch mode {
	case Strict:
		r, ok := d.translator.RangeConvertToSource(ran)
		if !ok {
			return common.NoRange(), false
		}
		return d.source.FromRange(enc, r), true
	case Edge:
		beg, ok := d.translator.ConvertToSource(ran.Beg(), false)
		if !ok {
			return common.NoRange(), false
		}
		end, ok := d.translator.ConvertToSource(ran.End(), true)
		if !ok {
			return common.NoRange(), false
		}
		return d.source.FromRange(enc, common.NewRange(beg, end)), true
	case Approximate:

		beg, ok := d.translator.ApproximateConvertToSource(ran.Beg())
		if !ok {
			return common.NoRange(), false
		}
		end, ok := d.translator.ApproximateConvertToSource(ran.End())
		if !ok {
			return common.NoRange(), false
		}
		return d.source.FromRange(enc, common.NewRange(beg, end)), true
	default:
		panic("invalid mode")
	}
}

func (d Doc) ApproximateTargetPos(enc common.Encoding, sourcePos common.Pos) (common.Pos, bool) {
	pos := d.source.IntoPos(enc, sourcePos)
	pos, ok := d.translator.ApproximateConvertToTarget(pos)
	if !ok {
		return common.NoPos(), false
	}
	return d.target.FromPos(enc, pos), true
}

/*
func (d Doc) ApproximateTargetRange(enc common.Encoding, sourceRange common.Range) (common.Range, bool) {
	ran := d.source.IntoRange(enc, sourceRange)
	beg, ok := d.translator.ApproximateConvertToTarget(ran.Beg())
	if !ok {
		return common.NoRange(), false
	}
	end, ok := d.translator.ApproximateConvertToTarget(ran.End())
	if !ok {
		return common.NoRange(), false
	}
	return d.target.FromRange(enc, common.NewRange(beg, end)), true
}

func (d Doc) ApproximateSourceRange(enc common.Encoding, targetRange common.Range) (common.Range, bool) {
	ran := d.target.IntoRange(enc, targetRange)
	beg, ok := d.translator.ApproximateConvertToSource(ran.Beg())
	if !ok {
		return common.NoRange(), false
	}
	end, ok := d.translator.ApproximateConvertToSource(ran.End())
	if !ok {
		return common.NoRange(), false
	}
	return d.source.FromRange(enc, common.NewRange(beg, end)), true
} */
