package kgen

import (
	"strings"
	"testing"
)

// sweepLKIProbe is Planar Guide's shape: an ability whose ChangeZoneAll
// RememberChanged$ sweep writes the whole remembered set, then registers an
// end-step delayed return of it (RememberObjects$ RememberedLKI). body is the
// delayed ability.
func sweepLKIProbe(body string) string {
	return strings.Join([]string{
		"Name:Probe",
		"ManaCost:W",
		"Types:Creature Human Cleric",
		"PT:1/1",
		"A:AB$ ChangeZoneAll | Cost$ 3 W Exile<1/CARDNAME> | ChangeType$ Creature | Origin$ Battlefield | Destination$ Exile | RememberChanged$ True | SubAbility$ DBDel | SpellDescription$ probe",
		"SVar:DBDel:DB$ DelayedTrigger | Mode$ Phase | Phase$ End of Turn | Execute$ TrigReturn | RememberObjects$ RememberedLKI",
		"SVar:TrigReturn:" + body,
		"Oracle:probe",
	}, "\n")
}

// TestLowerDelayedLkiSweep: Planar Guide's RememberedLKI delayed trigger over a
// ChangeZoneAll Remember sweep lowers, registering one DpMany program whose
// body is the many-object return (the kernel's one-object delayed record would
// return only the sweep's first creature); each out-of-shape body refuses by
// name, and a second remembering write keeps the one-object record.
func TestLowerDelayedLkiSweep(t *testing.T) {
	cs := lowerNamed(t, []string{"Planar Guide"})
	if len(cs) != 1 {
		t.Fatalf("lowered %d cards, want 1", len(cs))
	}
	for _, r := range cs[0].Refusals {
		t.Errorf("Planar Guide: refused: %s", r.Reason)
	}
	var sb strings.Builder
	dumpLIR(&sb, cs[0])
	lir := sb.String()
	for _, want := range []string{"ChangeZoneAll", "from=Battlefield|Remember", "DelayedTrigger", "B=3", "Return obj=DelayedRemembered", "DelayedMany"} {
		if !strings.Contains(lir, want) {
			t.Errorf("Planar Guide: lowered IR lacks %q:\n%s", want, lir)
		}
	}

	const name = "delayed trigger RememberedLKI over a ChangeZoneAll sweep"
	if got := abReasons(abLower(t, sweepLKIProbe(delayedLKIBody), false)); got != "" {
		t.Errorf("the in-shape probe refused: %q", got)
	}
	for label, want := range map[string]string{
		"another zone":   strings.Replace(delayedLKIBody, "Origin$ Exile", "Origin$ Graveyard", 1),
		"another body":   "DB$ ChangeZone | Origin$ Exile | Destination$ Hand | Defined$ DelayTriggerRememberedLKI",
		"body parameter": delayedLKIBody + " | Tapped$ True",
	} {
		got := abReasons(abLower(t, sweepLKIProbe(want), false))
		if !strings.Contains(got, name) || !strings.Contains(got, "the body is not one ChangeZone Exile->Battlefield") {
			t.Errorf("%s: refusals %q, want %q and the body reason", label, got, name)
		}
	}
	// a second remembering write: the set is no longer the sweep alone, and
	// the chain keeps the one-object record it had before (no refusal)
	twoWrites := strings.Replace(sweepLKIProbe(delayedLKIBody), "| SubAbility$ DBDel |",
		"| SubAbility$ DBMore |", 1) + "\nSVar:DBMore:DB$ ChangeZone | Defined$ Self | Origin$ Battlefield | Destination$ Exile | RememberChanged$ True | SubAbility$ DBDel"
	c := abLower(t, twoWrites, false)
	if got := abReasons(c); got != "" {
		t.Errorf("a second remembering write: refused %q", got)
	}
	var wb strings.Builder
	dumpLIR(&wb, c)
	if wl := wb.String(); strings.Contains(wl, "B=3") {
		t.Errorf("a second remembering write: lowered to the many-object program:\n%s", wl)
	}
}
