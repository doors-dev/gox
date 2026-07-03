package workspace

import (
	"fmt"
	"testing"

	"github.com/doors-dev/gox/internal/catalog/grammer"
	"github.com/doors-dev/gox/internal/common"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

const spaceHintTemplate = `package demo

import "github.com/doors-dev/gox"

elem F(v string) {
	%s
}
`

func spaceHintDoc(t *testing.T, content string) Doc {
	t.Helper()
	return makeDoc(t, fmt.Sprintf(spaceHintTemplate, content))
}

func contentLineRange(begCol, endCol int) common.Range {
	return common.NewRange(common.NewPos(5, begCol), common.NewPos(5, endCol))
}

func assertSpaceHints(t *testing.T, hints []SyntaxError, want []common.Range) {
	t.Helper()
	if len(hints) != len(want) {
		t.Fatalf("hints = %+v, want %d hints", hints, len(want))
	}
	for i, h := range hints {
		if h.Range != want[i] {
			t.Fatalf("hints[%d].Range = %v, want %v", i, h.Range, want[i])
		}
		if h.Message != "This space will be preserved in the output." {
			t.Fatalf("hints[%d].Message = %q", i, h.Message)
		}
	}
}

func TestSpaceHints(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    []common.Range
	}{
		{"leading", "<p> hi</p>", []common.Range{contentLineRange(4, 5)}},
		{"trailing", "<p>hi </p>", []common.Range{contentLineRange(6, 7)}},
		{"both", "<p> hi </p>", []common.Range{contentLineRange(4, 5), contentLineRange(7, 8)}},
		{"trailing before newline", "<p>hi \n\t</p>", []common.Range{contentLineRange(6, 7)}},
		{"no spaces", "<p>hi</p>", nil},
		{"newline boundaries", "<p>\n\thi\n\t</p>", nil},
		{"tab boundaries", "<p>\thi\t</p>", nil},
		{"space only content", "<p> </p>", nil},
		{"multi space only content", "<p>   </p>", nil},
		{"leading space after placeholder", "<p>~(v) text</p>", []common.Range{contentLineRange(8, 9)}},
		{"spaces around placeholder", "<p>a ~(v) b</p>", []common.Range{contentLineRange(5, 6), contentLineRange(10, 11)}},
		{"interior spaces", "<p>a   b</p>", nil},
		{"raw text", "<:> raw </:>", nil},
		{"script raw text", "<script> var x = 1; </script>", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := spaceHintDoc(t, c.content)
			assertSpaceHints(t, doc.SpaceHints(common.UTF8), c.want)
		})
	}
}

func TestSpaceHintsNilDoc(t *testing.T) {
	var doc Doc
	if hints := doc.SpaceHints(common.UTF8); hints != nil {
		t.Fatalf("nil doc hints = %+v, want nil", hints)
	}
}

func TestSpaceHintsSpaceOnlyContentIsFillerNode(t *testing.T) {
	doc := spaceHintDoc(t, "<p> </p>")
	if fillers := countNodesOfKind(doc.tree.RootNode(), grammer.GOX_SPACE_FILLER); fillers != 1 {
		t.Fatalf("space filler nodes = %d, want 1", fillers)
	}
	if texts := countNodesOfKind(doc.tree.RootNode(), grammer.GOX_PLAIN_TEXT); texts != 0 {
		t.Fatalf("plain text nodes = %d, want 0", texts)
	}
	assertSpaceHints(t, doc.SpaceHints(common.UTF8), nil)
}

func TestSpaceHintsEncodingConversion(t *testing.T) {
	doc := spaceHintDoc(t, "<p> héé </p>")
	assertSpaceHints(t, doc.SpaceHints(common.UTF8), []common.Range{
		contentLineRange(4, 5),
		contentLineRange(10, 11),
	})
	assertSpaceHints(t, doc.SpaceHints(common.UTF16), []common.Range{
		contentLineRange(4, 5),
		contentLineRange(8, 9),
	})
}

func countNodesOfKind(root *tree_sitter.Node, kind string) int {
	cursor := root.Walk()
	defer cursor.Close()
	count := 0
	for {
		if cursor.Node().Kind() == kind {
			count++
		}
		if cursor.GotoFirstChild() {
			continue
		}
		for !cursor.GotoNextSibling() {
			if !cursor.GotoParent() {
				return count
			}
		}
	}
}
