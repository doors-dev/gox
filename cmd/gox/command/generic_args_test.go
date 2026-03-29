package command

import "testing"

func TestParseGenericArgsDefaultsToCurrentDir(t *testing.T) {
	args, err := parseGenericArgs(nil, "format")
	if err != nil {
		t.Fatalf("parseGenericArgs() error = %v", err)
	}
	if args.Path() != "." {
		t.Fatalf("Path() = %q, want %q", args.Path(), ".")
	}
}

func TestParseGenericArgsAcceptsPositionalPath(t *testing.T) {
	args, err := parseGenericArgs([]string{"-no-ignore", "./internal"}, "format")
	if err != nil {
		t.Fatalf("parseGenericArgs() error = %v", err)
	}
	if args.Path() != "./internal" {
		t.Fatalf("Path() = %q, want %q", args.Path(), "./internal")
	}
	if !args.NoIgnore() {
		t.Fatal("NoIgnore() = false, want true")
	}
}

func TestParseGenericArgsAcceptsFlagsWithPositionalPath(t *testing.T) {
	args, err := parseGenericArgs([]string{"-force", "./pkg"}, "generate")
	if err != nil {
		t.Fatalf("parseGenericArgs() error = %v", err)
	}
	if args.Path() != "./pkg" {
		t.Fatalf("Path() = %q, want %q", args.Path(), "./pkg")
	}
	if !args.Force() {
		t.Fatal("Force() = false, want true")
	}
}

func TestParseGenericArgsRejectsMultiplePaths(t *testing.T) {
	_, err := parseGenericArgs([]string{"./pkg", "./internal"}, "format")
	if err == nil {
		t.Fatal("parseGenericArgs() error = nil, want error")
	}
}

func TestParseGenericArgsRejectsPathFlag(t *testing.T) {
	_, err := parseGenericArgs([]string{"-path", "./pkg"}, "format")
	if err == nil {
		t.Fatal("parseGenericArgs() error = nil, want error")
	}
}
