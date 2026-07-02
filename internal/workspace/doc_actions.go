package workspace

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/doors-dev/gox/internal/catalog/grammer"
	"github.com/doors-dev/gox/internal/common"
	"github.com/doors-dev/gox/internal/docpath"
	"github.com/doors-dev/gox/internal/rust"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

const (
	KindExtractVariable  = "refactor.extract.variable"
	KindExtractConstant  = "refactor.extract.constant"
	KindExtractElem      = "refactor.extract.elem"
	KindExtractToNewFile = "refactor.extract.toNewFile"
)

type ActionEdit struct {
	Range   common.Range
	NewText string
}

type ActionCreateFile struct {
	URI  string
	Text string
}

type CodeAction struct {
	Title      string
	Kind       string
	URI        string
	Edits      []ActionEdit
	CreateFile *ActionCreateFile
}

func (d Doc) ExtractActions(enc common.Encoding, ran common.Range) []CodeAction {
	if d.tree == nil {
		return nil
	}
	root := d.tree.RootNode()
	if root == nil {
		return nil
	}
	src := d.source.IntoRange(enc, ran)
	var actions []CodeAction
	actions = append(actions, d.extractExpressionActions(enc, src, root)...)
	if a, ok := d.extractComponentAction(enc, src, root); ok {
		actions = append(actions, a)
	}
	if a, ok := d.extractToNewFileAction(enc, src, root); ok {
		actions = append(actions, a)
	}
	return actions
}

type insertMode int

const (
	insertAsSnippet insertMode = iota
	insertAsStatement
	insertAsFileScopeDecl
)

func (d Doc) extractExpressionActions(enc common.Encoding, src common.Range, root *tree_sitter.Node) []CodeAction {
	expr := d.selectedExpression(src, root)
	if expr == nil || expr.HasError() {
		return nil
	}
	anchor, mode, ok := findInsertAnchor(expr)
	if !ok {
		return nil
	}
	if expr.StartByte() == anchor.StartByte() {
		return nil
	}
	source := d.source.Source()
	exprText := expr.Utf8Text(source)
	exprRange := d.source.FromRange(enc, common.NewTSRange(expr.Range()))
	anchorRow := int(anchor.StartPosition().Row)
	indent := lineIndent(d.source.MustLine(anchorRow))
	insertPos := d.source.FromPos(enc, common.NewPos(anchorRow, 0))

	actions := make([]CodeAction, 0, 2)

	varName := d.uniqueName("newVar")
	actions = append(actions, CodeAction{
		Title: "Extract variable",
		Kind:  KindExtractVariable,
		URI:   d.sourceFile.URI(),
		Edits: bindingEdits(insertPos, exprRange, indent, mode, false, varName, exprText),
	})

	if isConstExpr(expr) {
		constName := d.uniqueName("newConst")
		actions = append(actions, CodeAction{
			Title: "Extract constant",
			Kind:  KindExtractConstant,
			URI:   d.sourceFile.URI(),
			Edits: bindingEdits(insertPos, exprRange, indent, mode, true, constName, exprText),
		})
	}
	return actions
}

func bindingEdits(insertPos common.Pos, exprRange common.Range, indent string, mode insertMode, constant bool, name, exprText string) []ActionEdit {
	var binding string
	if constant {
		binding = "const " + name + " = " + exprText
	} else {
		binding = name + " := " + exprText
	}
	var newText string
	switch mode {
	case insertAsSnippet:
		newText = indent + "~~ " + binding + " ~~\n"
	case insertAsFileScopeDecl:
		if constant {
			newText = binding + "\n\n"
		} else {
			newText = "var " + name + " = " + exprText + "\n\n"
		}
	default:
		newText = indent + binding + "\n"
	}
	return []ActionEdit{
		{Range: exprRange, NewText: name},
		{Range: common.NewRange(insertPos, insertPos), NewText: newText},
	}
}

func (d Doc) selectedExpression(src common.Range, root *tree_sitter.Node) *tree_sitter.Node {
	node := root.NamedDescendantForPointRange(src.Beg().TS(), src.End().TS())
	if node == nil {
		return nil
	}
	expr := climbToExpr(node)
	if expr == nil {
		expr = firstExprChild(node)
	}
	if expr == nil {
		return nil
	}
	if src.IsCursor() {
		expr = outermostExpr(expr)
	}
	return expr
}

