package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The MBC zone-moves-3 graveyard picks in lockstep, one matchup per card of
// the mbczonemoves3 fixture pool:
//
//   - Exhume (spelling other:graveyard choice without an owner, DefinedPlayer$
//     Player): each player puts a creature card from their own graveyard onto
//     the battlefield, the controller first;
//   - Genju of the Fields (the same spelling, DefinedPlayer$ You with
//     ChangeType$ Card.StrictlySelf): when the enchanted Plains is put into a
//     graveyard its controller may return the Genju from their graveyard.
//
// The expected answer at every decision is gorge's. Without the lowering the
// pool's table refuses the card and the matchup cannot be built.

type gpMatchup struct {
	card string
	deck []cardset.Entry
	opp  []cardset.Entry
}

var gpMatchups = []gpMatchup{
	{card: "Exhume", deck: []cardset.Entry{
		{Name: "Exhume", Count: 8}, {Name: "Walking Corpse", Count: 8}, {Name: "Shock", Count: 4}, {Name: "Swamp", Count: 14}, {Name: "Mountain", Count: 6},
	}, opp: []cardset.Entry{
		{Name: "Grizzly Bears", Count: 10}, {Name: "Shock", Count: 10}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	}},
	{card: "Genju of the Fields", deck: []cardset.Entry{
		{Name: "Genju of the Fields", Count: 8}, {Name: "Savannah Lions", Count: 6}, {Name: "Plains", Count: 26},
	}, opp: []cardset.Entry{
		{Name: "Stone Rain", Count: 12}, {Name: "Grizzly Bears", Count: 8}, {Name: "Mountain", Count: 20},
	}},
}

// gpReach is what the games did, read from gorge's log.
type gpReach struct {
	resolved  int // the card resolved (Resolve of a spell of that name)
	exhumed   int // Exhume: a creature moved graveyard -> battlefield during its resolution
	exhumedBy [2]int
	bothSides int // Exhume: resolutions that returned a creature for each seat
	returned  int // Genju: the Genju moved graveyard -> hand
}

func gpScan(g *GorgeSide, card string) gpReach {
	var r gpReach
	name := func(id state.ObjID) string {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	evs := g.E.L.Events
	for i, ev := range evs {
		switch {
		case ev.Kind == events.Resolve && name(ev.Obj) == card:
			r.resolved++
			if card != "Exhume" {
				continue
			}
			var seats [2]bool
			for _, w := range evs[i+1:] {
				if w.Kind == events.Priority || w.Kind == events.TurnChange {
					break
				}
				if w.Kind == events.MoveZone && w.From == state.ZGraveyard && w.To == state.ZBattlefield {
					r.exhumed++
					if o := g.E.G.Obj(w.Obj); o != nil && int(o.Owner) < 2 {
						seats[o.Owner] = true
						r.exhumedBy[o.Owner]++
					}
				}
			}
			if seats[0] && seats[1] {
				r.bothSides++
			}
		case ev.Kind == events.MoveZone && ev.From == state.ZGraveyard && ev.To == state.ZHand && name(ev.Obj) == "Genju of the Fields":
			r.returned++
		}
	}
	return r
}

func TestMBCZoneMoves3Lowers(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mbczonemoves3")
	if err != nil {
		t.Fatalf("mbczonemoves3 tables: %v (run make dirac-tables)", err)
	}
	for _, mu := range gpMatchups {
		t.Run(strings.ReplaceAll(mu.card, " ", ""), func(t *testing.T) {
			t.Parallel()
			decks := [2][]cardset.Entry{mu.deck, mu.opp}
			var gd, sides [2][]*cards.Card
			for s := range decks {
				gd[s] = expandEntries(t, reg, decks[s])
				if len(gd[s]) != 40 {
					t.Fatalf("deck %d: %d cards", s, len(gd[s]))
				}
			}
			tbl, _, _, err := set.Matchup(decks[0], decks[1])
			if err != nil {
				t.Fatalf("%s: the pool's table: %v", mu.card, err)
			}
			var tot gpReach
			for seed := uint64(1); seed <= 40; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchupSide(reg, [2]string{"gp-" + mu.card + "-a", "gp-" + mu.card + "-b"}, gd, sides, 1)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				g, err := NewGorgeSide(m, seed, cfg, nil)
				if err != nil {
					t.Fatal(err)
				}
				pol := poisonhighPolicy(seed, map[string]int{}, map[string]int{}) // casts whenever it can
				res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
				CloseSide(k)
				if strings.HasPrefix(res.Stall, KernelStallPrefix) {
					t.Errorf("%s seed %d: kernel stalled: %s", mu.card, seed, res.Stall)
				}
				if d := res.Div; d != nil {
					t.Errorf("%s seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mu.card, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
				r := gpScan(g, mu.card)
				tot.resolved += r.resolved
				tot.exhumed += r.exhumed
				tot.bothSides += r.bothSides
				tot.returned += r.returned
				tot.exhumedBy[0] += r.exhumedBy[0]
				tot.exhumedBy[1] += r.exhumedBy[1]
			}
			t.Logf("%s: %+v", mu.card, tot)
			switch mu.card {
			case "Exhume":
				if tot.resolved < 10 || tot.bothSides < 3 || tot.exhumedBy[0] < 3 || tot.exhumedBy[1] < 3 {
					t.Errorf("reach %+v: want Exhume resolved at least 10 times, and at least 3 resolutions that returned a creature for each seat", tot)
				}
			case "Genju of the Fields":
				if tot.returned < 3 {
					t.Errorf("reach %+v: want the Genju returned from the graveyard to the hand at least 3 times", tot)
				}
			}
		})
	}
}
