package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The TargetValidTargeting$ lockstep for every card the corpus classes newly
// lowered by the dot-zone, "You" and spell-side-filter handling: Confound and
// Intervene ("Creature.inRealZoneBattlefield", a dot before the zone word),
// Vigilant Martyr (the dot form with an Enchantment type), Avoid Fate and Ring
// of Immortals (a spell-side ValidTgts$ Instant,Aura filter beside the clause)
// and Dawn Charm and Cerulean Drake (TargetValidTargeting$ You, a player
// target).
//
// The opponent's Flame Slash is a sorcery aimed at the first deck's creatures:
// it meets Confound and Intervene but not Avoid Fate's Instant,Aura filter, so
// the offer lists differ if the spell-side filter is dropped. The opponent's
// Heroes' Reunion can target its own controller (player 1); a spell targeting
// the other player is not "You" to Dawn Charm's controller (player 0), so the
// offer differs if the seat check is relaxed to any player. The first deck's
// Lightning Bolt (with Mountains) can aim at the opponent's face for the same
// check from the other side. Shock, Lightning Bolt and Pacifism from the
// opponent aim at the first deck's creatures and face; Giant Growth at its own.
var kctrvtgtDecks = [2][]cardset.Entry{
	{
		{Name: "Confound", Count: 3}, {Name: "Intervene", Count: 3}, {Name: "Avoid Fate", Count: 3},
		{Name: "Dawn Charm", Count: 4}, {Name: "Vigilant Martyr", Count: 2}, {Name: "Cerulean Drake", Count: 2},
		{Name: "Ring of Immortals", Count: 2}, {Name: "Grizzly Bears", Count: 2}, {Name: "Lightning Bolt", Count: 1},
		{Name: "Giant Growth", Count: 1}, {Name: "Island", Count: 7}, {Name: "Plains", Count: 5},
		{Name: "Forest", Count: 3}, {Name: "Mountain", Count: 2},
	},
	{
		{Name: "Shock", Count: 2}, {Name: "Lightning Bolt", Count: 3}, {Name: "Pacifism", Count: 2},
		{Name: "Flame Slash", Count: 2}, {Name: "Giant Growth", Count: 2}, {Name: "Heroes' Reunion", Count: 4},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Hill Giant", Count: 2}, {Name: "Llanowar Elves", Count: 3},
		{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 5}, {Name: "Plains", Count: 4},
	},
}

func TestCtrvtgtLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "ctrvtgt", kctrvtgtDecks, 400, "KCTRVTGT_SEEDS")
}

// TestCtrvtgtLockstepMutation: with TgtSpellTgting ignoring the targeting
// clause (and the spell-side filter), some seed diverges, so the pool does
// meet an untargeting or wrong-targeting spell the cards must refuse.
func TestCtrvtgtLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugCtrShapesOff = true
	defer func() { dirac.DebugCtrShapesOff = false }()
	if !lockstepDiverges(t, "ctrvtgt", kctrvtgtDecks, 400) {
		t.Fatal("no seed diverged with the clause ignored: no spell met Confound, Intervene, Avoid Fate or Dawn Charm that its clause excludes")
	}
}

// TestCtrvtgtSpellFMutation: dropping only the spell-side filter diverges, so
// Flame Slash (a sorcery) really meets Avoid Fate's Instant,Aura exclusion.
func TestCtrvtgtSpellFMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugCtrSpellFOff = true
	defer func() { dirac.DebugCtrSpellFOff = false }()
	if !lockstepDiverges(t, "ctrvtgt", kctrvtgtDecks, 400) {
		t.Fatal("no seed diverged with the spell-side filter dropped: no spell met Avoid Fate that its Instant,Aura filter excludes")
	}
}

// TestCtrvtgtVYouMutation: accepting any player as the "You" target diverges,
// so a spell targeting a player other than Dawn Charm's controller really
// meets the seat check.
func TestCtrvtgtVYouMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugCtrVYouAnyPlayer = true
	defer func() { dirac.DebugCtrVYouAnyPlayer = false }()
	if !lockstepDiverges(t, "ctrvtgt", kctrvtgtDecks, 400) {
		t.Fatal("no seed diverged with \"You\" widened to any player: no spell targeted a player other than Dawn Charm's controller")
	}
}
