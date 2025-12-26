package common

import "github.com/doors-dev/gox/internal/jsonrpc"

func FromErr(wire *jsonrpc.WireError, err error) *Err {
	return NewErr(wire, err.Error())
}

func NewErr(wire *jsonrpc.WireError, msg string) *Err {
	return &Err{
		Wire: wire,
		Msg: msg,
	}
}

type Err struct {
	Wire *jsonrpc.WireError
	Msg string
}

func (e *Err) Error() string {
	return e.Msg
}
