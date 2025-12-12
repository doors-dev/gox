package text

import (
	"unicode/utf8"

	"github.com/doors-dev/gox/internal/common"
)

func (d Text) offset(pos common.Pos) int {
	if pos.Line() >= len(d.lineOffsets) {
		return -1
	}
	offsets := d.lineOffsets[pos.Line()]
	offset := offsets[0] + pos.Column()
	if offset > offsets[1] {
		return -1
	}
	return offset
}

func (d Text) IntoRange(enc common.Encoding, rang common.Range) common.Range {
	beg := d.IntoPos(enc, rang.Beg())
	end := d.IntoPos(enc, rang.End())
	return common.NewRange(beg, end)
}

func (d Text) FromRange(enc common.Encoding, rang common.Range) common.Range {
	beg := d.FromPos(enc, rang.Beg())
	end := d.FromPos(enc, rang.End())
	return common.NewRange(beg, end)
}

func (d Text) IntoPos(enc common.Encoding, pos common.Pos) common.Pos {
	if len(d.lineOffsets) <= pos.Line() {
		return pos
	}
	switch enc {
	case common.UTF16:
		l := d.MustLine(pos.Line())
		return common.NewPos(pos.Line(), Utf16to8(l, pos.Column()))
	case common.UTF8:
		return pos
	default:
		panic("invalid encoding")
	}
}

func (d Text) FromPos(enc common.Encoding, pos common.Pos) common.Pos {
	if len(d.lineOffsets) <= pos.Line() {
		return pos
	}
	switch enc {
	case common.UTF16:
		l := d.MustLine(pos.Line())
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
