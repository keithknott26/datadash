package datadash

import (
	"math"
	"sync"
	"testing"
)

// =========================================================================
// Position & Since — streaming consumer API
// =========================================================================

func TestPosition_StartsAtZero(t *testing.T) {
	r := NewRing(10)
	if r.Position() != 0 {
		t.Errorf("Position() on fresh ring = %d, want 0", r.Position())
	}
}

func TestPosition_IncrementsOnPush(t *testing.T) {
	r := NewRing(10)
	for i := 1; i <= 5; i++ {
		r.Push(float64(i))
		if got := r.Position(); got != int64(i) {
			t.Errorf("after %d pushes, Position() = %d, want %d", i, got, i)
		}
	}
}

func TestPosition_KeepsCountingPastCapacity(t *testing.T) {
	// Position is monotonic — it doesn't wrap when the ring overflows.
	r := NewRing(3)
	for i := 0; i < 100; i++ {
		r.Push(float64(i))
	}
	if got := r.Position(); got != 100 {
		t.Errorf("Position() = %d, want 100 (monotonic)", got)
	}
	if got := r.Len(); got != 3 {
		t.Errorf("Len() = %d, want 3 (capped)", got)
	}
}

func TestSince_EmptyRing(t *testing.T) {
	r := NewRing(10)
	values, pos := r.Since(0)
	if values != nil {
		t.Errorf("Since(0) on empty = %v, want nil", values)
	}
	if pos != 0 {
		t.Errorf("Since(0) pos = %d, want 0", pos)
	}
}

func TestSince_FromZero(t *testing.T) {
	r := NewRing(10)
	for i := 1; i <= 5; i++ {
		r.Push(float64(i))
	}
	values, pos := r.Since(0)
	if !equalFloats(values, []float64{1, 2, 3, 4, 5}) {
		t.Errorf("Since(0) = %v, want [1 2 3 4 5]", values)
	}
	if pos != 5 {
		t.Errorf("pos = %d, want 5", pos)
	}
}

func TestSince_Resumable(t *testing.T) {
	// Simulates a streaming consumer that resumes from where it left off.
	r := NewRing(100)

	for i := 1; i <= 5; i++ {
		r.Push(float64(i))
	}
	v1, p1 := r.Since(0)
	if !equalFloats(v1, []float64{1, 2, 3, 4, 5}) {
		t.Fatalf("first batch = %v", v1)
	}

	for i := 6; i <= 8; i++ {
		r.Push(float64(i))
	}
	v2, p2 := r.Since(p1)
	if !equalFloats(v2, []float64{6, 7, 8}) {
		t.Errorf("second batch = %v, want [6 7 8]", v2)
	}
	if p2 != 8 {
		t.Errorf("second pos = %d, want 8", p2)
	}

	// Calling Since again with the latest pos returns nothing.
	v3, p3 := r.Since(p2)
	if v3 != nil {
		t.Errorf("third call = %v, want nil", v3)
	}
	if p3 != 8 {
		t.Errorf("third pos = %d, want 8", p3)
	}
}

func TestSince_KeepsEmittingAfterOverflow(t *testing.T) {
	// Regression test for the sparkline-stops-after-a-while bug.
	// Once the ring fills, length-based tracking would plateau.
	// Position-based tracking via Since must keep emitting deltas.
	r := NewRing(10) // tiny capacity to overflow quickly

	var consumed []float64
	since := int64(0)

	// Push 50 values in batches of 5, draining via Since each time.
	for batch := 0; batch < 10; batch++ {
		for i := 0; i < 5; i++ {
			r.Push(float64(batch*5 + i))
		}
		got, newPos := r.Since(since)
		consumed = append(consumed, got...)
		since = newPos
	}

	// We should have received all 50 values, despite the ring's
	// capacity being only 10.
	if len(consumed) != 50 {
		t.Errorf("consumed %d values, want 50", len(consumed))
	}
	for i, v := range consumed {
		if v != float64(i) {
			t.Errorf("position %d: got %v, want %v", i, v, float64(i))
			break
		}
	}
}

func TestSince_HandlesLostHistory(t *testing.T) {
	// If `since` is older than what the ring still holds, it should
	// silently advance to the oldest available rather than panicking
	// or returning negative-length data.
	r := NewRing(5)
	for i := 1; i <= 100; i++ {
		r.Push(float64(i))
	}
	// since=0 is way older than what's still in the ring (positions 95–99).
	values, pos := r.Since(0)

	if pos != 100 {
		t.Errorf("pos = %d, want 100", pos)
	}
	// We get the most recent 5 values that survived the overflow.
	if !equalFloats(values, []float64{96, 97, 98, 99, 100}) {
		t.Errorf("values = %v, want [96 97 98 99 100]", values)
	}
}

