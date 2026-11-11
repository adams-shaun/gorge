package dirac

import (
	"math/bits"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCondSplitLiveGate: a Condition-split mana source (mana_condsplit.go) of
// each gate kind -- a counter on the source (Gemstone Caverns' luck), the
// controller's lands played (River of Tears') -- is a planner-deferred prefix
// gap whose census unit and activation answers follow the live gate. Gate off:
// the root's colour alone, one answer. Gate on: the census unit is the root's
// colour beside the arm's (gorge's condSplitLiveSub fold), the activation the
// arm's alone (the root is gated EQ0): a lucky Gemstone asks one of WUBRG and
// never makes {C}, River after a land drop makes {B} with no ask. The gate is
// posed in kernel state (a counter put on the source, landsPlayed bumped).
func TestCondSplitLiveGate(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, _, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	n := CardID(len(tbl.Cards))
	gem, river := n, n+1
	split := func(name string, root, arm kop.ColorMask, gate uint8, ctr kop.CounterKind) kop.CardDef {
		return kop.CardDef{Name: name, Types: kop.TLand, Abilities: []kop.Ability{{Kind: kop.AbMana,
			Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: root, ManaN: 1, SplitGate: gate, SplitCtr: ctr, SplitCols: arm}}}
	}
	tbl.Cards = append(tbl.Cards,
		split("Test Gemstone", kop.ColC, kop.ColAny, kop.SplitGateCounter, kop.CtrLuck),
		split("Test River", kop.ColU, kop.ColB, kop.SplitGateLands, 0))
	for _, c := range []struct {
		name         string
		card         CardID
		root, arm    uint8
		pose         func(sc *scen, o Obj)
		unpose       func(sc *scen, o Obj)
		wantAnswers  uint8
		firstOnColor int // the colour an answer-0 activation floats
	}{
		{"counter gate", gem, 1 << kop.C, uint8(kop.ColAny),
			func(sc *scen, o Obj) { sc.s.perms[sc.slot(o)].addCtrN(sc.s, int(kop.CtrLuck), 1) },
			func(sc *scen, o Obj) { sc.s.perms[sc.slot(o)].addCtrN(sc.s, int(kop.CtrLuck), -1) }, 5, kop.W},
		{"lands-played gate", river, 1 << kop.U, 1 << kop.B,
			func(sc *scen, o Obj) { sc.s.pl[0].landsPlayed = 1 },
			func(sc *scen, o Obj) { sc.s.pl[0].landsPlayed = 0 }, 1, kop.B},
	} {
		t.Run(c.name, func(t *testing.T) {
			sc := newScenTable(t, tbl, Config{NoElide: true})
			if !sc.k.gen.anyCondSplit {
				t.Fatal("precondition: the table has no Condition-split source")
			}
			o := sc.perm(c.card, 0)
			sl := sc.slot(o)
			if !sc.s.planDeferred(sc.k, 0, sl) || !sc.s.scriptTapsDefer(sc.k, 0).has(sl) {
				t.Fatal("precondition: the source is not a planner-deferred prefix gap")
			}
			unit := func() uint8 {
				var a avail
				sc.s.addTapsDeferUnits(sc.k, sl, &a)
				if a.n != 1 || a.cnt[0] != 1 {
					t.Fatalf("census unit %+v: want one unit", a)
				}
				return a.mask[0]
			}
			act := func() uint8 { return uint8(sc.s.manaPCGen(sc.k, sl).manaCol) }
			check := func(state string, wantUnit, wantAct uint8) {
				t.Helper()
				if got := unit(); got != wantUnit {
					t.Errorf("%s: census unit mask %#x, want %#x", state, got, wantUnit)
				}
				if got := act(); got != wantAct {
					t.Errorf("%s: activation mask %#x, want %#x", state, got, wantAct)
				}
				if got := sc.s.deferGapAnswers(sc.k, sl); got != uint8(bits.OnesCount8(wantAct)) {
					t.Errorf("%s: %d answers, want %d", state, got, bits.OnesCount8(wantAct))
				}
			}
			if c.root == c.arm || c.root|c.arm == c.root {
				t.Fatal("precondition: the gate's arm adds nothing to the root's production")
			}
			check("gate off", c.root, c.root)
			c.pose(sc, o)
			if !sc.s.splitGateOn(sc.k, sl, &sc.k.t.Cards[c.card].Abilities[0]) {
				t.Fatal("precondition: the posed gate does not hold")
			}
			// the root's colour stays in the census unit beside the arm's (rules
			// condSplitLiveSub counts the root always), the activation the arm's only
			check("gate on", c.root|c.arm, c.arm)
			if got := sc.s.deferGapAnswers(sc.k, sl); got != c.wantAnswers {
				t.Errorf("gate on: %d answers, want %d", got, c.wantAnswers)
			}
			// answer 0 floats the first colour of the activation mask, plain pool mana
			sc.s.tapDeferFloat(sc.k, 0, sl, 0)
			if sc.s.pl[0].pool[c.firstOnColor] != 1 {
				t.Errorf("answer 0 floated pool %v, want one mana of colour %d", sc.s.pl[0].pool, c.firstOnColor)
			}
			sc.s.pl[0].pool = [kop.NumManaKinds]uint8{}
			c.unpose(sc, o)
			check("gate off again", c.root, c.root)
		})
	}
	// River's gate reads gorge's LandsPlayed, reset only when the seat's own turn
	// begins: on the opponent's turn it still holds the seat's last own turn
	t.Run("lands-played gate on the opponent's turn", func(t *testing.T) {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		o := sc.perm(river, 0)
		sl := sc.slot(o)
		ab := &sc.k.t.Cards[river].Abilities[0]
		sc.s.pl[0].landsPlayed = 1
		sc.s.beginTurn(1)
		if sc.s.active != 1 || sc.s.pl[0].landsPlayed != 1 || sc.s.landsProj(0) != 0 {
			t.Fatalf("precondition: active %d, seat 0 lands played %d (shown %d) after the turn change", sc.s.active, sc.s.pl[0].landsPlayed, sc.s.landsProj(0))
		}
		if !sc.s.splitGateOn(sc.k, sl, ab) {
			t.Error("the gate is off on the opponent's turn: want gorge's stale LandsPlayed of the seat's last own turn")
		}
		sc.s.beginTurn(0) // the seat's own turn begins: reset, and it has played no land yet
		if sc.s.splitGateOn(sc.k, sl, ab) {
			t.Error("the gate holds at the start of the seat's own turn with no land played")
		}
	})
}
