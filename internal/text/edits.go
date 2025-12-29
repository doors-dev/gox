package text

import (
	"bufio"
	"bytes"
	"errors"
	"slices"
	"strings"
	"unicode"

	"github.com/doors-dev/gox/internal/common"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func (t Text) CR() {
	last := t.lineOffsets[len(t.lineOffsets)-1]
	last.SetEnd(len(t.source))
	t.source = append(t.source, '\n')
	beg := len(t.source)
	lastIndent := t.lastIdent()
	t.source = append(t.source, lastIndent...)
	end := len(t.source)
	t.lineOffsets = append(t.lineOffsets, newOffset(beg, end))
}

func (t Text) AppendString(s string) {
	t.Append([]byte(s))
}

func (t Text) Append(s []byte) {
	scanner := bufio.NewScanner(bytes.NewReader(s))
	scanner.Buffer(nil, maxTokenSize)
	first := true
	for scanner.Scan() {
		beg := len(t.source)
		t.source = append(t.source, scanner.Bytes()...)
		end := len(t.source)
		t.source = append(t.source, '\n')
		offset := newOffset(beg, end)
		if first {
			first = false
			last := len(t.lineOffsets) - 1
			if last == -1 {
				t.lineOffsets = append(t.lineOffsets, offset)
			} else {
				t.lineOffsets[last] = newOffset(t.lineOffsets[last].Beg(), offset.End())
			}
		} else {
			t.lineOffsets = append(t.lineOffsets, offset)
		}
	}
	if !first {
		if s[len(s)-1] == '\n' {
			t.lineOffsets = append(t.lineOffsets, newOffset(len(t.source), len(t.source)))
		} else {
			t.source = t.source[:len(t.source)-1]
		}
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
}

func (t *text) IndentRef(line []byte) {
	i := 0
	for unicode.IsSpace(rune(line[i])) && i < len(line) {
		i += 1
	}
	indent := indent{
		prefix: slices.Clone(line[:i]),
		fake:   false,
	}
	t.indents = append(t.indents, indent)
}

func (t *text) IndentBeg() {
	prefix := append(t.lastIdent(), '\t')
	indent := indent{
		prefix: prefix,
		fake:   false,
	}
	t.indents = append(t.indents, indent)
}

func (t *text) IndentFake() {
	t.CR()
	t.AppendString("{")
	prefix := append(t.lastIdent(), '\t')
	indent := indent{
		prefix: prefix,
		fake:   true,
	}
	t.indents = append(t.indents, indent)
}

func (t *text) IndentEnd() {
	last := t.indents[len(t.indents)-1]
	t.indents = t.indents[:len(t.indents)-1]
	if last.fake {
		t.CR()
		t.AppendString("}")
	}
}

func (t *text) LastPos() common.Pos {
	if len(t.lineOffsets) == 0 {
		return common.NewPos(0, 0)
	}
	return common.NewPos(len(t.lineOffsets), 0)
}

func (t *text) lastPos() common.Pos {
	if len(t.lineOffsets) == 0 {
		return common.NewPos(0, 0)
	}
	line := len(t.lineOffsets) - 1
	last := t.lineOffsets[line]
	return common.NewPos(line, last.Len())
}

func (d Text) Update(content string) (tree_sitter.InputEdit, bool, error) {
	b := common.Bytes(content)
	for i := len(b) - 1; i >= 0 && b[i] == '\n'; i-- {
		b = b[:i]
	}
	if slices.Equal(b, d.source) {
		return tree_sitter.InputEdit{}, false, nil
	}
	rang := common.Range{common.Pos{0, 0}, d.lastPos()}
	return d.Patch(rang, content)
}

func (d Text) Patch(ran common.Range, content string) (tree_sitter.InputEdit, bool, error) {
	defer d.ensureLineOffsets()
	// slog.Info("patch", "rang", ran, "content", content)
	patch, err := d.preparePatch(ran, content)
	if err != nil {
		return tree_sitter.InputEdit{}, false, err
	}
	if !patch.isValid() {
		return tree_sitter.InputEdit{}, false, nil
	}
	d.source = slices.Replace(d.source, patch.beg, patch.end, patch.content...)
	for i := patch.ran.End().Line() + 1; i < len(d.lineOffsets); i++ {
		d.lineOffsets[i].Shift(patch.diff())
	}
	endLine := patch.ran.End().Line()
	if endLine < len(d.lineOffsets) {
		endLine += 1
	}
	d.lineOffsets = slices.Replace(d.lineOffsets, patch.ran.Beg().Line(), endLine, patch.offsets...)
	return tree_sitter.InputEdit{
		StartByte:      uint(patch.beg),
		OldEndByte:     uint(patch.end),
		NewEndByte:     uint(patch.newEndOffset()),
		StartPosition:  tree_sitter.Point{Row: uint(patch.ran.Beg().Line()), Column: uint(patch.ran.Beg().Column())},
		OldEndPosition: tree_sitter.Point{Row: uint(patch.ran.End().Line()), Column: uint(patch.ran.End().Column())},
		NewEndPosition: tree_sitter.Point{Row: uint(patch.newEndLine), Column: uint(patch.newEndColumn)},
	}, true, nil
}

func (d Text) preparePatch(ran common.Range, content string) (patch, error) {
	beg := d.offset(ran.Beg())
	end := d.offset(ran.End())
	offsets := []offset{}
	buf := bytes.Buffer{}
	// slog.Info("patch", "rang", ran, "beg", beg, "end", end)
	if beg == -1 {
		// allow only empty lines above the exiting lines
		if ran.Beg().Column() != 0 {
			return patch{}, errors.New("invalid range")
		}
		begLine := len(d.lineOffsets) - 1
		begCol := d.lineOffsets[begLine].Len()
		beg = len(d.source)
		missingLines := ran.Beg().Line() - begLine
		for range missingLines {
			offset := buf.Len()
			offsets = append(offsets, newOffset(offset, offset))
			buf.WriteByte('\n')
		}
		pos := common.NewPos(begLine, begCol)
		ran = common.NewRange(pos, pos)
		end = beg
		// slog.Info("patch upd", "rang", ran, "beg", beg, "end", end, "addedLines", missingLines)
	} else if end == -1 {
		end = len(d.source)
		endLine := len(d.lineOffsets) - 1
		endCol := d.lineOffsets[endLine].Len()
		ran = common.NewRange(ran.Beg(), common.NewPos(endLine, endCol))
	}
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(nil, maxTokenSize)
	for scanner.Scan() {
		beg := buf.Len()
		buf.Write(scanner.Bytes())
		end := buf.Len()
		buf.WriteByte('\n')
		offset := newOffset(beg, end)
		offsets = append(offsets, offset)
	}
	if err := scanner.Err(); err != nil {
		return patch{}, err
	}
	if end != len(d.source) && strings.HasSuffix(content, "\n") {
		offsets = append(offsets, newOffset(buf.Len(), buf.Len()))
	} else {
		if buf.Len() != 0 {
			buf.Truncate(buf.Len() - 1)
		}
	}
	if len(offsets) == 0 {
		offsets = append(offsets, newOffset(0, 0))
	}
	if end == len(d.source) {
		for i := len(offsets) - 1; i > 0 && offsets[i].Len() == 0; i-- {
			offsets = offsets[:i]
			buf.Truncate(buf.Len() - 1)
		}
	}
	if buf.Len() == 0 && (len(offsets) != 1 || offsets[0].Len() != 0) {
		panic("invalid offsets")
	}
	if buf.Len() == 0 && end == len(d.source) && ran.Beg().Column() == 0 {
		for i := ran.Beg().Line(); i > 0 && d.lineOffsets[i].Len() == 0; i-- {
			o := d.lineOffsets[i-1]
			beg = o.End()
			ran = common.NewRange(common.NewPos(i-1, o.Len()), ran.End())
		}
	}
	for i := range offsets {
		offsets[i].Shift(beg)
	}
	leftExpanstion := ran.Beg().Column()
	rightExpansion := 0
	if end < len(d.source) {
		rightExpansion = d.lineOffsets[ran.End().Line()].Len() - ran.End().Column()
	}
	offsets[0].ExpandLeft(leftExpanstion)
	newEndColumn := offsets[len(offsets)-1].Len()
	newEndLine := ran.Beg().Line() + len(offsets) - 1
	offsets[len(offsets)-1].ExpandRight(rightExpansion)

	newChunk := buf.Bytes()
	/*
		prevChunk := d.Slice(ran)
		editIndex := 0
		for i := range max(len(prevChunk), len(newChunk)) {
			if i > len(prevChunk) {
				if editIndex == 0 {
					editIndex = len(newChunk) - 1
				}
				break
			}
			if i > len(newChunk) {
				editIndex = len(newChunk) - 1
				break
			}
			if prevChunk[i] != newChunk[i] {
				editIndex = i
			}
		}
		editIndex += beg */
	return patch{
		ran:          ran,
		beg:          beg,
		end:          end,
		newEndColumn: newEndColumn,
		newEndLine:   newEndLine,
		offsets:      offsets,
		content:      newChunk,
	}, nil
}

type patch struct {
	beg          int
	end          int
	ran          common.Range
	newEndLine   int
	newEndColumn int
	offsets      []offset
	content      []byte
}

func (p *patch) diff() int {
	return p.newEndOffset() - p.end
}

func (p *patch) newEndOffset() int {
	return p.beg + len(p.content)
}

func (p *patch) isValid() bool {
	return p.beg != p.end || len(p.content) != 0
}
