package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Mirrex lockstep: {T}: Add {C}, and {T}: Add one mana of any
// colour, activated only if Mirrex entered this turn (IsPresent$
// Card.Self+ThisTurnEntered; gorge pay.ManaActivationGateHolds). Both decks
// lean on Mirrex for their colours; matchup 1 adds Cackling Prowler (Ward
// {2}) beside Shock, so a ward payment runs the mana window, which activates
// Mirrex's ability 0 ({C}). The expected answer at every decision is gorge's.
var kmirrexDecks = [][2][]cardset.Entry{{
	{
		{Name: "Mirrex", Count: 14}, {Name: "Savannah Lions", Count: 6}, {Name: "Llanowar Elves", Count: 6},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Plains", Count: 4}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Mirrex", Count: 14}, {Name: "Shock", Count: 8}, {Name: "Wind Drake", Count: 6},
		{Name: "Hill Giant", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Island", Count: 4},
	},
}, {
	{
		{Name: "Mirrex", Count: 12}, {Name: "Cackling Prowler", Count: 6}, {Name: "Shock", Count: 8},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Forest", Count: 4}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "Mirrex", Count: 12}, {Name: "Cackling Prowler", Count: 6}, {Name: "Shock", Count: 8},
		{Name: "Llanowar Elves", Count: 6}, {Name: "Forest", Count: 4}, {Name: "Mountain", Count: 4},
	},
}}

// kmirrexTally counts, on gorge's side, Mirrex's mana activations by ability
// (ability 1 is the gated any-colour one) and its Mite token activations.
type kmirrexTally struct {
	any, colourless, games int
}

// kmirrexRun plays seeds 1..n of every matchup (n per matchup), failing on
// each divergence (report) or stopping at the first; it returns the number
// of divergent games.
func kmirrexRun(t *testing.T, n uint64, report bool, tal *kmirrexTally) int {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mirrex")
	if err != nil {
		t.Fatal(err)
	}
	div := 0
	for mi, md := range kmirrexDecks {
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
			m, err := NewMatchup(reg, [2]string{"mx-a", "mx-b"}, decks, 1)
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
						if o := e.G.Obj(src); o == nil || o.Face() == nil || o.Face().Name != "Mirrex" || tal == nil {
							return
						}
						if sa.Params["Produced"] == "Any" {
							tal.any++
						} else {
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

func TestMirrexLockstep(t *testing.T) {
	t.Parallel()
	var tal kmirrexTally
	kmirrexRun(t, 300, true, &tal)
	t.Logf("%d games: Mirrex any-colour activations %d, {C} activations %d", tal.games, tal.any, tal.colourless)
	if tal.any < 50 || tal.colourless < 50 {
		t.Errorf("Mirrex activations: any colour %d, {C} %d; want at least 50 each", tal.any, tal.colourless)
	}
}

// TestMirrexLockstepMutation: with the gate-on view's union dropped (the
// gated any-colour ability never counted), and with the gate read always
// on, some seed diverges.
func TestMirrexLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, m := range []uint8{5, 6} {
		dirac.SetDebugEntersTappedMutate(m)
		n := kmirrexRun(t, 100, false, nil)
		dirac.SetDebugEntersTappedMutate(0)
		if n == 0 {
			t.Errorf("mutation %d: no seed diverged", m)
		}
	}
}
