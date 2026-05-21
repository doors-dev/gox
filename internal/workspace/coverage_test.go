package workspace

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/doors-dev/gox/internal/catalog/grammer"
	"github.com/doors-dev/gox/internal/common"
	"github.com/doors-dev/gox/internal/docpath"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func TestDocCompletionsProvideTagAndAttributeSuggestions(t *testing.T) {
	tagDoc := makeDocLoose(t, `package demo

import _ "github.com/doors-dev/gox"

elem View() {
	<di
}
`)
	tagPos := posAfterMarker(t, tagDoc.SourceFile().Path(), "<di")
	tagCompletions, _ := tagDoc.Completions(common.UTF8, tagPos)
	if !hasCompletionLabel(tagCompletions, "<div/>") {
		t.Fatalf("tag completions = %#v, want <div/>", tagCompletions)
	}

	attrDoc := makeDocLoose(t, `package demo

import _ "github.com/doors-dev/gox"

elem View() {
	<div cla></div>
}
`)
	attrPos := posAfterMarker(t, attrDoc.SourceFile().Path(), "cla")
	attrCompletions, _ := attrDoc.Completions(common.UTF8, attrPos)
	if !hasCompletionLabel(attrCompletions, "class=") {
		t.Fatalf("attribute completions = %#v, want class=", attrCompletions)
	}

	root := attrDoc.tree.RootNode()
	attrNode := findFirstKind(root, grammer.GOX_ATTR_NAME)
	if attrNode == nil {
		t.Fatal("attr name node not found")
	}
	if isAttrAssigned(attrDoc.source.Source(), attrNode) {
		t.Fatal("isAttrAssigned() = true, want false")
	}
	tagName, ok := findAttrTagName(attrDoc.source.Source(), attrNode)
	if !ok || tagName != "div" {
		t.Fatalf("findAttrTagName() = (%q, %v), want (div, true)", tagName, ok)
	}
	openHead := attrNode.Parent().Parent()
	if openHead == nil || !hasAttrs(openHead) {
		t.Fatal("hasAttrs() = false, want true")
	}

	boolDoc := makeDocLoose(t, `package demo

import _ "github.com/doors-dev/gox"

elem View() {
	<input disab></input>
}
`)
	boolPos := posAfterMarker(t, boolDoc.SourceFile().Path(), "disab")
	boolCompletions, _ := boolDoc.Completions(common.UTF8, boolPos)
	if !hasCompletionLabel(boolCompletions, "disabled") {
		t.Fatalf("bool attr completions = %#v, want disabled", boolCompletions)
	}
	if hasCompletionLabel(boolCompletions, "disabled=") {
		t.Fatalf("bool attr completions = %#v, do not want disabled=", boolCompletions)
	}

	assignedDoc := makeDocLoose(t, `package demo

import _ "github.com/doors-dev/gox"

elem View() {
	<div class="card"></div>
}
`)
	assignedPos := posAfterMarker(t, assignedDoc.SourceFile().Path(), "class")
	assignedCompletions, _ := assignedDoc.Completions(common.UTF8, assignedPos)
	if !hasCompletionLabel(assignedCompletions, "class") {
		t.Fatalf("assigned attr completions = %#v, want class", assignedCompletions)
	}
	if hasCompletionLabel(assignedCompletions, "class=") {
		t.Fatalf("assigned attr completions = %#v, do not want class=", assignedCompletions)
	}
}

