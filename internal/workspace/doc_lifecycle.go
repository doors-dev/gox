package workspace

func (d Doc) SourceOpen(version int32) {
	d.sourceVersion = version
}

func (d Doc) SourceClose() {
	d.sourceVersion = -1
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
