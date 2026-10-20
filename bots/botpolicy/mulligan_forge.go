package botpolicy

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/decision"
)

// MulliganForge is the opt-in rule reproducing Forge's London mulligan
// (forge-game's LondonMulligan/MulliganService and the AI controller's
// scoreHand/wantMulligan/tuckCardsViaMulligan), clean-room in gorge's own
// vocabulary. It is NOT an alias of MulliganConstructed: the keep rule reads
// the deck's land ratio and has no colour or early-play test, and the
// bottoming rule bottoms non-producing lands first, then the basic with the
// most copies, then the worst creature or cheapest card.
//
// Sequencing. Forge draws seven, bottoms k cards, and only then decides
// keep/mulligan on the reduced hand (h = 7-k, passing k again). gorge asks
// keep/mulligan with seven in hand and the penalty in the prompt, so this
// rule simulates Forge's deterministic bottoming on the seven and judges the
// remainder; the later bottoming ask re-runs the same choice.
//
// Keep rule (threshold T = 4, Forge's Default.ai profile):
//   - h - k < T keeps; with h = 7-k that is 7-2k < 4, so two or more
//     mulligans always keep.
//   - If the library holds no lands, keep.
//   - L = reduced-hand lands, ratio = librarySize/libraryLands (integer).
//     L < 2 mulligans unless ratio > 6 (spell-heavy deck); L == h mulligans
//     unless ratio < 2 (land-heavy deck); h >= 7 and L >= h-1 mulligans
//     unless ratio < 2. Otherwise keep. No colour or curve check.
//
// Bottom rule (D = (n-k)/2, P = mana-producing lands in the whole seven):
// repeat k times:
//   - (a) if lands still in hand minus lands already bottomed exceeds D,
//     bottom the worst land. The double subtraction is a Forge quirk,
//     reproduced on purpose.
//   - (b) otherwise, if a nonland still in hand has P == 0 or CMC > P,
//     bottom the highest-CMC such card, ties first in hand order.
//   - (c) otherwise bottom the worst card: the worst land if only lands are
//     left; else the lowest-power, then lowest-CMC creature; else the
//     cheapest card, ties to the last in hand order.
//
// Worst land: among non-producing lands when any, else producing lands;
// +1 for a basic, -4 for a creature, highest score wins; on a tie a later
// land replaces the pick only when its printed name has strictly more copies
// in the pool.
//
// Deviations, deliberate: Forge counts {0} artifacts as lands for the keep
// rule (Card has no artifact flag); Forge's artifact/enchantment branch of
// "worst card", Living End, the NeedsToPlay SVar and the Reckless threshold
// (3) are not modelled. The creature comparison uses printed Power/CMC as an
// INFERRED approximation of Forge's evaluateCreature.
const MulliganForge MulliganRule = MulliganConstructed + 1

// forgeMulliganThreshold is Forge's Default.ai MULLIGAN_THRESHOLD.
const forgeMulliganThreshold = 4

type forgeHandCard struct {
	idx  int
	c    Card
	land bool
}

// forgeProducesMana reports whether the card's recorded mana production is
// non-empty. Unlike producesColour it does NOT treat "no recorded production"
// as every colour: Evolving Wilds is a non-producing land to Forge's bottom
// rule.
func forgeProducesMana(c Card) bool {
	if c.Basic || c.Produces.Any || c.Produces.Indeterminate {
		return true
	}
	for _, v := range c.Produces.Colour {
		if v > 0 {
			return true
		}
	}
	return false
}

func forgeNameCopies(pool []forgeHandCard, name string) int {
	n := 0
	for _, h := range pool {
		if h.c.PrintedName == name {
			n++
		}
	}
	return n
}

// forgeWorstLand returns the position in pool of the worst land by Forge's
// getWorstLand order. pool is expected to hold only lands.
func forgeWorstLand(pool []forgeHandCard) int {
	if len(pool) == 0 {
		return -1
	}
	var candidates []int
	for i, h := range pool {
		if !forgeProducesMana(h.c) {
			candidates = append(candidates, i)
		}
	}
	if len(candidates) == 0 {
		for i := range pool {
			candidates = append(candidates, i)
		}
	}
	score := func(c Card) int {
		s := 0
		if c.Basic {
			s++
		}
		if c.Creature {
			s -= 4
		}
		return s
	}
	best := candidates[0]
	bestScore := score(pool[best].c)
	for _, i := range candidates[1:] {
		s := score(pool[i].c)
		if s > bestScore {
			best, bestScore = i, s
			continue
		}
		if s == bestScore &&
			forgeNameCopies(pool, pool[i].c.PrintedName) > forgeNameCopies(pool, pool[best].c.PrintedName) {
			best = i
		}
	}
	return best
}

// forgeWorstCard returns the position in rem of Forge's "worst card" among
// the cards still in hand (rem may mix lands and nonlands).
func forgeWorstCard(rem []forgeHandCard) int {
	if len(rem) == 0 {
		return -1
	}
	allLands := true
	for _, h := range rem {
		if !h.land {
			allLands = false
			break
		}
	}
	if allLands {
		return forgeWorstLand(rem)
	}
	best := -1
	for i, h := range rem {
		if !h.c.Creature {
			continue
		}
		if best < 0 || h.c.Power < rem[best].c.Power ||
			(h.c.Power == rem[best].c.Power && h.c.CMC < rem[best].c.CMC) {
			best = i
		}
	}
	if best >= 0 {
		return best
	}
	best = 0
	for i := 1; i < len(rem); i++ {
		if rem[i].c.CMC <= rem[best].c.CMC {
			best = i
		}
	}
	return best
}

