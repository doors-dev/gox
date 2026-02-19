package workspace

import (
	"strings"

	"github.com/doors-dev/gox/internal/catalog/grammer"
	"github.com/doors-dev/gox/internal/common"
)

func (d Doc) Hover(enc common.Encoding, pos common.Pos) (message string, ran common.Range, ok bool) {
	sourcePos := d.source.IntoPos(enc, pos)
	n := d.tree.RootNode().DescendantForPointRange(sourcePos.TS(), sourcePos.TS())
	if n == nil || !strings.HasPrefix(n.Kind(), "gox_") || n.Kind() == grammer.GOX_SPACE_FILLER || n.Kind() == grammer.GOX_RAW_TEXT {
		return "", common.NoRange(), false
	}
	kind := n.Kind()[4:]
	kind = strings.ReplaceAll(kind, "_", " ")
	return kind, d.source.FromRange(enc, common.NewTSRange(n.Range())), true
}
