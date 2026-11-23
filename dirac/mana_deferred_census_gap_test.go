package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// CR 605.1a with CR 106.6: Primal Beyond's {C} census ability sits beside its
// any-colour ability restricted to Elemental spells and abilities. When a
// Taps watcher (Price of Glory, Activator$ Player.NonActive) makes the land
// planner-deferred, Flamekin Brawler's {R} pump is funded only by the
// restricted ability. The prefix route taps the land before the play, so the
// activation is offered and the watcher's trigger sits below it.
func TestDeferredCensusRestrictedGapOffersActivation(t *testing.T) {
	sc, id := deferredWatcherScen(t, "Primal Beyond", "Flamekin Brawler", "Price of Glory")
	sc.s.cfg |= cfgGorgeAutopay // the lockstep's autopay surface
	fb := sc.perm(id["Flamekin Brawler"], 0)
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
	if !hasOption(&d, MakeAction(AkActivate, TCode(fb), 0, 0)) {
		t.Fatalf("Flamekin Brawler's pump is not offered: %v", d.Options[:d.N])
	}
}
