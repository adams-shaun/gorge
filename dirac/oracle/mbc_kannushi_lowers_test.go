package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Protection from the colors of permanents you control (Empty-Shrine
// Kannushi's AddKeyword$ Protection from colorsYouCtrl; CR 702.16) in lockstep
// with gorge: the fixture pool (dirac/cardset/pools/fixtures/mbckannushi)
// plays the card beside red burn, red, green and white creatures. The Kannushi
// is white, so a white source never touches it, and a red source is withheld
// from it while its controller controls a red permanent (a Hill Giant): the
// targeting, block and damage options all follow (kop.StatProtYourColors,
// dirac/kw_protyours.go; gorge rules/protection.go protectedFrom).

var kannushiWatch = map[string]bool{"Empty-Shrine Kannushi": true}

var kannushiDecks = []fixtureDecks{{
	{
		{Name: "Empty-Shrine Kannushi", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Savannah Lions", Count: 3}, {Name: "Plains", Count: 10}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Savannah Lions", Count: 3}, {Name: "Elite Vanguard", Count: 3}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 6}, {Name: "Forest", Count: 5},
	},
}, {
	{
		{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Savannah Lions", Count: 3}, {Name: "Elite Vanguard", Count: 3}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 6}, {Name: "Forest", Count: 5},
	},
	{
		{Name: "Empty-Shrine Kannushi", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Savannah Lions", Count: 3}, {Name: "Plains", Count: 10}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
	},
}}

// dropProtYours turns every StatProtYourColors static of the table into a
// no-op, on copies (the lowering reverted); it fails the test when none exists.
func dropProtYours(t *testing.T) func(*kop.Table) {
	return func(tbl *kop.Table) {
		found := false
		for i := range tbl.Cards {
			cd := &tbl.Cards[i]
			cd.Abilities = append([]kop.Ability(nil), cd.Abilities...)
			for j := range cd.Abilities {
				if a := &cd.Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatProtYourColors {
					a.Static.Kind = kop.StatNone
					found = true
				}
			}
		}
		if !found {
			t.Fatal("precondition: the matchup table has no StatProtYourColors static")
		}
	}
}

func TestEmptyShrineKannushiLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstep(t, "mbckannushi", kannushiDecks, kannushiWatch, 160, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	if n := run.casts["Empty-Shrine Kannushi"]; n < 10 {
		t.Errorf("Empty-Shrine Kannushi cast %d times", n)
	}
	// The lowering reverted: gorge withholds the red burn and the white
	// creatures from the Kannushi, the kernel does not.
	mut := runFixtureLockstep(t, "mbckannushi", kannushiDecks, kannushiWatch, 160, false, dropProtYours(t))
	mut.requireDivergence(t, "the protection reverted")
}
