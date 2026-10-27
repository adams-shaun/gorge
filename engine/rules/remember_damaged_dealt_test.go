package rules

// cr-g240: RememberDamaged$ True remembers every recipient the emitter
// proposed, even one whose damage a replacement or a prevention stopped.
// These tests pin the corrected rule (CR 120.1: only damage actually dealt
// after replacement and prevention counts) end to end on the filing card
// (Incendiary Flow + Soul-Scar Mage) and on the two selector families that
// share the remember plumbing.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// rememberHandCardID returns the id of the first card named name in p's hand.
func rememberHandCardID(t *testing.T, e *Engine, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(state.ZHand, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	t.Fatalf("%q is not in seat %d's hand", name, p)
	return 0
}

// rememberCounterEvents sums the CounterChange amounts for obj/counter.
func rememberCounterEvents(e *Engine, obj state.ObjID, counter string) int32 {
	var n int32
	for _, ev := range e.L.Events {
		if ev.Kind == events.CounterChange && ev.Obj == obj && ev.Counter == counter {
			n += ev.Amount
		}
	}
	return n
}

// rememberChooseHas reports whether the source's persistent remembered list
// received an event naming id.
func rememberChooseHas(e *Engine, src, id state.ObjID) bool {
	for _, ev := range e.L.Events {
		if ev.Kind == events.Choose && ev.Obj == src && ev.Counter == "remembered" {
			for _, got := range ev.IDs {
				if got == id {
					return true
				}
			}
		}
	}
	return false
}

// rememberChooseEvents returns every remembered Choose event on src.
func rememberChooseEvents(e *Engine, src state.ObjID) []events.Event {
	var out []events.Event
	for _, ev := range e.L.Events {
		if ev.Kind == events.Choose && ev.Obj == src && ev.Counter == "remembered" {
			out = append(out, ev)
		}
	}
	return out
}

// rememberDamageEvents counts Damage events on obj.
func rememberDamageEvents(e *Engine, obj state.ObjID) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.Damage && ev.Obj == obj {
			n++
		}
	}
	return n
}

// rememberMovedContinuous reports whether a Moved-to-exile replacement whose
// remembered snapshot names obj was registered.
func rememberMovedContinuous(e *Engine, obj state.ObjID) bool {
	for _, ce := range e.continuous {
		if ce.ReplacementEvent != "Moved" {
			continue
		}
		for _, id := range ce.Remembered {
			if id == obj {
				return true
			}
		}
	}
	return false
}

// TestRememberDamagedSkipsReplacedDamage is the filing card: Soul-Scar Mage
// replaces Incendiary Flow's 3 noncombat damage to a Hill Giant with three
// -1/-1 counters, so the Giant was never dealt damage and its death must
// reach the graveyard, not Incendiary Flow's exile replacement.
func TestRememberDamagedSkipsReplacedDamage(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	flow := mustCorpusCard(t, reg, "Incendiary Flow")
	mage := mustCorpusCard(t, reg, "Soul-Scar Mage")
	giant := mustCorpusCard(t, reg, "Hill Giant")
	e, cfg := censusEngine(t, 2401, []*cards.Card{flow, mage}, []*cards.Card{giant})
	mageID := moveOwnerCard(t, e, 0, mage, state.ZBattlefield)
	giantID := moveOwnerCard(t, e, 1, giant, state.ZBattlefield)
	cardToHand(t, e, flow)
	flowID := rememberHandCardID(t, e, 0, "Incendiary Flow")

	// Preconditions: the mage's replacement source is where the engine looks
	// for it, and the Giant starts undamaged and counter-free.
	if o := e.G.Obj(mageID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Soul-Scar Mage is not on the battlefield: %+v", o)
	}
	if o := e.G.Obj(giantID); o == nil || o.Zone != state.ZBattlefield ||
		o.Damage != 0 || o.Counter("M1M1") != 0 {
		t.Fatalf("precondition: Hill Giant starts damaged or countered: %+v", o)
	}

	addMana(t, e, 0, "RR")
	cr601Cast(t, e, flowID, "")
	targetObject(t, e, giantID)
	passUntilStackEmpty(t, e, 30)

	// The replacement ran: three -1/-1 counters, and no damage event.
	if got := rememberCounterEvents(e, giantID, "M1M1"); got != 3 {
		t.Fatalf("Soul-Scar Mage put %d -1/-1 counters on the Giant, want 3", got)
	}
	if n := rememberDamageEvents(e, giantID); n != 0 {
		t.Fatalf("Incendiary Flow logged %d Damage event(s) on the Giant, want 0 (replaced)", n)
	}
	// The remember half must not have recorded it, and the exile replacement
	// must not have been registered over it.
	if rememberChooseHas(e, flowID, giantID) {
		t.Fatalf("the replaced Giant was remembered by Incendiary Flow: %v", rememberChooseEvents(e, flowID))
	}
	if rememberMovedContinuous(e, giantID) {
		t.Fatalf("Incendiary Flow registered its die-exile replacement over a Giant never dealt damage")
	}
	// The Giant died to the counters, so it goes to the graveyard.
	if o := e.G.Obj(giantID); o.Zone != state.ZGraveyard {
		t.Fatalf("Hill Giant is in %s, want graveyard (it was never dealt damage)", o.Zone)
	}
	replayCheck(t, e, cfg)
}

