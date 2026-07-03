package processor

import (
	"os"

	"golang.org/x/term"
)

var stdoutColor = term.IsTerminal(int(os.Stdout.Fd())) && os.Getenv("NO_COLOR") == ""
var stderrColor = term.IsTerminal(int(os.Stderr.Fd())) && os.Getenv("NO_COLOR") == ""

func paint(enabled bool, code string, s string) string {
	if !enabled {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func greenOut(s string) string {
	return paint(stdoutColor, "32", s)
}

func redOut(s string) string {
	return paint(stdoutColor, "31", s)
}

func yellowOut(s string) string {
	return paint(stdoutColor, "33", s)
}

func redErr(s string) string {
	return paint(stderrColor, "31", s)
}

func yellowErr(s string) string {
	return paint(stderrColor, "33", s)
}
