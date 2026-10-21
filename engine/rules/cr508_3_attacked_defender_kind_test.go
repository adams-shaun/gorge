// cr-g54: CR 508.3a/508.3e/506.6. "Whenever a creature attacks you" and
// "whenever you attack a player" are about attacks on a PLAYER: a creature
// attacking that player's planeswalker (or a battle) does not attack the
// player. gorge's Attacked$ / AttackedTarget$ gates matched the
// DeclareAttackers event's seat (ev.Player), which for a planeswalker or
// battle attack is the permanent's controller or protector, so the player
// half of the spec fired on permanent attacks. Real corpus listeners; the
// attackers and the planeswalker are authored.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	cr508BigBearSrc = "Name:Big Bear\nManaCost:3 G\nTypes:Creature Bear\nPT:4/4\nK:Dethrone\nOracle:x\n"
	cr508WalkerSrc  = "Name:Watch Walker\nManaCost:3\nTypes:Legendary Planeswalker Test\nLoyalty:9\nOracle:x\n"
)

// cr508Attack builds a two-seat board -- seat 0 controls Watch Walker and
// the given listeners, seat 1 a ready 4/4 with dethrone (CR 702.105a:
// "whenever this creature attacks the player with the most life or tied for
// most life"; both seats are at 20) and its own listeners -- and has seat 1
// attack either seat 0 (atWalker false) or Watch Walker (true). It returns
// the engine and the listener objects, seat 0's first, then the 4/4.
func cr508Attack(t *testing.T, atWalker bool, seat0, seat1 []string) (*Engine, []state.ObjID) {
	t.Helper()
	e := layerEngine(t)
	walker := onBoard(t, e, 0, cr508WalkerSrc)
	var ls []state.ObjID
	for _, n := range seat0 {
		ls = append(ls, onBoardCard(t, e, 0, corpusCard(t, n)))
	}
	for _, n := range seat1 {
		ls = append(ls, onBoardCard(t, e, 1, corpusCard(t, n)))
	}
	bear := onBoardReady(t, e, 1, cr508BigBearSrc)
	e.G.Active = 1
	e.G.Step = state.StepDeclareAttackers
	e.askAttackers()
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("expected an attackers decision, got %+v", d)
	}
	want := state.ObjID(0)
	if atWalker {
		want = walker
	}
	idx := -1
	for _, o := range d.Options {
		if o.Obj == bear && o.Player == 0 && o.Battle == want {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no attack option for the bear at battle=%d: %+v", want, d.Options)
	}
	submitChoices(t, e, idx)
	drainCombatPriority(t, e)
	return e, append(ls, bear)
}

// TestCR508_3PlaneswalkerAttackIsNotAnAttackOnItsController: the 4/4 attacks
// seat 0's planeswalker. Hissing Miasma ("whenever a creature attacks you"),
// Orim's Prayer ("whenever one or more creatures attack you") and seat 1's
// Bitter Work ("whenever you attack a player with ... power 4 or greater")
// and the attacker's own dethrone must not trigger.
func TestCR508_3PlaneswalkerAttackIsNotAnAttackOnItsController(t *testing.T) {
	t.Parallel()
	e, ls := cr508Attack(t, true, []string{"Hissing Miasma", "Orim's Prayer"}, []string{"Bitter Work"})
	for i, name := range []string{"Hissing Miasma", "Orim's Prayer", "Bitter Work", "dethrone"} {
		if got := observedTriggerCount(e, ls[i]); got != 0 {
			t.Errorf("CR 508.3a/e: %s triggered %d times on an attack on a planeswalker, want 0", name, got)
		}
	}
}

// TestCR508_3PlayerAttackStillTriggers is the guard: the same four
// listeners each trigger once when the 4/4 attacks seat 0 itself, and
// Revenge of Ravens ("attacks you or a planeswalker you control") triggers
// on both kinds of attack.
func TestCR508_3PlayerAttackStillTriggers(t *testing.T) {
	t.Parallel()
	e, ls := cr508Attack(t, false, []string{"Hissing Miasma", "Orim's Prayer", "Revenge of Ravens"}, []string{"Bitter Work"})
	for i, name := range []string{"Hissing Miasma", "Orim's Prayer", "Revenge of Ravens", "Bitter Work", "dethrone"} {
		if got := observedTriggerCount(e, ls[i]); got != 1 {
			t.Errorf("%s triggered %d times on an attack on its controller, want 1", name, got)
		}
	}
	e, ls = cr508Attack(t, true, []string{"Revenge of Ravens"}, nil)
	if got := observedTriggerCount(e, ls[0]); got != 1 {
		t.Errorf("Revenge of Ravens triggered %d times on an attack on its controller's planeswalker, want 1", got)
	}
}