func TestSince_NoNewData(t *testing.T) {
	r := NewRing(10)
	for i := 1; i <= 3; i++ {
		r.Push(float64(i))
	}
	values, pos := r.Since(3)
	if values != nil {
		t.Errorf("Since(currentPos) = %v, want nil", values)
	}
	if pos != 3 {
		t.Errorf("pos = %d, want 3", pos)
	}
}

func TestSince_ConcurrentReaders(t *testing.T) {
	// Verify that concurrent Push + Since calls don't race or panic.
	// We don't assert exact counts because the producer may outrun
	// the consumers — that's tested by TestSince_KeepsEmittingAfterOverflow.
	r := NewRing(1000)
	const total = 5000

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < total; i++ {
			r.Push(float64(i))
		}
	}()

	consumer := func() int {
		var since int64
		count := 0
		// Bounded iterations — not waiting for a count target.
		for i := 0; i < 500; i++ {
			values, pos := r.Since(since)
			count += len(values)
			since = pos
		}
		return count
	}

	results := make(chan int, 3)
	for i := 0; i < 3; i++ {
		go func() { results <- consumer() }()
	}

	wg.Wait()
	for i := 0; i < 3; i++ {
		<-results // just verify they all finish without hanging or panicking
	}
}

// =========================================================================
// NewRing
// =========================================================================

func TestNewRing_DefaultCapacity(t *testing.T) {
	tests := []struct {
		name string
		cap  int
		want int
	}{
		{"zero defaults to 4096", 0, 4096},
		{"negative defaults to 4096", -10, 4096},
		{"positive uses given value", 100, 100},
		{"one is fine", 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRing(tt.cap)
			if r.Cap() != tt.want {
				t.Errorf("Cap() = %d, want %d", r.Cap(), tt.want)
			}
			if r.Len() != 0 {
				t.Errorf("Len() = %d, want 0 for fresh ring", r.Len())
			}
		})
	}
}

// =========================================================================
// Push
// =========================================================================

func TestPush_SingleValue(t *testing.T) {
	r := NewRing(10)
	r.Push(42.5)
	if r.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", r.Len())
	}
	got := r.Values()
	if len(got) != 1 || got[0] != 42.5 {
		t.Errorf("Values() = %v, want [42.5]", got)
	}
}

func TestPush_FillsBuffer(t *testing.T) {
	r := NewRing(5)
	for i := 0; i < 5; i++ {
		r.Push(float64(i))
	}
	if r.Len() != 5 {
		t.Fatalf("Len() = %d, want 5", r.Len())
	}
	got := r.Values()
	want := []float64{0, 1, 2, 3, 4}
	if !equalFloats(got, want) {
		t.Errorf("Values() = %v, want %v", got, want)
	}
}

func TestPush_Overflow(t *testing.T) {
	r := NewRing(3)
	for i := 0; i < 10; i++ {
		r.Push(float64(i))
	}
	if r.Len() != 3 {
		t.Fatalf("Len() = %d, want 3 after overflow", r.Len())
	}
	got := r.Values()
	want := []float64{7, 8, 9}
	if !equalFloats(got, want) {
		t.Errorf("Values() = %v, want %v (last 3)", got, want)
	}
}

func TestPush_LargeOverflow(t *testing.T) {
	// Push way more than capacity to ensure modular arithmetic stays sane.
	r := NewRing(4)
	for i := 0; i < 1000; i++ {
		r.Push(float64(i))
	}
	got := r.Values()
	want := []float64{996, 997, 998, 999}
	if !equalFloats(got, want) {
		t.Errorf("Values() = %v, want %v", got, want)
	}
}

// =========================================================================
// PushBatch
// =========================================================================

func TestPushBatch_Empty(t *testing.T) {
	r := NewRing(5)
	r.PushBatch(nil)
	r.PushBatch([]float64{})
	if r.Len() != 0 {
		t.Errorf("Len() = %d, want 0", r.Len())
	}
}

func TestPushBatch_WithinCapacity(t *testing.T) {
	r := NewRing(10)
	r.PushBatch([]float64{1, 2, 3, 4})
	if r.Len() != 4 {
		t.Fatalf("Len() = %d, want 4", r.Len())
	}
	if got := r.Values(); !equalFloats(got, []float64{1, 2, 3, 4}) {
		t.Errorf("Values() = %v", got)
	}
}

