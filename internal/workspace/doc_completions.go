package workspace

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/doors-dev/gox/internal/catalog/attrs"
	"github.com/doors-dev/gox/internal/catalog/completion"
	"github.com/doors-dev/gox/internal/catalog/grammer"
	"github.com/doors-dev/gox/internal/catalog/tags"
	"github.com/doors-dev/gox/internal/common"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type Completion struct {
	Range common.Range
	Text  string
	Label string
	kind  completion.Kind
}

func (c Completion) Kind() completion.Kind {
	if c.kind != 0 {
		return c.kind
	}
	return completion.Text
}

func hasAttrs(node *tree_sitter.Node) bool {
	child := node.ChildByFieldName("attrs")
	return child != nil
}

func tagIsEditable(node *tree_sitter.Node) bool {
	cursor := node.Walk()
	defer cursor.Close()
	for _, child := range node.Children(cursor) {
		if child.IsMissing() {
			continue
		}
		if child.Kind() == grammer.GOX_SELF_CLOSING_HEAD_END || child.Kind() == grammer.GOX_HEAD_END {
			return false
		}
	}
	return true
}

func findImpicidCloseName(source []byte, node *tree_sitter.Node) (string, bool) {
	parent := node.Parent()
	if parent == nil {
		return "", false
	}
	if parent.IsError() {
		return findImpicidCloseName(source, parent)
	}
	kind := parent.Kind()
	if !strings.HasPrefix(kind, "gox") || strings.HasPrefix(kind, "gox_tilde") || kind == grammer.GOX_BLOCK {
		return "", false
	}
	if kind != grammer.GOX_SCRIPT_HEAD && kind != grammer.GOX_STYLE_HEAD && kind != grammer.GOX_RAW_HEAD {
		close := parent.ChildByFieldName("close")
		if close == nil {
			return findImpicidCloseName(source, parent)
		}
		if close.Kind() != grammer.GOX_IMPLICID_CLOSE {
			return findImpicidCloseName(source, parent)
		}
	}
	open := parent.ChildByFieldName("open")
	if open == nil {
		return "", false
	}
	name := open.ChildByFieldName("name")
	if name == nil {
		return "", false
	}
	return name.Utf8Text(source), true
}

func isAttrAssigned(source []byte, node *tree_sitter.Node) (yes bool) {
	i := int(node.EndByte())
	for i < len(source) {
		rune, l := utf8.DecodeRune(source[i:])
		if unicode.IsSpace(rune) {
			i += l
			continue
		}
		yes = rune == '='
		break
	}
	return
}

func findAttrTagName(source []byte, attrName *tree_sitter.Node) (string, bool) {
	attr := attrName.Parent()
	if attr == nil {
		return "", false
	}
	open := attr.Parent()
	if open == nil {
		return "", false
	}
	name := open.ChildByFieldName("name")
	if name == nil {
		return "", false
	}
	return name.Utf8Text(source), true
}

func isFirstChild(node *tree_sitter.Node) bool {
	parent := node.Parent()
	if parent == nil {
		return false
	}
	child := parent.Child(0)
	if child == nil {
		return false
	}
	return child.Equals(*node)
}

func isSnippet(source []byte, node *tree_sitter.Node) bool {
	if node.Kind() != grammer.GOX_TILDE_MARKER {
		return false
	}
	if (node.EndByte() - node.StartByte()) == 1 {
		return false
	}
	return source[node.EndByte()-1] == '~'
}

func isGox(node *tree_sitter.Node) bool {
	node = node.Parent()
	for {
		if node == nil {
			return false
		}
		if node.IsError() {
			node = node.Parent()
			continue
		}
		return strings.HasPrefix(node.GrammarName(), "gox_")
	}
}

