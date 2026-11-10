package kgen

import (
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const condNotPresentBase = `Name:P
ManaCost:1 G
Types:Creature Elf
PT:1/1
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigCZ | TriggerDescription$ x
SVar:TrigCZ:DB$ ChangeZone | Origin$ Graveyard | Destination$ Hand | ChangeType$ Creature.YouOwn | RememberChanged$ True | Hidden$ True | Optional$ True | SubAbility$ DBCounter
SVar:DBCounter:DB$ PutCounter | Defined$ Self | CounterType$ P1P1 | ConditionDefined$ Remembered | COND | SubAbility$ DBTidy
SVar:DBTidy:DB$ Cleanup | ClearRemembered$ True
Oracle:x
`

func lowerCondProbe(t *testing.T, cond string) *lCard {
	t.Helper()
	src := strings.Replace(condNotPresentBase, "COND", cond, 1)
	if src == condNotPresentBase {
		t.Fatal("the probe was not built")
	}
	card, diags := cards.ParseBytes("probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("script did not parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{byName: map[string]*cards.Card{"P": card}, nDecks: map[string]int{}, reg: testutil.CorpusRegistry(t)}
	return lowerCard(p, "P", card, false)
}

// countersCond returns the condition of the SkipIf that guards the lowered
// PutCounter (the instruction right before it).
func countersCond(t *testing.T, c *lCard) lCond {
	t.Helper()
	for _, a := range c.Abilities {
		for i, in := range a.Prog {
			if in.Op == "PutCounter" && i > 0 && a.Prog[i-1].Op == "SkipIf" {
				return a.Prog[i-1].Cond
			}
		}
	}
	t.Fatal("no SkipIf-guarded PutCounter lowered")
	return lCond{}
}

// TestConditionNotPresentAsPresent: ConditionNotPresent$ X lowers as
// ConditionPresent$ X | ConditionCompare$ EQ0, and refuses beside either.
func TestConditionNotPresentAsPresent(t *testing.T) {
	np := lowerCondProbe(t, "ConditionNotPresent$ Card")
	for _, rf := range np.Refusals {
		t.Errorf("ConditionNotPresent$ Card refused: %s", rf.Reason)
	}
	eq := lowerCondProbe(t, "ConditionPresent$ Card | ConditionCompare$ EQ0")
	for _, rf := range eq.Refusals {
		t.Fatalf("the EQ0 spelling refused: %s", rf.Reason)
	}
	got, want := countersCond(t, np), countersCond(t, eq)
	if want.Kind != cRefMatches || want.Op != opEQ || !reflect.DeepEqual(want.A, cst(0)) {
		t.Fatalf("the EQ0 spelling lowered to Op %v A %v, want EQ 0", want.Op, want.A)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ConditionNotPresent$ lowered to %+v, want %+v", got, want)
	}
	// the present spelling without the compare differs (GE 1): the comparison is real
	ge := lowerCondProbe(t, "ConditionPresent$ Card")
	if reflect.DeepEqual(countersCond(t, ge), want) {
		t.Error("precondition: the GE1 spelling equals the EQ0 one")
	}

	for _, extra := range []string{
		"ConditionNotPresent$ Card | ConditionCompare$ EQ0",
		"ConditionNotPresent$ Card | ConditionPresent$ Card",
	} {
		c := lowerCondProbe(t, extra)
		var seen bool
		for _, rf := range c.Refusals {
			seen = seen || strings.Contains(rf.Reason, "ConditionNotPresent$ beside")
		}
		if !seen {
			t.Errorf("%q: refusals %v, want ConditionNotPresent$ beside", extra, c.Refusals)
		}
	}
}
