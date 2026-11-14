package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/state"
)

// phaseProbeScript builds an ETB probe card whose second trigger carries the
// Phase$ gate spelling (Dovin's Acuity's "during your main phase", The Blue
// Spirit's "during combat"; gorge trigger_phase_gate.go phaseGate reads it as
// the trigger fires).
func phaseProbeScript(phase string) string {
	return "Name:Phase Gate Probe\nManaCost:2 U\nTypes:Creature\nPT:1/1\n" +
		"T:Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ ProbeDraw | TriggerDescription$ Draw on enter.\n" +
		"T:Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ ProbeDraw2 | Phase$ " + phase + " | TriggerDescription$ When CARDNAME enters during the named phase, draw a card.\n" +
		"SVar:ProbeDraw:DB$ Draw | NumCards$ 1 | Defined$ You\n" +
		"SVar:ProbeDraw2:DB$ Draw | NumCards$ 2\n" +
		"Oracle:x\n"
}

// TestTriggerPhaseCond: a trigger's Phase$ list or range lowers to the shared
// vStepIn step gate the kernel reads as the trigger fires (and at resolution,
// which agrees for every step with priority; an untap-step set refuses, see
// TestTriggerPhaseRefusals); the name set and the mask come from the one
// shared phase parser (state.ParsePhases, gorge phaseGate's own read).
func TestTriggerPhaseCond(t *testing.T) {
	for _, c := range []struct {
		name, phase string
		steps       []state.Step
	}{
		{"main list", "Main1,Main2", []state.Step{state.StepMain1, state.StepMain2}},
		{"combat range", "BeginCombat->EndCombat", []state.Step{
			state.StepBeginCombat, state.StepDeclareAttackers, state.StepDeclareBlockers,
			state.StepCombatDamage, state.StepEndCombat}},
		{"named with a space", "Declare Attackers", []state.Step{state.StepDeclareAttackers}},
	} {
		t.Run(c.name, func(t *testing.T) {
			def, pt, refs := kdiscfiltBuild(t, c.name, phaseProbeScript(c.phase))
			if len(refs) != 0 {
				t.Fatalf("refusals: %q", refs)
			}
			var want int
			for _, s := range c.steps {
				want |= phaseStepBits[s]
			}
			found := 0
			for i := range def.Abilities {
				a := &def.Abilities[i]
				if a.Trig.Cond == 0 {
					continue
				}
				found++
				cond := pt.conds[a.Trig.Cond]
				if cond.Kind != kop.CondValue || cond.Neg || cond.Op != kop.CmpEQ {
					t.Fatalf("cond %v, want a plain value gate", cond)
				}
				v := pt.exprs[cond.A.N]
				if v.Kind != kop.ExStepIn || int(v.N) != want {
					t.Fatalf("value %+v, want ExStepIn mask %#x", v, want)
				}
			}
			if found != 1 {
				t.Fatalf("%d gated triggers, want 1", found)
			}
		})
	}
}

// TestTriggerPhaseRefusals: the shapes outside the lowered class refuse by
// name, never by lowering a half-read gate.
func TestTriggerPhaseRefusals(t *testing.T) {
	for _, c := range []struct{ name, phase, want string }{
		{"unknown phase", "NotAPhase", "unknown phase"},
		{"backwards range", "EndCombat->BeginCombat", "unknown phase"},
		{"empty element", "Main1,,Main2", "unknown phase"},
		{"untap", "Untap", "untap step"},
		{"range from untap", "Untap->Draw", "untap step"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, _, refs := kdiscfiltBuild(t, c.name, phaseProbeScript(c.phase))
			if len(refs) == 0 {
				t.Fatalf("no refusals, want %q", c.want)
			}
			kdefWant(t, reasons(refs), c.want)
		})
	}
}
