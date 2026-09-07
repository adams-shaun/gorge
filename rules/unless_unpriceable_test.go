package rules

import (
	"sort"
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// walkAllSAs visits every SA reachable from a face: its printed abilities,
// the effect of every trigger, the With of every replacement, and the body
// of every SVar (resolved on demand). This is the same traversal the Audit *cost
// walker in counter_unlesspay_test.go and the Discard walker use, so the
// compiled corpus is swept the way a card is actually compiled -- an
// UnlessCost$ that only reaches an effect through a SubAbility$ or an SVar
// body is still found.
func walkAllSAs(face *cards.Face, visit func(*cards.SA)) {
	var walk func(sa *cards.SA)
	walk = func(sa *cards.SA) {
		for ; sa != nil; sa = sa.Sub {
			visit(sa)
		}
	}
	for _, a := range face.Abilities {
		walk(a)
	}
	for _, t := range face.Triggers {
		walk(t.Effect)
	}
	for _, r := range face.Repls {
		walk(r.With)
	}
	for name := range face.SVars {
		walk(cards.ResolveSVar(face.SVars, name))
	}
}

// unpriceableCounterCards returns the sorted set of card names whose
// compiled corpus carries a Counter SA with a non-empty UnlessCost$ that
// ParseCost cannot price -- the population I-5 protects. The walker dedups
// per (face name, SA line) so a signature reached twice (printed and through
// an SVar) counts once.
func unpriceableCounterCards(reg *cards.Registry) []string {
	set := map[string]struct{}{}
	for _, c := range reg.Cards {
		for _, f := range c.Faces {
			walkAllSAs(f, func(sa *cards.SA) {
				if sa.API != "Counter" {
					return
				}
				uc := sa.Params["UnlessCost"]
				if uc == "" {
					return
				}
				if !ParseCost(uc).Priceable() {
					set[f.Name] = struct{}{}
				}
			})
		}
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// TestUnlessCostUnpriceablePopulation pins the corpus population of Counter
// SAs whose UnlessCost$ ParseCost cannot price. It is the executable version
// of the I-5 scope: every one of them must satisfy the same !Priceable()
// predicate (so no SVar-sourced X, cast-time X or Sac component escapes as a
// special case), and the set itself is a golden -- a corpus or grammar change
// that adds or removes an unpriceable unless-cost here is a real scope change
// that must be understood, not silently absorbed. Measured on the compiled
// .cards/ir.gob.gz corpus at FORGE_REF: 24 distinct cards, of which 21 carry
// UnlessCost$ X (the I-5 population the issue names) and 3 a Sac<...> part
// (Blood Funnel, Brain Gorgers, Mana Vortex). Raw .cards/cardsfolder lines
// with UnlessCost$ X number the same 21.
func TestUnlessCostUnpriceablePopulation(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	got := unpriceableCounterCards(reg)
	var want = []string{
		"Blood Funnel", "Brain Gorgers", "Broken Ambitions", "Cephalid Shrine",
		"Clash of Wills", "Condescend", "Dispelling Exhale", "In the Eye of Chaos",
		"Invoke Prejudice", "Lilting Refrain", "Logic Knot", "Mana Vortex",
		"Martyr of Frost", "Mausoleum Wanderer", "Mindswipe", "Overrule",
		"Power Sink", "Rethink", "Spectral Denial", "Spell Rupture",
		"Swallowed by Leviathan", "Syncopate", "Thassa's Rebuff", "We Say Thee Nay!",
	}
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("unpriceable-unless-cost card population = %d (%v), want %d (%v)",
			len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("population mismatch at %d: got %q want %q\nfull got: %v\nfull want: %v",
				i, got[i], want[i], got, want)
		}
	}
	// The two repo-deck cases the issue names must be in the population.
	for _, must := range []string{"Mausoleum Wanderer", "Power Sink", "Condescend"} {
		if !containsString(got, must) {
			t.Fatalf("%s is not in the unpriceable population %v", must, got)
		}
	}
}

func containsString(a []string, s string) bool {
	i := sort.SearchStrings(a, s)
	return i < len(a) && a[i] == s
}

// xCounterFixture is counterFixture for a counterspell with an {X} in its own
// cost: after choosing the cast, it answers the cast-time KChoose with
// xValue, then targets the creature spell. Everything after that (resolution,
// the unless_pay ask) is the caller's.
func xCounterFixture(t *testing.T, reg *cards.Registry, counter, creature string, xValue string) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	e := handEngine(t, mustCorpusCard(t, reg, counter), mustCorpusCard(t, reg, creature))
	ids := handIDsByFace(e)
	casterID, creatureID := ids[counter], ids[creature]
	if casterID == 0 || creatureID == 0 {
		t.Fatalf("hand missing %q or %q", counter, creature)
	}
	e.G.Players[0].Pool[state.MU] = 8
	e.G.Players[0].Pool[state.MG] = 8
	e.G.Players[0].Pool[state.MR] = 8
	e.askPriority(0)

	submitChoices(t, e, passToCast(t, e, creatureID))
	submitChoices(t, e, passToCast(t, e, casterID))
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "x" && strconv.Itoa(int(o.Amount)) == xValue {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("no X=%s option in %+v", xValue, d.Options)
		}
		submitChoices(t, e, idx)
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected a target decision after casting %s, got %+v", counter, d)
	}
	spellIdx := -1
	for _, o := range d.Options {
		if o.Obj == creatureID {
			spellIdx = o.Index
		}
	}
	if spellIdx < 0 {
		t.Fatalf("creature spell not offered as a participation target: %+v", d.Options)
	}
	submitChoices(t, e, spellIdx)
	return e, casterID, creatureID
}

