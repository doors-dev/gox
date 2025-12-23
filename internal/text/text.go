package text

import (
	"slices"

	"github.com/doors-dev/gox/internal/common"
)

const maxTokenSize = 1 << 24

func NewText() Text {
	t := &text{}
	t.ensureLineOffsets()
	return t
}

type Text = *text

type indent struct {
	prefix []byte
	fake   bool
}

type text struct {
	source      []byte
	lineOffsets []offset
	indents     []indent
}

func (t Text) Clone() Text {
	return &text{
		source:      slices.Clone(t.source),
		lineOffsets: slices.Clone(t.lineOffsets),
		indents:     slices.Clone(t.indents),
	}
}

func (t Text) Cursor() common.Pos {
	last := len(t.lineOffsets) - 1
	if last == -1 {
		return common.NewPos(0, 0)
	}
	lastOffset := t.lineOffsets[last]
	return common.NewPos(last, lastOffset.Len())
}

func (t *text) lastIdent() []byte {
	if len(t.indents) == 0 {
		return []byte{}
	}
	return t.indents[len(t.indents)-1].prefix
}
