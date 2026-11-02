package kgen

import (
	"strings"
	"testing"
)

// klkidelayedCarriers is one card per end-step DelayedTrigger form the
// trigger-card RememberObjects$ lowering admits (lower_delayedobjects.go): each
// must lower with no refusal, and its lowered IR must contain every
// fragment.
var klkidelayedCarriers = map[string]struct {
	form string
	lir  []string
}{
	"Esoteric Duplicator": {"RememberObjects$ TriggeredCardLKICopy + CopyPermanent Defined$ DelayTriggerRememberedLKI",
		[]string{"DelayedTrigger obj=Triggered C=2 CS=NextEndStep", "CopyAsToken obj=DelayedRemembered A=1"}},
	"Biolume Egg": {"RememberObjects$ TriggeredCard + ChangeZone Defined$ DelayTriggerRemembered Transformed$ True",
		[]string{"DelayedTrigger obj=Triggered C=2 CS=NextEndStep", "Return obj=DelayedRemembered C=1 CS=Transformed|from=Graveyard"}},
	"Resurrection Orb": {"RememberObjects$ TriggeredCard + ChangeZone Defined$ DelayTriggerRememberedLKI Origin$ Graveyard",
		[]string{"DelayedTrigger obj=Triggered C=2 CS=NextEndStep", "Return obj=DelayedRemembered C=1 CS=from=Graveyard"}},
	"Ochre Jelly": {"RememberObjects$ TriggeredCardLKICopy + CopyPermanent WithCountersType$ P1P1 WithCountersAmount$ TriggerRemembered$CardCounters.P1P1/HalfDown",
		[]string{"DelayedTrigger obj=Triggered C=1 CS=NextEndStep", "CopyAsTokenCtr obj=DelayedRemembered A=1 B=CountersOn(DelayedRemembered,P1P1)/2 CS=P1P1"}},
	"A-Ochre Jelly": {"the rebalanced Ochre Jelly (same script)",
		[]string{"DelayedTrigger obj=Triggered C=1 CS=NextEndStep", "CopyAsTokenCtr obj=DelayedRemembered A=1 B=CountersOn(DelayedRemembered,P1P1)/2 CS=P1P1"}},
	"Littjara Mirrorlake": {"CopyPermanent ValidTgts$ with the constant-1 WithCountersType$ rider",
		[]string{"CopyAsTokenCtr obj=Target0 A=1 B=1 CS=P1P1"}},
	"Loyal Cathar": {"RememberObjects$ TriggeredNewCardLKICopy + Transformed$ GainControl$ of DelayTriggerRememberedLKI",
		[]string{"DelayedTrigger obj=TriggeredNew C=1 CS=NextEndStep", "Return obj=DelayedRemembered C=1 CS=UnderYourControl|Transformed|from=Graveyard"}},
}

func TestGenDelayedLKICopy(t *testing.T) {
	var names []string
	for n := range klkidelayedCarriers {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		want, ok := klkidelayedCarriers[c.Name]
		if !ok {
			continue
		}
		seen++
		for _, r := range c.Refusals {
			t.Errorf("%s (%s): refused: %s", c.Name, want.form, r.Reason)
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		for _, frag := range want.lir {
			if !strings.Contains(sb.String(), frag) {
				t.Errorf("%s (%s): lowered IR lacks %q:\n%s", c.Name, want.form, frag, sb.String())
			}
		}
	}
	if seen != len(klkidelayedCarriers) {
		t.Errorf("lowered %d of %d carriers", seen, len(klkidelayedCarriers))
	}
}

// TestGenCopyCounterRiderRefusals documents the boundary of the
// CopyPermanent WithCountersType$ rider: the kernel op (ExtCopyTokenCtr)
// carries only the source, the amount and the kind, so a combination with a
// copy exception, a copy flag or NumCopies$ is refused by name, never lowered
// with the counters dropped; a counter kind the kernel has no name for is
// refused too, and TriggerRemembered$CardCounters outside a delayed trigger
// that remembered the trigger's own card is not read.
func TestGenCopyCounterRiderRefusals(t *testing.T) {
	root := "SP$ Draw | Cost$ B"
	const combo = "CopyPermanent WithCountersType$ beside copy exceptions, AtEOT$/PumpKeywords$/RememberTokens$ or NumCopies$"
	for _, c := range []struct{ name, sub, want string }{
		{"num copies", "DB$ CopyPermanent | Defined$ Self | NumCopies$ 2 | WithCountersType$ P1P1", combo},
		{"at eot exile", "DB$ CopyPermanent | Defined$ Self | AtEOT$ Exile | WithCountersType$ P1P1", combo},
		{"remember tokens", "DB$ CopyPermanent | Defined$ Self | RememberTokens$ True | WithCountersType$ P1P1", combo},
		{"copy exception", "DB$ CopyPermanent | Defined$ Self | NonLegendary$ True | WithCountersType$ P1P1", combo},
		{"unknown kind", "DB$ CopyPermanent | Defined$ Self | WithCountersType$ NOSUCHKIND", `CopyPermanent WithCountersType$ "NOSUCHKIND"`},
		{"amount without kind", "DB$ CopyPermanent | Defined$ Self | WithCountersAmount$ 2", "unread params [WithCountersAmount]"},
		{"live counters read outside a delayed trigger", "DB$ CopyPermanent | Defined$ Self | WithCountersType$ P1P1 | WithCountersAmount$ TriggerRemembered$CardCounters.P1P1/HalfDown", "TriggerRemembered$CardCounters.P1P1 outside a delayed trigger remembering the trigger's own card"},
	} {
		t.Run(c.name, func(t *testing.T) {
			kdefWant(t, kdefRefusals(t, root, c.sub), c.want)
		})
	}
}
