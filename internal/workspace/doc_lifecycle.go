package workspace

import (
	"log/slog"
	"os"
)

func (d Doc) SourceOpen(version int32) {
	d.sourceVersion = version
}

func (d Doc) SourceClose() bool {
	d.sourceVersion = -1
	content, err := os.ReadFile(d.sourceFile.Path())
	if err != nil {
		return false
	}
	before := d.TargetContent()
	d.SourceUpdate(string(content))
	d.syncTarget()
	return d.TargetContent() != before
}

func (d Doc) Reload() bool {
	if d.SourceIsOpened() {
		return false
	}
	content, err := os.ReadFile(d.sourceFile.Path())
	if err != nil {
		return false
	}
	if upd, err := d.SourceUpdate(string(content)); err != nil || !upd {
		return false
	}
	d.syncTarget()
	return true
}

func (d Doc) syncTarget() {
	needsUpdate, err := d.CheckTarget()
	if err == nil && needsUpdate {
		err = d.TargetWrite()
	}
	if err != nil {
		slog.Error("Generated target sync failed", "source", d.SourceFile().Path(), "target", d.TargetFile().Path(), "error", err)
	}
}

func (d Doc) SourceIsOpened() bool {
	return d.sourceVersion != -1
}

func (d Doc) TargetOpen(version int32) {
	d.targetVersion = version
}

func (d Doc) TargetClose() {
	d.targetVersion = -1
}

func (d Doc) TargetIsOpened() bool {
	return d.targetVersion != -1
}

func (d Doc) opened() bool {
	return d.SourceIsOpened() || d.TargetIsOpened()
}
