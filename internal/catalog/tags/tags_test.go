package tags

import "testing"

func TestKindString(t *testing.T) {
	if BR.String() != "br" {
		t.Errorf("BR = %q", BR.String())
	}
	if DIV.String() != "div" {
		t.Errorf("DIV = %q", DIV.String())
	}
	if ANY.String() != "*" {
		t.Errorf("ANY = %q", ANY.String())
	}
	if RAW.String() != ":" {
		t.Errorf("RAW = %q", RAW.String())
	}
}

func TestIsVoid(t *testing.T) {
	cases := map[Kind]bool{
		BR:    true,
		IMG:   true,
		INPUT: true,
		LINK:  true,
		DIV:   false,
		SPAN:  false,
		BODY:  false,
		RAW:   false,
	}
	for k, want := range cases {
		if got := k.IsVoid(); got != want {
			t.Errorf("%s.IsVoid() = %v, want %v", k, got, want)
		}
	}
}

func TestEquals(t *testing.T) {
	if !DIV.Equals("div") {
		t.Error("DIV.Equals(div) = false")
	}
	if !DIV.Equals("DIV") {
		t.Error("DIV.Equals(DIV) = false")
	}
	if DIV.Equals("span") {
		t.Error("DIV.Equals(span) = true")
	}
}

func TestFindTagEmpty(t *testing.T) {
	if got := FindTag(""); got != nil {
		t.Errorf("FindTag(\"\") = %v", got)
	}
}

func TestFindTagPrefix(t *testing.T) {
	got := FindTag("d")
	if len(got) == 0 {
		t.Fatal("FindTag(d) returned no matches")
	}
	// Should include div, dl, dt, etc.
	seen := map[Kind]bool{}
	for _, k := range got {
		seen[k] = true
	}
	if !seen[DIV] {
		t.Error("FindTag(d) missing DIV")
	}
}

func TestFindTagExactInsertedFirst(t *testing.T) {
	got := FindTag("div")
	if len(got) == 0 || got[0] != DIV {
		t.Fatalf("FindTag(div)[0] = %v, want DIV at front", got)
	}
}

func TestFindTagCaseInsensitive(t *testing.T) {
	got := FindTag("DIV")
	if len(got) == 0 || got[0] != DIV {
		t.Fatalf("FindTag(DIV)[0] = %v", got)
	}
}

func TestFindTagNoMatch(t *testing.T) {
	got := FindTag("zzznotaag")
	if len(got) != 0 {
		t.Errorf("FindTag(zzz) = %v", got)
	}
}
