package text

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/doors-dev/gox/internal/common"
)

// normalizeRef is what Load and a full Patch store for raw: split at \n, drop
// one trailing \r per line, drop trailing empty lines, join with \n.
func normalizeRef(raw []byte) []byte {
	segs := bytes.Split(raw, []byte("\n"))
	for i, s := range segs {
		segs[i] = bytes.TrimSuffix(s, []byte("\r"))
	}
	for len(segs) > 0 && len(segs[len(segs)-1]) == 0 {
		segs = segs[:len(segs)-1]
	}
	return bytes.Join(segs, []byte("\n"))
}

var normalizedCases = []struct{ raw, norm string }{
	{"", ""},
	{"\n", ""},
	{"\n\n\n", ""},
	{"\r", ""},
	{"\r\n", ""},
	{"\n\r", ""},
	{"\r\n\r\n", ""},
	{"\r\r", "\r"},
	{"\r\r\n", "\r"},
	{"a", "a"},
	{"a\r", "a"},
	{"a\r\n", "a"},
	{"a\n", "a"},
	{"a\r\r", "a\r"},
	{"a\r\r\n", "a\r"},
	{"a\r\r\r\n", "a\r\r"},
	{"a\rb", "a\rb"},
	{"a\rb\r", "a\rb"},
	{"a\r\r\nb", "a\r\nb"},
	{"a\n\r\r\n\n", "a\n\r"},
	{"\r\na", "\na"},
	{"\r\ra", "\r\ra"},
	{"a\n \n", "a\n "},
	{"x\r\ny\r\n", "x\ny"},
	{"x\r\ny\r\n\r\n\n", "x\ny"},
	{"x\ny\n\n\n", "x\ny"},
	{"x\r\n\r\ny", "x\n\ny"},
	{"x\n\r\ny\r", "x\n\ny"},
	{"x\r\n\ty\r\n}\r\n", "x\n\ty\n}"},
}

var unequalCases = []struct{ raw, norm string }{
	{"a\r", "a\r"},
	{"a\n", "a\n"},
	{"a\nb\n", "a\n"},
	{"", "\n"},
	{"\n", "\n"},
	{"\r", "\r"},
	{"a\r\r", "a"},
	{"a\r\r\n", "a"},
	{"a\r\r\nb", "a\nb"},
	{"a\r\rb", "a\rb"},
	{"ax\rb", "axb"},
	{"a\rb", "a\nb"},
	{"a\rb", "ab"},
	{"\r\na", "a"},
	{"a\n \n", "a"},
	{"a\n\nb", "a\nb"},
	{"a\nb", "a\n\nb"},
	{"a", ""},
	{"", "a"},
	{"a", "b"},
	{"ab", "a"},
	{"a", "ab"},
	{"a\r\n", "a\r"},
	{"a\r\nb", "a\r\nb"},
	{"\r\r", ""},
	{"\r\r", "\r\r"},
	{"a\r\n\r", "a\r"},
	{"a\n\r\r", "a\n"},
	{"x\r\ny\r\n", "x\ny\n"},
	{"x\r\ny\r\n", "x\r\ny"},
}

func checkEqualNormalized(t *testing.T, raw []byte, ref []byte, normalized []byte) {
	want := bytes.Equal(ref, normalized)
	if got := EqualNormalized(raw, normalized); got != want {
		t.Fatalf("EqualNormalized(%q, %q) = %v, want %v", raw, normalized, got, want)
	}
}

func checkOffsets(t *testing.T, tx Text) {
	t.Helper()
	var want []offset
	beg := 0
	for _, line := range bytes.Split(tx.source, []byte("\n")) {
		want = append(want, newOffset(beg, beg+len(line)))
		beg += len(line) + 1
	}
	if !slices.Equal(tx.lineOffsets, want) {
		t.Fatalf("source %q has offsets %v, want %v", tx.source, tx.lineOffsets, want)
	}
}

func loadString(t *testing.T, path string, raw []byte) Text {
	t.Helper()
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}
	tx := NewText()
	if err := tx.Load(path); err != nil {
		t.Fatal(err)
	}
	return tx
}

