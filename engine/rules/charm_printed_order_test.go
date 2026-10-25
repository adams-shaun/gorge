package rules

// A modal spell's chosen modes resolve -- and have their targets asked -- in
// PRINTED order (CR 608.2c, CR 700.2a), not the order the player picked them.
// The choice SET is unchanged; only its execution order is normalised. These
// tests cover a non-pawprint charm whose modes interact, the per-mode target
// ask order, and the pawprint (CR 700.2i) guard.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// printedOrderCharmScript prints ReanimateMode first and DestroyMode second.
// If the answer's order were followed, DestroyMode would sweep an empty board
// before ReanimateMode returned the creature; in printed order the returned
// creature is then destroyed, so its final zone proves which order ran.
func printedOrderCharmScript() string {
	return "Name:Printed Order Charm\nManaCost:B\nTypes:Instant\n" +
		"A:SP$ Charm | CharmNum$ 2 | Choices$ ReanimateMode,DestroyMode\n" +
		"SVar:ReanimateMode:DB$ ChangeZone | Origin$ Graveyard | Destination$ Battlefield | ValidTgts$ Creature.YouOwn | TgtPrompt$ Choose target creature card in your graveyard | SpellDescription$ Return target creature card from your graveyard to the battlefield.\n" +
		"SVar:DestroyMode:DB$ DestroyAll | ValidCards$ Creature | SpellDescription$ Destroy each creature.\n" +
		"Oracle:x\n"
}

// pawprintPrintedOrderCharmScript is the same interaction with a pawprint
// budget (CR 700.2i): each mode costs one {P} under a budget of two.
func pawprintPrintedOrderCharmScript() string {
	return "Name:Pawprint Printed Order Charm\nManaCost:B\nTypes:Instant\n" +
		"A:SP$ Charm | CharmNum$ 2 | Pawprint$ 2 | Choices$ ReanimateMode,DestroyMode\n" +
		"SVar:ReanimateMode:DB$ ChangeZone | Origin$ Graveyard | Destination$ Battlefield | ValidTgts$ Creature.YouOwn | TgtPrompt$ Choose target creature card in your graveyard | Pawprint$ 1 | SpellDescription$ Return target creature card from your graveyard to the battlefield.\n" +
		"SVar:DestroyMode:DB$ DestroyAll | ValidCards$ Creature | Pawprint$ 1 | SpellDescription$ Destroy each creature.\n" +
		"Oracle:x\n"
}

// TestCharmModesResolveInPrintedOrder picks mode 2 (DestroyMode) then mode 1
// (ReanimateMode) of an interacting two-mode charm. Printed order resolves
// ReanimateMode first and DestroyMode second, so the returned creature ends
// in the graveyard. Under pick order DestroyMode sweeps nothing and the
// creature stays on the battlefield.
func TestCharmModesResolveInPrintedOrder(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureDeck(t, 6410, printedOrderCharmScript(), vanillaCreatureScript)
	bear := addToGraveyard(t, e, 0, vanillaCreatureScript)
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: seeded Bear zone = %+v, want graveyard", o)
	}
	addMana(t, e, 0, "B")
	d := castFixture(t, e, id, -1)
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("pending = %+v, want the cast mode announcement", d)
	}
	submitChoices(t, e, 1, 0) // DestroyMode first, ReanimateMode second
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending = %+v, want ReanimateMode's target ask", d)
	}
	bearOpt := -1
	for _, o := range d.Options {
		if o.Obj == bear {
			bearOpt = o.Index
		}
	}
	if bearOpt < 0 {
		t.Fatalf("ReanimateMode's ask did not offer the graveyard Bear: %+v", d.Options)
	}
	submitChoices(t, e, bearOpt)
	passUntilStackEmpty(t, e, 20)

	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("Bear zone = %+v, want graveyard: the modes did not resolve in printed order (ReanimateMode before DestroyMode)", o)
	}
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("resolved Charm zone = %+v, want graveyard", o)
	}
	replayCheck(t, e, cfg)
}

