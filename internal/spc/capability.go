package spc

import (
	"errors"
	"math"
)

// CapabilityWindow is how many recent readings capability is calculated over.
const CapabilityWindow = 50

// Spec holds the engineering limits a characteristic must stay within.
type Spec struct {
	LSL float64 `json:"lsl"`
	USL float64 `json:"usl"`
}

type Capability struct {
	Cp           float64 `json:"cp"`
	Cpk          float64 `json:"cpk"`
	Pp           float64 `json:"pp"`
	Ppk          float64 `json:"ppk"`
	Mean         float64 `json:"mean"`
	SigmaWithin  float64 `json:"sigmaWithin"`
	SigmaOverall float64 `json:"sigmaOverall"`
	N            int     `json:"n"`
}

var ErrBadSpec = errors.New("upper spec limit must be greater than lower spec limit")

// ComputeCapability measures how well the most recent readings fit inside the spec limits.
// Cp/Cpk use short-term (within) sigma from moving ranges; Pp/Ppk use overall standard deviation.
func ComputeCapability(values []float64, s Spec) (Capability, error) {
	if s.USL <= s.LSL {
		return Capability{}, ErrBadSpec
	}
	if len(values) > CapabilityWindow {
		values = values[len(values)-CapabilityWindow:]
	}
	n := len(values)
	if n < 2 {
		return Capability{}, ErrNotEnoughData
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
	within := mrSum / float64(n-1) / d2

	var sq float64
	for _, v := range values {
		sq += (v - mean) * (v - mean)
	}
	overall := math.Sqrt(sq / float64(n-1))

	c := Capability{Mean: mean, SigmaWithin: within, SigmaOverall: overall, N: n}
	width := s.USL - s.LSL
	nearest := math.Min(s.USL-mean, mean-s.LSL)
	if within > 0 {
		c.Cp = width / (6 * within)
		c.Cpk = nearest / (3 * within)
	}
	if overall > 0 {
		c.Pp = width / (6 * overall)
		c.Ppk = nearest / (3 * overall)
	}
	return c, nil
}