// TestPowerSinkCastTimeXUnlessPayCannotSucceedFromEmptyPool is the case the
// old reading of I-5 got wrong: Power Sink's {X} comes from a real cast-time
// choice (Count$xPaid), so an earlier analysis claimed it was protected. It is
// not. We choose X = 2 at cast time, drain the payer, then answer the
// unless_pay ask "pay" -- and because the UnlessCost the payment API must
// price is still the unpriceable {X} (ParseCost reads "X", it never reads
// Count$xPaid), the pay cannot succeed and Power Sink must counter the
// targeted spell rather than resolving inertly.
func TestPowerSinkCastTimeXUnlessPayCannotSucceedFromEmptyPool(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, _, creatureID := xCounterFixture(t, reg, "Power Sink", "Grizzly Bears", "2")

	// Empty the pool so nothing can cover the unless-pay cost.
	e.G.Players[0].Pool = state.Mana{}

	pay := drainUntilUnlessPay(t, e, 30)
	if pay == nil {
		t.Fatal("no unless_pay ask posed for Power Sink")
	}
	// The payer says "pay", but the cost is the unpriceable {X}: it must be a
	// hard decline and the targeted spell is countered, not resolved.
	submitChoices(t, e, pay.Options[0].Index)
	passUntilStackEmpty(t, e, 30)
	if z := e.G.Obj(creatureID).Zone; z != state.ZGraveyard {
		t.Fatalf("Power Sink should counter the spell on an unpriceable {X}: zone = %s, want Graveyard", z)
	}
}

// counterSA returns the first Counter SA on a card's first face (or nil).
func counterSA(t *testing.T, c *cards.Card) *cards.SA {
	t.Helper()
	for _, f := range c.Faces {
		for _, ab := range f.Abilities {
			if ab.API == "Counter" {
				return ab
			}
		}
	}
	t.Fatalf("%s has no printed Counter ability", c.Faces[0].Name)
	return nil
}

// inDeck reports whether name appears in the resolved repo deck.
func inDeck(deck []*cards.Card, name string) bool {
	for _, c := range deck {
		if c.Faces[0].Name == name {
			return true
		}
	}
	return false
}

// TestMausoleumWandererUnlessCostX pin the repo-deck case that makes I-5
// more than a corpus corner: Mausoleum Wanderer's activated Counter ability
// carries UnlessCost$ X (X is the Wanderer's power, from an SVar -- the
// engine never reads it), and the card ships in two of the 12 replay-golden
// repo decks (mono-blue-tempo, uw-tempo). Its compiled UnlessCost parses to
// an unpriceable {X}, which the payment API must decline. (The ability's own
// Sac<1/CARDNAME> activation cost is a separate CARDNAME-substitution gap in
// the cost grammar and is not exercised here exactly for that reason; the
// card is pinned by its compiled Counter SA and its presence in the repo
// decks, not by playing its activation.)
func TestMausoleumWandererUnlessCostX(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	wanderer := mustCorpusCard(t, reg, "Mausoleum Wanderer")
	sa := counterSA(t, wanderer)
	uc := sa.Params["UnlessCost"]
	if uc != "X" {
		t.Fatalf("Mausoleum Wanderer unless cost = %q, want X", uc)
	}
	if ParseCost(uc).Priceable() {
		t.Fatalf("Mausoleum Wanderer's UnlessCost %q must be unpriceable", uc)
	}
	for _, deck := range []string{"mono-blue-tempo", "uw-tempo"} {
		if !inDeck(testutil.RepoDeck(t, reg, deck), "Mausoleum Wanderer") {
			t.Fatalf("Mausoleum Wanderer missing from repo deck %s", deck)
		}
	}
}

// TestParseCostPriceable is the unit-level predicate test behind I-5: it pins
// exactly which parsed shapes the payment API can price, so the "unpriceable
// cost" property travels through Cost.Priceable and not through a
// `if cost == "X"` special case.
func TestParseCostPriceable(t *testing.T) {
	cases := []struct {
		cost string
		want bool
	}{
		{"", true},                 // free
		{"no cost", true},          // free
		{"1", true},                // one generic
		{"2 U", true},              // generic + colour
		{"R R", true},              // two red
		{"X", false},               // the I-5 shape: unpriceable
		{"X 2", false},             // X alongside a real generic amount is still unpriceable
		{"Y", true},                // Y collapses to {1} -- a substitution, but priceable
		{"Z", true},                // Z likewise, priceable
		{"Sac<1/Creature>", false}, // non-mana part Pay cannot charge
		{"Sac<1/Land>", false},
		{"PayLife<5>", true}, // flattened to {1} generic -- priceable substitution
		{"T", false},         // Tap is a non-mana part Pay cannot charge
	}
	for _, tc := range cases {
		if got := ParseCost(tc.cost).Priceable(); got != tc.want {
			t.Errorf("Priceable(%q) = %v, want %v", tc.cost, got, tc.want)
		}
	}
}
