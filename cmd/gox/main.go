package main

import (
	"fmt"
	"os"

	"github.com/doors-dev/gox/cmd/gox/command"
)

func main() {
	cmdErr, runErr := command.Execute(starter{})
	if cmdErr != nil {
		fmt.Fprintln(os.Stderr, cmdErr.Error())
		os.Exit(1)
		return
	}
	if runErr != nil {
		fmt.Fprintln(os.Stderr, runErr.Error())
		os.Exit(exitCode(runErr))
		return
	}
}

func exitCode(err error) int {
	if ec, ok := err.(interface{ ExitCode() int }); ok {
		return ec.ExitCode()
	}
	return 1
}