// forgeTuck returns the idx values to bottom, in pick order, per Forge's
// tuckCardsViaMulligan loop. It stops early when the hand runs out.
func forgeTuck(hand []forgeHandCard, k int) []int {
	if k <= 0 || len(hand) == 0 {
		return nil
	}
	if k > len(hand) {
		k = len(hand)
	}
	n := len(hand)
	d := (n - k) / 2
	producingLands := 0
	for _, h := range hand {
		if h.land && forgeProducesMana(h.c) {
			producingLands++
		}
	}
	rem := slices.Clone(hand)
	bottomedLands := 0
	out := make([]int, 0, k)
	for len(out) < k && len(rem) > 0 {
		lands := 0
		for _, h := range rem {
			if h.land {
				lands++
			}
		}
		if lands-bottomedLands > d {
			pool := make([]forgeHandCard, 0, lands)
			for _, h := range rem {
				if h.land {
					pool = append(pool, h)
				}
			}
			pick := pool[forgeWorstLand(pool)]
			out = append(out, pick.idx)
			for i, h := range rem {
				if h.idx == pick.idx {
					rem = slices.Delete(rem, i, i+1)
					break
				}
			}
			bottomedLands++
			continue
		}
		uncastable := -1
		for i, h := range rem {
			if h.land {
				continue
			}
			if producingLands == 0 || int(h.c.CMC) > producingLands {
				if uncastable < 0 || h.c.CMC > rem[uncastable].c.CMC {
					uncastable = i
				}
			}
		}
		if uncastable >= 0 {
			pick := rem[uncastable]
			out = append(out, pick.idx)
			rem = slices.Delete(rem, uncastable, uncastable+1)
			continue
		}
		worst := forgeWorstCard(rem)
		pick := rem[worst]
		out = append(out, pick.idx)
		rem = slices.Delete(rem, worst, worst+1)
		if pick.land {
			bottomedLands++
		}
	}
	return out
}

// forgeHand is the deciding seat's hand census, in ascending ObjID order (the
// IDTable's insertion order is the adapter's zone walk, not a hand order).
// ok is false when the census disagrees with HandSize, and the caller keeps.
func (b Board) forgeHand() ([]forgeHandCard, bool) {
	ids := slices.Clone(b.Cards.Keys())
	slices.Sort(ids)
	hand := make([]forgeHandCard, 0, len(ids))
	for _, id := range ids {
		c, known := b.Cards.Lookup(id)
		if !known || !c.Castable || c.OnBattlefield {
			continue
		}
		hand = append(hand, forgeHandCard{idx: len(hand), c: c, land: known && isLandCard(c)})
	}
	return hand, len(hand) > 0 && int32(len(hand)) == b.HandSize
}

// keepForge is MulliganForge's keep/mulligan rule for a hand that will
// bottom `bottom` cards if kept.
func (b Board) keepForge(bottom int) bool {
	hand, ok := b.forgeHand()
	if !ok {
		return true
	}
	n := len(hand)
	h := n - bottom
	if h-bottom < forgeMulliganThreshold {
		return true
	}
	tucked := forgeTuck(hand, bottom)
	kept := make([]forgeHandCard, 0, h)
	for _, hc := range hand {
		if !slices.Contains(tucked, hc.idx) {
			kept = append(kept, hc)
		}
	}
	lands := 0
	for _, hc := range kept {
		if hc.land {
			lands++
		}
	}
	ratio := 3 // neutral without a manifest: neither deck override fires
	if b.OwnDeck != nil {
		total, deckLands := 0, 0
		for _, r := range b.OwnDeck.Main {
			total += r.Count
			if r.Land {
				deckLands += r.Count
			}
		}
		if total > 0 {
			librarySize := total - h
			libraryLands := deckLands - lands
			if libraryLands <= 0 {
				return true
			}
			ratio = librarySize / libraryLands
		}
	}
	switch {
	case lands < 2:
		return ratio > 6
	case lands == h:
		return ratio < 2
	case h >= 7 && lands >= h-1:
		return ratio < 2
	default:
		return true
	}
}

// forgeBottoming is MulliganForge's bottoming answer. It returns the option
// indices to bottom, ascending.
func (b Board) forgeBottoming(d *decision.Decision) []int {
	m := d.Max
	if m < 0 {
		m = 0
	}
	if m > len(d.Options) {
		m = len(d.Options)
	}
	if m <= 0 {
		return nil
	}
	hand := make([]forgeHandCard, 0, len(d.Options))
	for _, o := range d.Options {
		c, known := b.Cards.Lookup(o.Obj)
		hand = append(hand, forgeHandCard{idx: o.Index, c: c, land: known && isLandCard(c)})
	}
	out := forgeTuck(hand, m)
	slices.Sort(out)
	return out
}
