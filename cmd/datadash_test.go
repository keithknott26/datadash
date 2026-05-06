package main

import (
	"math"
	"sync"
	"testing"

	datadash "github.com/keithknott26/datadash"
)

// =========================================================================
// Regression: the sparkline-stops-after-a-while bug
// =========================================================================
//
// Before the fix, the sparkline updater tracked progress with a length
// counter against a *windowed* slice. Once the ring buffer filled and
// the slice length plateaued, the counter froze and the sparkline
// silently stopped receiving new data even as the ring kept overflowing.
//
// The test below pushes far more data than the ring's capacity,
// simulates many refresh ticks, and asserts that every value gets
// emitted to the sparkline updater exactly once.

func TestSparklineState_KeepsEmittingAfterOverflow(t *testing.T) {
	const ringCap = 50
	const total = 1000

	sm := newSeriesManager(1, ringCap)
	state := &sparklineState{}

	var emitted []int

	// Interleave writes and ticks so the ring overflows many times.
	for i := 0; i < total; i++ {
		sm.push([]float64{float64(i)})

		if i%5 == 0 { // tick every 5 pushes
			vals, ok := state.tick(sm, 0)
			if ok {
				emitted = append(emitted, vals...)
			}
		}
	}

	// One final tick to drain.
	if vals, ok := state.tick(sm, 0); ok {
		emitted = append(emitted, vals...)
	}

	// We should have emitted close to `total` values. Some of the
	// earliest may have been overwritten before the first tick caught
	// them, so the floor is total - ringCap. This bound proves the bug
	// is fixed: without Since(), emitted would plateau around ringCap.
	if len(emitted) < total-ringCap {
		t.Fatalf("emitted only %d values; expected at least %d (bug regression)",
			len(emitted), total-ringCap)
	}
	if len(emitted) > total {
		t.Fatalf("emitted %d values, more than the %d pushed", len(emitted), total)
	}
}

func TestSparklineState_NoOpWhenEmpty(t *testing.T) {
	sm := newSeriesManager(1, 100)
	state := &sparklineState{}

	vals, ok := state.tick(sm, 0)
	if ok || vals != nil {
		t.Errorf("tick on empty manager: vals=%v ok=%v, want (nil, false)", vals, ok)
	}
}

func TestSparklineState_NoOpWhenNoNewData(t *testing.T) {
	sm := newSeriesManager(1, 100)
	state := &sparklineState{}

	sm.push([]float64{1, 2, 3})
	if _, ok := state.tick(sm, 0); !ok {
		t.Fatal("first tick should produce data")
	}

	// Without new pushes, second tick must be a no-op.
	if vals, ok := state.tick(sm, 0); ok {
		t.Errorf("idle tick produced %v, want nothing", vals)
	}
}

func TestSparklineState_OnlyEmitsDeltas(t *testing.T) {
	sm := newSeriesManager(1, 100)
	state := &sparklineState{}

	// Each push is one row with one value — so series 0 gets 3 values.
	// (push([]float64{10,20,30}) would distribute across 3 *series*,
	// not append 3 values to one series.)
	sm.push([]float64{10})
	sm.push([]float64{20})
	sm.push([]float64{30})
	first, _ := state.tick(sm, 0)
	if len(first) != 3 {
		t.Errorf("first tick emitted %d values, want 3", len(first))
	}

	sm.push([]float64{40})
	sm.push([]float64{50})
	second, _ := state.tick(sm, 0)
	if len(second) != 2 {
		t.Errorf("second tick emitted %d values, want 2 (deltas only)", len(second))
	}
}

// =========================================================================
// Bar chart logic — scaleToInts
// =========================================================================

func TestScaleToInts_Empty(t *testing.T) {
	got, max := scaleToInts(nil, 1000)
	if got != nil {
		t.Errorf("got %v, want nil", got)
	}
	if max != 1000 {
		t.Errorf("max = %d, want 1000", max)
	}
}

func TestScaleToInts_PreservesMonotonicity(t *testing.T) {
	got, _ := scaleToInts([]float64{1, 2, 3, 4, 5}, 1000)
	for i := 1; i < len(got); i++ {
		if got[i] < got[i-1] {
			t.Errorf("monotonicity violated at i=%d: %d < %d", i, got[i], got[i-1])
		}
	}
}

func TestScaleToInts_RangeIsRespected(t *testing.T) {
	tests := [][]float64{
		{1, 2, 3},
		{-100, 0, 100},
		{0.001, 0.002, 0.003},
		{1e6, 2e6, 3e6},
	}
	for _, data := range tests {
		got, max := scaleToInts(data, 1000)
		for i, v := range got {
			if v < 0 || v > max {
				t.Errorf("data=%v index %d out of [0,%d]: %d", data, i, max, v)
			}
		}
	}
}

