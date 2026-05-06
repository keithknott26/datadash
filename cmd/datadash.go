package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	datadash "github.com/keithknott26/datadash"

	"github.com/mum4k/termdash"
	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/container"
	"github.com/mum4k/termdash/keyboard"
	"github.com/mum4k/termdash/linestyle"
	"github.com/mum4k/termdash/terminal/tcell"
	"github.com/mum4k/termdash/terminal/terminalapi"
	"github.com/mum4k/termdash/widgetapi"
	"github.com/mum4k/termdash/widgets/barchart"
	"github.com/mum4k/termdash/widgets/linechart"
	"github.com/mum4k/termdash/widgets/sparkline"
	"github.com/mum4k/termdash/widgets/text"
	"github.com/spf13/cobra"
)

// =========================================================================
// Constants
// =========================================================================

const (
	appVersion     = "0.5.0"
	maxSeries      = 8
	defaultRingCap = 8192
	statsPercent   = 18

	// brailleMultiplier controls how many points we feed per chart cell.
	// termdash's linechart renders lines using braille characters which
	// give 2×4 sub-cell resolution per character. Feeding 4× the cell
	// width gives the canvas enough density to draw smooth, high-fidelity
	// curves without wasting cycles.
	brailleMultiplier = 4

	// barMaxBars caps how many bars are displayed in a bar chart panel —
	// more than this and the bars become too thin to read.
	barMaxBars = 30

	// barScale is the integer range we map float values into for the
	// barchart and sparkline widgets (which only accept ints).
	barScale = 1000
)

var palette = []cell.Color{
	cell.ColorGreen,
	cell.ColorBlue,
	cell.ColorRed,
	cell.ColorYellow,
	cell.ColorMagenta,
	cell.ColorCyan,
	cell.ColorWhite,
	cell.Color(208),
}

var paletteNames = []string{
	"green", "blue", "red", "yellow", "magenta", "cyan", "white", "orange",
}

// =========================================================================
// CLI (Cobra)
// =========================================================================

var cfg struct {
	delimiter    string
	format       string
	labelMode    string
	chartType    string
	scroll       bool
	showAvg      bool
	avgWindow    int
	redraw       time.Duration
	seek         time.Duration
	jsonFields   []string
	debug        bool
	url          string
	pollInterval time.Duration
	httpHeaders  []string
	downsample   string
	highFidelity bool
}

var rootCmd = &cobra.Command{
	Use:   "datadash [file]",
	Short: "Visualize and graph data in the terminal",
	Long: `DataDash renders streaming or tabular data as interactive
terminal charts with a separate panel for each data series.

  cat data.csv | datadash
  datadash metrics.json --field cpu --field mem
  datadash --url http://localhost:9100/metrics --poll 5s --field cpu
  datadash --chart bar < data.csv
  datadash --chart sparkline < data.csv
  datadash demo                          # built-in showcase
  datadash demo metrics                  # specific demo

  seq 100 | awk '{print sin($1/5)}' | datadash --scroll`,
	Version:      appVersion,
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         runApp,
}

