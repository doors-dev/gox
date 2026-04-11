package attrs

import "testing"

func TestKindString(t *testing.T) {
	if ID.String() != "id" {
		t.Errorf("ID = %q", ID.String())
	}
	if CLASS.String() != "class" {
		t.Errorf("CLASS = %q", CLASS.String())
	}
}

func TestIsBool(t *testing.T) {
	cases := map[Kind]bool{
		ASYNC:    true,
		DISABLED: true,
		HIDDEN:   true,
		CHECKED:  true,
		ID:       false,
		CLASS:    false,
		HREF:     false,
	}
	for k, want := range cases {
		if got := k.IsBool(); got != want {
			t.Errorf("%s.IsBool() = %v, want %v", k.String(), got, want)
		}
	}
}

func TestIsEqual(t *testing.T) {
	if !CLASS.IsEqual("class") {
		t.Error("CLASS.IsEqual(class) = false")
	}
	if !CLASS.IsEqual("Class") {
		t.Error("CLASS.IsEqual(Class) = false")
	}
	if CLASS.IsEqual("id") {
		t.Error("CLASS.IsEqual(id) = true")
	}
}

func TestFindAttrForTag(t *testing.T) {
	got := Find("input", "")
	if len(got) == 0 {
		t.Fatal("Find(input) returned nothing")
	}
	// Should include CLASS (global ANY) and DISABLED (input-specific bool).
	seen := map[Kind]bool{}
	for _, k := range got {
		seen[k] = true
	}
	if !seen[CLASS] {
		t.Error("Find(input) missing CLASS")
	}
	if !seen[DISABLED] {
		t.Error("Find(input) missing DISABLED")
	}
}

func TestFindAttrPrefix(t *testing.T) {
	got := Find("input", "dis")
	if len(got) == 0 {
		t.Fatal("Find(input, dis) returned nothing")
	}
	for _, k := range got {
		s := k.String()
		if len(s) < 3 || s[:3] != "dis" {
			t.Errorf("Find returned %q without prefix", s)
		}
	}
}

func TestFindAttrUnknownTag(t *testing.T) {
	got := Find("unknown-tag", "")
	// Still returns global (ANY) attrs
	seen := map[Kind]bool{}
	for _, k := range got {
		seen[k] = true
	}
	if !seen[CLASS] {
		t.Error("Find(unknown) missing global CLASS")
	}
}

func TestFindAttrNoMatch(t *testing.T) {
	got := Find("input", "zzznotanattr")
	if len(got) != 0 {
		t.Errorf("Find(input, zzz) = %v", got)
	}
}

func TestFindAttrCaseInsensitive(t *testing.T) {
	got := Find("input", "DIS")
	if len(got) == 0 {
		t.Fatal("Find(input, DIS) returned nothing")
	}
}
