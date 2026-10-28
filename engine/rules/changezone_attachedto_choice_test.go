package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestChangeZoneAttachedToFilterAsksController is CR 303.4f / 301.5c for
// ChangeZone's `AttachedTo$ <filter>` spelling: One Last Job's "Aura or
// Equipment" mode resolves `AttachedTo$ Creature.YouCtrl`. With two legal
// creatures the resolving controller chooses which one the returned Aura or
// Equipment enters attached to (the second option here); with exactly one
// there is nothing to choose and no decision is posed. Before the fix the
// first Defined$ match was taken silently, so the chosen-second assertions
// failed while the one-creature shape passed vacuously.
//
// The Aura half also pins the CR 303.4f legality narrowing: the filter's
// candidates are screened by the entering Aura's own Enchant restriction, so
// a spell with two creatures but one unenchantable asks only about the legal
// one. The Equipment half takes the post-move rider (the Aura half enters
// through the entry gate's named bearer) and must attach to the same answer.
func TestChangeZoneAttachedToFilterAsksController(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		card string
	}{
		{"aura", "Holy Strength"},
		{"equipment", "Bonesplitter"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("two creatures asks and takes the chosen one", func(t *testing.T) {
				reg := searchTestRegistry(t)
				e, ids := attachedToFixture(t, reg, 9411, []string{"One Last Job"},
					[]string{"Grizzly Bears", "Hill Giant"}, []string{tc.card})
				bears, giant := ids["Grizzly Bears"], ids["Hill Giant"]
				card := ids[tc.card]
				if bears == 0 || giant == 0 || bears == giant || card == 0 {
					t.Fatalf("precondition: two distinct battlefield creatures and the graveyard card, got %d, %d, %d",
						bears, giant, card)
				}
				if o := e.G.Obj(bears); o == nil || o.Zone != state.ZBattlefield {
					t.Fatalf("precondition: creature %d = %+v, want it on the battlefield", bears, o)
				}
				if o := e.G.Obj(giant); o == nil || o.Zone != state.ZBattlefield {
					t.Fatalf("precondition: creature %d = %+v, want it on the battlefield", giant, o)
				}
				if o := e.G.Obj(card); o == nil || o.Zone != state.ZGraveyard {
					t.Fatalf("precondition: returned card %d = %+v, want it in the graveyard", card, o)
				}
				castOneLastJobAttachMode(t, e, card)

				d := passPriorityUntilNonPriority(t, e)
				if d.Kind != decision.KChoose || d.ResumeKind != "changezone_attach" {
					t.Fatalf("ask after resolution = %s/%q, want the ChangeZone attach choice: %+v",
						d.Kind, d.ResumeKind, d)
				}
				if d.Player != 0 {
					t.Fatalf("attach choice posed to seat %d, want the controller (0)", d.Player)
				}
				if len(d.Options) != 2 {
					t.Fatalf("attach choice offers %d options, want the two legal creatures: %+v",
						len(d.Options), d.Options)
				}
				first, second := d.Options[0].Obj, d.Options[1].Obj
				if first == second || (first != bears && first != giant) || (second != bears && second != giant) {
					t.Fatalf("precondition: options name the two distinct creatures; got %+v", d.Options)
				}
				submitChoices(t, e, d.Options[1].Index)
				passUntilStackEmpty(t, e, 30)

				o := e.G.Obj(card)
				if o == nil || o.Zone != state.ZBattlefield {
					t.Fatalf("returned card %d is %+v, want it on the battlefield", card, o)
				}
				if o.AttachedTo != second {
					t.Fatalf("returned card AttachedTo = %d, want the chosen second creature %d (first was %d)",
						o.AttachedTo, second, first)
				}
				// Replay: the answer is a recorded decision, not a silent read
				// of the option order on re-resolution.
				found := false
				for _, ev := range e.L.Events {
					if ev.Kind == events.DecisionMade && strings.HasPrefix(ev.Text, "choose:") {
						found = true
					}
				}
				if !found {
					t.Fatal("no choose DecisionMade event in the log; the answer would not replay")
				}
				replayCheck(t, e, attachedToChoiceReplayConfig(t, reg, 9411,
					[]string{"One Last Job"}, []string{"Grizzly Bears", "Hill Giant"}, []string{tc.card}))
			})

			t.Run("one creature does not ask", func(t *testing.T) {
				reg := searchTestRegistry(t)
				e, ids := attachedToFixture(t, reg, 9412, []string{"One Last Job"},
					[]string{"Grizzly Bears"}, []string{tc.card})
				bears, card := ids["Grizzly Bears"], ids[tc.card]
				if bears == 0 || card == 0 {
					t.Fatalf("precondition: one battlefield creature and the graveyard card, got %d, %d", bears, card)
				}
				if o := e.G.Obj(bears); o == nil || o.Zone != state.ZBattlefield {
					t.Fatalf("precondition: creature %d = %+v, want it on the battlefield", bears, o)
				}
				if o := e.G.Obj(card); o == nil || o.Zone != state.ZGraveyard {
					t.Fatalf("precondition: returned card %d = %+v, want it in the graveyard", card, o)
				}
				castOneLastJobAttachMode(t, e, card)
				// passUntilStackEmpty fatals on any unhandled KChoose, so a
				// spurious attach ask fails here rather than being answered.
				passUntilStackEmpty(t, e, 30)
				o := e.G.Obj(card)
				if o == nil || o.Zone != state.ZBattlefield || o.AttachedTo != bears {
					t.Fatalf("returned card %+v, want it attached to the only creature %d", o, bears)
				}
			})
		})
	}
}

