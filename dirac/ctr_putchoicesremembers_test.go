package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// putChoicesScen builds a resolving spell whose program is the PutCounter
// Choices$ recipient pick over creatures followed by an OpPutCounter on the
// pick (remember: its B=2 RememberCards$ arm, amount n), with lions Savannah
// Lions on seat 0's battlefield.
func putChoicesScen(t *testing.T, lions, n int, remember bool) (*scen, []Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Savannah Lions", Count: 4}, {Name: "Plains", Count: 16}}
	set := cardsettest.Set(t, "fdn")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	creature := kop.FilterID(len(tbl.Filters) - 1)
	put := kop.Instr{Op: kop.OpPutCounter, Obj: kop.RefChosen, A: kop.Const(n), C: int16(kop.CtrP1P1)}
	if remember {
		put.B = kop.Const(2)
	}
	pick := kop.Instr{Op: kop.OpChooseCard, Obj: kop.RefController, F: creature, C: choosePutCounterPick}
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Put Choices", Types: kop.TArtifact,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered, Prog: []kop.Instr{pick, put}}}})
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	s := sc.s
	var objs []Obj
	for i := 0; i < lions; i++ {
		objs = append(objs, sc.perm(id["Savannah Lions"], 0))
	}
	srcObj := sc.perm(src, 0)
	e := stackEntry{kind: seTrigger, obj: srcObj, srcVer: s.objs[srcObj].ver, card: src, ab: 0, ctrl: 0, trigObj: NoObj}
	s.sp = 1
	s.stack[0] = e
	s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
	return sc, objs
}

func ctrOf(sc *scen, o Obj) int { return sc.s.perms[sc.s.objs[o].slot].ctrLow(int(kop.CtrP1P1)) }

// TestPutCounterChoicesRemembers: PutCounter's Choices$ pick with
// RememberCards$ (OpPutCounter B=2 on RefChosen) remembers the creature that
// took the counters, only when the CounterNum$ amount is > 0, and nothing
// without the flag (gorge counters_put.go putCounterPickApply/rememberPlaced).
func TestPutCounterChoicesRemembers(t *testing.T) {
	// a lone candidate is taken unasked and remembered
	sc, lions := putChoicesScen(t, 1, 1, true)
	if sc.s.g.nrem != 0 {
		t.Fatalf("precondition: %d objects remembered before the run", sc.s.g.nrem)
	}
	sc.s.runProg(sc.k)
	if sc.s.g.nrem != 1 || sc.s.g.rem[0] != lions[0] {
		t.Fatalf("remembered %d objects (first %d), want the lone creature %d", sc.s.g.nrem, sc.s.g.rem[0], lions[0])
	}
	if ctrOf(sc, lions[0]) != 1 {
		t.Fatalf("the creature has %d +1/+1 counters, want 1", ctrOf(sc, lions[0]))
	}

	// the same program without the flag remembers nothing
	sc, lions = putChoicesScen(t, 1, 1, false)
	sc.s.runProg(sc.k)
	if ctrOf(sc, lions[0]) != 1 || sc.s.g.nrem != 0 {
		t.Fatalf("without B=2: counters %d, remembered %d, want 1 and 0", ctrOf(sc, lions[0]), sc.s.g.nrem)
	}

	// an amount of 0 places nothing and remembers nothing
	sc, lions = putChoicesScen(t, 1, 0, true)
	sc.s.runProg(sc.k)
	if ctrOf(sc, lions[0]) != 0 || sc.s.g.nrem != 0 {
		t.Fatalf("CounterNum 0: counters %d, remembered %d, want 0 and 0", ctrOf(sc, lions[0]), sc.s.g.nrem)
	}

	// no creature: nothing to pick, nothing remembered
	sc, _ = putChoicesScen(t, 0, 1, true)
	sc.s.runProg(sc.k)
	if sc.s.g.nrem != 0 {
		t.Fatalf("no creature: remembered %d, want none", sc.s.g.nrem)
	}

	// two candidates ask; the picked one (the second) is the one remembered
	sc, lions = putChoicesScen(t, 2, 1, true)
	sc.s.runProg(sc.k)
	if sc.s.res.ask != raGenChooseCard {
		t.Fatalf("two candidates: ask %d, want the pick ask %d", sc.s.res.ask, raGenChooseCard)
	}
	d := sc.s.Step()
	if d.Kind == DkNone || int(d.N) < 2 {
		t.Fatalf("pick decision %v with %d options, want a choice of 2", d.Kind, d.N)
	}
	var pick Action
	found := false
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == AkChoose && Obj(d.Options[i].Source()) == lions[1] {
			pick, found = d.Options[i], true
		}
	}
	if !found {
		t.Fatalf("no option names the second creature (options %v)", d.Options[:d.N])
	}
	sc.s.Apply(pick)
	if ctrOf(sc, lions[1]) != 1 || ctrOf(sc, lions[0]) != 0 {
		t.Fatalf("counters first %d second %d, want 0 and 1", ctrOf(sc, lions[0]), ctrOf(sc, lions[1]))
	}
	if sc.s.g.nrem != 1 || sc.s.g.rem[0] != lions[1] {
		t.Fatalf("remembered %d (first %d), want only the picked creature %d", sc.s.g.nrem, sc.s.g.rem[0], lions[1])
	}
}
