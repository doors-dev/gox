package processor

import "golang.org/x/term"

import (
	"os"
)

var useColor = term.IsTerminal(int(os.Stderr.Fd())) && os.Getenv("NO_COLOR") == ""

func green(s string) string {
	if !useColor { return s }
	return "\x1b[32m" + s + "\x1b[0m"
}
func red(s string) string {
	if !useColor { return s }
	return "\x1b[31m" + s + "\x1b[0m"
}



