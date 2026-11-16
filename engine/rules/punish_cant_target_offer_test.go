package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Dense Foliage ("creatures can't be the targets of spells") leaves Punish the
// Enemy's second target, "target creature" (a SubAbility$ link of the spell),
// without a candidate, so the spell can't be announced and is not offered
// (CR 601.2c, 115.1). The link's Kind is DB, not SP; it takes its root's.
func TestPunishTheEnemyNotOfferedUnderDenseFoliage(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	foliage := lookup(t, reg, "Dense Foliage")
	scout := lookup(t, reg, "Talas Scout")
	punish := lookup(t, reg, "Punish the Enemy")
	e, _ := corpusEngineCfg(t, reg, []*cards.Card{foliage, scout}, []*cards.Card{punish})

	fid := moveByName(t, e, 0, "Dense Foliage", state.ZBattlefield)
	moveByName(t, e, 0, "Talas Scout", state.ZBattlefield)
	cantTargetOnBoard(t, e, fid)
	pid := moveByName(t, e, 1, "Punish the Enemy", state.ZHand)
	if sa := e.G.Obj(pid).Face().SpellAbility(); sa == nil || len(e.collectSubTargetPreAsks(sa)) != 1 {
		t.Fatalf("precondition: Punish the Enemy has no targeted chain link: %+v", sa)
	}
	addMana(t, e, 1, "RRRRR")
	e.askPriority(1)
	if castOffered(e, pid) {
		t.Fatalf("Punish the Enemy offered with no creature a spell may target: %+v", e.Pending().Options)
	}
}
