package workspace

import (
	"log/slog"

	"github.com/doors-dev/gox/internal/common"
)

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
