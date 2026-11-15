package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Bargain trio's StBargained discount (cost_reducespell.go) at the
// offer and at the charge, per card: the pool-pays surface the lockstep
// shares with TestBargainLockstep cannot float {U}{U} or {3}{G}{G}, so Hamlet
// Glutton and Ice Out are driven here with an exact floating pool. The pool
// pays the bargained price and not the plain one: the plain cast is not
// offered, the bargained cast is, it charges the discounted price (the pool
// empties), and one mana short offers neither.

func krsScene(t *testing.T, name string) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: name, Count: 1}, {Name: "Ornithopter", Count: 1}, {Name: "Forest", Count: 1}}
	set := cardsettest.Set(t, "reducespell")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatalf("%s: matchup: %v", name, err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if !tbl.Cards[id[name]].Bargain {
		t.Fatalf("%s: CardDef.Bargain is false", name)
	}
	return newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true, GorgeAutopaySurface: true}), id
}

// krsCast casts o bargained (the election takes cand) and returns the stack
// entry once it sits on top of the stack at a priority decision.
func krsCast(t *testing.T, sc *scen, o Obj, card CardID, cand Obj) stackEntry {
	t.Helper()
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCastCasualty && a.Source() == TCode(uint8(o)) {
			sc.s.Apply(a)
			break
		}
	}
	for step := 0; step < 80; step++ {
		d = sc.s.Step()
		if sc.s.sp > 0 && d.Kind == DkPriority && sc.s.stack[sc.s.sp-1].kind == seSpell && sc.s.stack[sc.s.sp-1].card == card {
			return sc.s.stack[sc.s.sp-1]
		}
		if d.N == 0 {
			continue
		}
		pick := d.Options[0]
		if d.Kind == DkChoose && d.Sub == ChGenBargain {
			for i := 0; i < int(d.N); i++ {
				if Obj(d.Options[i].Source()) == cand {
					pick = d.Options[i]
				}
			}
		}
		sc.s.Apply(pick)
	}
	t.Fatalf("card %d: the bargained cast did not reach the stack", card)
	return stackEntry{}
}

// krsTarget puts an opposing spell on the stack for a counterspell to target.
func krsTarget(sc *scen, id map[string]CardID) {
	sp := sc.obj(id["Ornithopter"], 1)
	sc.s.objs[sp].zone = kop.ZoneStack
	sc.s.push(stackEntry{kind: seSpell, obj: sp, ctrl: 1, card: id["Ornithopter"], trigObj: NoObj})
}

func TestReducespellBargainPrice(t *testing.T) {
	// pool order: W U B R G C
	for _, tc := range []struct {
		name       string
		exact, bad [kop.NumManaKinds]uint8
		plain, bgd uint8 // generic part, plain and bargained
	}{
		{"Hamlet Glutton", [6]uint8{3, 0, 0, 0, 2}, [6]uint8{2, 0, 0, 0, 2}, 5, 3},
		{"Ice Out", [6]uint8{0, 2}, [6]uint8{0, 1}, 1, 0},
		{"Johann's Stopgap", [6]uint8{1, 1}, [6]uint8{0, 1}, 3, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc, id := krsScene(t, tc.name)
			card := id[tc.name]
			k := sc.k
			if !k.bargainRed {
				t.Fatal("prep.bargainRed unset for a StBargained table")
			}
			o := sc.hand(card, 0)
			cand := sc.perm(id["Ornithopter"], 0)
			if tc.name == "Ice Out" {
				krsTarget(sc, id)
			}
			base := k.cards[card].cost
			if got := sc.s.adjustCostGen(k, 0, card, o, base); got.Generic != tc.plain {
				t.Errorf("plain price generic %d, want %d", got.Generic, tc.plain)
			}
			if got := sc.s.bargainedPrice(k, 0, card, o, base); got.Generic != tc.bgd {
				t.Errorf("bargained price generic %d, want %d", got.Generic, tc.bgd)
			}

			sc.s.pl[0].pool = tc.bad
			if acts := sc.castOptions(o); len(acts) != 0 {
				t.Fatalf("one mana short: %d cast options, want none", len(acts))
			}
			sc.s.pl[0].pool = tc.exact
			acts := sc.castOptions(o)
			if len(acts) != 1 || acts[0].Kind() != AkCastCasualty {
				t.Fatalf("the discounted pool: cast options %v, want only the bargained cast", acts)
			}
			e := krsCast(t, sc, o, card, cand)
			if e.flags&sfKicked == 0 {
				t.Error("the bargained cast left no provenance flag")
			}
			if sc.s.pl[0].pool != [kop.NumManaKinds]uint8{} {
				t.Errorf("pool after the bargained cast %v, want it spent on the discounted price", sc.s.pl[0].pool)
			}

			DebugReduceSpellFlip = true
			defer func() { DebugReduceSpellFlip = false }()
			sc2, id2 := krsScene(t, tc.name)
			o2 := sc2.hand(id2[tc.name], 0)
			sc2.perm(id2["Ornithopter"], 0)
			if tc.name == "Ice Out" {
				krsTarget(sc2, id2)
			}
			sc2.s.pl[0].pool = tc.exact
			if acts := sc2.castOptions(o2); len(acts) != 0 {
				t.Errorf("mutation on: %d cast options, want none (no discount)", len(acts))
			}
		})
	}
}

// TestPricedSpellSkipsItselfInItsZone: CR 601.2a has the spell on the stack by the
// time CR 601.2f determines its cost, so a spell cast from a graveyard is not one
// of the cards there its own reduction counts, as a count or as a card type.
func TestPricedSpellSkipsItselfInItsZone(t *testing.T) {
	for _, kind := range []kop.ExprKind{kop.ExCountGraveyard, kop.ExGraveTypes} {
		deck := []cardset.Entry{{Name: "Driftgloom Coyote", Count: 2}, {Name: "Plains", Count: 8}}
		sc, id := newBLBScenTweak(t, deck, func(tb *kop.Table, id map[string]CardID) {
			tb.Exprs = append(tb.Exprs, kop.Expr{Kind: kind})
			cd := &tb.Cards[id["Driftgloom Coyote"]]
			cd.Abilities = append(append([]kop.Ability(nil), cd.Abilities...), kop.Ability{Kind: kop.AbStatic, Static: kop.Static{
				Kind: kop.StatReduceCost, Zone: kop.ZoneCommand, StFlags: kop.StDynamic, VA: kop.ValueRef{K: kop.VExpr, N: int16(len(tb.Exprs) - 1)}}})
		})
		c := id["Driftgloom Coyote"]
		spell := sc.grave(c, 0, 1)[0]
		sc.grave(id["Plains"], 0, 1)
		cost := sc.k.t.Cards[c].Cost
		if got, want := sc.s.adjustCost(sc.k, 0, c, spell, cost).Generic, cost.Generic-1; got != want {
			t.Errorf("expr %d, the spell priced: {%d}, want {%d}", kind, got, want)
		}
		if got, want := sc.s.adjustCostGen(sc.k, 0, c, spell, cost).Generic, cost.Generic-1; got != want {
			t.Errorf("expr %d, the spell priced with its object: {%d}, want {%d}", kind, got, want)
		}
		if got, want := sc.s.adjustCost(sc.k, 0, c, NoObj, cost).Generic, cost.Generic-2; got != want {
			t.Errorf("expr %d, nothing priced: {%d}, want {%d}", kind, got, want)
		}
	}
}