func climbToExpr(n *tree_sitter.Node) *tree_sitter.Node {
	for n != nil {
		if isExtractableExpr(n.Kind()) {
			return n
		}
		if isExprBoundary(n.Kind()) {
			return nil
		}
		n = n.Parent()
	}
	return nil
}

func isExprBoundary(kind string) bool {
	switch kind {
	case "source_file", "statement_list", "block", "expression_list",
		"argument_list", "parameter_list", "return_statement",
		"short_var_declaration", "var_spec", "const_spec",
		grammer.GOX_BLOCK, grammer.GOX_HEAD, grammer.GOX_CONTAINER_HEAD,
		grammer.GOX_VOID_HEAD, grammer.GOX_SELF_CLOSING_HEAD, grammer.GOX_RAW_HEAD,
		grammer.GOX_SCRIPT_HEAD, grammer.GOX_STYLE_HEAD, grammer.GOX_OPEN_HEAD,
		grammer.GOX_CLOSE_HEAD, grammer.GOX_TILDE, grammer.GOX_TILDE_IF,
		grammer.GOX_TILDE_FOR, grammer.GOX_TILDE_SNIPPET, grammer.GOX_TILDE_BLOCK,
		grammer.GOX_ATTR, grammer.GOX_SINGLE_ARG, grammer.GOX_MULTI_ARG:
		return true
	}
	return false
}

func findInsertAnchor(expr *tree_sitter.Node) (*tree_sitter.Node, insertMode, bool) {
	n := expr
	for {
		p := n.Parent()
		if p == nil {
			return nil, 0, false
		}
		switch p.Kind() {
		case grammer.GOX_BLOCK, grammer.GOX_HEAD, grammer.GOX_CONTAINER_HEAD:
			if isTemplateContent(n.Kind()) {
				return n, insertAsSnippet, true
			}
		case "statement_list", "block":
			if n.IsNamed() && n.Kind() != grammer.COMMENT {
				return n, insertAsStatement, true
			}
		case "source_file":
			return n, insertAsFileScopeDecl, true
		}
		n = p
	}
}

func (d Doc) extractComponentAction(enc common.Encoding, src common.Range, root *tree_sitter.Node) (CodeAction, bool) {
	markup, template := d.selectedMarkup(src, root)
	if markup == nil {
		return CodeAction{}, false
	}
	enclosing := enclosingTopLevelDecl(markup)
	if enclosing == nil || enclosing.HasError() {
		return CodeAction{}, false
	}
	source := d.source.Source()
	if referencesAny(markup, source, enclosingScopeNames(markup, source)) {
		return CodeAction{}, false
	}

	name := d.uniqueName("NewElem")
	markupText := string(d.source.Slice(common.NewTSRange(markup.Range())))

	replacement := name + "()"
	if template {
		replacement = "~(" + name + "())"
	}
	replaceRange := d.source.FromRange(enc, common.NewTSRange(markup.Range()))

	insertRow := int(enclosing.EndPosition().Row) + 1
	insertPos := d.source.FromPos(enc, common.NewPos(insertRow, 0))
	newElem := "\n" + formatElem(name, markupText)

	return CodeAction{
		Title: "Extract to elem",
		Kind:  KindExtractElem,
		URI:   d.sourceFile.URI(),
		Edits: []ActionEdit{
			{Range: replaceRange, NewText: replacement},
			{Range: common.NewRange(insertPos, insertPos), NewText: newElem},
		},
	}, true
}

func (d Doc) selectedMarkup(src common.Range, root *tree_sitter.Node) (*tree_sitter.Node, bool) {
	node := root.NamedDescendantForPointRange(src.Beg().TS(), src.End().TS())
	for n := node; n != nil; n = n.Parent() {
		if n.Kind() == grammer.GOX_ELEMENT {
			return n, false
		}
		if isMarkupHead(n.Kind()) {
			if p := n.Parent(); p != nil {
				switch p.Kind() {
				case grammer.GOX_BLOCK, grammer.GOX_HEAD, grammer.GOX_CONTAINER_HEAD:
					return n, true
				}
			}
		}
		if n.Kind() == "source_file" {
			break
		}
	}
	return nil, false
}

func isMarkupHead(kind string) bool {
	switch kind {
	case grammer.GOX_HEAD, grammer.GOX_CONTAINER_HEAD, grammer.GOX_VOID_HEAD,
		grammer.GOX_SELF_CLOSING_HEAD, grammer.GOX_RAW_HEAD,
		grammer.GOX_SCRIPT_HEAD, grammer.GOX_STYLE_HEAD:
		return true
	}
	return false
}