func init() {
	// Use PersistentFlags so subcommands (like `demo`) inherit them.
	f := rootCmd.PersistentFlags()

	// Input parsing
	f.StringVarP(&cfg.delimiter, "delimiter", "d", "", "Column delimiter (auto-detected if omitted)")
	f.StringVarP(&cfg.format, "format", "f", "auto", "Input format: auto, csv, tsv, json, space, single")
	f.StringVarP(&cfg.labelMode, "label-mode", "m", "first", "X-axis label source: first, time, none")
	f.StringSliceVar(&cfg.jsonFields, "field", nil, "JSON field to extract (dot path, repeatable)")

	// Chart type & rendering
	f.StringVarP(&cfg.chartType, "chart", "c", "line", "Chart type: line, bar, sparkline")
	f.StringVar(&cfg.downsample, "downsample", "lttb", "Downsampling: lttb, minmax, none")
	f.BoolVar(&cfg.highFidelity, "high-fidelity", true, "Maximize braille sub-cell resolution")

	// Streaming behavior
	f.BoolVarP(&cfg.scroll, "scroll", "s", false, "Scroll mode — keep newest data on screen")
	f.BoolVarP(&cfg.showAvg, "average-line", "a", false, "Show a rolling average overlay (line chart only)")
	f.IntVarP(&cfg.avgWindow, "average-seek", "z", 500, "Rolling average window size")
	f.DurationVarP(&cfg.redraw, "redraw-interval", "r", 100*time.Millisecond, "Screen redraw interval")
	f.DurationVarP(&cfg.seek, "seek-interval", "l", 20*time.Millisecond, "Input line read interval")

	// HTTP source
	f.StringVar(&cfg.url, "url", "", "Poll an HTTP endpoint instead of reading stdin/file")
	f.DurationVar(&cfg.pollInterval, "poll", 5*time.Second, "HTTP poll interval (with --url)")
	f.StringSliceVar(&cfg.httpHeaders, "header", nil, "HTTP header to send, 'Key: Value' (repeatable)")

	// Misc
	f.BoolVar(&cfg.debug, "debug", false, "Enable debug logging to stderr")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// =========================================================================
// Application entry
// =========================================================================

func runApp(cmd *cobra.Command, args []string) error {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "\ndatadash: unexpected error: %v\n", r)
			fmt.Fprintf(os.Stderr, "Please report: https://github.com/keithknott26/datadash/issues\n")
			os.Exit(1)
		}
	}()

	// ── Logging ────────────────────────────────────────────────────────
	level := slog.LevelWarn
	if cfg.debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	// ── Validate chart type ────────────────────────────────────────────
	switch cfg.chartType {
	case "line", "bar", "sparkline":
	default:
		return fmt.Errorf("invalid --chart %q (want line, bar, or sparkline)", cfg.chartType)
	}

	// ── Resolve input source ───────────────────────────────────────────
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	input, closeFn, err := resolveInput(rootCtx, args)
	if err != nil {
		return err
	}
	if closeFn != nil {
		defer closeFn()
	}

	// ── Build reader ───────────────────────────────────────────────────
	var readerOpts []datadash.ReaderOption
	if f := datadash.ParseFormat(cfg.format); f != datadash.FormatAuto {
		readerOpts = append(readerOpts, datadash.WithFormat(f))
	}
	if cfg.delimiter != "" {
		readerOpts = append(readerOpts, datadash.WithDelimiter(cfg.delimiter))
	}
	if len(cfg.jsonFields) > 0 {
		readerOpts = append(readerOpts, datadash.WithJSONFields(cfg.jsonFields))
	}
	readerOpts = append(readerOpts, datadash.WithLabelMode(cfg.labelMode))
	reader := datadash.NewReader(input, readerOpts...)

	// ── Read first row to discover series count ────────────────────────
	firstRow, err := reader.Read()
	if err != nil {
		return fmt.Errorf("cannot read input: %w", err)
	}
	numSeries := len(firstRow.Values)
	if numSeries > maxSeries {
		numSeries = maxSeries
	}
	if numSeries == 0 {
		return fmt.Errorf("no numeric data found in input")
	}
	headers := reader.Headers()

	slog.Debug("detected",
		"format", reader.Format(),
		"chart", cfg.chartType,
		"series", numSeries,
		"headers", headers,
		"downsample", cfg.downsample,
	)

	// ── Terminal ───────────────────────────────────────────────────────
	t, err := tcell.New()
	if err != nil {
		return fmt.Errorf("terminal init: %w", err)
	}
	defer t.Close()

	termSize := t.Size()
	chartWidth := int(float64(termSize.X) * float64(100-statsPercent) / 100.0)
	if chartWidth < 20 {
		chartWidth = 20
	}
	ringCap := chartWidth * brailleMultiplier * 4
	if ringCap < defaultRingCap {
		ringCap = defaultRingCap
	}

	// ── Data layer ─────────────────────────────────────────────────────
	sm := newSeriesManager(numSeries, ringCap)
	sm.setHeaders(headers)
	sm.push(firstRow.Values)
	if firstRow.Label != "" {
		sm.setLastLabel(firstRow.Label)
	}
	vs := newViewState()
	showHelp := &atomBool{}

	// ── Build per-series panels (each owns its chart + stats widget) ──
	panels, err := buildPanels(numSeries, sm)
	if err != nil {
		return err
	}

	// ── Layout ─────────────────────────────────────────────────────────
	rootOpts := []container.Option{
		container.Border(linestyle.Light),
		container.BorderTitle(fmt.Sprintf(" DataDash v%s — press ? for help ", appVersion)),
		container.BorderTitleAlignCenter(),
	}
	rootOpts = append(rootOpts, buildPanelGrid(panels)...)

	c, err := container.New(t, rootOpts...)
	if err != nil {
		return fmt.Errorf("container: %w", err)
	}

	// ── Reader goroutine ───────────────────────────────────────────────
	go func() {
		for {
			select {
			case <-rootCtx.Done():
				return
			default:
			}
			row, err := reader.Read()
			if err == io.EOF {
				slog.Debug("input EOF")
				return
			}
			if err != nil {
				slog.Debug("skip line", "err", err)
				continue
			}
			sm.push(row.Values)
		if row.Label != "" {
			sm.setLastLabel(row.Label)
		}
			if cfg.seek > 0 {
				time.Sleep(cfg.seek)
			}
		}
	}()

	// ── Refresh goroutine ──────────────────────────────────────────────
	go func() {
		tick := time.NewTicker(cfg.redraw)
		defer tick.Stop()
		for {
			select {
			case <-rootCtx.Done():
				return
			case <-tick.C:
				refreshCharts(panels, sm, vs, chartWidth)
				refreshStats(panels, sm, showHelp.get())
			}
		}
	}()

	// ── Keyboard ───────────────────────────────────────────────────────
	panStep := chartWidth / 10
	if panStep < 5 {
		panStep = 5
	}
	keyHandler := func(k *terminalapi.Keyboard) {
		switch k.Key {
		case 'q', keyboard.KeyEsc:
			rootCancel()
		case 'h', keyboard.KeyArrowLeft:
			vs.panLeft(panStep)
		case 'l', keyboard.KeyArrowRight:
			vs.panRight(panStep)
		case '+', '=':
			vs.zoomIn()
		case '-':
			vs.zoomOut()
		case 'r':
			vs.reset()
		case '?':
			showHelp.toggle()
		}
	}

	err = termdash.Run(rootCtx, t, c,
		termdash.KeyboardSubscriber(keyHandler),
		termdash.RedrawInterval(cfg.redraw),
	)
	if err != nil && rootCtx.Err() == nil {
		return err
	}
	return nil
}

