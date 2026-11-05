package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Sanar, Innovative First-Year over the digimprint pool: a library reveal
// that stays in place (RememberRevealed$ with RevealedDestination$ Library),
// five per-colour hidden picks out of the remembered cards into exile, a
// shuffle, and a MayPlay over the imprinted cards.

var sanarDecks = [2][]cardset.Entry{
	{
		{Name: "Sanar, Innovative First-Year", Count: 4}, {Name: "Black Widow, Super Spy", Count: 4},
		{Name: "Isamaru, Hound of Konda", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Volcanic Island", Count: 4}, {Name: "Island", Count: 4},
		{Name: "Mountain", Count: 4}, {Name: "Swamp", Count: 4}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Isamaru, Hound of Konda", Count: 4},
		{Name: "Black Widow, Super Spy", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Volcanic Island", Count: 4},
		{Name: "Mountain", Count: 4}, {Name: "Forest", Count: 4}, {Name: "Plains", Count: 4},
	},
}

// sanarScan counts, in gorge's event log, the library-to-exile moves made
// while the latest pushed trigger or spell is Sanar (the five picks), and
// the casts out of exile after one (the MayPlay tail).
func sanarScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	face := func(id state.ObjID) string {
		if ob := g.E.G.Obj(id); ob != nil && ob.Face() != nil {
			return ob.Face().Name
		}
		return ""
	}
	last, afterSanar := "", false
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.PutOnStack, events.TriggerPush, events.AbilityPush:
			if nm := face(ev.Obj); nm != "" {
				last = nm
				afterSanar = afterSanar || nm == "Sanar, Innovative First-Year"
			}
			if ev.Kind == events.PutOnStack && ev.From == state.ZExile && afterSanar && last != "Black Widow, Super Spy" {
				n["sanarcast"]++
			}
		case events.MoveZone:
			if ev.From == state.ZLibrary && ev.To == state.ZExile && last == "Sanar, Innovative First-Year" {
				n["sanarexile"]++
			}
		}
	}
}

func TestSanarInnovativeFirstYearLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("digimprint")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range sanarDecks {
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
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(sanarDecks[0], sanarDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	played, branch := map[string]int{}, map[string]int{}
	games, divs := 0, 0
	for seed := uint64(1); seed <= 128; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"sanar-a", "sanar-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		inner := eagerPolicy(seed, played)
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return inner.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		games++
		sanarScan(gs, branch)
		if d := res.Div; d != nil {
			divs++
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			kczDump(t, gs)
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	t.Logf("%d games, %d divergences; sanar exiles %d, casts %d", games, divs, branch["sanarexile"], branch["sanarcast"])
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if branch["sanarexile"] < 20 {
		t.Errorf("sanar colour exiles %d, want at least 20", branch["sanarexile"])
	}
	if branch["sanarcast"] < 1 {
		t.Errorf("sanar free casts %d, want at least 1", branch["sanarcast"])
	}
}
