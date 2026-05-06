package datadash

import (
	"math"
	"math/rand"
	"testing"
)

// =========================================================================
// LTTB — passthrough cases
// =========================================================================

func TestLTTB_PassthroughWhenSmaller(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5}

	// threshold >= len → return original
	got := LTTB(data, 5)
	if !sameSlice(got, data) {
		t.Errorf("LTTB returned different slice for threshold == len")
	}

	got = LTTB(data, 100)
	if !sameSlice(got, data) {
		t.Errorf("LTTB returned different slice for threshold > len")
	}
}

func TestLTTB_PassthroughWhenThresholdTooSmall(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	for _, threshold := range []int{0, 1, 2} {
		got := LTTB(data, threshold)
		if !sameSlice(got, data) {
			t.Errorf("LTTB(_, %d) should passthrough, got modified slice", threshold)
		}
	}
}

// =========================================================================
// LTTB — preserves endpoints
// =========================================================================

func TestLTTB_PreservesFirstAndLast(t *testing.T) {
	data := make([]float64, 1000)
	for i := range data {
		data[i] = math.Sin(float64(i) / 10)
	}

	for _, threshold := range []int{3, 10, 50, 100, 500} {
		got := LTTB(data, threshold)
		if got[0] != data[0] {
			t.Errorf("threshold=%d: first point %v, want %v", threshold, got[0], data[0])
		}
		if got[len(got)-1] != data[len(data)-1] {
			t.Errorf("threshold=%d: last point %v, want %v",
				threshold, got[len(got)-1], data[len(data)-1])
		}
	}
}

// =========================================================================
// LTTB — output length matches threshold
// =========================================================================

func TestLTTB_OutputLength(t *testing.T) {
	data := make([]float64, 1000)
	for i := range data {
		data[i] = float64(i)
	}

	for _, threshold := range []int{3, 10, 100, 500, 999} {
		got := LTTB(data, threshold)
		if len(got) != threshold {
			t.Errorf("threshold=%d: len(out)=%d, want %d", threshold, len(got), threshold)
		}
	}
}

// =========================================================================
// LTTB — preserves visual extremes
// =========================================================================

func TestLTTB_PreservesPeaks(t *testing.T) {
	// 1000 points of low noise with one big spike in the middle.
	// LTTB should always keep the spike when downsampling.
	data := make([]float64, 1000)
	for i := range data {
		data[i] = 0.5
	}
	spikeIdx := 500
	data[spikeIdx] = 100 // dramatic peak

	got := LTTB(data, 50)

	maxVal := got[0]
	for _, v := range got {
		if v > maxVal {
			maxVal = v
		}
	}
	// The peak might not be exactly preserved (could pick a neighbor),
	// but the maximum value in the output should be close to 100.
	if maxVal < 50 {
		t.Errorf("peak lost in downsampling: max in output = %v, want ≥ 50", maxVal)
	}
}

func TestLTTB_PreservesValleys(t *testing.T) {
	data := make([]float64, 1000)
	for i := range data {
		data[i] = 50
	}
	data[500] = -100 // dramatic valley

	got := LTTB(data, 50)

	minVal := got[0]
	for _, v := range got {
		if v < minVal {
			minVal = v
		}
	}
	if minVal > -50 {
		t.Errorf("valley lost: min in output = %v, want ≤ -50", minVal)
	}
}

// =========================================================================
// LTTB — linear data
// =========================================================================

func TestLTTB_LinearData(t *testing.T) {
	// Strictly monotonic data should remain monotonic after downsampling.
	data := make([]float64, 1000)
	for i := range data {
		data[i] = float64(i) * 2
	}

	got := LTTB(data, 100)
	for i := 1; i < len(got); i++ {
		if got[i] < got[i-1] {
			t.Errorf("monotonicity violated at i=%d: %v < %v", i, got[i], got[i-1])
		}
	}
}

// =========================================================================
// LTTB — random data sanity
// =========================================================================

