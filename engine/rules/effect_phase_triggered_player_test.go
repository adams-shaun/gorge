package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEffectPhaseTriggerTriggeredPlayer pins the Phase registration's
// TriggeredPlayer. Chandra, Awakened Inferno's +2 gives each opponent an
// emblem whose upkeep trigger is
// `DB$ DealDamage | Defined$ TriggeredPlayer | NumDmg$ 1`. A Phase
// registration has no firing event, so the queued Ctx must carry the player
// whose step it is as TriggerPlayer; without it Defined$ TriggeredPlayer
// resolves to nobody and the emblem deals no damage.
func TestEffectPhaseTriggerTriggeredPlayer(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	bear := mustCorpusCard(t, reg, "Grizzly Bears")
	e, cfg, chandra := walkerBoard(t, reg, "Chandra, Awakened Inferno", bear)
	// Precondition: Chandra is on the battlefield at 6 loyalty and the
	// opponent is at 20 life, so the assertions below compare real values.
	if o := e.G.Obj(chandra); o.Zone != state.ZBattlefield || o.Counter("LOYALTY") != 6 {
		t.Fatalf("precondition: Chandra zone %s loyalty %d, want battlefield/6", o.Zone, o.Counter("LOYALTY"))
	}
	if got := e.G.Players[1].Life; got != 20 {
		t.Fatalf("precondition: opponent life = %d, want 20", got)
	}
	delayedBefore := len(e.G.Delayed)
	e.Advance()
	// Ability 0 is the +2 Effect (see the corpus script).
	opt := abilityOption(t, e, chandra, 0)
	submitChoices(t, e, opt.Index)
	passUntilStackEmpty(t, e, 30)
	// Precondition: the +2 registered exactly one delayed trigger, and it is
	// the opponent's (the EffectOwner$ Player.Opponent emblem).
	if len(e.G.Delayed) != delayedBefore+1 {
		t.Fatalf("+2 registered %d delayed triggers, want 1", len(e.G.Delayed)-delayedBefore)
	}
	dt := &e.G.Delayed[len(e.G.Delayed)-1]
	if dt.Controller != 1 {
		t.Fatalf("emblem registration controller = %d, want 1", dt.Controller)
	}
	// The opponent's next upkeep (turn 3, seat 1) is when the emblem's
	// trigger fires; it must deal 1 damage to its controller.
	driveToStepAll(t, e, 3, 1, state.StepUpkeep)
	passUntilStackEmpty(t, e, 30)
	if got := e.G.Players[1].Life; got != 19 {
		t.Fatalf("opponent life after the emblem's upkeep trigger = %d, want 19", got)
	}
	replayCheck(t, e, cfg)
}
