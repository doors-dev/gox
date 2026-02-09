package workspace

import (
	"strings"

	"github.com/doors-dev/gox/internal/catalog/grammer"
	"github.com/doors-dev/gox/internal/common"
)

func (d Doc) Hover(enc common.Encoding, pos common.Pos) (message string, ran common.Range, ok bool) {
	sourcePos := d.source.IntoPos(enc, pos)
	n := d.tree.RootNode().DescendantForPointRange(sourcePos.TS(), sourcePos.TS())
	if n == nil || !strings.HasPrefix(n.Kind(), "gox") || n.Kind() == grammer.GOX_SPACE_FILLER {
		return "", common.NoRange(), false
	}
	return n.Kind(), d.source.FromRange(enc, common.NewTSRange(n.Range())), true
}
