package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The Mirrex lockstep: {T}: Add {C}, and {T}: Add one mana of any
// colour, activated only if Mirrex entered this turn (IsPresent$
// Card.Self+ThisTurnEntered; gorge pay.ManaActivationGateHolds). Both decks
// lean on Mirrex for their colours; matchup 1 adds Cackling Prowler (Ward
// {2}) beside Shock, so a ward payment runs the mana window, which activates
// Mirrex's ability 0 ({C}). The expected answer at every decision is gorge's.
var ksurgeDecks = [][2][]cardset.Entry{{
	{
		{Name: "Surge Engine", Count: 14}, {Name: "Island", Count: 16}, {Name: "Grizzly Bears", Count: 6}, {Name: "Shock", Count: 4},
	},
	{
		{Name: "Surge Engine", Count: 8}, {Name: "Island", Count: 10}, {Name: "Mountain", Count: 6},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Wind Drake", Count: 6}, {Name: "Hill Giant", Count: 4},
	},
}}

// ksurgeTally counts, on gorge's side, Surge Engine's activations by ability
// (0 = loses defender and can't be blocked, 1 = becomes blue 5/4, 2 = draw
// three) and the games played.
type ksurgeTally struct {
	ab    [3]int
	games int
}

// ksurgeRun plays seeds 1..n of every matchup (n per matchup), failing on
// each divergence (report) or stopping at the first; it returns the number
// of divergent games.
func ksurgeRun(t *testing.T, n uint64, report bool, tal *ksurgeTally, mut func(*kop.Override)) int {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("surge")
	if err != nil {
		t.Fatal(err)
	}
	div := 0
	for mi, md := range ksurgeDecks {
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
		if mut != nil {
			for i := range tbl.Overrides {
				mut(&tbl.Overrides[i])
			}
		}
		for seed := uint64(1); seed <= n; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"sg-a", "sg-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			seen := 0
			check := func(ref *GorgeSide, _ Side) *Divergence {
				evs := ref.E.L.Events
				for ; seen < len(evs); seen++ {
					ev := evs[seen]
					if ev.Kind != events.AbilityPush || tal == nil {
						continue
					}
					if o := ref.E.G.Obj(ev.Obj); o != nil && o.Face() != nil && o.Face().Name == "Surge Engine" && ev.Amount >= 0 && ev.Amount < 3 {
						tal.ab[ev.Amount]++
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

// gorgeSurgeFixTime is gorge cr-animate-cantblockby (b055edbfd): an Animate
// staticAbilities$ CantBlockBy grant is registered for real.
const gorgeSurgeFixTime = "20261026162550"

func TestSurgeLockstep(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeSurgeFixTime) {
		t.Skip("gorge before the Animate CantBlockBy grant (b055edbfd)")
	}
	var tal ksurgeTally
	ksurgeRun(t, 600, true, &tal, nil)
	t.Logf("%d games: Surge Engine activations %v", tal.games, tal.ab)
	if tal.ab[0] < 50 || tal.ab[1] < 20 || tal.ab[2] < 5 {
		t.Errorf("Surge Engine activations %v; want at least 50, 20 and 5", tal.ab)
	}
}

// TestSurgeLockstepMutation: with the override's removed Defender kept, and
// with its Unblockable grant dropped, some seed diverges.
func TestSurgeLockstepMutation(t *testing.T) {
	if !gorgePinAtLeast(t, gorgeSurgeFixTime) {
		t.Skip("gorge before the Animate CantBlockBy grant (b055edbfd)")
	}
	for name, mut := range map[string]func(*kop.Override){
		"defender kept":    func(o *kop.Override) { o.RemoveKw = 0 },
		"unblockable lost": func(o *kop.Override) { o.AddKw = 0 },
	} {
		if n := ksurgeRun(t, 100, false, nil, mut); n == 0 {
			t.Errorf("mutation %q: no seed diverged", name)
		}
	}
}