func (d Doc) Completions(enc common.Encoding, pos common.Pos) (completions []Completion, complete bool) {
	sourcePos := d.source.IntoPos(enc, pos)
	nodePos := common.NewPos(sourcePos.Line(), max(sourcePos.Column()-1, 0))
	node := d.tree.RootNode().DescendantForPointRange(nodePos.TS(), nodePos.TS())
	if node == nil {
		return
	}
	if node.Kind() == grammer.GOX_ATTR_NAME {
		assigned := isAttrAssigned(d.source.Source(), node)
		tagName, ok := findAttrTagName(d.source.Source(), node)
		if !ok {
			return
		}
		str := node.Utf8Text(d.source.Source())
		attrs := attrs.Find(tagName, str)
		if len(attrs) == 0 {
			return
		}
		rang := d.source.FromRange(enc, common.NewTSRange(node.Range()))
		first := attrs[0]
		if !assigned && (len(attrs) == 1 || first.IsEqual(str)) {
			completions = append(completions, Completion{
				Range: rang,
				Text:  first.String() + "=($0)",
				Label: first.String() + "=(..)",
				kind:  completion.Snippet,
			})
			completions = append(completions, Completion{
				Range: rang,
				Text:  first.String() + "=({\n\t$0\n})",
				Label: first.String() + "=({..})",
				kind:  completion.Snippet,
			})
		}
		for _, attr := range attrs {
			var text string
			var label string
			if assigned || attr.IsBool() {
				text = attr.String()
				label = attr.String()
			} else {
				text = attr.String() + "=\"$0\""
				label = attr.String() + "=\"..\""
			}
			completions = append(completions, Completion{
				Range: rang,
				Text:  text,
				Label: label,
				kind:  completion.Snippet,
			})
		}
	}
	if node.Kind() == grammer.GOX_TILDE_MARKER || (node.Kind() == "~" && isGox(node)) {
		ran := d.source.FromRange(enc, common.NewTSRange(node.Range()))
		if isSnippet(d.source.Source(), node) {
			if !isFirstChild(node) {
				return
			}
			completions = append(completions, Completion{
				Text:  "~~\n$0\n~~",
				Label: "~~ ... ",
				Range: ran,
				kind:  completion.Snippet,
			})
			return
		}
		completions = append(completions, Completion{
			Text:  "~($0)",
			Label: "~(..)",
			Range: ran,
			kind:  completion.Snippet,
		})
		completions = append(completions, Completion{
			Text:  "~(if $1 {\n\t$2\n})",
			Label: "~(if..)",
			Range: ran,
			kind:  completion.Snippet,
		})
		completions = append(completions, Completion{
			Text:  "~(for $1 {\n\t$2\n})",
			Label: "~(for..)",
			Range: ran,
			kind:  completion.Snippet,
		})
		completions = append(completions, Completion{
			Text:  "~({\n\t$0\n})",
			Label: "~({..})",
			Range: ran,
			kind:  completion.Snippet,
		})
		completions = append(completions, Completion{
			Text:  "~>($0)",
			Label: "~>(..)",
			Range: ran,
			kind:  completion.Snippet,
		})
		completions = append(completions, Completion{
			Text:  "~~\n$0\n~~",
			Label: "~~ ... ",
			Range: ran,
			kind:  completion.Snippet,
		})
		return
	}
	if node.Kind() == grammer.GOX_HEAD_NAME {
		str := node.Utf8Text(d.source.Source())
		parent := node.Parent()
		if parent == nil {
			return
		}
		if !tagIsEditable(parent) {
			return
		}
		var ran common.Range
		ran = d.source.FromRange(enc, common.NewTSRange(node.Range()))
		isOpen := parent.Kind() == grammer.GOX_OPEN_HEAD
		isSelf := parent.Kind() == grammer.GOX_SELF_CLOSING_HEAD
		if isOpen || isSelf {
			options := tags.FindTag(str)
			for _, tag := range options {
				var label = "<" + tag.String() + "/>"
				var text string
				if tag.IsVoid() {
					text = tag.String() + ">"
				} else {
					text = tag.String() + "$2>$1</" + tag.String() + ">"
				}
				completions = append(completions, Completion{
					Range: ran,
					kind:  completion.Snippet,
					Text:  text,
					Label: label,
				})
			}
			return
		}
		return
	}
	if node.Kind() == grammer.GOX_ERRONEOUS_CLOSE_HEAD_NAME {
		complete = true
		name, ok := findImpicidCloseName(d.source.Source(), node)
		if !ok {
			return
		}
		ran := d.source.FromRange(enc, common.NewTSRange(node.Range()))
		completions = append(completions, Completion{
			Range: ran,
			Text:  name + ">",
			Label: "</" + name + ">",
			kind:  completion.Snippet,
		})
		return
	}
	if sourcePos.Column() < 2 {
		return
	}
	beg := common.NewPos(sourcePos.Line(), sourcePos.Column()-2)
	tail := string(d.source.Slice(common.NewRange(beg, sourcePos)))
	if tail == "</" {
		name, ok := findImpicidCloseName(d.source.Source(), node)
		if !ok {
			return
		}
		completions = append(completions, Completion{
			Range: common.NewRange(pos, pos),
			Text:  name + ">",
			Label: "</" + name + ">",
			kind:  completion.Snippet,
		})
		return
	}
	return
}
