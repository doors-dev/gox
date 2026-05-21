package lsp

import (
	"github.com/bytedance/sonic/ast"
)

func requestLogAttrs(j Json) []any {
	uris := requestURIs(j)
	attrs := make([]any, 0, 10)
	switch len(uris) {
	case 0:
	case 1:
		attrs = append(attrs, "uri", uris[0])
	default:
		attrs = append(attrs, "uris", uris)
	}
	if languageID := stringField(j.Get("textDocument").Get("languageId")); languageID != "" {
		attrs = append(attrs, "languageId", languageID)
	}
	if version, ok := intField(j.Get("textDocument").Get("version")); ok {
		attrs = append(attrs, "version", version)
	}
	if count, ok := arrayLen(j.Get("contentChanges")); ok {
		attrs = append(attrs, "contentChanges", count)
	}
	if count, ok := arrayLen(j.Get("workspaceFolders")); ok {
		attrs = append(attrs, "workspaceFolders", count)
	}
	if count, ok := arrayLen(j.Get("documentChanges")); ok {
		attrs = append(attrs, "documentChanges", count)
	}
	if count, ok := objectLen(j.Get("changes")); ok {
		attrs = append(attrs, "changeUris", count)
	}
	if n, ok := textBytes(j); ok {
		attrs = append(attrs, "textBytes", n)
	}
	return attrs
}

func requestURIs(j Json) []string {
	seen := make(map[string]struct{})
	var uris []string
	addString := func(uri string) {
		if uri == "" {
			return
		}
		if _, ok := seen[uri]; ok {
			return
		}
		seen[uri] = struct{}{}
		uris = append(uris, uri)
	}
	add := func(node Json) {
		addString(stringField(node))
	}
	add(j.Get("uri"))
	add(j.Get("targetUri"))
	add(j.Get("oldUri"))
	add(j.Get("newUri"))
	add(j.Get("textDocument").Get("uri"))
	add(j.Get("item").Get("uri"))
	add(j.Get("from").Get("uri"))

	changes := j.Get("changes")
	if changes.Exists() {
		changes.ForEach(func(path ast.Sequence, _ *ast.Node) bool {
			if path.Key != nil {
				addString(*path.Key)
			}
			return len(uris) < 5
		})
	}
	documentChanges := j.Get("documentChanges")
	if documentChanges.Exists() {
		documentChanges.ForEach(func(_ ast.Sequence, node *ast.Node) bool {
			add(node.Get("textDocument").Get("uri"))
			add(node.Get("uri"))
			add(node.Get("oldUri"))
			add(node.Get("newUri"))
			return len(uris) < 5
		})
	}
	return uris
}

func stringField(j Json) string {
	if j == nil || !j.Exists() {
		return ""
	}
	str, err := j.String()
	if err != nil {
		return ""
	}
	return str
}

func intField(j Json) (int64, bool) {
	if j == nil || !j.Exists() {
		return 0, false
	}
	i, err := j.Int64()
	return i, err == nil
}

func arrayLen(j Json) (int, bool) {
	if j == nil || !j.Exists() {
		return 0, false
	}
	arr, err := j.ArrayUseNode()
	if err != nil {
		return 0, false
	}
	return len(arr), true
}

func objectLen(j Json) (int, bool) {
	if j == nil || !j.Exists() {
		return 0, false
	}
	count := 0
	j.ForEach(func(path ast.Sequence, _ *ast.Node) bool {
		if path.Key != nil {
			count++
		}
		return true
	})
	return count, true
}

func textBytes(j Json) (int, bool) {
	if text := stringField(j.Get("textDocument").Get("text")); text != "" {
		return len(text), true
	}
	if text := stringField(j.Get("text")); text != "" {
		return len(text), true
	}
	return 0, false
}
