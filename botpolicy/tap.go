package botpolicy

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// T1 -- the need-aware tap gate (task op6). The tap-for-mana "activate"
// options legalActions (rules/legal.go) offers are gated on "do any of the
// cards this seat could cast need mana the pool does not yet have". The old
// position-first block tapped the first "activate" in every main phase
// until every source was spent, floating mana in bulk and letting each
// step's end (CR 500.4) empty whatever a cast never spent; this file
// replaces the WHEN with a need check computed from board facts alone (the
// policy is stateless: no memory across decisions).
//
// The WHICH was position-first when op6 landed, because the View then
// projected no per-permanent mana production and nothing in the offer
// distinguished a Plains from an Island. It no longer is: CardView.Produces
// (dp2) carries each permanent's produced colours, so chooseTap below picks
// the source whose colour answers a pip the pool still owes. See chooseTap's
// own doc comment for the ordering rules; what survives from op6 here is the
// WHEN, not the WHICH.
//
// The check is deliberately the engine's own gate re-derived at the
// granularity botpolicy may read: a card is "castable with the pool" when
// the pool's total covers its converted cost and its coloured pips cover
// every coloured symbol in its printed cost -- the CanPay shape
// rules/cast.go's castable runs, re-expressed over the Board's Card facts
// because botpolicy cannot import rules (Ruling F7). The engine only ever
// offers a "cast" option once the CURRENT pool can pay the cost, so
// "nothing castable yet + tapping could help" is exactly "some castable
// card the pool cannot pay" -- the triggers of the tap loop, and the
// moment the pool pays the cheapest such card the cast options open and
// chooseCast takes over.

// hasFlashback is the tap gate's graveyard-castability test: does the
// derived keyword list (the engine's Derived() output, which the View also
// projects as CardView.Keywords) carry the Flashback keyword? It is the
// policy's mirror of rules/legal.go's own flashback-gate read (e.HasKeyword
// over the same list), with the same head-strip cards.KeywordHead applies
// so a parameterised keyword can never shadow the head word.
func hasFlashback(kws []string) bool {
	for _, k := range kws {
		if strings.EqualFold(cards.KeywordHead(k), "Flashback") {
			return true
		}
	}
	return false
}

// tapWants reports whether any card the deciding seat could cast from a
// castable zone (Card.Castable: hand, command zone, graveyard with
// Flashback) has a cost the current pool cannot pay:
//
//   - cost is the printed converted mana cost, plus the CR 903.8 command
//     tax ({2} per prior command-zone cast) when the card is its owner's
//     commander sitting in the command zone -- the same tax chooseCast's
//     CR1 prices, so a tapped-out commander cast is not gated early;
//   - a card with CMC 0 (a land, a free spell) never wants a tap: it is
//     castable without mana or played through the land drop;
//   - the pool covers the card when its total is at least the cost AND
//     each coloured pip of the printed cost has a matching pool slot (the
//     colour-blindness of CmcOf stops here: a {U}{B} card against a
//     {U}{U} pool still wants a tap, because the pool cannot pay it).
//
// A card the seat cannot currently cast for ANY amount of mana for a
// non-mana reason (no legal targets, a timing restriction) still triggers
// taps: the policy cannot read those facts, and its old behaviour did
// worse (it tapped regardless of the hand at all).
func (b Board) tapWants() bool {
	for id, c := range b.Cards {
		if !c.Castable || c.CMC <= 0 {
			continue
		}
		if !b.poolPays(id, c) {
			return true
		}
	}
	return false
}

// poolPays prices one castable card against the current pool: can the pool
// pay its cost (CMC plus the command-zone tax, CR 903.8) with its coloured
// pips covered? This is the policy's read-only approximation of the
// engine's CanPay, at the CmcOf granularity the Board carries.
func (b Board) poolPays(id state.ObjID, c Card) bool {
	cost := c.CMC
	if cmdr, ok := b.Commanders[id]; ok && cmdr.InCommandZone {
		cost += 2 * cmdr.Casts
	}
	if b.Pool.Total() < cost {
		return false
	}
	// The coloured-pip demands: a single-letter W/U/B/R/G symbol is one
	// pip of that colour; every other symbol (numbers, {X}, hybrids,
	// Phyrexian, colourless) is generic, which CmcOf already counted in
	// CMC and any mana pays. Pips are counted per colour first, then
	// compared with the pool once, so a {U}{U} card against a single
	// floating blue still wants a tap.
	pips := colourPips(c.ManaCost)
	for si, n := range pips {
		if n > b.Pool[si] {
			return false
		}
	}
	return true
}

// colourPips counts the coloured (WUBRG) pips of a printed Forge mana cost
// into the five colour slots. A single-letter W/U/B/R/G symbol is one pip of
// that colour; every other symbol (numbers, {X}, hybrids, Phyrexian,
// colourless) is generic -- CmcOf already counted it in CMC and any mana
// pays it. This is the shared pip count behind both poolPays and the
// colour-aware tap gate's need map, so the two always agree.
func colourPips(mc string) [5]int32 {
	var pips [5]int32
	for _, sym := range strings.Fields(braceForm.Replace(mc)) {
		if len(sym) != 1 {
			continue
		}
		si := state.ManaIndex(sym[0]) // MW..MG for WUBRG, MC for anything else
		if si > state.MG {
			continue // not one of the five colours
		}
		pips[si]++
	}
	return pips
}

