package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
)

// Only a player, a planeswalker or a battle can be attacked (CR 506.3,
// 508.1b), judged by the object's CURRENT types: a planeswalker Imprisoned
// in the Moon ("is a colorless land ... loses all other card types", layer
// 4) is no longer one. gorge offered it by its printed face (dirac lockstep
// pairs 4159, 7216).
func TestImprisonedPlaneswalkerIsNotAnAttackTarget(t *testing.T) {
	t.Parallel()
	const prison = "Name:Moon Prison\nTypes:Enchantment\nS:Mode$ Continuous | Affected$ Planeswalker | AddType$ Land | RemoveCardTypes$ True\nOracle:x\n"
	for _, imprisoned := range []bool{false, true} {
		e := combatEngine(t)
		attacker := onBoardReady(t, e, 0, planeswalkerAttacker)
		pw := onBoard(t, e, 1, combatPlaneswalker)
		e.emit(events.Event{Kind: events.CounterChange, Obj: pw, Counter: "LOYALTY", Amount: 5})
		if imprisoned {
			onBoard(t, e, 1, prison)
			if e.hasType(e.G.Obj(pw), "Planeswalker") {
				t.Fatal("fixture: the imprisoned walker still has the Planeswalker type")
			}
		}
		e.askAttackers()
		d := e.Pending()
		if d == nil || d.Kind != decision.KAttackers {
			t.Fatalf("declare attackers decision = %+v", d)
		}
		offered := false
		for _, opt := range d.Options {
			if opt.Obj == attacker && opt.Battle == pw {
				offered = true
			}
		}
		if offered == imprisoned {
			t.Fatalf("imprisoned=%v: attack at the walker offered=%v", imprisoned, offered)
		}
		if got := e.canAttackBattle(pw, 0); got == imprisoned {
			t.Fatalf("imprisoned=%v: canAttackBattle=%v", imprisoned, got)
		}
	}
}
