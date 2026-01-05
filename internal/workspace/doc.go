package workspace

import (
	"errors"
	"log/slog"
	"slices"

	"github.com/doors-dev/gox/internal/assembler"
	"github.com/doors-dev/gox/internal/text"
	"github.com/doors-dev/gox/internal/translator"
	tree_sitter_gox "github.com/doors-dev/tree-sitter-gox/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type Doc = *doc

func NewDoc(file File) Doc {
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
		sourceFile:    file,
		parser:        parser,
		sourceVersion: -1,
		targetVersion: -1,
	}
}

type doc struct {
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

func (d Doc) PrintTarget() {
	d.target.Print()
}

func (d Doc) Err() error {
	if d == nil {
		return errors.New("File is not a part of the workspace")
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
	return slices.Equal(d.target.Source(), d.draft.Source())
}

func (d Doc) Load() error {
	d.source = text.NewText()
	return d.source.Load(d.sourceFile.Path())
}

func (d Doc) Parse() error {
	d.tree = d.parser.Parse(d.source.Source(), nil)
	if d.tree.RootNode().HasError() {
		return errors.New("Parser detected ERROR nodes, please ensure that syntax is correct.")
	}
	return nil
}


func (d Doc) Init() {
	if !d.sourceFile.Exists() {
		d.targetRemove()
		d.err = errors.New("Source file not exists")
		return
	}
	err := d.Load()
	if err != nil {
		d.err = errors.New("Source reading error: " + err.Error())
		return
	}
	if d.tree != nil {
		d.tree.Close()
	}
	d.Parse()
	d.Assemble()
	d.resetDraft()
	if d.TargetIsUpToDate() {
		return 
	}
	err = d.TargetWrite()
	if err != nil {
		d.err = errors.New("Target writing error: " + err.Error())
		return
	}
}

func (d Doc) Save() error {
	if d.TargetIsUpToDate() {
		return nil
	}
	err := d.TargetWrite()
	if err != nil && !d.TargetIsOpened() {
		d.resetDraft()
	}
	return err
}

func (d Doc) TargetIsUpToDate() bool {
	return d.TargetFile().IsEqual(d.target.Source()) 
}

func (d Doc) TargetWrite() error {
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
	if !d.TargetFile().Exists() {
		return
	}
	if err := d.TargetFile().Remove(); err != nil {
		slog.Error("Target file [" + d.TargetFile().Path() + "] remove error: " + err.Error())
	}
}
