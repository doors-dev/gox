package assembler

import "runtime/debug"

var version = "v0.0.0"

func Version() string {
	return version
}

func init() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	version = info.Main.Version
}
