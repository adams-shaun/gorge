package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Task 16 keyword triggers: Undying, Evolve, Exalted, Prowess. Each keyword
// is expanded by cards/keywords.go into an ordinary ChangesZone / Attacks /
// SpellCast trigger routed through trigger_match.go's own modes (see
// keyword_registration_test.go's pin and the acceptance ratchet), so these
// tests drive the same engine paths a real card of each keyword exercises.
//
// The tests emit setup events directly (e.emit) and then call e.priorityRound
// -- the engine's "CR 117.5: handle state-based actions and triggered
// abilities before granting priority" entry point -- to place any queued
// trigger on the stack ahead of passUntilStackEmpty draining it. This deviates
// from the brief's bare e.Advance() calls, which are a no-op here: emitting an
// event directly leaves whatever decision was pending (genesis's first
// priority in newFixtureDeck) untouched, so e.Advance() returns immediately
// without ever running putTriggersOnStack. e.priorityRound() is the same
// refresh existing helpers (addMana, putCreature) use for this exact reason.
// See the task report.

func TestUndyingReturnsOnceWithACounter(t *testing.T) {
	geist := "Name:Geist\nManaCost:G G\nTypes:Creature Spirit\nPT:2/1\nK:Haste\nK:Undying\nOracle:x\n"
	e, cfg, g := newFixtureDeck(t, 81, geist)
	e.emit(events.Event{Kind: events.MoveZone, Obj: g, From: state.ZHand, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.Damage, Obj: g, Amount: 3})
	e.checkStateBased()
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)
	if o := e.G.Obj(g); o.Zone != state.ZBattlefield || o.Counter("P1P1") != 1 || e.Power(g) != 3 {
		t.Fatalf("after first death: %s, counters %d", o.Zone, o.Counter("P1P1"))
	}
	e.emit(events.Event{Kind: events.Damage, Obj: g, Amount: 5})
	e.checkStateBased()
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(g).Zone != state.ZGraveyard {
		t.Fatal("undying returned a creature that had a +1/+1 counter")
	}
	replayCheck(t, e, cfg)
}

func TestEvolveGrowsOnlyForBiggerCreatures(t *testing.T) {
	small := "Name:Small\nManaCost:1\nTypes:Creature\nPT:1/1\nOracle:x\n"
	big := "Name:Big\nManaCost:2\nTypes:Creature\nPT:2/2\nOracle:x\n"
	theirs := "Name:Theirs\nManaCost:5\nTypes:Creature\nPT:5/5\nOracle:x\n"
	e, cfg, one := newFixtureDeck(t, 82,
		"Name:One\nManaCost:G\nTypes:Creature Human Ooze\nPT:1/1\nK:Evolve\nOracle:x\n",
		small, big)
	e.emit(events.Event{Kind: events.MoveZone, Obj: one, From: state.ZHand, To: state.ZBattlefield})
	e.priorityRound()
	putCreature(t, e, 0, small) // equal size: must not evolve
	e.Advance()
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(one).Counter("P1P1") != 0 {
		t.Fatal("evolved for an equal-size creature")
	}
	putCreature(t, e, 0, big) // strictly bigger: must evolve
	e.Advance()
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(one).Counter("P1P1") != 1 {
		t.Fatal("did not evolve for a bigger creature")
	}
	// An opponent's creature (seat 1) entering must not evolve seat 0's One:
	// Evolve's expansion is ValidCard$ Creature.YouCtrl+Other, so a seat on
	// the other side of the table is simply not YouCtrl. The card is minted
	// onto seat 1's battlefield via TokenCreate (newFixtureDeck's extras only
	// ever seed seat 0, so there is no real seat-1 card to move).
	e.priorityRound()
	putToken(t, e, 1, theirs, state.ZBattlefield)
	e.Advance()
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(one).Counter("P1P1") != 1 {
		t.Fatal("evolved for an opponent's creature")
	}
	replayCheck(t, e, cfg)
}

func TestExaltedPumpsALoneAttackerAndProwessPumpsOnNoncreatureSpells(t *testing.T) {
	knight := "Name:Knight\nManaCost:1 B\nTypes:Creature Human Knight\nPT:2/1\nK:Exalted\nOracle:x\n"
	other := "Name:Other\nManaCost:1\nTypes:Creature\nPT:1/1\nOracle:x\n"
	e, cfg, k := newFixtureDeck(t, 83, knight, other)
	e.emit(events.Event{Kind: events.MoveZone, Obj: k, From: state.ZHand, To: state.ZBattlefield})
	e.priorityRound()
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{k}})
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)
	if e.Power(k) != 3 {
		t.Fatalf("lone attacker power %d", e.Power(k))
	}
	// End combat so the first pump expires (until-end-of-turn) and k returns
	// to 2/1, so the two-attacker assertion below measures a fresh attack.
	e.cleanupStep()
	if e.Power(k) != 2 {
		t.Fatalf("power after cleanup %d, want 2", e.Power(k))
	}
	o := putCreature(t, e, 0, other)
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{k, o}})
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)
	if e.Power(k) != 2 {
		t.Fatal("exalted fired for two attackers")
	}
	replayCheck(t, e, cfg)

	e2, cfg2, sw := newFixtureDeck(t, 84,
		"Name:Swift\nManaCost:R\nTypes:Creature Human Monk\nPT:1/2\nK:Haste\nK:Prowess\nOracle:x\n",
		"Name:Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n")
	e2.emit(events.Event{Kind: events.MoveZone, Obj: sw, From: state.ZHand, To: state.ZBattlefield})
	e2.priorityRound()
	bolt := addToHand(t, e2, 0, "Name:Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n")
	addMana(t, e2, 0, "R")
	e2.Advance()
	castObj(t, e2, bolt)
	passUntilStackEmpty(t, e2, 20)
	if e2.Power(sw) != 2 || e2.Toughness(sw) != 3 {
		t.Fatalf("prowess: %d/%d", e2.Power(sw), e2.Toughness(sw))
	}
	replayCheck(t, e2, cfg2)
}
