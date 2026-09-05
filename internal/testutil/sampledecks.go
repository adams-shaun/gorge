package testutil

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/cards"
)

// colours cycles WUBRG's basic-land shapes in a fixed order, so seat i's
// deck is deterministically this colour and no two seats within the first
// five are mirror images of each other (Ruling P9).
var colours = [...]struct{ letter, land string }{
	{"R", "Mountain"}, {"G", "Forest"}, {"W", "Plains"}, {"U", "Island"}, {"B", "Swamp"},
}

// SampleDecks returns n player names and n ~40-card decks authored inline,
// so the fuzz gate runs on a clean checkout with no corpus fetched (Ruling
// P9). Each deck is one colour's basics, a vanilla creature sized by seat
// (so seats are not mirror images of each other), a burn spell that targets
// a creature or a player, a pump spell, a permanent with a mandatory
// triggered ability, and a permanent with an OptionalDecider$ You triggered
// ability -- the shapes rules/trigger_order_test.go already exercises,
// re-authored here rather than imported (testutil cannot import rules: the
// fuzz test using this is package rules itself). SampleDecks is a pure
// function of n: no randomness, so the same n always yields the same decks
// in the same order.
func SampleDecks(t testing.TB, n int) (names []string, decks [][]*cards.Card) {
	t.Helper()
	names = make([]string, n)
	decks = make([][]*cards.Card, n)
	for i := 0; i < n; i++ {
		names[i] = string(rune('a' + i))
		decks[i] = deckFor(t, i)
	}
	return names, decks
}

// deckFor builds seat i's 40-card deck: 17 basics, 7 vanilla creatures, and
// 4 copies each of a burn spell, a pump spell, a mandatory-trigger
// permanent and an optional-trigger permanent (17+7+4+4+4+4 = 40). Every
// nonland card costs a single pip of the deck's own colour (Ruling T25-b,
// fix round 1: the creature used to cost `1 %s`, two mana the bot's own
// priority policy could never assemble at sorcery speed -- see bot.go's
// isMain gating for the other half of that fix), so the bot's simple
// priority policy can actually cast them and exercise KAttackers/KBlockers
// and real combat damage during the fuzz gate, not only KPriority/KTarget/
// KTriggerOrder/KTriggerOptional.
func deckFor(t testing.TB, seat int) []*cards.Card {
	t.Helper()
	c := colours[seat%len(colours)]
	size := int32(2 + seat%3) // vanilla creature P/T varies 2..4 by seat.

	land := parseCard(t, fmt.Sprintf(
		"Name:%s\nTypes:Basic Land %s\nOracle:x\n", c.land, c.land))
	creature := parseCard(t, fmt.Sprintf(
		"Name:%s Whelp %d\nManaCost:%s\nTypes:Creature Whelp\nPT:%d/%d\nOracle:x\n",
		c.land, seat, c.letter, size, size))
	// burnSrc and pumpSrc are the supplement's own literal texts (Ruling
	// P9): a spell that targets a creature or a player, and one that
	// targets a creature only.
	burn := parseCard(t, fmt.Sprintf(
		"Name:%s Bolt %d\nManaCost:%s\nTypes:Instant\n"+
			"A:SP$ DealDamage | ValidTgts$ Creature,Player | NumDmg$ 2\nOracle:x\n",
		c.land, seat, c.letter))
	pump := parseCard(t, fmt.Sprintf(
		"Name:%s Growth %d\nManaCost:%s\nTypes:Instant\n"+
			"A:SP$ Pump | ValidTgts$ Creature | NumAtt$ +2 | NumDef$ +2\nOracle:x\n",
		c.land, seat, c.letter))
	// warden mirrors rules/trigger_order_test.go's gainerSrc shape: a plain
	// Mode$ Phase upkeep trigger with no OptionalDecider$, so it is never
	// asked about (CR 603.3b needs a choice only when there is one).
	warden := parseCard(t, fmt.Sprintf(
		"Name:%s Warden %d\nManaCost:%s\nTypes:Enchantment\n"+
			"T:Mode$ Phase | Phase$ Upkeep | Execute$ TrigGain | TriggerDescription$ gain 1 life\n"+
			"SVar:TrigGain:DB$ GainLife | LifeAmount$ 1 | Defined$ You\nOracle:x\n",
		c.land, seat, c.letter))
	// almsgiver mirrors trigger_order_test.go's mayGainSrc shape
	// (OptionalDecider$ You), so KTriggerOptional gets real fuzz coverage.
	almsgiver := parseCard(t, fmt.Sprintf(
		"Name:%s Almsgiver %d\nManaCost:%s\nTypes:Enchantment\n"+
			"T:Mode$ Phase | Phase$ Upkeep | OptionalDecider$ You | Execute$ TrigGain | "+
			"TriggerDescription$ you may gain 1 life\n"+
			"SVar:TrigGain:DB$ GainLife | LifeAmount$ 1 | Defined$ You\nOracle:x\n",
		c.land, seat, c.letter))

	var deck []*cards.Card
	add := func(card *cards.Card, n int) {
		for i := 0; i < n; i++ {
			deck = append(deck, card)
		}
	}
	add(land, 17)
	add(creature, 7)
	add(burn, 4)
	add(pump, 4)
	add(warden, 4)
	add(almsgiver, 4)
	return deck
}

// parseCard is card(t, src) from rules/turn_test.go, re-authored here
// (testutil cannot import rules, and rules' own test helper is unexported):
// parse, link the SVar/trigger chain, then apply the intrinsics (basic land
// mana) the corpus assumes the engine supplies.
func parseCard(t testing.TB, src string) *cards.Card {
	t.Helper()
	c, diags := cards.ParseBytes("testutil.txt", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("testutil: parsing %q: %v", src, diags)
	}
	c.Link()
	for _, f := range c.Faces {
		f.ApplyIntrinsics()
	}
	return c
}
