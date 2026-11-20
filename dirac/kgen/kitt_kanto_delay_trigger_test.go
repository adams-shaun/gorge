package kgen

import (
	"strings"
	"testing"
)

// The 2026-11-20 Forge respellings Kitt Kanto, Mayhem Diva and Suppressor
// Skyguard carry. Inline scripts, so the probes do not depend on the pin of
// the seat's corpus.

const kittProbe = "Name:Kitt Probe\nManaCost:1 R G W\nTypes:Legendary Creature Cat Bard\nPT:3/3\n" +
	"T:Mode$ Phase | Phase$ BeginCombat | Execute$ TrigImmediateTrig | TriggerZones$ Battlefield | TriggerDescription$ x\n" +
	"SVar:TrigImmediateTrig:AB$ ImmediateTrigger | Cost$ tapXType<2/Creature> | PARAMS | Execute$ TrigPump | TriggerDescription$ x\n" +
	"SVar:TrigPump:DB$ Pump | ValidTgts$ Creature.ControlledBy DelayTriggerRemembered | TgtPrompt$ x | NumAtt$ +2 | NumDef$ +2 | KW$ Trample | SubAbility$ DBGoad\n" +
	"SVar:DBGoad:DB$ Goad | Defined$ Targeted\nOracle:x\n"

// TestKittKantoDelayTriggerRemembered: `RememberObjects$ TriggeredPlayer` over a
// Phase trigger makes the pump's "ControlledBy DelayTriggerRemembered" target
// the active player's creature (kernel PredActiveCtrl), the reading the
// retired ActivePlayerCtrl spelling had.
func TestKittKantoDelayTriggerRemembered(t *testing.T) {
	lc := lowerProbe(t, kittProbe, "Kitt Probe", "RememberObjects$ TriggeredPlayer")
	if r := probeRefusals(lc); r != "" {
		t.Fatalf("Kitt Kanto's respelled pump refused: %s", r)
	}
	found := false
	for _, ab := range lc.Abilities {
		for _, tg := range ab.Targets {
			if tg.F != nil && len(tg.F.Clauses) == 1 && tg.F.Clauses[0].CmpStat == cmpActiveCtrl {
				found = true
			}
		}
	}
	if !found {
		var b strings.Builder
		dumpLIR(&b, lc)
		t.Fatalf("no pump target carries the active-player-controls word:\n%s", b.String())
	}
	// The word without the capture names nobody this build models: it refuses.
	lc = lowerProbe(t, kittProbe, "Kitt Probe", "TriggerAmount$ 1")
	if r := probeRefusals(lc); !strings.Contains(r, "DelayTriggerRemembered") {
		t.Errorf("an ImmediateTrigger with no RememberObjects$ must refuse the DelayTriggerRemembered word, got %q", r)
	}
}

const skyguardProbe = "Name:Sky Probe\nManaCost:2 W U\nTypes:Creature Human Knight\nPT:2/4\nK:Flying\n" +
	"T:Mode$ AttackersDeclared | AttackingPlayer$ Player | PARAMS | CheckDefinedPlayer$ Player.OpponentOf TriggeredAttackingPlayer+!Defending | TriggerZones$ Battlefield | Execute$ TrigSuppression | TriggerDescription$ x\n" +
	"SVar:TrigSuppression:DB$ Effect | ReplacementEffects$ RPrevent | Duration$ UntilEndOfCombat | AILogic$ Fog\n" +
	"SVar:RPrevent:Event$ DamageDone | Prevent$ True | IsCombat$ True | ActiveZones$ Command | ValidTarget$ You | Description$ x\nOracle:x\n"

// TestSuppressorSkyguardGate: with two seats the attacker's only opponent is
// the attacked seat, so the intervening-if never holds: it lowers as a
// condition that is never true on a card marked two-seat. The gate's
// precondition (AttackedTarget$ You) is read, never assumed.
func TestSuppressorSkyguardGate(t *testing.T) {
	lc := lowerProbe(t, skyguardProbe, "Sky Probe", "AttackedTarget$ You")
	if r := probeRefusals(lc); r != "" {
		t.Fatalf("Suppressor Skyguard's respelled gate refused: %s", r)
	}
	if !lc.TwoSeat {
		t.Error("the gate reads the two-seat table: the card must be marked two-seat")
	}
	var tr *lTrigger
	for _, ab := range lc.Abilities {
		if ab.Trigger != nil {
			tr = ab.Trigger
		}
	}
	if tr == nil {
		t.Fatal("no lowered trigger")
	}
	// A condition that is false: 0 >= 1.
	c := tr.Cond
	if c.Kind != cValue || !c.A.isConst() || !c.B.isConst() || c.A.N != 0 || c.B.N != 1 || c.Op != opGE || c.Neg {
		t.Errorf("trigger condition = %+v, want the never-true 0 >= 1", c)
	}
	// Without AttackedTarget$ You there is no Defending seat to name.
	lc = lowerProbe(t, skyguardProbe, "Sky Probe", "AttackedTarget$ Player")
	if r := probeRefusals(lc); !strings.Contains(r, "CheckDefinedPlayer") {
		t.Errorf("a gate outside AttackedTarget$ You must refuse, got %q", r)
	}
}
