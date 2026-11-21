package rules

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestProtectionChoiceOffersTypes pins the Protection effect's Choices$ ask:
// a Choices$ list naming a card-type token poses the real mid-resolution ask
// (the five colours, then the named type or types), and the chosen quality
// resolves into a "Protection from <quality>" keyword that actually protects
// (CR 702.16) — the colourless arm included, which protects from a source
// with no colour.
func TestProtectionChoiceOffersTypes(t *testing.T) {
	t.Parallel()

	const bearSrc = "Name:Plain Bear\nManaCost:G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

	// castAndChoose seeds the spell in seat 0's hand, the bear on its
	// battlefield and the sources in seat 1's deck, casts the spell at the
	// bear and answers the protection choice with the pick-th option.
	castAndChoose := func(t *testing.T, spell string, opps []string, wantLabels []string, pick int) (*Engine, Config, state.ObjID) {
		t.Helper()
		deck1 := make([]*cards.Card, 0, len(opps)+40)
		for _, src := range opps {
			deck1 = append(deck1, card(t, src))
		}
		cfg := seatZeroStart(Config{Seed: 1, Names: []string{"a", "b"},
			Decks: [][]*cards.Card{
				append([]*cards.Card{card(t, spell), card(t, bearSrc)}, mountainDeck(t, 38)...),
				append(deck1, mountainDeck(t, 40-len(opps))...),
			},
			Tokens: map[string]*cards.Card{},
		})
		e := New(cfg)
		e.Advance()
		bear := moveSeeded(t, e, 0, bearSrc, state.ZBattlefield)
		name := card(t, spell).Faces[0].Name
		spellID := state.ObjID(0)
		for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
			for _, id := range e.G.Zone(z, 0) {
				if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
					if z != state.ZHand {
						e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: state.ZHand})
						e.pending = nil
					}
					spellID = id
				}
			}
		}
		if spellID == 0 {
			t.Fatalf("precondition: spell %q not seeded", name)
		}
		addMana(t, e, 0, "W")
		d := e.Pending()
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "cast" && o.Obj == spellID {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("precondition: no cast option for the spell: %+v", d.Options)
		}
		submitChoices(t, e, idx)
		d = e.Pending()
		if d == nil || d.Kind != decision.KTarget {
			t.Fatalf("precondition: expected a target decision, got %+v", d)
		}
		ti := -1
		for _, o := range d.Options {
			if o.Obj == bear {
				ti = o.Index
			}
		}
		if ti < 0 {
			t.Fatalf("precondition: the bear is not offered as a target: %+v", d.Options)
		}
		submitChoices(t, e, ti)
		d = passUntilAsk(t, e)
		if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "protectionchoice" {
			t.Fatalf("protection choice = %+v, want KChoose protectionchoice", d)
		}
		var labels []string
		for _, o := range d.Options {
			labels = append(labels, o.Label)
		}
		if !slices.Equal(labels, wantLabels) {
			t.Fatalf("offered %v, want %v", labels, wantLabels)
		}
		submitChoices(t, e, d.Options[pick].Index)
		passUntilStackEmpty(t, e, 20)
		return e, cfg, bear
	}

	t.Run("anyColor_artifact", func(t *testing.T) {
		const spell = "Name:Paper Blessing\nManaCost:W\nTypes:Instant\n" +
			"A:SP$ Protection | ValidTgts$ Creature.YouCtrl | Gains$ Choice | Choices$ AnyColor,Artifact\nOracle:x\n"
		e, cfg, bear := castAndChoose(t, spell, []string{
			"Name:Clockwork Beast\nManaCost:4\nTypes:Artifact Creature Beast\nPT:4/4\nOracle:x\n",
			"Name:Red Bear\nManaCost:R\nTypes:Creature Bear\nPT:2/2\nOracle:x\n",
		}, []string{"white", "blue", "black", "red", "green", "Artifact"}, 5)
		if !slices.Contains(e.Keywords(bear), "Protection from Artifact") {
			t.Fatalf("keywords=%v, want the chosen Artifact quality", e.Keywords(bear))
		}
		artifact := moveByName(t, e, 1, "Clockwork Beast", state.ZBattlefield)
		red := moveByName(t, e, 1, "Red Bear", state.ZBattlefield)
		if !e.protectedFrom(bear, artifact) {
			t.Fatal("protection from Artifact did not protect from an artifact source")
		}
		if e.protectedFrom(bear, red) {
			t.Fatal("protection from Artifact protected from a red source")
		}
		replayCheck(t, e, cfg)
	})

	t.Run("anyColor_colorless", func(t *testing.T) {
		const spell = "Name:Paper Runes\nManaCost:W\nTypes:Instant\n" +
			"A:SP$ Protection | ValidTgts$ Creature.YouCtrl | Gains$ Choice | Choices$ AnyColor,colorless\nOracle:x\n"
		e, cfg, bear := castAndChoose(t, spell, []string{
			"Name:Grey Beast\nManaCost:5\nTypes:Creature Beast\nPT:5/5\nOracle:x\n",
			"Name:White Bear\nManaCost:W\nTypes:Creature Bear\nPT:2/2\nOracle:x\n",
		}, []string{"white", "blue", "black", "red", "green", "colorless"}, 5)
		if !slices.Contains(e.Keywords(bear), "Protection from colorless") {
			t.Fatalf("keywords=%v, want the chosen colorless quality", e.Keywords(bear))
		}
		colorless := moveByName(t, e, 1, "Grey Beast", state.ZBattlefield)
		white := moveByName(t, e, 1, "White Bear", state.ZBattlefield)
		if !e.protectedFrom(bear, colorless) {
			t.Fatal("protection from colorless did not protect from a colourless source")
		}
		if e.protectedFrom(bear, white) {
			t.Fatal("protection from colorless protected from a white source")
		}
		replayCheck(t, e, cfg)
	})

	t.Run("cardType", func(t *testing.T) {
		const spell = "Name:Paper Guard\nManaCost:W\nTypes:Instant\n" +
			"A:SP$ Protection | ValidTgts$ Creature.YouCtrl | Gains$ Choice | Choices$ CardType\nOracle:x\n"
		e, cfg, bear := castAndChoose(t, spell, []string{
			"Name:Red Bear\nManaCost:R\nTypes:Creature Bear\nPT:2/2\nOracle:x\n",
			"Name:Barren Rock\nManaCost:no cost\nTypes:Land\nOracle:x\n",
		}, []string{"Artifact", "Battle", "Creature", "Enchantment", "Instant", "Kindred", "Land", "Planeswalker", "Sorcery"}, 2)
		if !slices.ContainsFunc(e.Keywords(bear), func(k string) bool {
			return strings.HasPrefix(k, "Protection from Creature")
		}) {
			t.Fatalf("keywords=%v, want the chosen Creature card type", e.Keywords(bear))
		}
		creature := moveByName(t, e, 1, "Red Bear", state.ZBattlefield)
		land := moveByName(t, e, 1, "Barren Rock", state.ZBattlefield)
		if !e.protectedFrom(bear, creature) {
			t.Fatal("protection from Creature did not protect from a creature source")
		}
		if e.protectedFrom(bear, land) {
			t.Fatal("protection from Creature protected from a land source")
		}
		replayCheck(t, e, cfg)
	})
}
