package workspace

import (
	"os"
	"strings"
	"testing"

	"github.com/doors-dev/gox/internal/common"
)

func reparseAnchor(t *testing.T, src string) common.Pos {
	t.Helper()
	k := strings.Index(src, "func(ctx context.Context, r doors.RequestRawHook) bool {\n\t\t\t\tbody")
	if k < 0 {
		t.Fatal("anchor not found")
	}
	k += len("func(ctx context.Context, r")
	return common.NewPos(strings.Count(src[:k], "\n"), k-strings.LastIndex(src[:k], "\n")-1)
}

// Inserting "(" into a func literal's parameters and deleting it again leaves
// the text as it was. The incremental parse keeps a stale error there, and
// Reparse must leave a tree as clean as a fresh parse.
func TestReparseClearsStaleErrorsAfterUndo(t *testing.T) {
	data, err := os.ReadFile("testdata/reparse.txt")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	doc := makeDoc(t, src)
	if doc.tree.RootNode().HasError() {
		t.Fatal("the initial parse has errors")
	}
	at := reparseAnchor(t, src)
	if _, err := doc.SourcePatch(common.UTF8, common.NewRange(at, at), "("); err != nil {
		t.Fatal(err)
	}
	if _, err := doc.SourcePatch(common.UTF8, common.NewRange(at, common.NewPos(at.Line(), at.Column()+1)), ""); err != nil {
		t.Fatal(err)
	}
	if doc.source.String() != src {
		t.Fatal("the text was not restored")
	}
	if !doc.tree.RootNode().HasError() {
		t.Fatal("the incremental parse no longer keeps a stale error; the fixture does not test Reparse")
	}
	doc.Reparse()
	if doc.tree.RootNode().HasError() {
		t.Fatal("the tree has errors after Reparse")
	}
}
