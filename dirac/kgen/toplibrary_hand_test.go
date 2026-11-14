package kgen

import (
	"strings"
	"testing"
)

// The absent-Optional$ TopOfLibrary moves (lower_toplibrary.go): Vivien's
// Grizzly's PeekAndReveal else-branch puts the peeked card into the hand, or
// onto the library's bottom, with no yes / no ask -- the node's Condition$
// gate decides which arm runs (gorge moveDefinedLibraryObjects with
// cz.OptionalTrue false: no ask, and an empty library's fetch moves nothing).

const vivienProbe = `Name:Top Lib Probe
ManaCost:2 G
Types:Creature Bear Spirit
PT:2/3
A:AB$ PeekAndReveal | Cost$ 3 G | PeekAmount$ 1 | RevealValid$ Creature,Planeswalker | RevealOptional$ True | RememberRevealed$ True | SubAbility$ DBTake | SpellDescription$ probe
SVar:DBTake:DB$ ChangeZone | Defined$ TopOfLibrary | Origin$ Library | Destination$ Hand | ConditionDefined$ Remembered | ConditionPresent$ Creature,Planeswalker | ConditionCompare$ EQ1 | SubAbility$ DBTail
SVar:DBTail:DB$ ChangeZone | Defined$ TopOfLibrary | Origin$ Library | Destination$ Library | ConditionDefined$ Remembered | ConditionPresent$ Creature,Planeswalker | ConditionCompare$ EQ0 | LibraryPosition$ -1 | SubAbility$ DBWipe
SVar:DBWipe:DB$ Cleanup | ClearRemembered$ True
Oracle:probe
`

func TestTopLibraryNoOptionalLowers(t *testing.T) {
	if rs := lowerScript(t, "Top Lib Probe", vivienProbe); len(rs) != 0 {
		t.Fatalf("probe refused: %v", rs)
	}

	// Vivien's Grizzly lowers, to the condition-gated moves the kernel reads
	vg := lowerCorpusCard(t, "Vivien's Grizzly")
	for _, rf := range vg.Refusals {
		t.Errorf("Vivien's Grizzly refused: %s", rf.Reason)
	}
	var ops []string
	for _, a := range vg.Abilities {
		for i := range a.Prog {
			ops = append(ops, a.Prog[i].Op)
		}
	}
	// DBTake: the Condition$ gate, the empty-library skip, then the move;
	// DBTail: the EQ0 gate, the skip, the bottom move.
	want := strings.Join([]string{"SkipIf", "SkipIf", "Return", "SkipIf", "SkipIf", "ToLibrary"}, ",")
	if got := strings.Join(ops, ","); !strings.Contains(got, want) {
		t.Errorf("Vivien's Grizzly ops %s, want %s inside", got, want)
	}

	// each out-of-shape variant refuses by name
	if rs := lowerScript(t, "Top Lib Probe", strings.Replace(vivienProbe, "Origin$ Library | Destination$ Hand", "Origin$ Library | Destination$ Hand | Optional$ False", 1)); !anyRefusal(rs, "Optional$") {
		t.Errorf("Optional$ False: refusals %q, want one naming it", rs)
	}
	if rs := lowerScript(t, "Top Lib Probe", strings.Replace(vivienProbe, "Destination$ Hand", "Destination$ Battlefield", 1)); !anyRefusal(rs, "Destination$ other than") {
		t.Errorf("to Battlefield: refusals %q, want the destination refusal", rs)
	}
	if rs := lowerScript(t, "Top Lib Probe", strings.Replace(vivienProbe, "SubAbility$ DBTake", "SubAbility$ DBTake | Shuffle$ True", 1)); !anyRefusal(rs, "Shuffle") {
		t.Errorf("shuffled hand move: refusals %q, want Shuffle named", rs)
	}
}
