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
// policy is stateless: no memory across decisions), and keeps the WHICH
// source position-first -- the View projects no per-permanent mana
// production, so nothing in the offer distinguishes the lands' colours
// (the conventional stand-in is recorded in AGENTS.md's Known
// approximations).
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
	var pips [5]int32
	for _, sym := range strings.Fields(braceForm.Replace(c.ManaCost)) {
		if len(sym) != 1 {
			continue
		}
		si := state.ManaIndex(sym[0]) // MW..MG for WUBRG, MC for anything else
		if si > state.MG {
			continue // not one of the five colours
		}
		pips[si]++
	}
	for si, n := range pips {
		if n > b.Pool[si] {
			return false
		}
	}
	return true
}

// chooseTap is the KPriority tap gate: it returns one "activate" option
// (the first offered, position-first within the group) when the pool
// cannot yet pay any castable card, or -1 when tapping has nothing to
// enable -- in which case the caller falls through to the land drop and
// the cast. It consumes no rng: the pick is a pure function of the offered
// options and the board facts.
func (b Board) chooseTap(d *decision.Decision) int {
	if !b.tapWants() {
		return -1
	}
	for _, o := range d.Options {
		if o.Kind == "activate" {
			return o.Index
		}
	}
	return -1
}
