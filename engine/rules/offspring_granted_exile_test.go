package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A creature cast with Zinnia's GRANTED offspring that is exiled (or dies) with the
// granted trigger still on the stack mints its 1/1 copy anyway: the trigger fired at the
// enter with the pay-time fact (CR 702.175a), and resolves from last known information
// (CR 608.2h). The printed path's twin is TestOffspringSourceThatLeftStillMints; the
// kernel's twin is dirac/oracle TestOffspringGrantedSourceExiledMints.
func TestOffspringGrantedSourceThatLeftStillMints(t *testing.T) {
	t.Parallel()
	for _, to := range []state.Zone{state.ZExile, state.ZGraveyard} {
		reg := searchTestRegistry(t)
		zinnia := searchCorpusCard(t, reg, "Zinnia, Valley's Voice")
		e, cfg, _ := offspringEngine(t, zinnia)
		zinniaID := searchMoveByName(t, e, "Zinnia, Valley's Voice", state.ZHand)
		addMana(t, e, 0, "URWCC")
		submitChoices(t, e, replicateOption(t, e, zinniaID, "").Index)
		passUntilStackEmpty(t, e, 20)
		addMana(t, e, 0, "GCCCCC")
		hero := searchMoveByName(t, e, "Grizzly Bears", state.ZHand)
		submitChoices(t, e, replicateOption(t, e, hero, "offspring").Index)
		for i := 0; i < 8 && e.G.Obj(hero).Zone != state.ZBattlefield; i++ {
			passOnce(t, e)
		}
		if len(e.G.Stack) != 1 {
			t.Fatalf("to %v: stack %v after entering, want the granted offspring trigger", to, e.G.Stack)
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: hero, From: state.ZBattlefield, To: to})
		passUntilStackEmpty(t, e, 20)
		toks := offspringTokens(t, e, hero)
		if len(toks) != 1 {
			t.Fatalf("to %v: token copies = %d, want 1", to, len(toks))
		}
		if p, tg := e.Power(toks[0]), e.Toughness(toks[0]); p != 1 || tg != 1 {
			t.Fatalf("to %v: offspring copy is %d/%d, want 1/1", to, p, tg)
		}
		replayCheck(t, e, cfg)
	}
}
