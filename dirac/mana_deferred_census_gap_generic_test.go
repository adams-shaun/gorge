package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// CR 605.1a: the census ability of a restricted-census source (Primal Beyond's
// {C}) is still a legal payment route beside its restricted one, so a generic
// play the restricted batch cannot pay (Orcish Captain's {1} flip, no
// Elemental) stays offered while a Taps watcher (Price of Glory) holds the
// land planner-deferred. The gap's unit is one tap's alternative mask, the
// census answer beside the restricted colours (riderPlainFirst's pattern), so
// the census answer prices the {1} and the restricted colours an Elemental
// play's coloured pips.
func TestDeferredCensusGapStillOffersGenericPlay(t *testing.T) {
	sc, id := deferredWatcherScen(t, "Primal Beyond", "Orcish Captain", "Price of Glory")
	sc.s.cfg |= cfgGorgeAutopay // the lockstep's autopay surface
	oc := sc.perm(id["Orcish Captain"], 0)
	pb := sc.perm(id["Primal Beyond"], 0)
	sc.perm(id["Price of Glory"], 1)
	// Primal Beyond enters tapped unless its controller reveals an Elemental
	// card; the scenario's hand is empty, so untap the land the payment needs.
	sc.s.perms[sc.slot(pb)].flags &^= pfTapped
	// Price of Glory watches a non-active player's land taps: seat 0 is the
	// payer, seat 1 the active player.
	sc.s.active = 1
	sc.s.prio = 0
	sc.s.step = kop.StepMain1
	if !sc.s.planDeferred(sc.k, 0, sc.slot(pb)) {
		t.Fatal("precondition: Primal Beyond is not planner-deferred")
	}
	var d Decision
	sc.s.priorityOptions(sc.k, &d)
	if !hasOption(&d, MakeAction(AkActivate, TCode(oc), 0, 0)) {
		t.Fatalf("Orcish Captain's flip is not offered: %v", d.Options[:d.N])
	}
}
