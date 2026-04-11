// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonrpc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestMakeIDAndMessageRoundTrip(t *testing.T) {
	t.Run("make id", func(t *testing.T) {
		cases := []struct {
			name  string
			input any
			valid bool
			raw   any
		}{
			{name: "nil", input: nil, valid: false, raw: nil},
			{name: "float", input: float64(7), valid: true, raw: int64(7)},
			{name: "string", input: "abc", valid: true, raw: "abc"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				id, err := MakeID(tc.input)
				if err != nil {
					t.Fatalf("MakeID() error = %v", err)
				}
				if id.IsValid() != tc.valid {
					t.Fatalf("IsValid() = %v, want %v", id.IsValid(), tc.valid)
				}
				if id.Raw() != tc.raw {
					t.Fatalf("Raw() = %#v, want %#v", id.Raw(), tc.raw)
				}
			})
		}

		if _, err := MakeID(true); err == nil {
			t.Fatal("MakeID(true) error = nil, want error")
		}
	})

	t.Run("request round trip", func(t *testing.T) {
		msg, err := NewCall(Int64ID(11), "workspace/symbol", map[string]string{"query": "Card"})
		if err != nil {
			t.Fatalf("NewCall() error = %v", err)
		}
		data, err := EncodeMessage(msg)
		if err != nil {
			t.Fatalf("EncodeMessage() error = %v", err)
		}
		decoded, err := DecodeMessage(data)
		if err != nil {
			t.Fatalf("DecodeMessage() error = %v", err)
		}
		req, ok := decoded.(*Request)
		if !ok {
			t.Fatalf("decoded type = %T, want *Request", decoded)
		}
		if !req.IsCall() || req.Method != "workspace/symbol" || req.ID.Raw() != int64(11) {
			t.Fatalf("decoded request = %#v", req)
		}
		if string(req.Params) != `{"query":"Card"}` {
			t.Fatalf("Params = %s", req.Params)
		}
	})

	t.Run("response round trip", func(t *testing.T) {
		wireErr := NewError(42, "boom")
		msg, err := NewResponse(StringID("id-1"), map[string]int{"count": 2}, wireErr)
		if err != nil {
			t.Fatalf("NewResponse() error = %v", err)
		}
		data, err := EncodeMessage(msg)
		if err != nil {
			t.Fatalf("EncodeMessage() error = %v", err)
		}
		decoded, err := DecodeMessage(data)
		if err != nil {
			t.Fatalf("DecodeMessage() error = %v", err)
		}
		resp, ok := decoded.(*Response)
		if !ok {
			t.Fatalf("decoded type = %T, want *Response", decoded)
		}
		if resp.ID.Raw() != "id-1" {
			t.Fatalf("ID = %#v, want id-1", resp.ID.Raw())
		}
		if string(resp.Result) != `{"count":2}` {
			t.Fatalf("Result = %s", resp.Result)
		}
		var got *WireError
		if !errors.As(resp.Error, &got) {
			t.Fatalf("Error = %T, want *WireError", resp.Error)
		}
		if got.Code != 42 || got.Message != "boom" {
			t.Fatalf("WireError = %#v", got)
		}
	})
}

