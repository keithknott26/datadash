package datadash

import "math"

// LTTB implements the Largest Triangle Three Buckets algorithm by Sveinn
// Steinarsson (2013). Given a series of N points, it returns a series of
// `threshold` points that visually preserves the original's shape — peaks,
// dips, and inflection points are kept, while runs of similar values are
// thinned out. This is what Grafana, Plotly, and most production charting
// libraries use for time-series downsampling.
//
// If threshold >= len(data) or threshold < 3, the original is returned
// unchanged.
//
// Runtime: O(n). Allocates one slice of size `threshold`.
func LTTB(data []float64, threshold int) []float64 {
	n := len(data)
	if threshold >= n || threshold < 3 {
		return data
	}

	sampled := make([]float64, 0, threshold)
	sampled = append(sampled, data[0]) // first point is always kept

	// Width of each bucket (excluding the two anchored endpoints).
	every := float64(n-2) / float64(threshold-2)

	a := 0 // index of the previously selected point

	for i := 0; i < threshold-2; i++ {
		// Compute the average point of the *next* bucket — this becomes
		// the third vertex of every triangle we'll consider in this bucket.
		avgStart := int(float64(i+1)*every) + 1
		avgEnd := int(float64(i+2)*every) + 1
		if avgEnd > n {
			avgEnd = n
		}

		avgX, avgY := 0.0, 0.0
		bucketLen := float64(avgEnd - avgStart)
		for j := avgStart; j < avgEnd; j++ {
			avgX += float64(j)
			avgY += data[j]
		}
		avgX /= bucketLen
		avgY /= bucketLen

		// Search the *current* bucket for the point that forms the largest
		// triangle with `a` (last selected) and the next bucket's average.
		rangeStart := int(float64(i)*every) + 1
		rangeEnd := int(float64(i+1)*every) + 1

		pAX := float64(a)
		pAY := data[a]

		maxArea := -1.0
		next := rangeStart
		for j := rangeStart; j < rangeEnd; j++ {
			// Triangle area = |½ · ((Ax−Cx)·(By−Ay) − (Ax−Bx)·(Cy−Ay))|
			area := math.Abs(
				(pAX-avgX)*(data[j]-pAY) - (pAX-float64(j))*(avgY-pAY),
			) * 0.5
			if area > maxArea {
				maxArea = area
				next = j
			}
		}

		sampled = append(sampled, data[next])
		a = next
	}

	sampled = append(sampled, data[n-1]) // last point is always kept
	return sampled
}

// MinMaxDecimate is a faster but lower-quality alternative to LTTB.
// It splits the input into `threshold/2` buckets and keeps the min and
// max of each. Useful when you want extremes preserved but don't care
// about visual fidelity of the curve between them — e.g. audio waveforms.
func MinMaxDecimate(data []float64, threshold int) []float64 {
	n := len(data)
	if threshold >= n || threshold < 4 {
		return data
	}

	buckets := threshold / 2
	out := make([]float64, 0, threshold)

	bucketSize := float64(n) / float64(buckets)
	for i := 0; i < buckets; i++ {
		start := int(float64(i) * bucketSize)
		end := int(float64(i+1) * bucketSize)
		if end > n {
			end = n
		}
		if start >= end {
			continue
		}

		mn, mx := data[start], data[start]
		mnIdx, mxIdx := start, start
		for j := start + 1; j < end; j++ {
			if data[j] < mn {
				mn = data[j]
				mnIdx = j
			}
			if data[j] > mx {
				mx = data[j]
				mxIdx = j
			}
		}

		// Emit in original order so the line draws correctly.
		if mnIdx < mxIdx {
			out = append(out, mn, mx)
		} else {
			out = append(out, mx, mn)
		}
	}
	return out
}
