package processor

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/doors-dev/gox/internal/rust"
	"github.com/doors-dev/gox/internal/workspace"
	"github.com/go-git/go-billy/v6/osfs"
	"github.com/go-git/go-git/v6/plumbing/format/gitignore"
	"golang.org/x/sync/errgroup"
)

type task int

const (
	formatting task = iota
	generation
)

type processor struct {
	ignore    gitignore.Matcher
	force     bool
	isDir     bool
	root      string
	task      task
	wg        errgroup.Group
	errors    []string
	updated   atomic.Int32
	skipped   atomic.Int32
	removed   atomic.Int32
	formatted atomic.Int32
}

func (p *processor) formatPrint(dur time.Duration) {
	if len(p.errors) > 0 {
		fmt.Print(red("FAIL") + " ")
	} else {
		fmt.Print(green("SUCCESS") + " ")
	}
	fmt.Printf(
		"[formatted=%d skipped=%d errors=%d duration=%dms]\n",
		p.formatted.Load(), p.skipped.Load(), len(p.errors), dur.Milliseconds(),
	)
}

func (p *processor) genPrint(dur time.Duration) {
	if len(p.errors) > 0 {
		fmt.Print(red("FAIL") + " ")
	} else {
		fmt.Print(green("SUCCESS") + " ")
	}
	fmt.Printf(
		"[updated=%d skipped=%d removed=%d errors=%d duration=%dms]\n",
		p.updated.Load(), p.skipped.Load(), p.removed.Load(), len(p.errors), dur.Milliseconds(),
	)
}

func (p *processor) addError(err error, parts ...string) {
	p.errors = append(p.errors, red("✗")+" ["+strings.Join(parts, "")+"] "+err.Error())
}

func (p *processor) genFile(file workspace.File) {
	var source, target workspace.File
	if file.Kind() == workspace.KindSource {
		source = file
		target = file.Reverse()
	} else {
		source = file.Reverse()
		target = file
	}
	if !source.Exists() {
		if !target.Exists() {
			return
		}
		err := target.Remove()
		if err != nil {
			p.addError(err, "Target file ", target.Path(), " removal error")
			return
		}
		p.removed.Add(1)
		return
	}
	doc := workspace.NewDoc(source)
	err := doc.Load()
	if err != nil {
		p.addError(err, "Source file ", source.Path(), " loading error")
		return
	}
	err = doc.Parse()
	if err != nil {
		p.addError(err, "Source file ", source.Path(), " parsing error")
		return
	}
	doc.Assemble()
	if !p.force {
		needsUpdate, err := doc.CheckTarget()
		if !needsUpdate {
			p.skipped.Add(1)
			return
		}
		if err != nil {
			p.addError(err, "Target file ", target.Path(), " checking error")
			return
		}
	}
	err = doc.TargetWrite()
	if err != nil {
		p.addError(err, "Target file ", target.Path(), " writing error")
		return
	}
	p.updated.Add(1)
}

func (p *processor) walkGen(path string) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}
	files := make(map[string]workspace.File)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		path := filepath.Join(path, e.Name())
		if p.ignore != nil {
			rel, _ := filepath.Rel(p.root, path)
			rel = filepath.ToSlash(rel)
			parts := strings.Split(rel, "/")
			if p.ignore.Match(parts, e.IsDir()) {
				continue
			}
		}
		if e.IsDir() {
			p.walkGen(path)
			continue
		}
		file, ok := workspace.NewFile(path)
		if !ok {
			continue
		}
		_, registered := files[file.Name()]
		if file.Kind() == workspace.KindSource || !registered {
			files[file.Name()] = file
			continue
		}
	}
	for _, file := range files {
		p.wg.Go(func() error {
			if file.Kind() == workspace.KindTarget {
				file = file.Reverse()
			}
			p.genFile(file)
			return nil
		})
	}
}

func (p *processor) format(path string, formatter func([]byte) ([]byte, error)) {
	file, err := os.ReadFile(path)
	if err != nil {
		p.addError(err, "File ", path, " reading error")
		return
	}
	formatted, err := formatter(file)
	if err != nil {
		p.addError(err, "File ", path, " formatting error")
		return
	}
	if formatted == nil {
		p.skipped.Add(1)
		return
	}
	err = os.WriteFile(path, formatted, 0644)
	if err != nil {
		p.addError(err, "File ", path, " writing error")
		return
	}
	p.formatted.Add(1)
}

func (p *processor) walkFmt(path string) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}
	for _, e := range entries {
		path := filepath.Join(path, e.Name())
		if p.ignore != nil {
			rel, _ := filepath.Rel(p.root, path)
			rel = filepath.ToSlash(rel)
			parts := strings.Split(rel, "/")
			if p.ignore.Match(parts, e.IsDir()) {
				continue
			}
		}
		if e.IsDir() {
			p.walkFmt(path)
			continue
		}
		p.formatFile(path)
	}
}

func (p *processor) formatFile(path string) bool {
	if strings.HasSuffix(path, ".gox") {
		p.wg.Go(func() error {
			p.format(path, rust.Format)
			return nil
		})
		return true
	}
	if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, ".x.go") {
		p.wg.Go(func() error {
			p.format(path, func(b []byte) ([]byte, error) {
				o, err := format.Source(b)
				if err != nil {
					return nil, err
				}
				if bytes.Equal(b, o) {
					return nil, nil
				}
				return o, nil
			})
			return nil
		})
		return true
	}
	return false
}

func (p *processor) run() error {
	switch p.task {
	case generation:
		start := time.Now()
		if p.isDir {
			p.walkGen(p.root)
			p.wg.Wait()
		} else {
			file, ok := workspace.NewFile(p.root)
			if !ok {
				return errors.New("Expected a .gox or .x.go file: " + p.root)
			}
			p.genFile(file)
		}
		p.genPrint(time.Since(start))
		if len(p.errors) > 0 {
			return errors.New(strings.Join(p.errors, "\n"))
		}
		return nil
	case formatting:
		start := time.Now()
		if p.isDir {
			p.walkFmt(p.root)
		} else if !p.formatFile(p.root) {
			return errors.New("Expected a .go or .gox file: " + p.root)
		}
		p.wg.Wait()
		p.formatPrint(time.Since(start))
		if len(p.errors) > 0 {
			return errors.New(strings.Join(p.errors, "\n"))
		}
		return nil
	default:
		panic("unsupported task")
	}
}

func newProcessor(path string, noIgnore bool, force bool, task task) (*processor, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	var ignore gitignore.Matcher = nil
	isDir := info.IsDir()
	if !noIgnore && isDir {
		fs := osfs.New(path)
		pats, err := gitignore.ReadPatterns(fs, nil)
		if err != nil {
			panic("gitignore read error: " + err.Error())
		}
		ignore = gitignore.NewMatcher(pats)
	}
	p := processor{
		ignore: ignore,
		isDir:  isDir,
		root:   path,
		task:   task,
		force:  force,
	}
	p.wg.SetLimit(runtime.GOMAXPROCS(0) * 2)
	return &p, nil
}

func Generate(path string, noIgnore bool, force bool) error {
	p, err := newProcessor(path, noIgnore, force, generation)
	if err != nil {
		return err
	}
	return p.run()
}

func Format(path string, noIgnore bool, force bool) error {
	p, err := newProcessor(path, noIgnore, force, formatting)
	if err != nil {
		return err
	}
	return p.run()
}