func TestPushBatch_Overflow(t *testing.T) {
	r := NewRing(3)
	r.PushBatch([]float64{1, 2, 3, 4, 5, 6, 7})
	if got := r.Values(); !equalFloats(got, []float64{5, 6, 7}) {
		t.Errorf("Values() = %v, want [5 6 7]", got)
	}
}

// =========================================================================
// Values & Last
// =========================================================================

func TestValues_Empty(t *testing.T) {
	r := NewRing(5)
	if got := r.Values(); got != nil {
		t.Errorf("Values() = %v, want nil for empty ring", got)
	}
}

func TestLast_Variations(t *testing.T) {
	r := NewRing(10)
	for i := 1; i <= 5; i++ {
		r.Push(float64(i))
	}

	tests := []struct {
		n    int
		want []float64
	}{
		{0, nil},
		{1, []float64{5}},
		{3, []float64{3, 4, 5}},
		{5, []float64{1, 2, 3, 4, 5}},
		{10, []float64{1, 2, 3, 4, 5}}, // n > total → all values
		{100, []float64{1, 2, 3, 4, 5}},
	}
	for _, tt := range tests {
		got := r.Last(tt.n)
		if !equalFloats(got, tt.want) {
			t.Errorf("Last(%d) = %v, want %v", tt.n, got, tt.want)
		}
	}
}

func TestLast_EmptyRing(t *testing.T) {
	r := NewRing(5)
	if got := r.Last(3); got != nil {
		t.Errorf("Last on empty ring = %v, want nil", got)
	}
}

func TestSnapshot_NegativeMod(t *testing.T) {
	// Specifically exercise the negative-modulo branch in snapshot().
	// After overflow, the start index can be conceptually "negative"
	// before wrapping; the +cap*2 in snapshot guards against this.
	r := NewRing(5)
	for i := 0; i < 8; i++ {
		r.Push(float64(i))
	}
	got := r.Values()
	want := []float64{3, 4, 5, 6, 7}
	if !equalFloats(got, want) {
		t.Errorf("Values() after wrap = %v, want %v", got, want)
	}
}

// =========================================================================
// Reset
// =========================================================================

func TestReset(t *testing.T) {
	r := NewRing(5)
	for i := 0; i < 3; i++ {
		r.Push(float64(i))
	}
	r.Reset()
	if r.Len() != 0 {
		t.Errorf("Len() after Reset = %d, want 0", r.Len())
	}
	if got := r.Values(); got != nil {
		t.Errorf("Values() after Reset = %v, want nil", got)
	}

	// Should still be usable after reset.
	r.Push(99)
	if r.Len() != 1 || r.Values()[0] != 99 {
		t.Errorf("ring not usable after Reset")
	}
}

func TestReset_Empty(t *testing.T) {
	r := NewRing(5)
	r.Reset() // no-op on empty ring shouldn't panic
	if r.Len() != 0 {
		t.Errorf("Len() = %d, want 0", r.Len())
	}
}

// =========================================================================
// Stats
// =========================================================================

func TestStats_Empty(t *testing.T) {
	r := NewRing(5)
	s := r.Stats()
	if s != (Stats{}) {
		t.Errorf("Stats() on empty ring = %+v, want zero", s)
	}
}

func TestStats_SingleValue(t *testing.T) {
	r := NewRing(5)
	r.Push(42)
	s := r.Stats()
	if s.Min != 42 || s.Max != 42 || s.Mean != 42 {
		t.Errorf("single-value stats incorrect: %+v", s)
	}
	if s.StdDev != 0 {
		t.Errorf("StdDev for single value = %v, want 0", s.StdDev)
	}
	if s.Count != 1 {
		t.Errorf("Count = %d, want 1", s.Count)
	}
	if s.P50 != 42 || s.P95 != 42 || s.P99 != 42 {
		t.Errorf("percentiles for single value should all be 42, got %+v", s)
	}
}

func TestStats_KnownValues(t *testing.T) {
	// Values 1..10 — easy to verify by hand.
	r := NewRing(10)
	for i := 1; i <= 10; i++ {
		r.Push(float64(i))
	}
	s := r.Stats()

	checks := []struct {
		name    string
		got     float64
		want    float64
		epsilon float64
	}{
		{"Min", s.Min, 1, 0},
		{"Max", s.Max, 10, 0},
		{"Mean", s.Mean, 5.5, 1e-9},
		{"StdDev", s.StdDev, math.Sqrt(8.25), 1e-9},
		{"P50", s.P50, 5.5, 1e-9},
	}
	for _, c := range checks {
		if math.Abs(c.got-c.want) > c.epsilon {
			t.Errorf("%s = %v, want %v (±%v)", c.name, c.got, c.want, c.epsilon)
		}
	}
	if s.Count != 10 {
		t.Errorf("Count = %d, want 10", s.Count)
	}
}

