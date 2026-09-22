package rules

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func witherCorpusCard(t *testing.T, path string) *cards.Card {
	t.Helper()
	c, ds := cards.Parse(filepath.Join("..", ".cards", "cardsfolder", path))
	if len(ds) != 0 {
		t.Fatalf("parse %s: %v", path, ds)
	}
	if ds = c.Link(); len(ds) != 0 {
		t.Fatalf("link %s: %v", path, ds)
	}
	return c
}

func assertWitherCounter(t *testing.T, e *Engine, target state.ObjID, amount int32) {
	t.Helper()
	if got := e.G.Obj(target).Counter("M1M1"); got != amount {
		t.Fatalf("target -1/-1 counters = %d, want %d", got, amount)
	}
	if got := e.G.Obj(target).Damage; got != 0 {
		t.Fatalf("target marked damage = %d, want 0", got)
	}
}

func TestWitherVillagePillagersRealSourceDealsCounters(t *testing.T) {
	e := combatEngine(t)
	source := onBoardCard(t, e, 0, witherCorpusCard(t, "v/village_pillagers.txt"))
	target := onBoard(t, e, 1, "Name:Target\nTypes:Creature\nPT:4/4\nOracle:x\n")
	if !e.HasKeyword(source, "Wither") || e.G.Obj(target).Zone != state.ZBattlefield {
		t.Fatal("precondition: real Village Pillagers source/creature target not live")
	}
	effects.Resolve(e, &effects.Ctx{Source: source, Controller: 0,
		Targets: []state.Target{{Obj: target}}}, &cards.SA{Kind: "DB", API: "DealDamage",
		Params: map[string]string{"Defined": "Targeted", "NumDmg": "1"}})
	assertWitherCounter(t, e, target, 1)
}

func TestWitherGrantedKeywordStartsAndStopsWithStatic(t *testing.T) {
	e := combatEngine(t)
	grantor := onBoardCard(t, e, 0, witherCorpusCard(t, "m/massacre_girl_known_killer.txt"))
	source := onBoard(t, e, 0, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n")
	target := onBoard(t, e, 1, "Name:Target\nTypes:Creature\nPT:4/4\nOracle:x\n")
	if e.G.Obj(source).Face() == nil || e.G.Obj(source).Face().HasKeyword("Wither") || !e.HasKeyword(grantor, "Menace") {
		t.Fatal("precondition: source prints Wither or grantor is not live")
	}
	if !e.HasKeyword(source, "Wither") {
		t.Fatal("precondition: static grant did not make source Wither")
	}
	effects.Resolve(e, &effects.Ctx{Source: source, Controller: 0, Targets: []state.Target{{Obj: target}}},
		&cards.SA{Kind: "DB", API: "DealDamage", Params: map[string]string{"Defined": "Targeted", "NumDmg": "1"}})
	assertWitherCounter(t, e, target, 1)
	e.emit(events.Event{Kind: events.MoveZone, Obj: grantor, From: state.ZBattlefield, To: state.ZGraveyard})
	if e.HasKeyword(source, "Wither") {
		t.Fatal("grantor removal left derived Wither live")
	}
	effects.Resolve(e, &effects.Ctx{Source: source, Controller: 0, Targets: []state.Target{{Obj: target}}},
		&cards.SA{Kind: "DB", API: "DealDamage", Params: map[string]string{"Defined": "Targeted", "NumDmg": "1"}})
	if e.G.Obj(target).Damage != 1 {
		t.Fatalf("ordinary damage after grant removal = %d, want 1", e.G.Obj(target).Damage)
	}
}

func TestWitherCombatDamageToBlockerUsesCounterForm(t *testing.T) {
	e := combatEngine(t)
	atk := onBoardReady(t, e, 0, "Name:Wither Knight\nTypes:Creature\nPT:2/2\nK:Wither\nOracle:x\n")
	blk := onBoard(t, e, 1, "Name:Blocker\nTypes:Creature\nPT:4/4\nOracle:x\n")
	if !e.HasKeyword(atk, "Wither") || e.G.Obj(blk).Zone != state.ZBattlefield {
		t.Fatal("precondition: attacker or blocker not live")
	}
	e.askAttackers()
	submitAttackers(t, e, atk)
	submitBlockers(t, e, blk)
	assertWitherCounter(t, e, blk, 2)
}

func TestWitherDamageToPlayerRemainsOrdinary(t *testing.T) {
	e := combatEngine(t)
	source := onBoard(t, e, 0, "Name:Wither Source\nTypes:Creature\nPT:2/2\nK:Wither\nOracle:x\n")
	before := e.G.Players[1].Life
	if !e.HasKeyword(source, "Wither") {
		t.Fatal("precondition: source does not have printed Wither")
	}
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2, Counter: "wither"})
	if got := e.G.Players[1].Life; got != before-2 {
		t.Fatalf("player life = %d, want %d", got, before-2)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.CounterChange && ev.Counter == "M1M1" {
			t.Fatal("Wither player damage emitted creature counters")
		}
	}
}

