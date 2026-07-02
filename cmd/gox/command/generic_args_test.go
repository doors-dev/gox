package command

import (
	"slices"
	"testing"
)

func TestParseGenericArgsDefaultsToCurrentDir(t *testing.T) {
	args, err := parseGenericArgs(nil, "format")
	if err != nil {
		t.Fatalf("parseGenericArgs() error = %v", err)
	}
	if got := args.Paths(); !slices.Equal(got, []string{"."}) {
		t.Fatalf("Paths() = %v, want [.]", got)
	}
}

func TestParseGenericArgsAcceptsPositionalPath(t *testing.T) {
	args, err := parseGenericArgs([]string{"-no-ignore", "./internal"}, "format")
	if err != nil {
		t.Fatalf("parseGenericArgs() error = %v", err)
	}
	if got := args.Paths(); !slices.Equal(got, []string{"./internal"}) {
		t.Fatalf("Paths() = %v, want [./internal]", got)
	}
	if !args.NoIgnore() {
		t.Fatal("NoIgnore() = false, want true")
	}
}

func TestParseGenericArgsAcceptsMultiplePaths(t *testing.T) {
	args, err := parseGenericArgs([]string{"-force", "./pkg", "a.gox", "./cmd/..."}, "generate")
	if err != nil {
		t.Fatalf("parseGenericArgs() error = %v", err)
	}
	if got := args.Paths(); !slices.Equal(got, []string{"./pkg", "a.gox", "./cmd/..."}) {
		t.Fatalf("Paths() = %v, want three operands", got)
	}
	if !args.Force() {
		t.Fatal("Force() = false, want true")
	}
}

func TestParseGenericArgsCheckAndNoGo(t *testing.T) {
	args, err := parseGenericArgs([]string{"-check", "-no-go", "."}, "format")
	if err != nil {
		t.Fatalf("parseGenericArgs() error = %v", err)
	}
	if !args.Check() {
		t.Fatal("Check() = false, want true")
	}
	if args.FormatGo() {
		t.Fatal("FormatGo() = true, want false (-no-go)")
	}
}

func TestParseGenericArgsRejectsPathFlag(t *testing.T) {
	_, err := parseGenericArgs([]string{"-path", "./pkg"}, "format")
	if err == nil {
		t.Fatal("parseGenericArgs() error = nil, want error")
	}
}