// =========================================================================
// Input source resolution
// =========================================================================

func resolveInput(ctx context.Context, args []string) (io.Reader, func(), error) {
	if demoInput != nil {
		return demoInput, nil, nil
	}

	if cfg.url != "" {
		opts := []datadash.HTTPOption{datadash.HTTPInterval(cfg.pollInterval)}
		for _, h := range cfg.httpHeaders {
			k, v, ok := strings.Cut(h, ":")
			if !ok {
				return nil, nil, fmt.Errorf("invalid --header %q (expected 'Key: Value')", h)
			}
			opts = append(opts, datadash.HTTPHeader(strings.TrimSpace(k), strings.TrimSpace(v)))
		}
		src := datadash.NewHTTPSource(ctx, cfg.url, opts...)
		if cfg.format == "auto" {
			cfg.format = "json"
		}
		return src, func() { _ = src.Close() }, nil
	}

	if len(args) > 0 {
		f, err := os.Open(args[0])
		if err != nil {
			return nil, nil, fmt.Errorf("cannot open %s: %w", args[0], err)
		}
		return f, func() { _ = f.Close() }, nil
	}

	info, err := os.Stdin.Stat()
	if err == nil && (info.Mode()&os.ModeCharDevice) != 0 {
		return nil, nil, fmt.Errorf(
			"no input — pipe data, provide a file, use --url, or try `datadash demo`")
	}
	return os.Stdin, nil, nil
}

// =========================================================================
// Chart panels — line / bar / sparkline
// =========================================================================

