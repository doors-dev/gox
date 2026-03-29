package command

import (
	"flag"
	"fmt"
)

func parseGenericArgs(args []string, command string) (GenericArgs, error) {
	set := flag.NewFlagSet(command, flag.ContinueOnError)
	noIngore := set.Bool("no-ignore", false, "do not respect .gitignore")
	force := set.Bool("force", false, "gen overwrites existing files without checking")
	if err := set.Parse(args); err != nil {
		return GenericArgs{}, err
	}
	pathValue, err := parsePathArg(set)
	if err != nil {
		return GenericArgs{}, err
	}
	return GenericArgs{
		path:     pathValue,
		noIngore: *noIngore,
		force:    *force,
	}, nil
}

func parsePathArg(set *flag.FlagSet) (string, error) {
	args := set.Args()
	if len(args) == 0 {
		return ".", nil
	}
	if len(args) > 1 {
		return "", fmt.Errorf("expected at most 1 path argument, got %d", len(args))
	}
	return args[0], nil
}

type GenericArgs struct {
	path     string
	noIngore bool
	force    bool
}

func (a GenericArgs) Force() bool {
	return a.force
}

func (a GenericArgs) NoIgnore() bool {
	return a.noIngore
}

func (a GenericArgs) Path() string {
	return a.path
}
