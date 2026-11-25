package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// hurkylRaiseSrc is Hurkyl's Final Meditation's raise line: Affected$ Card.Self
// with no ValidCard$ (the corpus's one cost static spelled so).
const hurkylRaiseSrc = "Name:Hurkyl Probe\nManaCost:1 U\nTypes:Instant\n" +
	"S:Mode$ RaiseCost | Condition$ NotPlayerTurn | Affected$ Card.Self | Type$ Spell | EffectZone$ All | Amount$ 3\n" +
	"A:SP$ Draw | NumCards$ 1\nOracle:x\n"

// TestHurkylSelfRaiseChargesOnlyItself pins that a RaiseCost spelled
// Affected$ Card.Self raises its own card, on the opponent's turn only, and
// no other spell in hand: the cost walks read ValidCard$, so the script's
// Affected$ is respelled at load (cards.Static.respellCostAffectedSelf).
func TestHurkylSelfRaiseChargesOnlyItself(t *testing.T) {
	t.Parallel()
	raises := func(active state.PlayerID) (self, bear int32) {
		e, _, h := newFixtureDeck(t, 91, hurkylRaiseSrc, testBearSrc)
		e.G.Active = active
		for _, r := range e.costModifiers(0, h, spellScope("")).Raises {
			self += r
		}
		found := false
		for _, id := range e.G.Zone(state.ZHand, 0) {
			if o := e.G.Obj(id); id != h && o.Face() != nil && o.Face().Name == "Bear" {
				found = true
				for _, r := range e.costModifiers(0, id, spellScope("")).Raises {
					bear += r
				}
			}
		}
		if !found {
			t.Fatal("no Bear in seat 0's hand to price beside the Hurkyl")
		}
		return self, bear
	}
	if self, bear := raises(0); self != 0 || bear != 0 {
		t.Errorf("on its controller's turn: Hurkyl raise %d, Bear raise %d, want 0 and 0", self, bear)
	}
	if self, bear := raises(1); self != 3 || bear != 0 {
		t.Errorf("on the opponent's turn: Hurkyl raise %d, Bear raise %d, want 3 and 0", self, bear)
	}
}