// chartPanel bundles a widget with the function that updates it from
// fresh data, plus its own per-row stats text widget so stats stay
// vertically aligned with the chart they describe.
type chartPanel struct {
	widget      widgetapi.Widget
	statsWidget *text.Text
	refresh     func(sm *seriesManager, vs *viewState, chartWidth int)
	title       string
	color       cell.Color
	idx         int
}

// buildPanels constructs one chart panel per data series, picking the
// widget type and refresh strategy from cfg.chartType. Each panel also
// owns its own stats text widget.
func buildPanels(numSeries int, sm *seriesManager) ([]chartPanel, error) {
	panels := make([]chartPanel, numSeries)

	for i := 0; i < numSeries; i++ {
		idx := i
		color := palette[i%len(palette)]
		colorName := paletteNames[i%len(paletteNames)]
		name := sm.seriesName(i)
		title := fmt.Sprintf(" %s (%s) ", name, colorName)

		var p chartPanel
		var err error

		switch cfg.chartType {
		case "line":
			p, err = newLinePanel(idx, color, title)
		case "bar":
			p, err = newBarPanel(idx, color, title)
		case "sparkline":
			p, err = newSparklinePanel(idx, color, name, title)
		}
		if err != nil {
			return nil, fmt.Errorf("series %d: %w", i, err)
		}

		sw, err := text.New(text.RollContent())
		if err != nil {
			return nil, fmt.Errorf("series %d stats: %w", i, err)
		}
		p.statsWidget = sw

		panels[i] = p
	}
	return panels, nil
}

// ── Line chart ───────────────────────────────────────────────────────────

func newLinePanel(idx int, color cell.Color, title string) (chartPanel, error) {
	lc, err := linechart.New(
		linechart.YAxisAdaptive(),
		linechart.AxesCellOpts(cell.FgColor(cell.ColorNumber(244))),
		linechart.YLabelCellOpts(cell.FgColor(cell.ColorNumber(244))),
		linechart.XLabelCellOpts(cell.FgColor(cell.ColorNumber(244))),
	)
	if err != nil {
		return chartPanel{}, err
	}

	refresh := func(sm *seriesManager, vs *viewState, chartWidth int) {
		offset, zoom := vs.get()
		raw := sm.slice(idx, chartWidth, offset, zoom)
		if len(raw) == 0 {
			return
		}

		target := chartWidth * brailleMultiplier
		if !cfg.highFidelity {
			target = chartWidth
		}
		data := downsampleFor(raw, target)

		_ = lc.Series("data", data,
			linechart.SeriesCellOpts(cell.FgColor(color)),
		)
		if cfg.showAvg && len(data) > 1 {
			_ = lc.Series("avg",
				rollingAverage(data, cfg.avgWindow),
				linechart.SeriesCellOpts(cell.FgColor(cell.Color(245))),
			)
		}
	}

	return chartPanel{widget: lc, refresh: refresh, title: title, color: color, idx: idx}, nil
}

// ── Bar chart ────────────────────────────────────────────────────────────

func newBarPanel(idx int, color cell.Color, title string) (chartPanel, error) {
	// One bar chart per panel — each bar is a recent value.
	colors := make([]cell.Color, barMaxBars)
	for i := range colors {
		colors[i] = color
	}
	bc, err := barchart.New(
		barchart.BarColors(colors),
		barchart.ValueColors(repeat(cell.ColorBlack, barMaxBars)),
		barchart.ShowValues(),
	)
	if err != nil {
		return chartPanel{}, err
	}

	refresh := func(sm *seriesManager, vs *viewState, chartWidth int) {
		// Bar charts always show the most recent N values — pan/zoom
		// doesn't apply meaningfully.
		raw := sm.slice(idx, chartWidth, 0, 1.0)
		if len(raw) == 0 {
			return
		}
		// Trim to barMaxBars most recent values.
		if len(raw) > barMaxBars {
			raw = raw[len(raw)-barMaxBars:]
		}
		ints, max := scaleToInts(raw, barScale)
		_ = bc.Values(ints, max)
	}

	return chartPanel{widget: bc, refresh: refresh, title: title, color: color, idx: idx}, nil
}

// ── Sparkline ────────────────────────────────────────────────────────────

