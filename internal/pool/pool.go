// Package pool provides a generic object pool that automatically resets
// objects before they are returned to the pool.
package pool

import (
	"sync"
)

// Resetter is the interface that wraps the Reset method.
// Types that can be reused in the pool must implement this method
// to clear their internal state.
type Resetter interface {
	Reset()
}

// Pool is a generic object pool. It holds objects of type T,
// where T must satisfy the Resetter interface.
//
// The pool is safe for concurrent use by multiple goroutines.
type Pool[T Resetter] struct {
	newFunc func() T
	pool    sync.Pool
}

// New creates a new Pool that uses the provided newFunc to create
// new instances of T when the pool is empty.
//
// Example:
//
//	p := New(func() *MyStruct { return &MyStruct{} })
//	obj := p.Get()
//	defer p.Put(obj)
func New[T Resetter](newFunc func() T) *Pool[T] {
	p := &Pool[T]{
		newFunc: newFunc,
	}
	p.pool.New = func() interface{} {
		return newFunc()
	}
	return p
}

// Get retrieves an object from the pool. It may be either a newly
// allocated object (using the factory function) or a previously
// recycled one. The caller is responsible for using the object
// and then returning it via Put.
func (p *Pool[T]) Get() T {
	return p.pool.Get().(T)
}

// Put returns an object to the pool. Before storing it, the method
// calls the object's Reset() to clear its state, ensuring that the
// next Get receives a clean object.
func (p *Pool[T]) Put(x T) {
	x.Reset()
	p.pool.Put(x)
}