func TestDocCompletionsHandleImplicitCloseAndProxyTilde(t *testing.T) {
	closeDoc := makeDocLoose(t, `package demo

import _ "github.com/doors-dev/gox"

elem View() {
	<section>
		<div></
	</section>
}
`)
	closePos := posAfterMarker(t, closeDoc.SourceFile().Path(), "</")
	closeCompletions, _ := closeDoc.Completions(common.UTF8, closePos)
	if !hasCompletionLabel(closeCompletions, "</div>") {
		t.Fatalf("close completions = %#v, want </div>", closeCompletions)
	}

	proxyDoc := makeDocLoose(t, `package demo

import _ "github.com/doors-dev/gox"

elem View() {
	<div>~~</div>
}
`)
	proxyPos := posAfterMarker(t, proxyDoc.SourceFile().Path(), "~~")
	proxyCompletions, _ := proxyDoc.Completions(common.UTF8, proxyPos)
	if len(proxyCompletions) != 1 {
		t.Fatalf("proxy tilde completions len = %d, want 1 (%#v)", len(proxyCompletions), proxyCompletions)
	}
	if !hasCompletionLabel(proxyCompletions, "~(..)") {
		t.Fatalf("proxy tilde completions = %#v, want ~(..)", proxyCompletions)
	}
	if hasCompletionLabel(proxyCompletions, "~(if..)") {
		t.Fatalf("proxy tilde completions = %#v, do not want ~(if..)", proxyCompletions)
	}
}

