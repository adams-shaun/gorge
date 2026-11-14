package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestXWitnessSkipsADeferredRiderSource: gorge's lowered {X} witness
// (potentialModeCastPlan) pays the largest X the planner's census funds, and
// that census never lists Twitching Doll's counter-rider mana ("source:rider",
// mana_rider.go). The kernel's witness census (availUngranted, used when the
// table has granted mana abilities) must skip it as availFor does, or it prices
// The Rollercrusher Ride ({X}{2}{R}) one X too high and taps the Doll, which
// puts a nest counter on it (CR 601.2b, 605.3b).
func TestXWitnessSkipsADeferredRiderSource(t *testing.T) {
	sc, id := xScen(t, "std-dsk", []cardset.Entry{{Name: "The Rollercrusher Ride", Count: 1}, {Name: "Twitching Doll", Count: 1}, {Name: "Enduring Vitality", Count: 1}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8}})
	// The full surface (the lockstep rows' autopay=false) lowers the hand cast
	// gorge's cast planner never prices (cast.go, gorgeOffers); xScen's autopay
	// surface alone offers no {X} cast from lands and never reaches lowerWitness.
	sc.s.cfg = sc.s.cfg&^cfgGorgeAutopay | cfgGorgeLowering
	// Enduring Vitality's granted mana ability gives the table grantPC, so the
	// X witness takes availUngranted.
	if sc.k.gen == nil || len(sc.k.gen.grantPC) == 0 {
		t.Fatal("precondition: the table has no granted mana ability, so availUngranted is never reached")
	}
	lands := []Obj{sc.perm(id["Mountain"], 0), sc.perm(id["Mountain"], 0), sc.perm(id["Forest"], 0)}
	doll := sc.perm(id["Twitching Doll"], 0)
	if !sc.s.scriptTapsDefer(sc.k, 0).has(sc.slot(doll)) {
		t.Fatal("precondition: the Doll is not a planner-deferred source")
	}
	if sc.tapped(doll) {
		t.Fatal("precondition: the Doll is tapped")
	}
	nest := int(kop.CtrNest)
	if n := sc.s.perms[sc.slot(doll)].ctrN(sc.s, nest); n != 0 {
		t.Fatalf("precondition: the Doll entered with %d nest counters", n)
	}
	ride := sc.hand(id["The Rollercrusher Ride"], 0)
	sc.do(t, AkCast, TCode(ride), scAny)
	if d := sc.s.Step(); d.Kind != DkChoose || d.Sub != ChX {
		t.Fatalf("decision kind %d sub %d, want the X ask", d.Kind, d.Sub)
	}
	if sc.tapped(doll) {
		t.Fatal("the witness tapped Twitching Doll, which gorge's census never lists")
	}
	if n := sc.s.perms[sc.slot(doll)].ctrN(sc.s, nest); n != 0 {
		t.Fatalf("the Doll has %d nest counters, want none", n)
	}
	for i, l := range lands {
		if !sc.tapped(l) {
			t.Fatalf("land %d is untapped: the witness pays {2}{R} (X = 0) from the three lands", i)
		}
	}
}
