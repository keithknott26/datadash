package main

import (
	"context"
	"fmt"
	"io"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// =========================================================================
// Demo subcommand
// =========================================================================

var demoCmd = &cobra.Command{
	Use:   "demo [name]",
	Short: "Run a built-in demo dataset",
	Long: `Demo mode generates synthetic data and pipes it into datadash so
you can see what the tool does without finding a dataset.

Available demos:
  sine      Multiple sine waves at different frequencies
  walk      Gaussian random walk (looks like a stock chart)
  metrics   Simulated server metrics (CPU, memory, requests, errors)
  anomaly   A baseline signal with occasional spikes
  pulse     Periodic pulses with decay
  cycle     Cycles through every demo, ~30s each (default)`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDemo,
}

func init() {
	rootCmd.AddCommand(demoCmd)
}

func runDemo(cmd *cobra.Command, args []string) error {
	name := "cycle"
	if len(args) > 0 {
		name = strings.ToLower(args[0])
	}

	pr, pw := io.Pipe()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the generator in the background.
	go func() {
		defer pw.Close()
		runGenerator(ctx, pw, name)
	}()

	// Replace stdin with the pipe so the existing pipeline picks it up.
	// We don't literally swap os.Stdin — we just reuse runApp's logic by
	// forging a fake input source. The simplest path is to set a global
	// override read by runApp.
	demoInput = pr
	defer func() { demoInput = nil }()

	return runApp(rootCmd, nil)
}

// demoInput is read by runApp when non-nil, instead of stdin.
// This is set by runDemo and consulted in cmd/datadash.go.
var demoInput io.Reader

// =========================================================================
// Generator dispatch
// =========================================================================

func runGenerator(ctx context.Context, w io.Writer, name string) {
	gens := map[string]func(context.Context, io.Writer){
		"sine":    genSine,
		"walk":    genWalk,
		"metrics": genMetrics,
		"anomaly": genAnomaly,
		"pulse":   genPulse,
	}

	if name == "cycle" {
		// Cycle through each demo for ~30 seconds.
		order := []string{"sine", "metrics", "walk", "anomaly", "pulse"}
		for {
			for _, n := range order {
				select {
				case <-ctx.Done():
					return
				default:
				}
				fmt.Fprintln(w, headerFor(n))
				ctxDemo, cancel := context.WithTimeout(ctx, 30*time.Second)
				gens[n](ctxDemo, w)
				cancel()
			}
		}
	}

	gen, ok := gens[name]
	if !ok {
		fmt.Fprintln(w, "available demos: sine, walk, metrics, anomaly, pulse, cycle")
		return
	}
	fmt.Fprintln(w, headerFor(name))
	gen(ctx, w)
}

func headerFor(name string) string {
	switch name {
	case "sine":
		return "t\tsin\tcos\ttan/3"
	case "walk":
		return "t\tprice_a\tprice_b\tprice_c"
	case "metrics":
		return "t\tcpu_pct\tmem_pct\treq_per_s\terrors"
	case "anomaly":
		return "t\tlatency_ms"
	case "pulse":
		return "t\tsignal\tdecay"
	}
	return ""
}

// =========================================================================
// Generators
//
// All generators write tab-separated rows at ~50ms cadence, formatted so
// the existing Reader auto-detects them as TSV. The first column is a
// timestamp and is consumed as the X-axis label.
// =========================================================================

func tickEvery(d time.Duration, ctx context.Context, fn func(t int)) {
	tick := time.NewTicker(d)
	defer tick.Stop()
	step := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			fn(step)
			step++
		}
	}
}

func genSine(ctx context.Context, w io.Writer) {
	tickEvery(40*time.Millisecond, ctx, func(i int) {
		t := float64(i) / 10
		// Clamp tan — it approaches ±∞ near π/2, producing NaN/Inf
		// that would corrupt stats and crash the chart.
		tanVal := math.Tan(t * 0.3)
		if math.IsNaN(tanVal) || math.IsInf(tanVal, 0) {
			tanVal = 0
		}
		tanVal = clamp(tanVal/3, -5, 5)

		fmt.Fprintf(w, "%d\t%.4f\t%.4f\t%.4f\n",
			i,
			math.Sin(t),
			math.Cos(t*0.7),
			tanVal,
		)
	})
}

func genWalk(ctx context.Context, w io.Writer) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	a, b, c := 100.0, 50.0, 200.0

	tickEvery(50*time.Millisecond, ctx, func(i int) {
		a += r.NormFloat64() * 0.8
		b += r.NormFloat64() * 0.4
		c += r.NormFloat64() * 1.5
		fmt.Fprintf(w, "%d\t%.2f\t%.2f\t%.2f\n", i, a, b, c)
	})
}

func genMetrics(ctx context.Context, w io.Writer) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	tickEvery(50*time.Millisecond, ctx, func(i int) {
		t := float64(i) / 30

		// CPU has a daily-cycle baseline plus jitter.
		cpu := 35 + 25*math.Sin(t*0.2) + r.NormFloat64()*4

		// Memory creeps up slowly with occasional GC drops.
		mem := 50 + math.Mod(t*0.5, 30)
		if r.Float64() < 0.005 {
			mem -= 15 // simulate a GC
		}

		// Requests follow a sine pattern with bursts.
		reqs := 200 + 80*math.Sin(t*0.4) + r.NormFloat64()*15
		if r.Float64() < 0.02 {
			reqs += 200 // burst
		}

		// Errors are rare but spiky.
		errs := math.Max(0, r.NormFloat64()*2)
		if r.Float64() < 0.01 {
			errs += float64(r.Intn(20))
		}

		fmt.Fprintf(w, "%d\t%.1f\t%.1f\t%.0f\t%.0f\n",
			i,
			clamp(cpu, 0, 100),
			clamp(mem, 0, 100),
			math.Max(0, reqs),
			errs,
		)
	})
}

func genAnomaly(ctx context.Context, w io.Writer) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	tickEvery(40*time.Millisecond, ctx, func(i int) {
		// Baseline latency: ~50ms with some jitter.
		latency := 50 + r.NormFloat64()*5

		// 1% chance of a spike up to 500ms.
		if r.Float64() < 0.01 {
			latency += 200 + r.Float64()*250
		}
		// 0.2% chance of a really nasty outlier.
		if r.Float64() < 0.002 {
			latency += 1500 + r.Float64()*1000
		}

		fmt.Fprintf(w, "%d\t%.1f\n", i, math.Max(0, latency))
	})
}

func genPulse(ctx context.Context, w io.Writer) {
	tickEvery(40*time.Millisecond, ctx, func(i int) {
		t := float64(i) / 10

		// Sharp pulses every 50 steps with exponential decay.
		phase := math.Mod(t, 5.0)
		signal := math.Exp(-phase*1.5) * 100

		// Slower envelope.
		decay := 50 * math.Exp(-math.Mod(t, 20)/8)

		fmt.Fprintf(w, "%d\t%.2f\t%.2f\n", i, signal, decay)
	})
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}