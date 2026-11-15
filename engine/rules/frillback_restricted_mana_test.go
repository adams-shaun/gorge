package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// frillbackWithLandAsk casts Tranquil Frillback from floating {R}{R}{G} with
// the named land untapped beside it and returns the first choose ask the enter
// trigger poses once it resolves.
func frillbackWithLandAsk(t *testing.T, land string) *decision.Decision {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	cfg := Config{Seed: 7, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{lookup(t, reg, "Tranquil Frillback")}, mountainDeck(t, 39)...),
			mountainDeck(t, 40),
		},
		Tokens: reg.Tokens,
	}
	e := New(seatZeroStart(cfg))
	e.Advance()
	tb := findAndMoveToHand(t, e, 0, "Tranquil Frillback")
	e.pending = nil
	e.Advance()
	driveToStep(t, e, 3, 0, state.StepMain1)
	putOnBattlefield(t, e, 0, lookup(t, reg, land))
	addMana(t, e, 0, "RRG")
	submitChoices(t, e, castOptionIdx(e.Pending(), "cast", tb))
	for i := 0; i < 30; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		if d.Kind == decision.KChoose {
			return d
		}
		pass := 0
		for _, o := range d.Options {
			if o.Kind == "pass" {
				pass = o.Index
			}
		}
		submitChoices(t, e, pass)
	}
	t.Fatal("the enter trigger never posed its choose ask")
	return nil
}

// CR 106.6 with CR 601.2b: Plaza of Heroes' any-colour mana is spendable only
// on a legendary spell, so it cannot pay Tranquil Frillback's {G}, and the
// "pay {G} up to three times" announcement is not offered. A Forest in its
// place makes the same announcement (0..3 is the control).
func TestFrillbackCountAskIgnoresRestrictedMana(t *testing.T) {
	t.Parallel()
	if d := frillbackWithLandAsk(t, "Forest"); len(d.Options) == 0 || d.Options[0].Kind != "trigger_cost_x" {
		t.Fatalf("control: a Forest should make the count announcement, got %+v", d.Options)
	}
	d := frillbackWithLandAsk(t, "Plaza of Heroes")
	for _, o := range d.Options {
		if o.Kind == "trigger_cost_x" {
			t.Fatalf("Plaza of Heroes' legendary-only mana was counted toward {G}: %+v", d.Options)
		}
	}
}