func newSparklinePanel(idx int, color cell.Color, name, title string) (chartPanel, error) {
	sl, err := sparkline.New(
		sparkline.Label(name, cell.FgColor(color)),
		sparkline.Color(color),
	)
	if err != nil {
		return chartPanel{}, err
	}

	state := &sparklineState{}

	refresh := func(sm *seriesManager, vs *viewState, chartWidth int) {
		ints, ok := state.tick(sm, idx)
		if ok {
			_ = sl.Add(ints)
		}
	}

	return chartPanel{widget: sl, refresh: refresh, title: title, color: color, idx: idx}, nil
}

// sparklineState is the testable bit of newSparklinePanel — given a
// series manager and series index, it returns the *next* batch of
// scaled int values to feed to the sparkline widget. By tracking an
// absolute ring-buffer position (rather than slice lengths) it keeps
// emitting deltas correctly even after the ring has overflowed many
// times.
type sparklineState struct {
	since int64
}

// tick advances state and returns (newValues, true) when there's
// fresh data, or (nil, false) when nothing new has been pushed.
func (s *sparklineState) tick(sm *seriesManager, idx int) ([]int, bool) {
	values, pos := sm.sliceSince(idx, s.since)
	if len(values) == 0 {
		return nil, false
	}
	s.since = pos

	// Scale using the full series range so values from different ticks
	// are visually comparable. Per-batch scaling would cause the
	// sparkline to flicker as different batches normalize differently.
	st := sm.stats(idx)
	ints := scaleWithRange(values, st.Min, st.Max, barScale)
	return ints, true
}

// =========================================================================
// Layout — each series gets its own [chart | stats] row
// =========================================================================

// buildPanelGrid produces a vertically-stacked layout where every row
// is a horizontal split of [chart | stats]. Stats blocks are therefore
// physically aligned with the chart they describe — without this, a
// single tall stats panel just stacks sequentially and drifts out of
// sync with the per-row chart heights.
func buildPanelGrid(panels []chartPanel) []container.Option {
	if len(panels) == 1 {
		return rowFor(panels[0])
	}
	pct := 100 / len(panels)
	if pct < 5 {
		pct = 5
	}
	return []container.Option{
		container.SplitHorizontal(
			container.Top(rowFor(panels[0])...),
			container.Bottom(buildPanelGrid(panels[1:])...),
			container.SplitPercent(pct),
		),
	}
}

// rowFor builds a single row: stats on the left, chart on the right.
func rowFor(p chartPanel) []container.Option {
	return []container.Option{
		container.SplitVertical(
			container.Left(
				container.Border(linestyle.Light),
				container.PlaceWidget(p.statsWidget),
			),
			container.Right(
				container.Border(linestyle.Light),
				container.BorderTitle(p.title),
				container.BorderColor(p.color),
				container.PlaceWidget(p.widget),
			),
			container.SplitPercent(statsPercent),
		),
	}
}

// =========================================================================
// Refresh
// =========================================================================

func refreshCharts(panels []chartPanel, sm *seriesManager, vs *viewState, chartWidth int) {
	for i := range panels {
		panels[i].refresh(sm, vs, chartWidth)
	}
}

func refreshStats(panels []chartPanel, sm *seriesManager, showHelp bool) {
	for i := range panels {
		// Show help in the first panel's stats slot when toggled, so the
		// user can find it without it duplicating across every row.
		refreshPanelStats(&panels[i], sm, showHelp && i == 0)
	}
}

