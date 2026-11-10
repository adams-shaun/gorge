package kgen

import (
	"strings"
	"testing"
)

// delayedStepRememberCarriers: the step-keyed delayed arms share the end-step
// arm's RememberObjects$ reading. Each card lowers with no refusal and its
// lowered IR registers the delayed program at the named step.
var delayedStepRememberCarriers = map[string]struct{ form, lir string }{
	"Casey Jones, Vigilante": {"upkeep RememberObjects$ Targeted in a chain with no target slot (an empty set)", "DelayedTrigger C=1 CS=YourNextUpkeep"},
	"Feral Encounter":        {"beginning of combat without RememberObjects$ (a body that reads none)", "DelayedTrigger C=1 CS=NextBeginCombat"},
}

func TestDelayedStepRemember(t *testing.T) {
	var names []string
	for n := range delayedStepRememberCarriers {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		want, ok := delayedStepRememberCarriers[c.Name]
		if !ok {
			continue
		}
		seen++
		for _, r := range c.Refusals {
			t.Errorf("%s (%s): refused: %s", c.Name, want.form, r.Reason)
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		if !strings.Contains(sb.String(), want.lir) {
			t.Errorf("%s (%s): lowered IR lacks %q:\n%s", c.Name, want.form, want.lir, sb.String())
		}
	}
	if seen != len(delayedStepRememberCarriers) {
		t.Errorf("lowered %d of %d carriers", seen, len(delayedStepRememberCarriers))
	}
}

// delayedStepProbe is a creature whose enters trigger targets two creatures
// and registers a delayed program at <phase> remembering <ro>.
func delayedStepProbe(phase, ro string) string {
	return strings.Join([]string{
		"Name:Probe",
		"ManaCost:1 R",
		"Types:Creature Human",
		"PT:2/2",
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ ProbeTap | TriggerDescription$ probe",
		"SVar:ProbeTap:DB$ Tap | TargetMax$ 2 | TargetMin$ 0 | ValidTgts$ Creature | SubAbility$ ProbeDelay",
		"SVar:ProbeDelay:DB$ DelayedTrigger | Mode$ Phase | Phase$ " + phase + " | ThisTurn$ True | RememberObjects$ " + ro + " | Execute$ ProbeUntap",
		"SVar:ProbeUntap:DB$ Untap | Defined$ DelayTriggerRememberedLKI",
		"Oracle:probe",
	}, "\n")
}

// TestDelayedStepRememberFortuneLowers: Fortune, Loyal Steed's end of combat
// body remembers what it exiles (itself and the chosen saddler) and returns
// exactly that. The registration is capture-only (the attack trigger's
// capture is no memory of the delayed trigger), so the body reads a
// body-local set the kernel's program carries: it lowers, each referent of
// `Defined$ Self & ChosenCard` its own remembering exile, then the return of
// the remembered set.
func TestDelayedStepRememberFortuneLowers(t *testing.T) {
	cs := lowerNamed(t, []string{"Fortune, Loyal Steed"})
	if len(cs) != 1 {
		t.Fatalf("lowered %d cards, want 1", len(cs))
	}
	for _, r := range cs[0].Refusals {
		t.Errorf("refused: %s", r.Reason)
	}
	var sb strings.Builder
	dumpLIR(&sb, cs[0])
	for _, want := range []string{
		"DelayedTrigger C=3 CS=NextEndCombat",
		"Exile obj=Self CS=Remember|from=Battlefield",
		"Exile obj=Chosen CS=Remember|from=Battlefield",
		"Return obj=Remembered C=1",
	} {
		if !strings.Contains(sb.String(), want) {
			t.Errorf("lowered IR lacks %q:\n%s", want, sb.String())
		}
	}
}

// delayedBodyProbe is a creature whose attack trigger registers an
// end-of-combat delayed program with no RememberObjects$ and the given
// two-link body (head, then a sub-ability).
func delayedBodyProbe(head, sub string) string {
	return strings.Join([]string{
		"Name:Probe",
		"ManaCost:1 R",
		"Types:Creature Human",
		"PT:2/2",
		"T:Mode$ Attacks | ValidCard$ Card.Self | Execute$ ProbeDelay | TriggerDescription$ probe",
		"SVar:ProbeDelay:DB$ DelayedTrigger | Mode$ Phase | Phase$ EndCombat | Execute$ ProbeHead",
		"SVar:ProbeHead:" + head + " | SubAbility$ ProbeSub",
		"SVar:ProbeSub:" + sub,
		"Oracle:probe",
	}, "\n")
}

// TestDelayedStepRememberReadBeforeWriteRefused: the capture-only reading
// holds only for a body whose every Remembered read follows its own write; a
// body that reads Remembered first still refuses.
func TestDelayedStepRememberReadBeforeWriteRefused(t *testing.T) {
	const want = "delayed end-of-combat trigger body reads Remembered"
	for name, c := range map[string]struct{ head, sub string }{
		"read first":        {"DB$ ChangeZone | Defined$ Remembered | Origin$ All | Destination$ Battlefield", "DB$ ChangeZone | Defined$ Self | Origin$ Battlefield | Destination$ Exile | RememberChanged$ True"},
		"read beside write": {"DB$ ChangeZone | Defined$ Self | Origin$ Battlefield | Destination$ Exile | RememberChanged$ True | ChangeNum$ Count$Remembered", "DB$ Cleanup | ClearRemembered$ True"},
	} {
		got := abReasons(abLower(t, delayedBodyProbe(c.head, c.sub), false))
		if !strings.Contains(got, want) {
			t.Errorf("%s: refusals %q, want %q", name, got, want)
		}
	}
}

// TestDelayedStepRememberRefusesSets: the delayed record carries one object,
// so a remembered set or a target slot of two refuses by name at each step.
func TestDelayedStepRememberRefusesSets(t *testing.T) {
	for _, c := range []struct{ phase, ro, want string }{
		{"Upkeep", "Remembered", `delayed upkeep trigger RememberObjects "Remembered": a set of several objects`},
		{"BeginCombat", "Remembered", `delayed beginning-of-combat trigger RememberObjects "Remembered": a set of several objects`},
		{"EndCombat", "RememberedLKI", `delayed end-of-combat trigger RememberObjects "RememberedLKI": a set of several objects`},
		{"Upkeep", "Targeted", `delayed trigger RememberObjects "Targeted": target slot 0`},
		{"BeginCombat", "Targeted", `delayed trigger RememberObjects "Targeted": target slot 0`},
		{"EndCombat", "Targeted", `delayed trigger RememberObjects "Targeted": target slot 0`},
	} {
		got := abReasons(abLower(t, delayedStepProbe(c.phase, c.ro), false))
		if !strings.Contains(got, c.want) {
			t.Errorf("%s %s: refusals %q, want %q", c.phase, c.ro, got, c.want)
		}
	}
}
