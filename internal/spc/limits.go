package spc

import (
	"errors"
	"math"
)

// d2 is the bias-correction constant for a moving range of 2 consecutive points.
const d2 = 1.128

type Limits struct {
	Center float64 `json:"center"`
	Sigma  float64 `json:"sigma"`
	UCL    float64 `json:"ucl"`
	LCL    float64 `json:"lcl"`
	N      int     `json:"n"`
}

var ErrNotEnoughData = errors.New("need at least 2 values to compute limits")

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
	for i := 1; i < n; i++ {
		mrSum += math.Abs(values[i] - values[i-1])
	}
	mrBar := mrSum / float64(n-1)
	sigma := mrBar / d2

	return Limits{
		Center: mean,
		Sigma:  sigma,
		UCL:    mean + 3*sigma,
		LCL:    mean - 3*sigma,
		N:      n,
	}, nil
}