func refreshPanelStats(p *chartPanel, sm *seriesManager, showHelp bool) {
	p.statsWidget.Reset()
	var b strings.Builder

	if showHelp {
		b.WriteString(" ── Keys ─────\n")
		b.WriteString(" h/← pan left\n")
		b.WriteString(" l/→ pan right\n")
		b.WriteString(" +/= zoom in\n")
		b.WriteString(" -   zoom out\n")
		b.WriteString(" r   reset\n")
		b.WriteString(" ?   help\n")
		b.WriteString(" q   quit\n")
		_ = p.statsWidget.Write(b.String())
		return
	}

	st := sm.stats(p.idx)
	if st.Count == 0 {
		_ = p.statsWidget.Write(" (no data)\n")
		return
	}

	latest, _ := sm.latest(p.idx)

	// Use the row's label column (e.g. timestamp from the data) if
	// present; fall back to wall-clock time for streams without labels.
	timeStr := time.Now().Format("15:04:05")
	if label := sm.label(); label != "" {
		timeStr = label
	}

	b.WriteString(fmt.Sprintf(" Time    %s\n", timeStr))
	b.WriteString(fmt.Sprintf(" Value   %s\n", datadash.FormatValue(latest)))
	b.WriteString(fmt.Sprintf(" Count   %d\n", st.Count))
	b.WriteString(fmt.Sprintf(" Min     %s\n", datadash.FormatValue(st.Min)))
	b.WriteString(fmt.Sprintf(" Mean    %s\n", datadash.FormatValue(st.Mean)))
	b.WriteString(fmt.Sprintf(" Median  %s\n", datadash.FormatValue(st.P50)))
	b.WriteString(fmt.Sprintf(" Max     %s\n", datadash.FormatValue(st.Max)))
	b.WriteString(fmt.Sprintf(" Outlier %d\n", st.Outliers))

	_ = p.statsWidget.Write(b.String())
}

// =========================================================================
// Helpers
// =========================================================================

// downsampleFor reduces `data` to roughly `target` points using the
// configured algorithm. Returns the input unchanged when downsampling
// isn't beneficial (data already smaller than target, or disabled).
func downsampleFor(data []float64, target int) []float64 {
	if len(data) <= target {
		return data
	}
	switch cfg.downsample {
	case "none":
		return data
	case "minmax":
		return datadash.MinMaxDecimate(data, target)
	default:
		return datadash.LTTB(data, target)
	}
}

// scaleToInts maps a float slice into the [0, scale] integer range,
// shifting the minimum to 0. Used by bar charts where each call
// replaces all bars (so per-call min/max is fine).
func scaleToInts(data []float64, scale int) ([]int, int) {
	if len(data) == 0 {
		return nil, scale
	}
	min, max := data[0], data[0]
	for _, v := range data {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return scaleWithRange(data, min, max, scale), scale
}

// scaleWithRange maps floats into [0, scale] using a fixed min/max
// range. Used by sparklines where the range must be consistent across
// incremental Add() calls to avoid visual jumps.
func scaleWithRange(data []float64, min, max float64, scale int) []int {
	out := make([]int, len(data))

	if min == max {
		for i := range out {
			out[i] = scale / 2
		}
		return out
	}

	span := max - min
	for i, v := range data {
		n := int((v - min) / span * float64(scale))
		// Clamp — float precision can push values just outside [0, scale].
		if n < 0 {
			n = 0
		}
		if n > scale {
			n = scale
		}
		out[i] = n
	}
	return out
}

// repeat returns a slice of length n filled with c.
func repeat(c cell.Color, n int) []cell.Color {
	out := make([]cell.Color, n)
	for i := range out {
		out[i] = c
	}
	return out
}

// rollingAverage computes a windowed moving average over data.
func rollingAverage(data []float64, window int) []float64 {
	n := len(data)
	if n == 0 {
		return nil
	}
	if window <= 0 || window > n {
		window = n
	}
	avg := make([]float64, n)
	sum := 0.0
	cnt := 0
	for i := 0; i < n; i++ {
		sum += data[i]
		cnt++
		if cnt > window {
			sum -= data[i-window]
			cnt = window
		}
		avg[i] = sum / float64(cnt)
	}
	return avg
}

// =========================================================================
// View state
// =========================================================================

type viewState struct {
	mu     sync.Mutex
	offset int
	zoom   float64
}

func newViewState() *viewState { return &viewState{zoom: 1.0} }

func (v *viewState) panLeft(step int) { v.mu.Lock(); v.offset += step; v.mu.Unlock() }
func (v *viewState) panRight(step int) {
	v.mu.Lock()
	v.offset -= step
	if v.offset < 0 {
		v.offset = 0
	}
	v.mu.Unlock()
}
func (v *viewState) zoomIn()  { v.mu.Lock(); v.zoom = math.Max(v.zoom*0.7, 0.05); v.mu.Unlock() }
func (v *viewState) zoomOut() { v.mu.Lock(); v.zoom = math.Min(v.zoom*1.4, 20.0); v.mu.Unlock() }
func (v *viewState) reset()   { v.mu.Lock(); v.offset = 0; v.zoom = 1.0; v.mu.Unlock() }
func (v *viewState) get() (int, float64) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.offset, v.zoom
}

