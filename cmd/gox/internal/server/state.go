package server

import (
	"context"
	"sync"

	"github.com/doors-dev/gox/internal/lsp"
)

type pendingRequest struct {
	callback lsp.Callback
	origin   lsp.Origin
}

type responseState struct {
	mu       sync.Mutex
	counter  int64
	requests map[int64]pendingRequest
	origins  map[lsp.Origin]int64
}

func newResponseState() *responseState {
	return &responseState{
		requests: make(map[int64]pendingRequest),
		origins:  make(map[lsp.Origin]int64),
	}
}

func (s *responseState) respond(id int64, r lsp.Response) bool {
	s.mu.Lock()
	pending, ok := s.requests[id]
	if !ok {
		s.mu.Unlock()
		return false
	}
	delete(s.requests, id)
	if pending.origin.ID.IsValid() {
		delete(s.origins, pending.origin)
	}
	s.mu.Unlock()
	pending.callback(r)
	return true
}

func (s *responseState) cancelAll() {
	s.mu.Lock()
	callbacks := make([]lsp.Callback, 0, len(s.requests))
	for _, pending := range s.requests {
		callbacks = append(callbacks, pending.callback)
	}
	s.requests = make(map[int64]pendingRequest)
	s.origins = make(map[lsp.Origin]int64)
	s.mu.Unlock()
	for _, callback := range callbacks {
		callback(lsp.Response{Err: context.Canceled})
	}
}

func (s *responseState) issueRequest(callback lsp.Callback, origin lsp.Origin) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counter += 1
	s.requests[s.counter] = pendingRequest{callback: callback, origin: origin}
	if origin.ID.IsValid() {
		s.origins[origin] = s.counter
	}
	return s.counter
}

func (s *responseState) cancelTarget(origin lsp.Origin) (int64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.origins[origin]
	return id, ok
}
