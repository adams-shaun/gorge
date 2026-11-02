package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The tap-another-permanent mana lockstep: Saruli
// Caretaker and Springleaf Drum, "{T}, Tap an untapped creature you control:
// Add one mana of any color", in a Pauper Spy-like deck whose Balustrade Spy
// needs {B} beside few Swamps. gorge's planner census cannot price the
// creature choice (a relaxable gap), so the mana is only ever reached by a
// scripted prefix (rules potentialPrefixScript): the kernel's tapXSearch
// must pick the same creature to tap (battlefield order) and the same
// colour (the first the play needs) at every play. The mana creatures
// (Elves of Deep Shadow, Llanowar Elves) are candidate taps AND mana
// sources, so tapping one decides the rest of the turn. The expected answer
// at every decision is gorge's.
var kcaretakerDecks = [][2][]cardset.Entry{{
	{
		{Name: "Saruli Caretaker", Count: 4}, {Name: "Springleaf Drum", Count: 3}, {Name: "Balustrade Spy", Count: 4},
		{Name: "Elves of Deep Shadow", Count: 4}, {Name: "Llanowar Elves", Count: 3}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Skinrender", Count: 2}, {Name: "Chainer's Edict", Count: 2}, {Name: "Doom Blade", Count: 2},
		{Name: "Gray Merchant of Asphodel", Count: 2}, {Name: "Vampire Nighthawk", Count: 1},
		{Name: "Swamp", Count: 6}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Faerie Miscreant", Count: 4}, {Name: "Spellstutter Sprite", Count: 3}, {Name: "Pestermite", Count: 3},
		{Name: "Ninja of the Deep Hours", Count: 3}, {Name: "Counterspell", Count: 3}, {Name: "Shock", Count: 3},
		{Name: "Lightning Bolt", Count: 3}, {Name: "Wind Drake", Count: 3}, {Name: "Hill Giant", Count: 2},
		{Name: "Island", Count: 9}, {Name: "Mountain", Count: 4},
	},
}, {
	{
		{Name: "Saruli Caretaker", Count: 4}, {Name: "Springleaf Drum", Count: 4}, {Name: "Balustrade Spy", Count: 4},
		{Name: "Elves of Deep Shadow", Count: 3}, {Name: "Llanowar Elves", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Skinrender", Count: 2}, {Name: "Doom Blade", Count: 3}, {Name: "Vampire Nighthawk", Count: 2},
		{Name: "Swamp", Count: 7}, {Name: "Forest", Count: 3},
	},
	{
		{Name: "Saruli Caretaker", Count: 4}, {Name: "Springleaf Drum", Count: 4}, {Name: "Balustrade Spy", Count: 4},
		{Name: "Elves of Deep Shadow", Count: 4}, {Name: "Llanowar Elves", Count: 3}, {Name: "Hill Giant", Count: 3},
		{Name: "Lightning Bolt", Count: 3}, {Name: "Shock", Count: 3}, {Name: "Gray Merchant of Asphodel", Count: 2},
		{Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 4},
	},
}}

// kcaretakerTally counts, on gorge's side, the mana activations that paid a
// tapXType cost, by source.
type kcaretakerTally struct {
	caretaker, drum, games int
}

// kcaretakerRun plays seeds 1..n of every matchup, failing on each
// divergence (report) or stopping at the first; it returns the divergent
// games.
func kcaretakerRun(t *testing.T, n uint64, report bool, tal *kcaretakerTally) int {
	t.Helper()
	return kcaretakerRunM(t, -1, n, report, tal)
}

// kcaretakerRunM is kcaretakerRun over matchup only (-1: every matchup).
func kcaretakerRunM(t *testing.T, only int, n uint64, report bool, tal *kcaretakerTally) int {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("caretaker")
	if err != nil {
		t.Fatal(err)
	}
	div := 0
	for mi, md := range kcaretakerDecks {
		if only >= 0 && mi != only {
			continue
		}
		var decks [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= n; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"ct-a", "ct-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			check := func(ref *GorgeSide, _ Side) *Divergence {
				if ref.E.ManaAbilityHook == nil && tal != nil {
					e := ref.E
					e.ManaAbilityHook = func(_ state.PlayerID, src state.ObjID, sa *cards.SA) {
						if !strings.Contains(sa.Params["Cost"], "tapXType") {
							return
						}
						if o := e.G.Obj(src); o != nil && o.Face() != nil {
							switch o.Face().Name {
							case "Saruli Caretaker":
								tal.caretaker++
							case "Springleaf Drum":
								tal.drum++
							}
						}
					}
				}
				return nil
			}
			res := Lockstep(m, seed, k, pkPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40, Check: check})
			CloseSide(k)
			if tal != nil {
				tal.games++
			}
			if res.Stall != "" && res.Stall != "turns" {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				div++
				if !report {
					t.Logf("matchup %d seed %d diverges (decision %d: %s)", mi, seed, d.At, d.What)
					return div
				}
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return div
}

// kcaretakerLockstep plays 300 seeds of matchup mi (two matchups: 600
// games), no divergence, and the tapXType mana paid for plays on gorge's side.
func kcaretakerLockstep(t *testing.T, mi int, minCaretaker, minDrum int) {
	var tal kcaretakerTally
	kcaretakerRunM(t, mi, 300, true, &tal)
	t.Logf("%d games: Saruli Caretaker tapXType activations %d, Springleaf Drum %d", tal.games, tal.caretaker, tal.drum)
	if tal.caretaker < minCaretaker || tal.drum < minDrum {
		t.Errorf("tapXType mana activations: Caretaker %d, Drum %d; want at least %d and %d", tal.caretaker, tal.drum, minCaretaker, minDrum)
	}
}

// TestCaretakerLockstepM0: the Spy deck against blue-red tempo.
func TestCaretakerLockstepM0(t *testing.T) {
	t.Parallel()
	kcaretakerLockstep(t, 0, 100, 200)
}

// TestCaretakerLockstepM1: the mirror, both decks with Caretakers and Drums
// (several gaps at once: the search's depth, the order of the activations).
func TestCaretakerLockstepM1(t *testing.T) {
	t.Parallel()
	kcaretakerLockstep(t, 1, 200, 600)
}

// TestCaretakerLockstepMutation: without the gap, with the colour always
// the first, and with the last creature tapped first, some seed diverges.
func TestCaretakerLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoTapMana = true
	n := kcaretakerRun(t, 40, false, nil)
	dirac.DebugNoTapMana = false
	if n == 0 {
		t.Error("no seed diverged without the tap-another-permanent gap: the pool never needs the mana")
	}
	for _, m := range []int{1, 2} {
		dirac.SetDebugTapXMutate(m)
		n := kcaretakerRun(t, 100, false, nil)
		dirac.SetDebugTapXMutate(0)
		if n == 0 {
			t.Errorf("mutation %d: no seed diverged", m)
		}
	}
}
