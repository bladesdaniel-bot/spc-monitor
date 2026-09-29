package spc

import (
	"errors"
	"math"
)

// d2 is the bias-correction constant for a moving range of 2 consecutive points.
const d2 = 1.128

// d4 is the upper control limit factor for a moving range of 2 consecutive points.
const d4 = 3.267

type Limits struct {
	Center float64 `json:"center"`
	Sigma  float64 `json:"sigma"`
	UCL    float64 `json:"ucl"`
	LCL    float64 `json:"lcl"`
	MRBar  float64 `json:"mrBar"`
	MRUCL  float64 `json:"mrUcl"`
	N      int     `json:"n"`
}

var ErrNotEnoughData = errors.New("need at least 2 values to compute limits")

// MovingRanges returns the absolute jump between each pair of consecutive values.
// Entry i-1 is the jump that lands on value i.
func MovingRanges(values []float64) []float64 {
	if len(values) < 2 {
		return []float64{}
	}
	out := make([]float64, len(values)-1)
	for i := 1; i < len(values); i++ {
		out[i-1] = math.Abs(values[i] - values[i-1])
	}
	return out
}

func ComputeLimits(values []float64) (Limits, error) {
	n := len(values)
	if n < 2 {
		return Limits{}, ErrNotEnoughData
	}

	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(n)

	var mrSum float64
	for _, mr := range MovingRanges(values) {
		mrSum += mr
	}
	mrBar := mrSum / float64(n-1)
	sigma := mrBar / d2

	return Limits{
		Center: mean,
		Sigma:  sigma,
		UCL:    mean + 3*sigma,
		LCL:    mean - 3*sigma,
		MRBar:  mrBar,
		MRUCL:  d4 * mrBar,
		N:      n,
	}, nil
}

// CheckMR flags moving ranges above the MR chart's upper limit:
// a sudden jump between consecutive readings, even if the average hasn't moved.
func CheckMR(values []float64, l Limits) []Violation {
	out := []Violation{}
	if l.MRUCL <= 0 {
		return out
	}
	for i := 1; i < len(values); i++ {
		mr := math.Abs(values[i] - values[i-1])
		if mr > l.MRUCL {
			out = append(out, Violation{Rule: 1, Index: i, Value: mr, Description: "moving range above MR upper limit"})
		}
	}
	return out
}
