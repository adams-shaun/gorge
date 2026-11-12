package rules

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestNicknameSacOffer locks the finding kgen's Sac<1/NICKNAME> lowering
// (dirac/kgen lower.go, kop.CostSacSelf) copies: gorge reads Forge's
// Sac<1/NICKNAME> cost as the source itself (pay.SacrificeMatchSpec's
// NICKNAME -> CARDNAME) and offers the activation in a priority window.
// Syr Ginger, the Meal Ender ({2}, {T}, Sacrifice Syr Ginger: you gain life
// equal to its power) is the measured carrier; Adric, Mathematical Genius
// (Ultimate Sacrifice — {1}{U}, Sacrifice Adric: counter target spell or
// ability) is the classmate whose offer only waits for a target (its menu
// withholds the option on an empty stack, CR 601.2c).
func TestNicknameSacOffer(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name string
		// menu: the card's activation targets nothing, so a priority window
		// offers it outright. Adric's Ultimate Sacrifice targets a spell or
		// ability (CR 601.2c): on an empty stack the offer gate withholds
		// it, which is the target rule, not the cost, so only its payability
		// is asserted here.
		menu bool
	}{
		{name: "Syr Ginger, the Meal Ender", menu: true},
		{name: "Adric, Mathematical Genius"},
	} {
		name := tc.name
		g := mustCorpusCard(t, reg, name)
		var cost string
		for _, ab := range g.Faces[0].Abilities {
			if ab != nil && ab.Kind == "AB" {
				cost = ab.ParamStr(cards.PKCost)
			}
		}
		if !strings.Contains(cost, "Sac<1/NICKNAME>") {
			t.Fatalf("%s fixture drift: no Sac<1/NICKNAME> activation cost (got %q)", name, cost)
		}
		deck := append([]*cards.Card{g}, mountainDeck(t, 39)...)
		cfg := seatZeroStart(Config{Seed: 1, Names: []string{"a", "b"}, Tokens: reg.Tokens,
			Decks: [][]*cards.Card{deck, mountainDeck(t, 40)}})
		e := New(cfg)
		e.Advance()
		toMain1(t, e)
		// {T} must be available at the window; a test face edit (no event)
		// keeps the creature out of summoning sickness's tap ban.
		g.Faces[0].Keywords = append(g.Faces[0].Keywords, "Haste")
		id := moveSeededCard(t, e, 0, g, state.ZBattlefield)
		addMana(t, e, 0, testManaFor(cost))
		e.pending = nil
		e.priorityRound()
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("%s: no priority window: %+v", name, d)
		}
		var labels []string
		var offered bool
		for _, o := range d.Options {
			if o.Label != "" {
				labels = append(labels, o.Label)
				offered = offered || strings.Contains(o.Label, name)
			}
		}
		if tc.menu && !offered {
			t.Errorf("%s: the priority menu offers none of its activations (labels %v)", name, labels)
		}
		if id == 0 {
			t.Fatalf("%s: no battlefield object", name)
		}
		var nick *cards.SA
		for _, ab := range g.Faces[0].Abilities {
			if ab != nil && ab.Kind == "AB" && strings.Contains(ab.ParamStr(cards.PKCost), "Sac<1/NICKNAME>") {
				nick = ab
			}
		}
		if nick == nil {
			t.Fatalf("%s fixture drift: no Sac<1/NICKNAME> ability", name)
		}
		c := ParseCost(nick.ParamStr(cards.PKCost))
		if !pay.NonManaCastable(asPayer(e), 0, id, c, true, "") {
			t.Errorf("%s: the Sac<1/NICKNAME> cost is not payable as the source (non-mana part)", name)
		}
		if !e.offerCastable(0, id, c, abilityScope(nick), true) {
			t.Errorf("%s: the Sac<1/NICKNAME> activation is not offerable", name)
		}
	}
}

// testManaFor spells the mana a cost's symbols need as pool letters
// (addMana reads one letter per symbol, WUBRGC): a generic number is that
// many mana of any color, spelled U.
func testManaFor(cost string) string {
	var out []byte
	for _, part := range strings.Fields(cost) {
		if strings.Contains(part, "<") || part == "T" {
			break
		}
		if n, err := strconv.Atoi(part); err == nil {
			out = append(out, []byte(strings.Repeat("U", n))...)
			continue
		}
		out = append(out, part...)
	}
	return string(out)
}
