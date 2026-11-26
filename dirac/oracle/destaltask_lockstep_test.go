package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Emeria Shepherd's landfall trigger (kgen lower_czaltdest.go's destination
// choice, kernel OpAskDestAlt): "return target nonland permanent card from
// your graveyard to your hand. If that land is a Plains, you may return that
// card to the battlefield instead". gorge (engine/effects/zone_change.go
// changeZoneAltDestination) asks changezone_dest_alt only when the entering
// land is a Plains; any other land takes the hand without a question. The
// pairs play both answers: Plains lands pose the ask and a random policy
// answers primary (hand) and alternate (battlefield) both ways, and the
// Forest drops in the same decks check that no ask is posed without a Plains.
func TestDestAltAskLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("emeriashepherd")
	if err != nil {
		t.Fatal(err)
	}
	entries := [2][]cardset.Entry{
		{{Name: "Emeria Shepherd", Count: 10}, {Name: "Grizzly Bears", Count: 8}, {Name: "Plains", Count: 12}, {Name: "Forest", Count: 10}},
		{{Name: "Murder", Count: 8}, {Name: "Grizzly Bears", Count: 8}, {Name: "Llanowar Elves", Count: 4}, {Name: "Swamp", Count: 10}, {Name: "Forest", Count: 10}},
	}
	decks := kcastDeck(t, reg, entries)
	tbl, _, _, err := set.Matchup(entries[0], entries[1])
	if err != nil {
		t.Fatal(err)
	}
	dest, _ := canon.ChooseCode("destination")
	var n [2]int // primary (hand), alternate (battlefield)
	stalls := 0
	for seed := uint64(1); seed <= 60; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"destaltask-a", "destaltask-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if res.Stall != "" && len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
		}
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
		}
		for _, a := range res.Actions {
			if a.Kind() == canon.ActChoose+canon.CanonKind(dest) {
				n[a.Mode()&1]++
			}
		}
	}
	t.Logf("primary %d, alternate %d, kernel stalls %d", n[0], n[1], stalls)
	if stalls != 0 {
		t.Errorf("%d kernel stalls", stalls)
	}
	if n[0] < 10 || n[1] < 10 {
		t.Errorf("answers reached primary %d / alternate %d, want at least 10 of each", n[0], n[1])
	}
}
