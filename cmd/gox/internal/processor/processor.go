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

	"github.com/doors-dev/gox/internal/rust"
	"github.com/doors-dev/gox/internal/workspace"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"golang.org/x/sync/errgroup"
)

type task int

const (
	formatting task = iota
	generation
)

type ExitError struct {
	code int
	msg  string
}

func (e *ExitError) Error() string { return e.msg }
func (e *ExitError) ExitCode() int { return e.code }

type processor struct {
	force bool
	check bool
	fmtGo bool
	task  task

	ignore gitignore.Matcher
	root   string

	wg      errgroup.Group
	mu      sync.Mutex
	errors  []string
	changed []string
	seen    map[string]bool

	updated   atomic.Int32
	skipped   atomic.Int32
	removed   atomic.Int32
	formatted atomic.Int32
}

func (p *processor) addError(err error, parts ...string) {
	line := red("✗") + " [" + strings.Join(parts, "") + "] " + err.Error()
	p.mu.Lock()
	p.errors = append(p.errors, line)
	p.mu.Unlock()
}

func (p *processor) addChange(path string, reason string) {
	p.mu.Lock()
	p.changed = append(p.changed, yellow("~")+" "+reason+" "+path)
	p.mu.Unlock()
}

func (p *processor) claim(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.seen[abs] {
		return false
	}
	p.seen[abs] = true
	return true
}

func (p *processor) failed() bool {
	return len(p.errors) > 0 || (p.check && len(p.changed) > 0)
}

func (p *processor) formatPrint(dur time.Duration) {
	p.printResult()
	if p.failed() {
		fmt.Print(red("FAIL") + " ")
	} else {
		fmt.Print(green("SUCCESS") + " ")
	}
	if p.check {
		fmt.Printf(
			"[needsWork=%d skipped=%d errors=%d duration=%dms]\n",
			len(p.changed), p.skipped.Load(), len(p.errors), dur.Milliseconds(),
		)
		return
	}
	fmt.Printf(
		"[formatted=%d skipped=%d errors=%d duration=%dms]\n",
		p.formatted.Load(), p.skipped.Load(), len(p.errors), dur.Milliseconds(),
	)
}

func (p *processor) genPrint(dur time.Duration) {
	p.printResult()
	if p.failed() {
		fmt.Print(red("FAIL") + " ")
	} else {
		fmt.Print(green("SUCCESS") + " ")
	}
	if p.check {
		fmt.Printf(
			"[needsWork=%d skipped=%d errors=%d duration=%dms]\n",
			len(p.changed), p.skipped.Load(), len(p.errors), dur.Milliseconds(),
		)
		return
	}
	fmt.Printf(
		"[updated=%d skipped=%d removed=%d errors=%d duration=%dms]\n",
		p.updated.Load(), p.skipped.Load(), p.removed.Load(), len(p.errors), dur.Milliseconds(),
	)
}

