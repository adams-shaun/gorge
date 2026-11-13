package oracle

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The MBC zone-moves-13 family in lockstep, one matchup per card of the
// mbczonemoves13 fixture pool, each seat 0 deck built around it:
//
//   - Future Sight (spelling other:MayLookAt shape): the library-top look
//     beside MayPlay$ on one static, so lands are played and spells cast
//     from the top of the library;
//   - Cunning Wish (param:ChangeZone.Reveal): the revealed pick of an
//     instant from a real sideboard into the hand;
//   - Doomsday (Origin "Graveyard,Library" on its search and its sweep,
//     RearrangeTopOfLibrary NumCards LibrarySizeOf(You)): five cards of the
//     library and graveyard on top unshuffled, the rest of both exiled, the
//     five ordered.
//
// The expected answer at every decision is gorge's. Without the lowering the
// pool's table refuses the card and the matchup cannot be built.

// zm13Opp is the opponent of every matchup: creatures and burn.
var zm13Opp = []cardset.Entry{
	{Name: "Grizzly Bears", Count: 10}, {Name: "Shock", Count: 10}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
}

type zm13Matchup struct {
	card string
	deck []cardset.Entry
	side []cardset.Entry // seat 0's sideboard
}

var zm13Matchups = []zm13Matchup{
	{card: "Future Sight", deck: []cardset.Entry{
		{Name: "Future Sight", Count: 8}, {Name: "Opt", Count: 6}, {Name: "Unsummon", Count: 4}, {Name: "Island", Count: 22},
	}},
	{card: "Cunning Wish", deck: []cardset.Entry{
		{Name: "Cunning Wish", Count: 8}, {Name: "Opt", Count: 6}, {Name: "Counterspell", Count: 4}, {Name: "Island", Count: 22},
	}, side: []cardset.Entry{
		// instants it may fetch, and a sorcery and a land it may not
		{Name: "Unsummon", Count: 1}, {Name: "Brainstorm", Count: 1}, {Name: "Shock", Count: 1},
		{Name: "Divination", Count: 1}, {Name: "Island", Count: 1},
	}},
	{card: "Doomsday", deck: []cardset.Entry{
		{Name: "Doomsday", Count: 8}, {Name: "Dark Ritual", Count: 4}, {Name: "Night's Whisper", Count: 4},
		{Name: "Walking Corpse", Count: 4}, {Name: "Swamp", Count: 20},
	}},
}

// zm13Reach is what the games did with the card, read from gorge's log.
type zm13Reach struct {
	resolved  int // the card resolved (Resolve of a spell of that name)
	fromTop   int // Future Sight: a card cast or played from seat 0's library
	wished    int // Cunning Wish: a sideboard card moved into a hand
	sweptLib  int // Doomsday: a library card exiled while it resolved
	sweptGy   int // Doomsday: a graveyard card exiled while it resolved
	gyToLib   int // Doomsday: a graveyard card put into the library while it resolved
	arranged  int // Doomsday: rearrange picks with two or more cards to order
	libraryOK int // Doomsday: ... with at most five cards left in the library (the sweep exiled the rest)
}

