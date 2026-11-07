package rules

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const targetReducerMaulerSrc = "Name:Test Mauler\nManaCost:R\nTypes:Creature Dwarf Warrior\nPT:2/1\n" +
	"S:Mode$ ReduceCost | ValidTarget$ Card.Self | ValidSpell$ Activated.Equip | Activator$ You | Amount$ 2\n" +
	"Oracle:Equip abilities you activate that target this creature cost {2} less to activate.\n"

const targetReducerMailSrc = "Name:Test Mail\nManaCost:1 R\nTypes:Artifact Equipment\nK:Equip:4\n" +
	"S:Mode$ Continuous | Affected$ Creature.EquippedBy | AddPower$ 1\nOracle:x\n"

const targetReducerBearSrc = "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

func targetReducerBoard(t *testing.T, attached bool) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	e, _, mail := newFixtureDeck(t, 71, targetReducerMailSrc, targetReducerBearSrc, targetReducerMaulerSrc)
	e.emit(events.Event{Kind: events.MoveZone, Obj: mail, From: state.ZHand, To: state.ZBattlefield})
	mauler := putCreature(t, e, 0, targetReducerMaulerSrc)
	if attached {
		bear := putCreature(t, e, 0, targetReducerBearSrc)
		attachFixture(t, e, mail, bear)
	}
	putLands(t, e, 0, 2)
	e.Advance()
	return e, mail, mauler
}

func TestEquipTargetReducerOfferedFromLands(t *testing.T) {
	for _, attached := range []bool{false, true} {
		e, mail, _ := targetReducerBoard(t, attached)
		if d := e.Pending(); d == nil {
			t.Fatal("no decision")
		}
		if _, ok := findAbilityOption(e, mail, 0); !ok {
			t.Errorf("attached=%v: equip {4} onto the {2}-less target not offered with two lands: %+v", attached, e.Pending().Options)
		}
	}
}

// TestEquipTargetReducerActivationPaysTwo drives the offered equip through
// its target ask and payment: it targets the reducer's creature, taps only
// the two lands for the reduced {2}, and attaches.
func TestEquipTargetReducerActivationPaysTwo(t *testing.T) {
	e, mail, mauler := targetReducerBoard(t, false)
	opt, ok := findAbilityOption(e, mail, 0)
	if !ok {
		t.Fatalf("equip not offered: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	for i := 0; i < 8; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision mid-activation")
		}
		if d.Kind == decision.KTarget {
			idx := -1
			for _, o := range d.Options {
				if o.Obj == mauler {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("the reducer's creature is not a target: %+v", d.Options)
			}
			submitChoices(t, e, idx)
			break
		}
		t.Fatalf("unexpected %v before the target ask", d.Kind)
	}
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Obj(mail).AttachedTo; got != mauler {
		t.Fatalf("attached to %d, want the reducer's creature %d", got, mauler)
	}
	tapped := 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o.Face().Name == "Mountain" && o.Tapped {
			tapped++
		}
	}
	if tapped != 2 {
		t.Fatalf("%d lands tapped for the reduced equip, want 2", tapped)
	}
}