func (p *processor) printResult() {
	for _, c := range p.changed {
		fmt.Println(c)
	}
	for _, e := range p.errors {
		fmt.Println(e)
	}
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
	if !p.claim(source.Path()) {
		return
	}
	if !source.Exists() {
		if !target.Exists() {
			return
		}
		if p.check {
			p.addChange(target.Path(), "orphaned, would remove")
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
	defer doc.Close()
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
	if p.check {
		needsUpdate, err := doc.CheckTarget()
		if err != nil {
			p.addError(err, "Target file ", target.Path(), " checking error")
			return
		}
		if needsUpdate {
			p.addChange(target.Path(), "stale, would regenerate")
		} else {
			p.skipped.Add(1)
		}
		return
	}
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

func (p *processor) ignored(path string, isDir bool) bool {
	if p.ignore == nil {
		return false
	}
	rel, err := filepath.Rel(p.root, path)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	return p.ignore.Match(strings.Split(rel, "/"), isDir)
}

func (p *processor) walkGen(path string) {
	entries, err := os.ReadDir(path)
	if err != nil {
		p.addError(err, "Directory ", path, " reading error")
		return
	}
	files := make(map[string]workspace.File)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		path := filepath.Join(path, e.Name())
		if p.ignored(path, e.IsDir()) {
			continue
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
	if !p.claim(path) {
		return
	}
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
	if formatted == nil || bytes.Equal(formatted, file) {
		p.skipped.Add(1)
		return
	}
	if p.check {
		p.addChange(path, "would reformat")
		return
	}
	err = writeFileAtomic(path, formatted)
	if err != nil {
		p.addError(err, "File ", path, " writing error")
		return
	}
	p.formatted.Add(1)
}

func writeFileAtomic(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".gox-fmt-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	_, err = tmp.Write(data)
	if err == nil {
		err = tmp.Chmod(info.Mode())
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmpPath, path)
	}
	if err != nil {
		os.Remove(tmpPath)
	}
	return err
}

func (p *processor) walkFmt(path string) {
	entries, err := os.ReadDir(path)
	if err != nil {
		p.addError(err, "Directory ", path, " reading error")
		return
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		path := filepath.Join(path, e.Name())
		if p.ignored(path, e.IsDir()) {
			continue
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
			p.format(path, func(b []byte) ([]byte, error) {
				out, err := rust.Format(b)
				if err != nil {
					if report := workspace.SyntaxReport(b); report != "" {
						return nil, errors.New(err.Error() + "\n" + report)
					}
				}
				return out, err
			})
			return nil
		})
		return true
	}
	if p.fmtGo && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, ".x.go") {
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

func normalizePattern(operand string) (string, bool) {
	p := operand
	if len(p) > 1 {
		p = strings.TrimSuffix(p, "/")
	}
	if p == "..." {
		return ".", true
	}
	if base, found := strings.CutSuffix(p, "/..."); found {
		if base == "" {
			return "/", true
		}
		return base, true
	}
	if strings.Contains(p, "/.../") || strings.HasSuffix(p, "...") {
		return operand, false
	}
	return operand, true
}

func (p *processor) buildIgnore(dir string, noIgnore bool) gitignore.Matcher {
	if noIgnore {
		return nil
	}
	fs := osfs.New(dir)
	pats, err := gitignore.ReadPatterns(fs, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, yellow("!")+" could not fully read .gitignore rules ("+err.Error()+"); continuing without full ignore filtering")
	}
	return gitignore.NewMatcher(pats)
}

func (p *processor) processOperand(operand string, noIgnore bool) {
	base, ok := normalizePattern(operand)
	if !ok {
		p.addError(errors.New("unsupported \"...\" pattern; use a directory or a trailing \"/...\" (e.g. ./... or pkg/...)"), "Path ", operand)
		return
	}
	info, err := os.Stat(base)
	if err != nil {
		p.addError(err, "Path ", operand)
		return
	}
	if info.IsDir() {
		p.root = base
		p.ignore = p.buildIgnore(base, noIgnore)
		if p.task == generation {
			p.walkGen(base)
		} else {
			p.walkFmt(base)
		}
		p.wg.Wait()
		return
	}
	p.root = filepath.Dir(base)
	p.ignore = nil
	if p.task == generation {
		file, ok := workspace.NewFile(base)
		if !ok {
			p.addError(errors.New("Expected a .gox or .x.go file"), "Path ", operand)
			return
		}
		if file.Kind() == workspace.KindTarget {
			file = file.Reverse()
		}
		p.genFile(file)
	} else if !p.formatFile(base) {
		p.addError(errors.New("Expected a .go or .gox file"), "Path ", operand)
	}
	p.wg.Wait()
}

func (p *processor) run(paths []string, noIgnore bool) error {
	start := time.Now()
	for _, operand := range paths {
		p.processOperand(operand, noIgnore)
	}
	if p.task == generation {
		p.genPrint(time.Since(start))
	} else {
		p.formatPrint(time.Since(start))
	}
	if p.check {
		if len(p.errors) > 0 {
			return &ExitError{code: 2, msg: fmt.Sprintf("%d error(s) during --check", len(p.errors))}
		}
		if len(p.changed) > 0 {
			return &ExitError{code: 1, msg: fmt.Sprintf("%d file(s) need work", len(p.changed))}
		}
		return nil
	}
	if len(p.errors) > 0 {
		return errors.New(strings.Join(p.errors, "\n"))
	}
	return nil
}

func newProcessor(force bool, check bool, fmtGo bool, task task) *processor {
	p := &processor{
		task:  task,
		force: force,
		check: check,
		fmtGo: fmtGo,
		seen:  make(map[string]bool),
	}
	p.wg.SetLimit(runtime.GOMAXPROCS(0) * 2)
	return p
}

func Generate(paths []string, noIgnore bool, force bool, check bool) error {
	return newProcessor(force, check, true, generation).run(paths, noIgnore)
}

func Format(paths []string, noIgnore bool, force bool, check bool, fmtGo bool) error {
	return newProcessor(force, check, fmtGo, formatting).run(paths, noIgnore)
}
