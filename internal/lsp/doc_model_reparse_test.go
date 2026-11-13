package lsp

import (
	"os"
	"strings"
	"testing"
)

// Inserting "(" and deleting it again through ranged didChange notifications,
// one change per notification or both in one, leaves no stale syntax error.
func TestDocModelRangedUndoLeavesNoStaleErrors(t *testing.T) {
	data, err := os.ReadFile("../workspace/testdata/reparse.txt")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	k := strings.Index(src, "func(ctx context.Context, r doors.RequestRawHook) bool {\n\t\t\t\tbody")
	if k < 0 {
		t.Fatal("anchor not found")
	}
	k += len("func(ctx context.Context, r")
	line := strings.Count(src[:k], "\n")
	col := k - strings.LastIndex(src[:k], "\n") - 1
	change := func(start, end int, text string) map[string]any {
		return map[string]any{
			"range": map[string]any{
				"start": map[string]any{"line": line, "character": start},
				"end":   map[string]any{"line": line, "character": end},
			},
			"text": text,
		}
	}
	for _, batched := range []bool{false, true} {
		env := newDocModelEnv(t)
		env.write(env.source, src)
		env.start(env.root)
		env.open(env.source, src)
		if batched {
			env.notify(didChange, map[string]any{
				"textDocument":   map[string]any{"uri": docModelURI(env.source), "version": 2},
				"contentChanges": []map[string]any{change(col, col, "("), change(col, col+1, "")},
			})
		} else {
			env.changeRange(env.source, 2, [2]int{line, col}, [2]int{line, col}, "(")
			env.changeRange(env.source, 3, [2]int{line, col}, [2]int{line, col + 1}, "")
		}
		env.settle()
		doc, _ := env.router.session.man().Doc(docModelURI(env.source))
		if doc == nil {
			t.Fatal("no document")
		}
		if report := doc.SyntaxErrorReport(); report != "" {
			t.Fatalf("batched=%v: stale syntax errors after edit and undo:\n%s", batched, report)
		}
	}
}
