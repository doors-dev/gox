package workspace

import (
	"errors"
	"log/slog"
	"os"
	"slices"

	"github.com/doors-dev/gox/internal/assembler"
	"github.com/doors-dev/gox/internal/text"
	"github.com/doors-dev/gox/internal/translator"
	tree_sitter_gox "github.com/doors-dev/tree-sitter-gox/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type Doc = *doc

type ows interface {
	lock()
	unlock()
}

type dummyWs struct{}

func (d dummyWs) lock() {}

func (d dummyWs) unlock() {}

func NewDoc(file File, ws ows) Doc {
	if file.Kind() != KindSource {
		file = file.Reverse()
	}
	lang := tree_sitter.NewLanguage(tree_sitter_gox.Language())
	parser := tree_sitter.NewParser()
	err := parser.SetLanguage(lang)
	if err != nil {
		panic(err)
	}
	return &doc{
		ws:            ws,
		sourceFile:    file,
		parser:        parser,
		sourceVersion: -1,
		targetVersion: -1,
	}
}

type doc struct {
	ws            ows
	parser        *tree_sitter.Parser
	sourceFile    File
	tree          *tree_sitter.Tree
	source        text.Text
	target        text.Text
	draft         text.Text
	translator    translator.Translator
	sourceVersion int32
	targetVersion int32
	err           error
}

func (d Doc) Lock() error {
	if d == nil {
		return errors.New("file not exists")
	}
	d.ws.lock()
	if d.Err() != nil {
		d.ws.unlock()
		return d.Err()
	}
	return nil
}

func (d Doc) Unlock() {
	d.ws.unlock()
}

func (d Doc) PrintTarget() {
	d.target.Print()
}

func (d Doc) Err() error {
	if d == nil {
		return errors.New("file not exists")
	}
	return d.err
}

func (d Doc) Assemble() {
	d.target, d.translator = assembler.Assemble(d.source, d.tree.RootNode())
}

func (d Doc) resetDraft() {
	d.draft = d.target.Clone()
}

func (d Doc) SubmitTargetDraft() bool {
	/*
		slog.Info("submit draft", "target", d.target.Source())
		slog.Info("submit draft", "draft", d.draft.Source())
	*/
	return slices.Equal(d.target.Source(), d.draft.Source())
}

func (d Doc) Init() {
	if !d.sourceFile.Exists() {
		d.targetRemove()
		d.err = errors.New("file not exists")
		return
	}
	d.source = text.NewText()
	err := d.source.Load(d.sourceFile.Path())
	if err != nil {
		d.err = err
		return
	}
	if d.tree != nil {
		d.tree.Close()
	}
	d.tree = d.parser.Parse(d.source.Source(), nil)
	d.Assemble()
	d.resetDraft()
	err = d.targetWrite()
	if err != nil {
		d.err = err
		return
	}
}

func (d Doc) Save() error {
	err := d.targetWrite()
	if err != nil && !d.TargetIsOpened() {
		d.resetDraft()
	}
	return err
}

func (d Doc) targetWrite() error {
	hash, ok := d.TargetFile().Hash()
	if ok && hash == d.target.Hash() {
		return nil
	}
	return d.target.Save(d.TargetFile().Path())
}

func (d Doc) Name() string {
	return d.sourceFile.Name()
}

func (d Doc) SourceFile() File {
	return d.sourceFile
}

func (d Doc) TargetContent() string {
	return d.target.String()
}

func (d Doc) TargetFile() File {
	return d.sourceFile.Reverse()
}

func (d Doc) Delete() {
	d.targetRemove()
}

func (d Doc) targetRemove() {
	slog.Info("removing target file: " + d.TargetFile().Path())
	if !d.TargetFile().Exists() {
		return
	}
	err := os.Remove(d.TargetFile().Path())
	if err != nil {
		slog.Error("generated filer removal error: " + err.Error())
	}
}
