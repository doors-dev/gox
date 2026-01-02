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
	"sync"
	"sync/atomic"
	"time"

	"github.com/doors-dev/gox/internal/formatter"
	"github.com/doors-dev/gox/internal/workspace"
	"github.com/go-git/go-billy/v6/osfs"
	"github.com/go-git/go-git/v6/plumbing/format/gitignore"
	"github.com/panjf2000/ants/v2"
)

type task int

const (
	formatting task = iota
	generation
)

type processor struct {
	pool      *ants.Pool
	ignore    gitignore.Matcher
	root      string
	task      task
	wg        sync.WaitGroup
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
	if doc.TargetIsUpToDate() {
		p.skipped.Add(1)
		return
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
		p.wg.Add(1)
		p.pool.Submit(func() {
			defer p.wg.Done()
			if file.Kind() == workspace.KindTarget {
				file = file.Reverse()
			}
			p.genFile(file)
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
	return
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
			p.walkGen(path)
			continue
		}
		if strings.HasSuffix(path, ".gox") {
			p.format(path, formatter.Format)
			continue
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, ".x.go") {
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
			continue
		}
	}
}

func (p *processor) run() error {
	info, err := os.Stat(p.root)
	if err != nil {
		return err
	}
	isDir := info.IsDir()
	switch p.task {
	case generation:
		start := time.Now()
		if isDir {
			p.walkGen(p.root)
			p.wg.Wait()
		} else {
			file, ok := workspace.NewFile(p.root)
			if !ok {
				return errors.New("Not .gox or .x.go file: " + p.root)
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
		p.walkFmt(p.root)
		p.formatPrint(time.Since(start))
		if len(p.errors) > 0 {
			return errors.New(strings.Join(p.errors, "\n"))
		}
		return nil
	default:
		panic("unsupported task")
	}
}

func newProcessor(path string, noIgnore bool, task task) *processor {
	p, err := ants.NewPool(runtime.GOMAXPROCS(0) * 2)
	if err != nil {
		panic(err)
	}
	var ignore gitignore.Matcher = nil
	if !noIgnore {
		fs := osfs.New(path)
		pats, err := gitignore.ReadPatterns(fs, nil)
		if err != nil {
			panic("gitignore read error: " + err.Error())
		}
		ignore = gitignore.NewMatcher(pats)
	}
	return &processor{
		pool:   p,
		ignore: ignore,
		root:   path,
		task:   task,
	}
}

func Generate(path string, noIgnore bool) error {
	return newProcessor(path, noIgnore, generation).run()
}

func Format(path string, noIgnore bool) error {
	return newProcessor(path, noIgnore, formatting).run()
}
