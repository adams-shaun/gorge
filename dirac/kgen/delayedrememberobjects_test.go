package kgen

import (
	"strings"
	"testing"
)

// delayedRememberObjectsCarriers: delayed registrations whose RememberObjects$
// names one object the lowering can resolve. Each lowers with no refusal and
// its lowered IR registers the object the form names.
var delayedRememberObjectsCarriers = map[string]struct{ form, lir string }{
	"U.S.S. Farragut, Bold Explorer": {
		"a triggered ability's Remembered set that its own hand-to-battlefield choice of one card wrote",
		"DelayedTrigger obj=Remembered C=1 CS=NextEndStep"},
	"The Guardian of Forever": {
		"the one card an activation cost sacrificed",
		"DelayedTrigger obj=Sacrificed C=1 CS=NextEndStep"},
}

func TestDelayedRememberObjects(t *testing.T) {
	var names []string
	for n := range delayedRememberObjectsCarriers {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		want, ok := delayedRememberObjectsCarriers[c.Name]
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
	if seen != len(delayedRememberObjectsCarriers) {
		t.Errorf("lowered %d of %d carriers", seen, len(delayedRememberObjectsCarriers))
	}
}

// dugSetProbe is Dragonhawk's shape (an enters trigger that digs, remembers the
// dug cards and delays an end-step damage trigger over them) with the delayed
// body's damage amount given: only the exact count-read body lowers.
func dugSetProbe(numDmg, y string) string {
	return strings.Join([]string{
		"Name:Probe",
		"ManaCost:3 R R",
		"Types:Creature Dragon",
		"PT:5/5",
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigDig | TriggerDescription$ probe",
		"SVar:TrigDig:DB$ Dig | Defined$ You | DigNum$ 2 | ChangeNum$ All | DestinationZone$ Exile | RememberChanged$ True | SubAbility$ DBDelay",
		"SVar:DBDelay:DB$ DelayedTrigger | Mode$ Phase | Phase$ End of Turn | ValidPlayer$ You | RememberObjects$ Remembered | Execute$ TrigDamage | SubAbility$ DBClear",
		"SVar:TrigDamage:DB$ DamageAll | ValidPlayers$ Player.Opponent | NumDmg$ " + numDmg,
		"SVar:DBClear:DB$ Cleanup | ClearRemembered$ True",
		"SVar:Y:" + y,
		"Oracle:probe",
	}, "\n")
}

// TestDelayedRememberObjectsRefusesSets: a dig's several remembered cards are
// carried only for the one body the count read covers (Dragonhawk, Fate's
// Tempest, pinned by TestDragonhawkLowering): the same shape with another
// count (no doubling), another amount (a fixed 3) or another reader keeps the
// refusal by name. The step-keyed arms keep refusing the set too
// (delayedstepremember_test.go).
func TestDelayedRememberObjectsRefusesSets(t *testing.T) {
	const want = "a set of several objects"
	for name, c := range map[string]struct{ numDmg, y string }{
		"the count without the doubling": {"Y", "Count$ValidExile Card.IsTriggerRemembered"},
		"a fixed amount":                 {"3", "Count$ValidExile Card.IsTriggerRemembered/Twice"},
		"another zone":                   {"Y", "Count$ValidGraveyard Card.IsTriggerRemembered/Twice"},
	} {
		got := abReasons(abLower(t, dugSetProbe(c.numDmg, c.y), false))
		if !strings.Contains(got, want) {
			t.Errorf("%s: refusals %q, want %q", name, got, want)
		}
	}
	// and the exact shape lowers, so the refusals above are the shape guard's
	if got := abReasons(abLower(t, dugSetProbe("Y", "Count$ValidExile Card.IsTriggerRemembered/Twice"), false)); got != "" {
		t.Errorf("the count-read shape refused: %q", got)
	}
}
