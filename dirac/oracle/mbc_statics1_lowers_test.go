package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMBCStatics1Lowers plays each Mirage Incarnation of the MBC statics-1
// family in lockstep against gorge on a table built for exactly its deck
// (kgen refused every one of them before): a static printed with EffectZone$
// Graveyard ("As long as CARDNAME is in your graveyard and you control a
// <land>, creatures you control have <keyword>") grants its keyword to the
// owner's creatures only while the card sits in the owner's graveyard and
// the owner controls the land type. The mechanism is reached when an
// Incarnation is in a graveyard and a creature of that seat with no printed
// copy of the keyword has it; the games diverge if the kernel's grant
// differs from gorge's at any decision.
func TestMBCStatics1Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	// the opponent's Mind Rot puts the Incarnations in the graveyard (no
	// random game otherwise loses one often enough); its Walkers are the
	// blockers a granted first strike or landwalk is tested against
	rot := []cardset.Entry{{Name: "Mind Rot", Count: 8}, {Name: "Phyrexian Walker", Count: 8}, {Name: "Swamp", Count: 24}}
	for _, tc := range []struct {
		name, kw, land string
	}{
		{"Anger", "Haste", "Mountain"},
		{"Wonder", "Flying", "Island"},
		{"Filth", "Landwalk", "Swamp"}, // HasKeyword compares the head: Landwalk:Swamp is "Landwalk"
		{"Valor", "First Strike", "Plains"},
		{"Brawn", "Trample", "Forest"}, // lowers with the family; the fifth Incarnation (not in the MBC lists)
	} {
		t.Run(tc.name, func(t *testing.T) {
			deck := []cardset.Entry{{Name: tc.name, Count: 10}, {Name: "Phyrexian Walker", Count: 10}, {Name: tc.land, Count: 20}}
			decks := [2][]cardset.Entry{deck, rot}
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
			// the grant is live: an Incarnation in a seat's graveyard and one
			// of that seat's Phyrexian Walker (a 0/3 artifact creature: none prints the keyword) has it
			live := func(ref *GorgeSide) bool {
				for p := range state.PlayerID(2) {
					inGrave := false
					for _, id := range ref.E.G.Zone(state.ZGraveyard, p) {
						if o := ref.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == tc.name {
							inGrave = true
						}
					}
					if !inGrave {
						continue
					}
					for _, id := range ref.E.G.Zone(state.ZBattlefield, p) {
						if o := ref.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Phyrexian Walker" && ref.E.HasKeyword(id, tc.kw) {
							return true
						}
					}
				}
				return false
			}
			reached := 0
			for seed := uint64(1); seed <= 16; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"statics1-a", "statics1-b"}, cs, 0)
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
					hit = hit || live(ref)
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
			t.Logf("%s: the grant live in %d of 16 games", tc.name, reached)
			if reached < 3 {
				t.Errorf("the grant live in %d of 16 games, want at least 3", reached)
			}
		})
	}
}
