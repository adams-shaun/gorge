package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// animatedDaggersSrc is an Equipment that is also a creature (Vibranium
// Energy Daggers animated), so the creature filter of its own Equip matches
// it and only the attach self-exclusion keeps it out of the pool.
const animatedDaggersSrc = "Name:Test Animated Daggers\nManaCost:1\nTypes:Artifact Creature Equipment\nPT:2/2\n" +
	"K:Equip:1\n" +
	"Oracle:Equip {1}\n"

// targetObjs lists the object options of a target ask, sorted.
func targetObjs(d *decision.Decision) []state.ObjID {
	var out []state.ObjID
	for _, o := range d.Options {
		if o.Kind != "player" && o.Obj != 0 {
			out = append(out, o.Obj)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// TestCopiedEquipDoesNotOfferItsOwnEquipment: CR 707.10b/c, 301.5c -- Echo,
// Perceptive Prodigy copies an animated Equipment's Equip ability; the copy's
// new-target ask offers exactly what the original activation offered, never
// the Equipment that is the copy's source.
func TestCopiedEquipDoesNotOfferItsOwnEquipment(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	echo := searchCorpusCard(t, reg, "Echo, Perceptive Prodigy")
	bears := searchCorpusCard(t, reg, "Grizzly Bears")
	mountain := searchCorpusCard(t, reg, "Mountain")
	daggersCard := card(t, animatedDaggersSrc)
	fill := func(cs ...*cards.Card) []*cards.Card {
		out := slices.Clone(cs)
		for len(out) < 40 {
			out = append(out, mountain)
		}
		return out
	}
	cfg := Config{Seed: 7, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{fill(echo, bears, daggersCard), fill()},
		Tokens: reg.Tokens}
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	if e.G.Active != 0 {
		t.Fatalf("seat %d is active; the sorcery-speed Equip needs seat 0's main phase", e.G.Active)
	}
	echoID := miscMoveByName(t, e, 0, "Echo, Perceptive Prodigy", state.ZBattlefield)
	bearID := miscMoveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	daggers := miscMoveByName(t, e, 0, "Test Animated Daggers", state.ZBattlefield)
	e.emit(events.Event{Kind: events.SetupSettled, Obj: echoID})
	e.priorityRound()
	addMana(t, e, 0, "CCCC")

	// PRECONDITION: the Equipment is a creature, so its own creature filter
	// would match it.
	if !e.IsCreature(daggers) {
		t.Fatalf("the animated Equipment %d is not a creature", daggers)
	}

	submitChoices(t, e, abilityOptionFor(t, e, daggers).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected the Equip target ask, got %+v", d)
	}
	orig := targetObjs(d)
	if slices.Contains(orig, daggers) || !slices.Contains(orig, bearID) || !slices.Contains(orig, echoID) {
		t.Fatalf("the Equip activation offered %v (want Echo %d and bears %d, never the Equipment %d)", orig, echoID, bearID, daggers)
	}
	submitChoices(t, e, chainOptIndex(t, d, func(o decision.Option) bool { return o.Obj == bearID }))
	for d = e.Pending(); d != nil && d.Kind != decision.KPriority; d = e.Pending() {
		t.Fatalf("unexpected decision after the Equip target: %+v", d)
	}
	if len(e.G.Stack) == 0 {
		t.Fatal("the Equip ability is not on the stack")
	}
	equipObj := e.G.Stack[len(e.G.Stack)-1]
	if so := e.G.Obj(equipObj); so == nil || so.Source != daggers {
		t.Fatalf("top of stack %d is not the Equipment's ability: %+v", equipObj, so)
	}

	submitChoices(t, e, abilityOptionFor(t, e, echoID).Index)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected Echo's target ask, got %+v", d)
	}
	submitChoices(t, e, chainOptIndex(t, d, func(o decision.Option) bool { return o.Obj == equipObj }))

	d = passUntilNonPriority(t, e, 30)
	if d.Kind != decision.KTarget || d.ResumeKind != "copy_targets" {
		t.Fatalf("expected the copy's copy_targets ask, got %+v", d)
	}
	co := e.G.Obj(d.Source)
	if co == nil || !co.IsCopy || co.Source != daggers {
		t.Fatalf("ask source %d is not a copy of the Equip with the Equipment as source: %+v", d.Source, co)
	}
	if got := targetObjs(d); !slices.Equal(got, orig) {
		t.Fatalf("the copy offered %v, the original activation %v (Equipment %d)", got, orig, daggers)
	}
	submitChoices(t, e, chainOptIndex(t, d, func(o decision.Option) bool { return o.Obj == echoID }))
	replayCheck(t, e, cfg)
}
