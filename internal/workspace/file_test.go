package workspace

import (
	"path/filepath"
	"testing"
)

func TestValidateTargetComparesMajorMinor(t *testing.T) {
	const body = "package demo\n\nfunc A() {}\n"
	const changed = "package demo\n\nfunc B() {}\n"
	for _, tc := range []struct {
		name  string
		file  string
		ours  string
		newer bool
	}{
		{"same version", "v0.4.0", "v0.4.0", false},
		{"newer patch", "v0.4.3", "v0.4.0", false},
		{"older patch", "v0.4.0", "v0.4.3", false},
		{"higher minor", "v0.5.0", "v0.4.9", true},
		{"higher major", "v1.0.0", "v0.4.0", true},
		{"higher major, lower minor", "v1.0.0", "v0.9.0", true},
		{"lower minor", "v0.3.9", "v0.4.0", false},
		{"lower major", "v0.9.9", "v1.0.0", false},
		{"pre-release of the same minor", "v0.4.0-rc.1", "v0.4.0", false},
		{"ours a pre-release, newer patch", "v0.4.1", "v0.4.0-rc.1", false},
		{"pre-release of a higher minor", "v0.5.0-rc.1", "v0.4.3", true},
		{"ours a pseudo-version, newer patch", "v0.4.5", "v0.4.1-0.20261112130044-51e421d92b09", false},
		{"ours a dirty pseudo-version, higher minor", "v0.5.0", "v0.4.1-0.20261112130044-51e421d92b09+dirty", true},
		{"pseudo-version of a higher minor", "v0.5.0-0.20261112130044-51e421d92b09", "v0.4.3", true},
		{"pseudo-version of the same minor", "v0.4.1-0.20261112130044-51e421d92b09", "v0.4.0", false},
		{"devel file", "(devel)", "v0.4.0", false},
		{"devel ours", "v9.9.9", "(devel)", false},
		{"invalid file version", "9.9.9", "v0.4.0", false},
		{"invalid ours", "v9.9.9", "0.4.0", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "a.x.go")
			file, _ := NewFile(path)
			ours := []byte("// Managed by GoX " + tc.ours + "\n" + body)
			for _, disk := range []struct {
				body  string
				stale bool
			}{{body, false}, {changed, true}} {
				writeTestFile(t, path, "// Managed by GoX "+tc.file+"\n"+disk.body)
				stale, err := file.ValidateTarget(ours)
				if stale != disk.stale || (err != nil) != tc.newer {
					t.Fatalf("file %s, ours %s, stale body %v: ValidateTarget() = (%v, %v)", tc.file, tc.ours, disk.stale, stale, err)
				}
			}
		})
	}
}

func TestValidateTargetReadsCRLFHeader(t *testing.T) {
	const body = "package demo\r\n\r\nfunc A() {}\r\n"
	ours := []byte("// Managed by GoX v0.4.0\npackage demo\n\nfunc A() {}\n")
	for _, tc := range []struct {
		name   string
		header string
		newer  bool
	}{{"higher minor", "v0.5.0", true}, {"higher major", "v1.0.0", true}, {"older", "v0.3.3", false}} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "a.x.go")
			file, _ := NewFile(path)
			writeTestFile(t, path, "// Managed by GoX "+tc.header+"\r\n"+body)
			stale, err := file.ValidateTarget(ours)
			if tc.newer {
				if err == nil || err.Error() != "This generated file was created by a much newer GoX version. Update GoX, or run 'gox gen -force' to overwrite it." {
					t.Fatalf("ValidateTarget() = (%v, %v), want the newer GoX version error", stale, err)
				}
				return
			}
			if err != nil || !stale {
				t.Fatalf("ValidateTarget() = (%v, %v), want (true, nil)", stale, err)
			}
		})
	}
}
