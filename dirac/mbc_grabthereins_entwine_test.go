package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestEntwineBeyondTheIronworksBoundIsNotOffered: Grab the Reins' entwined cast
// is {3}{R}+{2}{R}: seven mana with two red. Beside one Mountain, two Plains,
// Patriar's Seal and Krark-Clan Ironworks, gorge's potential bound reaches six
// (the Ironworks' fixed {C}{C} counts once), so only the plain cast is offered.
func TestEntwineBeyondTheIronworksBoundIsNotOffered(t *testing.T) {
	// cardsettest.Set, not cardsettest.Synth: only the lockstep oracle and kgen
	// read raw tables (internal/archtest TestOnlyTheOracleReadsRawTables), and
	// every card this scenario needs is lowered in the mbc table.
	set := cardsettest.Set(t, "mbc")
	var deck []cardset.Entry
	for _, n := range []string{"Grab the Reins", "Krark-Clan Ironworks", "Patriar's Seal", "Crimson Fleet Commodore", "Mountain", "Plains"} {
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTable(t, tbl, Config{NoElide: true, GorgeLowering: true, GorgeAutopaySurface: true})
	for _, n := range []string{"Plains", "Plains", "Mountain", "Patriar's Seal", "Krark-Clan Ironworks", "Crimson Fleet Commodore"} {
		sc.perm(id[n], 0)
	}
	reins := sc.hand(id["Grab the Reins"], 0)
	d := sc.s.Step()
	casts := 0
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(reins) {
			casts++
		}
	}
	if casts != 1 {
		t.Fatalf("Grab the Reins offered %d casts, want only the plain one (the entwined seven mana is past the bound)", casts)
	}
}
