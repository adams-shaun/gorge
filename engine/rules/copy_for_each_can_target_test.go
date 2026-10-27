package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// runForEachScenario runs one inline XMage-port scenario and requires a clean
// audit: no fails and an empty replay diff.
func runForEachScenario(t *testing.T, sc string) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleAuditScenarioJSON(reg, []byte(sc), true)
	if err != nil || len(res.Fails) != 0 {
		t.Fatalf("scenario: err=%v fails=%v", err, res.Fails)
	}
}

// TestZadaCopiesForEachOtherCreature pins CR 707.10d for CopyForEachCanTarget$
// (XMage ZadaHedronGrinderTest#testTargetsByAI): Arbor Armament targeting only
// Zada is copied once per other creature, each copy targeting a different one,
// so all three creatures get one counter (4/4, 3/3, 3/3), not two on Zada.
func TestZadaCopiesForEachOtherCreature(t *testing.T) {
	t.Parallel()
	runForEachScenario(t, `{"name":"zada","cr":["707.10d"],"why":"XMage ZadaHedronGrinderTest#testTargetsByAI","setup":{"p0":{"hand":["Arbor Armament"],"battlefield":["Mountain","Zada, Hedron Grinder","Silvercoat Lion","Balduvian Bears","Forest"]}},"steps":[{"op":"cast","seat":0,"card":"p0:Arbor Armament","mana":"G","targets":["p0:Zada, Hedron Grinder"]},{"op":"pass_to","seat":0,"step":"begin-combat","active":"p0"},{"op":"resolve","seat":0}],"expect":[{"card":"p0:Zada, Hedron Grinder","pt":"4/4"},{"card":"p0:Silvercoat Lion","pt":"3/3"},{"card":"p0:Balduvian Bears","pt":"3/3"}]}`)
}

// TestZadaStriveCopies is the strive-spell port (XMage
// ZadaHedronGrinderTest#testWithStriveSpell).
func TestZadaStriveCopies(t *testing.T) {
	t.Parallel()
	runForEachScenario(t, `{"name":"zada-strive","cr":["707.10d"],"why":"XMage ZadaHedronGrinderTest#testWithStriveSpell","setup":{"p0":{"hand":["Rouse the Mob"],"battlefield":["Mountain","Zada, Hedron Grinder","Silvercoat Lion"]}},"steps":[{"op":"cast","seat":0,"card":"p0:Rouse the Mob","mana":"R","targets":["p0:Zada, Hedron Grinder"]},{"op":"pass_to","seat":0,"step":"begin-combat","active":"p0"},{"op":"resolve","seat":0}],"expect":[{"card":"p0:Zada, Hedron Grinder","pt":"5/3"},{"card":"p0:Silvercoat Lion","pt":"4/2"},{"card":"p0:Silvercoat Lion","keywords":["Trample"]}]}`)
}

// TestCopyWithoutForEachKeepsTarget: a CopySpellAbility line without
// CopyForEachCanTarget$ (Twincast) is untouched -- one copy that keeps the
// original's target. Lightning Bolt at the opponent, copied once by Twincast
// with the inherited target kept, deals 3 + 3 and leaves exactly two bolts'
// worth of damage, not one copy per candidate.
func TestCopyWithoutForEachKeepsTarget(t *testing.T) {
	t.Parallel()
	reg := choiceCorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg,
		[]*cards.Card{choiceCorpusCard(t, "Lightning Bolt"), choiceCorpusCard(t, "Twincast"), choiceCorpusCard(t, "Grizzly Bears")}, nil)
	bolt := moveByName(t, e, 0, "Lightning Bolt", state.ZHand)
	twin := moveByName(t, e, 0, "Twincast", state.ZHand)
	moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	e.pending = nil
	e.priorityRound()
	life0 := e.G.Players[1].Life

	addMana(t, e, 0, "R")
	cr707CastCostPaid(t, e, bolt, 0, 0)
	cr707TwincastAndResolve(t, e, twin, bolt, 0)

	if got := e.G.Players[1].Life; got != life0-6 {
		t.Errorf("seq %d: opponent life %d, want %d (one copy keeping the opponent as its target)", len(e.L.Events), got, life0-6)
	}
	replayCheck(t, e, cfg)
}
