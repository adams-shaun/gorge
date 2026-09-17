package searchprobe

import (
	"fmt"
	"math"
)

type floatRandom interface{ Float64() float64 }

func normalizeWeights(logWeights []float64) ([]float64, float64, error) {
	max := math.Inf(-1)
	for _, w := range logWeights {
		if math.IsNaN(w) || math.IsInf(w, 1) {
			return nil, 0, fmt.Errorf("nonfinite log weight")
		}
		if w > max {
			max = w
		}
	}
	if math.IsInf(max, -1) {
		return nil, 0, fmt.Errorf("no positive proposal weight")
	}
	out := make([]float64, len(logWeights))
	total := 0.0
	for i, w := range logWeights {
		out[i] = math.Exp(w - max)
		total += out[i]
	}
	squares := 0.0
	for i := range out {
		out[i] /= total
		squares += out[i] * out[i]
	}
	return out, 1 / squares, nil
}
func weightedIndices(weights []float64, count int, r floatRandom) ([]int, error) {
	if count < 0 || r == nil {
		return nil, fmt.Errorf("invalid resampling request")
	}
	total := 0.0
	lastPositive := -1
	for i, w := range weights {
		if w < 0 || math.IsNaN(w) || math.IsInf(w, 0) {
			return nil, fmt.Errorf("invalid sampling weight")
		}
		total += w
		if w > 0 {
			lastPositive = i
		}
	}
	if total <= 0 || math.IsInf(total, 0) {
		return nil, fmt.Errorf("no finite positive sampling mass")
	}
	out := make([]int, count)
	for i := range out {
		u := r.Float64() * total
		cumulative := 0.0
		out[i] = lastPositive // rounding at the upper edge must not select a zero
		for j, w := range weights {
			cumulative += w
			if u < cumulative {
				out[i] = j
				break
			}
		}
	}
	return out, nil
}
