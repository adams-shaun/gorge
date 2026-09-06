package state

import (
	"math/rand/v2"
)

// KnownOpponentDecklists is the strategic assumption a determinization carries
// about whether the searching seat knows what is in the opponents' decks.
//
// gorge's decks come from repo decklists, so the information is *available* to
// a search that wants it. Whether it may assume it is a genuine strategic
// question: true after game 1 of a match taught the seat the opponents' lists,
// false at the start of game 1 or across unknown opponents.
//
// This round chooses false -- the honest default -- and the choice is defended
// in the D1 report against the case where it is wrong.
//
// Note that this flag is deliberately a NO-OP on Determinize's output: a
// determinization always re-deals the TRUE hidden cards (the pool is the
// opponents' actual hand + library), so the underlying multiset is identical
// whether the seat "knows" the lists or not. What the flag governs is a
// downstream search's priors -- e.g. whether it may lean on known archetype
// shape when pruning or pre-computing plans. It is carried here as the single
// documented place the assumption lives, so C2 does not re-derive it in two
// spots and let the two drift apart.
const KnownOpponentDecklists = false

// Determinize returns a NEW game (built on Clone; the receiver is never
// mutated) in which every fact fromSeat cannot legally see has been
// re-randomised and every fact it can see is byte-identical -- the honest
// information set a search agent (C2) is allowed to plan with. Ruling C0: a
// search agent searches determinizations, never the true state.
//
// What stays put:
//
//   - fromSeat's own hand is untouched, element for element.
//   - every public fact -- battlefield, graveyard, exile, the stack, and every
//     scalar field on Player and Game -- is byte-identical.
//   - the Objs arena is preserved: Objs[i] still has ID i+1, and no ObjID is
//     invented, dropped or duplicated.
//
// What moves:
//
//   - fromSeat's own library is order-randomised, composition preserved. A
//     player knows what is in their deck; they do not know its order. This is
//     intentionally NOT skipped because the searching seat "owns" those cards.
//   - every other seat's hidden zones (library + hand) are pooled per owner and
//     re-dealt to the same per-zone counts. An opponent's hand size is public;
//     its contents are not. Cards never change owner.
//
// Known unsoundness: gorge records a reveal (and any card known from play) as
// a one-shot Note event in the event log, never as persistent per-object
// state, and a determinization has no access to the log, so it cannot tell
// which opponent cards became known during play. A card an opponent revealed,
// or drew from a public zone, is re-randomised like any other unknown card. The
// concrete case it gets wrong is documented in the D1 report; tracking
// revealed-ness well enough to exempt such cards is deliberately deferred --
// no mechanism is invented this round.
//
// Face-down permanents (morph/manifest) do not exist in gorge's corpus
// (state/object.go has no such flag and cards/keywords.go registers none), so
// there is no hidden-identity public object to handle.
//
// rng is consumed, never constructed here. Same seed, same determinization,
// every time -- the engine is event-sourced, hash-chained and replayable, and
// a search that cannot be replayed cannot be debugged.
func (g *Game) Determinize(fromSeat PlayerID, rng *rand.Rand) *Game {
	c := g.Clone()

	// fromSeat's own library: order-randomised, composition preserved.
	shuffleZone(c, ZLibrary, fromSeat, rng)

	// Every other seat: pool hidden zones and re-deal to the same counts.
	for p := PlayerID(0); p < PlayerID(len(c.Players)); p++ {
		if p != fromSeat {
			redistributeHidden(c, p, rng)
		}
	}
	return c
}

// shuffleZone randomises the order of one zone's ids, preserving its multiset.
// For fromSeat's own library this expresses "a player knows their deck, not
// its order".
func shuffleZone(g *Game, z Zone, p PlayerID, rng *rand.Rand) {
	ids := g.Zone(z, p)
	if len(ids) < 2 {
		return
	}
	rng.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
}

// redistributeHidden pools the ids in seat p's hidden zones (library + hand),
// shuffles them, and re-deals them to the same per-zone counts it started
// with. Cards never cross owners: the pool is p's alone. Each object's own
// Zone field is kept consistent with its new zone membership, because
// events.Move reads o.Zone as the authoritative source zone and the zones
// arrays must agree with it. Both ends of the re-deal are explicitly set (not
// appended) so no stale ids leak from a prior shuffle.
func redistributeHidden(g *Game, p PlayerID, rng *rand.Rand) {
	lib := g.Zone(ZLibrary, p)
	hand := g.Zone(ZHand, p)
	nLib, nHand := len(lib), len(hand)
	total := nLib + nHand
	if total < 2 {
		return
	}
	order := make([]ObjID, 0, total)
	order = append(order, lib...)
	order = append(order, hand...)
	rng.Shuffle(total, func(i, j int) { order[i], order[j] = order[j], order[i] })

	g.SetZone(ZLibrary, p, order[:nLib])
	g.SetZone(ZHand, p, order[nLib:])
	for _, id := range order[:nLib] {
		if o := g.Obj(id); o != nil {
			o.Zone = ZLibrary
		}
	}
	for _, id := range order[nLib:] {
		if o := g.Obj(id); o != nil {
			o.Zone = ZHand
		}
	}
}
