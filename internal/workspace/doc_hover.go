package workspace

import (
	"log/slog"
	"strings"

	"github.com/doors-dev/gox/internal/common"
)

func (d Doc) Hover(enc common.Encoding, pos common.Pos) (message string, ran common.Range, ok bool) {
	sourcePos := d.source.IntoPos(enc, pos)
	n := d.tree.RootNode().DescendantForPointRange(sourcePos.TS(), sourcePos.TS())
	if n == nil || !strings.HasPrefix(n.Kind(), "gox") {
		return "", common.NoRange(), false
	}
	slog.Info("hovering on ", "kind", n.Kind(), " at ", common.NewTSRange(n.Range()))
	return n.Kind(), d.source.FromRange(enc, common.NewTSRange(n.Range())), true
}
