// Package alock implements an array-based lock, providing fair mutual exclusion for a fixed number
// of goroutines. The ArrayLock type uses an array of flags to coordinate lock acquisition between
// goroutines, ensuring FIFO ordering by maintaining a circular queue.
//
// The array-based lock provides several benefits:
//   - Fair scheduling with FIFO ordering of lock acquisition
//   - Bounded memory usage based on the number of goroutines
//   - Each goroutine spins on its own dedicated flag, reducing contention
//
// Example usage:
//
//	lock := alock.NewArrayLock(4) // Support up to 4 goroutines
//
//	// Blocking acquisition
//	lock.Lock()
//	// ... critical section ...
//	lock.Unlock()
//
//	// Non-blocking try-lock
//	if lock.TryLock() {
//	    // ... critical section ...
//	    lock.Unlock()
//	}
//
// The number of goroutines must be known in advance and should match the maximum number
// of goroutines that will contend for the lock. Using more goroutines than specified
// will cause them to share slots, potentially leading to unfair scheduling.
package alock

import (
	"runtime"
	"sync/atomic"
)

// Share manages a shared lock among multiple goroutines.
type Share struct {
	flags []uint32 // Array of flags to indicate whether a goroutine can acquire the lock
	tail  uint32   // Atomic index to assign slots to incoming goroutines
	size  uint32   // Size of the flags array (number of goroutines)
}

// ArrayLock manages a local lock for each goroutine.
type ArrayLock struct {
	share   *Share
	myIndex uint32
}

// NewArrayLock initializes a new array lock with the given number of goroutines.
func NewArrayLock(numGoroutines uint32) *ArrayLock {
	share := &Share{
		size:  numGoroutines,
		tail:  0,
		flags: make([]uint32, numGoroutines),
	}
	share.flags[0] = 1 // Set the first flag to 1 to allow the first goroutine to acquire the lock

	return &ArrayLock{share: share}
}

// Lock attempts to acquire the lock for the current goroutine.
func (al *ArrayLock) Lock() {
	lock := al.share
	// Atomically increment the tail and determine the slot for the current goroutine.
	slot := atomic.AddUint32(&lock.tail, 1) % lock.size
	al.myIndex = slot

	// Spin until the flag for this slot is set to 1.
	for atomic.LoadUint32(&lock.flags[slot]) == 0 {
		// Yield to allow other goroutines to run, not sure if this is the best approach.
		runtime.Gosched()
	}
}

// Unlock releases the lock, allowing the next goroutine in the queue to acquire it.
func (al *ArrayLock) Unlock() {
	lock := al.share
	slot := al.myIndex

	// Set the current slot's flag to 0 to indicate release.
	atomic.StoreUint32(&lock.flags[slot], 0)

	// Set the next slot's flag to 1 to allow the next goroutine to acquire the lock.
	nextSlot := (slot + 1) % lock.size
	atomic.StoreUint32(&lock.flags[nextSlot], 1)
}

// TryLock attempts to acquire the lock without blocking. Returns true if successful.
func (al *ArrayLock) TryLock() bool {
	lock := al.share
	tail := atomic.LoadUint32(&lock.tail)
	if atomic.LoadUint32(&lock.flags[tail%lock.size]) == 1 {
		if atomic.CompareAndSwapUint32(&lock.tail, tail, tail+1) {
			al.myIndex = tail % lock.size
			return true
		}
	}
	return false
}