// cardScore ranks one card the way chooseCast ranks an offered cast (cast.go's
// castScore), so the tap gate can pick the same "intended" card the casting
// policy would most want to enable: a creature scores 30 + 4*Power, a
// non-creature its mana value. The tap gate reads it to choose WHICH card's
// colour need to tap toward, not which card to cast -- the cast itself stays
// with chooseCast.
func (b Board) cardScore(c Card) int32 {
	if c.Creature {
		return 30 + c.Power*4
	}
	return c.CMC
}

// bestUnpayable returns the castable card the pool cannot currently pay that
// the casting policy would most want to enable (highest cardScore). It is the
// tap gate's "intended spell": the card that makes tapWants true, whose
// colour need chooseTap targets. It consumes no rng and breaks ties on object
// id, so map iteration order cannot reach the pick.
func (b Board) bestUnpayable() (state.ObjID, Card, bool) {
	bestID := state.ObjID(0)
	var best Card
	var bestScore int32 = -1
	found := false
	for id, c := range b.Cards {
		if !c.Castable || c.CMC <= 0 {
			continue
		}
		if b.poolPays(id, c) {
			continue
		}
		s := b.cardScore(c)
		if !found || s > bestScore || (s == bestScore && int(id) < int(bestID)) {
			bestID, best, bestScore, found = id, c, s, true
		}
	}
	return bestID, best, found
}

// neededColours is the set of coloured pips the pool cannot yet cover for one
// card: index i is true when the card's cost carries more pips of colour i
// than the pool holds. This is exactly the colour the tap gate should aim a
// tap at -- a source that yields an already-covered colour adds nothing toward
// making this card castable.
func (b Board) neededColours(c Card) [5]bool {
	pips := colourPips(c.ManaCost)
	var need [5]bool
	for i := 0; i < 5; i++ {
		need[i] = pips[i] > b.Pool[i]
	}
	return need
}

// chooseTap is the KPriority tap gate. It returns one "activate" option when
// the pool cannot yet pay any castable card, or -1 when tapping has nothing to
// enable -- in which case the caller falls through to the land drop and the
// cast.
//
// T2 (Task dp2): the WHICH source is colour-aware instead of position-first.
// chooseTap names the intended card (the best-scoring castable card the pool
// cannot pay) and taps toward its colour need, ordered by a three-tier
// preference that the production honesty fix (bl1) tightened:
//
//   - tier 0: a source that DEMONSTRABLY produces a colour the intended card
//     needs (producesColour, which is only true for a known, guaranteed
//     colour slot -- an indeterminate source contributes nothing to claim
//     it). This is the only tier a tap is aimed at a specific pip.
//   - tier 1: a source that demonstrably produces some mana, but none of a
//     needed colour (a screw in colour, or a pure generic shortfall): it at
//     least adds to the pool, so it outranks a source that may produce
//     nothing at all.
//   - tier 2: a source that demonstrably produces nothing -- no known colour
//     slot. An Indeterminate-amount source lands here because it contributes
//     contributes zero to every colour slot (detected via Colour, never by reading the
//     Indeterminate flag), so such a source is a last resort only: it might
//     produce nothing, so chooseTap never PREFERS it over a source that
//     demonstrably produces.
//   - within a tier, the source with the FEWEST distinct colours is tapped
//     first (Task dp2's least-flexible-first: the mono-colour source before
//     the dual, keeping the flexible source for the colour it is still
//     needed for); a tie breaks on option index.
//
// It consumes no rng: the pick is a pure function of the offered options and
// the board facts.
func (b Board) chooseTap(d *decision.Decision) int {
	if !b.tapWants() {
		return -1
	}
	_, c, ok := b.bestUnpayable()
	if !ok {
		return -1
	}
	need := b.neededColours(c)
	best := -1
	bestTier := 3
	bestFlex := 0
	for _, o := range d.Options {
		if o.Kind != "activate" {
			continue
		}
		prod := b.Cards[o.Obj].Produces
		matches := false
		for i := 0; i < 5; i++ {
			if need[i] && prod.ProducesColour(i) {
				matches = true
				break
			}
		}
		// demonstrable: a colour slot > 0 is production the pool will
		// actually receive (an indeterminate amount contributes 0 to every
		// slot, so it cannot make this true).
		demonstrable := prod.Colour[0] > 0 || prod.Colour[1] > 0 || prod.Colour[2] > 0 ||
			prod.Colour[3] > 0 || prod.Colour[4] > 0 || prod.Colour[5] > 0
		tier := 2
		if matches {
			tier = 0
		} else if demonstrable {
			tier = 1
		}
		flex := prod.DistinctColours()
		if best == -1 || tier < bestTier ||
			(tier == bestTier && flex < bestFlex) ||
			(tier == bestTier && flex == bestFlex && o.Index < best) {
			best, bestTier, bestFlex = o.Index, tier, flex
		}
	}
	return best
}
