package main

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// manaDeckCard is one deck card as the mana-base check reads it.
type manaDeckCard struct {
	face *cards.Face
	land bool
}

// isManaSource reports whether the card is a source of colour col (0..4,
// WUBRG): a land, or a nonland of mana value <= 3, whose mana abilities make
// the colour (an any-colour producer counts for every colour).
func (c manaDeckCard) isManaSource(col int) bool {
	if !c.land && c.face.ManaValue() > 3 {
		return false
	}
	mp := c.face.ManaProduction()
	return mp.Any || mp.Colour[col] > 0
}

// colourRuleHolds applies the mana-base colour rule to one deck of lands
// lands: per colour with pips, a colour under 15% of the pips needs
// max(3, 0.18·lands) sources, otherwise 0.35, 0.45 or 0.55 of the lands as
// its heaviest cost has 1, 2 or 3+ pips of it.
func colourRuleHolds(deck []manaDeckCard, basics [5]int, lands int) bool {
	var pips, heaviest [5]int
	total := 0
	for _, c := range deck {
		for col := 0; col < 5; col++ {
			p := int(cards.ManaCostChroma(c.face.ManaCost, 1<<col))
			pips[col] += p
			total += p
			heaviest[col] = max(heaviest[col], p)
		}
	}
	for col := 0; col < 5; col++ {
		if pips[col] == 0 {
			continue
		}
		src := basics[col]
		for _, c := range deck {
			if c.isManaSource(col) {
				src++
			}
		}
		l := float64(lands)
		var need float64
		switch {
		case float64(pips[col]) < 0.15*float64(total):
			need = max(3, 0.18*l)
		case heaviest[col] >= 3:
			need = 0.55 * l
		case heaviest[col] == 2:
			need = 0.45 * l
		default:
			need = 0.35 * l
		}
		if float64(src) < need {
			return false
		}
	}
	return true
}

// TestDraftsimCommanderManaBase: 100 Commander-draft events at 60 cards
// (400 decks) build decks with at least 23 lands (a deck short of mana
// producers, or of the sources one colour needs, trades its weakest spells
// for more), at least 25 mana sources (lands plus MV <= 3 producers), a mean
// of at most 25 lands, and the per-colour source rule in at least 95% of the
// decks. The mean lands were 17.5 and the sources 18.6 with the Limited
// land count.
func TestDraftsimCommanderManaBase(t *testing.T) {
	dir := corpusReady(t)
	o := options{set: "MBC", collation: "draft", format: event.FormatCommander, deckSize: 60,
		structure: event.StructureNone, seed: 881000001, events: 100, players: "bot",
		drafter: "rating-colour", builder: "best23", dir: dir}
	var out strings.Builder
	res, err := run(o, &out)
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out.String())
	}
	reg, err := testutil.OpenCorpusRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.events) != 100 {
		t.Fatalf("run produced %d events, want 100", len(res.events))
	}
	decks, fewLands, fewSources, badColour, sumLands, sumSources := 0, 0, 0, 0, 0, 0
	for e, er := range res.events {
		ev, err := event.ReplayLog(er.log)
		if err != nil {
			t.Fatalf("event %d: %v", e, err)
		}
		for s := 0; s < 4; s++ {
			d, ok := ev.SavedDeck(s)
			if !ok {
				t.Fatalf("event %d seat %d saved no deck", e, s)
			}
			names := map[string]string{}
			for _, ref := range ev.Pool(s) {
				names[ref.ID] = ref.Name
			}
			var deck []manaDeckCard
			lands, producers := 0, 0
			for _, id := range d.Main {
				c, ok := reg.Lookup(names[id])
				if !ok || len(c.Faces) == 0 {
					t.Fatalf("event %d seat %d: card %q not in the corpus", e, s, names[id])
				}
				mc := manaDeckCard{face: c.Faces[0], land: c.Faces[0].IsLand()}
				deck = append(deck, mc)
				switch {
				case mc.land:
					lands++
				case mc.face.ManaValue() <= 3 && !mc.face.ManaProduction().IsZero():
					producers++
				}
			}
			for _, n := range d.Basics {
				lands += n
			}
			if d.Total() != 60 {
				t.Fatalf("event %d seat %d: %d cards, want 60", e, s, d.Total())
			}
			decks++
			sumLands += lands
			sumSources += lands + producers
			if lands < 23 {
				fewLands++
			}
			if lands+producers < 25 {
				fewSources++
			}
			if !colourRuleHolds(deck, d.Basics, lands) {
				badColour++
			}
		}
	}
	t.Logf("%d decks: mean lands %.1f, mean sources %.1f; under 23 lands in %d, under 25 sources in %d, colour rule broken in %d",
		decks, float64(sumLands)/float64(decks), float64(sumSources)/float64(decks), fewLands, fewSources, badColour)
	if fewLands != 0 {
		t.Errorf("%d of %d decks have fewer than 23 lands", fewLands, decks)
	}
	if fewSources != 0 {
		t.Errorf("%d of %d decks have fewer than 25 mana sources", fewSources, decks)
	}
	if sumLands > 25*decks {
		t.Errorf("mean lands %.1f, want at most 25", float64(sumLands)/float64(decks))
	}
	if badColour*100 > decks*5 {
		t.Errorf("the colour rule holds in %d of %d decks, want at least 95%%", decks-badColour, decks)
	}
}
