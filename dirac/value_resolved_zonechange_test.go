package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestResolvedThisTurnSurvivesZoneChange pins the identity
// Count$ResolvedThisTurn uses. gorge's events.ResolvedAbilityKey is
// (source ObjID, ability body) with no zone stamp, so a permanent that
// leaves and returns keeps its tally; the kernel used to carry the
// source's ver in each record and drop the earlier resolutions at the
// zone change, so South Pole Voyager's second resolution skipped its
// draw (std-tla pair 1478).
func TestResolvedThisTurnSurvivesZoneChange(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Harvestrite Host", Count: 2}, {Name: "Plains", Count: 8}})
	hh := id["Harvestrite Host"]
	ab := histAb(t, sc, hh, kop.TrigETB)
	if sc.k.t.Cards[hh].Abilities[ab].AFlags&kop.AfResolveCount == 0 {
		t.Fatal("Harvestrite Host's trigger does not count its resolutions")
	}
	o := sc.perm(hh, 0)
	resolve := func() int {
		sc.s.res.active = true
		sc.s.res.e = stackEntry{kind: seTrigger, obj: o, srcVer: sc.s.objs[o].ver, card: hh, ab: ab, ctrl: 0}
		if !sc.s.trigResolveOK(sc.k) {
			t.Fatal("the trigger has no intervening-if")
		}
		n := sc.s.resolvedThisTurn(sc.k)
		sc.s.res.active = false
		return n
	}
	if n := resolve(); n != 1 {
		t.Fatalf("first resolution reads %d, want 1", n)
	}
	// The zone change bumps ver (CR 400.7's new object). The old record
	// matched that ver, so the first resolution vanished from the tally.
	before := sc.s.objs[o].ver
	sc.s.objs[o].ver++
	if sc.s.objs[o].ver == before {
		t.Fatal("precondition: the zone change did not advance the source's ver")
	}
	if n := resolve(); n != 2 {
		t.Fatalf("second resolution after a zone change reads %d, want 2 (gorge events.ResolvedAbilityKey has no zone stamp)", n)
	}
	// A different permanent of the same card still has its own tally.
	other := sc.perm(hh, 0)
	sc.s.res.active = true
	sc.s.res.e = stackEntry{kind: seTrigger, obj: other, srcVer: sc.s.objs[other].ver, card: hh, ab: ab, ctrl: 0}
	if n := sc.s.resolvedThisTurn(sc.k); n != 0 {
		t.Fatalf("a second Host reads %d resolutions, want 0", n)
	}
	sc.s.res.active = false
}
