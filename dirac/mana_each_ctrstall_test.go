package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// eachStallBoard builds the tapReflectScen board with the Birds of Paradise
// rewritten to an all-at-once "{T}: Add {W} and {U}" (an each-colour gap)
// and, when rider, carrying the counter rider of the tap (Twitching Doll's
// shape: the rider puts a +1/+1 counter on this as the mana resolves,
// kop.OpPutCounter over kop.RefSelf) beside Branching Evolution's and Hardened
// Scales' replacements over the controller's permanents — two of mixed kinds,
// so the placement's order matters. It returns the scenario, the cast card in
// hand, and the Birds' slot.
func eachStallBoard(t *testing.T, rider bool, w, u uint8) (*scen, Obj, Slot) {
	t.Helper()
	var ember, branching, scales CardID
	tbl, id := tapsDeferShapeTable(t, func(tbl *kop.Table, id map[string]CardID) {
		if rider {
			for _, rk := range []struct {
				kind kop.ReplKind
				a    int16
			}{{kop.ReplDoubleCountersOn, 0}, {kop.ReplAddCountersOn, 1}} {
				r := kop.Ability{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: rk.kind, C: int16(kop.CtrP1P1), A: rk.a,
					Filter: kop.FilterID(len(tbl.Filters))}}
				tbl.Filters = append(append([]kop.Filter(nil), tbl.Filters...), kop.Filter{Ctrl: kop.CtrlYou})
				n, c := "Test Branching", &branching
				if rk.a != 0 {
					n, c = "Test Scales", &scales
				}
				*c = CardID(len(tbl.Cards))
				tbl.Cards = append(tbl.Cards, kop.CardDef{Name: n, Types: kop.TEnchantment, Abilities: []kop.Ability{r}})
			}
			rewriteMana(tbl, id["Birds of Paradise"], func(a *kop.Ability) {
				a.ManaColors, a.ManaEach, a.ManaN = kop.ColW|kop.ColU, true, 1
				a.AFlags |= kop.AfManaEachColor // the each-colour gap's flag (gfEachCol)
				a.Prog = []kop.Instr{{Op: kop.OpPutCounter, Obj: kop.RefSelf, A: kop.Const(1), C: int16(kop.CtrP1P1)}}
			})
		} else {
			rewriteMana(tbl, id["Birds of Paradise"], func(a *kop.Ability) {
				a.ManaColors, a.ManaEach, a.ManaN = kop.ColW|kop.ColU, true, 1
				a.AFlags |= kop.AfManaEachColor // the each-colour gap's flag (gfEachCol)
			})
		}
		ember = CardID(len(tbl.Cards))
		emberCost := kop.ManaCost{}
		emberCost.Pips[kop.W] = w
		emberCost.Pips[kop.U] = u
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Ember", Types: kop.TInstant, Cost: emberCost})
	})
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	if rider {
		sc.perm(branching, 0) // the two counter replacements, the controller's
		sc.perm(scales, 0)
	}
	b := sc.perm(id["Birds of Paradise"], 0)
	emberO := sc.hand(ember, 0)
	return sc, emberO, sc.slot(b)
}

// TestEachScriptStallPropagates: the each search's only payment line is the
// Birds of Paradise rewritten all-at-once ({T}: Add {W} and {U}). With the
// counter rider of the tap on it, the line's tap resolves the rider inside the
// activation, where Branching Evolution's and Hardened Scales' replacements —
// two of mixed kinds — both match the placement: gorge would pose its CR 616.1
// order ask, which the kernel cannot pose mid-payment, so the line's fork
// latches StallCtrOrder (manaRider). The cast ({W}{W}{U}{U}) is past every
// line, so the search's fork latched, every line failed and eachScript
// declares the stall on the real state instead of dropping the play as "no
// payment": the game draws as the unmodelled stall instead of silently losing
// the cast gorge offers (TestFiltScriptStallPropagates's shape). The control:
// without the rider the tap prices clean and the same unpayable cast is
// refused with no stall (a plain refusal declares nothing).
func TestEachScriptStallPropagates(t *testing.T) {
	for _, arm := range []struct {
		rider bool
		stall uint8
	}{{true, StallCtrOrder}, {false, 0}} {
		sc, emberO, birds := eachStallBoard(t, arm.rider, 2, 2)
		s, k := sc.s, sc.k
		a := MakeAction(AkCast, TCode(emberO), 0, 0)
		if s.Stalled() {
			t.Fatalf("precondition: the board alone latched %s", StallName(s.StallReason()))
		}
		if !s.scriptGaps(k, 0).has(birds) {
			t.Fatalf("precondition: the rewritten Birds are no each-colour gap (rider %v)", arm.rider)
		}
		if s.eachScript(k, a, 0, false) {
			t.Fatalf("the search paid the cast, want its only line to stall (rider %v)", arm.rider)
		}
		if (s.Stalled() && s.StallReason() != arm.stall) || (!s.Stalled() && arm.stall != 0) {
			t.Fatalf("rider %v: stalled %v reason %s, want %s",
				arm.rider, s.Stalled(), StallName(s.StallReason()), StallName(arm.stall))
		}
	}
}
