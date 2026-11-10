package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDelayedSelfRegistrationVersion: a delayed program's pending trigger
// carries the source's version as it stood when the program registered
// (CR 603.7c), not the version at fire. A source that left and returned in
// between is a new object, so the body's Self (moveObj/objRef compare the
// live version with srcVer) must not name it. Both fire paths are covered:
// the end-step fast path and fireDelayedWhen (end of combat).
func TestDelayedSelfRegistrationVersion(t *testing.T) {
	for _, tc := range []struct {
		name string
		when int
		fire func(*State, *prep)
	}{
		{"end step", 0, (*State).fireDelayedGen},
		{"end combat", kop.DelayEndCombat, (*State).fireDelayedEndCombat},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deck := []cardset.Entry{{Name: "Plains", Count: 20}}
			set := cardsettest.Set(t, "fdn")
			tbl, _, _, err := set.Matchup(deck, deck)
			if err != nil {
				t.Fatal(err)
			}
			gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
			c := CardID(len(tbl.Cards))
			tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bauble", Types: kop.TArtifact, Abilities: []kop.Ability{
				{Kind: kop.AbActivated, Prog: []kop.Instr{{Op: kop.OpDelayedProgram, C: 1, A: kop.Const(tc.when)}}},
				{Kind: kop.AbTriggered, AFlags: kop.AfLatent, Prog: gain}}})
			sc := newScenTable(t, tbl, Config{NoElide: true})
			o := sc.perm(c, 0)
			sc.s.res.e.ctrl, sc.s.res.e.obj, sc.s.res.e.card = 0, o, c
			for i := range tbl.Cards[c].Abilities[0].Prog {
				sc.s.exec(sc.k, &tbl.Cards[c].Abilities[0].Prog[i])
			}
			if sc.s.g.ndp != 1 {
				t.Fatalf("%d delayed programs registered, want 1", sc.s.g.ndp)
			}
			v0 := sc.s.objs[o].ver
			sc.s.leave(sc.k, sc.slot(o), kop.ZoneExile, false)
			sc.s.enter(sc.k, o, 0, false, kop.ZoneExile)
			if sc.s.objs[o].ver == v0 {
				t.Fatalf("precondition: ver = %d before and after the blink, want a new object", v0)
			}
			n0 := sc.s.npend
			tc.fire(sc.s, sc.k)
			if sc.s.npend != n0+1 {
				t.Fatalf("%d pending after the fire, want %d", sc.s.npend, n0+1)
			}
			if got := sc.s.pend[n0].srcVer; got != v0 {
				t.Fatalf("pending srcVer = %d, want the registration version %d (live ver %d)", got, v0, sc.s.objs[o].ver)
			}
		})
	}
}
