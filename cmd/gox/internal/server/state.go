package server

import (
	"context"
	"sync"

	"github.com/doors-dev/gox/internal/lsp"
)

type responseState struct {
	mu       sync.Mutex
	counter  int64
	requests map[int64]lsp.Callback
}

func newResponseState() *responseState {
	return &responseState{
		requests: make(map[int64]lsp.Callback),
	}
}

func (s *responseState) respond(id int64, r lsp.Response) bool {
	s.mu.Lock()
	callback, ok := s.requests[id]
	if !ok {
		s.mu.Unlock()
		return false
	}
	delete(s.requests, id)
	s.mu.Unlock()
	callback(r)
	return true
}

func (s *responseState) cancelAll() {
	s.mu.Lock()
	callbacks := make([]lsp.Callback, 0, len(s.requests))
	for _, r := range s.requests {
		callbacks = append(callbacks, r)
	}
	s.requests = make(map[int64]lsp.Callback)
	s.mu.Unlock()
	for _, callback := range callbacks {
		callback(lsp.Response{Err: context.Canceled})
	}
}

func (s *responseState) issueRequest(callback lsp.Callback) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counter += 1
	s.requests[s.counter] = callback
	return s.counter
}