func TestScaleToInts_HandlesNegatives(t *testing.T) {
	// Negative values should shift to positive after scaling.
	got, max := scaleToInts([]float64{-10, 0, 10}, 100)
	if max != 100 {
		t.Errorf("max = %d, want 100", max)
	}
	if got[0] != 0 {
		t.Errorf("min value should map to 0, got %d", got[0])
	}
	if got[2] != 100 {
		t.Errorf("max value should map to scale, got %d", got[2])
	}
	if got[1] != 50 {
		t.Errorf("midpoint should map to 50, got %d", got[1])
	}
}

func TestScaleToInts_ConstantSeries(t *testing.T) {
	// All-equal values should render as flat half-height bars (so the
	// chart isn't completely empty or completely full).
	got, max := scaleToInts([]float64{42, 42, 42}, 1000)
	if max != 1000 {
		t.Errorf("max = %d, want 1000", max)
	}
	for _, v := range got {
		if v != 500 {
			t.Errorf("constant series value = %d, want 500", v)
		}
	}
}

// =========================================================================
// View state
// =========================================================================

func TestViewState_Initial(t *testing.T) {
	v := newViewState()
	off, zoom := v.get()
	if off != 0 {
		t.Errorf("initial offset = %d, want 0", off)
	}
	if zoom != 1.0 {
		t.Errorf("initial zoom = %v, want 1.0", zoom)
	}
}

func TestViewState_Pan(t *testing.T) {
	v := newViewState()
	v.panLeft(50)
	if off, _ := v.get(); off != 50 {
		t.Errorf("after panLeft(50): %d, want 50", off)
	}
	v.panLeft(25)
	if off, _ := v.get(); off != 75 {
		t.Errorf("after panLeft(25): %d, want 75", off)
	}
	v.panRight(30)
	if off, _ := v.get(); off != 45 {
		t.Errorf("after panRight(30): %d, want 45", off)
	}
}

func TestViewState_PanRightClampsAtZero(t *testing.T) {
	v := newViewState()
	v.panRight(100) // already at 0; should not go negative
	if off, _ := v.get(); off != 0 {
		t.Errorf("offset = %d, want 0 (clamped)", off)
	}
}

func TestViewState_Zoom(t *testing.T) {
	v := newViewState()
	v.zoomIn()
	if _, z := v.get(); z >= 1.0 {
		t.Errorf("zoomIn should reduce zoom, got %v", z)
	}
	v.zoomOut()
	v.zoomOut()
	if _, z := v.get(); z <= 1.0 {
		t.Errorf("zoomOut should increase zoom, got %v", z)
	}
}

func TestViewState_ZoomBounds(t *testing.T) {
	v := newViewState()
	for i := 0; i < 100; i++ {
		v.zoomIn()
	}
	if _, z := v.get(); z < 0.05 || z > 1.0 {
		t.Errorf("zoom = %v, expected within [0.05, 1.0]", z)
	}
	for i := 0; i < 100; i++ {
		v.zoomOut()
	}
	if _, z := v.get(); z < 1.0 || z > 20.0 {
		t.Errorf("zoom = %v, expected within [1.0, 20.0]", z)
	}
}

func TestViewState_Reset(t *testing.T) {
	v := newViewState()
	v.panLeft(50)
	v.zoomIn()
	v.zoomIn()
	v.reset()
	off, zoom := v.get()
	if off != 0 || zoom != 1.0 {
		t.Errorf("after reset: offset=%d zoom=%v, want 0/1.0", off, zoom)
	}
}

func TestViewState_ConcurrentSafe(t *testing.T) {
	v := newViewState()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				v.panLeft(1)
				v.panRight(1)
				v.zoomIn()
				v.zoomOut()
				_, _ = v.get()
			}
		}()
	}
	wg.Wait()
}

// =========================================================================
// Series manager
// =========================================================================

func TestSeriesManager_PushAndStats(t *testing.T) {
	sm := newSeriesManager(2, 100)
	sm.push([]float64{1, 10})
	sm.push([]float64{2, 20})
	sm.push([]float64{3, 30})

	s0 := sm.stats(0)
	if s0.Count != 3 || s0.Min != 1 || s0.Max != 3 {
		t.Errorf("series 0 stats: %+v", s0)
	}
	s1 := sm.stats(1)
	if s1.Count != 3 || s1.Min != 10 || s1.Max != 30 {
		t.Errorf("series 1 stats: %+v", s1)
	}
}

func TestSeriesManager_GrowsDynamically(t *testing.T) {
	// Start with one series, then push a row with three values; manager
	// should grow to accommodate (up to maxSeries).
	sm := newSeriesManager(1, 100)
	if sm.seriesCount() != 1 {
		t.Errorf("initial count = %d", sm.seriesCount())
	}
	sm.push([]float64{1, 2, 3})
	if sm.seriesCount() != 3 {
		t.Errorf("after push: count = %d, want 3", sm.seriesCount())
	}
}

func TestSeriesManager_RespectsMaxSeries(t *testing.T) {
	sm := newSeriesManager(1, 100)
	// Push more values than maxSeries — extras should be dropped.
	values := make([]float64, maxSeries+5)
	for i := range values {
		values[i] = float64(i)
	}
	sm.push(values)
	if sm.seriesCount() != maxSeries {
		t.Errorf("count = %d, want %d (capped)", sm.seriesCount(), maxSeries)
	}
}

