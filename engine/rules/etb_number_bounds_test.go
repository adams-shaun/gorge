package rules

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The as-enters number ask (etbOptions' "number" arm) honours the card's own
// Min$/Max$ through effects.ETBNumberChoices -- the same bound reader the
// mid-resolution ChooseNumber ask uses -- instead of a fixed 0..12.

func talionLike(minmax string) string {
	return "Name:Talion Like\nManaCost:2 U B\nTypes:Creature Faerie Noble\nPT:3/4\nK:ETBReplacement:Other:ChooseNumber\n" +
		"SVar:ChooseNumber:DB$ ChooseNumber | " + minmax + " | Defined$ You | SpellDescription$ As CARDNAME enters, choose a number.\nOracle:x\n"
}

func etbNumberAsk(t *testing.T, src string) *decision.Decision {
	t.Helper()
	e, _, find := etbConfig(t, 44, []string{src}, []string{"Name:Filler\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1\nOracle:x\n"})
	find("Talion Like", 0)
	addMana(t, e, 0, "WWUUBB")
	castFirst(t, e, "cast")
	d := passUntilNonPriority(t, e, 40)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "etb" || d.Options[0].Kind != "number" {
		t.Fatalf("expected the entry number ask, got %+v", d)
	}
	return d
}

func TestETBNumberAskHonoursMinMax(t *testing.T) {
	t.Parallel()
	d := etbNumberAsk(t, talionLike("Min$ 1 | Max$ 10"))
	if len(d.Options) != 10 || d.Options[0].Amount != 1 || d.Options[9].Amount != 10 {
		t.Fatalf("options = %+v, want 1..10", d.Options)
	}
	d = etbNumberAsk(t, talionLike("Max$ 3"))
	if len(d.Options) != 4 || d.Options[0].Amount != 0 || d.Options[3].Amount != 3 {
		t.Fatalf("options = %+v, want 0..3", d.Options)
	}
}

// A Max$ above the historical 0..12 default must be offered in full (the
// secretly-choose-1..20 carrier).
func TestETBNumberAskOffersBoundAboveTheDefault(t *testing.T) {
	t.Parallel()
	d := etbNumberAsk(t, talionLike("Min$ 1 | Max$ 20"))
	if len(d.Options) != 20 || d.Options[19].Amount != 20 {
		t.Fatalf("options = %d, want 1..20", len(d.Options))
	}
}

// Census (the ratchet): every corpus as-enters ChooseNumber carrier that
// states Min$/Max$ yields an entry list inside exactly those literal bounds.
// The carrier count is pinned so a new one is looked at.
func TestETBNumberCarriersAreBounded(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := handEngine(t, corpusCard(t, "Lightning Bolt"))
	bounded := 0
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			for i := range f.Repls {
				r := &f.Repls[i]
				if r.With == nil || r.With.API != "ChooseNumber" || r.ParamStr(cards.PKKeyword) != "ETBReplacement" {
					continue
				}
				mn, hasMin := r.With.Params["Min"]
				mx, hasMax := r.With.Params["Max"]
				if !hasMin && !hasMax {
					continue
				}
				bounded++
				opts := effects.ETBNumberChoices(e, state.ObjID(1), 0, f.SVars, r.With)
				lo, hi := 0, 12
				if v, err := strconv.Atoi(strings.TrimSpace(mn)); err == nil && hasMin {
					lo = v
				}
				if v, err := strconv.Atoi(strings.TrimSpace(mx)); err == nil && hasMax {
					hi = v
				}
				if len(opts) == 0 || opts[0].Amount != lo || opts[len(opts)-1].Amount != hi {
					t.Errorf("%s: options %d..%d, want %d..%d", f.Name, opts[0].Amount, opts[len(opts)-1].Amount, lo, hi)
				}
			}
		}
	}
	if bounded != 6 {
		t.Fatalf("bounded as-enters ChooseNumber carriers = %d, want 6 (update after review)", bounded)
	}
}

// CR 208.3: a noncreature spell has no power or toughness, so "power equal to
// the chosen number" never holds for it -- even for a chosen 0. Goes through
// Engine.matchesSpec, the seam every rules-side match takes, which binds
// layer-derived P/T (0/0 for a P/T-less object).
func TestPowerEqualNeverMatchesNoncreatureSpell(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusCard(t, "Lightning Bolt"), corpusCard(t, "Forest"), corpusCard(t, "Grizzly Bears"))
	hand := e.G.Zone(state.ZHand, 0)
	if len(hand) != 3 {
		t.Fatalf("precondition: hand = %d cards, want 3", len(hand))
	}
	bolt, land, bear := hand[0], hand[1], hand[2]
	o := e.G.Obj(bolt)
	if o == nil || o.Face().PT != "" || o.Face().Cmc() != 1 {
		t.Fatalf("precondition: want an instant with no printed P/T and mana value 1: %+v", o)
	}
	if b := e.G.Obj(bear); b == nil || b.Face().PT != "2/2" {
		t.Fatalf("precondition: Grizzly Bears should print 2/2: %+v", b)
	}
	if !e.matchesSpec("Card.cmcEQ1", bolt, effects.SpecContext{}) {
		t.Fatal("precondition: cmcEQ1 should match the mana value 1 spell")
	}
	for _, id := range []state.ObjID{bolt, land} {
		for _, spec := range []string{"Card.powerEQ0", "Card.toughnessEQ0", "Card.powerEQtoughness", "Card.powerNOT0", "Card.powerLT1", "Card.toughnessLE0"} {
			if e.matchesSpec(spec, id, effects.SpecContext{}) {
				t.Errorf("%s matched P/T-less %s", spec, e.G.Obj(id).Face().Name)
			}
		}
	}
	// Controls: a creature's P/T still compares.
	if !e.matchesSpec("Card.powerEQ2", bear, effects.SpecContext{}) {
		t.Error("Card.powerEQ2 should match Grizzly Bears")
	}
	if e.matchesSpec("Card.powerEQ0", bear, effects.SpecContext{}) {
		t.Error("Card.powerEQ0 should not match Grizzly Bears")
	}
}
