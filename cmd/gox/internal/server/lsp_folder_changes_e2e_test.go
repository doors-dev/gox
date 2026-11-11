package server_test

import (
	"path/filepath"
	"testing"
)

// The editor keeps an unsaved .gox buffer open while its folder is replaced
// by the parent folder, and then while every folder covering it is removed
// (another folder keeps gopls running). gox keeps following the buffer, so
// gopls answers from the buffer text throughout.
func TestLSPFolderChangesKeepUnsavedBuffer(t *testing.T) {
	root := filepath.Join(t.TempDir(), "main")
	pkg := filepath.Join(root, "pkg")
	writeFile(t, filepath.Join(root, "go.mod"), singleFolderGoMod)
	existing := writeMarkedFile(t, filepath.Join(pkg, "existing.gox"), unsavedExistingSource)
	other := filepath.Join(t.TempDir(), "other")
	writeFile(t, filepath.Join(other, "go.mod"), "module example.com/other\n\ngo 1.25.1\n")
	parent := workspaceFolder{URI: uriForPath(root), Name: "main"}
	child := workspaceFolder{URI: uriForPath(pkg), Name: "pkg"}
	h := newSingleFolderHarness(t, pkg)

	sourceURI := h.openFile(existing.Path, "gox")
	h.waitForHover(sourceURI, existing.Marker("existing_def"), "func Existing() string")
	insert := func(name string) {
		h.patchFile(sourceURI, unsavedEdit{
			Range: lspRange{Start: lspPosition{Line: 1}, End: lspPosition{Line: 1}},
			Text:  "\nfunc " + name + "() string { return \"x\" }\n",
		})
	}
	insert("AddedFirst")
	checkpoint := h.checkpoint()

	h.changeWorkspaceFolders([]workspaceFolder{parent, {URI: uriForPath(other), Name: "other"}}, nil)
	insert("AddedSecond")
	// Buffer: AddedSecond on line 2, AddedFirst on line 4.
	h.waitForHover(sourceURI, lspPosition{Line: 4, Character: 6}, "func AddedFirst() string")

	h.changeWorkspaceFolders(nil, []workspaceFolder{parent, child})
	insert("AddedThird")
	h.waitForHover(sourceURI, lspPosition{Line: 6, Character: 6}, "func AddedFirst() string")
	h.waitForHover(sourceURI, lspPosition{Line: 2, Character: 6}, "func AddedThird() string")

	h.closeFile(existing.Path)
	h.assertNoErrorMessagesAfter(checkpoint, "folder changes with an unsaved buffer")
}
