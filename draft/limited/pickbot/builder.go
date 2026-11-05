package pickbot

import (
	"sort"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/engine/cards"
)

// colourPairs is the ten colour pairs in fixed order; a tie for the best
// pair stays with the earlier one, so the list order is the tie-break.
var colourPairs = [10][2]int{
	{0, 1}, {0, 2}, {0, 3}, {0, 4},
	{1, 2}, {1, 3}, {1, 4},
	{2, 3}, {2, 4},
	{3, 4},
}

// candidate is one pool card's rating and facts for the builder.
type candidate struct {
	id       string
	name     string
	rating   float64
	mask     uint8
	pips     [5]int
	mv       int
	creature bool
	class    cardClass
}

// Best23 is the default builder: it picks the colour pair whose best
// 23 playable cards rate highest, plays those 23 preferring MinCreatures
// creatures and capping mana value 6+ at 3, and fills the rest with basics
// through
// build.AutoLands. A pool too thin to field 23 in-pair playables is filled
// with the best remaining cards of any colour. The caller owns the deck's
// name and is the one who can say so in it (build.Deck carries no name
// field).
type Best23 struct {
	// Lookup resolves card names to card facts (colour identity, mana
	// pips, type line, mana value). The FDN rated path needs it only to
	// tell lands and creatures apart.
	Lookup CardLookup
	Tuning Tuning
	// Ratings, when non-nil, replaces the embedded rating file (see
	// RatingColour.Ratings).
	Ratings *RatingsOverride
}

// tuning returns the builder's knob set, filling the zero value with the
// spec defaults.
func (b *Best23) tuning() Tuning {
	if b.Tuning == (Tuning{}) {
		return DefaultTuning
	}
	return b.Tuning
}

// card resolves a name through the builder's Lookup.
func (b *Best23) card(name string) *cards.Card {
	if b.Lookup == nil {
		return nil
	}
	return b.Lookup(name)
}

// candidates rates and sorts every pool card that is not a basic land and,
// when the builder has a Lookup, that the corpus resolves:
// rating descending, name then ID ascending for a deterministic tie-break.
func (b *Best23) candidates(set string, pool []limited.CardRef) []candidate {
	t := b.tuning()
	out := make([]candidate, 0, len(pool))
	for _, ref := range pool {
		if b.Lookup != nil && b.Lookup(ref.Name) == nil {
			// The card corpus has no script for it (a set drafted before
			// its cards are scripted): the engine cannot play it, so the
			// deck does not include it.
			continue
		}
		facts := factsOf(b.Lookup, ref.Name)
		c := candidate{
			id:   ref.ID,
			name: ref.Name,
			mask: facts.mask,
			pips: facts.pips,
		}
		if card := b.card(ref.Name); card != nil && len(card.Faces) > 0 {
			f := card.Faces[0]
			if f.IsLand() {
				continue
			}
			c.creature = f.IsCreature()
			c.mv = int(f.Cmc())
			c.class = classOf(f)
		}
		if b.Ratings != nil {
			c.rating = (b.Ratings.Value(ref.Name) - t.RatingCentre) * t.RatingScale
		} else if e, ok := RawRating(set, ref.Name); ok {
			c.rating = (e.GihWr - t.RatingCentre) * t.RatingScale
		} else {
			c.rating = (Heuristic(b.card(ref.Name), ref.Rarity) - t.HeuristicCentre) * t.HeuristicScale
		}
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].rating != out[j].rating {
			return out[i].rating > out[j].rating
		}
		if out[i].name != out[j].name {
			return out[i].name < out[j].name
		}
		return out[i].id < out[j].id
	})
	return out
}

// Build turns a finished pool into a deck.
func (b *Best23) Build(set string, pool []limited.CardRef) build.Deck {
	t := b.tuning()
	all := b.candidates(set, pool)

	// 1. The colour pair whose best MainSize playable cards rate highest,
	// scored on its in-pair cards alone: a pair short of
	// MainSize playables pays ShortPairPenalty per unfilled slot instead of
	// being credited with the off-colour cards the build would pad it with,
	// so a thin pair padded with other colours' best cards cannot outscore
	// a deep real pair. The build itself still pads the chosen pair with the
	// best remaining cards of any colour; that off-colour fill is never
	// scored.
	var best, bestAll []candidate
	bestSum := 0.0
	for _, pair := range colourPairs {
		allowed := uint8(1<<pair[0] | 1<<pair[1])
		var inPair []candidate
		for _, c := range all {
			if c.mask&^allowed == 0 {
				inPair = append(inPair, c)
			}
		}
		main23 := make([]candidate, 0, t.MainSize)
		sum := 0.0
		taken := make(map[string]bool, t.MainSize)
		for _, c := range inPair {
			if len(main23) >= t.MainSize {
				break
			}
			main23 = append(main23, c)
			taken[c.id] = true
			sum += c.rating
		}
		sum += t.ShortPairPenalty * float64(t.MainSize-len(main23))
		for _, c := range all {
			if len(main23) >= t.MainSize {
				break
			}
			if taken[c.id] {
				continue
			}
			main23 = append(main23, c)
			taken[c.id] = true
		}
		if sum > bestSum || best == nil {
			bestSum, best, bestAll = sum, main23, inPair
		}
	}

	// 2. Play the best 23, capping high-mana-value cards and then
	// preferring creatures up to MinCreatures.
	main := make([]candidate, 0, t.MainSize)
	taken := make(map[string]bool, t.MainSize)
	var k caps
	for _, c := range best {
		if len(main) >= t.MainSize {
			break
		}
		if !k.fits(t, c) {
			continue
		}
		k.add(t, c, 1)
		main = append(main, c)
		taken[c.id] = true
	}
	main = b.preferCreatures(main, bestAll, taken, &k)
	main = b.fillToMainSize(main, bestAll, all, taken, &k)
	main = b.curveFloors(main, bestAll, taken, &k)

	deck := build.Deck{Basics: [5]int{}}
	var pips [5]int
	for _, c := range main {
		deck.Main = append(deck.Main, c.id)
		for i, p := range c.pips {
			pips[i] += p
		}
	}
	deck.Basics = build.AutoLands(deck.Main, func(string) [5]int { return pips })
	return deck
}

