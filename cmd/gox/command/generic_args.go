package command

import (
	"flag"
)

func parseGenericArgs(args []string, command string) (GenericArgs, error) {
	set := flag.NewFlagSet(command, flag.ContinueOnError)
	noIngore := set.Bool("no-ignore", false, "do not respect .gitignore")
	force := set.Bool("force", false, "gen overwrites existing files without checking")
	check := set.Bool("check", false, "report files that need work and exit non-zero; write nothing")
	noGo := set.Bool("no-go", false, "fmt: format .gox files only, leave plain .go files untouched")
	if err := set.Parse(args); err != nil {
		return GenericArgs{}, err
	}
	paths := set.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}
	return GenericArgs{
		paths:    paths,
		noIngore: *noIngore,
		force:    *force,
		check:    *check,
		noGo:     *noGo,
	}, nil
}

type GenericArgs struct {
	paths    []string
	noIngore bool
	force    bool
	check    bool
	noGo     bool
}

func (a GenericArgs) Force() bool {
	return a.force
}

func (a GenericArgs) NoIgnore() bool {
	return a.noIngore
}

func (a GenericArgs) Check() bool {
	return a.check
}

func (a GenericArgs) FormatGo() bool {
	return !a.noGo
}

func (a GenericArgs) Paths() []string {
	return a.paths
}
