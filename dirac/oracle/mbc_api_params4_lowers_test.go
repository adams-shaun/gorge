package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMBCApiParams4Lowers plays one lowered card of each MBC api-params-4
// spelling kgen now lowers, in lockstep against gorge on a table built for
// exactly its decks (kgen refused every one of these cards before):
//
//   - param:Untap.AIManaPref (Basalt Monolith's "{3}: Untap this artifact"):
//     AIManaPref$ is Forge's AI mana-source hint, ignored by both engines.
//     The Monolith never untaps in its untap step, so any Untap event on one
//     is the ability resolving. Its untap is paid while another Monolith
//     stands untapped, so the game also pins the payment planner's
//     no_untap tier: gorge plans a printed "doesn't untap during your untap
//     step" source as last resort with irreversible cost 25 (pay
//     costNoUntap), tapping the Forests first; the kernel tapped the other
//     Monolith until dirac/mana.go did the same.
//   - param:GainControl.AllValid, two shapes: Varchild, Betrayer of
//     Kjeldor's leaves-the-battlefield "gain control of all Survivors" (a
//     permanent grant; the opponent's Lightning Bolts take Varchild off the
//     battlefield after it hands out Survivors), and Mob Rule's until-end-of-
//     turn steal with Untap$ and haste (the threaten riders over a sweep).
//     Reached when a Survivor (Varchild) or any permanent (Mob Rule) is
//     controlled by a player other than its owner.
//
// Urza's Bauble (param:Reveal.Random) and Thomil, the Destroyer
// (param:x.UnlessSwitched) stay refused on their other families' blockers:
// see the report.
func TestMBCApiParams4Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	bears := []cardset.Entry{{Name: "Grizzly Bears", Count: 16}, {Name: "Forest", Count: 24}}
	boltBears := []cardset.Entry{{Name: "Grizzly Bears", Count: 10}, {Name: "Lightning Bolt", Count: 8}, {Name: "Forest", Count: 11}, {Name: "Mountain", Count: 11}}
	stolen := func(g *state.Game, name string) bool {
		for i := range g.Objs {
			o := &g.Objs[i]
			if o.Zone != state.ZBattlefield || o.Controller == o.Owner || o.Face() == nil {
				continue
			}
			if strings.HasPrefix(o.Face().Name, name) {
				return true
			}
		}
		return false
	}
	for _, tc := range []struct {
		name    string
		deck    []cardset.Entry
		opp     []cardset.Entry
		reached func(ref *GorgeSide, from *int) bool
	}{
		{
			name: "UntapAIManaPref/Basalt_Monolith",
			deck: []cardset.Entry{{Name: "Basalt Monolith", Count: 16}, {Name: "Grizzly Bears", Count: 8}, {Name: "Forest", Count: 16}},
			opp:  bears,
			reached: func(ref *GorgeSide, from *int) bool {
				evs := ref.E.L.Events
				hit := false
				for _, e := range evs[*from:] {
					if e.Kind != events.Untap {
						continue
					}
					if o := ref.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Basalt Monolith" {
						hit = true
					}
				}
				*from = len(evs)
				return hit
			},
		},
		{
			name: "GainControlAllValid/Varchild_Betrayer_of_Kjeldor",
			deck: []cardset.Entry{{Name: "Varchild, Betrayer of Kjeldor", Count: 22}, {Name: "Mountain", Count: 18}},
			opp:  boltBears,
			reached: func(ref *GorgeSide, _ *int) bool {
				return stolen(ref.E.G, "Survivor Token")
			},
		},
		{
			name: "GainControlAllValidEOT/Mob_Rule",
			deck: []cardset.Entry{{Name: "Mob Rule", Count: 14}, {Name: "Grizzly Bears", Count: 6}, {Name: "Mountain", Count: 20}},
			opp:  bears,
			reached: func(ref *GorgeSide, _ *int) bool {
				return stolen(ref.E.G, "")
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decks := [2][]cardset.Entry{tc.deck, tc.opp}
			reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
			if err != nil {
				t.Fatal(err)
			}
			var names [2][]string
			var cs [2][]*cards.Card
			for s, d := range decks {
				for _, e := range d {
					names[s] = append(names[s], e.Name)
					c, ok := reg.Lookup(e.Name)
					if !ok {
						t.Fatalf("%s not in the corpus", e.Name)
					}
					for range e.Count {
						cs[s] = append(cs[s], c)
					}
				}
			}
			set := printedDelveSet(t, names)
			tbl, _, _, err := set.Matchup(decks[0], decks[1])
			if err != nil {
				t.Fatal(err)
			}
			const games = 16
			reached := 0
			for seed := uint64(1); seed <= games; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"apiparams4-a", "apiparams4-b"}, cs, 0)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				hit, from := false, 0
				rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40}
				rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
					hit = tc.reached(ref, &from) || hit
					return nil
				}
				res := Lockstep(m, seed, k, RandomPolicy(seed), rc)
				CloseSide(k)
				if err := unmodelledStallError(t.Name(), &res); err != nil {
					t.Errorf("seed %d: %v", seed, err)
				}
				if d := res.Div; d != nil {
					t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
				}
				if hit {
					reached++
				}
			}
			t.Logf("%s: the mechanism reached in %d of %d games", tc.name, reached, games)
			if reached < 3 {
				t.Errorf("the mechanism reached in %d of %d games, want at least 3", reached, games)
			}
		})
	}
}
