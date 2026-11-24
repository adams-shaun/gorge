package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Fiend Hunter: when it enters you may exile another target creature; when
// it leaves the battlefield the exiled card returns under its owner's
// control. The return reads Defined$ ExiledWith (Forge 9db1009ef8 dropped the
// RememberChanged/DirectRemembered pair): the exile records the exiled-with
// link and the return reads it (dirac/zone_exiledwith.go, CardDef.ExiledWith). Murder
// and Shock make Fiend Hunter leave. The fixture pool is
// dirac/cardset/pools/fixtures/fiendhunter.

var fiendHunterDecks = []fixtureDecks{{
	{
		{Name: "Fiend Hunter", Count: 12}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Plains", Count: 16}, {Name: "Swamp", Count: 6},
	},
	{
		{Name: "Hill Giant", Count: 6}, {Name: "Grizzly Bears", Count: 6}, {Name: "Shock", Count: 6},
		{Name: "Murder", Count: 4}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 10},
	},
}}

var fiendHunterWatch = map[string]bool{"Fiend Hunter": true, "Murder": true, "Shock": true}

func TestFiendHunterLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstep(t, "fiendhunter", fiendHunterDecks, fiendHunterWatch, 120, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	if run.casts["Fiend Hunter"] < 60 {
		t.Errorf("only %d Fiend Hunter casts", run.casts["Fiend Hunter"])
	}
}

// TestFiendHunterMutation drops the exiled-with record, so the leaves
// trigger's return reads an empty set and the exiled card stays in exile:
// the lockstep must diverge (gorge returns it).
func TestFiendHunterMutation(t *testing.T) {
	run := runFixtureLockstep(t, "fiendhunter", fiendHunterDecks, fiendHunterWatch, 120, false, func(tbl *kop.Table) {
		found := false
		for i := range tbl.Cards {
			if cd := &tbl.Cards[i]; cd.Name == "Fiend Hunter" {
				if !cd.ExiledWith {
					t.Fatal("Fiend Hunter lowered without ExiledWith")
				}
				cd.ExiledWith, found = false, true
			}
		}
		if !found {
			t.Fatal("Fiend Hunter is not in the table")
		}
	})
	run.requireDivergence(t, "the leaves trigger returns nothing")
}
