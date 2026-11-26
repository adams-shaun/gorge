package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Cut a Deal ("Each opponent draws a card, then you draw a card for each
// opponent who drew a card this way") lowers in lockstep: the opponent's draw
// remembers every card it moved (RememberDrawn$ AllReplaced), the second Draw
// counts the opponents holding a remembered card
// (PlayerCountOpponents$HasPropertyIsRememberedOrController: one or zero in a
// two-player game) and the Cleanup clears the set. Stinkweed Imp's dredge
// replaces a draw with a return from the graveyard, so a replaced draw is
// still remembered (the card the replacement moved); Tome Scour fills the
// graveyards.
func cutADealDeck() []string {
	var d []string
	add := func(n string, c int) {
		for range c {
			d = append(d, n)
		}
	}
	add("Cut a Deal", 6)
	add("Stinkweed Imp", 7)
	add("Tome Scour", 7)
	add("Plains", 10)
	add("Forest", 2)
	add("Island", 8)
	return d
}

// cutADealPolicy plays lands, then casts Tome Scour and the Imps while no
// graveyard holds an Imp, and Cut a Deal once one does, so the opponent's draw
// meets the dredge window; any other decision (the dredge answer included) is
// random.
func cutADealPolicy(m *Matchup, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind != DPriority || seed%5 == 0 || c.G == nil {
			return rnd.Pick(c)
		}
		imp := false
		for p := state.PlayerID(0); p < 2; p++ {
			for _, id := range c.G.E.G.Zone(state.ZGraveyard, p) {
				if o := c.G.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Stinkweed Imp" {
					imp = true
				}
			}
		}
		var deal, other, land []Canon
		for _, o := range c.D.Options {
			switch {
			case o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == "Cut a Deal":
				deal = append(deal, o)
			case o.Kind() == ActCast:
				other = append(other, o)
			case o.Kind() == ActPlayLand:
				land = append(land, o)
			}
		}
		pick := func(opts []Canon) Canon {
			d := *c.D
			d.Options = opts
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		switch {
		case len(land) > 0:
			return pick(land)
		case imp && len(deal) > 0:
			return pick(deal)
		case len(other) > 0:
			return pick(other)
		}
		return rnd.Pick(c)
	})
}

type cutADealObs struct {
	resolved int // Cut a Deal resolutions in gorge's log
	dredged  int // a Stinkweed Imp moved graveyard -> hand (a dredge-replaced draw)
	inDeal   int // a dredge inside a Cut a Deal's resolution (its opponent draw was replaced)
}

func (o *cutADealObs) tally(g *GorgeSide) {
	is := func(ev events.Event, name string) bool {
		ob := g.E.G.Obj(ev.Obj)
		return ob != nil && ob.Face() != nil && ob.Face().Name == name
	}
	dredge := func(ev events.Event) bool {
		return ev.Kind == events.MoveZone && ev.From == state.ZGraveyard && ev.To == state.ZHand && is(ev, "Stinkweed Imp")
	}
	evs := g.E.L.Events
	for i, ev := range evs {
		switch {
		case dredge(ev):
			o.dredged++
		case ev.Kind == events.Resolve && is(ev, "Cut a Deal"):
			o.resolved++
			// the opponent's draw follows the Resolve; the replacement
			// names itself in an event ("Dredge 5 (mill, then return ...)")
			for j := i + 1; j < len(evs) && j <= i+5; j++ {
				if strings.HasPrefix(evs[j].Text, "Dredge") {
					o.inDeal++
					break
				}
			}
		}
	}
}

func runCutADeal(t *testing.T, seeds int) (games, divs, stalls int, obs *cutADealObs) {
	t.Helper()
	obs = &cutADealObs{}
	reg := synthReg(t)
	deck := cutADealDeck()
	if len(deck) != 40 {
		t.Fatalf("deck is %d cards", len(deck))
	}
	set := cardset.AllowUnverified(printedDelveSet(t, [2][]string{deck, deck}))
	if i, ok := set.Lookup("Cut a Deal"); !ok {
		t.Fatal("Cut a Deal is not in the table: it did not lower")
	} else if got := set.Status(i); got == cardset.StatusNotLowered {
		t.Fatalf("Cut a Deal status = %v, refusal = %q", got, set.Reason(i))
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(deck), entries(deck))
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, n := range deck {
			c, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not in the corpus", n)
			}
			decks[s] = append(decks[s], c)
		}
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"cutadeal-a", "cutadeal-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, cutADealPolicy(m, seed), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 25})
		obs.tally(g)
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Errorf("seed %d: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return games, divs, stalls, obs
}

// TestCutADealLockstep plays Cut a Deal games: the plain draw, the
// dredge-replaced draw and the cleanup agree in both engines.
func TestCutADealLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, obs := runCutADeal(t, 96)
	t.Logf("%d games, %d divergences, %d stalls, %d resolutions, %d dredges, %d inside a resolution", games, divs, stalls, obs.resolved, obs.dredged, obs.inDeal)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if obs.resolved == 0 {
		t.Fatal("no Cut a Deal resolved: the draw chain was never reached")
	}
	if obs.inDeal == 0 {
		t.Fatal("no Cut a Deal draw was dredge-replaced: the replacement-remembered arm never ran")
	}
}
