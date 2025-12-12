package utils

import (
	"encoding/json"
	"io"
	"unicode"
	"unsafe"
)

var (
	tagOpen    = []byte("<")
	tagClose   = []byte("</")
	tagEnd     = []byte(">")
	attrSpace  = []byte{' '}
	attrAssign = []byte{'='}
	attrQuot   = []byte{'"'}
	htmlQuot   = []byte("&#34;")
	htmlApos   = []byte("&#39;")
	htmlAmp    = []byte("&amp;")
	htmlLt     = []byte("&lt;")
	htmlGt     = []byte("&gt;")
	htmlNull   = []byte("\uFFFD")
)

type EscapedWriter struct {
	W           io.Writer
	TrimNewline bool
}

func (a *EscapedWriter) Write(b []byte) (int, error) {
	last := 0
	sum := 0
	adj := 0
	if a.TrimNewline && len(b) > 0 && b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
		adj = 1
	}
	for i, c := range b {
		var html []byte
		switch c {
		case '\000':
			html = htmlNull
		case '"':
			html = htmlQuot
		case '\'':
			html = htmlApos
		case '&':
			html = htmlAmp
		case '<':
			html = htmlLt
		case '>':
			html = htmlGt
		default:
			continue
		}
		n, err := a.W.Write(b[last:i])
		sum += n
		if err != nil {
			return sum, err
		}
		_, err = a.W.Write(html)
		if err != nil {
			return sum, err
		}
		last = i + 1
		sum += 1
	}
	n, err := a.W.Write(b[last:])
	sum += n
	if err != nil {
		sum += adj
	}
	return sum, err
}

func writeName(w io.Writer, name string) error {
	b := unsafe.Slice(unsafe.StringData(name), len(name))
	last := 0
	for i, c := range b {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == ':' {
			continue
		}
		_, err := w.Write(b[last:i])
		if err != nil {
			return err
		}
		last = i + 1
	}
	_, err := w.Write(b[last:])
	return err
}

func WriteRawText(w io.Writer, text string) error {
	b := unsafe.Slice(unsafe.StringData(text), len(text))
	_, err := w.Write(b)
	return err
}

func WriteEscapedText(w io.Writer, text string) error {
	ew := &EscapedWriter{
		W: w,
	}
	b := unsafe.Slice(unsafe.StringData(text), len(text))
	_, err := ew.Write(b)
	return err
}

func WriteTagOpenBeg(w io.Writer, name string) error {
	_, err := w.Write(tagOpen)
	if err != nil {
		return err
	}
	return writeName(w, name)
}

func WriteTagOpenEnd(w io.Writer) error {
	_, err := w.Write(tagEnd)
	return err
}

func WriteTagClose(w io.Writer, name string) error {
	_, err := w.Write(tagClose)
	if err != nil {
		return err
	}
	err = writeName(w, name)
	if err != nil {
		return err
	}
	_, err = w.Write(tagEnd)
	return err
}

func writeAttrValue(w io.Writer, value []string) error {
	_, err := w.Write(attrQuot)
	if err != nil {
		return err
	}
	ew := &EscapedWriter{
		W: w,
	}
	prevSpace := true
	for _, v := range value {
		if len(v) == 0 {
			continue
		}
		if !prevSpace {
			spaced := unicode.IsSpace(rune(v[0]))
			if !spaced {
				_, err = w.Write(attrSpace)
				if err != nil {
					return err
				}
			}
		}
		prevSpace = unicode.IsSpace(rune(v[len(v)-1]))
		b := unsafe.Slice(unsafe.StringData(v), len(v))
		_, err = ew.Write(b)
		if err != nil {
			return err
		}
	}
	_, err = w.Write(attrQuot)
	return err
}

func writeAttrJsonValue(w io.Writer, value any) error {
	_, err := w.Write(attrQuot)
	if err != nil {
		return err
	}
	ew := &EscapedWriter{
		W:           w,
		TrimNewline: true,
	}
	enc := json.NewEncoder(ew)
	enc.SetEscapeHTML(false)
	err = enc.Encode(value)
	if err != nil {
		return err
	}
	_, err = w.Write(attrQuot)
	return err
}

func WriteBoolAttr(w io.Writer, name string) error {
	return writeName(w, name)
}

func WriteAttr(w io.Writer, name string, value []string) error {
	err := writeName(w, name)
	if err != nil {
		return err
	}
	_, err = w.Write(attrAssign)
	if err != nil {
		return err
	}
	return writeAttrValue(w, value)
}

func WriteAttrJson(w io.Writer, name string, value any) error {
	err := writeName(w, name)
	if err != nil {
		return err
	}
	_, err = w.Write(attrAssign)
	if err != nil {
		return err
	}
	return writeAttrJsonValue(w, value)
}