// TestRememberDamagedStillExilesDealtCreature is the control: without the
// damage replacement the same spell deals its 3 damage to a 2/2, so the
// creature WAS dealt damage and the exile rider still applies.
func TestRememberDamagedStillExilesDealtCreature(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	flow := mustCorpusCard(t, reg, "Incendiary Flow")
	bears := mustCorpusCard(t, reg, "Grizzly Bears")
	e, cfg := censusEngine(t, 2402, []*cards.Card{flow}, []*cards.Card{bears})
	bearsID := moveOwnerCard(t, e, 1, bears, state.ZBattlefield)
	cardToHand(t, e, flow)
	flowID := rememberHandCardID(t, e, 0, "Incendiary Flow")

	if o := e.G.Obj(bearsID); o == nil || o.Zone != state.ZBattlefield || o.Damage != 0 {
		t.Fatalf("precondition: Grizzly Bears is not an undamaged battlefield creature: %+v", o)
	}

	addMana(t, e, 0, "RR")
	cr601Cast(t, e, flowID, "")
	targetObject(t, e, bearsID)
	passUntilStackEmpty(t, e, 30)

	// The damage landed, so the remember half and the exile replacement ran.
	if n := rememberDamageEvents(e, bearsID); n != 1 {
		t.Fatalf("Incendiary Flow logged %d Damage event(s) on the Bears, want 1", n)
	}
	if !rememberChooseHas(e, flowID, bearsID) {
		t.Fatalf("the dealt-damage Bears was not remembered by Incendiary Flow")
	}
	if !rememberMovedContinuous(e, bearsID) {
		t.Fatalf("Incendiary Flow did not register its die-exile replacement over the Bears")
	}
	if o := e.G.Obj(bearsID); o.Zone != state.ZExile {
		t.Fatalf("Grizzly Bears is in %s, want exile (it was dealt lethal damage)", o.Zone)
	}
	replayCheck(t, e, cfg)
}

// rememberPreventShield is the bodyless full prevention shape Selfless
// Squire's RPrevent line uses, carried by a permanent so a single damage
// event can be fully prevented.
const rememberPreventShield = `Name:Test Damage Shield
Types:Enchantment
R:Event$ DamageDone | ActiveZones$ Battlefield | Prevent$ True | ValidTarget$ You | Description$ Prevent all damage that would be dealt to you.
Oracle:x
`

// rememberBoltSrc is the DealDamage + RememberDamaged$ True shape under test,
// aimed at a player so the prevention shield can stop it outright.
const rememberBoltSrc = `Name:Test Remember Bolt
ManaCost:R
Types:Sorcery
A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3 | RememberDamaged$ True | SpellDescription$ x
Oracle:x
`

