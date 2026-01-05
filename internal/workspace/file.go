package workspace

import (
	"bytes"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	// "github.com/zeebo/blake3"
)

type FileKind string

const (
	KindSource  FileKind = ".gox"
	KindTarget  FileKind = ".x.go"
	KindUnknown FileKind = ""
)

func NewFile(path string) (File, bool) {
	if KindSource.Belongs(path) {
		return File{
			name: strings.TrimSuffix(path, KindSource.String()),
			kind: KindSource,
		}, true
	}
	if KindTarget.Belongs(path) {
		return File{
			name: strings.TrimSuffix(path, KindTarget.String()),
			kind: KindTarget,
		}, true
	}
	return File{}, false
}

func NewFileFromURI(uri string) (File, bool) {
	path, err := url.Parse(uri)
	if err != nil {
		return File{}, false
	}
	return NewFile(path.Path)
}

type File struct {
	name string
	kind FileKind
}

func (f File) Dir() string {
	return filepath.Dir(f.name)
}

func (f File) IsValid() bool {
	return f.name != "" && f.kind != ""
}

func (f File) Remove() error {
	return os.Remove(f.Path())
}

func (f FileKind) Belongs(path string) bool {
	return strings.HasSuffix(path, f.String())
}

func (f FileKind) Revert() FileKind {
	switch f {
	case KindSource:
		return KindTarget
	case KindTarget:
		return KindSource
	default:
		panic("unknown file type")
	}
}

func (f FileKind) String() string {
	switch f {
	case KindSource:
		return string(f)
	case KindTarget:
		return string(f)
	default:
		panic("unknown file type")
	}
}

func (f File) URI() string {
	u := url.URL{
		Scheme: "file",
		Path:   f.Path(),
	}
	return u.String()
}

func (f File) Reverse() File {
	return File{
		name: f.name,
		kind: f.kind.Revert(),
	}
}

func (f File) Name() string {
	return f.name
}

func (f File) Kind() FileKind {
	return f.kind
}

func (f File) Path() string {
	return f.name + f.kind.String()
}

func (f File) Exists() bool {
	info, err := os.Stat(f.Path())
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func (f File) IsEqual(b []byte) bool {
	fl, err := os.Open(f.Path())
	if err != nil {
		return false
	}
	defer fl.Close()
	if info, err := fl.Stat(); err == nil && info.Size() != int64(len(b)) {
		return false
	}
	br := bytes.NewReader(b)
	const chunk = 32 * 1024
	bufFile := make([]byte, chunk)
	bufBytes := make([]byte, chunk)
	for {
		nf, ef := fl.Read(bufFile)
		nb, eb := br.Read(bufBytes)
		if nf != nb || !bytes.Equal(bufFile[:nf], bufBytes[:nb]) {
			return false
		}
		if ef == io.EOF && eb == io.EOF {
			return true
		}
		if ef != nil && ef != io.EOF {
			return false
		}
		if eb != nil && eb != io.EOF {
			return false
		}
		if ef == io.EOF || eb == io.EOF {
			return false
		}
	}
}

func (f File) remove() error {
	return os.Remove(f.Path())
}
