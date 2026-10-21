package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// shieldEngine is corpusEngineCfg with Wingshield Agent (a 2/3 that enters
// with a shield counter, K:etbCounter:SHIELD:1) on seat 1's battlefield and
// the seat-0 spells in seat 0's library. It returns the Agent's id.
func shieldEngine(t *testing.T, spells ...string) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	var mine []*cards.Card
	for _, s := range spells {
		mine = append(mine, lookup(t, reg, s))
	}
	e, cfg := corpusEngineCfg(t, reg, mine, []*cards.Card{lookup(t, reg, "Wingshield Agent")})
	agent := moveByName(t, e, 1, "Wingshield Agent", state.ZBattlefield)
	if n := e.G.Obj(agent).Counter("SHIELD"); n != 1 {
		t.Fatalf("CR 122.1c seq %d: precondition: Wingshield Agent entered with %d shield counters, want 1", len(e.L.Events), n)
	}
	return e, cfg, agent
}

// TestCR122_1cShieldCounterReplacesDestruction: CR 122.1c, "If this
// permanent would be destroyed as the result of an effect, instead remove a
// shield counter from it." Murder on Wingshield Agent removes its shield
// counter and the Agent stays on the battlefield; a second Murder, with no
// counter left, destroys it.
func TestCR122_1cShieldCounterReplacesDestruction(t *testing.T) {
	t.Parallel()
	e, cfg, agent := shieldEngine(t, "Murder", "Murder")
	first := moveByName(t, e, 0, "Murder", state.ZHand)
	addMana(t, e, 0, "BBC")
	castAt(t, e, first, agent)
	o := e.G.Obj(agent)
	if o.Zone != state.ZBattlefield {
		t.Fatalf("CR 122.1c seq %d: Murder destroyed Wingshield Agent through its shield counter (zone %v)", len(e.L.Events), o.Zone)
	}
	if n := o.Counter("SHIELD"); n != 0 {
		t.Fatalf("CR 122.1c seq %d: shield counters after the replaced destruction = %d, want 0", len(e.L.Events), n)
	}
	second := moveByName(t, e, 0, "Murder", state.ZHand)
	addMana(t, e, 0, "BBC")
	castAt(t, e, second, agent)
	if z := e.G.Obj(agent).Zone; z != state.ZGraveyard {
		t.Fatalf("CR 122.1c seq %d: a second Murder with no shield counter left: zone %v, want graveyard", len(e.L.Events), z)
	}
	replayCheck(t, e, cfg)
}

// TestCR122_1cShieldCounterPreventsDamage: CR 122.1c, "If damage would be
// dealt to this permanent, prevent that damage and remove a shield counter
// from it." Shock on Wingshield Agent is prevented and spends the counter;
// a second Shock is dealt normally.
func TestCR122_1cShieldCounterPreventsDamage(t *testing.T) {
	t.Parallel()
	e, cfg, agent := shieldEngine(t, "Shock", "Shock")
	first := moveByName(t, e, 0, "Shock", state.ZHand)
	addMana(t, e, 0, "R")
	castAt(t, e, first, agent)
	o := e.G.Obj(agent)
	if o.Damage != 0 || o.Counter("SHIELD") != 0 {
		t.Fatalf("CR 122.1c seq %d: after Shock: damage %d shield %d, want 0 damage (prevented) and 0 shield counters",
			len(e.L.Events), o.Damage, o.Counter("SHIELD"))
	}
	second := moveByName(t, e, 0, "Shock", state.ZHand)
	addMana(t, e, 0, "R")
	castAt(t, e, second, agent)
	if o := e.G.Obj(agent); o.Zone != state.ZBattlefield || o.Damage != 2 {
		t.Fatalf("CR 122.1c seq %d: second Shock with no shield counter: zone %v damage %d, want battlefield and 2", len(e.L.Events), o.Zone, o.Damage)
	}
	replayCheck(t, e, cfg)
}

// TestCR122_1jHoneCounterPumpsEquippedCreature: CR 122.1j, "A hone counter
// on an Equipment gives +1/+0 to any creature that Equipment is attached
// to." Sting, Bilbo's Sword with two hone counters, attached to Grizzly
// Bears, makes the Bears 4/2; removing one counter makes them 3/2;
// unattaching makes them 2/2 while Sting keeps its counter.
func TestCR122_1jHoneCounterPumpsEquippedCreature(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Sting, Bilbo's Sword"), lookup(t, reg, "Grizzly Bears")}, nil)
	sting := moveByName(t, e, 0, "Sting, Bilbo's Sword", state.ZBattlefield)
	passUntilStackEmpty(t, e, 20) // Sting's ETB: no opponent creature, no counters
	bears := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	e.emit(events.Event{Kind: events.Attach, Obj: sting, IDs: []state.ObjID{bears}})
	seedObjectCounter(t, e, sting, "HONE", 2)
	pt := func(want int32, what string) {
		t.Helper()
		if p, tough := e.Power(bears), e.Toughness(bears); p != want || tough != 2 {
			t.Fatalf("CR 122.1j seq %d: %s: Grizzly Bears %d/%d, want %d/2", len(e.L.Events), what, p, tough, want)
		}
	}
	pt(4, "two hone counters on the attached Sting")
	seedObjectCounter(t, e, sting, "HONE", -1)
	pt(3, "one hone counter left")
	e.emit(events.Event{Kind: events.Unattached, Obj: sting, IDs: []state.ObjID{bears}})
	pt(2, "Sting unattached")
	if n := e.G.Obj(sting).Counter("HONE"); n != 1 {
		t.Fatalf("CR 122.1j seq %d: Sting has %d hone counters after unattaching, want 1", len(e.L.Events), n)
	}
	replayCheck(t, e, cfg)
}
