package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// hbotPairs plays this call's recorded games of a synthetic pool (pinnedHBotGames;
// recorded from the pairs khbot deals at seed 78 on the auto-pay surface, with only
// the refused cards left out) in lockstep with gorge's heuristic bot, requires each
// to reach every condition in reached, and fails on any compared disagreement whose
// description contains want.
func hbotPairs(t *testing.T, poolName string, want string, reached ...hbotCond) {
	t.Helper()
	run := hbotPinRun{Pool: poolName, Set: cardset.AllowUnverified(cardsettest.Synth(t, poolName)), FactsFromSet: true, ScanSeed: 78}
	pinnedHBotGames(t, run, Config{GorgeAutopaySurface: true, PassElision: true}, func(r ParityRec) bool {
		return !r.Unoffered && !r.Agree && strings.Contains(r.Where, want)
	}, reached...)
}

// TestHBotPicksTheManaColourTheCardNeeds: a resolving effect's "any combination
// of colours" pick takes first the colour the card the tap gate would aim at still
// needs (policy.go's chooseManaColour, CR 106.3), and the rest in offered order.
func TestHBotPicksTheManaColourTheCardNeeds(t *testing.T) {
	hbotPairs(t, "std-blb", "choose:mana", answersWith("choose:mana"))
}

// TestHBotListsAChangelingsPrintedTypesFirst: a creature-type choice lists first
// the creature types printed on the cards its chooser owns (rules.creatureTypeOptions
// reads the face), so a changeling there adds only its printed types, not every
// type it has (CR 702.73a); the bot takes the first option (Patchwork Banner).
func TestHBotListsAChangelingsPrintedTypesFirst(t *testing.T) {
	hbotPairs(t, "std-blb", "choose:type", answersWith("choose:type"))
}

// TestHBotReadsAHandCardsDefinedPower: the bot values a creature card in hand
// at its characteristic-defining power, which applies in every zone (CR 604.3)
// and which gorge's cardWorth reads, whatever amount the CDA counts (The
// Traveler and Captain Pike count differently named lands you control).
func TestHBotReadsAHandCardsDefinedPower(t *testing.T) {
	hbotPairs(t, "std-trk", "cast(", inHand("The Traveler", "Captain Pike"))
}

// TestHBotTakesTheDiscardUnlessAlternative: "discard two cards unless you discard
// a creature card" offers the unless alternative first in gorge (effects
// discard_unless), and the bot takes the first offer (CR 701.9b); the kernel's
// canonical order lists it second (Winternight Stories).
func TestHBotTakesTheDiscardUnlessAlternative(t *testing.T) {
	hbotPairs(t, "std-tdm", "choose:unless", answersWith("choose:unless"))
}
