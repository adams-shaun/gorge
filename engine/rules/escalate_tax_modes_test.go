package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const fixLevySrc = "Name:Fix Levy\nManaCost:2\nTypes:Artifact\n" +
	"S:Mode$ RaiseCost | ValidCard$ Card.nonCreature | Type$ Spell | Amount$ 1 | Description$ Noncreature spells cost {1} more to cast.\n" +
	"Oracle:x\n"

// The escalate mode count is clamped to what the composed total (printed cost,
// the spell's cost-raising statics, one escalate payment per extra mode) can
// pay, not to the bare printed cost: Borrowed Grace {2}{W} + Escalate {1}{W}
// under a {1} tax needs {4}{W}{W} for two modes, so with 5 mana floating
// the second mode is unaffordable, and the mode ask must top out at one mode (the kernel's
// escalateModeAffordable agrees; Borrowed Malevolence under Thorn of Amethyst).
func TestEscalateModeCountPricesRaiseCostStatics(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sp, ok := reg.Lookup(fixGraceN)
	if !ok {
		t.Fatalf("corpus fixture: %s missing", fixGraceN)
	}
	deck := []*cards.Card{sp, card(t, fixLevySrc)}
	deckA := []*cards.Card{}
	for len(deck) < 40 {
		deck = append(deck, mountainDeck(t, 1)...)
	}
	for len(deckA) < 40 {
		deckA = append(deckA, mountainDeck(t, 1)...)
	}
	cfg := seatZeroStart(Config{Seed: 91, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{deck, deckA}, Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	moveByName(t, e, 0, fixGraceN, state.ZHand)
	moveByName(t, e, 0, "Fix Levy", state.ZBattlefield)
	// {2}{W} + {1} tax = 4 pays; + escalate {1}{W} = 6 does not. Untaxed, the
	// second mode costs 5 and a bare-cost clamp would offer it.
	addMana(t, e, 0, "111WW")
	castSpellOption(t, e, fixGraceN)
	d := e.Pending()
	if d == nil || d.Kind != decision.KModes || d.ResumeKind != "cast_modes" {
		t.Fatalf("pending = %+v, want the cast_modes KModes ask", d)
	}
	if d.Max != 1 {
		t.Fatalf("mode ask Max = %d, want 1 (two taxed modes cost 6, 5 mana floating)", d.Max)
	}
}
