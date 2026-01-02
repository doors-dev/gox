package command

import "flag"

func parseGenericArgs(args []string, command string) (GenericArgs, error) {
	set := flag.NewFlagSet(command, flag.ContinueOnError)
	path := set.String("path", ".", "path to the file or directory to "+command)
	noIngore := set.Bool("no-ignore", false, "do not respect .gitignore")
	err := set.Parse(args)
	if err != nil {
		return GenericArgs{}, err
	}
	return GenericArgs{
		path: *path,
		noIngore: *noIngore,
	}, err
}

type GenericArgs struct {
	path string
	noIngore bool
}


func (a GenericArgs) NoIgnore() bool {
	return a.noIngore
}

func (a GenericArgs) Path() string {
	return a.path
}
