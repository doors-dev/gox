package workspace

import (
	"log/slog"
	"math"

	"github.com/doors-dev/gox/internal/common"
	"github.com/doors-dev/gox/internal/formatter"
)

type Formatted struct {
	Text  string
	Range common.Range
}

func (d Doc) Format(enc common.Encoding) (Formatted, error) {
	output, err := formatter.Format(d.source.Source())
	if err != nil {
		return Formatted{}, err
	}
	ran := common.NewRange(common.NewPos(0, 0), common.NewPos(math.MaxInt32, 0))
	return Formatted{
		Range: ran,
		Text:  string(output),
	}, nil
}

func (d Doc) SourceUpdate(content string) (bool, error) {
	slog.Info("source update", "content", content)
	slog.Info("source update", "source", d.source.String())

	edit, upd, err := d.source.Update(content)
	if err != nil {
		slog.Error("patch error: " + err.Error())
		return false, err
	}
	slog.Info("source update", "eq", !upd)
	if !upd {
		return false, nil
	}
	oldTree := d.tree
	defer oldTree.Close()
	oldTree.Edit(&edit)
	d.tree = d.parser.Parse(d.source.Source(), nil)
	d.Assemble()
	return true, nil
}

func (d Doc) SourcePatch(enc common.Encoding, ran common.Range, content string) (bool, error) {
	r := d.source.IntoRange(enc, ran)
	edit, upd, err := d.source.Patch(r, content)
	if err != nil {
		slog.Error("patch error: " + err.Error())
		return false, err
	}
	if !upd {
		return false, nil
	}
	oldTree := d.tree
	defer oldTree.Close()
	oldTree.Edit(&edit)
	d.tree = d.parser.Parse(d.source.Source(), oldTree)
	return true, nil
}

func (d Doc) TargetDraftPatch(enc common.Encoding, ran common.Range, content string) error {
	r := d.draft.IntoRange(enc, ran)
	_, _, err := d.draft.Patch(r, content)
	if err != nil {
		slog.Error("patch error: " + err.Error())
	}
	return err
}

func (d Doc) TargetDraftUpdate(content string) error {
	_, _, err := d.draft.Update(content)
	if err != nil {
		slog.Error("patch error: " + err.Error())
	}
	return err
}
