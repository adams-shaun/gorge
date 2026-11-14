// Defensive Formation: "Rather than the attacking player, you assign the
// combat damage of each creature attacking you." The defending player, not
// the active player, divides a multiply-blocked attacker's damage (CR 510.1c
// as the card rewrites it). Authored fixtures.
package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	defFormationSrc = "Name:Defensive Formation\nManaCost:W\nTypes:Enchantment\nS:Mode$ Continuous | Affected$ You | AddKeyword$ You assign combat damage of each creature attacking you.\nOracle:x\n"
	defWalkerSrc    = "Name:Formation Walker\nManaCost:2 W\nTypes:Planeswalker Test\nLoyalty:4\nOracle:x\n"
)

// TestDefensiveFormationDefenderAssigns: Brute (4/4, seat 0) is blocked by
// Wall Two and Ogre Wall; seat 1 controls Defensive Formation, so seat 1
// divides Brute's damage and the split it picks is the one applied.
func TestDefensiveFormationDefenderAssigns(t *testing.T) {
	t.Parallel()
	for _, withFormation := range []bool{true, false} {
		e := combatEngine(t)
		brute := onBoardReady(t, e, 0, bandBruteSrc)
		if withFormation {
			onBoard(t, e, 1, defFormationSrc)
		}
		wall := onBoard(t, e, 1, bandWall2Src)
		ogre := onBoard(t, e, 1, bandOgreSrc)
		e.askAttackers()
		submitAttackers(t, e, brute)
		submitBlockerPairsOnly(t, e, [2]state.ObjID{brute, wall}, [2]state.ObjID{brute, ogre})
		drainCombatPriority(t, e)
		if got := slices.Contains(e.playerKeywords(1), kwDefenderAssignsCombatDamage); got != withFormation {
			t.Fatalf("precondition: seat 1 holds the keyword = %v, want %v", got, withFormation)
		}
		if n := len(e.liveBlockers(e.G.Obj(brute))); n != 2 {
			t.Fatalf("precondition: Brute has %d live blockers, want 2", n)
		}
		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose || d.Source != brute {
			t.Fatalf("pending = %+v, want the division of Brute's damage", d)
		}
		want := state.PlayerID(0)
		if withFormation {
			want = 1
		}
		if d.Player != want {
			t.Fatalf("formation=%v: Brute's damage is divided by seat %d, want seat %d", withFormation, d.Player, want)
		}
		for _, o := range d.Options {
			if o.Player != want {
				t.Fatalf("formation=%v: option %+v names seat %d, want seat %d", withFormation, o, o.Player, want)
			}
		}
		if !withFormation {
			continue
		}
		submitDivision(t, e, []int32{4, 0})
		if got := e.G.Obj(wall).Damage; got != 4 {
			t.Fatalf("Wall Two damage = %d, want 4 (the defender's split)", got)
		}
		if got := e.G.Obj(ogre).Damage; got != 0 {
			t.Fatalf("Ogre Wall damage = %d, want 0 (the defender's split)", got)
		}
	}
}

// TestDefensiveFormationLeavesOtherShapesAlone: a single blocker raises no
// division at all; a Formation the ATTACKER controls leaves the chooser as
// the active player; and an attacker aimed at the Formation controller's
// planeswalker is not "attacking you", so the active player still divides.
func TestDefensiveFormationLeavesOtherShapesAlone(t *testing.T) {
	t.Parallel()
	t.Run("one blocker", func(t *testing.T) {
		t.Parallel()
		e := combatEngine(t)
		brute := onBoardReady(t, e, 0, bandBruteSrc)
		onBoard(t, e, 1, defFormationSrc)
		wall := onBoard(t, e, 1, bandWall2Src)
		e.askAttackers()
		submitAttackers(t, e, brute)
		submitBlockerPairsOnly(t, e, [2]state.ObjID{brute, wall})
		drainCombatPriority(t, e)
		if !slices.Contains(e.playerKeywords(1), kwDefenderAssignsCombatDamage) {
			t.Fatal("precondition: seat 1 lacks the keyword")
		}
		if d := e.Pending(); d != nil && d.Kind == decision.KChoose && d.Source == brute {
			t.Fatalf("one blocker raised a division decision: %+v", d)
		}
		if got := e.G.Obj(wall).Damage; got != 4 {
			t.Fatalf("Wall Two damage = %d, want 4", got)
		}
	})
	t.Run("formation on the attacker's side", func(t *testing.T) {
		t.Parallel()
		e := combatEngine(t)
		brute := onBoardReady(t, e, 0, bandBruteSrc)
		onBoard(t, e, 0, defFormationSrc)
		wall := onBoard(t, e, 1, bandWall2Src)
		ogre := onBoard(t, e, 1, bandOgreSrc)
		e.askAttackers()
		submitAttackers(t, e, brute)
		submitBlockerPairsOnly(t, e, [2]state.ObjID{brute, wall}, [2]state.ObjID{brute, ogre})
		drainCombatPriority(t, e)
		if !slices.Contains(e.playerKeywords(0), kwDefenderAssignsCombatDamage) ||
			slices.Contains(e.playerKeywords(1), kwDefenderAssignsCombatDamage) {
			t.Fatal("precondition: the keyword is not seat 0's alone")
		}
		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose || d.Source != brute || d.Player != 0 {
			t.Fatalf("pending = %+v, want seat 0's division of Brute's damage", d)
		}
	})
	t.Run("attacking a planeswalker", func(t *testing.T) {
		t.Parallel()
		e := combatEngine(t)
		brute := onBoardReady(t, e, 0, bandBruteSrc)
		onBoard(t, e, 1, defFormationSrc)
		walker := onBoard(t, e, 1, defWalkerSrc)
		wall := onBoard(t, e, 1, bandWall2Src)
		ogre := onBoard(t, e, 1, bandOgreSrc)
		e.askAttackers()
		d0 := e.Pending()
		idx := -1
		for _, o := range d0.Options {
			if o.Obj == brute && o.Battle == walker {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("precondition: no option attacks the planeswalker: %+v", d0.Options)
		}
		submitChoices(t, e, idx)
		drainCombatPriority(t, e)
		if e.G.Obj(brute).AttackingBattle != walker {
			t.Fatal("precondition: Brute is not attacking the planeswalker")
		}
		submitBlockerPairsOnly(t, e, [2]state.ObjID{brute, wall}, [2]state.ObjID{brute, ogre})
		drainCombatPriority(t, e)
		if !slices.Contains(e.playerKeywords(1), kwDefenderAssignsCombatDamage) {
			t.Fatal("precondition: seat 1 lacks the keyword")
		}
		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose || d.Source != brute || d.Player != 0 {
			t.Fatalf("pending = %+v, want seat 0's division of Brute's damage (planeswalker attack)", d)
		}
	})
}
