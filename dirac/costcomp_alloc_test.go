package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCostCompositionMarkAllocs: pricing a spell's target menu one target at a
// time (Luminous Rebuke's reduction depends on its target) marks the build as
// composing its cost, so a "second spell" reduction still sees the cast's own
// push hidden, and allocates nothing doing it. The mark is cleared afterwards,
// and kept when it was already set.
func TestCostCompositionMarkAllocs(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Luminous Rebuke", Count: 1}, {Name: "Burglar Rat", Count: 1}, {Name: "Plains", Count: 8}})
	s := sc.s
	r := sc.perm(id["Burglar Rat"], 1)
	s.perms[sc.slot(r)].flags |= pfTapped
	s.pl[0].pool[kop.W] = 1
	lr := sc.obj(id["Luminous Rebuke"], 0)
	s.place(lr, kop.ZoneHand, false)
	*s.bld() = build{e: stackEntry{kind: seSpell, obj: lr, card: id["Luminous Rebuke"], ctrl: 0, srcVer: s.objs[lr].ver}}
	s.bld().e.tgt = noTargets
	s.ctl = ctlBuild
	if !s.buildLive() {
		t.Fatal("no live build to mark")
	}
	k := sc.k
	cd := &k.t.Cards[id["Luminous Rebuke"]]
	var buf [MaxOptions]uint8
	n := s.targetCands(k, &cd.Abilities[0].Targets[0], 0, NoSlot, lr, &buf)
	if n == 0 {
		t.Fatal("no target candidate for the reprice")
	}
	run := func() {
		b := buf
		s.affordableTargets(k, &b, n)
	}
	run()
	if s.bld().bfl&bflCostComp != 0 {
		t.Fatal("the cost-composition mark outlived the target pricing")
	}
	if a := testing.AllocsPerRun(50, run); a != 0 {
		t.Fatalf("%.0f allocations per target-menu pricing, want 0", a)
	}
	// the mark is set while pricing, and a mark already set stays
	if !s.markCostComposition() || s.bld().bfl&bflCostComp == 0 {
		t.Fatal("markCostComposition did not set the bit on a live spell build")
	}
	if s.markCostComposition() {
		t.Fatal("a second mark reports it set the bit")
	}
	s.unmarkCostComposition(false)
	if s.bld().bfl&bflCostComp == 0 {
		t.Fatal("restoring a mark that was not ours cleared the bit")
	}
	s.unmarkCostComposition(true)
	if s.bld().bfl&bflCostComp != 0 {
		t.Fatal("restoring our mark left the bit set")
	}
}
