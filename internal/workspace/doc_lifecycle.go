package workspace

import "os"

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
	return d.TargetContent() != before
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
