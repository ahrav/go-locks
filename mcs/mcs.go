// Package mcs implements the Mellor-Crummey Scott (MCS) lock, a scalable FIFO queue-based spin lock.
//
// An MCS lock provides several advantages over traditional spin locks:
//   - FIFO ordering ensures fair lock acquisition
//   - Each thread spins on a local variable, reducing memory contention and cache invalidation
//   - Memory usage scales with the number of threads contending for the lock
//   - Predictable performance under high contention
//
// Example usage:
//
//	lock := mcs.NewLock()
//	node := &mcs.QNode{}
//
//	// Blocking acquisition
//	lock.Lock(node)
//	// ... critical section ...
//	lock.Unlock(node)
//
//	// Non-blocking try-lock
//	if lock.TryLock(node) {
//	    // ... critical section ...
//	    lock.Unlock(node)
//	}
//
// Each goroutine must maintain its own QNode instance. A single QNode should not be
// used concurrently by multiple goroutines. For scenarios requiring multiple locks,
// use NewLockArray and NewQNodeArray to efficiently manage multiple lock instances.
package mcs

import (
	"runtime"
	"sync/atomic"
)

// QNode represents a queue node in the MCS lock.
type QNode struct {
	next    atomic.Pointer[QNode]
	waiting uint32
}

// Lock represents the MCS lock.
type Lock struct {
	tail atomic.Pointer[QNode]
}

// NewLock creates a new MCS lock.
func NewLock() *Lock { return new(Lock) }

// TryLock attempts to acquire the lock without blocking.
// Returns true if lock was acquired, false otherwise.
func (l *Lock) TryLock(node *QNode) bool {
	node.next.Store(nil)
	return l.tail.CompareAndSwap(nil, node)
}

// Lock acquires the lock.
func (l *Lock) Lock(node *QNode) {
	node.next.Store(nil)
	pred := l.tail.Swap(node) // Atomically put ourselves at the tail

	if pred == nil { // No predecessor, lock acquired
		return
	}

	// Someone else is holding the lock, wait for predecessor to signal us.
	atomic.StoreUint32(&node.waiting, 1)
	pred.next.Store(node) // Link to predecessor

	// Spin until predecessor signals us.
	for atomic.LoadUint32(&node.waiting) != 0 {
		// Similar to PAUSE in the C version, not sure if this is correct?
		// Maybe just use a for loop?
		runtime.Gosched()
	}
}

// Unlock releases the lock.
func (l *Lock) Unlock(node *QNode) {
	// Check if there's a successor.
	if node.next.Load() == nil {
		// No one waiting? Try to set tail to nil.
		if l.tail.CompareAndSwap(node, nil) {
			return
		}

		// Someone in the process of enqueuing, wait for them.
		for {
			succ := node.next.Load()
			if succ != nil {
				atomic.StoreUint32(&succ.waiting, 0) // Signal successor
				return
			}
			runtime.Gosched()
		}
	}

	// Signal our successor.
	succ := node.next.Load()
	atomic.StoreUint32(&succ.waiting, 0)
}

// IsFree returns true if the lock is currently free.
func (l *Lock) IsFree() bool { return l.tail.Load() == nil }