// TestCharmTargetsAskedInPrintedOrder picks mode 2 (PickPlayer) then mode 1
// (PickCreature). The combined per-mode target ask must present printed order:
// group 0 is PickCreature's creature option and group 1 is PickPlayer's player
// option, matching the order the modes later resolve in.
func TestCharmTargetsAskedInPrintedOrder(t *testing.T) {
	t.Parallel()
	charm := "Name:Target Order Charm\nManaCost:B\nTypes:Instant\n" +
		"A:SP$ Charm | CharmNum$ 2 | Choices$ PickCreature,PickPlayer\n" +
		"SVar:PickCreature:DB$ Pump | ValidTgts$ Creature.YouCtrl | NumAtt$ +1 | NumDef$ +1 | SpellDescription$ Target creature you control gets +1/+1 until end of turn.\n" +
		"SVar:PickPlayer:DB$ LoseLife | ValidTgts$ Player | LifeAmount$ 1 | SpellDescription$ Target player loses 1 life.\n" +
		"Oracle:x\n"
	e, cfg, id := newFixtureDeck(t, 6411, charm, vanillaCreatureScript)
	bear := putCreature(t, e, 0, vanillaCreatureScript)
	if got := e.Power(bear); got != 2 {
		t.Fatalf("precondition: Bear power = %d, want 2", got)
	}
	if got := e.G.Players[1].Life; got != 20 {
		t.Fatalf("precondition: seat 1 life = %d, want 20", got)
	}
	addMana(t, e, 0, "B")
	d := castFixture(t, e, id, -1)
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("pending = %+v, want the cast mode announcement", d)
	}
	submitChoices(t, e, 1, 0) // PickPlayer first, PickCreature second

	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.Min != 2 || d.Max != 2 {
		t.Fatalf("pending = %+v, want the two modes' combined target ask", d)
	}
	if len(d.ResumeModes) != 2 || d.ResumeModes[0] != "PickCreature" || d.ResumeModes[1] != "PickPlayer" {
		t.Fatalf("target ask modes = %v, want printed order [PickCreature PickPlayer]", d.ResumeModes)
	}
	creatureOpt, playerOpt := -1, -1
	for _, o := range d.Options {
		switch o.Group {
		case "charm-mode-0":
			if o.Kind == "player" {
				t.Fatalf("mode 0 option = %+v, want the printed-first creature mode", o)
			}
			if o.Obj == bear {
				creatureOpt = o.Index
			}
		case "charm-mode-1":
			if o.Kind != "player" {
				t.Fatalf("mode 1 option kind = %q, want the printed-second player mode", o.Kind)
			}
			if o.Player == 1 {
				playerOpt = o.Index
			}
		}
	}
	if creatureOpt < 0 || playerOpt < 0 {
		t.Fatalf("target ask did not offer both modes' targets: creature %d player %d in %+v", creatureOpt, playerOpt, d.Options)
	}
	submitChoices(t, e, creatureOpt, playerOpt)
	passUntilStackEmpty(t, e, 20)

	if got := e.Power(bear); got != 3 {
		t.Fatalf("Bear power = %d, want 3 (PickCreature's target bound to mode 0)", got)
	}
	if got := e.G.Players[1].Life; got != 19 {
		t.Fatalf("seat 1 life = %d, want 19 (PickPlayer's target bound to mode 1)", got)
	}
	replayCheck(t, e, cfg)
}

// TestPawprintPrintedOrder guards CR 700.2i's printed-order execution: a
// budgeted charm choosing its modes in reverse still resolves them in printed
// order, so the reanimation happens before the sweep and the creature ends in
// the graveyard.
func TestPawprintPrintedOrder(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureDeck(t, 6412, pawprintPrintedOrderCharmScript(), vanillaCreatureScript)
	bear := addToGraveyard(t, e, 0, vanillaCreatureScript)
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: seeded Bear zone = %+v, want graveyard", o)
	}
	addMana(t, e, 0, "B")
	d := castFixture(t, e, id, -1)
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("pending = %+v, want the cast mode announcement", d)
	}
	if !d.Budgeted || d.MaxSum != 2 {
		t.Fatalf("pawprint ask budget = %d (budgeted %v), want a live budget of 2", d.MaxSum, d.Budgeted)
	}
	submitChoices(t, e, 1, 0) // DestroyMode first, ReanimateMode second
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending = %+v, want ReanimateMode's target ask", d)
	}
	bearOpt := -1
	for _, o := range d.Options {
		if o.Obj == bear {
			bearOpt = o.Index
		}
	}
	if bearOpt < 0 {
		t.Fatalf("ReanimateMode's ask did not offer the graveyard Bear: %+v", d.Options)
	}
	submitChoices(t, e, bearOpt)
	passUntilStackEmpty(t, e, 20)

	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("Bear zone = %+v, want graveyard: pawprint modes did not resolve in printed order", o)
	}
	replayCheck(t, e, cfg)
}
