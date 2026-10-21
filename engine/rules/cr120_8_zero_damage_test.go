package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 120.8: a source that would deal 0 damage deals no damage at all -- no
// Damage event, so nothing triggers on it, no replacement has an event to
// modify, and lifelink gains nothing. CR 107.1b: a negative power used as a
// damage amount is 0. CR 603.2g: an event that did not happen (the
// marked-damage heal of cleanup, regeneration or HealDamage is not damage
// dealt) triggers nothing.

// cr1208FightSA is the fixture fight: the resolving source fights the one
// targeted creature (the TestFightZeroPowerAndOffBattlefieldNoWedge shape).
func cr1208FightSA() *cards.SA {
	return &cards.SA{Kind: "DB", API: "Fight", Params: map[string]string{
		"Defined": "Self", "ValidTgts": "Creature",
		"TargetMin": "0", "TargetMax": "1", "TgtPrompt": "x"}}
}

// cr1208DamageTo returns every logged Damage event whose recipient is obj.
func cr1208DamageTo(e *Engine, from int, obj state.ObjID) []events.Event {
	var out []events.Event
	for _, ev := range e.L.Events[from:] {
		if ev.Kind == events.Damage && ev.Obj == obj {
			out = append(out, ev)
		}
	}
	return out
}

// A 0-power creature whose "whenever this deals damage" trigger is the probe
// fights a 2/2: its own hit is 0 damage, so no Damage event lands on the
// Bear and the trigger is not queued. The Bear's 2 back still lands.
func TestCR120_8ZeroPowerFightDealsNoDamageAndTriggersNothing(t *testing.T) {
	t.Parallel()
	const pingerSrc = "Name:Zero Pinger\nManaCost:G\nTypes:Creature\nPT:0/3\n" +
		"T:Mode$ DamageDone | ValidSource$ Card.Self | Execute$ TrigGain | TriggerZones$ Battlefield | TriggerDescription$ x\n" +
		"SVar:TrigGain:DB$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"
	e := handEngine(t)
	pinger := onBoard(t, e, 0, pingerSrc)
	bear := onBoard(t, e, 1, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	mark := len(e.L.Events)
	queued := len(e.pendingTriggers)
	effects.Resolve(e, &effects.Ctx{Source: pinger, Controller: 0,
		Targets: []state.Target{{Obj: bear}}, TargetsOffered: true}, cr1208FightSA())
	if got := cr1208DamageTo(e, mark, bear); len(got) != 0 {
		t.Fatalf("0-power fighter's hit logged %+v; CR 120.8: 0 damage is no damage event", got)
	}
	if got := cr1208DamageTo(e, mark, pinger); len(got) != 1 || got[0].Amount != 2 {
		t.Fatalf("the Bear's hit back = %+v, want one 2-damage event on the pinger", got)
	}
	if got := len(e.pendingTriggers); got != queued {
		t.Fatalf("pending triggers %d -> %d; CR 120.8/603.2g: a 0-damage hit triggers nothing", queued, got)
	}
}

// Lifelink on a 0-power fighter gains nothing (CR 120.8 + 702.15a). This is a
// guard: payLifelinkRider already gates dealt <= 0.
func TestCR120_8ZeroPowerLifelinkGainsNoLife(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	linker := onBoard(t, e, 0, "Name:Zero Linker\nManaCost:W\nTypes:Creature\nPT:0/3\nK:Lifelink\nOracle:x\n")
	bear := onBoard(t, e, 1, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	mark := len(e.L.Events)
	life := e.G.Players[0].Life
	effects.Resolve(e, &effects.Ctx{Source: linker, Controller: 0,
		Targets: []state.Target{{Obj: bear}}, TargetsOffered: true}, cr1208FightSA())
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.LifeChange && ev.Player == 0 {
			t.Fatalf("0-power lifelink fighter logged %+v; CR 120.8: no damage, no life", ev)
		}
	}
	if got := e.G.Players[0].Life; got != life {
		t.Fatalf("seat 0 life %d -> %d; want unchanged", life, got)
	}
}

// Torbran, Thane of Red Fell (corpus): "If a red source you control would
// deal damage ... it deals that much damage plus 2 instead." A 0-power red
// fighter would deal 0, so CR 120.8 leaves Torbran no event to replace: the
// opponent's creature takes nothing. A guard: the DamageDone replacement
// matcher already rejects Amount <= 0 (replacement_match.go), and moving the
// gate up to the emitter must keep it that way.
func TestCR120_8ZeroDamageHasNoEventToReplace(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	torbran := searchCorpusCard(t, reg, "Torbran, Thane of Red Fell")
	e := handEngine(t)
	onBoardCard(t, e, 0, torbran)
	imp := onBoard(t, e, 0, "Name:Zero Imp\nManaCost:R\nTypes:Creature Imp\nPT:0/3\nOracle:x\n")
	bear := onBoard(t, e, 1, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/3\nOracle:x\n")
	mark := len(e.L.Events)
	effects.Resolve(e, &effects.Ctx{Source: imp, Controller: 0,
		Targets: []state.Target{{Obj: bear}}, TargetsOffered: true}, cr1208FightSA())
	for _, ev := range cr1208DamageTo(e, mark, bear) {
		if ev.Amount > 0 {
			t.Fatalf("Torbran replaced a 0-damage hit into %+v; CR 120.8: there is no event to replace", ev)
		}
	}
	if got := e.G.Obj(bear).Damage; got != 0 {
		t.Fatalf("Bear marked damage = %d, want 0", got)
	}
}

// A negative-power fighter (CR 107.1b: its damage is 0) must not lower the
// marked damage already on its opponent.
func TestCR120_8NegativePowerFightLeavesMarkedDamage(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	weak := onBoard(t, e, 0, "Name:Weakling\nManaCost:B\nTypes:Creature\nPT:1/3\nOracle:x\n")
	bear := onBoard(t, e, 1, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/3\nOracle:x\n")
	effects.Resolve(e, &effects.Ctx{Source: weak, Controller: 0}, &cards.SA{Kind: "DB", API: "Pump",
		Params: map[string]string{"Defined": "Self", "NumAtt": "-3"}})
	if got := e.Power(weak); got != -2 {
		t.Fatalf("Weakling power = %d, want -2 (setup)", got)
	}
	e.emit(events.Event{Kind: events.Damage, Obj: bear, Amount: 1})
	if got := e.G.Obj(bear).Damage; got != 1 {
		t.Fatalf("setup: Bear marked damage = %d, want 1", got)
	}
	mark := len(e.L.Events)
	effects.Resolve(e, &effects.Ctx{Source: weak, Controller: 0,
		Targets: []state.Target{{Obj: bear}}, TargetsOffered: true}, cr1208FightSA())
	if got := cr1208DamageTo(e, mark, bear); len(got) != 0 {
		t.Fatalf("negative-power hit logged %+v; CR 107.1b/120.8: it deals no damage", got)
	}
	if got := e.G.Obj(bear).Damage; got != 1 {
		t.Fatalf("Bear marked damage = %d after a -2-power fight, want 1 (unchanged)", got)
	}
}

// Crag Saurian (corpus): "Whenever a source deals damage to Crag Saurian,
// that source's controller gains control of Crag Saurian." Removing its
// marked damage -- cleanup (CR 514.2), a regeneration shield (CR 701.19a) or
// HealDamage -- is not damage dealt, so it must not trigger (CR 603.2g); and
// each removal must still clear the damage (the guard).
func TestCR603_2gDamageRemovalIsNotDamageDealt(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	saurian := searchCorpusCard(t, reg, "Crag Saurian")
	for _, tc := range []struct {
		name string
		heal func(e *Engine, id state.ObjID)
	}{
		{"cleanup", func(e *Engine, id state.ObjID) { e.cleanupBody() }},
		{"heal", func(e *Engine, id state.ObjID) { effects.HealMarkedDamage(e, id) }},
		{"regenerate", func(e *Engine, id state.ObjID) {
			e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "Shield", Amount: 1})
			if !effects.ReplaceDestruction(e, id) {
				t.Fatal("setup: the regeneration shield was not consumed")
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := handEngine(t)
			id := onBoardCard(t, e, 0, saurian)
			e.emit(events.Event{Kind: events.Damage, Obj: id, Amount: 2})
			if got := e.G.Obj(id).Damage; got != 2 {
				t.Fatalf("setup: marked damage = %d, want 2", got)
			}
			queued := len(e.pendingTriggers)
			tc.heal(e, id)
			if got := e.G.Obj(id).Damage; got != 0 {
				t.Fatalf("marked damage after %s = %d, want 0 (the removal must still work)", tc.name, got)
			}
			if got := len(e.pendingTriggers); got != queued {
				t.Fatalf("%s queued %d trigger(s); CR 603.2g: removing damage is not dealing it", tc.name, got-queued)
			}
		})
	}
}

// Removing marked damage is not damage, so protection (CR 702.16e prevents
// DAMAGE) has nothing to prevent: a HealDamage that runs while a damage
// source is published -- Wolverine's replacement body heals inside the
// replaced hit, under that hit's source -- still clears the damage.
func TestCR603_2gHealIsNotPreventedByProtection(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	red := onBoard(t, e, 1, "Name:Red Imp\nManaCost:R\nTypes:Creature Imp\nPT:1/1\nOracle:x\n")
	knight := onBoard(t, e, 0, "Name:Pro Knight\nManaCost:W\nTypes:Creature Knight\nPT:2/4\nK:Protection from red\nOracle:x\n")
	e.emit(events.Event{Kind: events.Damage, Obj: knight, Amount: 2})
	if got := e.G.Obj(knight).Damage; got != 2 {
		t.Fatalf("setup: marked damage = %d, want 2", got)
	}
	prev := e.SetDamageSource(red)
	effects.HealMarkedDamage(e, knight)
	e.SetDamageSource(prev)
	if got := e.G.Obj(knight).Damage; got != 0 {
		t.Fatalf("marked damage after heal under a red source = %d, want 0 (a heal is not damage; protection cannot prevent it)", got)
	}
}
