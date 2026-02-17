package lsp

import (
	"errors"
	"strings"
	"github.com/bytedance/sonic/ast"
	"github.com/doors-dev/gox/internal/common"
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
		return nil, errors.New("Expecting array for workspace folders")
	}
	uris := make([]string, 0, len(a))
	for _, folder := range a {
		uri := folder.Get("uri")
		if uri == nil {
			return nil, errors.New("Can't read workspace folder URI")
		}
		uriStr, err := uri.String()
		if err != nil {
			return nil, errors.New("Can't read workspace folder URI")
		}
		uris = append(uris, uriStr)
	}
	return uris, nil
}

func (r jsonInitDriver) getWorkspaceDirs(j Json) ([]string, error) {
	folders := j.Get("workspaceFolders")
	if !folders.Exists() {
		return nil, errors.New("Can't get workspace folders from the intialize request")
	}
	return r.getWorkspaceDirsFromArray(folders)
}

func (r jsonInitDriver) readEncoding(j Json) (common.Encoding, error) {
	cap := j.Get("capabilities")
	if !cap.Exists() {
		return common.UTF16, errors.New("no capabilities")
	}
	encoding := cap.Get("positionEncoding")
	if !encoding.Exists() {
		return common.UTF16, nil
	}
	str, err := encoding.String()
	if err != nil {
		return common.UTF16, errors.New("positionEncoding is not a string")
	}
	switch str {
	case "utf-8":
		return common.UTF8, nil
	case "utf-16":
		return common.UTF16, nil
	default:
		return common.UTF16, errors.New("unsupported positionEncoding")
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
		return errors.New("no utf-16 encoding")
	}
	general.Set("positionEncoding", ast.NewArray([]ast.Node{ast.NewString("utf-16")}))
	return nil
}

func (d jsonInitDriver) insertCompletionTriggers(j Json) error {
	cap := j.Get("capabilities")
	if !cap.Exists() {
		return errors.New("no capabilities")
	}
	completion := cap.Get("completionProvider")
	if !completion.Exists() {
		return errors.New("no completion provider")
	}
	triggers := completion.Get("triggerCharacters")
	if !triggers.Exists() {
		return errors.New("no completion triggers")
	}
	triggers.Add(ast.NewString("<"))
	triggers.Add(ast.NewString("/"))
	triggers.Add(ast.NewString("~"))
	return nil
}

func (r jsonInitDriver) getWorkspaceChanges(j Json) (added []string, removed []string, err error) {
	event := j.Get("event")
	if !event.Exists() {
		return nil, nil, errors.New("no event")
	}
	addedNodes := event.Get("added")
	if addedNodes.Exists() {
		a, err := addedNodes.ArrayUseNode()
		if err == nil {
			for _, node := range a {
				uri := *node.Get("uri")
				str, err := uri.String()
				if err != nil {
					return nil, nil, errors.New("added uri is not string")
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
				uri := *node.Get("uri")
				str, err := uri.String()
				if err != nil {
					return nil, nil, errors.New("removed uri is not string")
				}
				removed = append(removed, str)
			}
		}
	}
	return
}
