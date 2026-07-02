package text

import (
	"unicode/utf8"

	"github.com/doors-dev/gox/internal/common"
)

func (t Text) offset(pos common.Pos, strict bool) int {
	if pos.Line() >= len(t.lineOffsets) {
		if strict {
			return -1
		}
		return len(t.source)
	}
	offsets := t.lineOffsets[pos.Line()]
	offset := offsets[0] + pos.Column()
	if offset > offsets[1] {
		if strict {
			return -1
		}
		return offsets[1]
	}
	return offset
}

func (t Text) IntoRange(enc common.Encoding, rang common.Range) common.Range {
	beg := t.IntoPos(enc, rang.Beg())
	end := t.IntoPos(enc, rang.End())
	return common.NewRange(beg, end)
}

func (t Text) FromRange(enc common.Encoding, rang common.Range) common.Range {
	beg := t.FromPos(enc, rang.Beg())
	end := t.FromPos(enc, rang.End())
	return common.NewRange(beg, end)
}

func (t Text) IntoPos(enc common.Encoding, pos common.Pos) common.Pos {
	if len(t.lineOffsets) <= pos.Line() {
		return pos
	}
	switch enc {
	case common.UTF16:
		l := t.MustLine(pos.Line())
		return common.NewPos(pos.Line(), Utf16to8(l, pos.Column()))
	case common.UTF8:
		return pos
	default:
		panic("invalid encoding")
	}
}

func (t Text) FromPos(enc common.Encoding, pos common.Pos) common.Pos {
	if len(t.lineOffsets) <= pos.Line() {
		return pos
	}
	switch enc {
	case common.UTF16:
		l := t.MustLine(pos.Line())
		return common.NewPos(pos.Line(), Utf8to16(l, pos.Column()))
	case common.UTF8:
		return pos
	default:
		panic("invalid encoding")
	}
}

func Utf16to8(b []byte, offset int) int {
	count16 := 0
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		n16 := 1
		if r > 0xFFFF {
			n16 = 2
		}
		if count16+n16 > offset {
			return i
		}
		count16 += n16
		i += size
	}
	if offset <= count16 {
		return len(b)
	}
	extra := offset - count16
	return len(b) + extra
}

func Utf8to16(b []byte, offset int) int {
	u16 := 0
	i := 0
	for i < offset && i < len(b) {
		r, sz := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && sz == 1 {
			i++
			u16++
			continue
		}
		if r <= 0xFFFF {
			u16++
		} else {
			u16 += 2
		}
		i += sz
	}
	if offset > i {
		u16 += offset - i
	}
	return u16
}

func newOffset(beg int, end int) offset {
	return offset{beg, end}
}

type offset [2]int

func (o *offset) ExpandLeft(length int) {
	o[0] -= length
}

func (o *offset) ExpandRight(length int) {
	o[1] += length
}

func (o *offset) SetEnd(offset int) {
	o[1] = offset
}

func (o *offset) Shift(diff int) {
	o[0] += diff
	o[1] += diff
}

func (o offset) Beg() int {
	return o[0]
}

func (o offset) End() int {
	return o[1]
}

func (o offset) Len() int {
	return o[1] - o[0]
}
