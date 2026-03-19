package assembler

import (
	"github.com/doors-dev/gox/internal/common"
	tree_sitter_gox "github.com/doors-dev/tree-sitter-gox/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type query int

const (
	QueryElemKeyword query = iota
	QueryPackage
	QueryElementOrBlock
	QueryGoXImport
	QueryFields
	QueryInterfaceMethods
)

var queries = [...]string{
	QueryElementOrBlock: `(gox_element) (gox_block)`,
	QueryPackage:        `(package_clause) @package`,
	QueryElemKeyword:    `("elem")`,
	QueryGoXImport: `
(
  (import_spec
    path: (raw_string_literal) @path
  ) @import
  (#eq? @path "` + "`github.com/doors-dev/gox`" + `")
)

(
  (import_spec
    path: (interpreted_string_literal) @path
  ) @import
  (#eq? @path "\"github.com/doors-dev/gox\"")
)
	`,
	QueryFields: `
(field_declaration 
		name: (field_identifier) @name
)
`,
	QueryInterfaceMethods: `
(method_elem 
	  name: (field_identifier) @name
)
`,
}

func (d query) Exists(source []byte, node *tree_sitter.Node) bool {
	matches, cursor := d.Matches(source, node)
	defer cursor.Close()
	return matches.Next() != nil
}

func (q query) IterateCapture(source []byte, node *tree_sitter.Node, capture string, f func(node *tree_sitter.Node) bool) {
	captures, cursor := q.Captures(source, node)
	defer cursor.Close()
	query := buildQueries[q]
	index, ok := query.CaptureIndexForName(capture)
	if !ok {
		panic("error: capture not found")
	}
	m, _ := captures.Next()
	for m != nil {
		index := uint32(index)
		for _, capture := range m.Captures {
			if capture.Index != index {
				continue
			}
			if f(&capture.Node) {
				return
			}
		}
		m, _ = captures.Next()
	}
}

func (q query) Captures(source []byte, node *tree_sitter.Node) (tree_sitter.QueryCaptures, *tree_sitter.QueryCursor) {
	cursor := tree_sitter.NewQueryCursor()
	query := buildQueries[q]
	captures := cursor.Captures(query, node, source)
	return captures, cursor
}

func (q query) Matches(source []byte, node *tree_sitter.Node) (tree_sitter.QueryMatches, *tree_sitter.QueryCursor) {
	cursor := tree_sitter.NewQueryCursor()
	query := buildQueries[q]
	matches := cursor.Matches(query, node, source)
	return matches, cursor
}

var buildQueries = make([]*tree_sitter.Query, len(queries))

func init() {
	lang := tree_sitter.NewLanguage(tree_sitter_gox.Language())
	for i, query := range queries {
		query, err := tree_sitter.NewQuery(lang, query)
		if err != nil {
			panic(err)
		}
		buildQueries[i] = query
	}
}

func goxIsImported(content []byte, root *tree_sitter.Node) (result bool) {
	QueryGoXImport.IterateCapture(content, root, "import", func(node *tree_sitter.Node) bool {
		name := node.ChildByFieldName("name")
		result = name == nil
		return result
	})
	return result
}

func NeedsGoxImport(content []byte, root *tree_sitter.Node) (gox bool) {
	if goxIsImported(content, root) {
		return false
	}
	if QueryElemKeyword.Exists(content, root) {
		return true
	}
	if QueryElementOrBlock.Exists(content, root) {
		return true
	}
	return false
}

func WhereToImport(content []byte, root *tree_sitter.Node) (common.Pos, bool) {
	m, c := QueryPackage.Matches(content, root)
	defer c.Close()
	p := m.Next()
	if p == nil {
		return common.NoPos(), false
	}
	pos := common.NewTSPos(p.Captures[0].Node.EndPosition())
	return pos, true
}
