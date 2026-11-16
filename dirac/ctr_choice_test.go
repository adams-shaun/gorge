package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// pccKinds is the printed list of the fixture's choice (gorge asks them in
// list order, effects counters_put.go's counterKinds loop). It is deliberately
// not in kernel kind order: the kernel's decision options sit in canonical
// order, kernel kind order, which the oracle's counterKindAction compares by
// kind.
var pccKinds = []kop.CounterKind{kop.CtrMenace, kop.CtrFlying, kop.CtrP1P1}

type pccFix struct {
	sc          *scen
	host, enter CardID
}

// newPccFix is a table with an activated enchantment "put your choice of a
// menace, flying or +1/+1 counter on target enchantment you control" and a
// creature that enters with the same choice.
func newPccFix(t *testing.T) *pccFix {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, _, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	var pl CardID = 255
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Plains" {
			pl = CardID(i)
		}
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TEnchantment, Ctrl: kop.CtrlYou})
	mine := kop.FilterID(len(tbl.Filters) - 1)
	b, c := kop.PutCtrChoiceOperands(pccKinds)
	choice := func(obj kop.Ref) kop.Instr {
		return kop.Instr{Op: kop.OpExt, A: kop.Const(int(kop.ExtPutCounterChoice)), Obj: obj, B: b, C: c}
	}
	f := &pccFix{host: CardID(len(tbl.Cards)), enter: CardID(len(tbl.Cards) + 1)}
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Choice", Types: kop.TEnchantment, Abilities: []kop.Ability{{Kind: kop.AbActivated,
			Targets: [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject, Filter: mine}}, NTargets: 1,
			Prog: []kop.Instr{choice(kop.RefTarget0)}}}},
		kop.CardDef{Name: "Test Enter", Types: kop.TCreature, Abilities: []kop.Ability{
			{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplEntersProgram}, Prog: []kop.Instr{choice(kop.RefReplaced)}}}},
	)
	f.sc = newScenTableLib(t, tbl, pl, Config{NoElide: true})
	f.sc.s.g.on = true
	return f
}

// modes is the kinds a decision offers, in option order.
func pccModes(t *testing.T, d Decision) []uint8 {
	t.Helper()
	var modes []uint8
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() != AkChoose {
			t.Fatalf("option %d is kind %d, want a plain pick", i, d.Options[i].Kind())
		}
		modes = append(modes, d.Options[i].Mode())
	}
	return modes
}

// TestPutCounterChoicePosesGorgeOptions: the controller is asked one counter
// kind pick over the listed kinds in printed order (gorge's Min=Max=1
// "counter_kind" KChoose, always asked), and one counter of the picked kind
// is put: on the target when the ability resolves, on the permanent as it
// enters.
func TestPutCounterChoicePosesGorgeOptions(t *testing.T) {
	var printed []uint8
	for _, k := range pccKinds {
		printed = append(printed, uint8(k))
	}
	want := slices.Sorted(slices.Values(printed))
	if slices.Equal(want, printed) {
		t.Fatal("the fixture list is already in kernel kind order: the test cannot tell the offered set from the list")
	}
	count := func(sc *scen, o Obj, k kop.CounterKind) int { return sc.s.perms[sc.slot(o)].ctrN(sc.s, int(k)) }
	// the pick, made on a fresh fixture each time: kind is the one chosen, at
	// its index in the decision's (kind-ordered) options
	for _, kind := range pccKinds {
		pick := slices.Index(want, uint8(kind))
		t.Run("resolving", func(t *testing.T) {
			f := newPccFix(t)
			sc := f.sc
			host, tgt := sc.perm(f.host, 0), sc.perm(f.host, 0)
			sc.do(t, AkActivate, TCode(host), scAny)
			sc.do(t, AkTarget, scAny, TCode(tgt))
			sc.do(t, AkPass, scAny, scAny)
			sc.do(t, AkPass, scAny, scAny)
			d := sc.s.Step()
			if d.Kind != DkChoose || d.Sub != ChRemCtrKind || d.Player != 0 {
				t.Fatalf("decision kind %d sub %d player %d, want player 0's counter-kind pick", d.Kind, d.Sub, d.Player)
			}
			if got := pccModes(t, d); !slices.Equal(got, want) {
				t.Fatalf("kinds offered %v, want the listed kinds in canonical order %v", got, want)
			}
			sc.s.Apply(d.Options[pick])
			for _, k := range pccKinds {
				n := 0
				if k == kind {
					n = 1
				}
				if got := count(sc, tgt, k); got != n {
					t.Fatalf("picked kind %d: kind %d has %d counters, want %d", kind, k, got, n)
				}
			}
		})
		t.Run("enters", func(t *testing.T) {
			f := newPccFix(t)
			sc := f.sc
			o := sc.perm(f.enter, 0)
			d := sc.s.Step()
			if d.Kind != DkChoose || d.Sub != ChRemCtrKind || d.Player != 0 {
				t.Fatalf("decision kind %d sub %d player %d, want player 0's counter-kind pick", d.Kind, d.Sub, d.Player)
			}
			if got := pccModes(t, d); !slices.Equal(got, want) {
				t.Fatalf("kinds offered %v, want the listed kinds in canonical order %v", got, want)
			}
			if n := count(sc, o, kind); n != 0 {
				t.Fatalf("%d counters before the pick, want none", n)
			}
			sc.s.Apply(d.Options[pick])
			for _, k := range pccKinds {
				n := 0
				if k == kind {
					n = 1
				}
				if got := count(sc, o, k); got != n {
					t.Fatalf("picked kind %d: kind %d has %d counters, want %d", kind, k, got, n)
				}
			}
		})
	}
}
