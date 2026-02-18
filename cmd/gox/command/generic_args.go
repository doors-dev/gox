package command

import "flag"

func parseGenericArgs(args []string, command string) (GenericArgs, error) {
	set := flag.NewFlagSet(command, flag.ContinueOnError)
	path := set.String("path", ".", "path to the file or directory to "+command)
	noIngore := set.Bool("no-ignore", false, "do not respect .gitignore")
	force := set.Bool("force", false, "gen overwrites existing files without checking")
	err := set.Parse(args)
	if err != nil {
		return GenericArgs{}, err
	}
	return GenericArgs{
		path:     *path,
		noIngore: *noIngore,
		force:    *force,
	}, err
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
