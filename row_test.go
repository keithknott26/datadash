package datadash

import (
	"errors"
	"io"
	"strings"
	"testing"
)

// =========================================================================
// Format type & ParseFormat
// =========================================================================

func TestFormat_String(t *testing.T) {
	tests := []struct {
		f    Format
		want string
	}{
		{FormatAuto, "auto"},
		{FormatCSV, "csv"},
		{FormatTSV, "tsv"},
		{FormatSpace, "space"},
		{FormatJSON, "json"},
		{FormatSingle, "single"},
		{Format(99), "auto"}, // unknown defaults to "auto"
	}
	for _, tt := range tests {
		if got := tt.f.String(); got != tt.want {
			t.Errorf("Format(%d).String() = %q, want %q", tt.f, got, tt.want)
		}
	}
}

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input string
		want  Format
	}{
		{"csv", FormatCSV},
		{"CSV", FormatCSV},
		{" csv ", FormatCSV},
		{"tsv", FormatTSV},
		{"space", FormatSpace},
		{"json", FormatJSON},
		{"ndjson", FormatJSON},
		{"single", FormatSingle},
		{"auto", FormatAuto},
		{"", FormatAuto},
		{"unknown", FormatAuto},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParseFormat(tt.input); got != tt.want {
				t.Errorf("ParseFormat(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// =========================================================================
// Auto-detection
// =========================================================================

func TestReader_AutoDetect_TSV(t *testing.T) {
	input := "x\ty\tz\n1\t2\t3\n4\t5\t6\n"
	rd := NewReader(strings.NewReader(input))
	row, err := rd.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if rd.Format() != FormatTSV {
		t.Errorf("Format() = %v, want TSV", rd.Format())
	}
	want := []float64{1, 2, 3}
	if !equalFloats(row.Values, want) {
		t.Errorf("Values = %v, want %v", row.Values, want)
	}
	if h := rd.Headers(); !equalStrings(h, []string{"x", "y", "z"}) {
		t.Errorf("Headers = %v", h)
	}
}

func TestReader_AutoDetect_CSV(t *testing.T) {
	input := "a,b,c\n10,20,30\n40,50,60\n"
	rd := NewReader(strings.NewReader(input))
	row, err := rd.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if rd.Format() != FormatCSV {
		t.Errorf("Format() = %v, want CSV", rd.Format())
	}
	if !equalFloats(row.Values, []float64{10, 20, 30}) {
		t.Errorf("Values = %v", row.Values)
	}
}

func TestReader_AutoDetect_PipeDelimited(t *testing.T) {
	input := "1|2|3\n4|5|6\n"
	rd := NewReader(strings.NewReader(input))
	row, err := rd.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	// Pipe-delimited resolves to FormatCSV with '|' delimiter.
	if rd.Format() != FormatCSV {
		t.Errorf("Format() = %v, want CSV (pipe)", rd.Format())
	}
	if !equalFloats(row.Values, []float64{1, 2, 3}) {
		t.Errorf("Values = %v", row.Values)
	}
}

func TestReader_AutoDetect_Space(t *testing.T) {
	input := "1 2 3\n4 5 6\n"
	rd := NewReader(strings.NewReader(input))
	row, err := rd.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if rd.Format() != FormatSpace {
		t.Errorf("Format() = %v, want Space", rd.Format())
	}
	if !equalFloats(row.Values, []float64{1, 2, 3}) {
		t.Errorf("Values = %v", row.Values)
	}
}

func TestReader_AutoDetect_Single(t *testing.T) {
	input := "10\n20\n30\n"
	rd := NewReader(strings.NewReader(input))
	row, err := rd.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if rd.Format() != FormatSingle {
		t.Errorf("Format() = %v, want Single", rd.Format())
	}
	if !equalFloats(row.Values, []float64{10}) {
		t.Errorf("Values = %v", row.Values)
	}
}

func TestReader_AutoDetect_JSON(t *testing.T) {
	input := `{"cpu": 45.2, "mem": 72.1}
{"cpu": 50.0, "mem": 70.0}
`
	rd := NewReader(strings.NewReader(input))
	row, err := rd.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if rd.Format() != FormatJSON {
		t.Errorf("Format() = %v, want JSON", rd.Format())
	}
	if len(row.Values) != 2 {
		t.Fatalf("Values = %v, want 2 elements", row.Values)
	}
	// Headers should be sorted alphabetically.
	h := rd.Headers()
	if !equalStrings(h, []string{"cpu", "mem"}) {
		t.Errorf("Headers = %v, want [cpu mem]", h)
	}
}

// =========================================================================
// Forced format
// =========================================================================

func TestReader_ForcedJSON_DiscoversFields(t *testing.T) {
	// Regression: forcing -f json used to skip field discovery, leaving
	// jsonFields empty and producing zero rows.
	input := `{"a": 1.0, "b": 2.0}` + "\n" + `{"a": 3.0, "b": 4.0}` + "\n"
	rd := NewReader(strings.NewReader(input), WithFormat(FormatJSON))

	row, err := rd.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(row.Values) != 2 {
		t.Errorf("Values = %v, want 2 elements", row.Values)
	}
	if h := rd.Headers(); !equalStrings(h, []string{"a", "b"}) {
		t.Errorf("Headers = %v", h)
	}
}

func TestReader_ForcedJSON_WithFields(t *testing.T) {
	input := `{"x": 100, "y": 200, "z": 300}` + "\n"
	rd := NewReader(strings.NewReader(input),
		WithFormat(FormatJSON),
		WithJSONFields([]string{"y", "z"}),
	)
	row, err := rd.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !equalFloats(row.Values, []float64{200, 300}) {
		t.Errorf("Values = %v, want [200 300]", row.Values)
	}
}

func TestReader_ForcedCSV_WithDelimiter(t *testing.T) {
	input := "1;2;3\n4;5;6\n"
	rd := NewReader(strings.NewReader(input),
		WithFormat(FormatCSV),
		WithDelimiter(";"),
	)
	row, err := rd.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !equalFloats(row.Values, []float64{1, 2, 3}) {
		t.Errorf("Values = %v", row.Values)
	}
}

func TestReader_ForcedTSV(t *testing.T) {
	rd := NewReader(strings.NewReader("1\t2\n"), WithFormat(FormatTSV))
	row, _ := rd.Read()
	if !equalFloats(row.Values, []float64{1, 2}) {
		t.Errorf("Values = %v", row.Values)
	}
}

func TestReader_ForcedSpace(t *testing.T) {
	rd := NewReader(strings.NewReader("1   2   3\n"), WithFormat(FormatSpace))
	row, _ := rd.Read()
	if !equalFloats(row.Values, []float64{1, 2, 3}) {
		t.Errorf("Values = %v", row.Values)
	}
}

func TestReader_ForcedSingle(t *testing.T) {
	rd := NewReader(strings.NewReader("42\n"), WithFormat(FormatSingle))
	row, _ := rd.Read()
	if !equalFloats(row.Values, []float64{42}) {
		t.Errorf("Values = %v", row.Values)
	}
}

// =========================================================================
// Header detection
// =========================================================================

func TestReader_HeadersDetected(t *testing.T) {
	input := "time,foo,bar\n1,10,20\n2,30,40\n"
	rd := NewReader(strings.NewReader(input))
	_, _ = rd.Read()
	h := rd.Headers()
	if !equalStrings(h, []string{"time", "foo", "bar"}) {
		t.Errorf("Headers = %v", h)
	}
}

func TestReader_NoHeaders_AllNumeric(t *testing.T) {
	input := "1,2,3\n4,5,6\n"
	rd := NewReader(strings.NewReader(input))
	_, _ = rd.Read()
	if rd.Headers() != nil {
		t.Errorf("Headers = %v, want nil for all-numeric first row", rd.Headers())
	}
}

func TestReader_LabelMode_First(t *testing.T) {
	// First column non-numeric → used as label, dropped from values.
	input := "00:00\t10\t20\n00:01\t30\t40\n"
	rd := NewReader(strings.NewReader(input))
	row, err := rd.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if row.Label != "00:00" {
		t.Errorf("Label = %q, want %q", row.Label, "00:00")
	}
	if !equalFloats(row.Values, []float64{10, 20}) {
		t.Errorf("Values = %v, want [10 20]", row.Values)
	}
}

func TestReader_LabelMode_None(t *testing.T) {
	// Numeric first column with label-mode=none → all values kept.
	input := "1\t10\t20\n2\t30\t40\n"
	rd := NewReader(strings.NewReader(input), WithLabelMode("none"))
	row, _ := rd.Read()
	if row.Label != "" {
		t.Errorf("Label = %q, want empty", row.Label)
	}
	if !equalFloats(row.Values, []float64{1, 10, 20}) {
		t.Errorf("Values = %v, want [1 10 20]", row.Values)
	}
}

// =========================================================================
// JSON specifics
// =========================================================================

func TestReader_JSON_NestedFields(t *testing.T) {
	input := `{"host": "web1", "metrics": {"cpu": 45, "mem": 80}}` + "\n"
	rd := NewReader(strings.NewReader(input),
		WithFormat(FormatJSON),
		WithJSONFields([]string{"metrics.cpu", "metrics.mem"}),
	)
	row, err := rd.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !equalFloats(row.Values, []float64{45, 80}) {
		t.Errorf("Values = %v", row.Values)
	}
}

func TestReader_JSON_StringNumber(t *testing.T) {
	input := `{"price": "1234.56"}` + "\n"
	rd := NewReader(strings.NewReader(input),
		WithFormat(FormatJSON),
		WithJSONFields([]string{"price"}),
	)
	row, err := rd.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(row.Values) != 1 || row.Values[0] != 1234.56 {
		t.Errorf("Values = %v, want [1234.56]", row.Values)
	}
}

func TestReader_JSON_BooleanField(t *testing.T) {
	input := `{"healthy": true, "broken": false}` + "\n"
	rd := NewReader(strings.NewReader(input),
		WithFormat(FormatJSON),
		WithJSONFields([]string{"healthy", "broken"}),
	)
	row, _ := rd.Read()
	if !equalFloats(row.Values, []float64{1, 0}) {
		t.Errorf("Values = %v, want [1 0]", row.Values)
	}
}

func TestReader_JSON_MissingField_DefaultsToZero(t *testing.T) {
	input := `{"a": 5}` + "\n"
	rd := NewReader(strings.NewReader(input),
		WithFormat(FormatJSON),
		WithJSONFields([]string{"a", "missing"}),
	)
	row, _ := rd.Read()
	if !equalFloats(row.Values, []float64{5, 0}) {
		t.Errorf("Values = %v, want [5 0]", row.Values)
	}
}

func TestReader_JSON_InvalidLineSkipped(t *testing.T) {
	input := `not json
{"a": 1}
{also broken
{"a": 2}
`
	rd := NewReader(strings.NewReader(input), WithFormat(FormatJSON), WithJSONFields([]string{"a"}))
	first, err := rd.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if first.Values[0] != 1 {
		t.Errorf("first = %v, want 1", first.Values)
	}
	second, err := rd.Read()
	if err != nil {
		t.Fatalf("Read second: %v", err)
	}
	if second.Values[0] != 2 {
		t.Errorf("second = %v, want 2", second.Values)
	}
}

func TestReader_JSON_NonObjectSkipped(t *testing.T) {
	input := `[1,2,3]
{"a": 1}
`
	rd := NewReader(strings.NewReader(input), WithFormat(FormatJSON), WithJSONFields([]string{"a"}))
	row, err := rd.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if row.Values[0] != 1 {
		t.Errorf("Values = %v, want [1]", row.Values)
	}
}

func TestReader_JSON_AutoDiscover_NoNumeric(t *testing.T) {
	// All-string object → discovery error → row skipped.
	input := `{"a": "hello", "b": "world"}` + "\n" + `{"x": 1}` + "\n"
	rd := NewReader(strings.NewReader(input), WithFormat(FormatJSON))
	_, err := rd.Read()
	if err == nil {
		t.Error("expected error on all-string JSON, got nil")
	}
}

// =========================================================================
// Edge cases & malformed input
// =========================================================================

func TestReader_BlankLinesSkipped(t *testing.T) {
	input := "\n\n1,2,3\n\n4,5,6\n\n"
	rd := NewReader(strings.NewReader(input))

	r1, _ := rd.Read()
	if !equalFloats(r1.Values, []float64{1, 2, 3}) {
		t.Errorf("first row = %v", r1.Values)
	}
	r2, _ := rd.Read()
	if !equalFloats(r2.Values, []float64{4, 5, 6}) {
		t.Errorf("second row = %v", r2.Values)
	}
	if _, err := rd.Read(); !errors.Is(err, io.EOF) {
		t.Errorf("expected EOF, got %v", err)
	}
}

func TestReader_EmptyInput(t *testing.T) {
	rd := NewReader(strings.NewReader(""))
	if _, err := rd.Read(); !errors.Is(err, io.EOF) {
		t.Errorf("expected EOF on empty input, got %v", err)
	}
}

func TestReader_OnlyBlankLines(t *testing.T) {
	rd := NewReader(strings.NewReader("\n\n\n"))
	if _, err := rd.Read(); !errors.Is(err, io.EOF) {
		t.Errorf("expected EOF, got %v", err)
	}
}

func TestReader_MalformedRowsSkipped(t *testing.T) {
	input := "abc,def,ghi\n1,2,3\n"
	rd := NewReader(strings.NewReader(input))
	row, err := rd.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	// First line was treated as headers, so we get the second line.
	if !equalFloats(row.Values, []float64{1, 2, 3}) {
		t.Errorf("Values = %v", row.Values)
	}
}

func TestReader_PartiallyNumericRow(t *testing.T) {
	// Row with mixed text/numbers — text fields are silently dropped.
	input := "1,oops,3\n"
	rd := NewReader(strings.NewReader(input), WithFormat(FormatCSV))
	row, _ := rd.Read()
	if !equalFloats(row.Values, []float64{1, 3}) {
		t.Errorf("Values = %v, want [1 3]", row.Values)
	}
}

func TestReader_NaNAndInfRejected(t *testing.T) {
	input := "1,NaN,Inf,2\n"
	rd := NewReader(strings.NewReader(input), WithFormat(FormatCSV))
	row, _ := rd.Read()
	if !equalFloats(row.Values, []float64{1, 2}) {
		t.Errorf("Values = %v, want [1 2] (NaN/Inf dropped)", row.Values)
	}
}

func TestReader_DecorationsStripped(t *testing.T) {
	// A row of pure decorations would trigger the header heuristic
	// (all fields fail bare ParseFloat), so include a real header line
	// and use label-mode=none to keep all three values.
	input := "x\ty\tz\n$1,234.56\t99%\t€42.00\n"
	rd := NewReader(strings.NewReader(input),
		WithFormat(FormatTSV),
		WithLabelMode("none"),
	)
	row, err := rd.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	want := []float64{1234.56, 99, 42}
	if !equalFloats(row.Values, want) {
		t.Errorf("Values = %v, want %v", row.Values, want)
	}
}

func TestReader_ScientificNotation(t *testing.T) {
	rd := NewReader(strings.NewReader("1.5e3,2E-2\n"), WithFormat(FormatCSV))
	row, _ := rd.Read()
	if !equalFloats(row.Values, []float64{1500, 0.02}) {
		t.Errorf("Values = %v", row.Values)
	}
}

func TestReader_AllRowsStream(t *testing.T) {
	input := "1\n2\n3\n4\n5\n"
	rd := NewReader(strings.NewReader(input))

	var got []float64
	for {
		row, err := rd.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		got = append(got, row.Values[0])
	}
	if !equalFloats(got, []float64{1, 2, 3, 4, 5}) {
		t.Errorf("got %v, want [1 2 3 4 5]", got)
	}
}

// =========================================================================
// stripDecorations (private)
// =========================================================================

func TestStripDecorations(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"$1234", "1234"},
		{"1,234.56", "1234.56"},
		{"99%", "99"},
		{"€42.00", "42.00"},
		{"-5.5", "-5.5"},
		{"+10", "+10"},
		{"1.5e3", "1.5e3"},
		{"1.5E-2", "1.5E-2"},
		{"  42  ", "42"},
		{"£1,000,000", "1000000"},
		{"¥500", "500"},
		{"abc", "abc"}, // non-numeric chars passed through to ParseFloat
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := stripDecorations(tt.input); got != tt.want {
				t.Errorf("stripDecorations(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// =========================================================================
// FormatValue & FormatAxisValue
// =========================================================================

func TestFormatValue(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{1, "1.0"},
		{-1, "-1.0"},
		{42.5, "42.5"},
		{999, "999.0"}, // %.1f for 1 <= abs < 1000
		{1000, "1000"}, // %.0f for 1000 <= abs < 10000
		{9999, "9999"},
		{10000, "10.0K"}, // K suffix kicks in at 10000
		{1500000, "1.5M"},
		{2000000000, "2.0G"},
		{0.5, "0.50"},  // %.2f for 0.01 <= abs < 1
		{0.05, "0.05"}, // %.2f for 0.01 <= abs < 1
		{0.001, "1.0e-03"},
		{-1500000, "-1.5M"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := FormatValue(tt.in); got != tt.want {
				t.Errorf("FormatValue(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFormatAxisValue_Padding(t *testing.T) {
	tests := []struct {
		in    float64
		width int
		want  string
	}{
		{1, 8, "     1.0"},
		{1000000, 8, "    1.0M"},
		{0, 5, "    0"},
		{42.5, 4, "42.5"}, // exactly fits
		{42.5, 2, "42.5"}, // overflow returns unpadded
	}
	for _, tt := range tests {
		got := FormatAxisValue(tt.in, tt.width)
		if got != tt.want {
			t.Errorf("FormatAxisValue(%v, %d) = %q, want %q", tt.in, tt.width, got, tt.want)
		}
	}
}

// =========================================================================
// extractJSONFloat (private — exercised through JSON path, plus direct)
// =========================================================================

func TestExtractJSONFloat(t *testing.T) {
	obj := map[string]interface{}{
		"flat":   42.0,
		"strNum": "3.14",
		"yes":    true,
		"no":     false,
		"nested": map[string]interface{}{
			"deep": map[string]interface{}{
				"value": 99.9,
			},
		},
	}

	tests := []struct {
		path    string
		want    float64
		wantErr bool
	}{
		{"flat", 42, false},
		{"strNum", 3.14, false},
		{"yes", 1, false},
		{"no", 0, false},
		{"nested.deep.value", 99.9, false},
		{"missing", 0, true},
		{"flat.subfield", 0, true},        // descending into a non-object
		{"nested.missing.value", 0, true}, // missing intermediate
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got, err := extractJSONFloat(obj, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// =========================================================================
// sortStrings (private)
// =========================================================================

func TestSortStrings(t *testing.T) {
	s := []string{"charlie", "alpha", "bravo", "delta"}
	sortStrings(s)
	if !equalStrings(s, []string{"alpha", "bravo", "charlie", "delta"}) {
		t.Errorf("sortStrings = %v", s)
	}
}

func TestSortStrings_Empty(t *testing.T) {
	var s []string
	sortStrings(s) // shouldn't panic
}

// =========================================================================
// helpers
// =========================================================================

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}