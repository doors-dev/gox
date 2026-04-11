package workspace

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/doors-dev/gox/internal/docpath"
	"golang.org/x/mod/semver"
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
	docuri, err := docpath.ParseDocumentURI(uri)
	if err != nil {
		slog.Error("Document URI parsing error", "error", err.Error())
		return File{}, false
	}
	return NewFile(docuri.Path())
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
	return string(docpath.URIFromPath(f.Path()))
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

func (f File) ValidateTarget(b []byte) (needsUpdate bool, err error) {
	if f.kind != KindTarget {
		panic("not a target file")
	}
	fl, err := os.Open(f.Path())
	if err != nil {
		return true, nil
	}
	defer fl.Close()
	br := bytes.NewReader(b)
	fileVersion, err := readFirstLine(fl)
	if err != nil {
		return true, nil
	}
	bytesVersion, err := readFirstLine(br)
	if err != nil {
		panic(err)
	}
	if semver.IsValid(fileVersion) && semver.IsValid(bytesVersion) && semver.Compare(fileVersion, bytesVersion) == 1 {
		err = errors.New("This generated file was created by a newer GoX version. Update GoX, or run 'gox gen -force' to overwrite it.")
	}
	const chunk = 32 * 1024
	bufFile := make([]byte, chunk)
	bufBytes := make([]byte, chunk)
	for {
		nf, ef := io.ReadFull(fl, bufFile)
		nb, eb := io.ReadFull(br, bufBytes)
		if nf != nb || !bytes.Equal(bufFile[:nf], bufBytes[:nb]) {
			needsUpdate = true
			return
		}
		if ef != nil && ef != io.EOF && ef != io.ErrUnexpectedEOF {
			needsUpdate = true
			return
		}
		if eb != nil && eb != io.EOF && eb != io.ErrUnexpectedEOF {
			needsUpdate = true
			return
		}
		fileEnded := ef == io.EOF || ef == io.ErrUnexpectedEOF
		bytesEnded := eb == io.EOF || eb == io.ErrUnexpectedEOF
		if fileEnded != bytesEnded {
			needsUpdate = true
			return
		}
		if fileEnded && bytesEnded {
			return
		}
	}
}

func (f File) remove() error {
	return os.Remove(f.Path())
}

func readFirstLine(r io.Reader) (string, error) {
	var b [1]byte
	var firstLine []byte
	for {
		_, err := io.ReadFull(r, b[:])
		if err == io.EOF {
			return "", errors.New("Unexpected EOF.")
		}
		if err != nil {
			return "", err
		}
		if b[0] != '\n' {
			firstLine = append(firstLine, b[0])
			continue
		}
		break
	}
	i := bytes.LastIndex(firstLine, []byte("GoX "))
	if i < 0 {
		return "", errors.New("Invalid GoX header.")
	}
	version := firstLine[i+4:]
	return string(version), nil
}
