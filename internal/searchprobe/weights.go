package searchprobe

import (
	"fmt"
	"math"
	"sort"
)

type WeightDiagnostics struct {
	Accepted                                                                                                int
	LogWeightMin, LogWeightMedian, LogWeightMax                                                             float64
	MaxNormalizedMass, Top4NormalizedMass                                                                   float64
	Mass50Count, Mass90Count                                                                                int
	IsolationEligibleAttempts, IsolationSelectedAttempts, IsolationInsideAttempts, IsolationOutsideAttempts int
	IsolationSelectedMass, IsolationOutsideMass                                                             float64
	IsolationSelectedSquaredShare, IsolationOutsideSquaredShare                                             float64
}

type proposalDiagnostics struct {
	isolationEligible, isolationSelected, isolationInside, isolationOutside int
}

func summarizeWeightDiagnostics(logs, weights []float64, proposals []proposalDiagnostics) WeightDiagnostics {
	if len(logs) == 0 || len(logs) != len(weights) || len(logs) != len(proposals) {
		return WeightDiagnostics{}
	}
	d := WeightDiagnostics{Accepted: len(logs)}
	sortedLogs := append([]float64(nil), logs...)
	sort.Float64s(sortedLogs)
	d.LogWeightMin = sortedLogs[0]
	d.LogWeightMax = sortedLogs[len(sortedLogs)-1]
	mid := len(sortedLogs) / 2
	if len(sortedLogs)%2 == 0 {
		d.LogWeightMedian = (sortedLogs[mid-1] + sortedLogs[mid]) / 2
	} else {
		d.LogWeightMedian = sortedLogs[mid]
	}
	sortedWeights := append([]float64(nil), weights...)
	sort.Sort(sort.Reverse(sort.Float64Slice(sortedWeights)))
	d.MaxNormalizedMass = sortedWeights[0]
	for i, weight := range sortedWeights {
		if i < 4 {
			d.Top4NormalizedMass += weight
		}
		if d.Mass50Count == 0 {
			var mass float64
			for _, w := range sortedWeights[:i+1] {
				mass += w
			}
			if mass+1e-12 >= 0.5 {
				d.Mass50Count = i + 1
			}
		}
		if d.Mass90Count == 0 {
			var mass float64
			for _, w := range sortedWeights[:i+1] {
				mass += w
			}
			if mass+1e-12 >= 0.9 {
				d.Mass90Count = i + 1
			}
		}
	}
	var squaredTotal, selectedSquared, outsideSquared float64
	for i, proposal := range proposals {
		weight := weights[i]
		squared := weight * weight
		squaredTotal += squared
		if proposal.isolationEligible > 0 {
			d.IsolationEligibleAttempts++
		}
		if proposal.isolationSelected > 0 {
			d.IsolationSelectedAttempts++
			d.IsolationSelectedMass += weight
			selectedSquared += squared
		}
		if proposal.isolationInside > 0 {
			d.IsolationInsideAttempts++
		}
		if proposal.isolationOutside > 0 {
			d.IsolationOutsideAttempts++
			d.IsolationOutsideMass += weight
			outsideSquared += squared
		}
	}
	if squaredTotal > 0 {
		d.IsolationSelectedSquaredShare = selectedSquared / squaredTotal
		d.IsolationOutsideSquaredShare = outsideSquared / squaredTotal
	}
	return d
}

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
