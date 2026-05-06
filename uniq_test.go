package datadash

import "testing"

func TestUniq_Strings(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{"empty", []string{}, []string{}},
		{"nil", nil, []string{}},
		{"all unique", []string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"all same", []string{"x", "x", "x"}, []string{"x"}},
		{"mixed", []string{"a", "b", "a", "c", "b"}, []string{"a", "b", "c"}},
		{"preserves first occurrence order",
			[]string{"c", "a", "b", "a", "c"}, []string{"c", "a", "b"}},
		{"empty strings count", []string{"", "a", "", "b"}, []string{"", "a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Uniq(tt.input)
			if !equalStrings(got, tt.want) {
				t.Errorf("Uniq(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestUniq_Ints(t *testing.T) {
	got := Uniq([]int{1, 2, 1, 3, 2, 1, 4})
	want := []int{1, 2, 3, 4}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("position %d: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestUniq_Float64(t *testing.T) {
	got := Uniq([]float64{1.5, 2.5, 1.5, 3.5})
	want := []float64{1.5, 2.5, 3.5}
	if !equalFloats(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestUniqStrings_Alias(t *testing.T) {
	got := UniqStrings([]string{"a", "b", "a"})
	want := []string{"a", "b"}
	if !equalStrings(got, want) {
		t.Errorf("UniqStrings = %v, want %v", got, want)
	}
}

func BenchmarkUniq_Strings(b *testing.B) {
	input := make([]string, 1000)
	for i := range input {
		input[i] = string(rune('a' + i%26))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Uniq(input)
	}
}
