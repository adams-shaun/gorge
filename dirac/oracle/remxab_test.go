package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Amass / Token Remembered lockstep: Goblin Plate Mail
// attaches itself to the Army its enters trigger amassed (RememberAmass$,
// then Attach Defined$ Remembered), and Generous Plunderer's upkeep trigger
// creates a Treasure and, when it does, makes the targeted opponent create a
// tapped one (RememberOriginalTokens$, then an ImmediateTrigger whose
// TriggerAmount$ Remembered$Amount counts the Treasures just created).
// Counters read gorge's state after each decision:
//
//  1. a Plate Mail that attached itself to an Army token,
//  2. an opponent's Treasure token (the pool's only source is the Plunderer).
var kxDecks = [][2][]cardset.Entry{
	{ // Plate Mail and the Plunderer, against burn and creatures
		{
			{Name: "Goblin Plate Mail", Count: 8}, {Name: "Generous Plunderer", Count: 8}, {Name: "Savannah Lions", Count: 4},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 8},
		},
		{
			{Name: "Shock", Count: 6}, {Name: "Hill Giant", Count: 6}, {Name: "Grizzly Bears", Count: 6}, {Name: "Savannah Lions", Count: 4},
			{Name: "Mountain", Count: 6}, {Name: "Forest", Count: 6}, {Name: "Plains", Count: 6},
		},
	},
}

type kxTally struct {
	plated, treasures int
	seenPlate         map[state.ObjID]bool
	seenTreasure      map[state.ObjID]bool
}

func (p *kxTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	for pl := state.PlayerID(0); pl < 2; pl++ {
		for _, id := range g.Zone(state.ZBattlefield, pl) {
			o := g.Obj(id)
			if o == nil || o.Face() == nil {
				continue
			}
			switch o.Face().Name {
			case "Goblin Plate Mail":
				if h := g.Obj(o.AttachedTo); h != nil && h.IsToken && !p.seenPlate[id] {
					p.seenPlate[id] = true
					p.plated++
				}
			case "Treasure Token":
				if o.IsToken && o.Tapped && !p.seenTreasure[id] {
					p.seenTreasure[id] = true
					p.treasures++
				}
			}
		}
	}
	return nil
}

func TestRemxabLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("remxab")
	if err != nil {
		t.Fatal(err)
	}
	p := kxTally{seenPlate: map[state.ObjID]bool{}, seenTreasure: map[state.ObjID]bool{}}
	games := 0
	var matchups [][2][]cardset.Entry
	for _, d := range kxDecks {
		matchups = append(matchups, d, [2][]cardset.Entry{d[1], d[0]})
	}
	for mi, md := range matchups {
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
		var tbl *kop.Table
		if tbl, _, _, err = set.Matchup(md[0], md[1]); err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 96; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kx-a", "kx-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := fbPolicy(seed)
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 8000, MaxTurns: 80, Check: p.check})
			CloseSide(k)
			games++
			if res.Stall != "" {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games: Plate Mail attached to an Army %d, opponent Treasures %d", games, p.plated, p.treasures)
	if p.plated < 3 {
		t.Errorf("%d Plate Mails attached to an Army, want at least 3", p.plated)
	}
	if p.treasures < 3 {
		t.Errorf("%d opponent Treasures from the Plunderer's reflexive trigger, want at least 3", p.treasures)
	}
}
