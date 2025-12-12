package server

import (
	"context"

	"github.com/doors-dev/gox/internal/lsp"
)

type responseState struct {
	counter  int64
	requests map[int64]lsp.Callback
}

func newResponseState() *responseState {
	return &responseState{
		requests: make(map[int64]lsp.Callback),
	}
}

func (s *responseState) respond(id int64, r lsp.Response) bool {
	callback, ok := s.requests[id]
	if !ok {
		return false
	}
	delete(s.requests, id)
	callback(r)
	return true
}

func (s *responseState) cancelAll() {
	for _, r := range s.requests {
		r(lsp.Response{Err: context.Canceled})
	}
	s.requests = make(map[int64]lsp.Callback)
}

func (s *responseState) issueRequest(callback lsp.Callback) int64 {
	s.counter += 1
	s.requests[s.counter] = callback
	return s.counter
}
