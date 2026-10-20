package rules

// CR 603.3d imports CR 601.2c-d: a triggered ability that divides damage
// announces the division as it is put on the stack, right after its targets,
// with at least 1 per target; a target that becomes illegal before it
// resolves loses its share (CR 608.2b). Real corpus cards: Inferno Titan
// ("Whenever Inferno Titan enters or attacks, it deals 3 damage divided as
// you choose among one, two, or three targets") and Grizzly Bears.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestInfernoTitanTriggerAnnouncesDivision: the ETB trigger targets the
// Bears and the opponent, divides 2/1 while it is put on the stack, and the
// Bears die in response: the opponent takes exactly the announced 1.
func TestInfernoTitanTriggerAnnouncesDivision(t *testing.T) {
	t.Parallel()
	reg := choiceCorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{choiceCorpusCard(t, "Inferno Titan")},
		[]*cards.Card{choiceCorpusCard(t, "Grizzly Bears")})
	bear := moveByName(t, e, 1, "Grizzly Bears", state.ZBattlefield)
	moveByName(t, e, 0, "Inferno Titan", state.ZBattlefield)
	td := drainUntilAsk(t, e, 40)
	if td == nil || td.Kind != decision.KTarget {
		t.Fatalf("Inferno Titan's trigger target ask missing: %+v", td)
	}
	if td.Min != 1 || td.Max != 3 {
		t.Fatalf("target bounds = [%d,%d], want [1,3]", td.Min, td.Max)
	}
	submitChoices(t, e, optForObj(t, td, bear), optForPlayer(t, td, 1))
	dd := e.Pending()
	if !isCastDivision(dd) || dd.Min != 1 || dd.Max != 1 || len(dd.Options) != 2 {
		t.Fatalf("CR 603.3d: no 1-pick division announced as the trigger is put on the stack; pending = %+v", dd)
	}
	submitChoices(t, e, optForObj(t, dd, bear)) // Bears 2, player 1
	killBear(t, e, bear)
	passUntilStackEmpty(t, e, 30)
	if got := e.G.Players[1].Life; got != 19 {
		t.Fatalf("opponent life = %d, want 19: the announced 1 only (CR 603.3d, 601.2d, 608.2b)", got)
	}
	replayCheck(t, e, cfg)
}
