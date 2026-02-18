package text

import (
	"bufio"
	"bytes"
	"os"
)

func (t Text) Print() {
	os.Stdout.Write(t.Source())
}

func (t Text) Save(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(t.Source())
	return err
}

func (t Text) ensureLineOffsets() {
	if len(t.lineOffsets) != 0 {
		return
	}
	t.lineOffsets = append(t.lineOffsets, newOffset(0, 0))
}

func (t Text) Load(path string) error {
	defer t.ensureLineOffsets()
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(nil, maxTokenSize)
	buf := bytes.Buffer{}
	t.lineOffsets = nil
	for scanner.Scan() {
		beg := buf.Len()
		buf.Write(scanner.Bytes())
		end := buf.Len()
		buf.WriteByte('\n')
		offset := newOffset(beg, end)
		t.lineOffsets = append(t.lineOffsets, offset)
	}
	if scanner.Err() != nil {
		return scanner.Err()
	}
	if buf.Len() == 0 {
		t.source = nil
		return nil
	}
	buf.Truncate(buf.Len() - 1)
	for i := len(t.lineOffsets) - 1; i >= 0 && t.lineOffsets[i].Len() == 0; i-- {
		t.lineOffsets = t.lineOffsets[:i]
		buf.Truncate(buf.Len() - 1)
	}
	t.source = buf.Bytes()
	return nil
}
