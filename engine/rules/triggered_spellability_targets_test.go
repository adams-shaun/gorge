package rules

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestTriggeredSpellAbilityTargetsVesuvanCopiesTarget: the real Vesuvan
// Duplimancy's `Defined$ TriggeredSpellAbilityTargets` resolves to the cast
// spell's single target, so a spell targeting only your own creature makes a
// non-legendary token copy of that creature.
func TestTriggeredSpellAbilityTargetsVesuvanCopiesTarget(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	vesuvan := corpusCommander(t, reg, "Vesuvan Duplimancy")
	strike := corpusCommander(t, reg, "Defiant Strike")
	hero := card(t, "Name:Lone Hero\nManaCost:1 W\nTypes:Legendary Creature Human\nPT:2/2\nOracle:x\n")
	deck0 := append([]*cards.Card{vesuvan, strike, hero}, mountainDeck(t, 37)...)
	cfg := seatZeroStart(Config{Seed: 4711, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{deck0, mountainDeck(t, 40)},
		Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()

	moveToBattlefieldByName(t, e, 0, "Vesuvan Duplimancy")
	heroID := moveToBattlefieldByName(t, e, 0, "Lone Hero")
	strikeID := moveSeededToHand(t, e, 0, "Defiant Strike")
	e.priorityRound()

	// Precondition: the hero is the only creature, on the battlefield, and
	// is legendary, so "isn't legendary" is observable.
	if o := e.G.Obj(heroID); o.Zone != state.ZBattlefield || !slices.Contains(e.Derived(heroID).Types, "Legendary") {
		t.Fatalf("precondition: hero zone=%s types=%v", o.Zone, e.Derived(heroID).Types)
	}
	battlefieldBefore := len(e.G.Zone(state.ZBattlefield, 0))

	addMana(t, e, 0, "WWWWW")
	e.priorityRound()
	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == strikeID {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for Defiant Strike: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("target ask = %+v", d)
	}
	ti := -1
	for _, o := range d.Options {
		if o.Obj == heroID {
			ti = o.Index
		}
	}
	if ti < 0 {
		t.Fatalf("hero not offered as a target: %+v", d.Options)
	}
	submitChoices(t, e, ti)

	// Let the trigger and the spell resolve.
	for i := 0; i < 20 && len(e.G.Zone(state.ZBattlefield, 0)) == battlefieldBefore; i++ {
		d := e.Pending()
		if d == nil {
			break
		}
		pass := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				pass = o.Index
			}
		}
		if d.Kind == decision.KChoose && d.Prompt == "Choose a creature to copy" {
			// Choices$ Card narrows the selector's set: exactly the spell's
			// one target is offered, never the Duplimancy or a sweep.
			if len(d.Options) != 1 || d.Options[0].Obj != heroID {
				t.Fatalf("copy choice options = %+v, want only the hero %d", d.Options, heroID)
			}
			submitChoices(t, e, d.Options[0].Index)
			continue
		}
		if d.Kind != decision.KPriority || pass < 0 {
			t.Fatalf("unexpected ask while resolving: %+v", d)
		}
		submitChoices(t, e, pass)
	}

	var copies []state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		o := e.G.Obj(id)
		if id != heroID && o.IsToken && o.Face() != nil && o.Face().Name == "Lone Hero" {
			copies = append(copies, id)
		}
	}
	if len(copies) != 1 {
		t.Fatalf("token copies of Lone Hero = %d, want 1", len(copies))
	}
	if slices.Contains(e.Derived(copies[0]).Types, "Legendary") {
		t.Fatalf("token copy is legendary, want NonLegendary$ True")
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "is not resolvable; no copy") {
			t.Fatalf("unexpected Note: %q", ev.Text)
		}
	}
	replayCheck(t, e, cfg)
}
