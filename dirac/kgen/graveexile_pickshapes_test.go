package kgen

import (
	"strings"
	"testing"
)

// graveExilePickProbe is an enters-trigger creature that mills, then poses a
// Graveyard,Exile pick over the milled cards; PICK is the pick's own params.
const graveExilePickProbe = `Name:Grave Exile Probe
ManaCost:1 B
Types:Creature Elf
PT:1/1
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ PrMill | TriggerDescription$ probe
SVar:PrMill:DB$ Mill | NumCards$ 4 | RememberMilled$ True | SubAbility$ PrPick
SVar:PrPick:DB$ ChangeZone | Hidden$ True | Origin$ Graveyard,Exile | PICK | ChangeType$ Creature.IsRemembered | SubAbility$ PrClean
SVar:PrClean:DB$ Cleanup | ClearRemembered$ True
SVar:X:Count$Valid Elf
Oracle:probe
`

func graveExileProbe(pick string) string {
	return strings.Replace(graveExilePickProbe, "PICK", pick, 1)
}

// chooseMoves returns the card's built-form ChooseMove instructions.
func chooseMoves(t *testing.T, name string) []*lInstr {
	t.Helper()
	c := lowerCorpusCard(t, name)
	for _, rf := range c.Refusals {
		t.Errorf("%s refused: %s", name, rf.Reason)
	}
	var out []*lInstr
	for _, a := range c.Abilities {
		for i := range a.Prog {
			if a.Prog[i].Op == "ChooseMove" {
				out = append(out, &a.Prog[i])
			}
		}
	}
	return out
}

// TestGraveExilePickToLibraryTop: a Graveyard,Exile pick whose destination is
// the top of the library lowers (Lluwen, Imperfect Naturalist) as the same
// ChooseMove with a library destination; any other position, or no position,
// refuses by name.
func TestGraveExilePickToLibraryTop(t *testing.T) {
	cms := chooseMoves(t, "Lluwen, Imperfect Naturalist")
	if len(cms) != 1 {
		t.Fatalf("Lluwen: %d ChooseMove, want 1", len(cms))
	}
	cm := cms[0]
	if cm.C != zLibrary || !strings.Contains(cm.CS, "from=Graveyard,Exile") {
		t.Errorf("Lluwen: ChooseMove C=%d CS=%q, want a Graveyard,Exile pick into the library", cm.C, cm.CS)
	}
	if strings.Contains(cm.CS, "Mandatory") || strings.Contains(cm.CS, "Confirm") {
		t.Errorf("Lluwen: CS=%q, want a declinable up-to pick with no confirmation", cm.CS)
	}
	if rf := lowerScript(t, "Grave Exile Probe", graveExileProbe("Destination$ Library | LibraryPosition$ 0")); len(rf) != 0 {
		t.Fatalf("the top-of-library probe refused: %v", rf)
	}
	for _, tc := range []struct{ pick, want string }{
		{"Destination$ Library | LibraryPosition$ -1", "LibraryPosition"},
		{"Destination$ Library | LibraryPosition$ 2", "LibraryPosition"},
		{"Destination$ Library", "LibraryPosition"},
		{"Destination$ Graveyard", "Graveyard,Exile to"},
	} {
		rf := lowerScript(t, "Grave Exile Probe", graveExileProbe(tc.pick))
		if len(rf) == 0 || !strings.Contains(strings.Join(rf, "|"), tc.want) {
			t.Errorf("%q: refusals %v, want one naming %q", tc.pick, rf, tc.want)
		}
	}
}

// TestGraveExilePickTriggerCount: the pick's up-to count may be the
// triggering event's amount (Nashi, Searcher in the Dark); a count from any
// other variable still refuses by name.
func TestGraveExilePickTriggerCount(t *testing.T) {
	cms := chooseMoves(t, "Nashi, Searcher in the Dark")
	if len(cms) != 1 {
		t.Fatalf("Nashi: %d ChooseMove, want 1", len(cms))
	}
	if cms[0].A.Kind != vTriggerAmount {
		t.Errorf("Nashi: ChooseMove count kind %v, want the trigger amount", cms[0].A.Kind)
	}
	if rf := lowerScript(t, "Grave Exile Probe", graveExileProbe("Destination$ Hand | ChangeNum$ 2")); len(rf) != 0 {
		t.Fatalf("the constant-count probe refused: %v", rf)
	}
	rf := lowerScript(t, "Grave Exile Probe", graveExileProbe("Destination$ Hand | ChangeNum$ X"))
	if len(rf) == 0 || !strings.Contains(strings.Join(rf, "|"), "Graveyard,Exile pick ChangeNum$") {
		t.Errorf("a Count$ count: refusals %v, want the ChangeNum$ refusal", rf)
	}
}

// TestGraveExileTargetedReturn: a targeted return over Graveyard,Exile whose
// target alternatives name different zones (Sorceress's Schemes) needs a
// target slot spanning two zones. The kernel's TgtCard slot holds one zone
// (legal.go targetOK: ob.zone == the filter's zone), so the card is refused;
// this pins that stop until the slot grows a second zone.
func TestGraveExileTargetedReturn(t *testing.T) {
	c := lowerCorpusCard(t, "Sorceress's Schemes")
	if len(c.Refusals) == 0 {
		t.Fatal("Sorceress's Schemes lowers: drop this stop and add a lockstep repro")
	}
	if !strings.Contains(c.Refusals[0].Reason, "Origin$ Graveyard,Exile") {
		t.Errorf("first refusal %q, want the Graveyard,Exile origin", c.Refusals[0].Reason)
	}
}
