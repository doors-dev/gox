package lsp

import (
	"errors"
	"github.com/bytedance/sonic/ast"
	"github.com/doors-dev/gox/internal/common"
	"strings"
)

var jsonInit jsonInitDriver

type jsonInitDriver struct{}

func (r jsonInitDriver) prepeareForVscodeExtension(j Json) {
	caps := j.Get("capabilities")
	if !caps.Exists() {
		return
	}
	caps.Unset("foldingRangeProvider")
	caps.Unset("semanticTokensProvider")
}

func (r jsonInitDriver) isVscodeExtension(j Json) bool {
	clientInfo := j.Get("clientInfo")
	if !clientInfo.Exists() {
		return false
	}
	name := clientInfo.Get("name")
	if !name.Exists() {
		return false
	}
	str, err := name.String()
	if err != nil {
		return false
	}
	return strings.HasSuffix(str, "[GOX_EXT]")
}

func (r jsonInitDriver) getWorkspaceDirsFromArray(j Json) ([]string, error) {
	a, err := j.ArrayUseNode()
	if err != nil {
		return nil, errors.New("Workspace folders must be an array.")
	}
	uris := make([]string, 0, len(a))
	for _, folder := range a {
		uri := folder.Get("uri")
		if uri == nil {
			return nil, errors.New("A workspace folder URI is missing.")
		}
		uriStr, err := uri.String()
		if err != nil {
			return nil, errors.New("A workspace folder URI is invalid.")
		}
		uris = append(uris, uriStr)
	}
	return uris, nil
}

func (r jsonInitDriver) getWorkspaceDirs(j Json) ([]string, error) {
	folders := j.Get("workspaceFolders")
	if !folders.Exists() {
		return nil, errors.New("The initialize request is missing workspace folders.")
	}
	return r.getWorkspaceDirsFromArray(folders)
}

func (r jsonInitDriver) readEncoding(j Json) (common.Encoding, error) {
	cap := j.Get("capabilities")
	if !cap.Exists() {
		return common.UTF16, errors.New("Client capabilities are missing.")
	}
	encoding := cap.Get("positionEncoding")
	if !encoding.Exists() {
		return common.UTF16, nil
	}
	str, err := encoding.String()
	if err != nil {
		return common.UTF16, errors.New("The client position encoding must be a string.")
	}
	switch str {
	case "utf-8":
		return common.UTF8, nil
	case "utf-16":
		return common.UTF16, nil
	default:
		return common.UTF16, errors.New("The client position encoding is not supported.")
	}
}

func (r jsonInitDriver) setEncodings(j Json) error {
	cap := j.Get("capabilities")
	if !cap.Exists() {
		return nil
	}
	general := cap.Get("general")
	if !general.Exists() {
		return nil
	}
	encodings := general.Get("positionEncodings")
	if !encodings.Exists() {
		return nil
	}
	arr, err := encodings.ArrayUseNode()
	if err != nil {
		return nil
	}
	hasUtf16 := false
	hasUtf8 := false
	for _, v := range arr {
		str, err := v.String()
		if err != nil {
			continue
		}
		if str == "utf-16" {
			hasUtf16 = true
		}
		if str == "utf-8" {
			hasUtf8 = true
		}
	}
	if hasUtf8 {
		general.Set("positionEncoding", ast.NewArray([]ast.Node{ast.NewString("utf-8")}))
		return nil
	}
	if !hasUtf16 {
		return errors.New("The client does not support UTF-16 position encoding.")
	}
	general.Set("positionEncoding", ast.NewArray([]ast.Node{ast.NewString("utf-16")}))
	return nil
}

func (d jsonInitDriver) insertCompletionTriggers(j Json) error {
	cap := j.Get("capabilities")
	if !cap.Exists() {
		return errors.New("Server capabilities are missing.")
	}
	completion := cap.Get("completionProvider")
	if !completion.Exists() {
		return errors.New("The server completion provider is missing.")
	}
	triggers := completion.Get("triggerCharacters")
	if !triggers.Exists() {
		return errors.New("The server completion trigger list is missing.")
	}
	triggers.Add(ast.NewString("<"))
	triggers.Add(ast.NewString("/"))
	triggers.Add(ast.NewString("~"))
	return nil
}

func (r jsonInitDriver) getWorkspaceChanges(j Json) (added []string, removed []string, err error) {
	event := j.Get("event")
	if !event.Exists() {
		return nil, nil, errors.New("Workspace folder change event is missing.")
	}
	addedNodes := event.Get("added")
	if addedNodes.Exists() {
		a, err := addedNodes.ArrayUseNode()
		if err == nil {
			for _, node := range a {
				uri := node.Get("uri")
				if uri == nil {
					return nil, nil, errors.New("An added workspace folder URI is missing.")
				}
				str, err := uri.String()
				if err != nil {
					return nil, nil, errors.New("An added workspace folder URI is invalid.")
				}
				added = append(added, str)
			}
		}
	}
	removedNodes := event.Get("removed")
	if removedNodes.Exists() {
		a, err := removedNodes.ArrayUseNode()
		if err == nil {
			for _, node := range a {
				uri := node.Get("uri")
				if uri == nil {
					return nil, nil, errors.New("A removed workspace folder URI is missing.")
				}
				str, err := uri.String()
				if err != nil {
					return nil, nil, errors.New("A removed workspace folder URI is invalid.")
				}
				removed = append(removed, str)
			}
		}
	}
	return
}
