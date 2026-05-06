package datadash

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// ---------------------------------------------------------------------------
// Public types
// ---------------------------------------------------------------------------

// Format describes the detected or forced input format.
type Format int

const (
	FormatAuto  Format = iota // sniff from first line
	FormatCSV                 // comma-delimited
	FormatTSV                 // tab-delimited
	FormatSpace               // whitespace-delimited
	FormatJSON                // newline-delimited JSON objects
	FormatSingle              // one number per line
)

func (f Format) String() string {
	switch f {
	case FormatCSV:
		return "csv"
	case FormatTSV:
		return "tsv"
	case FormatSpace:
		return "space"
	case FormatJSON:
		return "json"
	case FormatSingle:
		return "single"
	default:
		return "auto"
	}
}

// ParseFormat converts a CLI string to a Format constant.
func ParseFormat(s string) Format {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "csv":
		return FormatCSV
	case "tsv":
		return FormatTSV
	case "space":
		return FormatSpace
	case "json", "ndjson":
		return FormatJSON
	case "single":
		return FormatSingle
	default:
		return FormatAuto
	}
}

// Row is a single parsed record.
type Row struct {
	Values []float64
	Label  string // optional X-axis label (first column or timestamp)
}

// ---------------------------------------------------------------------------
// Reader options
// ---------------------------------------------------------------------------

// ReaderOption configures a Reader.
type ReaderOption func(*Reader)

// WithFormat forces a specific input format instead of auto-detecting.
func WithFormat(f Format) ReaderOption {
	return func(r *Reader) { r.format = f }
}

// WithDelimiter overrides the auto-detected delimiter for CSV/TSV.
func WithDelimiter(d string) ReaderOption {
	return func(r *Reader) { r.forcedDelim = d }
}

// WithJSONFields specifies dot-path fields to extract from JSON objects.
// If empty, all top-level numeric fields are auto-discovered.
func WithJSONFields(fields []string) ReaderOption {
	return func(r *Reader) { r.jsonFields = fields }
}

// WithLabelMode sets how the X-axis label is derived.
//
//	"first" — use the first column as a label (default for multi-column)
//	"time"  — use wall-clock time as the label
//	"none"  — no labels
func WithLabelMode(mode string) ReaderOption {
	return func(r *Reader) { r.labelMode = strings.ToLower(mode) }
}

// ---------------------------------------------------------------------------
// Reader
// ---------------------------------------------------------------------------

// Reader wraps a buffered scanner and produces Rows.
type Reader struct {
	scanner    *bufio.Scanner
	format     Format
	forcedDelim string
	delimiter  string // resolved after sniffing
	labelMode  string
	jsonFields []string

	headers  []string
	detected bool // true after the first line has been inspected
	lineNum  int
}

// NewReader creates a streaming row reader. By default it auto-detects
// the format and delimiter from the first line of input.
func NewReader(r io.Reader, opts ...ReaderOption) *Reader {
	rd := &Reader{
		scanner:   bufio.NewScanner(r),
		format:    FormatAuto,
		labelMode: "first",
	}
	// Increase scanner buffer for wide rows (default 64 KB → 1 MB).
	rd.scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for _, o := range opts {
		o(rd)
	}
	return rd
}

// Headers returns column headers discovered from the first line.
// Returns nil if headers haven't been detected yet or the format has
// no header row.
func (rd *Reader) Headers() []string { return rd.headers }

// Format returns the resolved format (after at least one Read call).
func (rd *Reader) Format() Format { return rd.format }

// Read returns the next Row. Returns io.EOF at end of input.
func (rd *Reader) Read() (Row, error) {
	for {
		if !rd.scanner.Scan() {
			if err := rd.scanner.Err(); err != nil {
				return Row{}, fmt.Errorf("line %d: %w", rd.lineNum, err)
			}
			return Row{}, io.EOF
		}
		rd.lineNum++
		line := rd.scanner.Text()

		// Skip blank lines.
		if strings.TrimSpace(line) == "" {
			continue
		}

		// First non-blank line: detect format, possibly consume as header.
		if !rd.detected {
			rd.detected = true
			if err := rd.detect(line); err != nil {
				return Row{}, fmt.Errorf("line %d: %w", rd.lineNum, err)
			}
			// If the first line was consumed as a header, loop for data.
			if rd.headers != nil && rd.format != FormatJSON {
				continue
			}
		}

		row, err := rd.parseLine(line)
		if err != nil {
			// Skip unparseable lines in streaming mode.
			continue
		}
		return row, nil
	}
}