func TestLTTB_RandomData(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	data := make([]float64, 5000)
	for i := range data {
		data[i] = r.NormFloat64()
	}

	got := LTTB(data, 200)
	if len(got) != 200 {
		t.Errorf("len = %d, want 200", len(got))
	}

	// Stats should be roughly similar (we're representing the same dataset).
	originalMean := mean(data)
	sampledMean := mean(got)
	// With normal data, sampled mean should be within 0.5 of true mean.
	if math.Abs(originalMean-sampledMean) > 0.5 {
		t.Errorf("sampled mean drifted: original=%.3f, sampled=%.3f",
			originalMean, sampledMean)
	}
}

// =========================================================================
// MinMaxDecimate
// =========================================================================

func TestMinMaxDecimate_PassthroughWhenSmaller(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5}
	got := MinMaxDecimate(data, 10)
	if !sameSlice(got, data) {
		t.Errorf("expected passthrough")
	}
}

func TestMinMaxDecimate_PassthroughWhenThresholdTooSmall(t *testing.T) {
	data := make([]float64, 100)
	for _, threshold := range []int{0, 1, 2, 3} {
		got := MinMaxDecimate(data, threshold)
		if !sameSlice(got, data) {
			t.Errorf("threshold=%d should passthrough", threshold)
		}
	}
}

func TestMinMaxDecimate_PreservesExtremes(t *testing.T) {
	// Generate 1000 points where each chunk of 10 has a known min/max.
	data := make([]float64, 1000)
	for i := range data {
		// Sawtooth: 0, 1, 2, ..., 9, 0, 1, 2, ..., 9, ...
		data[i] = float64(i % 10)
	}

	// Decimate to 100 points (50 buckets × 2 points each).
	got := MinMaxDecimate(data, 100)

	if len(got) > 100 || len(got) < 50 {
		t.Errorf("len = %d, expected ~100", len(got))
	}

	// Output should contain both extremes (0 and 9) globally.
	hasMin, hasMax := false, false
	for _, v := range got {
		if v == 0 {
			hasMin = true
		}
		if v == 9 {
			hasMax = true
		}
	}
	if !hasMin || !hasMax {
		t.Errorf("extremes lost: hasMin=%v hasMax=%v", hasMin, hasMax)
	}
}

func TestMinMaxDecimate_OrderPreservation(t *testing.T) {
	// Each bucket emits min and max in their original-position order.
	// Construct data where the pattern alternates: bucket 0 has min-then-max,
	// bucket 1 has max-then-min.
	data := []float64{
		1, 5, 2, 6, // bucket: min=1 (idx 0), max=6 (idx 3) → emit [1, 6]
		8, 3, 7, 2, // bucket: max=8 (idx 4), min=2 (idx 7) → emit [8, 2]
	}
	got := MinMaxDecimate(data, 4)

	// Expected: [1, 6, 8, 2]
	want := []float64{1, 6, 8, 2}
	if !equalFloats(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// =========================================================================
// Benchmarks
// =========================================================================

func BenchmarkLTTB_1Mto1K(b *testing.B) {
	data := make([]float64, 1_000_000)
	for i := range data {
		data[i] = math.Sin(float64(i) / 1000)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = LTTB(data, 1000)
	}
}

func BenchmarkMinMaxDecimate_1Mto1K(b *testing.B) {
	data := make([]float64, 1_000_000)
	for i := range data {
		data[i] = math.Sin(float64(i) / 1000)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = MinMaxDecimate(data, 1000)
	}
}

// =========================================================================
// helpers
// =========================================================================

// sameSlice is true when a and b reference the same backing array
// (passthrough check) AND have the same length. Returning the input
// unchanged means LTTB/MinMaxDecimate should return the very same
// slice header, not a copy.
func sameSlice(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	return &a[0] == &b[0]
}

func mean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	s := 0.0
	for _, v := range data {
		s += v
	}
	return s / float64(len(data))
}
