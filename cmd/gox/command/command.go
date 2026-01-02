package command

import (
	"errors"
	"os"
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
  gen		Generates .x.go files from .gox files, removes orphaned .x.go files.
  fmt		Formats .go and .gox files`

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
			return errors.New("format arguments parse error: " + err.Error()), nil
		}
		return nil, s.Format(args)
	case "gen", "generate":
		args, err := parseGenericArgs(args, "generate")
		if err != nil {
			return errors.New("generate arguments parse error: " + err.Error()), nil
		}
		return nil, s.Generate(args)
	case "srv", "serve":
		args, err := parseServeArgs(args)
		if err != nil {
			return errors.New("serve arguments parse error: " + err.Error()), nil
		}
		return nil, s.Serve(args)
	default:
		return errors.New("unknown command: " + command + "\n\n" + help), nil
	}
}
