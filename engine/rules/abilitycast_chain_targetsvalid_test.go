package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// A Mode$ AbilityCast trigger's TargetsValid$ must judge the WHOLE target
// chain an activation announces (CR 602.2b importing 601.2c), not just the
// root declaration's targets. Boom Box's "destroy up to one target artifact,
// up to one target creature, and up to one target land" announces the
// creature through a SubAbility$ link; Ertha Jo, Frontier Mentor's "whenever
// you activate an ability that targets a creature or player" has to see it.
// The open cast flow used to publish only pc.targets (the root answers), so
// the chain link's answer was invisible and the trigger stayed silent.

const abilityChainTargetsValidWatcher = "Name:ChainWatcher\nManaCost:1 G\nTypes:Creature\nPT:2/2\n" +
	"T:Mode$ AbilityCast | ValidActivatingPlayer$ You | TargetsValid$ Creature | TriggerZones$ Battlefield | Execute$ ChainDraw\n" +
	"SVar:ChainDraw:DB$ Draw | Defined$ You | NumCards$ 1\n" +
	"A:AB$ Pump | Cost$ 1 R | ValidTgts$ Artifact | TargetMin$ 0 | TargetMax$ 1 | SubAbility$ SubPump\n" +
	"SVar:SubPump:DB$ Pump | ValidTgts$ Creature | TargetMin$ 1 | TargetMax$ 1 | NumAtt$ 2\n" +
	"Oracle:x\n"

const abilityChainTargetsValidTrinket = "Name:Trinket\nTypes:Artifact\nOracle:x\n"

// submitObjectTarget answers the pending target ask with want, failing if the
// option is not offered (the precondition that the target is where the rule
// looks).
func submitObjectTarget(t *testing.T, e *Engine, want state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected a target ask, got %+v", d)
	}
	for i, o := range d.Options {
		if o.Kind != "player" && o.Obj == want {
			submitChoices(t, e, i)
			return
		}
	}
	t.Fatalf("target %d not offered: %+v", want, d.Options)
}

// TestAbilityCastTargetsValidSeesChainLinkTarget: the trigger fires when the
// creature target is announced by the chain's SubAbility$ link while the root
// declaration targets an artifact.
func TestAbilityCastTargetsValidSeesChainLinkTarget(t *testing.T) {
	t.Parallel()
	watch := card(t, abilityChainTargetsValidWatcher)
	e, watchID := targetsValidEngine(t, watch)
	trinket := e.G.AddObject(card(t, abilityChainTargetsValidTrinket), 0)
	trinket.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, append(e.G.Zone(state.ZBattlefield, 0), trinket.ID))
	e.priorityRound()

	// The activation's root declaration targets the artifact; the creature is
	// chosen by the SubAbility$ link, a separate ask.
	e.beginActivation(0, decision.Option{Kind: "ability", Obj: watchID, Ability: 0})
	submitObjectTarget(t, e, trinket.ID)
	submitObjectTarget(t, e, watchID)

	if n := triggersFromSource(e, watchID); n != 1 {
		t.Fatalf("the watcher queued %d triggers for a chain-link creature target, want 1", n)
	}
	handBefore := len(e.G.Zone(state.ZHand, 0))
	drainTargetsValid(t, e)
	if got := len(e.G.Zone(state.ZHand, 0)) - handBefore; got != 1 {
		t.Fatalf("the trigger drew %d cards, want 1", got)
	}
}
