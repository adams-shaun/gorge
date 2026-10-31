package pickbot

import (
	"math"
	"math/rand/v2"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/engine/cards"
)

// RatingColour is the spec §4.6 drafter, v1: it scores every pack card as
// its centred rating plus the colour term, which ramps in over pack 1, and
// breaks ties with the seeded rng. The zero value is ready to use with the
// default tuning; set Lookup to give the heuristic and the colour term
// their card facts.
type RatingColour struct {
	// Lookup resolves card names to card facts (colours, and the heuristic
	// path's type lines). The FDN rated path works without it.
	Lookup CardLookup
	Tuning Tuning
	// Ratings, when non-nil, replaces the embedded rating file as the rated
	// path (see ParseRatingsOverride); a card it lacks is neutral.
	Ratings *RatingsOverride
}

// tuning returns the drafter's knob set, filling the zero value with the
// spec defaults.
func (d *RatingColour) tuning() Tuning {
	if d.Tuning == (Tuning{}) {
		return DefaultTuning
	}
	return d.Tuning
}

// cardFacts is the slice of a card's printed facts the drafter and builder
// read: its colour identity as a WUBRG bitmask and its colour pips, one per
// mana symbol that carries the colour (a hybrid pip counts for each of its
// colours, via cards.ManaCostChroma).
type cardFacts struct {
	mask uint8
	pips [5]int
}

// factsOf resolves one card's facts through a Lookup; a nil Lookup or an
// unknown name is the no-colour, no-pip card.
func factsOf(lookup CardLookup, name string) cardFacts {
	var f cardFacts
	if lookup == nil {
		return f
	}
	c := lookup(name)
	if c == nil {
		return f
	}
	f.mask = c.ColourIdentity()
	for _, face := range c.Faces {
		for i := 0; i < 5; i++ {
			f.pips[i] += int(cards.ManaCostChroma(face.ManaCost, 1<<i))
		}
	}
	return f
}

// centred is a card's rating on the drafter's centred scale: the rated
// path's win rate re-centred, the heuristic's points shifted onto the same
// rough scale otherwise.
func (d *RatingColour) centred(set string, ref limited.CardRef) float64 {
	t := d.tuning()
	if d.Ratings != nil {
		return (d.Ratings.Value(ref.Name) - t.RatingCentre) * t.RatingScale
	}
	if e, ok := RawRating(set, ref.Name); ok {
		return (e.GihWr - t.RatingCentre) * t.RatingScale
	}
	h := Heuristic(d.Lookup(ref.Name), ref.Rarity)
	return (h - t.HeuristicCentre) * t.HeuristicScale
}

// colourFit is the candidate's colour term: the mean, over the card's own
// colours, of each colour's share of the seat's rating-weighted picks so
// far. A colourless card earns the fixed ColourlessFit share.
func (d *RatingColour) colourFit(mask uint8, picks []limited.CardRef, set string) float64 {
	t := d.tuning()
	if mask == 0 {
		return t.ColourlessFit
	}
	var weight [5]float64
	total := 0.0
	for _, p := range picks {
		w := d.ratingWeight(set, p)
		if w <= 0 {
			continue
		}
		total += w
		m := factsOf(d.Lookup, p.Name).mask
		for c := 0; c < 5; c++ {
			if m&(1<<c) != 0 {
				weight[c] += w
			}
		}
	}
	if total == 0 {
		return 0
	}
	sum, colours := 0.0, 0
	for c := 0; c < 5; c++ {
		if mask&(1<<c) == 0 {
			continue
		}
		colours++
		sum += weight[c] / total
	}
	if colours == 0 {
		return 0
	}
	return sum / float64(colours)
}

// ratingWeight is a past pick's raw rating, the share weight: the rated
// metric where a file covers the set, the heuristic points elsewhere.
func (d *RatingColour) ratingWeight(set string, ref limited.CardRef) float64 {
	if d.Ratings != nil {
		return d.Ratings.Value(ref.Name)
	}
	if e, ok := RawRating(set, ref.Name); ok {
		return e.GihWr
	}
	return Heuristic(d.Lookup(ref.Name), ref.Rarity)
}

// Scores writes the drafter's score for every pack card, in v.Pack order,
// into dst (grown as needed) and returns it: the centred rating plus k
// times the colour fit, where k ramps linearly from 0 at pack 1 pick 1 to
// its full value by pack 1 pick 10 and holds after. Pick takes the best of
// exactly these scores; behaviour cloning records them as
// the teacher's soft targets.
func (d *RatingColour) Scores(v PickView, dst []float64) []float64 {
	dst = dst[:0]
	t := d.tuning()
	k := t.ColourFitMax
	if v.Round == 1 {
		frac := float64(v.PickNo-1) / float64(t.ColourFitRamp)
		if frac < 1 {
			k *= frac
		}
	}
	for _, ref := range v.Pack {
		dst = append(dst, d.centred(v.Set, ref)+k*d.colourFit(factsOf(d.Lookup, ref.Name).mask, v.Picks, v.Set))
	}
	return dst
}

// Pick scores every card in the pack (Scores) and returns the best one's
// ID. Ties within a small epsilon are broken with r.
func (d *RatingColour) Pick(v PickView, r *rand.Rand) string {
	if len(v.Pack) == 0 {
		return ""
	}
	return v.Pack[BestScore(d.Scores(v, nil), r)].ID
}

// BestScore is RatingColour's choice over scores: the highest, with every
// score within 1e-9 of the running best counted as tied and the tie broken
// uniformly with r (one r.IntN draw, only on a tie). It returns -1 for no
// scores.
func BestScore(scores []float64, r *rand.Rand) int {
	best, bestScore := -1, math.Inf(-1)
	var tied []int
	for i, score := range scores {
		switch {
		case score > bestScore+1e-9:
			best, bestScore, tied = i, score, append(tied[:0], i)
		case score > bestScore-1e-9:
			bestScore = math.Max(bestScore, score)
			tied = append(tied, i)
		}
	}
	if len(tied) > 1 {
		return tied[r.IntN(len(tied))]
	}
	return best
}
