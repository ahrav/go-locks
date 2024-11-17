package ticket

import (
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLockConcurrentAccess(t *testing.T) {
	lock := NewLock()
	const numGoroutines = 100
	const iterations = 500
	counter := 0
	var wg sync.WaitGroup

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for range iterations {
				lock.Lock()
				counter++
				lock.Unlock()
			}
		}()
	}
	wg.Wait()

	expected := numGoroutines * iterations
	assert.Equal(t, expected, counter, "Expected counter to be %d, got %d", expected, counter)
}

func TestLockFairness(t *testing.T) {
	lock := NewLock()
	const numGoroutines = 50

	// Track execution order and the head value at time of execution.
	type execution struct {
		goroutineID int
		headValue   uint32
	}
	var executions []execution
	var mutex sync.Mutex
	var wg sync.WaitGroup

	// Barrier to ensure all goroutines start competing for the lock simultaneously.
	var ready sync.WaitGroup
	ready.Add(1)

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			ready.Wait()

			// Acquire the lock - this will internally get a ticket.
			lock.Lock()

			// Record our execution with the current head value.
			mutex.Lock()
			executions = append(executions, execution{
				goroutineID: id,
				headValue:   atomic.LoadUint32(&lock.head),
			})
			mutex.Unlock()

			lock.Unlock()
		}(i)
	}

	ready.Done()
	wg.Wait()

	// Verify that head values are sequential.
	for i := 1; i < len(executions); i++ {
		assert.Equal(t,
			executions[i-1].headValue+1,
			executions[i].headValue,
			"Head values should be sequential. Execution order: %+v", executions)
	}
}

func TestLockStress(t *testing.T) {
	lock := NewLock()
	const numGoroutines = 10
	const iterations = 10000
	var wg sync.WaitGroup

	start := time.Now()
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				lock.Lock()
				time.Sleep(time.Microsecond)
				lock.Unlock()
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	assert.Less(t, duration, 5*time.Second, "Lock stress test took too long: %v", duration)
}

func TestSubAbs(t *testing.T) {
	tests := []struct {
		a, b     uint32
		expected uint32
	}{
		{0, 0, 0},
		{1, 1, 0},
		{10, 5, 5},
		{5, 10, 5},
		{math.MaxUint32, 0, math.MaxUint32},
		{0, math.MaxUint32, math.MaxUint32},
	}

	for _, tt := range tests {
		result := subAbs(tt.a, tt.b)
		assert.Equal(t, tt.expected, result, "subAbs(%d, %d) = %d; want %d", tt.a, tt.b, result, tt.expected)
	}
}

// BenchmarkMutexUncontended tests mutex performance with no contention
func BenchmarkMutexUncontended(b *testing.B) {
	var mu sync.Mutex
	for i := 0; i < b.N; i++ {
		mu.Lock()
		mu.Unlock()
	}
}

func BenchmarkMutexUncontendedParallel(b *testing.B) {
	var mu sync.Mutex
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mu.Lock()
			mu.Unlock()
		}
	})
}

// BenchmarkTicketLockUncontended tests ticket lock performance with no contention
func BenchmarkTicketLockUncontended(b *testing.B) {
	lock := NewLock()
	for i := 0; i < b.N; i++ {
		lock.Lock()
		lock.Unlock()
	}
}

func BenchmarkTicketLockUncontendedParallel(b *testing.B) {
	lock := NewLock()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			lock.Lock()
			lock.Unlock()
		}
	})
}

// BenchmarkMutexContended tests mutex performance under contention
func BenchmarkMutexContended(b *testing.B) {
	var mu sync.Mutex
	shared := 0
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mu.Lock()
			shared++
			mu.Unlock()
		}
	})
}

// BenchmarkTicketLockContended tests ticket lock performance under contention
func BenchmarkTicketLockContended(b *testing.B) {
	lock := NewLock()
	shared := 0
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			lock.Lock()
			shared++
			lock.Unlock()
		}
	})
}

func BenchmarkMutexHeavyContention(b *testing.B) {
	var mu sync.Mutex
	shared := 0
	for i := 0; i < b.N; i++ {
		mu.Lock()
		// Simulate some work inside critical section
		for i := 0; i < 100; i++ {
			shared++
		}
		mu.Unlock()
	}
}

// BenchmarkMutexHeavyContention simulates heavy contention with work inside critical section
func BenchmarkMutexHeavyContentionParallel(b *testing.B) {
	var mu sync.Mutex
	shared := 0
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mu.Lock()
			// Simulate some work inside critical section
			for i := 0; i < 100; i++ {
				shared++
			}
			mu.Unlock()
		}
	})
}

// BenchmarkTicketLockHeavyContention simulates heavy contention with work inside critical section
func BenchmarkTicketLockHeavyContention(b *testing.B) {
	lock := NewLock()
	shared := 0
	for i := 0; i < b.N; i++ {
		lock.Lock()
		// Simulate some work inside critical section
		for i := 0; i < 100; i++ {
			shared++
		}
		lock.Unlock()
	}
}

// BenchmarkTicketLockHeavyContention simulates heavy contention with work inside critical section
func BenchmarkTicketLockHeavyContentionParallel(b *testing.B) {
	lock := NewLock()
	shared := 0
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			lock.Lock()
			// Simulate some work inside critical section
			for i := 0; i < 100; i++ {
				shared++
			}
			lock.Unlock()
		}
	})
}

// BenchmarkMutexTryLock tests performance of try-lock pattern
func BenchmarkMutexTryLock(b *testing.B) {
	var mu sync.Mutex
	shared := 0
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if mu.TryLock() {
				shared++
				mu.Unlock()
			}
		}
	})
}

// BenchmarkTicketLockTryLock tests performance of try-lock pattern
func BenchmarkTicketLockTryLock(b *testing.B) {
	lock := NewLock()
	shared := 0
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if lock.TryLock() {
				shared++
				lock.Unlock()
			}
		}
	})
}
