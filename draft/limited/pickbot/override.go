package pickbot

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

// RatingsOverride is a training-only replacement for the embedded rating
// file: per-card values from an external JSON file (cmd/cardval output,
// {"Card": {"gih": x, ...}}), affinely rescaled onto the set's 17Lands
// gih_wr scale (same mean and population std) so the drafter's tuning
// constants keep their meaning. A card the file lacks gets the 17Lands
// mean, i.e. a centred-neutral value; it never falls back to 17Lands.
type RatingsOverride struct {
	values  map[string]float64
	neutral float64
	// Missing counts set cards rated by 17Lands but absent from the file.
	Missing []string
}

// Value is the card's rescaled rating (a gih_wr-equivalent win rate).
func (o *RatingsOverride) Value(name string) float64 {
	if v, ok := o.values[name]; ok {
		return v
	}
	return o.neutral
}

// The fixed gih_wr-equivalent scale an override takes on a set with no
// embedded 17Lands file: roughly FDN's own gih_wr mean and
// population std, so the drafter's tuning constants keep their meaning.
const (
	FallbackScaleMean = 0.5425
	FallbackScaleStd  = 0.0435
)

func meanStd(xs []float64) (float64, float64) {
	m := 0.0
	for _, x := range xs {
		m += x
	}
	m /= float64(len(xs))
	v := 0.0
	for _, x := range xs {
		v += (x - m) * (x - m)
	}
	return m, math.Sqrt(v / float64(len(xs)))
}

// ParseRatingsOverride takes field from every card entry of the JSON in b
// (read from path by the caller; path only labels errors) and rescales it
// onto set's embedded 17Lands scale. A set with no embedded rating file (a
// cold start: BLB, a new set) takes the fixed fallback scale instead
// (FallbackScaleMean/FallbackScaleStd), and its Missing list is empty since
// there is no 17Lands card list to compare against. The package stays
// pure: no file I/O.
func ParseRatingsOverride(b []byte, path, field, set string) (*RatingsOverride, error) {
	ref, haveRef := rated[set]
	var raw map[string]map[string]float64
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("ratings override %s: %w", path, err)
	}
	names := make([]string, 0, len(raw))
	for n, e := range raw {
		if _, ok := e[field]; ok {
			names = append(names, n)
		}
	}
	if len(names) < 2 {
		return nil, fmt.Errorf("ratings override %s: fewer than 2 cards carry field %q", path, field)
	}
	sort.Strings(names)
	xs := make([]float64, len(names))
	for i, n := range names {
		xs[i] = raw[n][field]
	}
	var refNames []string
	my, sy := FallbackScaleMean, FallbackScaleStd
	if haveRef {
		refNames = make([]string, 0, len(ref.Cards))
		for n := range ref.Cards {
			refNames = append(refNames, n)
		}
		sort.Strings(refNames)
		ys := make([]float64, len(refNames))
		for i, n := range refNames {
			ys[i] = ref.Cards[n].GihWr
		}
		my, sy = meanStd(ys)
	}
	mx, sx := meanStd(xs)
	if sx == 0 {
		return nil, fmt.Errorf("ratings override %s: field %q has zero spread", path, field)
	}
	o := &RatingsOverride{values: make(map[string]float64, len(names)), neutral: my}
	for i, n := range names {
		o.values[n] = my + (xs[i]-mx)*sy/sx
	}
	for _, n := range refNames {
		if _, ok := o.values[n]; !ok {
			o.Missing = append(o.Missing, n)
		}
	}
	return o, nil
}

// NewRatingsOverride is a RatingsOverride over values taken as they are, with
// no rescale: each value is already a gih_wr-equivalent win rate (a 17Lands
// ratings file, or ratings derived from simulated games' GIH win rates, doc
// 039's kernel draft loop). A card values lacks gets neutral; Missing lists
// the set's 17Lands-rated cards values lacks (empty for a set with no
// embedded rating file). values is copied.
func NewRatingsOverride(values map[string]float64, neutral float64, set string) *RatingsOverride {
	o := &RatingsOverride{values: make(map[string]float64, len(values)), neutral: neutral}
	for n, v := range values {
		o.values[n] = v
	}
	if ref, ok := rated[set]; ok {
		for n := range ref.Cards {
			if _, have := o.values[n]; !have {
				o.Missing = append(o.Missing, n)
			}
		}
		sort.Strings(o.Missing)
	}
	return o
}