// ---------------------------------------------------------------------------
// Detection
// ---------------------------------------------------------------------------

func (rd *Reader) detect(line string) error {
	// If format was forced, resolve delimiter and discover JSON fields.
	if rd.format != FormatAuto {
		rd.resolveDelimiter()
		if rd.format == FormatJSON {
			trimmed := strings.TrimSpace(line)
			if len(rd.jsonFields) == 0 {
				fields, err := discoverJSONFields(trimmed)
				if err != nil {
					return err
				}
				rd.jsonFields = fields
			}
			rd.headers = rd.jsonFields
		} else {
			rd.tryHeader(line)
		}
		return nil
	}

	// JSON: starts with '{'
	trimmed := strings.TrimSpace(line)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		rd.format = FormatJSON
		// Discover fields from first object.
		if len(rd.jsonFields) == 0 {
			fields, err := discoverJSONFields(trimmed)
			if err != nil {
				return err
			}
			rd.jsonFields = fields
			rd.headers = fields
		} else {
			rd.headers = rd.jsonFields
		}
		return nil
	}

	// Delimiter sniffing: count candidate separators.
	tabs := strings.Count(line, "\t")
	commas := strings.Count(line, ",")
	pipes := strings.Count(line, "|")

	switch {
	case tabs > 0 && tabs >= commas && tabs >= pipes:
		rd.format = FormatTSV
		rd.delimiter = "\t"
	case commas > 0 && commas >= pipes:
		rd.format = FormatCSV
		rd.delimiter = ","
	case pipes > 0:
		rd.format = FormatCSV
		rd.delimiter = "|"
	default:
		// Check for whitespace-separated numbers.
		fields := strings.Fields(line)
		if len(fields) > 1 {
			rd.format = FormatSpace
			rd.delimiter = "" // will use Fields()
		} else {
			rd.format = FormatSingle
		}
	}

	rd.tryHeader(line)
	return nil
}

func (rd *Reader) resolveDelimiter() {
	if rd.forcedDelim != "" {
		rd.delimiter = rd.forcedDelim
		return
	}
	switch rd.format {
	case FormatCSV:
		rd.delimiter = ","
	case FormatTSV:
		rd.delimiter = "\t"
	}
}

// tryHeader checks whether the first line looks like a header row
// (i.e. the majority of fields are non-numeric strings).
func (rd *Reader) tryHeader(line string) {
	fields := rd.splitLine(line)
	if len(fields) == 0 {
		return
	}
	nonNumeric := 0
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if _, err := strconv.ParseFloat(f, 64); err != nil && f != "" {
			nonNumeric++
		}
	}
	// If more than half are non-numeric, treat as header.
	if nonNumeric > len(fields)/2 {
		rd.headers = make([]string, len(fields))
		for i, f := range fields {
			rd.headers[i] = strings.TrimSpace(f)
		}
	}
}

// ---------------------------------------------------------------------------
// Line parsing
// ---------------------------------------------------------------------------

func (rd *Reader) parseLine(line string) (Row, error) {
	switch rd.format {
	case FormatJSON:
		return rd.parseJSON(line)
	case FormatSingle:
		return rd.parseSingle(line)
	default:
		return rd.parseDelimited(line)
	}
}

func (rd *Reader) parseDelimited(line string) (Row, error) {
	fields := rd.splitLine(line)
	if len(fields) == 0 {
		return Row{}, fmt.Errorf("empty row")
	}

	var row Row
	startCol := 0

	// If label mode is "first" and there are 2+ columns, treat
	// column 0 as the X-axis label.
	if rd.labelMode == "first" && len(fields) >= 2 {
		first := strings.TrimSpace(fields[0])
		if _, err := strconv.ParseFloat(first, 64); err != nil {
			row.Label = first
			startCol = 1
		}
	}

	for i := startCol; i < len(fields); i++ {
		f := strings.TrimSpace(fields[i])
		if f == "" {
			continue
		}
		// Strip common non-numeric decoration.
		f = stripDecorations(f)
		v, err := strconv.ParseFloat(f, 64)
		if err != nil {
			continue
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		row.Values = append(row.Values, v)
	}

	if len(row.Values) == 0 {
		return Row{}, fmt.Errorf("no numeric values in row")
	}
	return row, nil
}

func (rd *Reader) parseSingle(line string) (Row, error) {
	f := strings.TrimSpace(line)
	f = stripDecorations(f)
	v, err := strconv.ParseFloat(f, 64)
	if err != nil {
		return Row{}, fmt.Errorf("not a number: %q", f)
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return Row{}, fmt.Errorf("NaN/Inf value")
	}
	return Row{Values: []float64{v}}, nil
}

func (rd *Reader) parseJSON(line string) (Row, error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || trimmed[0] != '{' {
		return Row{}, fmt.Errorf("not a JSON object")
	}

	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
		return Row{}, fmt.Errorf("invalid JSON: %w", err)
	}

	var row Row
	for _, field := range rd.jsonFields {
		v, err := extractJSONFloat(obj, field)
		if err != nil {
			v = 0 // default missing fields to zero
		}
		row.Values = append(row.Values, v)
	}

	if len(row.Values) == 0 {
		return Row{}, fmt.Errorf("no numeric fields extracted")
	}
	return row, nil
}

