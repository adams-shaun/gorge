package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// printedAddability_test.go is the leaf for CR 613.1f's printed-Continuous
// AddAbility$ grant: a `S:Mode$ Continuous | Affected$ <spec> | AddAbility$
// <SVar>` static (Ichormoon Gauntlet, Tazri, viridian_longbow, kusari_gama)
// used to emit no state.ContinuousEffect at all, so its non-mana ability was
// never offered. The structural fix lives in rules/layers.go (staticEffects
// emits the grant), rules/legal.go (grantedAbilities threads the grantor
// through decision.Option.GrantSource) and rules/speed.go
// (beginGrantedActivation resolves the body from the grantor while the minted
// ability's Source is the recipient, via events.GrantAbilityPush).

// printedAddabilityEngine builds a two-seat fixture whose protagonist is seat
// 0 and whose deck holds every argument card. Callers place cards by pointer.
func printedAddabilityEngine(t *testing.T, list ...*cards.Card) *Engine {
	t.Helper()
	deck := append([]*cards.Card{}, list...)
	if len(deck) > 40 {
		t.Fatalf("fixture deck too large: %d", len(deck))
	}
	deck = append(deck, mountainDeck(t, 40-len(deck))...)
	cfg := seatZeroStart(Config{Seed: 7, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{deck, mountainDeck(t, 40)}})
	e := New(cfg)
	e.Advance()
	return e
}

// moveCardToBattlefield finds a deck card by pointer in seat 0's hidden zones
// and moves it onto the battlefield with a logged MoveZone, returning its id.
func moveCardToBattlefield(t *testing.T, e *Engine, c *cards.Card) state.ObjID {
	t.Helper()
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.Owner != 0 || (o.Zone != state.ZLibrary && o.Zone != state.ZHand) || o.Card != c {
			continue
		}
		id := o.ID
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZBattlefield})
		return id
	}
	t.Fatalf("card %q not found in seat 0's library/hand", c.Faces[0].Name)
	return 0
}

// findSVarOption returns the granted "ability" option anchored on svar for
// obj, or ok=false. Granted options carry SVar (never Ability), so the
// printed-index helper cannot find them.
func findSVarOption(t *testing.T, e *Engine, obj state.ObjID, svar string) (decision.Option, bool) {
	t.Helper()
	d := e.Pending()
	if d == nil {
		t.Fatalf("no decision pending while scanning for granted %q on %d", svar, obj)
	}
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == obj && o.SVar == svar {
			return o, true
		}
	}
	return decision.Option{}, false
}

// TestPrintedContinuousAddAbilityCrossObjectGrant covers Done-means (a): a
// printed static whose Affected$ is another permanent. The ability must be
// offered on the RECIPIENT, activate, pay its cost and resolve -- and the
// granted body must resolve from the grantor's face, not the recipient's
// (the recipient has no such SVar, which was the silent no-op).
func TestPrintedContinuousAddAbilityCrossObjectGrant(t *testing.T) {
	grantor := card(t, "Name:Grantor\nManaCost:0\nTypes:Artifact\n"+
		"S:Mode$ Continuous | Affected$ Creature.Other+YouCtrl | AddAbility$ Zap\n"+
		"SVar:Zap:AB$ Draw | Cost$ T | NumCards$ 1 | Defined$ You | SpellDescription$ Draw a card.\nOracle:x\n")
	recipient := card(t, "Name:Recipient\nManaCost:0\nTypes:Creature\nPT:1/1\nOracle:x\n")

	e := printedAddabilityEngine(t, grantor, recipient)
	moveCardToBattlefield(t, e, grantor)
	recipientID := moveCardToBattlefield(t, e, recipient)
	e.G.Obj(recipientID).SummonSick = false
	addMana(t, e, 0, "")

	opt, ok := findSVarOption(t, e, recipientID, "Zap")
	if !ok {
		t.Fatalf("cross-object printed AddAbility$ grant not offered on the recipient: %+v", e.Pending().Options)
	}
	if opt.GrantSource == 0 || opt.GrantSource == recipientID {
		t.Fatalf("granted option GrantSource = %d, want the grantor id", opt.GrantSource)
	}
	if grantorID := opt.GrantSource; e.G.Obj(grantorID).Face().Name != "Grantor" {
		t.Fatalf("GrantSource names %q, want Grantor", e.G.Obj(grantorID).Face().Name)
	}

	hand := len(e.G.Zone(state.ZHand, 0))
	submitChoices(t, e, opt.Index)
	if len(e.G.Stack) != 1 {
		t.Fatalf("activation did not push an ability object: stack=%v", e.G.Stack)
	}
	passUntilStackEmpty(t, e, 20)
	if got := len(e.G.Zone(state.ZHand, 0)); got != hand+1 {
		t.Fatalf("recipient's granted Draw did not resolve: hand %d -> %d", hand, got)
	}
}

