package utils

import (
	"errors"
	"fmt"
	"io"
	"unsafe"
)

var (
	tagOpen    = []byte("<")
	tagClose   = []byte("</")
	tagEnd     = []byte(">")
	attrAssign = []byte{'='}
	attrQuot   = []byte{'"'}
	htmlQuot   = []byte("&#34;")
	htmlApos   = []byte("&#39;")
	htmlAmp    = []byte("&amp;")
	htmlLt     = []byte("&lt;")
	htmlGt     = []byte("&gt;")
	htmlNull   = []byte("\uFFFD")
)

func NewEscapedWriter(w io.Writer) io.Writer {
	return &escapedWriter{
		w: w,
	}
}

type escapedWriter struct {
	w io.Writer
}

func (a *escapedWriter) Write(b []byte) (int, error) {
	last := 0
	sum := 0
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
		n, err := a.w.Write(b[last:i])
		sum += n
		if err != nil {
			return sum, err
		}
		_, err = a.w.Write(html)
		if err != nil {
			return sum, err
		}
		last = i + 1
		sum += 1
	}
	n, err := a.w.Write(b[last:])
	sum += n
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
	_, err := io.WriteString(w, text)
	return err
}

func WriteEscapedText(w io.Writer, text string) error {
	ew := &escapedWriter{
		w: w,
	}
	_, err := io.WriteString(ew, text)
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

type Output interface {
	Output(w io.Writer) error
}

func WriteAttrName(w io.Writer, name string) error {
	return writeName(w, name)
}

func WriteAttrValue(w io.Writer, value any) error {
	if value == nil {
		return errors.New("Nil attribute values cannot be written.")
	}
	if _, ok := value.(bool); ok {
		return errors.New("Boolean attribute values cannot be written.")
	}
	return writeAttrValue(w, value)
}

func writeAttrValue(w io.Writer, value any) error {
	ew := NewEscapedWriter(w)
	if o, ok := value.(Output); ok {
		if err := o.Output(ew); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprint(ew, value); err != nil {
			return err
		}
	}
	return nil
}

func WriteAttr(w io.Writer, name string, value any) error {
	if value == nil {
		return nil
	}
	if boolValue, ok := value.(bool); ok {
		if !boolValue {
			return nil
		}
		return writeName(w, name)
	}
	if err := writeName(w, name); err != nil {
		return err
	}
	if _, err := w.Write(attrAssign); err != nil {
		return err
	}
	if _, err := w.Write(attrQuot); err != nil {
		return err
	}
	if err := writeAttrValue(w, value); err != nil {
		return err
	}
	if _, err := w.Write(attrQuot); err != nil {
		return err
	}
	return nil
}