func enclosingTopLevelDecl(n *tree_sitter.Node) *tree_sitter.Node {
	for n != nil {
		p := n.Parent()
		if p != nil && p.Kind() == "source_file" {
			return n
		}
		n = p
	}
	return nil
}

func enclosingScopeNames(node *tree_sitter.Node, source []byte) map[string]bool {
	names := map[string]bool{}
	for n := node.Parent(); n != nil; n = n.Parent() {
		switch n.Kind() {
		case grammer.FUNC_DECLARATION, grammer.METHOD_DECLARATION,
			grammer.GOX_ELEM_FUNC_DEC, grammer.GOX_ELEM_METH_DEC,
			grammer.GOX_ELEM_FUNC_LIT, "func_literal":
			collectScopeNames(n, source, names)
		}
	}
	return names
}

func collectScopeNames(fn *tree_sitter.Node, source []byte, names map[string]bool) {
	if recv := fn.ChildByFieldName("receiver"); recv != nil {
		collectIdentifiers(recv, source, names)
	}
	if params := fn.ChildByFieldName("parameters"); params != nil {
		collectParamNames(params, source, names)
	}
	if result := fn.ChildByFieldName("result"); result != nil && result.Kind() == "parameter_list" {
		collectParamNames(result, source, names)
	}
	if body := fn.ChildByFieldName("body"); body != nil {
		collectLocalNames(body, source, names)
	}
}

func collectParamNames(params *tree_sitter.Node, source []byte, names map[string]bool) {
	for _, decl := range descendantKinds(params, "parameter_declaration", "variadic_parameter_declaration") {
		collectIdentifiers(&decl, source, names)
	}
}

func collectLocalNames(body *tree_sitter.Node, source []byte, names map[string]bool) {
	for _, n := range descendantKinds(body, "short_var_declaration", "range_clause") {
		if left := n.ChildByFieldName("left"); left != nil {
			collectIdentifiers(left, source, names)
		}
	}
	for _, n := range descendantKinds(body, "var_spec", "const_spec") {
		s := n
		cur := s.Walk()
		for _, c := range s.NamedChildren(cur) {
			if c.Kind() == "identifier" {
				names[c.Utf8Text(source)] = true
			}
		}
		cur.Close()
	}
}

func walkNodes(node *tree_sitter.Node, visit func(n *tree_sitter.Node) bool) {
	if !visit(node) {
		return
	}
	cur := node.Walk()
	defer cur.Close()
	for _, c := range node.Children(cur) {
		c := c
		walkNodes(&c, visit)
	}
}

func collectIdentifiers(node *tree_sitter.Node, source []byte, names map[string]bool) {
	walkNodes(node, func(n *tree_sitter.Node) bool {
		if n.Kind() == "identifier" {
			names[n.Utf8Text(source)] = true
			return false
		}
		return true
	})
}

func referencesAny(node *tree_sitter.Node, source []byte, names map[string]bool) bool {
	if len(names) == 0 {
		return false
	}
	found := false
	walkNodes(node, func(n *tree_sitter.Node) bool {
		if found {
			return false
		}
		if n.Kind() == "identifier" && names[n.Utf8Text(source)] {
			found = true
			return false
		}
		return true
	})
	return found
}

func descendantKinds(node *tree_sitter.Node, kinds ...string) []tree_sitter.Node {
	want := map[string]bool{}
	for _, k := range kinds {
		want[k] = true
	}
	var out []tree_sitter.Node
	walkNodes(node, func(n *tree_sitter.Node) bool {
		if want[n.Kind()] {
			out = append(out, *n)
		}
		return true
	})
	return out
}

func formatElem(name, markup string) string {
	fallback := "elem " + name + "() {\n" + indentBlock(markup, "\t") + "\n}\n"
	formatted, err := rust.Format([]byte("package p\n\nelem " + name + "() {\n" + markup + "\n}\n"))
	if err != nil || formatted == nil {
		return fallback
	}
	s := string(formatted)
	i := strings.Index(s, "\nelem ")
	if i < 0 {
		return fallback
	}
	return strings.TrimRight(s[i+1:], "\n") + "\n"
}

func indentBlock(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		lines[i] = prefix + ln
	}
	return strings.Join(lines, "\n")
}

