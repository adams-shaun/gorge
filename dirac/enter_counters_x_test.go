package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEnterCountersXIsTheEnteringPermanentsOwn: "enters with X +1/+1
// counters" reads the entering permanent's own X (CR 107.3f), so a creature
// put onto the battlefield by another effect enters with none, whatever X
// that effect was cast for (Pterafractyl put in by Mind into Matter).
func TestEnterCountersXIsTheEnteringPermanentsOwn(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	var plains CardID = 255
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Plains" {
			plains = CardID(i)
		}
	}
	base := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "X Beast", Types: kop.TCreature, Power: 1, Toughness: 1, Abilities: []kop.Ability{
			{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplEntersProgram}, Prog: []kop.Instr{
				{Op: kop.OpEnterWithCounters, Obj: kop.RefReplaced, A: kop.ValueRef{K: kop.VX}, C: int16(kop.CtrP1P1)}}}}},
		kop.CardDef{Name: "Count Skiff", Types: kop.TArtifact},
	)
	sc := newScenTableLib(t, tbl, plains, Config{NoElide: true})
	src := sc.perm(base+1, 0)
	g := sc.grave(base, 0, 1)[0]
	sc.s.res.active = true
	sc.s.res.e.obj, sc.s.res.e.ctrl, sc.s.res.e.x = src, 0, 4
	sc.s.res.e.tgt[0] = TCode(g)
	sc.s.npend = 0
	if sc.s.execReturn(sc.k, &kop.Instr{Op: kop.OpReturnEx, Obj: kop.RefTarget0, C: int16(kop.ZoneBattlefield)}, NoSlot) {
		t.Fatal("execReturn asked for an Aura host")
	}
	if !sc.onBF(g) {
		t.Fatal("the beast did not return")
	}
	if n := sc.s.perms[sc.s.objs[g].slot].ctrN(sc.s, int(kop.CtrP1P1)); n != 0 {
		t.Fatalf("entered with %d +1/+1 counters from the returning effect's X, want 0", n)
	}
	if sc.s.res.e.x != 4 {
		t.Fatalf("the effect's own X was left at %d, want 4", sc.s.res.e.x)
	}
}
