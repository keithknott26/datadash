package datadash

import "sort"

// Interface to use the uniq package. Identical to sort.Interface.
type Interface interface {
	// Len returns the number of elements.
	Len() int
	// Less tells if the element at index i should come
	// before the element at index j.
	Less(i, j int) bool
	// Swap swaps the elements at indexes i and j.
	Swap(i, j int)
}

// Uniq moves the first unique elements to the beginning of the *sorted*
// collection and returns the number of unique elements.
//
// It makes one call to data.Len to determine n, n-1 calls to data.Less, and
// O(n) calls to data.Swap. The unique elements remain in original sorted order,
// but the duplicate elements do not.
func Uniq(data Interface) int {
	len := data.Len()
	if len <= 1 {
		return len
	}
	i, j := 0, 1
	// find the first duplicate
	for j < len && data.Less(i, j) {
		i++
		j++
	}
	// this loop is simpler after the first duplicate is found
	for ; j < len; j++ {
		if data.Less(i, j) {
			i++
			data.Swap(i, j)
		}
	}
	return i + 1
}

// Stable moves the first unique elements to the beginning of the *sorted*
// collection and returns the number of unique elements, but also keeps the
// original order of duplicate elements.
//
// It makes one call to data.Len, O(n) calls to data.Less, and O(n*log(n)) calls
// to data.Swap.
func Stable(data Interface) int {
	return stable(data, 0, data.Len())
}

func stable(data Interface, start, end int) int {
	if n := end - start; n <= 2 {
		if n == 2 && !data.Less(start, start+1) {
			n--
		}
		return n
	}
	mid := start + (end-start)/2 // average safe from overflow
	ua := stable(data, start, mid)
	ub := stable(data, mid, end)
	if ua > 0 && ub > 0 && !data.Less(start+ua-1, mid) {
		mid++ // the first element in B is present in A
		ub--
	}
	shift(data, start+ua, mid, mid+ub)
	return ua + ub
}

// IsUnique reports whether data is sorted and unique.
func IsUnique(data Interface) bool {
	n := data.Len() - 1
	for i := 0; i < n; i++ {
		if !data.Less(i, i+1) {
			return false
		}
	}
	return true
}

// Float64s calls unique on a slice of float64.
func Float64s(a []float64) int {
	return Uniq(sort.Float64Slice(a))
}

// Float64sAreUnique tests whether the slice of float64 is sorted and unique.
func Float64sAreUnique(a []float64) bool {
	return IsUnique(sort.Float64Slice(a))
}

// Ints calls unique on a slice of int.
func Ints(a []int) int {
	return Uniq(sort.IntSlice(a))
}

// IntsAreUnique tests whether the slice of int is sorted and unique.
func IntsAreUnique(a []int) bool {
	return IsUnique(sort.IntSlice(a))
}

// Strings calls unique on a slice of string.
func Strings(a []string) int {
	return Uniq(sort.StringSlice(a))
}

// StringsAreUnique tests whether the slice of string is sorted and unique.
func StringsAreUnique(a []string) bool {
	return IsUnique(sort.StringSlice(a))
}

// shift exchanges elements in a sort.Interface from range [start,mid) with
// those in range [mid,end).
//
// It makes n calls to data.Swap in the average & worst case, and n/2 calls to
// data.Swap in the best case.
func shift(data Interface, start, mid, end int) {
	if start >= mid || mid >= end {
		return // no elements to shift
	}
	if mid-start == end-mid {
		// equal sizes, use faster algorithm
		swapn(data, start, mid, mid-start)
		return
	}
	reverse(data, start, mid)
	reverse(data, mid, end)
	reverse(data, start, end)
}

// reverse transposes elements in a sort.Interface so that the elements in range
// [start,end) are in reverse order.
//
// It makes n/2 calls to data.Swap.
func reverse(data Interface, start, end int) {
	end--
	for start < end {
		data.Swap(start, end)
		start++
		end--
	}
}

// swapn swaps the elements in two sections of equal length in a sort.Interface.
// The sections start at indices i & j.
//
// If the sections overlap (i.e. min(i,j)+n > max(i,j)) the result is undefined.
// It makes n calls to data.Swap.
func swapn(data Interface, i, j, n int) {
	for n > 0 {
		n--
		data.Swap(i+n, j+n)
	}
}
