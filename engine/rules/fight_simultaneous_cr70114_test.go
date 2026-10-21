package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 701.14a, cr-g18b: the two hits of a fight are dealt simultaneously, so
// both powers are read before either hit lands, and each hit is the
// fighting creature's own (its wither is read). cr70114DamageSince and
// cr70114TargetFightSA live in fight_cr70114_test.go (cr-g18a).

// TestCR701_14aInfectFightHitsAreSimultaneous: a 3/3 infect fighter fights
// a 3/3. Both powers are read before either hit lands (CR 701.14a), so the
// three -1/-1 counters the infect hit puts on the 3/3 (CR 702.90b) do not
// zero its hit back: the infect creature takes 3.
func TestCR701_14aInfectFightHitsAreSimultaneous(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	blight := onBoard(t, e, 0, "Name:Blight\nManaCost:2 B\nTypes:Creature Horror\nPT:3/3\nK:Infect\nOracle:x\n")
	bear := onBoard(t, e, 1, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:3/3\nOracle:x\n")
	if !e.HasKeyword(blight, "Infect") || e.Power(bear) != 3 {
		t.Fatalf("precondition: Blight infect %v, Bear power %d", e.HasKeyword(blight, "Infect"), e.Power(bear))
	}
	mark := len(e.L.Events)
	effects.Resolve(e, &effects.Ctx{Source: blight, Controller: 0,
		Targets: []state.Target{{Obj: bear}}, TargetsOffered: true}, cr70114TargetFightSA())
	var toBlight []events.Event
	for _, ev := range cr70114DamageSince(e, mark) {
		if ev.Obj == blight {
			toBlight = append(toBlight, ev)
		}
	}
	if len(toBlight) != 1 || toBlight[0].Amount != 3 {
		t.Fatalf("Bear -> Blight damage = %+v, want one 3: CR 701.14a, both powers are read before either hit (the infect counters must not zero the hit back)", toBlight)
	}
}

// TestCR701_14aFightHitsAreSimultaneous: Boggart Ram-Gang (3/3, wither)
// fights a 3/3. Both powers are read before either hit lands (CR 701.14a,
// "each deals damage equal to its power to the other"), so the wither
// -1/-1 counters on the 3/3 do not shrink its hit back: the Ram-Gang takes 3.
func TestCR701_14aFightHitsAreSimultaneous(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := handEngine(t)
	ram := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Boggart Ram-Gang"))
	bear := onBoard(t, e, 1, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:3/3\nOracle:x\n")
	if !e.HasKeyword(ram, "Wither") || e.Power(ram) != 3 || e.Power(bear) != 3 {
		t.Fatalf("precondition: Ram-Gang 3-power wither (%v, %d), Bear power %d", e.HasKeyword(ram, "Wither"), e.Power(ram), e.Power(bear))
	}
	mark := len(e.L.Events)
	effects.Resolve(e, &effects.Ctx{Source: ram, Controller: 0,
		Targets: []state.Target{{Obj: bear}}, TargetsOffered: true}, cr70114TargetFightSA())
	if got := e.G.Obj(bear).Counter("M1M1"); got != 3 {
		t.Errorf("Bear -1/-1 counters = %d, want 3: CR 702.80a, the fighting Ram-Gang is the source and its wither hit lands as counters", got)
	}
	var toRam []events.Event
	for _, ev := range cr70114DamageSince(e, mark) {
		if ev.Obj == ram {
			toRam = append(toRam, ev)
		}
	}
	if len(toRam) != 1 || toRam[0].Amount != 3 {
		t.Fatalf("Bear -> Ram-Gang damage = %+v, want one 3: CR 701.14a, both powers are read before either hit (wither must not shrink the hit back)", toRam)
	}
}
