package datadash

import (
	"math"
	"sort"
	"sync"
	"sync/atomic"
)

// Ring is a fixed-capacity circular buffer for streaming time-series data.
// Write operations use atomic counters under a write lock for safety;
// read operations take a shared lock so multiple consumers can snapshot
// concurrently without blocking each other.
type Ring struct {
	buf []float64
	cap int64

	// pos is the next write index (monotonically increasing, mod cap for slot).
	// len is the current fill level (clamped to cap).
	// Both use atomics so Len() can be called lock-free.
	pos atomic.Int64
	len atomic.Int64

	mu sync.RWMutex
}

// NewRing allocates a ring buffer that holds at most capacity values.
// If capacity <= 0 it defaults to 4096.
func NewRing(capacity int) *Ring {
	if capacity <= 0 {
		capacity = 4096
	}
	return &Ring{
		buf: make([]float64, capacity),
		cap: int64(capacity),
	}
}

// Push appends a single value, overwriting the oldest if full.
func (r *Ring) Push(v float64) {
	r.mu.Lock()
	idx := r.pos.Load() % r.cap
	r.buf[idx] = v
	r.pos.Add(1)
	if n := r.len.Load(); n < r.cap {
		r.len.Add(1)
	}
	r.mu.Unlock()
}

// PushBatch appends multiple values in a single lock acquisition.
func (r *Ring) PushBatch(values []float64) {
	if len(values) == 0 {
		return
	}
	r.mu.Lock()
	for _, v := range values {
		idx := r.pos.Load() % r.cap
		r.buf[idx] = v
		r.pos.Add(1)
		if n := r.len.Load(); n < r.cap {
			r.len.Add(1)
		}
	}
	r.mu.Unlock()
}

// Values returns every element in chronological order (oldest first).
// The caller owns the returned slice.
func (r *Ring) Values() []float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.snapshot(int(r.len.Load()))
}

// Last returns the most recent n values in chronological order.
// If n exceeds the current fill, all values are returned.
func (r *Ring) Last(n int) []float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	total := int(r.len.Load())
	if n > total {
		n = total
	}
	return r.snapshot(n)
}

// snapshot copies n most recent elements into a new slice (caller holds RLock).
func (r *Ring) snapshot(n int) []float64 {
	if n <= 0 {
		return nil
	}
	out := make([]float64, n)
	pos := r.pos.Load()
	start := pos - int64(n)
	for i := 0; i < n; i++ {
		out[i] = r.buf[(start+int64(i)+r.cap*2)%r.cap] // +cap*2 avoids negative mod
	}
	return out
}

// Len returns the number of elements currently in the buffer. Lock-free.
func (r *Ring) Len() int { return int(r.len.Load()) }

// Cap returns the maximum capacity.
func (r *Ring) Cap() int { return int(r.cap) }

// Position returns the total number of values ever pushed to the ring
// (monotonically increasing — never wraps within practical lifetimes).
// Lock-free.
func (r *Ring) Position() int64 { return r.pos.Load() }

// Since returns every value written at or after the absolute position
// `since`, in chronological order, along with the new high-water mark
// to pass on the next call.
//
// If `since` is older than the oldest value still in the buffer,
// the result starts from the oldest available value (i.e. `since` is
// silently advanced to `pos - len`). If no new values are available,
// returns (nil, currentPos).
//
// This is the right tool for streaming consumers (sparklines, log
// tails, etc.) that need to receive every value exactly once even as
// the ring overflows.
func (r *Ring) Since(since int64) (values []float64, pos int64) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	pos = r.pos.Load()
	n := int64(r.len.Load())
	if n == 0 || since >= pos {
		return nil, pos
	}

	oldest := pos - n
	if since < oldest {
		since = oldest
	}

	count := pos - since
	out := make([]float64, count)
	for i := int64(0); i < count; i++ {
		out[i] = r.buf[(since+i)%r.cap]
	}
	return out, pos
}

// Reset empties the buffer.
func (r *Ring) Reset() {
	r.mu.Lock()
	r.pos.Store(0)
	r.len.Store(0)
	r.mu.Unlock()
}

// -------------------------------------------------------------------
// Stats
// -------------------------------------------------------------------

// Stats holds summary statistics for the buffer contents.
type Stats struct {
	Min, Max, Mean, StdDev float64
	P50, P95, P99          float64
	Count                  int
}

// Stats computes summary statistics over the current buffer contents.
// Returns a zero Stats if the buffer is empty.
func (r *Ring) Stats() Stats {
	vals := r.Values()
	if len(vals) == 0 {
		return Stats{}
	}
	return computeStats(vals)
}

// StatsLast computes summary statistics over the most recent n values.
func (r *Ring) StatsLast(n int) Stats {
	vals := r.Last(n)
	if len(vals) == 0 {
		return Stats{}
	}
	return computeStats(vals)
}

func computeStats(vals []float64) Stats {
	n := len(vals)
	s := Stats{
		Min:   vals[0],
		Max:   vals[0],
		Count: n,
	}

	sum := 0.0
	for _, v := range vals {
		if v < s.Min {
			s.Min = v
		}
		if v > s.Max {
			s.Max = v
		}
		sum += v
	}
	s.Mean = sum / float64(n)

	// Standard deviation (population)
	sumSq := 0.0
	for _, v := range vals {
		d := v - s.Mean
		sumSq += d * d
	}
	s.StdDev = math.Sqrt(sumSq / float64(n))

	// Percentiles — sort a copy
	sorted := make([]float64, n)
	copy(sorted, vals)
	sort.Float64s(sorted)

	s.P50 = percentile(sorted, 50)
	s.P95 = percentile(sorted, 95)
	s.P99 = percentile(sorted, 99)

	return s
}

// percentile uses linear interpolation on a pre-sorted slice.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	rank := p / 100.0 * float64(len(sorted)-1)
	lo := int(math.Floor(rank))
	hi := lo + 1
	if hi >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	frac := rank - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}