func (d Doc) extractToNewFileAction(enc common.Encoding, src common.Range, root *tree_sitter.Node) (CodeAction, bool) {
	decl := topLevelDeclAt(root, src)
	if decl == nil {
		return CodeAction{}, false
	}
	source := d.source.Source()
	name := declName(decl, source)
	if name == "" {
		return CodeAction{}, false
	}
	pkg := packageName(root, source)
	if pkg == "" {
		return CodeAction{}, false
	}

	content := "package " + pkg + "\n\n" + decl.Utf8Text(source) + "\n"
	if formatted, err := rust.Format([]byte(content)); err == nil && formatted != nil {
		content = string(formatted)
	}

	uri := string(docpath.URIFromPath(d.newFilePath(name)))

	return CodeAction{
		Title:      "Extract '" + name + "' to a new file",
		Kind:       KindExtractToNewFile,
		URI:        d.sourceFile.URI(),
		Edits:      []ActionEdit{{Range: d.declDeletionRange(enc, decl), NewText: ""}},
		CreateFile: &ActionCreateFile{URI: uri, Text: content},
	}, true
}

var declKinds = map[string]bool{
	grammer.FUNC_DECLARATION:   true,
	grammer.METHOD_DECLARATION: true,
	grammer.GOX_ELEM_FUNC_DEC:  true,
	grammer.GOX_ELEM_METH_DEC:  true,
	grammer.TYPE_DECLARATION:   true,
	grammer.VAR_DECLARATION:    true,
	grammer.CONST_DECLARATION:  true,
}

func topLevelDeclAt(root *tree_sitter.Node, src common.Range) *tree_sitter.Node {
	cursor := root.Walk()
	defer cursor.Close()
	for _, child := range root.Children(cursor) {
		c := child
		if !declKinds[c.Kind()] {
			continue
		}
		declRange := common.NewTSRange(c.Range())
		if rangeContains(declHeaderRange(&c), src) || rangeContains(src, declRange) {
			return &c
		}
	}
	return nil
}

func declHeaderRange(decl *tree_sitter.Node) common.Range {
	start := common.NewTSPos(decl.StartPosition())
	end := common.NewTSPos(decl.EndPosition())
	if name := declNameNode(decl); name != nil {
		end = common.NewTSPos(name.EndPosition())
	}
	return common.NewRange(start, end)
}

func rangeContains(outer, inner common.Range) bool {
	return outer.Contains(inner.Beg(), true) && outer.Contains(inner.End(), true)
}

func declName(decl *tree_sitter.Node, source []byte) string {
	if name := declNameNode(decl); name != nil {
		return name.Utf8Text(source)
	}
	return ""
}

func declNameNode(decl *tree_sitter.Node) *tree_sitter.Node {
	switch decl.Kind() {
	case grammer.FUNC_DECLARATION, grammer.METHOD_DECLARATION,
		grammer.GOX_ELEM_FUNC_DEC, grammer.GOX_ELEM_METH_DEC:
		return decl.ChildByFieldName("name")
	case grammer.TYPE_DECLARATION:
		if spec := firstDescendantOfKind(decl, "type_spec", "type_alias"); spec != nil {
			return spec.ChildByFieldName("name")
		}
	case grammer.VAR_DECLARATION, grammer.CONST_DECLARATION:
		if spec := firstDescendantOfKind(decl, "var_spec", "const_spec"); spec != nil {
			if name := spec.ChildByFieldName("name"); name != nil {
				return name
			}
			return firstNamedChildOfKind(spec, "identifier")
		}
	}
	return nil
}

func firstDescendantOfKind(n *tree_sitter.Node, kinds ...string) *tree_sitter.Node {
	want := map[string]bool{}
	for _, k := range kinds {
		want[k] = true
	}
	var found *tree_sitter.Node
	walkNodes(n, func(node *tree_sitter.Node) bool {
		if found != nil {
			return false
		}
		if want[node.Kind()] {
			found = node
			return false
		}
		return true
	})
	return found
}

func packageName(root *tree_sitter.Node, source []byte) string {
	cursor := root.Walk()
	defer cursor.Close()
	for _, child := range root.Children(cursor) {
		if child.Kind() != grammer.PACKAGE {
			continue
		}
		if id := firstNamedChildOfKind(&child, "package_identifier"); id != nil {
			return id.Utf8Text(source)
		}
	}
	return ""
}

