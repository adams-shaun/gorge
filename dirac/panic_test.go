package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestPanicNativeEntryProgram: an encore-style sacrifice and a ward payment
// sacrifice or discard while a kernel-native entry resolves, on a table where
// some printed Sacrifice remembers. Such an entry has no program to read and
// remembers nothing.
func TestPanicNativeEntryProgram(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 4}, {Name: "Plains", Count: 8}})
	s, k := sc.s, sc.k
	k.gen.anyRemSac = true

	fc := id["Valley Floodcaller"]
	g := sc.perm(fc, 0)
	s.sacAtEndUntracked(g, 0)
	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkDone, 0xff, 0xff)
	sc.passUntil(t, kop.StepEnd)
	if d := s.Step(); d.Kind != DkPriority || s.sp != 1 || s.stack[0].ab != abDelayed {
		t.Fatalf("the end-step sacrifice is not on the stack: sp %d", s.sp)
	}
	sc.passBoth(t)
	s.Step()
	if sc.onBF(g) || s.g.nrem != 0 {
		t.Fatalf("on battlefield %v, remembered %d; want sacrificed and nothing remembered", sc.onBF(g), s.g.nrem)
	}

	for _, ab := range []uint8{abMadness, abDelayed, abWard, abProwess, 255} {
		p := sc.perm(fc, 0)
		s.res = resCtx{e: stackEntry{card: fc, ab: ab}, active: true}
		s.fireSac(k, sc.slot(p))
		if op := s.discBatchOpen(k); op != nil || s.g.nrem != 0 {
			t.Fatalf("ab %d: resolving discard %v, remembered %d; want neither", ab, op, s.g.nrem)
		}
	}
}

// TestPanicManyManaOneColour: seven mana of any one colour pays one colour's
// demand. {2}{B}{B}{G} from a Swamp, a Forest and that source takes the Forest
// for {G} and the source for {B}{B}; the Swamp and the source cannot pay both
// colours, though the sum of what they make covers them.
func TestPanicManyManaOneColour(t *testing.T) {
	tb := *v.Table
	tb.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	seven := tapMana(kop.ColAny)
	seven.ManaN = 7
	tb.Cards = append(tb.Cards, kop.CardDef{Name: "Test Seven", Types: kop.TArtifact, Abilities: []kop.Ability{seven}})
	sc := newScenTable(t, &tb, Config{})
	src := sc.perm(CardID(len(tb.Cards)-1), 0)
	swamp := sc.perm(v.Swamp, 0)
	forest := sc.perm(v.Forest, 0)
	c := cost(2, kop.B, kop.B, kop.G)
	sc.s.pay(sc.k, 0, &c, 0, NoObj)
	if !sc.tapped(src) || !sc.tapped(forest) || sc.tapped(swamp) {
		t.Fatalf("tapped source %v, Forest %v, Swamp %v; want the source and the Forest", sc.tapped(src), sc.tapped(forest), sc.tapped(swamp))
	}
}

// TestPanicReducedActivationWitness: the prefix search's cost prices an
// activation as the offer and the charge do. {10} less nine is one
// land, which pays; the printed {10} has no plan.
func TestPanicReducedActivationWitness(t *testing.T) {
	var mech CardID
	sc, id := newBLBScenTweak(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 4}, {Name: "Plains", Count: 8}}, func(tb *kop.Table, id map[string]CardID) {
		mech = id["Valley Floodcaller"]
		tb.Cards[mech].Abilities = []kop.Ability{{Kind: kop.AbActivated, Cost: kop.Cost{Mana: kop.ManaCost{Generic: 10}}, OwnReduce: kop.Const(9)}}
	})
	s, k := sc.s, sc.k
	k.gen.anyOwnReduce = true
	src := sc.perm(mech, 0)
	land := sc.perm(id["Plains"], 0)
	a := MakeAction(AkActivate, TCode(src), 0, 0)
	c, _, _, ok := s.grantPlayCost(k, 0, a, false)
	if !ok {
		t.Fatal("no cost for the activation")
	}
	s.tapFor(k, 0, &c, 0, NoObj)
	if !sc.tapped(land) {
		t.Fatal("the land was not tapped for the reduced cost")
	}
}

// TestPanicAffinityTreasure: a Treasure reduces an affinity spell's cost and
// pays for it. The cost was fixed before the mana was made (CR 601.2f), so
// sacrificing the Treasure does not raise it past what the sources made.
func TestPanicAffinityTreasure(t *testing.T) {
	deck := []cardset.Entry{{Name: "Valkyrie Aerial Unit", Count: 1}, {Name: "The Gold Saucer", Count: 1}, {Name: "Island", Count: 4}, {Name: "Plains", Count: 4}}
	tbl, da, _, err := cardsettest.Synth(t, "std-fin").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	tre := CardID(255)
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "c_a_treasure_sac" {
			tre = CardID(i)
		}
	}
	if tre == 255 {
		t.Fatal("no Treasure token in the table")
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	for i := 0; i < 2; i++ {
		sc.perm(id["Island"], 0)
	}
	for i := 0; i < 3; i++ {
		sc.perm(id["Plains"], 0)
	}
	tr := sc.obj(tre, 0)
	sc.s.objs[tr].of |= ofToken
	sc.enterAs(tr, 0)
	unit := sc.hand(id["Valkyrie Aerial Unit"], 0)
	sc.do(t, AkCast, uint8(unit), 0xff)
	sc.s.Step()
	if sc.s.sp != 1 || sc.s.stack[0].obj != unit || sc.onBF(tr) {
		t.Fatalf("stack %d, Treasure on the battlefield %v; want the spell cast and the Treasure gone", sc.s.sp, sc.onBF(tr))
	}
	if pool := sc.s.pl[0].pool; pool != ([kop.NumManaKinds]uint8{}) {
		t.Fatalf("pool %v left after the payment", pool)
	}
}

// TestMillPastTheParkedQueueStalls: under two Rests in Peace a mill parks each card
// for its order ask. One past the queue stalls by name, and no card is dropped.
func TestMillPastTheParkedQueueStalls(t *testing.T) {
	deck := []cardset.Entry{{Name: "Rest in Peace", Count: 2}, {Name: "Swamp", Count: 8}}
	tbl, da, _, err := cardsettest.Synth(t, "std-big").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true})
	sc.perm(id["Rest in Peace"], 0)
	sc.perm(id["Rest in Peace"], 1)
	s, k := sc.s, sc.k
	pl := &s.pl[0]
	for int(pl.libHi-pl.libLo) <= maxParked {
		o := sc.obj(id["Swamp"], 0)
		s.objs[o].zone = kop.ZoneLibrary
		pl.libBottom(o)
	}
	lib := int(pl.libHi - pl.libLo)
	if !s.parkMills(k, 0, lib, 0) {
		t.Fatal("the mill's first card has no order ask")
	}
	if s.g.stalled != StallReplCands {
		t.Fatalf("stall %q, want replcands", StallName(s.g.stalled))
	}
	n := 0
	for i := range s.cold.parked {
		if r := &s.cold.parked[i]; r.kind == parkMill {
			if r.obj != pl.lib[int(pl.libLo)+n] {
				t.Fatalf("parked %d is object %d, want the library's next", n, r.obj)
			}
			n++
		}
	}
	if n != maxParked || int(pl.libHi-pl.libLo) != lib {
		t.Fatalf("%d parked, library %d; want %d and %d untouched", n, pl.libHi-pl.libLo, maxParked, lib)
	}
}
