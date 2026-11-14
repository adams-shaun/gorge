package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// pceCards: the saga pool with a free "choose a counter on target
// enchantment you control, put another of that kind on it" enchantment.
type pceCards struct{ host, saga CardID }

func pceScen(t *testing.T) (*scen, pceCards) {
	t.Helper()
	deck := []cardset.Entry{{Name: "History of Benalia", Count: 1}, {Name: "Plains", Count: 1}}
	set := cardsettest.Set(t, "saga")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	var c pceCards
	for _, id := range da {
		if tbl.Cards[id].Name == "History of Benalia" {
			c.saga = id
		}
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TEnchantment, Ctrl: kop.CtrlYou})
	mine := kop.FilterID(len(tbl.Filters) - 1)
	c.host = CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Existing", Types: kop.TEnchantment, Abilities: []kop.Ability{{Kind: kop.AbActivated,
		Targets: [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject, Filter: mine}}, NTargets: 1,
		Prog: []kop.Instr{{Op: kop.OpExt, A: kop.Const(int(kop.ExtPutCounterExisting)), Obj: kop.RefTarget0, B: kop.Const(1)}}}}})
	return newScenTable(t, tbl, Config{NoElide: true}), c
}

// pceActivate activates the host's ability on target and returns the next
// decision once the ability has resolved or asked.
func (sc *scen) pceActivate(t *testing.T, host, target Obj) Decision {
	t.Helper()
	sc.do(t, AkActivate, TCode(host), scAny)
	sc.do(t, AkTarget, scAny, TCode(target))
	sc.do(t, AkPass, scAny, scAny)
	sc.do(t, AkPass, scAny, scAny)
	return sc.s.Step()
}

func (sc *scen) pceCount(o Obj, kind kop.CounterKind) int {
	return sc.s.perms[sc.slot(o)].ctrN(sc.s, int(kind))
}

// TestPutCounterExisting: the controller picks among the kinds on the
// target (kernel kind order), one kind or none asks nothing, a Saga's lore
// counter fires its chapter and a target gone at resolution does nothing.
func TestPutCounterExisting(t *testing.T) {
	t.Run("none", func(t *testing.T) {
		sc, c := pceScen(t)
		host, tgt := sc.perm(c.host, 0), sc.perm(c.host, 0)
		before := PutExistingStats.Asked.Load()
		if d := sc.pceActivate(t, host, tgt); d.Sub == ChRemCtrKind {
			t.Fatal("asked with no counters")
		}
		for k := kop.CounterKind(0); k < kop.NumCounterKinds3; k++ {
			if kop.IsCounterKind(int(k)) && sc.pceCount(tgt, k) != 0 {
				t.Fatalf("kind %d has %d counters, want none", k, sc.pceCount(tgt, k))
			}
		}
		if PutExistingStats.Asked.Load() != before {
			t.Fatal("the ask counter rose")
		}
	})
	t.Run("one", func(t *testing.T) {
		sc, c := pceScen(t)
		host, tgt := sc.perm(c.host, 0), sc.perm(c.host, 0)
		sc.s.perms[sc.slot(tgt)].addCtrN(sc.s, int(kop.CtrP1P1), 1)
		if d := sc.pceActivate(t, host, tgt); d.Sub == ChRemCtrKind {
			t.Fatal("asked with one kind")
		}
		if n := sc.pceCount(tgt, kop.CtrP1P1); n != 2 {
			t.Fatalf("%d +1/+1 counters, want 2", n)
		}
	})
	t.Run("two", func(t *testing.T) {
		for pick, want := range []kop.CounterKind{kop.CtrP1P1, kop.CtrQuest} {
			sc, c := pceScen(t)
			host, tgt := sc.perm(c.host, 0), sc.perm(c.host, 0)
			sc.s.perms[sc.slot(tgt)].addCtrN(sc.s, int(kop.CtrQuest), 1)
			sc.s.perms[sc.slot(tgt)].addCtrN(sc.s, int(kop.CtrP1P1), 1)
			before := PutExistingStats.Asked.Load()
			d := sc.pceActivate(t, host, tgt)
			if d.Kind != DkChoose || d.Sub != ChRemCtrKind || d.Player != 0 {
				t.Fatalf("decision kind %d sub %d player %d, want player 0's counter-kind pick", d.Kind, d.Sub, d.Player)
			}
			var modes []uint8
			for i := 0; i < int(d.N); i++ {
				if d.Options[i].Kind() != AkChoose {
					t.Fatalf("option %d is kind %d, want a plain pick (no Done)", i, d.Options[i].Kind())
				}
				modes = append(modes, d.Options[i].Mode())
			}
			if !slices.Equal(modes, []uint8{uint8(kop.CtrP1P1), uint8(kop.CtrQuest)}) {
				t.Fatalf("kinds offered %v, want +1/+1 then quest", modes)
			}
			if PutExistingStats.Asked.Load() != before+1 {
				t.Fatal("the ask counter did not rise by one")
			}
			sc.s.Apply(d.Options[pick])
			other := kop.CtrQuest
			if want == kop.CtrQuest {
				other = kop.CtrP1P1
			}
			if sc.pceCount(tgt, want) != 2 || sc.pceCount(tgt, other) != 1 {
				t.Fatalf("picked kind %d has %d, other %d, want 2 and 1", want, sc.pceCount(tgt, want), sc.pceCount(tgt, other))
			}
		}
	})
	t.Run("saga lore", func(t *testing.T) {
		sc, c := pceScen(t)
		host, saga := sc.perm(c.host, 0), sc.perm(c.saga, 0)
		sc.s.npend = 0
		if n := sc.pceCount(saga, kop.CtrLore); n != 1 {
			t.Fatalf("the Saga entered with %d lore counters, want 1", n)
		}
		sc.do(t, AkActivate, TCode(host), scAny)
		sc.do(t, AkTarget, scAny, TCode(saga))
		sc.do(t, AkPass, scAny, scAny)
		sc.do(t, AkPass, scAny, scAny)
		if n := sc.pceCount(saga, kop.CtrLore); n != 2 {
			t.Fatalf("%d lore counters, want 2", n)
		}
		if sc.s.npend == 0 && sc.s.sp == 0 {
			t.Fatal("chapter II did not trigger")
		}
	})
	t.Run("target gone", func(t *testing.T) {
		sc, c := pceScen(t)
		host, tgt := sc.perm(c.host, 0), sc.perm(c.host, 0)
		sc.s.perms[sc.slot(tgt)].addCtrN(sc.s, int(kop.CtrQuest), 1)
		sc.s.perms[sc.slot(tgt)].addCtrN(sc.s, int(kop.CtrP1P1), 1)
		sc.do(t, AkActivate, TCode(host), scAny)
		sc.do(t, AkTarget, scAny, TCode(tgt))
		sc.s.leave(sc.k, sc.slot(tgt), kop.ZoneGraveyard, false) // gone before the ability resolves
		sc.do(t, AkPass, scAny, scAny)
		sc.do(t, AkPass, scAny, scAny)
		if d := sc.s.Step(); d.Sub == ChRemCtrKind {
			t.Fatal("asked with the target gone")
		}
	})
}