// =========================================================================
// Series manager
// =========================================================================

type seriesManager struct {
	mu        sync.RWMutex
	rings     []*datadash.Ring
	headers   []string
	count     int
	lastLabel string // most recent X-axis label from the data
}

func newSeriesManager(numSeries, capacity int) *seriesManager {
	if numSeries > maxSeries {
		numSeries = maxSeries
	}
	rings := make([]*datadash.Ring, numSeries)
	for i := range rings {
		rings[i] = datadash.NewRing(capacity)
	}
	return &seriesManager{rings: rings, count: numSeries}
}

func (sm *seriesManager) push(values []float64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for len(values) > len(sm.rings) && len(sm.rings) < maxSeries {
		c := defaultRingCap
		if len(sm.rings) > 0 {
			c = sm.rings[0].Cap()
		}
		sm.rings = append(sm.rings, datadash.NewRing(c))
		sm.count = len(sm.rings)
	}
	for i := 0; i < sm.count && i < len(values); i++ {
		sm.rings[i].Push(values[i])
	}
}

func (sm *seriesManager) setHeaders(h []string) {
	sm.mu.Lock()
	sm.headers = h
	sm.mu.Unlock()
}

func (sm *seriesManager) seriesCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.count
}

func (sm *seriesManager) seriesName(idx int) string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.headers != nil {
		// For CSV/TSV with a label column, headers has one more entry
		// than sm.count (e.g. ["time","cpu","mem"] for 2 series).
		// For JSON, headers matches sm.count exactly (["cpu","mem"]).
		// Compute the offset rather than hard-coding +1.
		offset := 0
		if len(sm.headers) > sm.count {
			offset = len(sm.headers) - sm.count
		}
		hi := idx + offset
		if hi < len(sm.headers) {
			return sm.headers[hi]
		}
	}
	return fmt.Sprintf("series_%d", idx+1)
}

func (sm *seriesManager) slice(idx, visibleWidth, offset int, zoom float64) []float64 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if idx >= sm.count {
		return nil
	}
	all := sm.rings[idx].Values()
	if len(all) == 0 {
		return nil
	}
	window := int(float64(visibleWidth*brailleMultiplier) * zoom)
	if window < 10 {
		window = 10
	}
	if window > len(all) {
		window = len(all)
	}
	end := len(all) - offset
	if end > len(all) {
		end = len(all)
	}
	if end <= 0 {
		return nil
	}
	start := end - window
	if start < 0 {
		start = 0
	}
	return all[start:end]
}

// sliceSince returns values written since the given monotonic position,
// along with the new high-water mark. Used by the sparkline updater so
// it emits each value to the widget exactly once, even after many ring
// overflows.
func (sm *seriesManager) sliceSince(idx int, since int64) ([]float64, int64) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if idx >= sm.count {
		return nil, since
	}
	return sm.rings[idx].Since(since)
}

func (sm *seriesManager) stats(idx int) datadash.Stats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if idx >= sm.count {
		return datadash.Stats{}
	}
	return sm.rings[idx].Stats()
}

func (sm *seriesManager) latest(idx int) (float64, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if idx >= sm.count {
		return 0, false
	}
	return sm.rings[idx].Latest()
}

func (sm *seriesManager) setLastLabel(label string) {
	sm.mu.Lock()
	sm.lastLabel = label
	sm.mu.Unlock()
}

func (sm *seriesManager) label() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.lastLabel
}

// =========================================================================
// Helpers
// =========================================================================

type atomBool struct {
	mu sync.Mutex
	v  bool
}

func (a *atomBool) get() bool { a.mu.Lock(); defer a.mu.Unlock(); return a.v }
func (a *atomBool) toggle()   { a.mu.Lock(); a.v = !a.v; a.mu.Unlock() }