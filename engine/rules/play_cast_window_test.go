package rules

// CR 601.3, 101.2: a spell's own cast restriction ("Cast this spell only
// during ...") binds a cast made while another effect resolves: the
// restriction beats the effect's permission. CastProhibited is the
// effects-side election filter that keeps such a card out of a Play ask.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

const castWindowCrySrc = "Name:Window Cry\nManaCost:1 R\nTypes:Instant\n" +
	"A:SP$ PumpAll | ValidCards$ Creature | KW$ Menace | ActivationPhases$ BeginCombat->Declare Attackers | SpellDescription$ x\n" +
	"Oracle:x\n"

const castWindowGuardSrc = "Name:Window Guard\nManaCost:1 W\nTypes:Instant\n" +
	"A:SP$ PumpAll | ValidCards$ Creature.YouCtrl | NumDef$ 1 | IsPresent$ Creature.attacking | SpellDescription$ x\n" +
	"Oracle:x\n"

func TestCR601_3PlayElectionHonoursSpellCastWindow(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, castWindowCrySrc))
	cry := e.G.Zone(state.ZHand, 0)[0]
	e.G.Step = state.StepMain1
	if !e.CastProhibited(0, cry) {
		t.Errorf("a Play election offers the combat-only spell in the precombat main phase")
	}
	e.G.Step = state.StepDeclareAttackers
	if e.CastProhibited(0, cry) {
		t.Errorf("a Play election withholds the combat-only spell inside its window")
	}
}

func TestCR601_3PlayElectionHonoursSpellPresenceRestriction(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, castWindowGuardSrc))
	guard := e.G.Zone(state.ZHand, 0)[0]
	if !e.CastProhibited(0, guard) {
		t.Errorf("a Play election offers a spell castable only while a creature attacks, with no attacker")
	}
}
