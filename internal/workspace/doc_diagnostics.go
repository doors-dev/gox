package workspace

import (
	"fmt"
	"slices"
	"strings"

	"github.com/doors-dev/gox/internal/catalog/grammer"
	"github.com/doors-dev/gox/internal/common"
	tree_sitter_gox "github.com/doors-dev/tree-sitter-gox/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

const variadicProxyMessage = "Variadic arguments are not supported in proxy position."

type SyntaxError struct {
	Range   common.Range
	Message string
}

// syntaxErrorNode is a raw ERROR/MISSING node collected from a parse tree,
// with positions in tree-sitter (UTF-8 byte) coordinates.
type syntaxErrorNode struct {
	ran     common.Range
	missing bool
	kind    string
	near    string
}

func (e syntaxErrorNode) message() string {
	if e.kind == grammer.VARIADIC_ARGUMENT {
		return variadicProxyMessage
	}
	if e.missing {
		return "syntax error: missing " + strings.TrimPrefix(e.kind, "gox_")
	}
	return "syntax error"
}

// SyntaxErrors reports ERROR/MISSING nodes as LSP diagnostics, with ranges in
// the client encoding.
func (d Doc) SyntaxErrors(enc common.Encoding) []SyntaxError {
	if d == nil || d.tree == nil {
		return nil
	}
	nodes := collectSyntaxErrors(d.source.Source(), d.tree.RootNode())
	if len(nodes) == 0 {
		return nil
	}
	out := make([]SyntaxError, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, SyntaxError{
			Range:   d.source.FromRange(enc, n.ran),
			Message: capitalize(n.message()),
		})
	}
	return out
}

// SyntaxErrorReport formats the ERROR/MISSING nodes of the current tree as
// indented "line:col: message near \"...\"" lines (1-based), or "" when clean.
func (d Doc) SyntaxErrorReport() string {
	if d == nil || d.tree == nil {
		return ""
	}
	return formatSyntaxErrors(collectSyntaxErrors(d.source.Source(), d.tree.RootNode()))
}

// SyntaxReport parses source standalone and returns the same report as
// SyntaxErrorReport, for callers that do not hold a Doc.
func SyntaxReport(source []byte) string {
	lang := tree_sitter.NewLanguage(tree_sitter_gox.Language())
	parser := tree_sitter.NewParser()
	if err := parser.SetLanguage(lang); err != nil {
		return ""
	}
	defer parser.Close()
	tree := parser.Parse(source, nil)
	defer tree.Close()
	return formatSyntaxErrors(collectSyntaxErrors(source, tree.RootNode()))
}

func formatSyntaxErrors(nodes []syntaxErrorNode) string {
	if len(nodes) == 0 {
		return ""
	}
	var b strings.Builder
	for i, n := range nodes {
		if i > 0 {
			b.WriteByte('\n')
		}
		beg := n.ran.Beg()
		fmt.Fprintf(&b, "  %d:%d: %s", beg.Line()+1, beg.Column()+1, n.message())
		if !n.missing && n.near != "" {
			fmt.Fprintf(&b, " near %q", n.near)
		}
	}
	return b.String()
}

func collectSyntaxErrors(source []byte, root *tree_sitter.Node) []syntaxErrorNode {
	if root == nil {
		return nil
	}
	var out []syntaxErrorNode
	if root.HasError() {
		cursor := root.Walk()
		walkSyntaxErrors(source, cursor, root, &out)
		cursor.Close()
	}
	variadic := collectVariadicProxyArgs(root)
	if len(variadic) == 0 {
		return out
	}
	if len(out) == 0 {
		return variadic
	}
	out = append(out, variadic...)
	slices.SortStableFunc(out, func(a, b syntaxErrorNode) int {
		return a.ran.Beg().Compare(b.ran.Beg())
	})
	return out
}

func collectVariadicProxyArgs(root *tree_sitter.Node) []syntaxErrorNode {
	if root == nil {
		return nil
	}
	cursor := root.Walk()
	defer cursor.Close()
	var out []syntaxErrorNode
	for {
		node := cursor.Node()
		proxy := node.Kind() == grammer.GOX_TILDE_PROXY
		if proxy {
			appendVariadicProxyArgs(node, &out)
		}
		if !proxy && cursor.GotoFirstChild() {
			continue
		}
		for !cursor.GotoNextSibling() {
			if !cursor.GotoParent() {
				return out
			}
		}
	}
}

func appendVariadicProxyArgs(proxy *tree_sitter.Node, out *[]syntaxErrorNode) {
	value := proxy.ChildByFieldName("value")
	if value == nil {
		return
	}
	for i := range value.ChildCount() {
		arg := value.Child(i)
		if arg.Kind() != grammer.VARIADIC_ARGUMENT {
			continue
		}
		*out = append(*out, syntaxErrorNode{
			ran:  common.NewTSRange(arg.Range()),
			kind: arg.Kind(),
		})
	}
}

func walkSyntaxErrors(source []byte, cursor *tree_sitter.TreeCursor, node *tree_sitter.Node, out *[]syntaxErrorNode) {
	if node.IsError() {
		*out = append(*out, syntaxErrorNode{
			ran:  common.NewTSRange(node.Range()),
			kind: node.Kind(),
			near: snippet(node.Utf8Text(source)),
		})
		return
	}
	if node.IsMissing() {
		*out = append(*out, syntaxErrorNode{
			ran:     common.NewTSRange(node.Range()),
			missing: true,
			kind:    node.Kind(),
		})
		return
	}
	children := node.Children(cursor)
	for i := range children {
		child := &children[i]
		if !child.HasError() {
			continue
		}
		walkSyntaxErrors(source, cursor, child, out)
	}
}

func snippet(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	const max = 40
	r := []rune(s)
	if len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
