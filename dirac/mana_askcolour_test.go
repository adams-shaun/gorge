package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// askColScen: p0 controls two hand-built colour-asking mana sources (a
// ManaAskColour ability: {T}: choose a colour, add that much of it as p0's
// devotion to it) beside a black-pip permanent (devotion to black 3, to white
// 0), and holds a {X}{X} creature the announced-X window pays (its CR 601.2g
// window walks gorge's manual window, which takes a deferred source in its
// battlefield place).
func askColScen(t *testing.T) (sc *scen, srcs [2]Obj, ballista Obj) {
	t.Helper()
	tbl := v.NewTable()
	tbl.Exprs = append([]kop.Expr(nil), tbl.Exprs...)
	tbl.Exprs = append(tbl.Exprs, kop.Expr{Kind: kop.ExDevotionChosen})
	dev := kop.ValueRef{K: kop.VExpr, N: int16(len(tbl.Exprs) - 1)}
	add := func(cd kop.CardDef) CardID {
		tbl.Cards = append(tbl.Cards, cd)
		return CardID(len(tbl.Cards) - 1)
	}
	chooser := add(kop.CardDef{Name: "Test Chooser", Types: kop.TArtifact,
		Abilities: []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap},
			ManaColors: kop.ColAny, ManaN: 1, ManaV: dev, ManaAskColour: true}}})
	pips := add(kop.CardDef{Name: "Test Black Pips", Types: kop.TCreature, Power: 1, Toughness: 1,
		Cost: kop.ManaCost{Pips: [kop.NumManaKinds]uint8{2: 3}}})
	ball := add(kop.CardDef{Name: "Test Ballista", Types: kop.TCreature, Cost: kop.ManaCost{X: 2}, Power: -1, Toughness: -1})
	sc = newScenTable(t, tbl, Config{NoElide: true, GorgeAutopaySurface: true})
	srcs[0], srcs[1] = sc.perm(chooser, 0), sc.perm(chooser, 0)
	sc.perm(pips, 0)
	return sc, srcs, sc.hand(ball, 0)
}

// TestManaActivationAsksColourAndRecords: a source whose mana ability asks a
// colour at activation is tapped by the cast window (the fee paid before the
// ask), poses the ask over W U B R G, records the answer on the source, adds
// devotion-of-chosen mana of it in one unit, and clears the record, so the
// second source (and any later activation) asks again; the census prices no
// unit of it and autopay never taps it.
func TestManaActivationAsksColourAndRecords(t *testing.T) {
	sc, srcs, ball := askColScen(t)
	for _, o := range srcs {
		sl := sc.slot(o)
		if !sc.s.planDeferred(sc.k, 0, sl) {
			t.Fatal("precondition: the colour-asking source is not planner-deferred")
		}
		if sc.s.perms[sl].chosen != 255 {
			t.Fatalf("precondition: the source already records %d", sc.s.perms[sl].chosen)
		}
	}
	sc.do(t, AkCast, TCode(ball), scAny)
	asks := 0
	for i := 0; i < 50; i++ {
		d := sc.s.Step()
		if d.Kind == DkPriority {
			break
		}
		if d.Kind != DkChoose {
			t.Fatalf("unexpected decision kind %d sub %d opts %v", d.Kind, d.Sub, d.Options[:d.N])
		}
		switch d.Sub {
		case ChX:
			sc.s.Apply(MakeAction(AkChoose, 0, 0, 3)) // X = 3: {6}, two sources of three black
		case ChGenManaChoice:
			if d.N != 5 {
				t.Fatalf("the colour ask lists %d colours, want W U B R G: %v", d.N, d.Options[:d.N])
			}
			for j := 0; j < 5; j++ {
				if d.Options[j].Mode() != uint8(j) {
					t.Fatalf("option %d is colour %d, want WUBRG order", j, d.Options[j].Mode())
				}
			}
			var want [kop.NumManaKinds]uint8
			if asks == 1 {
				want[kop.B] = 3 // the first answer's mana is already floating
			}
			if got := sc.s.pl[0].pool; got != want {
				t.Fatalf("ask %d: pool %v, want %v", asks, got, want)
			}
			if got := sc.tapped(srcs[asks]); !got {
				t.Fatalf("ask %d: the source's {T} is not paid before the ask", asks)
			}
			asks++
			sc.s.Apply(MakeAction(AkChoose, 0, 0, uint8(kop.B)))
			for _, o := range srcs {
				if c := sc.s.perms[sc.slot(o)].chosen; c != 255 {
					t.Fatalf("after answer %d a source still records colour %d", asks, c)
				}
			}
		default:
			t.Fatalf("unexpected choose sub %d opts %v", d.Sub, d.Options[:d.N])
		}
	}
	if asks != 2 {
		t.Fatalf("%d colour asks, want one per activation (2)", asks)
	}
	if sc.s.ctl != ctlPriority {
		t.Fatalf("the cast never came back to priority (ctl %d)", sc.s.ctl)
	}
}