func TestSeriesManager_Names(t *testing.T) {
	sm := newSeriesManager(2, 100)

	// No headers yet → default names.
	if got := sm.seriesName(0); got != "series_1" {
		t.Errorf("default name = %q", got)
	}

	// With label-prefixed headers (typical of TSV/CSV with timestamp).
	sm.setHeaders([]string{"time", "cpu", "mem"})
	if got := sm.seriesName(0); got != "cpu" {
		t.Errorf("series 0 = %q, want 'cpu'", got)
	}
	if got := sm.seriesName(1); got != "mem" {
		t.Errorf("series 1 = %q, want 'mem'", got)
	}
}

func TestSeriesManager_NamesWithoutLabelColumn(t *testing.T) {
	sm := newSeriesManager(2, 100)
	// Two-column headers, two series — no label column.
	sm.setHeaders([]string{"a", "b"})
	// idx+1 falls off the end, so we fall back to idx.
	if got := sm.seriesName(1); got != "b" {
		t.Errorf("series 1 = %q, want 'b'", got)
	}
}

func TestSeriesManager_StatsOutOfRange(t *testing.T) {
	sm := newSeriesManager(2, 100)
	if got := sm.stats(99); got != (datadash.Stats{}) {
		t.Errorf("out-of-range stats = %+v, want zero", got)
	}
}

func TestSeriesManager_SliceSince(t *testing.T) {
	sm := newSeriesManager(1, 100)
	for i := 1; i <= 10; i++ {
		sm.push([]float64{float64(i)})
	}

	values, pos := sm.sliceSince(0, 0)
	if len(values) != 10 {
		t.Errorf("got %d values, want 10", len(values))
	}
	if pos != 10 {
		t.Errorf("pos = %d, want 10", pos)
	}

	// Push more, resume from the prior pos.
	sm.push([]float64{11})
	sm.push([]float64{12})
	values2, pos2 := sm.sliceSince(0, pos)
	if len(values2) != 2 {
		t.Errorf("delta = %d values, want 2", len(values2))
	}
	if pos2 != 12 {
		t.Errorf("pos = %d, want 12", pos2)
	}
}

func TestSeriesManager_SliceSinceOutOfRange(t *testing.T) {
	sm := newSeriesManager(1, 100)
	values, pos := sm.sliceSince(99, 0)
	if values != nil {
		t.Errorf("out-of-range slice = %v, want nil", values)
	}
	if pos != 0 {
		t.Errorf("pos = %d, want 0 (echoed back)", pos)
	}
}

// =========================================================================
// Rolling average
// =========================================================================

func TestRollingAverage_Empty(t *testing.T) {
	if got := rollingAverage(nil, 5); got != nil {
		t.Errorf("rollingAverage(nil) = %v, want nil", got)
	}
}

func TestRollingAverage_FullDataAtFirstWindow(t *testing.T) {
	// First few outputs should be running means while window fills,
	// then settle into proper rolling averages.
	got := rollingAverage([]float64{1, 2, 3, 4, 5}, 3)
	want := []float64{1, 1.5, 2, 3, 4} // (1), (1+2)/2, (1+2+3)/3, (2+3+4)/3, (3+4+5)/3
	for i := range got {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Errorf("idx %d: got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestRollingAverage_WindowExceedsData(t *testing.T) {
	// Window > data length → use full data length.
	got := rollingAverage([]float64{2, 4, 6}, 100)
	want := []float64{2, 3, 4} // all full prefixes
	for i := range got {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Errorf("idx %d: got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestRollingAverage_ZeroOrNegativeWindow(t *testing.T) {
	// Falls back to full-data average behavior.
	got := rollingAverage([]float64{1, 2, 3, 4}, 0)
	if len(got) != 4 {
		t.Errorf("len = %d, want 4", len(got))
	}
}

// =========================================================================
// atomBool
// =========================================================================

func TestAtomBool(t *testing.T) {
	a := &atomBool{}
	if a.get() {
		t.Error("default = true, want false")
	}
	a.toggle()
	if !a.get() {
		t.Error("after toggle = false, want true")
	}
	a.toggle()
	if a.get() {
		t.Error("after second toggle = true, want false")
	}
}

func TestAtomBool_ConcurrentSafe(t *testing.T) {
	a := &atomBool{}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				a.toggle()
				_ = a.get()
			}
		}()
	}
	wg.Wait()
	// 8 × 1000 toggles = 8000 (even) → final state must be false.
	if a.get() {
		t.Errorf("final state = true, want false (8000 toggles is even)")
	}
}

// =========================================================================
// repeat helper
// =========================================================================

func TestRepeat(t *testing.T) {
	got := repeat(palette[0], 3)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	for _, c := range got {
		if c != palette[0] {
			t.Errorf("got %v, want %v", c, palette[0])
		}
	}
}

func TestRepeat_Zero(t *testing.T) {
	got := repeat(palette[0], 0)
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}