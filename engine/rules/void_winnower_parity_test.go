package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// Void Winnower's two statics read Forge's cmcEven card property (CR 202.3:
// mana value; "Zero is even."). Before cr-g03 the numeric-predicate parser
// split "cmcEven" into comparator "Ev" and right-hand side "en", the RHS
// failed to resolve, and the predicate answered (false, recognised): both
// statics were silent no-ops and UnknownPredicates called the shape
// understood.

// TestVoidWinnowerForbidsOpponentEvenManaValueSpells is the CR 601.3
// prohibition: with an opponent's Void Winnower on the battlefield, seat 0
// is not offered Grizzly Bears (mana value 2) or Ornithopter (mana value 0,
// CR 202.3a: zero is even), and is still offered Llanowar Elves (mana value
// 1).
func TestVoidWinnowerForbidsOpponentEvenManaValueSpells(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusCard(t, "Grizzly Bears"), corpusCard(t, "Ornithopter"), corpusCard(t, "Llanowar Elves"))
	hand := e.G.Zone(state.ZHand, 0)
	bears, thopter, elves := hand[0], hand[1], hand[2]
	addMana(t, e, 0, "GG")
	for _, id := range []state.ObjID{bears, thopter, elves} {
		if castOptionNamed(e, id) == nil {
			t.Fatalf("precondition: %s not offered before Void Winnower is on the battlefield", e.G.Obj(id).Face().Name)
		}
	}
	onBoardCard(t, e, 1, corpusCard(t, "Void Winnower"))
	e.priorityRound()
	if castOptionNamed(e, bears) != nil {
		t.Error("Grizzly Bears (mana value 2, even) offered to Void Winnower's opponent")
	}
	if castOptionNamed(e, thopter) != nil {
		t.Error("Ornithopter (mana value 0, even: CR 202.3a) offered to Void Winnower's opponent")
	}
	if castOptionNamed(e, elves) == nil {
		t.Error("Llanowar Elves (mana value 1, odd) must stay castable under Void Winnower")
	}
}

// TestVoidWinnowerForbidsOpponentEvenManaValueBlockers is the CantBlockBy
// half: ValidBlocker$ Creature.OppCtrl+cmcEven. Seat 0's Grizzly Bears
// (even) cannot block Winnower's attack; seat 0's Llanowar Elves (odd) can;
// Winnower's controller's own even creature is not restricted (OppCtrl).
func TestVoidWinnowerForbidsOpponentEvenManaValueBlockers(t *testing.T) {
	t.Parallel()
	e := combatEngine(t)
	winnower := onBoardCard(t, e, 1, corpusCard(t, "Void Winnower"))
	e.G.Obj(winnower).IsAttacking, e.G.Obj(winnower).Attacking = true, 0
	bears := onBoardCard(t, e, 0, corpusCard(t, "Grizzly Bears"))
	elves := onBoardCard(t, e, 0, corpusCard(t, "Llanowar Elves"))
	ownBears := onBoardCard(t, e, 1, corpusCard(t, "Grizzly Bears"))
	attacker := onBoardCard(t, e, 0, corpusCard(t, "Llanowar Elves"))
	e.G.Obj(attacker).IsAttacking, e.G.Obj(attacker).Attacking = true, 1

	if !e.blockRestricted(bears, winnower) {
		t.Error("Grizzly Bears (mana value 2, even) must not be able to block under Void Winnower")
	}
	if e.canBlock(bears, winnower) {
		t.Error("canBlock: Grizzly Bears (even) blocked under Void Winnower")
	}
	if e.blockRestricted(elves, winnower) || !e.canBlock(elves, winnower) {
		t.Error("Llanowar Elves (mana value 1, odd) must stay able to block under Void Winnower")
	}
	if e.blockRestricted(ownBears, attacker) {
		t.Error("Void Winnower's controller's own even creature must not be restricted (ValidBlocker$ Creature.OppCtrl)")
	}
}

// TestVoidWinnowerRechecksChosenXOnTheStack is CR 202.3e + 601.2e: an
// {X}{G} spell has mana value 1 in hand (offered), but with X=1 it is a
// mana-value-2 spell on the stack, which Void Winnower forbids, so the cast
// is reversed; X=2 (mana value 3) is the legal control.
func TestVoidWinnowerRechecksChosenXOnTheStack(t *testing.T) {
	t.Parallel()
	for _, x := range []int{1, 2} {
		t.Run(map[int]string{1: "X1_even_reversed", 2: "X2_odd_legal"}[x], func(t *testing.T) {
			t.Parallel()
			spell := card(t, "Name:Verdant Sum\nManaCost:X G\nTypes:Sorcery\n"+
				"A:SP$ GainLife | Defined$ You | LifeAmount$ X | SpellDescription$ x\nSVar:X:Count$xPaid\nOracle:x\n")
			e := handEngine(t, spell)
			id := e.G.Zone(state.ZHand, 0)[0]
			onBoardCard(t, e, 1, corpusCard(t, "Void Winnower"))
			addMana(t, e, 0, "GGG")
			opt := castOptionNamed(e, id)
			if opt == nil {
				t.Fatal("precondition: an {X}{G} spell (mana value 1 in hand, CR 202.3e) must be offered under Void Winnower")
			}
			submitChoices(t, e, opt.Index)
			d := e.Pending()
			if d == nil || d.Kind != decision.KChoose || d.Source != id {
				t.Fatalf("no X decision after casting: %+v", d)
			}
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "x" && o.Amount == x {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("no X=%d choice in %+v", x, d.Options)
			}
			submitChoices(t, e, idx)
			for n := 0; n < 10; n++ {
				d = e.Pending()
				if d == nil || d.Kind == decision.KPriority {
					break
				}
				submitChoices(t, e, d.Options[0].Index)
			}
			zone := e.G.Obj(id).Zone
			if x == 1 && zone != state.ZHand {
				t.Errorf("X=1 makes the spell mana value 2 on the stack (CR 202.3e); Void Winnower forbids it, so the cast must be reversed to hand (CR 601.2e), got zone %s", zone)
			}
			if x == 2 && zone != state.ZStack {
				t.Errorf("X=2 gives mana value 3 (odd); the cast must complete, got zone %s", zone)
			}
		})
	}
}
