package workspace

import (
	"errors"
	"log/slog"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/doors-dev/gox/internal/assembler"
	"github.com/doors-dev/gox/internal/common"
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
	translator    translator.Translator
	sourceVersion int32
	targetVersion int32
	err           error
	diagnostics   *ast.Node
}

func (d Doc) StoreDiag(a *ast.Node) {
	m, _ := a.MarshalJSON()
	clone, _ := sonic.Get(m)
	d.diagnostics = &clone
}

func (d Doc) GetDiag() *ast.Node {
	return d.diagnostics
}

func (d Doc) PrintTarget() {
	d.target.Print()
}

func (d Doc) GoxImportPos(enc common.Encoding) (common.Pos, bool) {
	if !assembler.NeedsGoxImport(d.source.Source(), d.tree.RootNode()) {
		return common.NoPos(), false
	}
	pos, ok := assembler.WhereToImport(d.source.Source(), d.tree.RootNode())
	if !ok {
		return common.NoPos(), false
	}
	pos = d.source.IntoPos(enc, pos)
	return pos, true
}

func (d Doc) Err() error {
	if d == nil {
		return errors.New("This file is not part of the current workspace.")
	}
	return d.err
}

func (d Doc) Assemble() {
	d.target, d.translator = assembler.Assemble(d.sourceFile.Path(), d.source, d.tree.RootNode())
}

func (d Doc) Load() error {
	d.source = text.NewText()
	return d.source.Load(d.sourceFile.Path())
}

func (d Doc) Parse() error {
	d.tree = d.parser.Parse(d.source.Source(), nil)
	if d.tree.RootNode().HasError() {
		msg := "GoX could not parse this file. Fix the syntax errors and try again."
		if report := d.SyntaxErrorReport(); report != "" {
			msg += "\n" + report
		}
		return errors.New(msg)
	}
	return nil
}

func (d Doc) Init() {
	if !d.sourceFile.Exists() {
		d.targetRemove()
		d.err = errors.New("The source .gox file no longer exists.")
		slog.Error("Document init failed", "source", d.SourceFile().Path(), "target", d.TargetFile().Path(), "error", d.err)
		return
	}
	err := d.Load()
	if err != nil {
		d.err = errors.New("Could not read the source .gox file: " + err.Error())
		slog.Error("Document init failed", "source", d.SourceFile().Path(), "target", d.TargetFile().Path(), "error", d.err)
		return
	}
	if d.tree != nil {
		d.tree.Close()
	}
	if err := d.Parse(); err != nil {
		slog.Debug("Document parsed with syntax errors", "source", d.SourceFile().Path(), "target", d.TargetFile().Path(), "error", err)
	}
	d.Assemble()
	needsUpdate, err := d.CheckTarget()
	if err != nil {
		d.err = err
		slog.Error("Document init failed", "source", d.SourceFile().Path(), "target", d.TargetFile().Path(), "error", d.err)
		return
	}
	if !needsUpdate {
		return
	}
	err = d.TargetWrite()
	if err != nil {
		d.err = errors.New("Could not write the generated .x.go file: " + err.Error())
		slog.Error("Document init failed", "source", d.SourceFile().Path(), "target", d.TargetFile().Path(), "error", d.err)
		return
	}
	slog.Debug("Document initialized", "source", d.SourceFile().Path(), "target", d.TargetFile().Path(), "targetUpdated", needsUpdate)
}

func (d Doc) Save() error {
	return d.TargetWrite()
}

func (d Doc) CheckTarget() (needsUpdate bool, err error) {
	return d.TargetFile().ValidateTarget(d.target.Source())
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
