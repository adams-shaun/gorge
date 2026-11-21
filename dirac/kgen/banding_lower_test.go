package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestBatteringRamPumpLowers: Battering Ram's at-the-beginning-of-combat pump
// grants Banding (DB$ Pump | Defined$ Self | KW$ Banding | Duration$
// UntilEndOfCombat). KwMask is full (kop/keywords.go: "a new keyword is a
// KwID"), so the grant rides the override record's Banding bool beside
// CantAttack (kop.Override.Banding), never a mask bit; the record ends with
// the combat through the landed UntilEndCombat class (T14b-1). The card's
// Wall-destroy delayed trigger already lowered before it.
func TestBatteringRamPumpLowers(t *testing.T) {
	bcs := lowerPoolBuilt(t, []string{"Battering Ram"})
	if len(bcs) != 1 || bcs[0].name != "Battering Ram" {
		t.Fatalf("built %v, want one Battering Ram", bcs)
	}
	if n := len(bcs[0].refusals); n != 0 {
		t.Fatalf("Battering Ram refused (%d): %s", n, bcs[0].refusals[0].Reason)
	}
}

// TestBatteringRamPumpBuildsBandingRecord: the built table carries the grant
// as an override record with Banding and UntilEndCombat (the reader,
// dirac's hasBanding, sees nothing else), and no record grants Banding
// without the end-of-combat drop.
func TestBatteringRamPumpBuildsBandingRecord(t *testing.T) {
	set := cardsettest.Set(t, "pumpendcombat")
	tbl, _, _, err := set.Matchup(
		[]cardset.Entry{{Name: "Battering Ram", Count: 4}, {Name: "Swamp", Count: 16}},
		[]cardset.Entry{{Name: "Grizzly Bears", Count: 4}, {Name: "Mountain", Count: 16}})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for i, o := range tbl.Overrides {
		if !o.Banding {
			continue
		}
		n++
		if !o.UntilEndCombat {
			t.Errorf("override %d grants Banding without UntilEndCombat: %+v", i, o)
		}
	}
	if n != 1 {
		t.Fatalf("%d override records grant Banding, want the one Battering Ram pump", n)
	}
}