func (d Doc) declDeletionRange(enc common.Encoding, decl *tree_sitter.Node) common.Range {
	begRow := int(decl.StartPosition().Row)
	endRow := int(decl.EndPosition().Row)
	beg := common.NewPos(begRow, 0)
	lastLine := d.source.Cursor().Line()
	delEndRow := endRow + 1
	if delEndRow <= lastLine && isBlankLine(d.source.MustLine(delEndRow)) {
		delEndRow++
	}
	end := common.NewPos(delEndRow, 0)
	return d.source.FromRange(enc, common.NewRange(beg, end))
}

func (d Doc) newFilePath(name string) string {
	dir := filepath.Dir(d.sourceFile.Path())
	base := strings.ToLower(name)
	if base == "" {
		base = "extracted"
	}
	candidate := filepath.Join(dir, base+".gox")
	for i := 1; fileExists(candidate); i++ {
		candidate = filepath.Join(dir, base+strconv.Itoa(i)+".gox")
	}
	return candidate
}

func (d Doc) uniqueName(base string) string {
	source := d.source.String()
	name := base
	for i := 1; containsIdent(source, name); i++ {
		name = base + strconv.Itoa(i)
	}
	return name
}

func containsIdent(source, name string) bool {
	for i := 0; ; {
		j := strings.Index(source[i:], name)
		if j < 0 {
			return false
		}
		k := i + j
		before := k == 0 || !isIdentByte(source[k-1])
		after := k+len(name) >= len(source) || !isIdentByte(source[k+len(name)])
		if before && after {
			return true
		}
		i = k + 1
	}
}

func isIdentByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

func isExtractableExpr(kind string) bool {
	switch kind {
	case "binary_expression", "unary_expression", "call_expression",
		"selector_expression", "index_expression", "slice_expression",
		"parenthesized_expression", "composite_literal", "type_assertion_expression",
		"type_conversion_expression", "func_literal",
		"int_literal", "float_literal", "imaginary_literal", "rune_literal",
		"interpreted_string_literal", "raw_string_literal",
		"true", "false", "nil", "iota",
		grammer.GOX_ELEMENT, grammer.GOX_ELEM_FUNC_LIT:
		return true
	}
	return false
}

func outermostExpr(n *tree_sitter.Node) *tree_sitter.Node {
	for {
		p := n.Parent()
		if p == nil || !isExtractableExpr(p.Kind()) {
			return n
		}
		n = p
	}
}

func firstExprChild(n *tree_sitter.Node) *tree_sitter.Node {
	cursor := n.Walk()
	defer cursor.Close()
	for _, child := range n.NamedChildren(cursor) {
		if isExtractableExpr(child.Kind()) {
			c := child
			return &c
		}
	}
	return nil
}

func isTemplateContent(kind string) bool {
	switch kind {
	case grammer.GOX_ELEMENT, grammer.GOX_HEAD, grammer.GOX_CONTAINER_HEAD,
		grammer.GOX_VOID_HEAD, grammer.GOX_SELF_CLOSING_HEAD, grammer.GOX_RAW_HEAD,
		grammer.GOX_SCRIPT_HEAD, grammer.GOX_STYLE_HEAD, grammer.GOX_DOCTYPE,
		grammer.GOX_TILDE, grammer.GOX_TILDE_SNIPPET, grammer.GOX_TILDE_BLOCK,
		grammer.GOX_TILDE_PROXY, grammer.GOX_TILDE_COMMENT, grammer.GOX_COMMENT,
		grammer.GOX_PLAIN_TEXT:
		return true
	}
	return false
}

func isConstExpr(n *tree_sitter.Node) bool {
	switch n.Kind() {
	case "int_literal", "float_literal", "imaginary_literal", "rune_literal",
		"interpreted_string_literal", "raw_string_literal", "true", "false":
		return true
	case "parenthesized_expression":
		if inner := n.NamedChild(0); inner != nil {
			return isConstExpr(inner)
		}
	case "unary_expression":
		if op := n.ChildByFieldName("operand"); op != nil {
			return isConstExpr(op)
		}
	case "binary_expression":
		left := n.ChildByFieldName("left")
		right := n.ChildByFieldName("right")
		return left != nil && right != nil && isConstExpr(left) && isConstExpr(right)
	}
	return false
}

func firstNamedChildOfKind(n *tree_sitter.Node, kind string) *tree_sitter.Node {
	cursor := n.Walk()
	defer cursor.Close()
	for _, child := range n.NamedChildren(cursor) {
		if child.Kind() == kind {
			c := child
			return &c
		}
	}
	return nil
}

func lineIndent(line []byte) string {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return string(line[:i])
}

func isBlankLine(line []byte) bool {
	return strings.TrimSpace(string(line)) == ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
