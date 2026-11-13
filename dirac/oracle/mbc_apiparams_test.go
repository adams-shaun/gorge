package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMBCApiParams13Lowers plays one lowered card of each spelling the MBC
// api-params-13 family lowers, in lockstep against gorge on a table built
// for exactly its decks (kgen refused every one of these cards before):
//
//   - param:trigger.Dethrone (Grenzo's Cutthroat: K:Dethrone's Attacks
//     trigger, a fire-time gate on the attacked player having the most life
//     or tied). The Cutthroat attacks a Grizzly Bears deck that hits back, so
//     the gate goes both ways; a +1/+1 counter on a Cutthroat is the trigger
//     resolved. Treasonous Ogre, the MBC carrier, still refuses on its
//     PayLife mana ability.
//   - other:SetState Transform on a layout the kernel does not turn (Sunder
//     the Gateway's second mode: Incubate 2, then transform an Incubator
//     token you control): gorge's mandatory "setstate_pick" is posed.
func TestMBCApiParams13Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	bears := []cardset.Entry{{Name: "Grizzly Bears", Count: 16}, {Name: "Forest", Count: 24}}
	for _, tc := range []struct {
		name    string
		deck    []cardset.Entry
		reached func(ref *GorgeSide) bool
	}{
		{
			name: "Dethrone/Grenzos_Cutthroat",
			deck: []cardset.Entry{{Name: "Grenzo's Cutthroat", Count: 24}, {Name: "Mountain", Count: 16}},
			reached: func(ref *GorgeSide) bool {
				for p := range state.PlayerID(2) {
					for _, id := range ref.E.G.Zone(state.ZBattlefield, p) {
						if o := ref.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Grenzo's Cutthroat" && o.Counter("P1P1") > 0 {
							return true
						}
					}
				}
				return false
			},
		},
		{
			name: "SetStateTransformPick/Sunder_the_Gateway",
			deck: []cardset.Entry{{Name: "Sunder the Gateway", Count: 24}, {Name: "Plains", Count: 16}},
			reached: func(ref *GorgeSide) bool {
				d := ref.Pending()
				return d != nil && d.ResumeKind == "setstate_pick"
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decks := [2][]cardset.Entry{tc.deck, bears}
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
			reached := 0
			for seed := uint64(1); seed <= 16; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"apiparams-a", "apiparams-b"}, cs, 0)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				hit := false
				rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40}
				rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
					hit = hit || tc.reached(ref)
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
			t.Logf("%s: the mechanism reached in %d of 16 games", tc.name, reached)
			if reached < 3 {
				t.Errorf("the mechanism reached in %d of 16 games, want at least 3", reached)
			}
		})
	}
}
