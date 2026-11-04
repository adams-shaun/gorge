package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// A table with three spend restrictions (Villainous Hideout's Villain-only
// mana, Avengers Tower's Hero-only mana, Castle Doom's artifact-only mana)
// floats each kind's restricted mana as restricted: the kernel names every
// kind a Standard table holds, so no game stalls at capacity and every
// payment matches gorge's (CR 106.6).
func TestThreeSpendRestrictionsLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-msh")
	entries := []cardset.Entry{{Name: "Villainous Hideout", Count: 7}, {Name: "Avengers Tower", Count: 7},
		{Name: "Castle Doom", Count: 7}, {Name: "Surveillance Room", Count: 3}, {Name: "A.I.M. Synthoids", Count: 8}, {Name: "Killmonger, Scourge of Wakanda", Count: 6},
		{Name: "Stature, Size Shifter", Count: 6}, {Name: "Ka-Zar of the Savage Land", Count: 6},
		{Name: "Claim the Kingdom", Count: 4}, {Name: "Wakandan Royal Guard", Count: 4}}
	var deck []*cards.Card
	for _, e := range entries {
		cd, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		for i := 0; i < e.Count; i++ {
			deck = append(deck, cd)
		}
	}
	tbl, _, _, err := set.Matchup(entries, entries)
	if err != nil {
		t.Fatal(err)
	}
	for _, autopay := range []bool{false} { // the full surface: a player taps the restricted lands first
		// the seeds whose full-surface games float the third kind of mana
		for _, seed := range []uint64{6, 9, 10, 11, 15, 20, 23, 24} {
			m, err := NewMatchup(reg, [2]string{"trk-a", "trk-b"}, [2][]*cards.Card{deck, deck}, 0)
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{GorgeAutopaySurface: autopay}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg})
			CloseSide(k)
			if res.Div != nil {
				t.Fatalf("autopay=%v seed %d: divergence at %d: %s %s %s %v", autopay, seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Detail, res.Div.Diff)
			}
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Fatalf("autopay=%v seed %d: %s", autopay, seed, res.Stall)
			}
		}
	}
}

// Two restricted batches float at once: Helga, Skittish Seer's surplus {W}
// (big or {X} creature spells only) floats from one payment, and a later
// cast's mana window taps Lilypad Village for its {U} (Bird, Frog, Otter or
// Rat spells only) beside it. Each payment sees only the batches its
// restriction admits and carves them in the order they were added, as gorge's
// Players[p].RestrictedMana does (CR 106.6). The kernel held one restriction
// at a time and stalled at the Village's tap.
func TestTwoRestrictionsFloatTogetherLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-blb")
	r, err := ReadRepro("testdata/repro/helga_and_village_float_together.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	var es [2][]cardset.Entry
	for i := range es {
		for _, n := range r.Cards[i] {
			es[i] = append(es[i], cardset.Entry{Name: n, Count: 1})
		}
	}
	tbl, _, _, err := set.Matchup(es[0], es[1])
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions)
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	res := Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
	CloseSide(k)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Detail, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatal(res.Stall)
	}
	if res.Decisions < len(r.Actions) {
		t.Fatalf("the game ended at decision %d of the script's %d", res.Decisions, len(r.Actions))
	}
}
