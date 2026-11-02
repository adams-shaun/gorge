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
var kbaylenDecks = [][2][]cardset.Entry{{
	{
		{Name: "Baylen, the Haymaker", Count: 4}, {Name: "Raise the Alarm", Count: 3}, {Name: "Dragon Fodder", Count: 3},
		{Name: "Krenko's Command", Count: 3}, {Name: "Hordeling Outburst", Count: 2}, {Name: "Lightning Helix", Count: 3},
		{Name: "Giant Growth", Count: 2}, {Name: "Rancor", Count: 2}, {Name: "Llanowar Elves", Count: 2},
		{Name: "Mountain", Count: 6}, {Name: "Plains", Count: 5}, {Name: "Forest", Count: 5},
	},
	{
		{Name: "Hill Giant", Count: 5}, {Name: "Gray Ogre", Count: 5}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Doom Blade", Count: 3}, {Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 9},
	},
}, {
	{
		{Name: "Baylen, the Haymaker", Count: 4}, {Name: "Raise the Alarm", Count: 4}, {Name: "Dragon Fodder", Count: 4},
		{Name: "Krenko's Command", Count: 2}, {Name: "Hordeling Outburst", Count: 3}, {Name: "Lightning Helix", Count: 4},
		{Name: "Giant Growth", Count: 2}, {Name: "Rancor", Count: 1}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Mountain", Count: 6}, {Name: "Plains", Count: 4}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Baylen, the Haymaker", Count: 4}, {Name: "Raise the Alarm", Count: 4}, {Name: "Dragon Fodder", Count: 4},
		{Name: "Krenko's Command", Count: 3}, {Name: "Hordeling Outburst", Count: 2}, {Name: "Lightning Helix", Count: 3},
		{Name: "Giant Growth", Count: 1}, {Name: "Rancor", Count: 1}, {Name: "Llanowar Elves", Count: 2},
		{Name: "Birchlore Rangers", Count: 2},
		{Name: "Mountain", Count: 6}, {Name: "Plains", Count: 4}, {Name: "Forest", Count: 4},
	},
}}

// kbaylenTally counts, on gorge's side, the mana activations that paid a
// tapXType cost, by source.
type kbaylenTally struct {
	baylen, rangers, games int
}

// kbaylenRun plays seeds 1..n of every matchup, failing on each
// divergence (report) or stopping at the first; it returns the divergent
// games.
func kbaylenRun(t *testing.T, n uint64, report bool, tal *kbaylenTally) int {
	t.Helper()
	return kbaylenRunM(t, -1, n, report, tal)
}

// kbaylenRunM is kbaylenRun over matchup only (-1: every matchup).
func kbaylenRunM(t *testing.T, only int, n uint64, report bool, tal *kbaylenTally) int {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("baylen")
	if err != nil {
		t.Fatal(err)
	}
	div := 0
	for mi, md := range kbaylenDecks {
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
			m, err := NewMatchup(reg, [2]string{"kb-a", "kb-b"}, decks, 1)
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
							case "Baylen, the Haymaker":
								tal.baylen++
							case "Birchlore Rangers":
								tal.rangers++
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

// kbaylenLockstep plays 300 seeds of matchup mi (two matchups: 600
// games), no divergence, and the tapXType mana paid for plays on gorge's side.
func kbaylenLockstep(t *testing.T, mi int, minBaylen, minRangers int) {
	var tal kbaylenTally
	kbaylenRunM(t, mi, 300, true, &tal)
	t.Logf("%d games: tapXType mana activations: Baylen %d, Birchlore Rangers %d", tal.games, tal.baylen, tal.rangers)
	if tal.baylen < minBaylen || tal.rangers < minRangers {
		t.Errorf("tapXType mana activations: Baylen %d, Rangers %d; want at least %d and %d", tal.baylen, tal.rangers, minBaylen, minRangers)
	}
}

// TestBaylenLockstepM0: the Spy deck against blue-red tempo.
func TestBaylenLockstepM0(t *testing.T) {
	t.Parallel()
	kbaylenLockstep(t, 0, 50, 0)
}

// TestBaylenLockstepM1: the mirror, both decks with Caretakers and Drums
// (several gaps at once: the search's depth, the order of the activations).
func TestBaylenLockstepM1(t *testing.T) {
	t.Parallel()
	kbaylenLockstep(t, 1, 100, 20)
}

// TestBaylenLockstepMutation: without the gap, with the colour always
// the first, and with the last creature tapped first, some seed diverges.
func TestBaylenLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoTapMana = true
	n := kbaylenRun(t, 40, false, nil)
	dirac.DebugNoTapMana = false
	if n == 0 {
		t.Error("no seed diverged without the tap-another-permanent gap: the pool never needs the mana")
	}
	for _, m := range []int{1, 2} {
		dirac.SetDebugTapXMutate(m)
		n := kbaylenRun(t, 100, false, nil)
		dirac.SetDebugTapXMutate(0)
		if n == 0 {
			t.Errorf("mutation %d: no seed diverged", m)
		}
	}
}
