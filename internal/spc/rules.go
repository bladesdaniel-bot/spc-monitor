package spc

import "math"

type Violation struct {
	Rule        int     `json:"rule"`
	Index       int     `json:"index"`
	Value       float64 `json:"value"`
	Description string  `json:"description"`
}

func Check(values []float64, l Limits) []Violation {
	out := []Violation{}
	if l.Sigma <= 0 {
		return out
	}

	z := make([]float64, len(values))
	for i, v := range values {
		z[i] = (v - l.Center) / l.Sigma
	}

	for i := range z {
		if math.Abs(z[i]) > 3 {
			out = append(out, Violation{1, i, values[i], "1 point beyond 3 sigma"})
		}
		if i >= 2 && math.Abs(z[i]) > 2 && countSameSide(z[i-2:i+1], z[i], 2) >= 2 {
			out = append(out, Violation{2, i, values[i], "2 of 3 points beyond 2 sigma on the same side"})
		}
		if i >= 4 && math.Abs(z[i]) > 1 && countSameSide(z[i-4:i+1], z[i], 1) >= 4 {
			out = append(out, Violation{3, i, values[i], "4 of 5 points beyond 1 sigma on the same side"})
		}
		if i >= 7 && allSameSide(z[i-7:i+1]) {
			out = append(out, Violation{4, i, values[i], "8 points in a row on the same side of center"})
		}
	}
	return out
}

func countSameSide(window []float64, ref, k float64) int {
	n := 0
	for _, v := range window {
		if ref > 0 && v > k {
			n++
		}
		if ref < 0 && v < -k {
			n++
		}
	}
	return n
}

func allSameSide(window []float64) bool {
	above, below := true, true
	for _, v := range window {
		if v <= 0 {
			above = false
		}
		if v >= 0 {
			below = false
		}
	}
	return above || below
}
