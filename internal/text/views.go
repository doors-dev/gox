package text

import (
	"github.com/doors-dev/gox/internal/common"
)

func (t Text) String() string {
	return string(t.source)
}

func (t Text) Source() []byte {
	return t.source
}

func (d Text) MustLine(n int) []byte {
	return d.source[d.lineOffsets[n].Beg():d.lineOffsets[n].End()]
}

func (d Text) Slice(rang common.Range) []byte {
	if rang.IsCursor() {
		return nil
	}
	startOffset := d.offset(rang.Beg())
	endOffset := d.offset(rang.End())
	if startOffset == -1 || endOffset == -1 {
		panic("invalid range")
	}
	return d.source[startOffset:endOffset]
}

func (d Text) Rune(offset int) (rune, bool) {
	if offset < 0 || offset >= len(d.source) {
		return 0, false
	}
	return rune(d.source[offset]), true
}
