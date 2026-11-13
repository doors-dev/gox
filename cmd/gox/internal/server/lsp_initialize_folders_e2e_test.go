package server_test

import (
	"path/filepath"
	"testing"
)

func TestLSPInitializeWorkspaceShapes(t *testing.T) {
	cases := []struct {
		name    string
		folders bool
		params  func(folder workspaceFolder, root string) map[string]any
	}{
		{"workspace folders", true, func(folder workspaceFolder, root string) map[string]any {
			return map[string]any{"rootUri": folder.URI, "workspaceFolders": []workspaceFolder{folder}}
		}},
		{"only rootUri", false, func(folder workspaceFolder, root string) map[string]any {
			return map[string]any{"rootUri": folder.URI}
		}},
		{"null workspace folders with rootPath", false, func(folder workspaceFolder, root string) map[string]any {
			return map[string]any{"rootUri": nil, "rootPath": root, "workspaceFolders": nil}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "main")
			writeFile(t, filepath.Join(root, "go.mod"), singleFolderGoMod)
			existing := writeMarkedFile(t, filepath.Join(root, "pkg", "existing.gox"), unsavedExistingSource)
			folder := workspaceFolder{URI: uriForPath(root), Name: "main"}
			var folders []workspaceFolder
			if tc.folders {
				folders = []workspaceFolder{folder}
			}
			h := newLSPHarness(t, folders)
			params := tc.params(folder, root)
			params["processId"] = nil
			params["capabilities"] = map[string]any{
				"workspace":    map[string]any{"applyEdit": true, "workspaceFolders": true},
				"textDocument": map[string]any{"publishDiagnostics": map[string]any{}},
				"window":       map[string]any{},
			}
			h.callRaw("initialize", params)
			h.notify("initialized", map[string]any{})

			uri := h.openFile(existing.Path, "gox")
			h.waitForHover(uri, existing.Marker("existing_def"), "func Existing() string")
			waitForFileTextContains(t, filepath.Join(root, "pkg", "existing.x.go"), "func Existing() string")
			h.assertNoErrorMessagesAfter(0, tc.name)
		})
	}
}
