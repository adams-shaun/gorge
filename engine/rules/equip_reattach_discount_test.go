package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const (
	reattachMaulerSrc = "Name:Mauler\nManaCost:R\nTypes:Creature Dwarf Warrior\nPT:2/1\nS:Mode$ ReduceCost | ValidTarget$ Card.Self | ValidSpell$ Activated.Equip | Activator$ You | Amount$ 2 | Description$ Equip abilities you activate that target this creature cost {2} less to activate.\nOracle:x\n"
	reattachSpearSrc  = "Name:Spear\nManaCost:1 R\nTypes:Artifact Equipment\nS:Mode$ Continuous | AffectedDefined$ Equipped | Affected$ Creature | AddPower$ 2 | Description$ Equipped creature gets +2/+0.\nK:Equip:3\nOracle:x\n"
)

func reattachGame(t *testing.T, seed uint64, attached bool) (*Engine, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	cfg := Config{Seed: seed, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{card(t, reattachMaulerSrc), card(t, reattachSpearSrc)}, mountainDeck(t, 38)...),
			mountainDeck(t, 40),
		},
		Tokens: reg.Tokens,
	}
	e := New(seatZeroStart(cfg))
	e.Advance()
	mauler := moveSeeded(t, e, 0, reattachMaulerSrc, state.ZBattlefield)
	spear := moveSeeded(t, e, 0, reattachSpearSrc, state.ZBattlefield)
	const mt = "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n"
	moveSeeded(t, e, 0, mt, state.ZBattlefield)
	moveSeeded(t, e, 0, mt, state.ZBattlefield)
	e.pending = nil
	e.Advance()
	driveToStep(t, e, 3, 0, state.StepMain1)
	if attached {
		addMana(t, e, 0, "C")
		opt, ok := findAbilityOption(e, spear, 0)
		if !ok {
			t.Fatal("first equip not offered")
		}
		submitChoices(t, e, opt.Index)
		targetObject(t, e, mauler)
		passUntilStackEmpty(t, e, 20)
		if e.G.Obj(spear).AttachedTo != mauler {
			t.Fatal("precondition: spear not attached to the Mauler")
		}
	}
	return e, spear
}

func TestEquipDiscountOfferedWhenAlreadyAttachedToTarget(t *testing.T) {
	t.Parallel()
	for _, attached := range []bool{false, true} {
		e, spear := reattachGame(t, 611, attached)
		if _, ok := findAbilityOption(e, spear, 0); !ok {
			t.Fatalf("attached=%v: equip {3} with a -{2} target not offered from {2}", attached)
		}
	}
}
