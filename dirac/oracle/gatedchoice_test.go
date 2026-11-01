package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The gated-choice lockstep: the MSH basic-land cycle,
// {T}: Add {C}, and {T}: Add {c1} or {c2}, activated only if the land entered
// this turn or its controller controls a basic land (IsPresent$
// Card.Self+ThisTurnEntered,Land.Basic+YouCtrl; gorge
// pay.ManaActivationGateHolds). The decks carry a few basics so the gate is
// off on some turns and on in others; matchup 1 adds Cackling Prowler (Ward
// {2}), so a ward payment runs the mana window, which activates ability 0
// ({C}). The expected answer at every decision is gorge's.
var kgatedchoiceDecks = [][2][]cardset.Entry{{
	{
		{Name: "Dark Fortress", Count: 2}, {Name: "Gathering Place", Count: 3}, {Name: "Gleaming Bastion", Count: 3},
		{Name: "Hidden Lair", Count: 2}, {Name: "Training Compound", Count: 2},
		{Name: "Plains", Count: 1}, {Name: "Forest", Count: 1}, {Name: "Swamp", Count: 1},
		{Name: "Savannah Lions", Count: 6}, {Name: "Llanowar Elves", Count: 6}, {Name: "Grizzly Bears", Count: 6}, {Name: "Wind Drake", Count: 7},
	},
	{
		{Name: "Dark Fortress", Count: 3}, {Name: "Gathering Place", Count: 2}, {Name: "Gleaming Bastion", Count: 2},
		{Name: "Hidden Lair", Count: 3}, {Name: "Training Compound", Count: 3},
		{Name: "Island", Count: 1}, {Name: "Mountain", Count: 1}, {Name: "Swamp", Count: 1},
		{Name: "Shock", Count: 8}, {Name: "Wind Drake", Count: 6}, {Name: "Hill Giant", Count: 6}, {Name: "Grizzly Bears", Count: 4},
	},
}, {
	{
		{Name: "Dark Fortress", Count: 2}, {Name: "Gathering Place", Count: 3}, {Name: "Gleaming Bastion", Count: 3},
		{Name: "Hidden Lair", Count: 2}, {Name: "Training Compound", Count: 3},
		{Name: "Forest", Count: 1}, {Name: "Mountain", Count: 1},
		{Name: "Cackling Prowler", Count: 6}, {Name: "Shock", Count: 8}, {Name: "Grizzly Bears", Count: 6}, {Name: "Savannah Lions", Count: 5},
	},
	{
		{Name: "Dark Fortress", Count: 3}, {Name: "Gathering Place", Count: 2}, {Name: "Gleaming Bastion", Count: 3},
		{Name: "Hidden Lair", Count: 3}, {Name: "Training Compound", Count: 2},
		{Name: "Forest", Count: 1}, {Name: "Mountain", Count: 1}, {Name: "Plains", Count: 1},
		{Name: "Cackling Prowler", Count: 6}, {Name: "Shock", Count: 8}, {Name: "Llanowar Elves", Count: 6}, {Name: "Wind Drake", Count: 4},
	},
}}

var kgatedchoiceLands = map[string]bool{"Dark Fortress": true, "Gathering Place": true, "Gleaming Bastion": true, "Hidden Lair": true, "Training Compound": true}
var kgatedchoiceBasics = map[string]bool{"Plains": true, "Island": true, "Swamp": true, "Mountain": true, "Forest": true}

// kgatedchoiceTally counts, on gorge's side, the five lands' mana activations
// by ability (the Combo one vs {C}) and the decisions at which one of them sat
// untapped with its gate off (not entered this turn, no basic land).
type kgatedchoiceTally struct {
	colour, colourless, gateOff, games int
}

// kgatedchoiceRun plays seeds 1..n of every matchup, failing on each
// divergence (report) or stopping at the first; it returns the number of
// divergent games.
func kgatedchoiceRun(t *testing.T, n uint64, report bool, tal *kgatedchoiceTally) int {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("gatedchoice")
	if err != nil {
		t.Fatal(err)
	}
	div := 0
	for mi, md := range kgatedchoiceDecks {
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
			m, err := NewMatchup(reg, [2]string{"gc-a", "gc-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			check := func(ref *GorgeSide, _ Side) *Divergence {
				e := ref.E
				if e.ManaAbilityHook == nil {
					e.ManaAbilityHook = func(_ state.PlayerID, src state.ObjID, sa *cards.SA) {
						if o := e.G.Obj(src); o == nil || o.Face() == nil || !kgatedchoiceLands[o.Face().Name] || tal == nil {
							return
						}
						if strings.HasPrefix(sa.Params["Produced"], "Combo") {
							tal.colour++
						} else {
							tal.colourless++
						}
					}
				}
				if tal == nil {
					return nil
				}
				for p := range e.G.Players {
					basic := false
					var off int
					for _, id := range e.G.Zone(state.ZBattlefield, state.PlayerID(p)) {
						o := e.G.Obj(id)
						if o == nil || o.Face() == nil || o.Controller != state.PlayerID(p) {
							continue
						}
						if kgatedchoiceBasics[o.Face().Name] {
							basic = true
						} else if kgatedchoiceLands[o.Face().Name] && !o.Tapped && !o.EnteredThisTurn {
							off++
						}
					}
					if !basic {
						tal.gateOff += off
					}
				}
				return nil
			}
			res := Lockstep(m, seed, k, pkPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 20, Check: check})
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

func TestGatedchoiceLockstep(t *testing.T) {
	t.Parallel()
	var tal kgatedchoiceTally
	kgatedchoiceRun(t, 300, true, &tal)
	t.Logf("%d games: gated-choice activations %d, {C} activations %d, gate-off untapped decisions %d", tal.games, tal.colour, tal.colourless, tal.gateOff)
	if tal.colour < 50 || tal.colourless < 50 || tal.gateOff < 20 {
		t.Errorf("gated-choice activations %d, {C} %d, gate-off decisions %d; want at least 50, 50, 20", tal.colour, tal.colourless, tal.gateOff)
	}
}

// TestGatedchoiceMutation: with the gate-on union's choice colours dropped,
// some seed diverges.
func TestGatedchoiceMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugGatedChoice = true
	n := kgatedchoiceRun(t, 100, false, nil)
	dirac.DebugGatedChoice = false
	if n == 0 {
		t.Error("mutation: no seed diverged")
	}
}