// preferCreatures swaps the lowest-rated non-creatures out for the
// best-rated untaken creatures until the deck carries at least
// MinCreatures of them, respecting the high-mana-value cap. The
// preference is unconditional: a creature comes in even when the
// non-creature it replaces rates higher.
func (b *Best23) preferCreatures(main, inPair []candidate, taken map[string]bool, k *caps) []candidate {
	t := b.tuning()
	for countCreatures(main) < t.MinCreatures {
		swapIn, found := bestUntaken(inPair, taken, t, k, isCreature)
		if !found {
			return main
		}
		swapOut := -1
		for i := len(main) - 1; i >= 0; i-- {
			if !main[i].creature {
				swapOut = i
				break
			}
		}
		if swapOut < 0 {
			return main
		}
		out := main[swapOut]
		k.add(t, out, -1)
		k.add(t, swapIn, 1)
		delete(taken, out.id)
		taken[swapIn.id] = true
		main[swapOut] = swapIn
	}
	return main
}

// fillToMainSize tops a short main up with the best remaining cards, the
// in-pair ones first and then the off-colour rest, for a
// thin pool.
func (b *Best23) fillToMainSize(main, inPair, all []candidate, taken map[string]bool, k *caps) []candidate {
	t := b.tuning()
	for len(main) < t.MainSize {
		next, found := bestUntaken(inPair, taken, t, k, nil)
		if !found {
			next, found = bestUntaken(all, taken, t, k, nil)
		}
		if !found {
			return main
		}
		k.add(t, next, 1)
		taken[next.id] = true
		main = append(main, next)
	}
	return main
}

// curveFloors raises each curve bucket short of Tuning.CurveMin: it swaps
// the lowest-rated main card from a bucket above its own floor for the
// best untaken in-pair card of the short bucket, keeping the caps and
// MinCreatures. A zero CurveMin (the default) changes nothing.
func (b *Best23) curveFloors(main, inPair []candidate, taken map[string]bool, k *caps) []candidate {
	t := b.tuning()
	for bucket := range t.CurveMin {
		for k.buckets[bucket] < t.CurveMin[bucket] {
			out := -1
			for i, m := range main {
				mb := curveBucket(m.mv)
				if mb == bucket || k.buckets[mb] <= t.CurveMin[mb] {
					continue
				}
				if out < 0 || m.rating <= main[out].rating {
					out = i
				}
			}
			if out < 0 {
				return main
			}
			o := main[out]
			k.add(t, o, -1)
			in, found := bestUntaken(inPair, taken, t, k, func(c candidate) bool {
				return curveBucket(c.mv) == bucket && (c.creature || !o.creature || k.creatures >= t.MinCreatures)
			})
			if !found {
				k.add(t, o, 1)
				return main
			}
			k.add(t, in, 1)
			delete(taken, o.id)
			taken[in.id] = true
			main[out] = in
		}
	}
	return main
}

// caps counts what the main holds against the builder's caps: the
// high-mana-value cap, Tuning.CreatureMax and Tuning.CurveMax.
type caps struct {
	high, creatures int
	buckets         [6]int
}

// fits reports whether c can join the main without breaking a cap.
func (k *caps) fits(t Tuning, c candidate) bool {
	if c.mv >= t.HighValueMV && k.high >= t.MaxHighValue {
		return false
	}
	if c.creature && t.CreatureMax > 0 && k.creatures >= t.CreatureMax {
		return false
	}
	b := curveBucket(c.mv)
	return t.CurveMax[b] == 0 || k.buckets[b] < t.CurveMax[b]
}

// add counts c into (sign 1) or out of (sign -1) the main.
func (k *caps) add(t Tuning, c candidate, sign int) {
	if c.mv >= t.HighValueMV {
		k.high += sign
	}
	if c.creature {
		k.creatures += sign
	}
	k.buckets[curveBucket(c.mv)] += sign
}

func isCreature(c candidate) bool { return c.creature }

// bestUntaken finds the highest-rated untaken candidate keep accepts (nil
// accepts any) that fits the caps.
func bestUntaken(all []candidate, taken map[string]bool, t Tuning, k *caps, keep func(candidate) bool) (candidate, bool) {
	for _, c := range all {
		if taken[c.id] || keep != nil && !keep(c) || !k.fits(t, c) {
			continue
		}
		return c, true
	}
	return candidate{}, false
}

// countCreatures counts the main's creatures.
func countCreatures(main []candidate) int {
	n := 0
	for _, c := range main {
		if c.creature {
			n++
		}
	}
	return n
}
