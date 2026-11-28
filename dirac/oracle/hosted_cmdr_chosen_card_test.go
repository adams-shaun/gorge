package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Party Thrasher's Effect (DB$ Effect | StaticAbilities$ STPlay |
// ExileOnMoved$ Exile, no Duration$) grants "you may play the chosen card"
// over the card its Dig exiles and picks, and its printed static grants
// Convoke to noncreature spells cast from exile (CR 702.51a). Both chosen
// shapes play decision for decision against gorge: a chosen creature is cast
// from exile as an ordinary may-play cast, and a chosen instant is cast from
// exile with the convoke grant -- the grant is evaluated on the card while it
// is still in exile (CR 601.2a puts the spell on the stack, so the cast in
// progress is a noncreature spell cast from exile), which is where the two
// engines disagreed.
//
// The cases build their kernel table with printedDelveSetIn, so a case is RED
// while kgen refuses the Effect shape. A case with a passed predicate also
// FAILS unless some seed actually exiles and casts the chosen card from
// exile, and the instant case requires the cast to be paid by convoking a
// Party Thrasher, so a game that never exercised the grant is not a pass.
// Every seed is played (the shared shape helper stops once it has seen the
// mechanism; the offer divergence this case guards is later than that).
func TestHostedCmdrChosenCardMayPlay(t *testing.T) {
	t.Parallel()
	oppDeck := append(deckOf(8, "Grizzly Bears"), deckOf(32, "Forest")...)
	goblinDeck := append(append(deckOf(8, "Party Thrasher"), deckOf(8, "Raging Goblin")...), deckOf(24, "Mountain")...)
	boltDeck := append(append(deckOf(8, "Party Thrasher"), deckOf(8, "Lightning Bolt")...), deckOf(24, "Mountain")...)

	runChosenCardShape(t, "party_thrasher_chosen_creature",
		[2]string{"hcc-goblin-a", "hcc-goblin-b"}, [2][]string{goblinDeck, oppDeck},
		chosenCardCastFromExile("Raging Goblin"))
	runChosenCardShape(t, "party_thrasher_chosen_instant_convoke",
		[2]string{"hcc-bolt-a", "hcc-bolt-b"}, [2][]string{boltDeck, oppDeck},
		chosenCardConvokeCastFromExile("Lightning Bolt", "Party Thrasher"))
}

// runChosenCardShape plays every seed of the chosen-card decks in lockstep,
// gorge leading, and fails on the first divergence, on fewer than three real
// games, or when no game reached the case's mechanism. It is the shared
// hostedCmdrShape loop without its early stop, so a mechanism reached on an
// early seed cannot hide an offer divergence on a later one.
func runChosenCardShape(t *testing.T, name string, seats [2]string, decks [2][]string, reached func(*GorgeSide) bool) {
	t.Helper()
	dir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	set := printedDelveSetIn(t, dir, decks)
	var gd [2][]*cards.Card
	for s := range decks {
		for _, n := range decks[s] {
			card, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not in the corpus", n)
			}
			gd[s] = append(gd[s], card)
		}
		if len(gd[s]) != 40 {
			t.Fatalf("%s: seat %d deck has %d cards, want 40", name, s, len(gd[s]))
		}
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(decks[0]), entries(decks[1]))
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{}
	seen, played := false, 0
	for seed := uint64(1); seed <= 80; seed++ {
		m, err := NewMatchup(reg, seats, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40}
		rc.Finish = func(g *GorgeSide) {
			if !seen && reached(g) {
				seen = true
			}
		}
		res := Lockstep(m, seed, k, hostedCmdrKwCostsPolicy(m, hostedCmdrCase{carrier: "Party Thrasher"}, seed), rc)
		if d := res.Div; d != nil {
			CloseSide(k)
			t.Fatalf("%s seed %d: divergence at %d: %s %s %v\ngorge %v\ndirac %v", name, seed, d.At, d.What, d.Field, d.Diff, d.RefOptions, d.KerOptions)
		}
		if res.Decisions > 100 {
			played++
		}
		CloseSide(k)
	}
	if played < 3 {
		t.Fatalf("%s: only %d seeds played a real game", name, played)
	}
	if !seen {
		t.Fatalf("%s: the chosen-card grant was not reached on any seed, so the lockstep is vacuous", name)
	}
}

// chosenCardCastFromExile reports whether name was put on the stack from
// exile (the cast a may-play grant makes) in this game.
func chosenCardCastFromExile(name string) func(g *GorgeSide) bool {
	return func(g *GorgeSide) bool {
		for i := range g.E.L.Events {
			e := &g.E.L.Events[i]
			if e.Kind != events.PutOnStack || e.From != state.ZExile {
				continue
			}
			o := g.E.G.Obj(e.Obj)
			if o != nil && o.Face() != nil && o.Face().Name == name {
				return true
			}
		}
		return false
	}
}

// chosenCardConvokeCastFromExile reports whether name was put on the stack
// from exile while convoker was tapped: the convoke announcement taps every
// chosen creature before the spell is pushed (pay.EmitConvokeTaps), so a
// tapped convoker at that push is the grant paid for the cast.
func chosenCardConvokeCastFromExile(name, convoker string) func(g *GorgeSide) bool {
	return func(g *GorgeSide) bool {
		tapped := map[state.ObjID]bool{}
		for i := range g.E.L.Events {
			e := &g.E.L.Events[i]
			switch e.Kind {
			case events.Tap:
				tapped[e.Obj] = true
			case events.Untap:
				tapped[e.Obj] = false
			case events.PutOnStack:
				if e.From != state.ZExile {
					continue
				}
				o := g.E.G.Obj(e.Obj)
				if o == nil || o.Face() == nil || o.Face().Name != name {
					continue
				}
				for id, down := range tapped {
					if !down {
						continue
					}
					if c := g.E.G.Obj(id); c != nil && c.Face() != nil && c.Face().Name == convoker {
						return true
					}
				}
			}
		}
		return false
	}
}
