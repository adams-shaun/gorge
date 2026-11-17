package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// sweepCapScen builds a scenario over the sweepfrom fixture pool: the
// Vampire Nighthawk as the capture (deathtouch and lifelink), vanilla bodies
// around it, and a plain creature filter appended.
func sweepCapScen(t *testing.T) (*scen, map[string]CardID, kop.FilterID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Vampire Nighthawk", Count: 3}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Colossal Dreadmaw", Count: 2}, {Name: "Alpine Grizzly", Count: 3}, {Name: "Hill Giant", Count: 3},
		{Name: "Swamp", Count: 8}, {Name: "Forest", Count: 8}}
	set := cardsettest.Set(t, "sweepfrom")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	creature := kop.FilterID(len(tbl.Filters) - 1)
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	return sc, id, creature
}

// TestSweepFromCapturedBatch: ExtSweepFrom's captured-source sweep (a
// capturing emblem's DamageDealtOnce body, DamageSource$ Remembered) deals
// from the captured object: C&1 excludes the captured creature's own slot,
// C&2 adds each living player after the creatures, and all of it is one
// dealHits batch, so the source's deathtouch marks every hit and its
// lifelink gains once for the batch (gorge effDamageAll's one
// BeginDamageBatch; CR 702.15e). A captured source that left the battlefield
// falls back to the resolving effect, which carries no lifelink: the same
// hits, no gain.
func TestSweepFromCapturedBatch(t *testing.T) {
	sc, id, creature := sweepCapScen(t)
	s := sc.s
	nh := sc.perm(id["Vampire Nighthawk"], 0) // the capture: deathtouch and lifelink
	by := sc.perm(id["Grizzly Bears"], 0)
	dm := sc.perm(id["Colossal Dreadmaw"], 0)
	ag := sc.perm(id["Alpine Grizzly"], 1)
	gi := sc.perm(id["Hill Giant"], 1)
	if !sc.onBF(nh) || !sc.onBF(gi) {
		t.Fatal("precondition: the permanents did not enter")
	}
	s.res.e = stackEntry{kind: seTrigger, ctrl: 0, trigObj: nh, card: id["Vampire Nighthawk"]}
	in := kop.Instr{Op: kop.OpExt, A: kop.Const(int(kop.ExtSweepFrom)),
		Obj: kop.RefTriggerSource, B: kop.Const(2), C: int16(sweepNotSrc | sweepPlHits), F: creature}
	if !s.exec(sc.k, &in) {
		t.Fatal("the sweep suspended")
	}
	dmgOf := func(o Obj) int { return int(s.perms[sc.slot(o)].damage) }
	for _, c := range []struct {
		o   Obj
		dmg int
	}{{by, 2}, {dm, 2}, {ag, 2}, {gi, 2}, {nh, 0}} {
		if got := dmgOf(c.o); got != c.dmg {
			t.Fatalf("damage on %v = %d, want %d", c.o, got, c.dmg)
		}
	}
	if s.pl[0].life != 20-2+12 || s.pl[1].life != 20-2 {
		t.Fatalf("life %d/%d, want seat 0 at 30 (took 2, one batch lifelink gain of 12) and seat 1 at 18",
			s.pl[0].life, s.pl[1].life)
	}
	if s.perms[sc.slot(dm)].flags&pfDeathtouched == 0 {
		t.Fatal("the 6/6 took 2 damage from the deathtouch source without a deathtouch mark")
	}

	// The capture gone: the sweep's source is the resolving effect, with no
	// lifelink, and the nighthawk is now an ordinary sweep victim.
	hc := sc.hand(id["Grizzly Bears"], 1) // an off-battlefield object as the capture
	s.res.e = stackEntry{kind: seTrigger, ctrl: 0, trigObj: hc, card: id["Grizzly Bears"]}
	in.Obj, in.B, in.C, in.F = kop.RefTriggerSource, kop.Const(2), int16(sweepNotSrc|sweepPlHits), creature
	if !s.exec(sc.k, &in) {
		t.Fatal("the gone-source sweep suspended")
	}
	for _, c := range []struct {
		o   Obj
		dmg int
	}{{by, 4}, {dm, 4}, {ag, 4}, {gi, 4}, {nh, 2}} {
		if got := dmgOf(c.o); got != c.dmg {
			t.Fatalf("gone source: damage on %v = %d, want %d", c.o, got, c.dmg)
		}
	}
	if s.pl[0].life != 30-2 || s.pl[1].life != 18-2 {
		t.Fatalf("life %d/%d, want each seat 2 lower with no lifelink gain from the resolving effect",
			s.pl[0].life, s.pl[1].life)
	}
}
