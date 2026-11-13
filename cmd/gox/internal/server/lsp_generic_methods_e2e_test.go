package server_test

import (
	"debug/buildinfo"
	"fmt"
	"go/version"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func requireGenericMethods(t *testing.T) {
	t.Helper()
	if version.Compare(runtime.Version(), "go1.27") < 0 {
		t.Skipf("generic methods need go1.27, test toolchain is %s", runtime.Version())
	}
	goplsPath, err := exec.LookPath("gopls")
	if err != nil {
		t.Skip("gopls is not installed")
	}
	info, err := buildinfo.ReadFile(goplsPath)
	if err != nil {
		t.Skipf("read gopls build info: %v", err)
	}
	if version.Compare(info.GoVersion, "go1.27") < 0 {
		t.Skipf("generic methods need gopls built with go1.27, %s was built with %s", goplsPath, info.GoVersion)
	}
}

const genericMethodsSource = `package pkg

import "github.com/doors-dev/gox"

type Store[K comparable] struct{}

func (s Store[K]) /*@get_def*/Get[T any](key K, def T) T {
	return def
}

elem (s Store[K]) /*@item_def*/Item[T ~int | ~string](key K, item T) {
	<li>~(s./*@get_use*/Get[T](key, item))</li>
}

func Use() gox.Elem {
	return Store[string]{}./*@item_use*/Item[int]("k", /*@bad_arg*/"x")
}
`

func TestLSPGenericMethods(t *testing.T) {
	requireGenericMethods(t)
	root := filepath.Join(t.TempDir(), "main")
	writeFile(t, filepath.Join(root, "go.mod"), fmt.Sprintf(`module example.com/app

go 1.27

require github.com/doors-dev/gox v0.0.0

replace github.com/doors-dev/gox => %s
`, findRepoRoot(t)))
	source := writeMarkedFile(t, filepath.Join(root, "pkg", "store.gox"), genericMethodsSource)
	h := newSingleFolderHarness(t, root)
	start := h.checkpoint()
	uri := h.openFile(source.Path, "gox")

	h.waitForHover(uri, source.Marker("get_use"), "func (Store[K]).Get(key K, def T) T // func[T any](key K, def T) T")
	h.waitForHover(uri, source.Marker("item_use"), "func (Store[string]).Item(key string, item int) gox.Elem // func[T ~int | ~string](key string, item T) gox.Elem")

	for _, tc := range []struct{ use, def string }{
		{"get_use", "get_def"},
		{"item_use", "item_def"},
	} {
		locations := h.definitionLocations(uri, source.Marker(tc.use))
		if len(locations) != 1 || locations[0].URI != uri || locations[0].Range.Start != source.Marker(tc.def) {
			t.Fatalf("definition at %s must point at %+v in %s, got %+v", tc.use, source.Marker(tc.def), uri, locations)
		}
	}

	diagnostics := waitForDiagnostics(t, h, uri)
	if len(diagnostics.Diagnostics) != 1 {
		t.Fatalf("want exactly the bad argument diagnostic, got %+v", diagnostics.Diagnostics)
	}
	d := diagnostics.Diagnostics[0]
	if !strings.Contains(d.Message, `"x"`) || !strings.Contains(d.Message, "int") {
		t.Fatalf("unexpected diagnostic: %+v", d)
	}
	assertRangeContainsPosition(t, d.Range, source.Marker("bad_arg"))
	h.assertNoErrorMessagesAfter(start, "generic methods")
}
