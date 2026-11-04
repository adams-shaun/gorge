package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/search/hbot"
	"github.com/adams-shaun/gorge/engine/cards"
)

// hbotPairs plays each listed pairing (seed 78, the auto-pay surface, as khbot
// does) of a synthetic pool in lockstep with gorge's heuristic bot and fails on
// any compared disagreement whose description contains want. The decks are dealt
// as khbot deals them, with only the refused cards left out, so pair N here is
// khbot's -first-pair N.
func hbotPairs(t *testing.T, poolName string, pairs []int, want string) {
	t.Helper()
	c := &Corpus{Reg: synthReg(t), decks: map[string][]*cards.Card{}}
	set := cardset.AllowUnverified(cardsettest.Synth(t, poolName))
	pool, files, err := SynthPool(c.Reg, set, 1, SynthDecks)
	if err != nil {
		t.Fatal(err)
	}
	c.AddDecks(files)
	mk := func(tbl *kop.Table) KernelBot { return hbot.New(HBotFactsSet(c.Reg, tbl, nil, set)) }
	for _, i := range pairs {
		p := PairingsFrom(pool, i, 1, 78)[0]
		res, err := HBotParityGameOpts(c, p.A, p.B, p.Seed, Config{GorgeAutopaySurface: true, PassElision: true}, 20000, mk, HBotParityOptions{Set: set})
		if err != nil {
			t.Fatalf("%s pair %d: %v", poolName, i, err)
		}
		for _, r := range res.Recs {
			if !r.Unoffered && !r.Agree && strings.Contains(r.Where, want) {
				t.Errorf("%s pair %d: %s", poolName, i, r.Where)
			}
		}
	}
}

// TestHBotPicksTheManaColourTheCardNeeds: a resolving effect's "any combination
// of colours" pick takes first the colour the card the tap gate would aim at still
// needs (policy.go's chooseManaColour, CR 106.3), and the rest in offered order.
func TestHBotPicksTheManaColourTheCardNeeds(t *testing.T) {
	hbotPairs(t, "std-blb", []int{0, 57}, "choose:mana")
}

// TestHBotListsAChangelingsPrintedTypesFirst: a creature-type choice lists first
// the creature types printed on the cards its chooser owns (rules.creatureTypeOptions
// reads the face), so a changeling there adds only its printed types, not every
// type it has (CR 702.73a); the bot takes the first option (Patchwork Banner).
func TestHBotListsAChangelingsPrintedTypesFirst(t *testing.T) {
	hbotPairs(t, "std-blb", []int{53}, "choose:type")
}