func TestManagerWorkspaceLifecycleAndFileValidation(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	pathA := filepath.Join(dirA, "view.gox")
	pathB := filepath.Join(dirB, "other.gox")
	if err := os.WriteFile(pathA, []byte(sampleSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pathB, []byte(sampleSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager()
	manager.EnsureWorkspaces([]string{
		string(docpath.URIFromPath(dirA)),
		string(docpath.URIFromPath(dirB)),
	})

	docA, kindA := manager.Doc(string(docpath.URIFromPath(pathA)))
	if kindA != KindSource || docA == nil {
		t.Fatalf("Doc(pathA) = (%v, %v), want source doc", docA, kindA)
	}

	slashedManager := NewManager()
	slashedManager.EnsureWorkspaces([]string{string(docpath.URIFromPath(dirA)) + "/"})
	if doc, kind := slashedManager.Doc(string(docpath.URIFromPath(pathA))); doc == nil || kind != KindSource {
		t.Fatalf("Doc(pathA) with trailing slash workspace = (%v, %v), want source doc", doc, kind)
	}
	slashedManager.RemoveWorkspace(string(docpath.URIFromPath(dirA)))
	if doc, kind := slashedManager.Doc(string(docpath.URIFromPath(pathA))); doc != nil || kind != KindSource {
		t.Fatalf("Doc(pathA) after slash-normalized remove = (%v, %v), want nil source kind", doc, kind)
	}
	slashedManager.AddWorkspace(string(docpath.URIFromPath(dirA)) + "/")
	if doc, kind := slashedManager.Doc(string(docpath.URIFromPath(pathA))); doc == nil || kind != KindSource {
		t.Fatalf("Doc(pathA) after slash-normalized add = (%v, %v), want source doc", doc, kind)
	}
	slashedManager.RemoveWorkspace(string(docpath.URIFromPath(dirA)))

	removedWorkspace := manager.workspaces[0]
	manager.RemoveWorkspace(string(docpath.URIFromPath(dirA)))
	select {
	case <-removedWorkspace.done:
	case <-time.After(time.Second):
		t.Fatal("removed workspace scanner did not stop")
	}
	if doc, kind := manager.Doc(string(docpath.URIFromPath(pathA))); doc != nil || kind != KindSource {
		t.Fatalf("Doc(pathA) after remove = (%v, %v), want nil source kind", doc, kind)
	}
	if err := os.Remove(docA.TargetFile().Path()); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	time.Sleep(600 * time.Millisecond)
	if _, err := os.Stat(docA.TargetFile().Path()); err == nil {
		t.Fatalf("removed workspace regenerated target %s", docA.TargetFile().Path())
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}

	emptyDir := t.TempDir()
	manager.AddWorkspace(string(docpath.URIFromPath(emptyDir)))
	if doc, kind := manager.Doc(string(docpath.URIFromPath(filepath.Join(emptyDir, "missing.gox")))); doc != nil || kind != KindSource {
		t.Fatalf("Doc(missing in added workspace) = (%v, %v), want nil source kind", doc, kind)
	}

	content := []byte(docA.TargetContent())
	version, err := readFirstLine(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("readFirstLine(target content) error = %v", err)
	}
	semverContent := bytes.Replace(content, []byte(version), []byte("v0.0.0"), 1)
	if err := os.WriteFile(docA.TargetFile().Path(), semverContent, 0o644); err != nil {
		t.Fatalf("TargetWrite() error = %v", err)
	}
	needsUpdate, err := docA.TargetFile().ValidateTarget(semverContent)
	if err != nil || needsUpdate {
		t.Fatalf("ValidateTarget(same) = (%v, %v)", needsUpdate, err)
	}

	updated, err := docA.TargetFile().ValidateTarget(append(append([]byte{}, semverContent...), []byte("\n// changed")...))
	if err != nil || !updated {
		t.Fatalf("ValidateTarget(changed) = (%v, %v)", updated, err)
	}

	targetBytes, err := os.ReadFile(docA.TargetFile().Path())
	if err != nil {
		t.Fatal(err)
	}
	currentVersion, err := readFirstLine(bytes.NewReader(targetBytes))
	if err != nil {
		t.Fatalf("readFirstLine() error = %v", err)
	}
	newer := bytes.Replace(targetBytes, []byte(currentVersion), []byte("v9999.0.0"), 1)
	if err := os.WriteFile(docA.TargetFile().Path(), newer, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := docA.TargetFile().ValidateTarget(semverContent); err == nil {
		t.Fatal("ValidateTarget(newer file version) error = nil, want error")
	}

	develContent := bytes.Replace(semverContent, []byte("v0.0.0"), []byte("(devel)"), 1)
	if err := os.WriteFile(docA.TargetFile().Path(), develContent, 0o644); err != nil {
		t.Fatalf("write devel file: %v", err)
	}
	if needsUpdate, err := docA.TargetFile().ValidateTarget(develContent); err != nil || needsUpdate {
		t.Fatalf("ValidateTarget(devel == devel) = (%v, %v), want (false, nil)", needsUpdate, err)
	}
	if needsUpdate, err := docA.TargetFile().ValidateTarget(semverContent); err != nil || needsUpdate {
		t.Fatalf("ValidateTarget(devel file vs semver bytes) = (%v, %v), want (false, nil)", needsUpdate, err)
	}
	develChanged := append(append([]byte{}, develContent...), []byte("\n// changed")...)
	if needsUpdate, err := docA.TargetFile().ValidateTarget(develChanged); err != nil || !needsUpdate {
		t.Fatalf("ValidateTarget(devel file vs changed devel bytes) = (%v, %v), want (true, nil)", needsUpdate, err)
	}
	if err := os.WriteFile(docA.TargetFile().Path(), semverContent, 0o644); err != nil {
		t.Fatalf("restore semver file: %v", err)
	}
	if needsUpdate, err := docA.TargetFile().ValidateTarget(develContent); err != nil || needsUpdate {
		t.Fatalf("ValidateTarget(semver file vs devel bytes) = (%v, %v), want (false, nil)", needsUpdate, err)
	}

	toRemove := docA.TargetFile()
	if !toRemove.Exists() {
		t.Fatal("target file missing before remove")
	}
	if err := toRemove.remove(); err != nil {
		t.Fatalf("remove() error = %v", err)
	}
	if toRemove.Exists() {
		t.Fatal("target file still exists after remove")
	}
}

func posAfterMarker(t *testing.T, path string, marker string) common.Pos {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	index := strings.Index(string(data), marker)
	if index < 0 {
		t.Fatalf("%q not found in %s", marker, path)
	}
	line := 0
	col := 0
	for i := 0; i < index+len(marker); i++ {
		if data[i] == '\n' {
			line++
			col = 0
			continue
		}
		col++
	}
	return common.NewPos(line, col)
}

func hasCompletionLabel(completions []Completion, want string) bool {
	for _, c := range completions {
		if c.Label == want {
			return true
		}
	}
	return false
}

func findFirstKind(root *tree_sitter.Node, kind string) *tree_sitter.Node {
	cursor := root.Walk()
	defer cursor.Close()
	if root.Kind() == kind {
		return root
	}
	for _, child := range root.Children(cursor) {
		childCopy := child
		if found := findFirstKind(&childCopy, kind); found != nil {
			return found
		}
	}
	return nil
}