// TestPrintedContinuousAddAbilitySelfGrant covers Done-means (b): the
// self-scoped shape (Affected$ Card.Self), the path every existing Animate
// and max-speed grant shares. It must stay on the DelayedPush identity and
// resolve from the object itself.
func TestPrintedContinuousAddAbilitySelfGrant(t *testing.T) {
	self := card(t, "Name:Self Granter\nManaCost:0\nTypes:Artifact\n"+
		"S:Mode$ Continuous | Affected$ Card.Self | AddAbility$ Draw\n"+
		"SVar:Draw:AB$ Draw | Cost$ 0 | NumCards$ 1 | Defined$ You | SpellDescription$ Draw a card.\nOracle:x\n")

	e := printedAddabilityEngine(t, self)
	id := moveCardToBattlefield(t, e, self)
	addMana(t, e, 0, "")

	opt, ok := findSVarOption(t, e, id, "Draw")
	if !ok {
		t.Fatalf("self-scoped printed AddAbility$ grant not offered: %+v", e.Pending().Options)
	}
	if opt.GrantSource != id {
		t.Fatalf("self-grant GrantSource = %d, want the object itself %d", opt.GrantSource, id)
	}
	hand := len(e.G.Zone(state.ZHand, 0))
	submitChoices(t, e, opt.Index)
	// A self-grant mints through DelayedPush (the byte-identical legacy
	// identity), never GrantAbilityPush.
	if n := countKind(e.L.Events, events.DelayedPush, id); n != 1 {
		t.Fatalf("self-grant DelayedPush count = %d, want 1", n)
	}
	if n := countKind(e.L.Events, events.GrantAbilityPush, id); n != 0 {
		t.Fatalf("self-grant minted %d GrantAbilityPush events, want 0", n)
	}
	passUntilStackEmpty(t, e, 20)
	if got := len(e.G.Zone(state.ZHand, 0)); got != hand+1 {
		t.Fatalf("self-granted Draw did not resolve: hand %d -> %d", hand, got)
	}
}

// TestPrintedContinuousAddAbilityMultiValue covers Done-means (c): the
// `AddAbility$ A & B` multi-value form (6 corpus carriers) offers both
// bodies.
func TestPrintedContinuousAddAbilityMultiValue(t *testing.T) {
	multi := card(t, "Name:Multi Granter\nManaCost:0\nTypes:Artifact\n"+
		"S:Mode$ Continuous | Affected$ Card.Self | AddAbility$ Draw & Heal\n"+
		"SVar:Draw:AB$ Draw | Cost$ 0 | NumCards$ 1 | Defined$ You | SpellDescription$ Draw a card.\n"+
		"SVar:Heal:AB$ GainLife | Cost$ 0 | Defined$ You | LifeAmount$ 2 | SpellDescription$ Gain 2 life.\n"+
		"Oracle:x\n")

	e := printedAddabilityEngine(t, multi)
	id := moveCardToBattlefield(t, e, multi)
	addMana(t, e, 0, "")

	if _, ok := findSVarOption(t, e, id, "Draw"); !ok {
		t.Fatalf("first half of AddAbility$ Draw & Heal not offered: %+v", e.Pending().Options)
	}
	heal, ok := findSVarOption(t, e, id, "Heal")
	if !ok {
		t.Fatalf("second half of AddAbility$ Draw & Heal not offered: %+v", e.Pending().Options)
	}
	life := e.G.Players[0].Life
	submitChoices(t, e, heal.Index)
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Players[0].Life; got != life+2 {
		t.Fatalf("second AddAbility$ body did not resolve: life %d -> %d, want +2", life, got)
	}
}

