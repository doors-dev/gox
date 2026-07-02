package command

import (
	"errors"
	"os"

	"github.com/doors-dev/gox/internal/assembler"
)

type Starter interface {
	Default() error
	Serve(args ServeArgs) error
	Format(args GenericArgs) error
	Generate(args GenericArgs) error
}

const header = `GoX - syntax extension to Go.`
const help = `Commands:
  srv		Starts the GoX Language Server (default)
  gen		Generates .x.go files from .gox files and removes orphaned .x.go files
  fmt		Formats .go and .gox files
  ver		Prints the version

gen / fmt accept any number of file, directory, or recursion operands ("./...", "pkg/..."); default ".".
  -check	report files that need work and exit non-zero; write nothing
  -no-ignore	do not respect .gitignore
  -force	gen: overwrite generated files without checking
  -no-go	fmt: format .gox files only, leave plain .go untouched`

func Execute(s Starter) (error, error) {
	args := os.Args[1:]
	command := ""
	if len(args) != 0 {
		command = args[0]
		args = args[1:]
	}
	switch command {
	case "":
		return nil, s.Default()
	case "help", "--help", "-h", "-help", "--h":
		println(header)
		println()
		println(help)
		return nil, nil
	case "fmt", "format":
		args, err := parseGenericArgs(args, "format")
		if err != nil {
			return errors.New("Could not parse format arguments: " + err.Error()), nil
		}
		return nil, s.Format(args)
	case "gen", "generate":
		args, err := parseGenericArgs(args, "generate")
		if err != nil {
			return errors.New("Could not parse generate arguments: " + err.Error()), nil
		}
		return nil, s.Generate(args)
	case "srv", "serve":
		args, err := parseServeArgs(args)
		if err != nil {
			return errors.New("Could not parse serve arguments: " + err.Error()), nil
		}
		return nil, s.Serve(args)
	case "ver", "version":
		println(assembler.Version())
		return nil, nil
	default:
		return errors.New("Unknown command: " + command + "\n\n" + help), nil
	}
}
