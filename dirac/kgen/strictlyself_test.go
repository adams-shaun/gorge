package kgen

import (
	"strings"
	"testing"
)

// Card.StrictlySelf is Forge's mirror spelling of Card.Self: the candidate is
// the source object itself, read live. kgen reads it as the same fSelf flag.
func TestStrictlySelfFilterWord(t *testing.T) {
	f, err := parseFilter("Card.StrictlySelf", func(string) bool { return false }, func(string) bool { return false })
	if err != nil {
		t.Fatalf("Card.StrictlySelf refused: %v", err)
	}
	if len(f.Clauses) != 1 || f.Clauses[0].Flags&fSelf == 0 || f.Clauses[0].Types != 0 {
		t.Fatalf("Card.StrictlySelf lowered to %+v, want one fSelf clause", f.Clauses)
	}
}

// A source-identity presence read is an intervening-if, re-checked when the
// trigger resolves (CR 603.4), so it is kept as the source's live zone
// (gorge's Self is the stable object id, which survives a zone change) rather
// than the ordinary source count.
func TestStrictlySelfPresenceKeepsTheInterveningIf(t *testing.T) {
	grave := lowerScriptCard(t, "Probe Graveyard Presence", "Name:Probe Graveyard Presence\nManaCost:2 B\nTypes:Creature\nPT:2/2\n"+
		"T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | TriggerZones$ Graveyard | IsPresent$ Card.StrictlySelf | PresentZone$ Graveyard | PresentPlayer$ You | Execute$ TrigGain | TriggerDescription$ probe\n"+
		"SVar:TrigGain:DB$ GainLife | LifeAmount$ 1\n")
	if len(grave.Refusals) != 0 {
		t.Fatalf("graveyard presence refused: %v", grave.Refusals)
	}
	if len(grave.Abilities) == 0 || grave.Abilities[0].Trigger == nil {
		t.Fatalf("graveyard presence lowered to no trigger: %+v", grave.Abilities)
	}
	if c := grave.Abilities[0].Trigger.Cond; c.Kind != cSourceZone || c.Zone != zGraveyard {
		t.Fatalf("graveyard presence lowered to %+v, want SourceZone(Graveyard)", c)
	}

	command := lowerScriptCard(t, "Probe Command Presence", "Name:Probe Command Presence\nManaCost:1 W\nTypes:Creature\nPT:2/2\n"+
		"T:Mode$ Phase | Phase$ BeginCombat | ValidPlayer$ You | TriggerZones$ Command | IsPresent$ Card.StrictlySelf | PresentZone$ Command | Execute$ ProbePump | TriggerDescription$ probe\n"+
		"SVar:ProbePump:DB$ Pump | Defined$ Self | NumAtt$ +7 | NumDef$ +4\n")
	if len(command.Refusals) != 0 {
		t.Fatalf("command presence refused: %v", command.Refusals)
	}
	if len(command.Abilities) == 0 || command.Abilities[0].Trigger == nil {
		t.Fatalf("command presence lowered to no trigger: %+v", command.Abilities)
	}
	if c := command.Abilities[0].Trigger.Cond; c.Kind != cSourceZone || c.Zone != zCommand {
		t.Fatalf("command presence lowered to %+v, want SourceZone(Command)", c)
	}

	battlefield := lowerScriptCard(t, "Probe Battlefield Presence", "Name:Probe Battlefield Presence\nManaCost:1 W\nTypes:Creature\nPT:2/2\n"+
		"T:Mode$ Phase | Phase$ BeginCombat | ValidPlayer$ You | IsPresent$ Card.StrictlySelf | PresentZone$ Battlefield | Execute$ ProbePump | TriggerDescription$ probe\n"+
		"SVar:ProbePump:DB$ Pump | Defined$ Self | NumAtt$ +7 | NumDef$ +4\n")
	if len(battlefield.Refusals) != 0 {
		t.Fatalf("battlefield presence refused: %v", battlefield.Refusals)
	}
	if len(battlefield.Abilities) == 0 || battlefield.Abilities[0].Trigger == nil {
		t.Fatalf("battlefield presence lowered to no trigger: %+v", battlefield.Abilities)
	}
	if c := battlefield.Abilities[0].Trigger.Cond; c.Kind != cSourceZone || c.Zone != zBattlefield {
		t.Fatalf("battlefield presence lowered to %+v, want SourceZone(Battlefield)", c)
	}

	// Any/All must fail closed: gorge's presentZoneFromParam refuses them, and
	// the kernel's zone is ZoneNone, which a source in no zone would satisfy.
	for _, zone := range []string{"Any", "All"} {
		anyZone := lowerScriptCard(t, "Probe Any Presence", "Name:Probe Any Presence\nManaCost:1 W\nTypes:Creature\nPT:2/2\n"+
			"T:Mode$ Phase | Phase$ BeginCombat | ValidPlayer$ You | IsPresent$ Card.StrictlySelf | PresentZone$ "+zone+" | Execute$ ProbePump | TriggerDescription$ probe\n"+
			"SVar:ProbePump:DB$ Pump | Defined$ Self | NumAtt$ +7 | NumDef$ +4\n")
		if len(anyZone.Refusals) == 0 {
			t.Errorf("PresentZone$ %s of a source-identity read was lowered, want a refusal", zone)
		}
	}
}

// TriggeredAttacker$CardPower is the triggering attacker's live power (Arahbo,
// Roar of the World's X is its power), lowered through the trigger object ref.
func TestTriggeredAttackerCardPowerLowers(t *testing.T) {
	lc := lowerCorpusCard(t, "Arahbo, Roar of the World")
	for _, rf := range lc.Refusals {
		t.Errorf("Arahbo refused: %s", rf.Reason)
	}
	found := false
	for _, a := range lc.Abilities {
		for _, in := range a.Prog {
			if in.A.Kind == vPowerOf && in.A.Ref.Kind == rTriggered {
				found = true
			}
			if in.B.Kind == vPowerOf && in.B.Ref.Kind == rTriggered {
				found = true
			}
		}
	}
	if !found {
		var b strings.Builder
		for _, a := range lc.Abilities {
			for _, in := range a.Prog {
				b.WriteString(in.String())
				b.WriteByte('\n')
			}
		}
		t.Fatalf("Arahbo's TriggeredAttacker$CardPower is not a power-of-trigger value:\n%s", b.String())
	}
}
