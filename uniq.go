package datadash

// Uniq returns a new slice with duplicates removed, preserving
// the order of first occurrence. Works for any comparable type.
func Uniq[T comparable](input []T) []T {
	seen := make(map[T]struct{}, len(input))
	out := make([]T, 0, len(input))
	for _, v := range input {
		if _, exists := seen[v]; !exists {
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	return out
}

// UniqStrings is a convenience alias for string slices.
func UniqStrings(input []string) []string {
	return Uniq(input)
}