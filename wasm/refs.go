package mintlayer

import (
	"sync"
	"sync/atomic"
)

// refs is a global registry mapping opaque uintptr keys to Go values.
// It is used to pass Go objects to WASM as "externref" values.
// Keys are sequential integers, NOT Go pointers, so they are safe across GC cycles.
var refs = &refStore{m: make(map[uintptr]interface{}), next: 3}

// Pre-allocated keys for sentinel values that WASM code always needs.
const (
	refGlobal uintptr = 1 // "globalThis" object
	refCrypto uintptr = 2 // "crypto" object
)

func init() {
	refs.m[refGlobal] = globalSentinel{}
	refs.m[refCrypto] = cryptoSentinel{}
}

type refStore struct {
	mu   sync.RWMutex
	m    map[uintptr]interface{}
	next uintptr
}

// alloc stores a value in the registry and returns its key.
func (r *refStore) alloc(v interface{}) uintptr {
	r.mu.Lock()
	// next is always >= 3 so it can't collide with pre-allocated sentinels.
	key := atomic.AddUintptr(&r.next, 1)
	r.m[key] = v
	r.mu.Unlock()
	return key
}

// get retrieves the value for a key.
func (r *refStore) get(key uintptr) (interface{}, bool) {
	r.mu.RLock()
	v, ok := r.m[key]
	r.mu.RUnlock()
	return v, ok
}

// free removes a key from the registry.
func (r *refStore) free(key uintptr) {
	r.mu.Lock()
	delete(r.m, key)
	r.mu.Unlock()
}