func zm13Scan(g *GorgeSide, card string) zm13Reach {
	var r zm13Reach
	name := func(id state.ObjID) string {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	evs := g.E.L.Events
	for i, ev := range evs {
		switch {
		case ev.Kind == events.MoveZone && ev.From == state.ZLibrary && (ev.To == state.ZStack || ev.To == state.ZBattlefield) && ev.Player == 0:
			r.fromTop++
		case ev.Kind == events.MoveZone && ev.From == state.ZSideboard && ev.To == state.ZHand:
			r.wished++
		case ev.Kind == events.Resolve && name(ev.Obj) == card:
			r.resolved++
			if card != "Doomsday" {
				continue
			}
			for _, w := range evs[i+1:] {
				if w.Kind == events.Priority || w.Kind == events.TurnChange {
					break
				}
				if w.Kind != events.MoveZone {
					continue
				}
				switch {
				case w.From == state.ZLibrary && w.To == state.ZExile:
					r.sweptLib++
				case w.From == state.ZGraveyard && w.To == state.ZExile:
					r.sweptGy++
				case w.From == state.ZGraveyard && w.To == state.ZLibrary:
					r.gyToLib++
				}
			}
		}
	}
	return r
}

func TestMBCZoneMoves13Lowers(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mbczonemoves13")
	if err != nil {
		t.Fatalf("mbczonemoves13 tables: %v (run make dirac-tables)", err)
	}
	for _, mu := range zm13Matchups {
		t.Run(strings.ReplaceAll(mu.card, " ", ""), func(t *testing.T) {
			t.Parallel()
			decks := [2][]cardset.Entry{mu.deck, zm13Opp}
			var gd, sides [2][]*cards.Card
			for s := range decks {
				gd[s] = expandEntries(t, reg, decks[s])
				if len(gd[s]) != 40 {
					t.Fatalf("deck %d: %d cards", s, len(gd[s]))
				}
			}
			sides[0] = expandEntries(t, reg, mu.side)
			tbl, _, _, err := set.Matchup(append(append([]cardset.Entry(nil), decks[0]...), mu.side...), decks[1])
			if err != nil {
				t.Fatalf("%s: the pool's table: %v", mu.card, err)
			}
			var tot zm13Reach
			games, libOK, arranged := 0, 0, 0
			for seed := uint64(1); seed <= 40; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchupSide(reg, [2]string{"zm13-" + mu.card + "-a", "zm13-" + mu.card + "-b"}, gd, sides, 1)
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
				// Doomsday's rearrange: gorge's KArrange with Doomsday as its
				// source. Count the ordering picks with two or more cards
				// left, and check the sweep left at most five in the library.
				wrapped := PolicyFunc(func(c *PickCtx) Canon {
					if g := c.G; g != nil && g.fr != nil && g.fr.d != nil && len(c.D.Options) >= 2 {
						if d := g.fr.d; d.Kind == decision.KArrange {
							if o := g.E.G.Obj(d.Source); o != nil && o.Face() != nil && o.Face().Name == "Doomsday" {
								arranged++
								if len(g.E.G.Zone(state.ZLibrary, d.Player)) <= 5 {
									libOK++
								}
							}
						}
					}
					return pol.Pick(c)
				})
				res := LockstepLed(m, seed, g, k, wrapped, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
				CloseSide(k)
				games++
				if strings.HasPrefix(res.Stall, KernelStallPrefix) {
					t.Errorf("%s seed %d: kernel stalled: %s", mu.card, seed, res.Stall)
				}
				if d := res.Div; d != nil {
					t.Errorf("%s seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mu.card, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
				r := zm13Scan(g, mu.card)
				tot.resolved += r.resolved
				tot.fromTop += r.fromTop
				tot.wished += r.wished
				tot.sweptLib += r.sweptLib
				tot.sweptGy += r.sweptGy
				tot.gyToLib += r.gyToLib
			}
			tot.libraryOK, tot.arranged = libOK, arranged
			t.Logf("%s: %d games; %+v", mu.card, games, tot)
			want := fmt.Sprintf("%s resolved at least 10 times", mu.card)
			if tot.resolved < 10 {
				t.Errorf("reach %+v: want %s", tot, want)
			}
			switch mu.card {
			case "Future Sight":
				if tot.fromTop < 10 {
					t.Errorf("reach %+v: want at least 10 cards cast or played from the top of the library", tot)
				}
			case "Cunning Wish":
				if tot.wished < 10 {
					t.Errorf("reach %+v: want at least 10 instants wished from the sideboard", tot)
				}
			case "Doomsday":
				if tot.sweptLib < 100 || tot.sweptGy < 5 || tot.gyToLib < 1 || tot.arranged < 10 || tot.libraryOK != tot.arranged {
					t.Errorf("reach %+v: want library and graveyard sweeps, a graveyard card put back, and at least 10 ordering picks, each over a library of at most five", tot)
				}
			}
		})
	}
}
