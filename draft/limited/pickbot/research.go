package pickbot

import (
	"math"
	"math/rand/v2"
)

// Research drafters (prototype, draft-research 2026-11-05): a uniform random
// picker and a signal-reading Committed variant.

// RandomPick takes a uniformly random card.
type RandomPick struct{}

func (RandomPick) Pick(v PickView, r *rand.Rand) string {
	if len(v.Pack) == 0 {
		return ""
	}
	return v.Pack[r.IntN(len(v.Pack))].ID
}

// Signals is Committed plus a table-read term: each colour's openness is the
// mean, over packs the seat has already seen from its 3rd pick of a round on,
// of the best centred rating among that colour's cards left in the pack when
// the seat saw it (what upstream seats passed). Openness is z-scored across
// the five colours and Weight times the card's mean z is added to its score
// from pack 1 pick From to the end of pack 2; pack 3 uses half weight.
// A card that wheels (seen in the same round's pick p and p+Seats) adds its
// colours' wheel evidence through the same packs, since a wheel pack is a
// seen pack late in the round.
type Signals struct {
	Committed
	Weight float64
	From   int
}

// Knobbed is the knob-file drafter: Committed under its Tuning's needs
// weight, plus the Signals term when Tuning.SignalWeight is positive, a
// per-colour bias (added per card colour, mean over the card's colours), a
// per-pair bias for a card of exactly that pair, and a denial term: from
// pick 10 on, a card off the seat's pair earns Deny*max(0, centred)/10
// (hate-drafting). Zero knobs (Tuning.Filled) leave every extra term off,
// so the zero Knobbed drafts exactly as Committed.
type Knobbed struct {
	Lookup  CardLookup
	Tuning  Tuning
	Ratings *RatingsOverride
}

func (d *Knobbed) Scores(v PickView, dst []float64) []float64 {
	t := d.Tuning.Filled()
	c := Committed{Lookup: d.Lookup, Tuning: t, Ratings: d.Ratings, NeedWeight: t.Need}
	if t.SignalWeight > 0 {
		s := Signals{Committed: c, Weight: t.SignalWeight, From: t.SignalStart}
		dst = s.Scores(v, dst)
	} else {
		dst = c.Scores(v, dst)
	}
	if d.Lookup == nil {
		return dst
	}
	rc := &RatingColour{Lookup: d.Lookup, Tuning: t, Ratings: d.Ratings}
	var pair uint8
	if len(v.Picks) >= 9 {
		pair = c.pair(v.Picks)
	}
	for i, ref := range v.Pack {
		m := factsOf(d.Lookup, ref.Name).mask
		b, k := 0.0, 0
		for col := 0; col < 5; col++ {
			if m&(1<<col) != 0 {
				b += t.Bias[col]
				k++
			}
		}
		if k > 0 {
			dst[i] += b / float64(k)
		}
		if k == 2 {
			for p, cp := range colourPairs {
				if m == uint8(1<<cp[0]|1<<cp[1]) {
					dst[i] += t.PairBias[p]
				}
			}
		}
		if t.Deny != 0 && pair != 0 && m != 0 && m&^pair != 0 {
			dst[i] += t.Deny * math.Max(0, rc.centred(v.Set, ref)) / 10
		}
	}
	return dst
}

func (d *Knobbed) Pick(v PickView, r *rand.Rand) string {
	if len(v.Pack) == 0 {
		return ""
	}
	return v.Pack[BestScore(d.Scores(v, nil), r)].ID
}

func (d *Signals) Scores(v PickView, dst []float64) []float64 {
	dst = d.Committed.Scores(v, dst)
	w := d.Weight
	if w < 0 {
		return dst
	}
	if w == 0 {
		w = 4
	}
	from := d.From
	if from == 0 {
		from = 4
	}
	if d.Lookup == nil || (v.Round == 1 && v.PickNo < from) {
		return dst
	}
	if v.Round == 3 {
		w /= 2
	}
	rc := &RatingColour{Lookup: d.Lookup, Tuning: d.tuning(), Ratings: d.Ratings}
	var sum [5]float64
	n := 0
	// Seen holds packs in pick order; reconstruct each pack's pick number
	// within its round from the pack sizes (a round's packs shrink by one).
	for i, pack := range v.Seen {
		if len(pack) == 0 {
			continue
		}
		// pick-in-round: the first pack of a round is the largest; count
		// back to the last size increase.
		pin := 1
		for j := i; j > 0 && len(v.Seen[j-1]) == len(v.Seen[j])+1; j-- {
			pin++
		}
		if pin < 3 {
			continue
		}
		var best [5]float64
		for c := range best {
			best[c] = math.Inf(-1)
		}
		for _, ref := range pack {
			m := factsOf(d.Lookup, ref.Name).mask
			s := rc.centred(v.Set, ref)
			for c := 0; c < 5; c++ {
				if m&(1<<c) != 0 && s > best[c] {
					best[c] = s
				}
			}
		}
		for c := 0; c < 5; c++ {
			if math.IsInf(best[c], -1) {
				best[c] = -15
			}
			sum[c] += best[c]
		}
		n++
	}
	if n < 2 {
		return dst
	}
	mean := 0.0
	for c := range sum {
		sum[c] /= float64(n)
		mean += sum[c] / 5
	}
	sd := 0.0
	for c := range sum {
		sd += (sum[c] - mean) * (sum[c] - mean) / 5
	}
	sd = math.Sqrt(sd)
	if sd < 1e-6 {
		return dst
	}
	for i, ref := range v.Pack {
		m := factsOf(d.Lookup, ref.Name).mask
		z, k := 0.0, 0
		for c := 0; c < 5; c++ {
			if m&(1<<c) != 0 {
				z += (sum[c] - mean) / sd
				k++
			}
		}
		if k > 0 {
			dst[i] += w * z / float64(k)
		}
	}
	return dst
}

func (d *Signals) Pick(v PickView, r *rand.Rand) string {
	if len(v.Pack) == 0 {
		return ""
	}
	return v.Pack[BestScore(d.Scores(v, nil), r)].ID
}
