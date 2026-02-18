package assembler

import (
	"bytes"
	"encoding/json"
	"regexp"

	"github.com/doors-dev/gox/internal/common"
	"github.com/doors-dev/gox/internal/text"
	"github.com/doors-dev/gox/internal/translator"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func Assemble(source text.Text, root *tree_sitter.Node) (text.Text, translator.Translator) {
	target := text.NewText()
	translator := translator.NewTranslator()
	a := &assembler{
		target:     target,
		translator: translator,
		source:     source,
	}
	a.append(r("// Managed by GoX "), r(Version()))
	a.cr()
	a.cr()
	scanGoSource(a, root)
	return target, translator
}

type Assembler = *assembler

type assembler struct {
	source     text.Text
	target     text.Text
	translator translator.Translator
}

func (a *assembler) portal(beg tree_sitter.Point, end tree_sitter.Point) {
	ran := common.NewRange(common.NewPos(int(beg.Row), int(beg.Column)), common.NewPos(int(end.Row), int(end.Column)))
	if ran.IsCursor() {
		return
	}
	targetBeg := a.target.Cursor()
	snippet := a.source.Slice(ran)
	a.target.Append(snippet)
	targetEnd := a.target.Cursor()
	a.translator.Append(ran, common.NewRange(targetBeg, targetEnd))
}

func (a *assembler) indentRef(ref tree_sitter.Point) {
	line := a.source.MustLine(int(ref.Row))
	a.target.IndentRef(line)
}

func (a *assembler) indentBeg() {
	a.target.IndentBeg()
}

func (a *assembler) indentFake() {
	a.target.IndentFake()
}

func (a *assembler) indentEnd() {
	a.target.IndentEnd()
}

func (a *assembler) cr() {
	a.target.CR()
}

var ws = regexp.MustCompile(`\s+`)

func (a *assembler) append(parts ...part) {
	for _, part := range parts {
		node, ok := part.Portal()
		if ok {
			a.portal(node.StartPosition(), node.EndPosition())
			continue
		}
		str, ok := part.Raw()
		if ok {
			a.target.AppendString(str)
			continue
		}
		node, ok = part.Str()
		if ok {
			s := node.Utf8Text(a.source.Source())
			a.target.Append(stringLiteral(s))
			continue
		}
		node, ok = part.Text()
		if ok {
			s := node.Utf8Text(a.source.Source())
			s = ws.ReplaceAllString(s, " ")
			a.target.Append(stringLiteral(s))
			continue
		}
	}
}

func stringLiteral(s string) []byte {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.Encode(s)
	buf.Truncate(buf.Len() - 1)
	return buf.Bytes()
}
