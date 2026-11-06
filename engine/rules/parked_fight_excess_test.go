package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const parkedFighterSrc = "Name:Test Lifelink Giant\nManaCost:3 R\nTypes:Creature Giant\nPT:3/3\nK:Lifelink\nOracle:x\n"
const parkedBearSrc = "Name:Test Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

func TestParkedFightExcessAndLifelinkOnce(t *testing.T) {
	agni := corpusCardText(t, "t/the_last_agni_kai.txt")
	fated := corpusCardText(t, "f/fated_firepower.txt")
	e, cfg, spell := newFixtureDeck(t, 3, agni, fated, fated, parkedFighterSrc)
	f1 := moveSeeded(t, e, 0, fated, state.ZBattlefield)
	f2 := moveSeeded(t, e, 0, fated, state.ZBattlefield)
	e.emit(events.Event{Kind: events.CounterChange, Obj: f1, Counter: "FIRE", Amount: 1})
	e.emit(events.Event{Kind: events.CounterChange, Obj: f2, Counter: "FIRE", Amount: 1})
	e.pending = nil
	e.Advance()
	fighter := moveSeeded(t, e, 0, parkedFighterSrc, state.ZBattlefield)
	opp := putToken(t, e, 1, parkedBearSrc, state.ZBattlefield)
	// Preconditions: the replacement sources are on the battlefield with one
	// fire counter each, the fighter really has lifelink, and the victim is a
	// 2/2 (lethal 2) so a 5-damage hit leaves 3 excess.
	if o := e.G.Obj(f1); o == nil || o.Zone != state.ZBattlefield || o.Counter("FIRE") != 1 {
		t.Fatalf("first Fated Firepower not a battlefield permanent with one FIRE counter: %+v", o)
	}
	if o := e.G.Obj(f2); o == nil || o.Zone != state.ZBattlefield || o.Counter("FIRE") != 1 {
		t.Fatalf("second Fated Firepower not a battlefield permanent with one FIRE counter: %+v", o)
	}
	if !e.HasKeyword(fighter, "Lifelink") {
		t.Fatalf("fighter does not have lifelink")
	}
	if o := e.G.Obj(opp); o == nil || !e.IsCreature(opp) || e.Toughness(opp) != 2 {
		t.Fatalf("victim is not a 2-toughness creature: %+v", o)
	}
	addMana(t, e, 0, "RRR")
	start := len(e.L.Events)
	submitChoices(t, e, castOptionFor(t, e, spell).Index)
	replAsks := 0
	for i := 0; i < 40 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision with the stack at depth %d", len(e.G.Stack))
		}
		switch d.Kind {
		case decision.KTarget, decision.KChoose:
			submitChoices(t, e, 0)
		case decision.KReplacement:
			replAsks++
			if len(d.Options) != 2 {
				t.Fatalf("replacement ask has %d options, want 2 Fated Firepowers: %+v", len(d.Options), d.Options)
			}
			submitChoices(t, e, 0)
		case decision.KPriority:
			pass := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					pass = o.Index
				}
			}
			if pass < 0 {
				t.Fatalf("priority ask with no pass option: %+v", d.Options)
			}
			submitChoices(t, e, pass)
		default:
			t.Fatalf("unexpected decision %s while draining", d.Kind)
		}
	}
	if replAsks != 1 {
		t.Fatalf("replacement-order asks = %d, want 1", replAsks)
	}
	// The hit landed for 3 + 1 + 1, lethal was 2, so The Last Agni Kai's
	// ExcessSVar is 3 and DBMana adds 3 red.
	var landed, red int32
	for _, ev := range e.L.Events[start:] {
		if ev.Kind == events.Damage && ev.Obj == opp {
			landed = ev.Amount
		}
		if ev.Kind == events.ManaAdd && ev.Counter == "R" && ev.Player == 0 {
			red = ev.Amount
		}
	}
	if landed != 5 {
		t.Fatalf("damage dealt to the victim = %d, want 5", landed)
	}
	if red != 3 {
		t.Fatalf("DBMana added %d red, want 3 (excess 5-2)", red)
	}
	if got := e.G.Players[0].Life; got != 25 {
		t.Fatalf("controller life = %d, want 25 (20 + one 5-damage lifelink gain)", got)
	}
	replayCheck(t, e, cfg)
}
