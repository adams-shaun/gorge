package kgen

import (
	"strings"
	"testing"
)

// delayedLKIProbe is a sorcery that exiles its targets (RememberChanged$) and
// registers an end-step delayed return of them (RememberObjects$
// RememberedLKI): tgt/min/max shape the slot, body the delayed ability.
func delayedLKIProbe(tgt, body string) string {
	return strings.Join([]string{
		"Name:Probe",
		"ManaCost:2 U",
		"Types:Instant",
		"A:SP$ ChangeZone | " + tgt + " | Origin$ Battlefield | Destination$ Exile | RememberChanged$ True | SubAbility$ DBDel | SpellDescription$ probe",
		"SVar:DBDel:DB$ DelayedTrigger | Mode$ Phase | Phase$ End of Turn | Execute$ TrigReturn | RememberObjects$ RememberedLKI | SubAbility$ DBCleanup",
		"SVar:TrigReturn:" + body,
		"SVar:DBCleanup:DB$ Cleanup | ClearRemembered$ True",
		"Oracle:probe",
	}, "\n")
}

const delayedLKIBody = "DB$ ChangeZone | Origin$ Exile | Destination$ Battlefield | Defined$ DelayTriggerRememberedLKI"

// TestLowerDelayedLkiMultitarget: Hide on the Ceiling (X targets) and Temporal
// Tribulation (two) lower, registering one DpMany program whose body is the
// many-object return; each out-of-shape variant refuses by name.
func TestLowerDelayedLkiMultitarget(t *testing.T) {
	cs := lowerNamed(t, []string{"Hide on the Ceiling", "Temporal Tribulation"})
	if len(cs) != 2 {
		t.Fatalf("lowered %d cards, want 2", len(cs))
	}
	for _, c := range cs {
		for _, r := range c.Refusals {
			t.Errorf("%s: refused: %s", c.Name, r.Reason)
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		for _, want := range []string{"DelayedTrigger", "B=3", "Return obj=DelayedRemembered", "DelayedMany"} {
			if !strings.Contains(sb.String(), want) {
				t.Errorf("%s: lowered IR lacks %q:\n%s", c.Name, want, sb.String())
			}
		}
	}

	const name = "delayed trigger RememberedLKI over a multi-object target slot"
	two := "ValidTgts$ Permanent.YouCtrl | TargetMin$ 2 | TargetMax$ 2"
	if got := abReasons(abLower(t, delayedLKIProbe(two, delayedLKIBody), false)); got != "" {
		t.Errorf("the in-shape probe refused: %q", got)
	}
	for label, c := range map[string]struct{ tgt, body, want string }{
		"another zone":   {two, strings.Replace(delayedLKIBody, "Origin$ Exile", "Origin$ Graveyard", 1), "the body is not one ChangeZone Exile->Battlefield"},
		"body parameter": {two, delayedLKIBody + " | Tapped$ True", "the body is not one ChangeZone Exile->Battlefield"},
		"another body":   {two, "DB$ ChangeZone | Origin$ Exile | Destination$ Hand | Defined$ DelayTriggerRememberedLKI", "the body is not one ChangeZone Exile->Battlefield"},
	} {
		got := abReasons(abLower(t, delayedLKIProbe(c.tgt, c.body), false))
		if !strings.Contains(got, name) || !strings.Contains(got, c.want) {
			t.Errorf("%s: refusals %q, want %q and %q", label, got, name, c.want)
		}
	}
	// a slot that exiles nothing it remembers is another shape
	noRemember := strings.Replace(delayedLKIProbe(two, delayedLKIBody), " | RememberChanged$ True", "", 1)
	if got := abReasons(abLower(t, noRemember, false)); !strings.Contains(got, name) || !strings.Contains(got, "the remembered set is not the target slot's exile") {
		t.Errorf("no RememberChanged$: refusals %q", got)
	}
}