// TestRememberDamagedPreventedPlayerNotRemembered: a player whose damage was
// prevented was never dealt damage, so RememberDamaged$ True must leave that
// player out of both remembered halves.
func TestRememberDamagedPreventedPlayerNotRemembered(t *testing.T) {
	t.Parallel()
	shield := card(t, rememberPreventShield)
	bolt := card(t, rememberBoltSrc)
	e, cfg := censusEngine(t, 2403, []*cards.Card{bolt}, []*cards.Card{shield})
	shieldID := moveOwnerCard(t, e, 1, shield, state.ZBattlefield)
	cardToHand(t, e, bolt)
	boltID := rememberHandCardID(t, e, 0, "Test Remember Bolt")

	// Preconditions: the shield is live on seat 1 and the bolt's controller
	// has nothing remembered yet.
	if o := e.G.Obj(shieldID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the prevention shield is not on the battlefield: %+v", o)
	}
	if life := e.G.Players[1].Life; life != 20 {
		t.Fatalf("precondition: seat 1 starts at %d life, want 20", life)
	}

	addMana(t, e, 0, "R")
	cr601Cast(t, e, boltID, "")
	targetPlayer(t, e, 1)
	passUntilStackEmpty(t, e, 30)

	// The shield ran: the damage was fully prevented.
	if life := e.G.Players[1].Life; life != 20 {
		t.Fatalf("seat 1 life = %d, want 20: the prevention shield never fired", life)
	}
	prevented := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(strings.ToLower(ev.Text), "prevent") {
			prevented = true
		}
	}
	if !prevented {
		t.Fatal("no prevention Note was logged: the shield never replaced the damage")
	}

	// Neither remembered half may name the prevented player.
	if rememberChooseHas(e, boltID, state.PlayerRef(1)) {
		t.Fatal("a prevented player received the remembered Choose event")
	}
	if o := e.G.Obj(boltID); o != nil {
		for _, tg := range o.Remembered {
			if tg.IsPlayer && tg.Player == 1 {
				t.Fatalf("the prevented player is in the persistent remembered list: %v", o.Remembered)
			}
		}
	}
	replayCheck(t, e, cfg)
}

// TestReplaceDyingTargetedIgnoresDealt keeps the scope note: a
// ReplaceDyingDefined$ Targeted card is target-based, not damage-based, so
// Fiery Annihilation still exiles the target even when Soul-Scar Mage
// replaced all of its damage with counters.
func TestReplaceDyingTargetedIgnoresDealt(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	fiery := mustCorpusCard(t, reg, "Fiery Annihilation")
	mage := mustCorpusCard(t, reg, "Soul-Scar Mage")
	giant := mustCorpusCard(t, reg, "Hill Giant")
	e, cfg := censusEngine(t, 2404, []*cards.Card{fiery, mage}, []*cards.Card{giant})
	mageID := moveOwnerCard(t, e, 0, mage, state.ZBattlefield)
	giantID := moveOwnerCard(t, e, 1, giant, state.ZBattlefield)
	cardToHand(t, e, fiery)
	fieryID := rememberHandCardID(t, e, 0, "Fiery Annihilation")

	if o := e.G.Obj(mageID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Soul-Scar Mage is not on the battlefield: %+v", o)
	}
	if o := e.G.Obj(giantID); o == nil || o.Zone != state.ZBattlefield || o.Counter("M1M1") != 0 {
		t.Fatalf("precondition: Hill Giant is not an uncountered battlefield creature: %+v", o)
	}

	addMana(t, e, 0, "RRR")
	cr601Cast(t, e, fieryID, "")
	targetObject(t, e, giantID)
	passUntilStackEmpty(t, e, 30)

	// The replacement ran (5 counters, no damage), and the TARGETED selector
	// still registered the exile.
	if got := rememberCounterEvents(e, giantID, "M1M1"); got != 5 {
		t.Fatalf("Soul-Scar Mage put %d -1/-1 counters on the Giant, want 5", got)
	}
	if n := rememberDamageEvents(e, giantID); n != 0 {
		t.Fatalf("Fiery Annihilation logged %d Damage event(s) on the Giant, want 0 (replaced)", n)
	}
	if !rememberMovedContinuous(e, giantID) {
		t.Fatalf("the Targeted selector dropped a target whose damage was replaced")
	}
	if o := e.G.Obj(giantID); o.Zone != state.ZExile {
		t.Fatalf("Hill Giant is in %s, want exile: Targeted is target-based, not dealt-damage-based", o.Zone)
	}
	replayCheck(t, e, cfg)
}
