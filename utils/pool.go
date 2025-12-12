package utils

import "sync"

func NewPool[T any](new func() T) *Pool[T] {
	return &Pool[T]{
		pool: sync.Pool{
			New: func() any {
				return new()
			},
		},
	}
}

type Pool[T any] struct {
	pool sync.Pool
}

func (p *Pool[T]) Get() T {
	return p.pool.Get().(T)
}

func (p *Pool[T]) Put(v T) {
	p.pool.Put(v)
}

func NewStructPool[T any]() *Pool[*T] {
	return NewPool(func() *T {
		var v T
		return &v
	})
}
