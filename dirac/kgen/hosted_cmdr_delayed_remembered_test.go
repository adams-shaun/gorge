package kgen

import (
	"strings"
	"testing"
)

// The hosted-Commander delayed-trigger shapes this ticket lowers:
//
//   - Gift of Immortality: a triggered ability's ChangeZone writes its own
//     remembered set (RememberChanged$ on the returned trigger card), then
//     registers a DelayedTrigger RememberObjects$ Remembered whose body returns
//     the Aura attached to DelayTriggerRememberedLKI.
//   - Semester's End: a multi-object Exile RememberChanged$ registers a
//     RememberObjects$ RememberedLKI delayed trigger whose Effect gives the
//     returned creatures +1/+1 counters and planeswalkers loyalty counters
//     before returning the set.
//   - Unexplained Absence: a multi-object Exile RememberLKI$ whose chain's
//     Cloak reads RememberedController; the set's controllers are walked once
//     each (RepeatRemembered).
//   - Promise of Loyalty: VOW's named counter (the counter cover moved).
//
// Each card must lower with no refusal and carry the instruction the kernel
// reads; a regression refuses at kgen, so this is the lowering half of the
// lockstep coverage.
func TestHostedCmdrDelayedRememberedLowering(t *testing.T) {
	t.Parallel()
	cards := map[string][]string{
		"Gift of Immortality": {
			"DelayedTrigger obj=Remembered C=2 CS=NextEndStep",
			"Return obj=Self C=1 CS=AttachTo|from=Graveyard obj2=DelayedRemembered",
		},
		"Semester's End": {
			"DelayedTrigger obj=Remembered B=3",
			"Return obj=DelayedRemembered C=1 CS=from=Exile|DelayedMany|DelayedCtrByType",
		},
		"Unexplained Absence": {
			"Exile obj=Target0 CS=Remember|LKI|from=Battlefield",
			"RepeatEachCard C=3 skip=1",
			"Cloak obj=Each A=1",
		},
		"Promise of Loyalty": {
			"PutCounter obj=Chosen A=1 B=2 CS=VOW",
			"CreateEmblem obj=You",
			"emblem[0] trigger=0 statics=1 forgets-on-last-VOW remembers=set",
		},
	}
	for name, want := range cards {
		c := lowerCorpusCard(t, name)
		for _, rf := range c.Refusals {
			t.Errorf("%s refused: %s", name, rf.Reason)
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		for _, frag := range want {
			if !strings.Contains(sb.String(), frag) {
				t.Errorf("%s: lowered IR lacks %q:\n%s", name, frag, sb.String())
			}
		}
	}
}