// attachedToChoiceReplayConfig rebuilds the exact Config attachedToFixture
// hands New, so the log the test just played can be replayed against the same
// genesis.
func attachedToChoiceReplayConfig(t *testing.T, reg *cards.Registry, seed uint64,
	hand, battlefield, graveyard []string,
) Config {
	t.Helper()
	bear := searchCorpusCard(t, reg, "Grizzly Bears")
	forest := searchCorpusCard(t, reg, "Forest")
	deck := make([]*cards.Card, 0, 40)
	for _, name := range hand {
		deck = append(deck, searchCorpusCard(t, reg, name))
	}
	for _, name := range battlefield {
		deck = append(deck, searchCorpusCard(t, reg, name))
	}
	for _, name := range graveyard {
		deck = append(deck, searchCorpusCard(t, reg, name))
	}
	for len(deck) < 40 {
		deck = append(deck, forest, bear)
	}
	opp := make([]*cards.Card, 40)
	for i := range opp {
		opp[i] = forest
	}
	return seatZeroStart(Config{Seed: seed, Names: []string{"protagonist", "opponent"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
}

// castOneLastJobAttachMode casts One Last Job with only its Aura-or-Equipment
// Spree mode, targets the fixture card in the graveyard, and leaves the spell
// on the stack for the caller to drive through resolution.
func castOneLastJobAttachMode(t *testing.T, e *Engine, target state.ObjID) {
	t.Helper()
	// ManaCost {2}{W} plus the chosen mode's ModeCost$ {1}.
	addMana(t, e, 0, "WWWW")
	castNamed(t, e, "One Last Job")
	d := e.Pending()
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("after cast = %+v, want the Spree mode ask", d)
	}
	modeIdx := -1
	for _, o := range d.Options {
		if strings.Contains(o.Label, "Aura or Equipment") {
			modeIdx = o.Index
		}
	}
	if modeIdx < 0 {
		t.Fatalf("Aura-or-Equipment mode not offered: %+v", d.Options)
	}
	submitChoices(t, e, modeIdx)

	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("after the mode = %+v, want the graveyard-card target ask", d)
	}
	targetIdx := -1
	for _, o := range d.Options {
		if o.Obj == target {
			targetIdx = o.Index
		}
	}
	if targetIdx < 0 {
		t.Fatalf("graveyard card %d not offered as a target: %+v", target, d.Options)
	}
	submitChoices(t, e, targetIdx)
}
