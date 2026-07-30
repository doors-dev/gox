package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/doors-dev/gox/internal/common"
	"github.com/doors-dev/gox/internal/docpath"
)

func didSaveParams(t *testing.T, path string, text string) []byte {
	t.Helper()
	params, err := json.Marshal(map[string]any{
		"textDocument": map[string]any{
			"uri": string(docpath.URIFromPath(path)),
		},
		"text": text,
	})
	if err != nil {
		t.Fatal(err)
	}
	return params
}

func TestDidSaveOrphanTargetReportsErrorInsteadOfPanicking(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "view.gox"), []byte(lspHelperSource), 0o644); err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(root, "app.x.go")
	if err := os.WriteFile(orphan, []byte("package demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	bridge := &bridgeStub{}
	router := NewRouter(bridge)
	t.Cleanup(router.Stop)
	router.session.setEnc(common.UTF8)
	ensureRouterWorkspaces(router, string(docpath.URIFromPath(root)))

	router.Notification(Client, Request{
		Method: string(didSave),
		Params: didSaveParams(t, orphan, "package demo\n"),
	})

	for _, notification := range bridge.notifications {
		if notification.role == Gopls && notification.req.Method == string(didSave) {
			t.Fatalf("orphan target didSave was forwarded to gopls: %#v", notification)
		}
	}
	if len(bridge.notifications) == 0 {
		t.Fatal("orphan target didSave produced no client notification")
	}
	var reported string
	for _, notification := range bridge.notifications {
		var payload struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(notification.req.Params, &payload); err != nil {
			continue
		}
		if strings.Contains(payload.Message, "no longer exists") ||
			strings.Contains(payload.Message, "not part of the current workspace") {
			reported = payload.Message
		}
	}
	if reported == "" {
		t.Fatalf("orphan target didSave did not report a document error: %#v", bridge.notifications)
	}
}

func TestDidSaveTargetStillRevertsHandEdits(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "view.gox")
	if err := os.WriteFile(source, []byte(lspHelperSource), 0o644); err != nil {
		t.Fatal(err)
	}

	bridge := &bridgeStub{}
	router := NewRouter(bridge)
	t.Cleanup(router.Stop)
	router.session.setEnc(common.UTF8)
	ensureRouterWorkspaces(router, string(docpath.URIFromPath(root)))

	target := filepath.Join(root, "view.x.go")
	generated, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("generated target was not produced: %v", err)
	}
	handEdited := string(generated) + "\nvar handEdit = 1\n"
	if err := os.WriteFile(target, []byte(handEdited), 0o644); err != nil {
		t.Fatal(err)
	}

	bridge.notifications = nil
	router.Notification(Client, Request{
		Method: string(didSave),
		Params: didSaveParams(t, target, handEdited),
	})

	restored, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != string(generated) {
		t.Fatalf("hand edits were not reverted on save:\n%s", restored)
	}
	var warned bool
	for _, notification := range bridge.notifications {
		var payload struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(notification.req.Params, &payload); err != nil {
			continue
		}
		if strings.Contains(payload.Message, "reverted on save") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("revert-on-save warning was not sent: %#v", bridge.notifications)
	}
}
