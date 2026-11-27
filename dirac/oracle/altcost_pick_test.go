package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestAltCostPickGrantLockstep: Dream Halls' "Rather than pay the mana cost
// for a spell, its controller may discard a card that shares a color with that
// spell" (gorge alternativeCosts' battlefield grant, read with no ValidPlayer$
// so it reaches an opponent's spell too; kernel cast_altgrant.go). Both hands
// hold a red spell, another red card that shares its colour, and a green card
// that does not; neither seat has mana, so the granted discard cast is the
// only cast either can make. The policy takes every granted alternative cast
// and the colour-sharing discard, so the run fails if the offer or the pick
// differs between the engines; the counters require the opponent's spell to
// have reached the offer as well.
func TestAltCostPickGrantLockstep(t *testing.T) {
	t.Parallel()
	sc, err := ParseScenario([]byte(`{"format":"gorge-start-1","focus":"Dream Halls","players":2,"active":0,"max_turns":6,"opponent":"passive",
 "setup":{
  "p0":{"battlefield":["Dream Halls"],"hand":["Lightning Bolt","Raging Goblin","Grizzly Bears"]},
  "p1":{"hand":["Lightning Bolt","Raging Goblin","Grizzly Bears"]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	c, err := OpenCorpus(corpusDirOrSkip(t), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m, err := c.ScenarioMatchup(sc, 0)
	if err != nil {
		t.Fatal(err)
	}
	var names [2][]string
	for p := range names {
		for _, card := range m.SeatDeck(p) {
			names[p] = append(names[p], cardName(card))
		}
	}
	set := cardset.Raw(printedDelveSet(t, names))
	entries := func(d []*cards.Card) []cardset.Entry {
		var out []cardset.Entry
		for _, card := range d {
			n := cardName(card)
			i := slices.IndexFunc(out, func(e cardset.Entry) bool { return e.Name == n })
			if i < 0 {
				out = append(out, cardset.Entry{Name: n})
				i = len(out) - 1
			}
			out[i].Count++
		}
		return out
	}
	tbl, _, _, err := set.Matchup(entries(m.SeatDeck(0)), entries(m.SeatDeck(1)))
	if err != nil {
		t.Fatal(err)
	}
	discardKind, ok := canon.ChooseCode("discard")
	if !ok {
		t.Fatal("no canonical discard choose kind")
	}
	rc := RunConfig{Config: Config{}, MaxDecisions: 2000, MaxTurns: 6}
	var offers, picks, nonShare [2]int
	const seeds = 4
	next := uint64(1)
	for i := 0; i < seeds; i++ {
		seed := sc.GameSeed(next)
		next = seed + 1
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		rp := RandomPolicy(seed)
		pol := PolicyFunc(func(p *PickCtx) Canon {
			for _, a := range p.D.Options {
				if a.Kind() == ActCast && a.Mode()&0xF == 0 && a.Mode()>>4 >= 1 {
					offers[p.D.Player]++
					return a
				}
			}
			if p.D.Kind == DChoose && len(p.D.Options) > 0 && p.D.Options[0].Kind() == ActChoose+CanonKind(discardKind) {
				for _, a := range p.D.Options {
					if refCardName(p.G, a.Src()) == "Grizzly Bears" {
						nonShare[p.D.Player]++
					}
				}
				for _, a := range p.D.Options {
					if refCardName(p.G, a.Src()) != "Grizzly Bears" {
						picks[p.D.Player]++
						return a
					}
				}
				return p.D.Options[0]
			}
			return rp.Pick(p)
		})
		res := Lockstep(m, seed, k, pol, rc)
		CloseSide(k)
		if !res.Clean() {
			if res.Div == nil {
				t.Fatalf("seed %d: not clean without a divergence: stall %q", seed, res.Stall)
			}
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v (%s)\nref %v\nker %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail, res.Div.RefOptions, res.Div.KerOptions)
		}
	}
	t.Logf("%d seeds: granted alternative casts offered p0 %d p1 %d, colour-share discards taken p0 %d p1 %d, non-sharing candidates offered p0 %d p1 %d",
		seeds, offers[0], offers[1], picks[0], picks[1], nonShare[0], nonShare[1])
	for p := range offers {
		if offers[p] == 0 {
			t.Errorf("p%d was never offered Dream Halls' alternative cast", p)
		}
		if picks[p] == 0 {
			t.Errorf("p%d never paid Dream Halls' alternative cast with a colour-sharing discard", p)
		}
		if nonShare[p] != 0 {
			t.Errorf("p%d was offered %d non-sharing discard candidates (Grizzly Bears), want none", p, nonShare[p])
		}
	}
}