func TestEncodeIndentAndDecodeErrors(t *testing.T) {
	msg, err := NewNotification("initialized", map[string]string{"ok": "yes"})
	if err != nil {
		t.Fatalf("NewNotification() error = %v", err)
	}
	data, err := EncodeIndent(msg, "", "  ")
	if err != nil {
		t.Fatalf("EncodeIndent() error = %v", err)
	}
	if !bytes.Contains(data, []byte("\n  \"method\"")) {
		t.Fatalf("EncodeIndent() did not indent output:\n%s", data)
	}

	cases := []struct {
		name string
		data string
		want string
	}{
		{
			name: "bad version",
			data: `{"jsonrpc":"1.0","id":1,"method":"x"}`,
			want: `invalid message version tag`,
		},
		{
			name: "invalid id type",
			data: `{"jsonrpc":"2.0","id":true,"method":"x"}`,
			want: `invalid ID type`,
		},
		{
			name: "invalid request",
			data: `{"jsonrpc":"2.0","result":{}}`,
			want: ErrInvalidRequest.Error(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeMessage([]byte(tc.data))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("DecodeMessage() error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestToWireErrorAndWireErrorHelpers(t *testing.T) {
	if got := toWireError(nil); got != nil {
		t.Fatalf("toWireError(nil) = %#v, want nil", got)
	}

	wireErr := NewError(99, "direct")
	if got := toWireError(wireErr); got != wireErr {
		t.Fatalf("toWireError(direct) did not preserve pointer")
	}

	wrapped := errors.Join(errors.New("outer"), ErrMethodNotFound)
	got := toWireError(wrapped)
	if got.Code != ErrMethodNotFound.Code {
		t.Fatalf("Code = %d, want %d", got.Code, ErrMethodNotFound.Code)
	}
	if got.Message != wrapped.Error() {
		t.Fatalf("Message = %q, want %q", got.Message, wrapped.Error())
	}

	if !errors.Is(NewError(-10, "a"), NewError(-10, "b")) {
		t.Fatal("WireError.Is() did not match on code")
	}
	if ErrParse.Error() != "JSON RPC parse error" {
		t.Fatalf("ErrParse.Error() = %q", ErrParse.Error())
	}
}

func TestAsyncAndHandlerAdapters(t *testing.T) {
	a := newAsync()
	first := errors.New("first")
	second := errors.New("second")
	a.setError(first)
	a.setError(second)
	a.done()
	if err := a.wait(); !errors.Is(err, first) {
		t.Fatalf("wait() error = %v, want %v", err, first)
	}
	if err := a.wait(); !errors.Is(err, first) {
		t.Fatalf("second wait() error = %v, want %v", err, first)
	}

	pre := PreempterFunc(func(context.Context, *Request) (any, error) {
		return "ok", nil
	})
	result, err := pre.Preempt(context.Background(), &Request{})
	if err != nil || result != "ok" {
		t.Fatalf("Preempt() = (%v, %v), want (ok, nil)", result, err)
	}

	handler := HandlerFunc(func(context.Context, *Request) (any, error) {
		return 123, nil
	})
	result, err = handler.Handle(context.Background(), &Request{})
	if err != nil || result != 123 {
		t.Fatalf("Handle() = (%v, %v), want (123, nil)", result, err)
	}

	var def defaultHandler
	if _, err := def.Handle(context.Background(), &Request{}); !errors.Is(err, ErrNotHandled) {
		t.Fatalf("default Handle() error = %v, want ErrNotHandled", err)
	}
	if _, err := def.Preempt(context.Background(), &Request{}); !errors.Is(err, ErrNotHandled) {
		t.Fatalf("default Preempt() error = %v, want ErrNotHandled", err)
	}
}

func TestRawFramer(t *testing.T) {
	framer := RawFramer()
	var buf bytes.Buffer
	writer := framer.Writer(&buf)
	msg, err := NewNotification("workspace/didChange", map[string]any{"count": 1})
	if err != nil {
		t.Fatalf("NewNotification() error = %v", err)
	}
	if err := writer.Write(context.Background(), msg); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	reader := framer.Reader(&buf)
	read, err := reader.Read(context.Background())
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	req, ok := read.(*Request)
	if !ok || req.Method != "workspace/didChange" {
		t.Fatalf("Read() = %#v", read)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := writer.Write(ctx, msg); !errors.Is(err, context.Canceled) {
		t.Fatalf("Write(canceled) error = %v, want context.Canceled", err)
	}
	if _, err := reader.Read(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Read(canceled) error = %v, want context.Canceled", err)
	}
}

func TestHeaderFramer(t *testing.T) {
	framer := HeaderFramer()
	var buf bytes.Buffer
	writer := framer.Writer(&buf)
	msg, err := NewResponse(Int64ID(1), map[string]string{"status": "ok"}, nil)
	if err != nil {
		t.Fatalf("NewResponse() error = %v", err)
	}
	if err := writer.Write(context.Background(), msg); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if !strings.Contains(buf.String(), "Content-Length:") {
		t.Fatalf("header output missing Content-Length:\n%s", buf.String())
	}
	reader := framer.Reader(&buf)
	read, err := reader.Read(context.Background())
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	resp, ok := read.(*Response)
	if !ok || resp.ID.Raw() != int64(1) {
		t.Fatalf("Read() = %#v", read)
	}

	errorCases := []struct {
		name string
		data string
		want string
	}{
		{name: "missing length", data: "\r\n", want: "missing Content-Length header"},
		{name: "bad header", data: "oops\r\n\r\n", want: `invalid header line "oops"`},
		{name: "bad length", data: "Content-Length: nope\r\n\r\n", want: "failed parsing Content-Length"},
		{name: "zero length", data: "Content-Length: 0\r\n\r\n", want: "invalid Content-Length"},
	}
	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			reader := framer.Reader(strings.NewReader(tc.data))
			_, err := reader.Read(context.Background())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Read() error = %v, want %q", err, tc.want)
			}
		})
	}

	reader = framer.Reader(strings.NewReader(""))
	if _, err := reader.Read(context.Background()); !errors.Is(err, io.EOF) {
		t.Fatalf("Read(clean EOF) error = %v, want io.EOF", err)
	}
}