func updateString(t *testing.T, tx Text, raw []byte) bool {
	t.Helper()
	_, changed, err := tx.Update(string(raw))
	if err != nil {
		t.Fatalf("Update(%q): %v", raw, err)
	}
	return changed
}

func eachString(alphabet string, maxLen int, f func([]byte)) {
	buf := make([]byte, maxLen)
	var rec func(n int)
	rec = func(n int) {
		f(buf[:n])
		if n == maxLen {
			return
		}
		for i := range len(alphabet) {
			buf[n] = alphabet[i]
			rec(n + 1)
		}
	}
	rec(0)
}

func allStrings(alphabet string, maxLen int) [][]byte {
	var out [][]byte
	eachString(alphabet, maxLen, func(s []byte) {
		out = append(out, bytes.Clone(s))
	})
	return out
}

func TestEqualNormalizedCases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.gox")
	for _, c := range normalizedCases {
		raw, norm := []byte(c.raw), []byte(c.norm)
		if got := normalizeRef(raw); !bytes.Equal(got, norm) {
			t.Fatalf("normalizeRef(%q) = %q, want %q", raw, got, norm)
		}
		if got := loadString(t, path, raw).source; !bytes.Equal(got, norm) {
			t.Fatalf("Load(%q) stored %q, want %q", raw, got, norm)
		}
		tx := NewText()
		updateString(t, tx, raw)
		if !bytes.Equal(tx.source, norm) {
			t.Fatalf("Update(%q) stored %q, want %q", raw, tx.source, norm)
		}
		if updateString(t, tx, raw) {
			t.Fatalf("second Update(%q) changed the text", raw)
		}
		if !EqualNormalized(raw, norm) || !EqualNormalized(append(bytes.Clone(raw), '\n'), norm) {
			t.Fatalf("EqualNormalized(%q, %q) = false", raw, norm)
		}
		if EqualNormalized(raw, nil) != (len(norm) == 0) {
			t.Fatalf("EqualNormalized(%q, nil) != %v", raw, len(norm) == 0)
		}
		if EqualNormalized(raw, append(bytes.Clone(norm), '\n')) {
			t.Fatalf("EqualNormalized(%q, %q) = true", raw, c.norm+"\n")
		}
	}
	for _, c := range unequalCases {
		raw, norm := []byte(c.raw), []byte(c.norm)
		if bytes.Equal(normalizeRef(raw), norm) {
			t.Fatalf("normalizeRef(%q) = %q", raw, norm)
		}
		if EqualNormalized(raw, norm) {
			t.Fatalf("EqualNormalized(%q, %q) = true", raw, norm)
		}
	}
	for _, c := range [][2][]byte{{nil, nil}, {nil, {}}, {{}, nil}, {[]byte("\r\n\n"), nil}} {
		if !EqualNormalized(c[0], c[1]) {
			t.Fatalf("EqualNormalized(%q, %q) = false", c[0], c[1])
		}
	}
}

func TestEqualNormalizedExhaustive(t *testing.T) {
	const edits = "ab\r\n"
	var scratch []byte
	eachString("a\r\n", 10, func(raw []byte) {
		want := normalizeRef(raw)
		try := func(c []byte) {
			checkEqualNormalized(t, raw, want, c)
		}
		try(want)
		try(raw)
		try(bytes.TrimRight(raw, "\n"))
		try(normalizeRef(want))
		for i := range len(want) + 1 {
			try(want[:i])
			if i < len(want) {
				scratch = append(append(scratch[:0], want[:i]...), want[i+1:]...)
				try(scratch)
			}
			for j := range len(edits) {
				scratch = append(append(append(scratch[:0], want[:i]...), edits[j]), want[i:]...)
				try(scratch)
				if i < len(want) {
					scratch = append(scratch[:0], want...)
					scratch[i] = edits[j]
					try(scratch)
				}
			}
		}
	})
	for _, c := range []struct {
		alphabet      string
		rawLen, nrLen int
	}{
		{"a\r\n", 7, 6},
		{"ab\r\n", 5, 4},
	} {
		norms := allStrings(c.alphabet, c.nrLen)
		eachString(c.alphabet, c.rawLen, func(raw []byte) {
			want := normalizeRef(raw)
			for _, n := range norms {
				checkEqualNormalized(t, raw, want, n)
			}
		})
	}
}

func TestUpdateSkipsOnlyUnchangedSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.gox")
	var states []Text
	eachString("a\r\n", 4, func(s []byte) {
		tx := NewText()
		updateString(t, tx, s)
		states = append(states, tx, loadString(t, path, s))
		if last := len(tx.lineOffsets) - 1; last > 0 && tx.lineOffsets[last].Len() > 0 {
			cut := tx.Clone()
			ran := common.NewRange(common.NewPos(last, 0), common.NewPos(last, tx.lineOffsets[last].Len()))
			if _, _, err := cut.Patch(ran, ""); err != nil {
				t.Fatal(err)
			}
			states = append(states, cut)
		}
	})
	raws := allStrings("a\r\n", 6)
	for _, s := range states {
		checkOffsets(t, s)
		before := bytes.Clone(s.source)
		for _, raw := range raws {
			tx := s.Clone()
			want := normalizeRef(raw)
			if changed := updateString(t, tx, raw); changed == bytes.Equal(want, before) {
				t.Fatalf("Update(%q) on %q: changed = %v", raw, before, changed)
			}
			if !bytes.Equal(tx.source, want) {
				t.Fatalf("Update(%q) on %q stored %q, want %q", raw, before, tx.source, want)
			}
			checkOffsets(t, tx)
			if updateString(t, tx, raw) {
				t.Fatalf("second Update(%q) on %q changed the text", raw, before)
			}
		}
	}
}

func TestEqualNormalizedNoAllocs(t *testing.T) {
	raw := bytes.Repeat([]byte("\t<div class=\"x\">~(name)</div>\r\n\t<p>\ra</p>\n"), 1<<10)
	norm := normalizeRef(raw)
	allocs := testing.AllocsPerRun(10, func() {
		if !EqualNormalized(raw, norm) || EqualNormalized(raw, norm[:len(norm)-1]) {
			t.Fatal("wrong result")
		}
	})
	if allocs != 0 {
		t.Fatalf("EqualNormalized allocates %v times", allocs)
	}
}

func FuzzEqualNormalized(f *testing.F) {
	for _, c := range normalizedCases {
		f.Add([]byte(c.raw), []byte(c.norm))
	}
	for _, c := range unequalCases {
		f.Add([]byte(c.raw), []byte(c.norm))
	}
	f.Fuzz(func(t *testing.T, raw []byte, normalized []byte) {
		checkEqualNormalized(t, raw, normalizeRef(raw), normalized)
	})
}

func FuzzUpdateNormalizes(f *testing.F) {
	for _, c := range normalizedCases {
		for op := range uint8(7) {
			f.Add([]byte(c.raw), op, uint16(len(c.norm)/2), byte('\r'))
		}
	}
	f.Fuzz(func(t *testing.T, raw []byte, op uint8, pos uint16, b byte) {
		want := normalizeRef(raw)
		tx := NewText()
		updateString(t, tx, raw)
		if !bytes.Equal(tx.source, want) {
			t.Fatalf("Update(%q) stored %q, want %q", raw, tx.source, want)
		}
		checkOffsets(t, tx)
		if updateString(t, tx, raw) {
			t.Fatalf("second Update(%q) changed the text", raw)
		}
		c := bytes.Clone(want)
		p := int(pos) % (len(c) + 1)
		switch op % 7 {
		case 1:
			if p < len(c) {
				c = append(c[:p], c[p+1:]...)
			}
		case 2:
			if p < len(c) {
				c[p] = b
			}
		case 3:
			c = slices.Insert(c, p, b)
		case 4:
			c = c[:p]
		case 5:
			c = append(c, b)
		case 6:
			c = append(c, '\n')
		}
		checkEqualNormalized(t, raw, want, c)
	})
}
