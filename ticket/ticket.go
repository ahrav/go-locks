// Package ticket provides a fair mutual exclusion lock implementation using a ticket-based
// queuing system. The TicketLock type ensures FIFO ordering of lock acquisition by
// maintaining a queue of waiting goroutines using ticket numbers. This provides fairness
// by serving lock requests in the exact order they arrive, while implementing adaptive
// spinning strategies to balance CPU utilization with latency.
package ticket

import (
	"sync/atomic"
	"time"
	"unsafe"
)

// Lock implements a fair mutual exclusion lock using a ticket-based queuing system.
// The lock maintains a queue of waiting goroutines using ticket numbers, ensuring FIFO
// ordering of lock acquisition. This provides fairness by serving lock requests in the
// exact order they arrive.
//
// The internal implementation uses two counters:
// - head: represents the currently served ticket number
// - tail: represents the next available ticket number
//
// The lock is free when head == tail+1, and locked otherwise.
// The struct is carefully laid out to ensure proper alignment on 32-bit platforms.
type Lock struct {
	head uint32 // Current ticket being served
	tail uint32 // Next ticket to be issued
}

// NewLock creates a new TicketLock.
func NewLock() *Lock { return &Lock{head: 1, tail: 0} }

// TryLock attempts to acquire the lock without blocking. It returns true if the lock
// was acquired successfully, and false if the lock is currently held by another goroutine.
// This method provides a way to avoid blocking when the lock is unavailable.
func (t *Lock) TryLock() bool {
	me := t.tail
	meNew := me + 1
	return atomic.CompareAndSwapUint64(
		(*uint64)(unsafe.Pointer(t)),
		uint64(me+1)<<32|uint64(me),    // Expected: head should be tail+1 for lock to be free
		uint64(me+1)<<32|uint64(meNew), // New: keep head same, increment tail
	)
}

const (
	ticketBaseWait uint32 = 10
	ticketWaitNext        = 5
)

// Lock acquires the lock using a ticket-based queuing system. It implements an adaptive
// spinning strategy where goroutines wait proportionally to their distance from the head
// of the queue. When a goroutine is far back in the queue (>20 positions), it will sleep
// rather than spin to reduce CPU usage. This provides fair ordering of lock acquisition
// while attempting to balance CPU utilization with latency.
func (t *Lock) Lock() {
	myTicket := atomic.AddUint32(&t.tail, 1) // Get our ticket

	// Fast path for uncontended case
	cur := atomic.LoadUint32(&t.head)
	if cur == myTicket {
		return // No spinning needed if we get the lock immediately
	}

	wait := ticketBaseWait
	distancePrev := uint32(1)

	// Spin until it's our turn.
	for {
		// Determine who's turn it is.
		cur := atomic.LoadUint32(&t.head)
		if cur == myTicket {
			break // Yay! It's our turn
		}
		distance := subAbs(cur, myTicket) // How many people are in front of us?

		if distance > 1 { // If there are people in front of us, wait
			if distance != distancePrev { // If the distance has changed, reset the wait time
				distancePrev = distance
				wait = ticketBaseWait
			}

			// Spin proportionally to the distance from the head.
			// Further back = more iterations of Gosched.
			for range distance * wait {
				// Empty spin loop.
			}
		} else { // If we're next in line, wait a little bit
			for range ticketWaitNext {
				// Empty spin loop.
			}
		}

		if distance > 20 { // Sleep if we're far back in the queue
			time.Sleep(time.Millisecond)
		}
	}
}

// Unlock releases the lock.
func (t *Lock) Unlock() { atomic.AddUint32(&t.head, 1) }

// isFree checks if the lock is free.
func (t *Lock) isFree() bool { return (t.head - t.tail) == 1 }

func subAbs(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}
