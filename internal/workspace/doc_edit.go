package workspace

import (
	"errors"
	"log/slog"
	"math"

	"github.com/doors-dev/gox/internal/common"
	"github.com/doors-dev/gox/internal/rust"
)

type Formatted struct {
	Text  string
	Range common.Range
}

func (d Doc) Format(enc common.Encoding) (Formatted, error) {
	output, err := rust.Format(d.source.Source())
	if err != nil {
		return Formatted{}, errors.New("Could not format this file. Make sure the source is valid.")
	}
	if output == nil {
		return Formatted{Range: common.NoRange()}, nil
	}
	ran := common.NewRange(common.NewPos(0, 0), common.NewPos(math.MaxInt32, 0))
	return Formatted{
		Range: ran,
		Text:  string(output),
	}, nil
}

func (d Doc) SourceUpdate(content string) (bool, error) {
	edit, upd, err := d.source.Update(content)
	if err != nil {
		d.err = errors.New("Could not apply the edit.")
		slog.Error("Document source update failed", "source", d.SourceFile().Path(), "target", d.TargetFile().Path(), "error", d.err)
		return false, err
	}
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
		d.err = errors.New("Could not apply the edit.")
		slog.Error("Document source patch failed", "source", d.SourceFile().Path(), "target", d.TargetFile().Path(), "error", d.err)
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
