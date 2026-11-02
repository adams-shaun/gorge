package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Plaza of Heroes lockstep: {T}: Add {C}; {T}: Add one mana
// of any color, spent only on a legendary spell (RestrictValid$
// Spell.Legendary: a census gap the prefix search activates); {T}: Add one
// mana of any color among legendary permanents you control (AB$
// ManaReflected, ReflectProperty$ Is: gorge's planner census prices it).
// Both decks lean on Plaza for their colours and carry legendary creatures
// of several colours beside non-legendary ones; matchup 1 adds Cackling
// Prowler (Ward {2}) against Shock, so a ward payment runs the mana window,
// which activates Plaza's ability 0 ({C}). The expected answer at every
// decision is gorge's.
var kplazaDecks = [][2][]cardset.Entry{{
	{
		{Name: "Plaza of Heroes", Count: 8}, {Name: "Isamaru, Hound of Konda", Count: 6}, {Name: "Happy Hogan, Dauntless Driver", Count: 4},
		{Name: "Aziza, Mage Tower Captain", Count: 4}, {Name: "Savannah Lions", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Plains", Count: 5}, {Name: "Mountain", Count: 5},
	},
	{
		{Name: "Plaza of Heroes", Count: 8}, {Name: "Caelorna, Coral Tyrant", Count: 4}, {Name: "Haru, Hidden Talent", Count: 4},
		{Name: "Kalakscion, Hunger Tyrant", Count: 4}, {Name: "Jasmine Boreal", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Forest", Count: 4}, {Name: "Island", Count: 4}, {Name: "Swamp", Count: 4},
	},
}, {
	{
		{Name: "Plaza of Heroes", Count: 8}, {Name: "Cackling Prowler", Count: 6}, {Name: "Shock", Count: 8},
		{Name: "Isamaru, Hound of Konda", Count: 4}, {Name: "Haru, Hidden Talent", Count: 4}, {Name: "Forest", Count: 5}, {Name: "Mountain", Count: 5},
	},
	{
		{Name: "Plaza of Heroes", Count: 8}, {Name: "Cackling Prowler", Count: 6}, {Name: "Shock", Count: 8},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Happy Hogan, Dauntless Driver", Count: 4}, {Name: "Forest", Count: 5}, {Name: "Mountain", Count: 5},
	},
}}

// kplazaTally counts, on gorge's side, Plaza's mana activations by
// ability: the reflected one, the restricted any colour and {C}.
type kplazaTally struct {
	reflected, restricted, colourless, games int
}

// kplazaRun plays seeds 1..n of every matchup (n per matchup), failing on
// each divergence (report) or stopping at the first; it returns the number
// of divergent games.
func kplazaRun(t *testing.T, only int, n uint64, report bool, tal *kplazaTally) int {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("plaza")
	if err != nil {
		t.Fatal(err)
	}
	div := 0
	for mi, md := range kplazaDecks {
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
			m, err := NewMatchup(reg, [2]string{"pz-a", "pz-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			check := func(ref *GorgeSide, _ Side) *Divergence {
				if ref.E.ManaAbilityHook == nil {
					e := ref.E
					e.ManaAbilityHook = func(_ state.PlayerID, src state.ObjID, sa *cards.SA) {
						if o := e.G.Obj(src); o == nil || o.Face() == nil || o.Face().Name != "Plaza of Heroes" || tal == nil {
							return
						}
						switch {
						case sa.API == "ManaReflected":
							tal.reflected++
						case sa.Params["Produced"] == "Any":
							tal.restricted++
						default:
							tal.colourless++
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

// TestPlazaLockstep plays matchup 0 (350 seeds) and TestPlazaWardLockstep
// matchup 1 (250 seeds): 600 in all, split to keep each under a minute.
func TestPlazaLockstep(t *testing.T) {
	t.Parallel()
	var tal kplazaTally
	kplazaRun(t, 0, 350, true, &tal)
	kplazaCheck(t, &tal)
}

func TestPlazaWardLockstep(t *testing.T) {
	t.Parallel()
	var tal kplazaTally
	kplazaRun(t, 1, 250, true, &tal)
	kplazaCheck(t, &tal)
}

func kplazaCheck(t *testing.T, tal *kplazaTally) {
	t.Logf("%d games: Plaza activations reflected %d, restricted any colour %d, {C} %d", tal.games, tal.reflected, tal.restricted, tal.colourless)
	if tal.reflected < 50 || tal.restricted < 50 || tal.colourless < 50 {
		t.Errorf("Plaza activations: reflected %d, restricted %d, {C} %d; want at least 50 each", tal.reflected, tal.restricted, tal.colourless)
	}
}

// TestPlazaLockstepMutation: with the reflected colours never read (the
// view {C} alone), some seed diverges. (The relaxed-proof and prefix-search
// mutations 2 and 3 are gone with the gap: (85fddcb68)
// Plaza's restricted any colour is a planner alternative a legendary spell
// admits, no census gap, so no prefix search runs for it.)
func TestPlazaLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, m := range []uint8{1} {
		dirac.SetDebugReflMutate(m)
		n := kplazaRun(t, -1, 100, false, nil)
		dirac.SetDebugReflMutate(0)
		if n == 0 {
			t.Errorf("mutation %d: no seed diverged", m)
		}
	}
}
