package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestEnterCountersCountFromTheEnteringPermanent: a creature that "enters
// with a +1/+1 counter for each other creature and/or artifact you control"
// counts from the permanent entering and its controller, not from the effect
// that put it onto the battlefield (Rescue Skiff's trigger returning Luxknight
// Breacher to its owner: the Skiff's controller's board is not the count).
func TestEnterCountersCountFromTheEnteringPermanent(t *testing.T) {
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
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature | kop.TArtifact, Ctrl: kop.CtrlYou, Zone: kop.ZoneBattlefield, Flags: kop.FOther})
	f := kop.FilterID(len(tbl.Filters) - 1)
	base := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Count Beast", Types: kop.TCreature, Power: 2, Toughness: 2, Abilities: []kop.Ability{
			{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplEntersProgram}, Prog: []kop.Instr{
				{Op: kop.OpEnterWithCounters, Obj: kop.RefReplaced, A: kop.ValueRef{K: kop.VCountFilter, N: int16(f)}, C: int16(kop.CtrP1P1)}}}}},
		kop.CardDef{Name: "Count Skiff", Types: kop.TArtifact},
		kop.CardDef{Name: "Count Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
	)
	id := map[string]CardID{"Plains": plains, "Luxknight Breacher": base, "Rescue Skiff": base + 1, "Elegy Acolyte": base + 2}
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	skiff := sc.perm(id["Rescue Skiff"], 0)
	sc.perm(id["Elegy Acolyte"], 1)
	sc.perm(id["Elegy Acolyte"], 1)
	g := sc.grave(id["Luxknight Breacher"], 1, 1)[0]
	sc.s.res.e.obj, sc.s.res.e.ctrl = skiff, 0
	sc.s.res.e.tgt[0] = uint8(g)
	sc.s.npend = 0
	if sc.s.execReturn(sc.k, &kop.Instr{Op: kop.OpReturnEx, Obj: kop.RefTarget0, C: int16(kop.ZoneBattlefield)}, NoSlot) {
		t.Fatal("execReturn asked for an Aura host")
	}
	v := kop.ValueRef{K: kop.VCountFilter, N: int16(f)}
	// the resolving effect is the Skiff's, on player 0's side: its count is
	// player 0's board without the Skiff, not the Breacher's
	if got, plain := sc.s.enterCountValue(sc.k, v, sc.s.objs[g].slot, 1), sc.s.value(sc.k, v); got != 2 || plain == got {
		t.Fatalf("the count from the entering permanent is %d (the effect's own count is %d), want 2 and not the effect's", got, plain)
	}
	if !sc.onBF(g) {
		t.Fatal("the Breacher did not return")
	}
	if n := sc.s.perms[sc.s.objs[g].slot].ctrN(sc.s, int(kop.CtrP1P1)); n != 2 {
		t.Fatalf("the Breacher entered under its owner with %d +1/+1 counters, want 2 (its controller's two Bears)", n)
	}
}

// TestEnterCountersLedgerCountsTheEnteringControllers: "for each land that
// entered the battlefield under your control this turn" in an enters-with
// replacement (Bioengineered Future) is the entering creature's controller's
// ledger, not that of the player whose effect returned it (All-Fates
// Stalker's leaves-the-battlefield return of Gigastorm Titan at the end of
// the opponent's turn).
func TestEnterCountersLedgerCountsTheEnteringControllers(t *testing.T) {
	tbl := *v.Table
	tbl.Filters = append(append([]kop.Filter(nil), v.Table.Filters...), kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou, Zone: kop.ZoneBattlefield})
	fid := kop.FilterID(len(v.Table.Filters))
	tbl.Exprs = []kop.Expr{{Kind: kop.ExThisTurnEntered, N: int16(kop.ZoneBattlefield), F: fid, F4: fid, From: -1}}
	sc := newScenTable(t, &tbl, Config{})
	sc.s.enteredRing()
	o := sc.perm(v.GrizzlyBears, 1)
	sc.s.noteEnter(o, kop.ZoneHand, kop.ZoneBattlefield, true)
	sc.s.res.e.ctrl = 0
	val := kop.ValueRef{K: kop.VExpr, N: 0}
	if got := sc.s.enterCountValue(sc.k, val, sc.s.objs[o].slot, 1); got != 1 {
		t.Fatalf("the entering controller's ledger: got %d, want 1", got)
	}
	if got := sc.s.value(sc.k, val); got != 0 {
		t.Fatalf("the effect controller's ledger: got %d, want 0 (the pair has to differ to prove the override)", got)
	}
}
