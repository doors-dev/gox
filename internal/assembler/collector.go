package assembler

import tree_sitter "github.com/tree-sitter/go-tree-sitter"

type collector interface {
	portal(beg tree_sitter.Point, end tree_sitter.Point)
	indentRef(ref tree_sitter.Point)
	indentFake()
	indentBeg()
	indentEnd()
	cr()
	append(parts ...part)
}

type partKind int

const (
	nonePart   partKind = -1
	portalPart partKind = iota
	stringPart
	rawPart
	textPart
)

type part struct {
	kind  partKind
	value any
}

func (p *part) Portal() (*tree_sitter.Node, bool) {
	if p.kind != portalPart {
		return nil, false
	}
	return p.value.(*tree_sitter.Node), true
}

func (p *part) Str() (*tree_sitter.Node, bool) {
	if p.kind != stringPart {
		return nil, false
	}
	return p.value.(*tree_sitter.Node), true
}

func (p *part) Raw() (string, bool) {
	if p.kind != rawPart {
		return "", false
	}
	return p.value.(string), true
}

func (p *part) Text() (*tree_sitter.Node, bool) {
	if p.kind != textPart {
		return nil, false
	}
	return p.value.(*tree_sitter.Node), true
}

func t(node *tree_sitter.Node) part {
	return part{
		kind:  textPart,
		value: node,
	}
}

func s(node *tree_sitter.Node) part {
	switch node.Kind() {
	case RAW_STRING_LITERAL, INTERPETED_STRING_LITERAL:
		return part{
			kind:  portalPart,
			value: node,
		}
	default:
		return part{
			kind:  stringPart,
			value: node,
		}
	}
}

func r(str string) part {
	return part{
		kind:  rawPart,
		value: str,
	}
}

func p(node *tree_sitter.Node) part {
	if node == nil {
		return part{
			kind:  nonePart,
			value: nil,
		}
	}
	return part{
		kind:  portalPart,
		value: node,
	}
}