// TestPrintedContinuousAddAbilityManaGrantsExactlyOnce covers Done-means (3):
// the duplicate-offer trap. A printed AddAbility$ whose body is AB$ Mana must
// appear in availableManaAbilities exactly once -- before the fix the old
// direct static scan and the new grantedAbilities walk each contributed a
// member. The static holder is an Artifact granting the ability to ANOTHER
// artifact, so the grant is genuinely cross-object.
func TestPrintedContinuousAddAbilityManaGrantsExactlyOnce(t *testing.T) {
	holder := card(t, "Name:Mana Holder\nManaCost:0\nTypes:Artifact\n"+
		"S:Mode$ Continuous | Affected$ Artifact.Other+YouCtrl | AddAbility$ Rock\n"+
		"SVar:Rock:AB$ Mana | Cost$ T | Produced$ C\nOracle:x\n")
	target := card(t, "Name:Mana Target\nManaCost:0\nTypes:Artifact\nOracle:x\n")

	e := printedAddabilityEngine(t, holder, target)
	moveCardToBattlefield(t, e, holder)
	id := moveCardToBattlefield(t, e, target)

	ma := e.availableManaAbilities(0, id)
	if len(ma) != 1 {
		t.Fatalf("availableManaAbilities returned %d members for one printed AddAbility grant, want exactly 1", len(ma))
	}
	if ma[0].API != "Mana" {
		t.Fatalf("granted mana member API = %q, want Mana", ma[0].API)
	}
}

// TestPrintedContinuousAddAbilityCorpusBarbedField covers Done-means (2): the
// real corpus Aura Barbed Field grants its enchanted LAND "{T}: This land
// deals 1 damage to any target." The ability was inert before the fix.
func TestPrintedContinuousAddAbilityCorpusBarbedField(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	barbed := mustCorpusCard(t, reg, "Barbed Field")
	mountain := mustCorpusCard(t, reg, "Mountain")

	e := printedAddabilityEngine(t, barbed, mountain)
	auraID := moveCardToBattlefield(t, e, barbed)
	landID := moveCardToBattlefield(t, e, mountain)
	e.emit(events.Event{Kind: events.Attach, Obj: auraID, IDs: []state.ObjID{landID}})
	addMana(t, e, 0, "")

	opt, ok := findSVarOption(t, e, landID, "Damage")
	if !ok {
		t.Fatalf("Barbed Field's granted land ability not offered: %+v", e.Pending().Options)
	}
	if opt.GrantSource != auraID {
		t.Fatalf("Barbed Field grant GrantSource = %d, want aura %d", opt.GrantSource, auraID)
	}
	submitChoices(t, e, opt.Index)

	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected a KTarget decision after activating the granted damage ability, got %+v", d)
	}
	idx := indexOfPlayerOption(d, 1)
	if idx < 0 {
		t.Fatalf("opponent face not offered as a target by ValidTgts$ Any: %+v", d.Options)
	}
	life := e.G.Players[1].Life
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Players[1].Life; got != life-1 {
		t.Fatalf("Barbed Field's granted ability dealt no damage: life %d -> %d", life, got)
	}
	if !e.G.Obj(landID).Tapped {
		t.Fatal("the granted {T} cost did not tap the enchanted land")
	}
}

// TestPrintedContinuousAddAbilityCorpusKusariGama covers Done-means (2): the
// real corpus Equipment Kusari-Gama grants its equipped CREATURE "{2}: This
// creature gets +1/+0 until end of turn." The body's `Defined$ Self` must
// resolve to the equipped creature (the ability's Source), never to the
// Equipment -- the exact source-vs-recipient distinction the fix is about.
func TestPrintedContinuousAddAbilityCorpusKusariGama(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	gama := mustCorpusCard(t, reg, "Kusari-Gama")
	bear := mustCorpusCard(t, reg, "Grizzly Bears")

	e := printedAddabilityEngine(t, gama, bear)
	equipID := moveCardToBattlefield(t, e, gama)
	bearID := moveCardToBattlefield(t, e, bear)
	e.emit(events.Event{Kind: events.Attach, Obj: equipID, IDs: []state.ObjID{bearID}})
	addMana(t, e, 0, "CC")

	before := e.Derived(bearID).Power
	opt, ok := findSVarOption(t, e, bearID, "GamaPump")
	if !ok {
		t.Fatalf("Kusari-Gama's granted equipped-creature ability not offered: %+v", e.Pending().Options)
	}
	if opt.GrantSource != equipID {
		t.Fatalf("Kusari-Gama grant GrantSource = %d, want equipment %d", opt.GrantSource, equipID)
	}
	submitChoices(t, e, opt.Index)
	passUntilStackEmpty(t, e, 20)

	if got := e.Derived(bearID).Power; got != before+1 {
		t.Fatalf("Kusari-Gama's Defined$ Self resolved to the wrong object: equipped creature power %d -> %d, want +1", before, got)
	}
	if got := e.Derived(equipID).Power; got != 0 {
		t.Fatalf("Kusari-Gama's pump landed on the Equipment (power %d), not the creature", got)
	}
}
