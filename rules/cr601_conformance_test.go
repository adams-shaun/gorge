package rules

// Reference: Magic: The Gathering Comprehensive Rules, 2026-08-07 revision.
// These assertions expose UNFIXED divergences, not approved approximations.
// Enable explicitly with GORGE_CR_CONFORMANCE=1. Remove each opt-in guard when
// its defect is fixed; do not turn the incorrect behaviour into an expectation.
// Default skips preserve the tests-only dispatch's unchanged-engine gates.

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func requireCR601Audit(t *testing.T, finding string) {
	t.Helper()
	if os.Getenv("GORGE_CR_CONFORMANCE") != "1" {
		t.Skipf("CR 601 conformance: unfixed %s; NOT an AGENTS.md approximation; run with GORGE_CR_CONFORMANCE=1", finding)
	}
}

// TestCR601NoMandatoryCounterCastOnEmptyStack checks a deliberately narrow,
// independent oracle throughout the acceptance games: a mandatory spell target
// cannot exist on an empty stack (nor can the proposed spell target itself,
// CR 115.5 in this revision). Do NOT reuse askTarget/legalTargets here: agreement
// between two implementations is not proof of compliance with the CR.
//
// This does not generalise to modal spells, optional targets, or targets whose
// availability depends on choices made later during casting (CR 601.5).
func TestCR601NoMandatoryCounterCastOnEmptyStack(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, seats := range []int{2, 4, 6, 8} {
		t.Run(seatCount(seats), func(t *testing.T) {
			all := testutil.LegacyDeckNames()
			names := make([]string, seats)
			decks := make([][]*cards.Card, seats)
			for i := range names {
				names[i] = all[i%len(all)]
				decks[i] = testutil.RepoDeck(t, reg, names[i])
			}
			e := New(Config{Seed: 42, Names: names, Decks: decks, Tokens: reg.Tokens, Mulligans: 1})
			b := newTestBot(7)
			e.Advance()
			checked := 0
			for n := 0; !e.G.Over && e.Pending() != nil && n < 400000; n++ {
				d := e.Pending()
				if d.Kind == decision.KPriority && len(e.G.Stack) == 0 {
					// Count examined cards, not offending options: once fixed,
					// the absent cast options must not make the guard vacuous.
					for _, id := range e.G.Zone(state.ZHand, d.Player) {
						o := e.G.Obj(id)
						if o == nil || o.Face() == nil {
							continue
						}
						sa := o.Face().SpellAbility()
						if sa == nil || sa.API != "Counter" || sa.Params["TargetType"] != "Spell" || sa.Params["ValidTgts"] != "Card" {
							continue
						}
						// Only the unconditional, default one-target shape.
						if sa.Params["TargetMin"] != "" || sa.Params["TargetMax"] != "" || sa.Params["TgtZone"] != "" {
							continue
						}
						checked++
						for _, opt := range d.Options {
							if opt.Kind == "cast" && opt.Obj == id {
								t.Fatalf("%d seats, intent %d: priority decision seq %d offered %d (%q) with an empty stack — CR 601.2c: a required spell target must be available; CR 115.5 forbids targeting itself", seats, n, d.Seq, id, opt.Label)
							}
						}
					}
				}
				if err := e.Submit(b.answer(e, d)); err != nil {
					t.Fatalf("CR 601 audit driver: %d seats, intent %d: %v", seats, n, err)
				}
			}
			if checked == 0 {
				t.Fatal("CR 601.2c vacuity guard: no mandatory spell-target cards examined on an empty stack")
			}
			if !e.G.Over {
				t.Fatalf("CR 601 audit driver: game did not finish (turn %d)", e.G.Turn)
			}
			t.Logf("CR 601.2c: examined %d card/priority-boundary pairs", checked)
		})
	}
}

// TestCR601TargetsPrecedeManaPayment isolates a LEGAL cast, so I-2 cannot
// account for its failure. Real compiled burn spells can target either living
// player. At the unanswered target decision, their mana must still be unspent:
// CR 601.2 requires the listed order, with 601.2c before 601.2h.
//
// No synthetic SA or direct Game mutation is used. Repeated corpus cards make
// the opening hand deterministic; the mana setup itself is an applied event.
func TestCR601TargetsPrecedeManaPayment(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Lightning Bolt", "Shock", "Incinerate"} {
		t.Run(name, func(t *testing.T) {
			c, ok := reg.Lookup(name)
			if !ok {
				t.Fatalf("CR 601.2c/601.2h fixture: missing corpus card %q", name)
			}
			sa := c.Faces[0].SpellAbility()
			if sa == nil || sa.API != "DealDamage" || sa.Params["ValidTgts"] == "" {
				t.Fatalf("CR 601.2c/601.2h fixture: %q no longer has the targeted damage SA", name)
			}
			deck := make([]*cards.Card, 12)
			for i := range deck {
				deck[i] = c
			}
			e := New(Config{Seed: 42, Names: []string{"caster", "opponent"}, Decks: [][]*cards.Card{deck, deck}, Tokens: reg.Tokens})
			e.Advance()
			e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "R", Amount: 3})
			e.askPriority(0)
			d := e.Pending()
			idx := -1
			var source state.ObjID
			for _, opt := range d.Options {
				if opt.Kind == "cast" && opt.AltCostIndex == 0 && opt.Mode == "" {
					idx, source = opt.Index, opt.Obj
					break
				}
			}
			if idx < 0 {
				t.Fatalf("CR 601.2c/601.2h vacuity guard: no payable %q cast offered", name)
			}
			before := e.G.Players[0].Pool
			start := len(e.L.Events)
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
				t.Fatalf("CR 601.2c/601.2h cast: %v", err)
			}
			d = e.Pending()
			if d == nil || d.Kind != decision.KTarget || d.Source != source || len(d.Options) == 0 {
				t.Fatalf("CR 601.2c vacuity guard: expected unanswered target selection for %q, got %+v", name, d)
			}
			for _, ev := range e.L.Events[start:] {
				if ev.Kind == events.TargetsChosen && ev.Obj == source {
					t.Fatalf("CR 601.2c: targets for %q were recorded without the player's answer", name)
				}
			}
			if after := e.G.Players[0].Pool; after != before {
				t.Fatalf("target decision seq %d for source %d (%q) is unanswered but mana changed %v -> %v — CR 601.2c/601.2h: choose targets before paying costs", d.Seq, source, name, before, after)
			}
		})
	}
}