// ---------------------------------------------------------------------------
// JSON helpers
// ---------------------------------------------------------------------------

// discoverJSONFields parses a JSON object and returns all top-level
// keys whose values are numeric, in sorted order for determinism.
func discoverJSONFields(line string) ([]string, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		return nil, fmt.Errorf("JSON discovery: %w", err)
	}

	var fields []string
	for k, v := range obj {
		switch v.(type) {
		case float64, json.Number:
			fields = append(fields, k)
		}
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("no numeric fields in JSON object")
	}

	// Sort for deterministic column order.
	sortStrings(fields)
	return fields, nil
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		key := s[i]
		j := i - 1
		for j >= 0 && s[j] > key {
			s[j+1] = s[j]
			j--
		}
		s[j+1] = key
	}
}

// extractJSONFloat navigates a dot-separated path into a JSON object
// and returns the value as a float64.
//
//	"cpu"             → obj["cpu"]
//	"metrics.cpu"     → obj["metrics"]["cpu"]
func extractJSONFloat(obj map[string]interface{}, path string) (float64, error) {
	parts := strings.Split(path, ".")
	var current interface{} = obj

	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return 0, fmt.Errorf("field %q: parent is not an object", part)
		}
		current, ok = m[part]
		if !ok {
			return 0, fmt.Errorf("field %q: not found", part)
		}
	}

	switch v := current.(type) {
	case float64:
		return v, nil
	case json.Number:
		return v.Float64()
	case string:
		return strconv.ParseFloat(v, 64)
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("field is not numeric (type %T)", current)
	}
}

// ---------------------------------------------------------------------------
// Splitting & decoration helpers
// ---------------------------------------------------------------------------

func (rd *Reader) splitLine(line string) []string {
	if rd.format == FormatSpace || rd.delimiter == "" {
		return strings.Fields(line)
	}
	return strings.Split(line, rd.delimiter)
}

// stripDecorations removes currency symbols, percent signs, commas in
// numbers, and surrounding whitespace so "$ 1,234.56%" → "1234.56".
func stripDecorations(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
			b.WriteRune(c)
		case c == '.' || c == '-' || c == '+':
			b.WriteRune(c)
		case c == 'e' || c == 'E': // scientific notation
			b.WriteRune(c)
		case unicode.IsSpace(c), c == '$', c == '€', c == '£',
			c == '¥', c == '%', c == ',':
			// strip
		default:
			// Unknown char — keep it and let ParseFloat reject if invalid.
			b.WriteRune(c)
		}
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Formatting helpers (used by the main app for axis labels)
// ---------------------------------------------------------------------------

// FormatValue produces a human-friendly string for a numeric value,
// using SI suffixes for large numbers and scientific notation for
// very small ones.
func FormatValue(v float64) string {
	if v == 0 {
		return "0"
	}
	abs := math.Abs(v)
	switch {
	case abs >= 1_000_000_000:
		return fmt.Sprintf("%.1fG", v/1_000_000_000)
	case abs >= 1_000_000:
		return fmt.Sprintf("%.1fM", v/1_000_000)
	case abs >= 10_000:
		return fmt.Sprintf("%.1fK", v/1_000)
	case abs >= 1_000:
		return fmt.Sprintf("%.0f", v)
	case abs >= 1:
		return fmt.Sprintf("%.1f", v)
	case abs >= 0.01:
		return fmt.Sprintf("%.2f", v)
	default:
		return fmt.Sprintf("%.1e", v)
	}
}

// FormatAxisValue is like FormatValue but right-pads to a fixed
// width for aligned Y-axis display.
func FormatAxisValue(v float64, width int) string {
	s := FormatValue(v)
	if len(s) >= width {
		return s
	}
	return strings.Repeat(" ", width-len(s)) + s
}