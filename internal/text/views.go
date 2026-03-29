package text

import (
	"github.com/doors-dev/gox/internal/common"
)

func (t Text) String() string {
	return string(t.Source())
}

func (t Text) Source() []byte {
	source := t.source
	if len(source) > 0 && source[len(source)-1] != '\n' {
		source = append(source, '\n')
	}
	return source
}

func (t Text) MustLine(n int) []byte {
	return t.source[t.lineOffsets[n].Beg():t.lineOffsets[n].End()]
}

func (t Text) Slice(rang common.Range) []byte {
	if rang.IsCursor() {
		return nil
	}
	startOffset := t.offset(rang.Beg(), false)
	endOffset := t.offset(rang.End(), false)
	if startOffset == -1 || endOffset == -1 {
		panic("invalid range")
	}
	return t.source[startOffset:endOffset]
}

func (t Text) Rune(offset int) (rune, bool) {
	if offset < 0 || offset >= len(t.source) {
		return 0, false
	}
	return rune(t.source[offset]), true
}
