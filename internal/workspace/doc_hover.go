package workspace

import (
	"log/slog"

	"github.com/doors-dev/gox/internal/common"
)

type hoverResult int

const (
	nok = iota
	portal
	token
)

type HoverResult struct {
	result    hoverResult
	targetPos common.Pos
	message   string
	ran       common.Range
}

func (r HoverResult) Ok() bool {
	return r.result != nok
}

func (r HoverResult) Portal() (common.Pos, bool) {
	if r.result != portal {
		return common.NoPos(), false
	}
	return r.targetPos, true
}

func (r HoverResult) Token() (string, common.Range, bool) {
	if r.result != token {
		return "", common.NoRange(), false
	}
	return r.message, r.ran, true
}

func (d Doc) Hover(enc common.Encoding, pos common.Pos) HoverResult {
	sourcePos := d.source.IntoPos(enc, pos)
	targetPos, ok := d.TargetPos(enc, pos, Strict)
	if ok {
		return HoverResult{
			result:    portal,
			targetPos: targetPos,
		}
	}
	n := d.tree.RootNode().DescendantForPointRange(sourcePos.TS(), sourcePos.TS())
	if n == nil {
		return HoverResult{
			result: nok,
		}
	}
	slog.Info("hovering on ", "kind", n.Kind(), " at ", common.NewTSRange(n.Range()))
	return HoverResult{
		result:  token,
		message: n.Kind(),
		ran:     d.source.FromRange(enc, common.NewTSRange(n.Range())),
	}
}
