package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// plainWitnessScen is p0 at priority in its main phase with Guardian of the
// Great Door in hand: one Plains, a Treasure and four Mountains to tap for the
// additional cost, and p1's Zoetic Glyph on the Treasure (glyphOwner 0 puts the
// Glyph under p0). extraPlains adds a second Plains, the witness that pays
// without sacrificing.
func plainWitnessScen(t *testing.T, extraPlains bool, glyphOwner uint8) (*scen, map[string]CardID, map[string]Obj) {
	t.Helper()
	board := []string{"Plains", "c_a_treasure_sac", "Mountain", "Mountain", "Mountain", "Mountain"}
	if extraPlains {
		board = append(board, "Plains")
	}
	sc, id, objs := precastScen(t, board...)
	objs["Guardian of the Great Door"] = sc.hand(id["Guardian of the Great Door"], 0)
	objs["Zoetic Glyph"] = sc.perm(id["Zoetic Glyph"], glyphOwner)
	sc.s.attachTo(sc.k, sc.slot(objs["Zoetic Glyph"]), sc.slot(objs["c_a_treasure_sac"]))
	return sc, id, objs
}

// witnessSacrificePends plans Guardian's cost on a copy of the state, sacrifices
// the source, and asserts the pending trigger is the Glyph's
// leaves-the-battlefield ability controlled by glyphOwner: the trigger the cast
// is offered over (it waits for the cast, CR 601.2g, 603.3b).
func witnessSacrificePends(t *testing.T, sc *scen, c CardID, o Obj, glyphOwner uint8) {
	t.Helper()
	cost := sc.s.featCost(sc.k, 0, c, o, sc.k.cards[c].cost)
	tmp := sc.s.fork()
	defer sc.s.unfork()
	n0 := tmp.npend
	var pk planner
	pk.try = true
	tmp.tapForPK(sc.k, 0, &cost, 0, o, false, false, &pk)
	if !pk.found {
		t.Fatal("the potential planner found no witness for Guardian's cost")
	}
	if !plannerConsumesSac(&pk) {
		t.Fatal("the witness does not sacrifice a source; the test premise fails")
	}
	tmp.sba(sc.k)
	if tmp.npend <= n0 {
		t.Fatal("the witness's sacrifice pends no trigger; the test premise fails")
	}
	pt := tmp.pend[n0]
	cd := &sc.k.t.Cards[pt.card]
	if cd.Name != "Zoetic Glyph" {
		t.Fatalf("the sacrifice pends %q's trigger, want Zoetic Glyph's", cd.Name)
	}
	if int(pt.ab) >= len(cd.Abilities) {
		t.Fatalf("pending ability %d is out of range for Zoetic Glyph", pt.ab)
	}
	if a := &cd.Abilities[pt.ab]; a.Kind != kop.AbTriggered || a.Trig.Event != kop.TrigDies && a.Trig.Event != kop.TrigLeaves {
		t.Fatalf("pending ability is not a leaves-the-battlefield trigger: kind %d event %d", a.Kind, a.Trig.Event)
	}
	if pt.ctrl != glyphOwner {
		t.Fatalf("the pending Glyph trigger is controlled by p%d, want p%d", pt.ctrl, glyphOwner)
	}
}

// witnessKeepsSources plans Guardian's cost on a copy of the state and reports
// whether the witness keeps the sacrifice sources (gorge's
// potentialWitnessKeeping): the price of the board whose second Plains pays
// {W}{W}, so no sacrifice trigger waits.
func witnessKeepsSources(t *testing.T, sc *scen, c CardID, o Obj) bool {
	t.Helper()
	cost := sc.s.featCost(sc.k, 0, c, o, sc.k.cards[c].cost)
	tmp := sc.s.fork()
	defer sc.s.unfork()
	var pk planner
	pk.try = true
	tmp.tapForPK(sc.k, 0, &cost, 0, o, false, false, &pk)
	if !pk.found {
		t.Fatal("the potential planner found no witness for Guardian's cost")
	}
	return !plannerConsumesSac(&pk)
}

// guardianOffered steps to p0's priority decision and reports whether it offers
// the cast of hand object o.
func guardianOffered(t *testing.T, sc *scen, o Obj) bool {
	t.Helper()
	d := sc.s.Step()
	if d.Kind != DkPriority {
		t.Fatalf("decision kind %d, want priority", d.Kind)
	}
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == o {
			return true
		}
	}
	return false
}

// TestPlainWitnessOfferedWhenSacrificeTriggerWaitsForTheCast: the witness that
// pays Guardian's {W}{W} sacrifices a source, and the sacrifice sets off the
// opponent's Zoetic Glyph discover trigger. That trigger waits for the cast to
// finish (CR 601.2g, 603.3b), so the play stays a legal action and dirac keeps
// it in the lowering surface -- whether the trigger is p0's or p1's; a second
// Plains is the control witness that pays without sacrificing (gorge's
// potentialWitnessKeeping).
func TestPlainWitnessOfferedWhenSacrificeTriggerWaitsForTheCast(t *testing.T) {
	sc, id, objs := plainWitnessScen(t, false, 1)
	c := id["Guardian of the Great Door"]
	o := objs["Guardian of the Great Door"]
	treasure, glyph := sc.slot(objs["c_a_treasure_sac"]), sc.slot(objs["Zoetic Glyph"])
	if got := sc.s.perms[glyph].attached; got != treasure {
		t.Fatalf("Zoetic Glyph attached to slot %d, want the Treasure's %d", got, treasure)
	}
	if !sc.s.anySacSource(sc.k, 0) {
		t.Fatal("the Treasure is not a source the plan could sacrifice")
	}
	if pips := sc.k.cards[c].cost.Pips[kop.W]; pips != 2 {
		t.Fatalf("Guardian's cost has %d white pips, want 2", pips)
	}
	if ac := spellAddCost(sc.k, c); ac.Flags&kop.CostTapX == 0 {
		t.Fatal("Guardian's additional cost is not the tap-four part the test pays with the Mountains")
	}
	if sc.s.FeatPlannedCast(0, o, 0) {
		t.Fatal("gorge's cast planner prices Guardian; the potential witness is never reached")
	}
	witnessSacrificePends(t, sc, c, o, 1)
	if !guardianOffered(t, sc, o) {
		t.Fatal("Guardian is not offered although the sacrifice's opponent trigger waits for the cast")
	}

	// A second Plains: the potential planner keeps the Treasure, so no
	// sacrifice trigger waits at all; the cast is offered either way.
	sc, id, objs = plainWitnessScen(t, true, 1)
	c, o = id["Guardian of the Great Door"], objs["Guardian of the Great Door"]
	if !witnessKeepsSources(t, sc, c, o) {
		t.Fatal("with a second Plains the planner still sacrifices the Treasure; the control witness is not kept")
	}
	if !guardianOffered(t, sc, o) {
		t.Fatal("Guardian is not offered with a second Plains paying its {W}{W}")
	}

	// The Glyph under p0: its discover trigger is p's own, which the witness
	// walk answers.
	sc, id, objs = plainWitnessScen(t, false, 0)
	c, o = id["Guardian of the Great Door"], objs["Guardian of the Great Door"]
	witnessSacrificePends(t, sc, c, o, 0)
	if !guardianOffered(t, sc, o) {
		t.Fatal("Guardian is not offered when the sacrifice trigger is p0's own")
	}
}