func TestStats_Percentiles(t *testing.T) {
	r := NewRing(100)
	for i := 1; i <= 100; i++ {
		r.Push(float64(i))
	}
	s := r.Stats()

	// Percentiles use linear interpolation on a sorted slice indexed 0..99.
	// rank = p/100 * (n-1) = p/100 * 99 → P50 = 50.5, P95 = 95.05, P99 = 99.01.
	if math.Abs(s.P50-50.5) > 1e-9 {
		t.Errorf("P50 = %v, want 50.5", s.P50)
	}
	if math.Abs(s.P95-95.05) > 1e-9 {
		t.Errorf("P95 = %v, want 95.05", s.P95)
	}
	if math.Abs(s.P99-99.01) > 1e-9 {
		t.Errorf("P99 = %v, want 99.01", s.P99)
	}
}

func TestStats_NegativeValues(t *testing.T) {
	r := NewRing(5)
	for _, v := range []float64{-5, -3, -1, 1, 3} {
		r.Push(v)
	}
	s := r.Stats()
	if s.Min != -5 || s.Max != 3 {
		t.Errorf("min/max with negatives wrong: min=%v max=%v", s.Min, s.Max)
	}
	if math.Abs(s.Mean-(-1)) > 1e-9 {
		t.Errorf("Mean = %v, want -1", s.Mean)
	}
}

func TestStatsLast(t *testing.T) {
	r := NewRing(20)
	for i := 1; i <= 10; i++ {
		r.Push(float64(i))
	}

	s := r.StatsLast(3) // values 8, 9, 10
	if s.Count != 3 {
		t.Errorf("Count = %d, want 3", s.Count)
	}
	if s.Min != 8 || s.Max != 10 || math.Abs(s.Mean-9) > 1e-9 {
		t.Errorf("StatsLast(3) wrong: %+v", s)
	}
}

func TestStatsLast_Empty(t *testing.T) {
	r := NewRing(5)
	if got := r.StatsLast(3); got != (Stats{}) {
		t.Errorf("StatsLast on empty ring = %+v, want zero", got)
	}
}

// =========================================================================
// percentile (private — exercised via Stats but also direct edge cases)
// =========================================================================

func TestPercentile_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		sorted []float64
		p      float64
		want   float64
	}{
		{"empty", []float64{}, 50, 0},
		{"single", []float64{42}, 0, 42},
		{"single 100", []float64{42}, 100, 42},
		{"two values 0%", []float64{10, 20}, 0, 10},
		{"two values 100%", []float64{10, 20}, 100, 20},
		{"two values 50%", []float64{10, 20}, 50, 15},
		{"clamp above 100", []float64{1, 2, 3}, 150, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := percentile(tt.sorted, tt.p)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("percentile(%v, %v) = %v, want %v", tt.sorted, tt.p, got, tt.want)
			}
		})
	}
}

// =========================================================================
// Concurrency — run with -race
// =========================================================================

func TestRing_ConcurrentAccess(t *testing.T) {
	r := NewRing(1000)
	var wg sync.WaitGroup

	// 4 writer goroutines, 4 reader goroutines.
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				r.Push(float64(id*1000 + j))
			}
		}(i)
	}

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = r.Values()
				_ = r.Stats()
				_ = r.Len()
				_ = r.Last(50)
			}
		}()
	}

	wg.Wait()
	// 4 writers × 1000 pushes = 4000 total, capped at ring size 1000.
	if r.Len() != 1000 {
		t.Errorf("Len() = %d, want 1000", r.Len())
	}
}

// =========================================================================
// Benchmarks
// =========================================================================

func BenchmarkPush(b *testing.B) {
	r := NewRing(10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Push(float64(i))
	}
}

func BenchmarkPushBatch(b *testing.B) {
	r := NewRing(10000)
	batch := make([]float64, 100)
	for i := range batch {
		batch[i] = float64(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.PushBatch(batch)
	}
}

func BenchmarkValues(b *testing.B) {
	r := NewRing(10000)
	for i := 0; i < 10000; i++ {
		r.Push(float64(i))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.Values()
	}
}

func BenchmarkStats(b *testing.B) {
	r := NewRing(10000)
	for i := 0; i < 10000; i++ {
		r.Push(float64(i))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.Stats()
	}
}

// =========================================================================
// helpers
// =========================================================================

func equalFloats(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if math.Abs(a[i]-b[i]) > 1e-9 {
			return false
		}
	}
	return true
}