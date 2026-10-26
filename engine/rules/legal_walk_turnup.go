package rules

import "github.com/adams-shaun/gorge/engine/state"

// turnFaceUpOffers appends the turn-face-up special-action offers one
// face-down battlefield permanent carries: the Morph-family cast procedure,
// the manifested/cloaked creature card's mana-cost procedure (CR 701.40b,
// 701.58b), and the Morph/Megamorph/Disguise alternative a manifested or
// cloaked card may use instead (CR 701.40c-d, 701.58c-d). Each offer is gated
// on the same payability predicate the action re-reads, so a stale offer is
// never charged.
//
// It lives in the legal*.go walk family (not beside the turn-up machinery in
// morph_turnup.go) because the priority-offer vocabulary ratchet
// (TestPotentialActionsProjectsEveryPlayKind) parses exactly those files to
// find every Kind the walk emits; a move out of the family would hide the
// "turn_face_up" literal from it.
func (w *legalWalk) turnFaceUpOffers(p state.PlayerID, id state.ObjID) {
	e := w.e
	o := e.G.Obj(id)
	if o == nil || !o.FaceDown {
		return
	}
	offers := make([]morphFaceUp, 0, 3)
	if mf, ok := morphFaceUpCost(o); ok {
		offers = append(offers, mf)
	} else if o.Manifested || o.Cloaked {
		if mf, ok := manifestFaceUpCost(o, e.faceCost); ok {
			offers = append(offers, mf)
		}
		offers = append(offers, manifestKeywordFaceUpCosts(o)...)
	}
	if len(offers) == 0 {
		return
	}
	if e.turnFaceUpCantHappen(id) {
		// CR 614.1a: a live CantHappen turn-up replacement (Karlov
		// Watchdog's "permanents your opponents control can't be turned
		// face up during your turn") makes the special action illegal, so
		// the option is never offered -- and never paid for.
		return
	}
	for _, mf := range offers {
		// morphTurnUpPayablePriced: hyp nil is the floating-pool gate the
		// action itself re-reads; the potential walk prices the same cost
		// against its hypothetical bound, so a float-gated turn-up is a
		// potential play like every other mana-costed offer.
		mods, ok := e.morphTurnUpMods(p, id, mf)
		if !ok || !e.morphTurnUpPayablePriced(p, id, mf.special(), mf.cost, mods, w.pricing()) {
			continue
		}
		w.add("turn_face_up", "Turn face up ("+costPhrase(mf.cost)+")", id)
		if mode := mf.mode(); mode != "" {
			w.out[len(w.out)-1].Mode = mode
		}
	}
}
