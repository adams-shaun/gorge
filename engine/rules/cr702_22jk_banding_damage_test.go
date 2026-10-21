// cr-g53b: CR 702.22j-k (and 510.1d). Banding's damage half: a blocker that
// blocks a band (so more than one attacker) divides its combat damage among
// them, and because it blocks a creature with banding the ACTIVE player
// chooses the division; an attacker blocked by a creature with banding has
// its damage divided by the DEFENDING player. Authored fixtures.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	bandOgreSrc  = "Name:Ogre Wall\nManaCost:2 R\nTypes:Creature Ogre\nPT:3/8\nOracle:x\n"
	bandBruteSrc = "Name:Brute\nManaCost:3 G\nTypes:Creature Beast\nPT:4/4\nOracle:x\n"
	bandGuardSrc = "Name:Band Guard\nManaCost:W\nTypes:Creature Human Soldier\nPT:1/5\nK:Banding\nOracle:x\n"
	bandWall2Src = "Name:Wall Two\nManaCost:2 G\nTypes:Creature Wall\nPT:0/5\nOracle:x\n"
)

// TestCR702_22kActivePlayerDividesTheBandBlockersDamage: Band Knight (1/1,
// banding) and Plain Bear (2/2) attack as a band; Ogre Wall (3/8) blocks the
// Bear and so blocks both (CR 702.22h). Its 3 damage is divided among the
// two, by the active player (seat 0, CR 702.22k): all 3 to the Knight, so
// the Bear takes none.
func TestCR702_22kActivePlayerDividesTheBandBlockersDamage(t *testing.T) {
	t.Parallel()
	e := combatEngine(t)
	knight := onBoardReady(t, e, 0, bandKnightSrc)
	bear := onBoardReady(t, e, 0, bandBearSrc)
	ogre := onBoard(t, e, 1, bandOgreSrc)
	e.askAttackers()
	submitAttackersOnly(t, e, knight, bear)
	idx, _ := bandAskOptionFor(t, e, bear)
	submitChoices(t, e, idx)
	drainCombatPriority(t, e)
	submitBlockerPairsOnly(t, e, [2]state.ObjID{bear, ogre})
	drainCombatPriority(t, e)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Source != ogre || d.Player != 0 {
		t.Fatalf("CR 702.22k: pending = %+v, want seat 0's division of Ogre Wall's damage", d)
	}
	submitDivision(t, e, []int32{3, 0})
	if got := e.G.Obj(bear).Damage; got != 0 || e.G.Obj(bear).Zone != state.ZBattlefield {
		t.Fatalf("Plain Bear damage = %d zone %v, want 0 on the battlefield (Ogre Wall's 3 went to the Knight)", got, e.G.Obj(bear).Zone)
	}
	if got := e.G.Obj(knight).Zone; got != state.ZGraveyard {
		t.Fatalf("Band Knight zone = %v, want graveyard (3 damage)", got)
	}
}

// TestCR702_22jDefendingPlayerDividesDamageBlockedByBanding: Brute (4/4)
// is blocked by Band Guard (banding) and Wall Two. The defending player
// (seat 1) divides Brute's damage, not the active player (CR 702.22j).
func TestCR702_22jDefendingPlayerDividesDamageBlockedByBanding(t *testing.T) {
	t.Parallel()
	e := combatEngine(t)
	brute := onBoardReady(t, e, 0, bandBruteSrc)
	guard := onBoard(t, e, 1, bandGuardSrc)
	wall := onBoard(t, e, 1, bandWall2Src)
	e.askAttackers()
	submitAttackers(t, e, brute)
	submitBlockerPairsOnly(t, e, [2]state.ObjID{brute, guard}, [2]state.ObjID{brute, wall})
	drainCombatPriority(t, e)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Source != brute {
		t.Fatalf("pending = %+v, want the division of Brute's damage", d)
	}
	if d.Player != 1 {
		t.Fatalf("CR 702.22j: Brute's damage is divided by seat %d, want the defending player (seat 1)", d.Player)
	}
}
