package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The ValidCause$ read of a Draw replacement line (lower_drawrepl.go): gorge
// confines the line to draws its cause admits (engine/rules/trigger_cause.go
// drawCauseAdmits), so a kernel lowering that ignored the parameter would
// replace every draw the controller makes. Only the one spec the corpus
// carries (Unpredictable Cyclone's Activated.Cycling+nonLand) lowers, onto
// kop.ReplDrawCauseCyclingNonLand; every other spec refuses.

const causeSkipProbe = `Name:Probe Cause Skip
ManaCost:2
Types:Enchantment
R:Event$ Draw | ValidCause$ Activated.Cycling+nonLand | ActiveZones$ Battlefield | ValidPlayer$ You | Prevent$ True | Description$ probe.
`

// causeDrawBodyProbe is the gated line with the plain "draw N cards instead"
// body: ReplDrawMore's kernel walk (drawMore) never consults the cause, so the
// line must not take that kind with the gate dropped.
const causeDrawBodyProbe = `Name:Probe Cause Draw Body
ManaCost:2
Types:Enchantment
R:Event$ Draw | ValidCause$ Activated.Cycling+nonLand | ActiveZones$ Battlefield | ValidPlayer$ You | ReplaceWith$ ProbeDraw | Description$ probe.
SVar:ProbeDraw:DB$ Draw | Defined$ You | NumCards$ 2
`

const causeLandProbe = `Name:Probe Cause Land
ManaCost:2
Types:Enchantment
R:Event$ Draw | ValidCause$ Activated.Cycling+land | ActiveZones$ Battlefield | ValidPlayer$ You | Prevent$ True | Description$ probe.
`

const causeTriggeredProbe = `Name:Probe Cause Triggered
ManaCost:2
Types:Enchantment
R:Event$ Draw | ValidCause$ Triggered.Cycling+nonLand | ActiveZones$ Battlefield | ValidPlayer$ You | Prevent$ True | Description$ probe.
`

func TestDrawReplValidCause(t *testing.T) {
	t.Run("the cycling nonLand spec lowers onto the cause bit", func(t *testing.T) {
		c := lowerScriptCard(t, "Probe Cause Skip", causeSkipProbe)
		if len(c.Refusals) != 0 {
			t.Fatalf("refusals = %v, want none", c.Refusals)
		}
		if len(c.Repls) != 1 {
			t.Fatalf("repls = %d, want one", len(c.Repls))
		}
		r := c.Repls[0]
		if r.Kind != "DrawProgram" {
			t.Fatalf("kind = %q, want DrawProgram", r.Kind)
		}
		if r.Bits&kop.ReplDrawCauseCyclingNonLand == 0 {
			t.Fatalf("bits = %d, want ReplDrawCauseCyclingNonLand set (the gate would be dropped)", r.Bits)
		}
		if r.Bits&kop.ReplDrawSkip == 0 {
			t.Fatalf("bits = %d, want the Prevent$ skip bit kept", r.Bits)
		}
	})
	t.Run("a plain draw body under the gate is not DrawMore", func(t *testing.T) {
		// DrawMore's walk reads no cause: lowering it here would silently
		// widen the line to every draw (repl_draw.go).
		c := lowerScriptCard(t, "Probe Cause Draw Body", causeDrawBodyProbe)
		if len(c.Refusals) == 0 {
			t.Fatal("the gated draw body lowered; ReplDrawMore cannot carry the cause gate")
		}
		for _, rf := range c.Refusals {
			if !strings.Contains(rf.Reason, "Draw replacement") {
				t.Fatalf("refusal %q, want it to name the Draw replacement class", rf.Reason)
			}
		}
	})
	t.Run("a cause spec the kernel cannot evaluate refuses", func(t *testing.T) {
		for _, w := range []struct{ name, src string }{
			{"Probe Cause Land", causeLandProbe},           // a qualifier the gate does not read
			{"Probe Cause Triggered", causeTriggeredProbe}, // not an activated cause
		} {
			c := lowerScriptCard(t, w.name, w.src)
			if len(c.Refusals) == 0 {
				t.Fatalf("%s lowered; an unread ValidCause$ must fail closed", w.name)
			}
			var ok bool
			for _, rf := range c.Refusals {
				ok = ok || strings.Contains(rf.Reason, "ValidCause")
			}
			if !ok {
				t.Fatalf("%s refusals = %v, want one naming ValidCause", w.name, c.Refusals)
			}
		}
	})
}