func TestWitherRedirectRecomputesRecipientForm(t *testing.T) {
	e := combatEngine(t)
	source := onBoard(t, e, 0, "Name:Wither Source\nTypes:Creature\nPT:2/2\nK:Wither\nOracle:x\n")
	creature := onBoard(t, e, 1, "Name:Target\nTypes:Creature\nPT:4/4\nOracle:x\n")
	artifact := onBoard(t, e, 1, "Name:Artifact\nTypes:Artifact\nPT:0/0\nOracle:x\n")
	if !e.HasKeyword(source, "Wither") || !e.IsCreature(creature) || e.IsCreature(artifact) {
		t.Fatal("precondition: source/recipient classifications are not distinct")
	}
	// ReplaceEvent's Affected rewrite models a DamageDone redirect. The source
	// marker must be reclassified to the final recipient, not retained from the
	// original creature target.
	e.emit(events.Event{Kind: events.Damage, Obj: creature, Amount: 2, Counter: "wither"})
	assertWitherCounter(t, e, creature, 2)
	before := e.G.Obj(artifact).Damage
	e.emit(events.Event{Kind: events.Damage, Obj: artifact, Amount: 2, Counter: "wither"})
	if got := e.G.Obj(artifact).Damage; got != before+2 {
		t.Fatalf("redirected noncreature damage = %d, want %d", got, before+2)
	}
}

func TestWitherDamageToCreatureIsMinusOneCountersNotMarkedDamage(t *testing.T) {
	e, _, source := newFixtureDeck(t, 2, "Name:Wither Source\nTypes:Creature\nPT:2/2\nK:Wither\nOracle:x\n")
	target := onBoard(t, e, 1, "Name:Target\nTypes:Creature\nPT:3/3\nOracle:x\n")
	if !e.HasKeyword(source, "Wither") {
		t.Fatal("precondition: source does not read printed Wither")
	}
	if e.G.Obj(target).Zone != state.ZBattlefield || !e.IsCreature(target) {
		t.Fatal("precondition: target is not a battlefield creature")
	}
	effects.Resolve(e, &effects.Ctx{Source: source, Controller: 0,
		Targets: []state.Target{{Obj: target}}}, &cards.SA{Kind: "DB", API: "DealDamage",
		Params: map[string]string{"Defined": "Targeted", "NumDmg": "2"}})
	if got := e.G.Obj(target).Counter("M1M1"); got != 2 {
		t.Fatalf("target -1/-1 counters = %d, want 2", got)
	}
	if got := e.G.Obj(target).Damage; got != 0 {
		t.Fatalf("target marked damage = %d, want 0", got)
	}
	count := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.CounterChange && ev.Obj == target && ev.Counter == "M1M1" {
			count++
			if ev.Amount != 2 {
				t.Fatalf("counter placement amount = %d, want 2", ev.Amount)
			}
		}
	}
	if count != 1 {
		t.Fatalf("M1M1 CounterChange count = %d, want 1", count)
	}
}
