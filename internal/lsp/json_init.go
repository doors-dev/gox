package lsp

import (
	"errors"

	"github.com/bytedance/sonic/ast"
	"github.com/doors-dev/gox/internal/common"
)

var jsonInit jsonInitDriver

type jsonInitDriver struct{}

func (r jsonInitDriver) getWorkspaceDirs(j Json) ([]string, error) {
	folders := j.Get("workspaceFolders")
	if !folders.Exists() {
		return nil, errors.New("Can't get workspace folders from the intialize request")
	}
	a, err := folders.ArrayUseNode()
	if err != nil {
		return nil, errors.New("Can't get workspace folders from the intialize request")
	}
	uris := make([]string, 0, len(a))
	for _, folder := range a {
		uri := folder.Get("uri")
		if uri == nil {
			return nil, errors.New("Can't read workspace folder URI from the intialize request")
		}
		uriStr, err := uri.String()
		if err != nil {
			return nil, errors.New("Can't read workspace folder URI from the intialize request")
		}
		uris = append(uris, uriStr)
	}
	return uris, nil
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
