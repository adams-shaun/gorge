package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// ccFlush steps the priority round so a state-based action is checked after
// a resolution (a 2/2 holding 2 damage dies at the next check, not inside
// the resolution).
func ccFlush(t *testing.T, sc *scen) {
	t.Helper()
	for i := 0; i < 4; i++ {
		d := sc.s.Step()
		if d.Kind == DkPriority {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
			continue
		}
		return
	}
}

// ccScen builds a countercosts scenario at seat 0's main phase 1 with an
// empty hand and the given cards in the table (keclScen over the
// countercosts pool).
func ccScen(t *testing.T, deck []cardset.Entry) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Set(t, "countercosts")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	lib, ok := id["Swamp"]
	if !ok {
		lib, ok = id["Mountain"]
	}
	if !ok {
		t.Fatal("no basic in the countercosts table")
	}
	return newScenTableLib(t, tbl, lib, Config{NoElide: true}), id
}

// TestCounterCostSpellBlightPaid: Lethal Sting's chooser counter cost
// (AddCounter<1/M1M1/Creature.YouCtrl>, lowered to a Blight cost) picks a
// creature the caster controls at payment, the cast then resolves, and with
// no creature to blight the cast is not offered at all.
func TestCounterCostSpellBlightPaid(t *testing.T) {
	sc, id := ccScen(t, []cardset.Entry{
		{Name: "Lethal Sting", Count: 1},
		{Name: "Grizzly Bears", Count: 3},
		{Name: "Swamp", Count: 20},
	})
	bear := sc.perm(id["Grizzly Bears"], 0)
	sc.perm(id["Grizzly Bears"], 0)
	tgt := sc.perm(id["Grizzly Bears"], 1)
	sc.lands(0, id["Swamp"], 3)
	sp := sc.hand(id["Lethal Sting"], 0)
	sc.do(t, AkCast, TCode(sp), scAny)
	sc.do(t, AkTarget, scAny, TCode(tgt)) // a spell's target ask comes first
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChCostBlight || d.N != 2 {
		t.Fatalf("blight pick kind %d sub %d n %d, want ChCostBlight with 2 candidates", d.Kind, d.Sub, d.N)
	}
	sc.s.Apply(MakeAction(AkChoose, TCode(bear), 0, 0))
	keclBuildDone(t, sc)
	bearSlot, tgtSlot := sc.slot(bear), sc.slot(tgt) // the target dies resolving
	keclResolveAll(t, sc)
	if n := sc.s.perms[bearSlot].ctrLow(int(kop.CtrM1M1)); n != 1 {
		t.Errorf("the blighted creature has %d -1/-1 counters, want 1", n)
	}
	if sc.onBF(tgt) {
		t.Error("the destroyed target is still on the battlefield: the cast did not resolve")
	}
	if n := sc.s.perms[tgtSlot].ctrLow(int(kop.CtrM1M1)); n != 0 {
		t.Errorf("the un-blighted target has %d -1/-1 counters", n)
	}

	// No creature to blight: the cast is not offered.
	sc2, id2 := ccScen(t, []cardset.Entry{
		{Name: "Lethal Sting", Count: 1},
		{Name: "Swamp", Count: 20},
	})
	sc2.lands(0, id2["Swamp"], 3)
	sp2 := sc2.hand(id2["Lethal Sting"], 0)
	d = sc2.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(sp2)) {
			t.Fatalf("a chooser counter cost with no creature is offered: %v", d.Options[:d.N])
		}
	}
}

// TestCounterCostAbilityBlightPaid: Hatchet Bully's activation pays its
// chooser counter cost the same way ({2}{R}, {T}, put a -1/-1 counter on a
// creature you control: deal 2 damage), and with no other creature the
// source itself is the unasked pick.
func TestCounterCostAbilityBlightPaid(t *testing.T) {
	sc, id := ccScen(t, []cardset.Entry{
		{Name: "Hatchet Bully", Count: 1},
		{Name: "Grizzly Bears", Count: 2},
		{Name: "Mountain", Count: 20},
	})
	hb := sc.perm(id["Hatchet Bully"], 0)
	bear := sc.perm(id["Grizzly Bears"], 0)
	tgt := sc.perm(id["Grizzly Bears"], 1)
	sc.lands(0, id["Mountain"], 3)
	sc.do(t, AkActivate, TCode(hb), scAny)
	sc.do(t, AkTarget, scAny, TCode(tgt))
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChCostBlight || d.N != 2 {
		t.Fatalf("blight pick kind %d sub %d n %d, want ChCostBlight with 2 candidates", d.Kind, d.Sub, d.N)
	}
	sc.s.Apply(MakeAction(AkChoose, TCode(bear), 0, 0))
	keclBuildDone(t, sc)
	bearSlot, tgtSlot := sc.slot(bear), sc.slot(tgt) // the target dies resolving
	keclResolveAll(t, sc)
	ccFlush(t, sc)
	if n := sc.s.perms[bearSlot].ctrLow(int(kop.CtrM1M1)); n != 1 {
		t.Errorf("the blighted creature has %d -1/-1 counters, want 1", n)
	}
	if n := sc.s.perms[tgtSlot].ctrLow(int(kop.CtrM1M1)); n != 0 {
		t.Errorf("the damaged creature has %d -1/-1 counters", n)
	}
	if sc.onBF(tgt) {
		t.Error("the 2/2 target survived the 2 damage: the ability did not resolve")
	}

	// No other creature: the source itself is the unasked pick.
	sc2, id2 := ccScen(t, []cardset.Entry{
		{Name: "Hatchet Bully", Count: 1},
		{Name: "Mountain", Count: 20},
	})
	hb2 := sc2.perm(id2["Hatchet Bully"], 0)
	sc2.lands(0, id2["Mountain"], 3)
	sc2.do(t, AkActivate, TCode(hb2), scAny)
	d = sc2.s.Step()
	if d.Kind != DkTarget {
		t.Fatalf("source-only activation decision kind %d, want the target ask (no blight pick)", d.Kind)
	}
	sc2.s.Apply(MakeAction(AkTarget, 0, PlayerTarget0+1, 0)) // the opponent: "any target"
	keclBuildDone(t, sc2)
	keclResolveAll(t, sc2)
	ccFlush(t, sc2)
	if n := sc2.s.perms[sc2.slot(hb2)].ctrLow(int(kop.CtrM1M1)); n != 1 {
		t.Errorf("the source has %d -1/-1 counters, want its own blight pick's 1", n)
	}
}
