package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Thorin, Mountain-king's reflexive half: "that creature deals damage equal
// to its power to up to one target creature". The creature is the reflexive
// instance's remembered object (RememberObjects$ ParentTarget) and X is
// TriggerRemembered$CardPower. Raging Goblin (1/1) carrying Bonesplitter
// (+2/+0) has power 3; bounced in response, the card in hand has its printed
// power 1. CR 608.2h: the trigger uses the creature's last known power, 3,
// taken as the reflexive trigger went on the stack (rememberedPTLKI). Still on
// the battlefield, it reads the live power instead.

// thorinReflexive plays Thorin onto a board of Raging Goblin, Bonesplitter
// and Hill Giant, attaches Bonesplitter to the Goblin, aims the reflexive
// damage at Hill Giant and runs respond (the Goblin's equipped power
// asserted first) while the reflexive trigger waits on the stack. It returns
// the damage dealt to Hill Giant (-1 when none).
func thorinReflexive(t *testing.T, respond func(e *Engine, goblin state.ObjID)) (*Engine, Config, map[string]state.ObjID, int32) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e, cfg, ids := dsBoard(t, reg, "Raging Goblin", "Bonesplitter", "Hill Giant", "Thorin, Mountain-king")
	goblin, splitter, giant := ids["Raging Goblin"], ids["Bonesplitter"], ids["Hill Giant"]
	thorin := ids["Thorin, Mountain-king"]
	moveToHand(t, e, thorin) // parked by dsBoard; cast it so its ETB really triggers
	addMana(t, e, 0, "RRRR")
	castFirstOf(t, e, thorin)
	responded := false
	for i := 0; i < 60; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision pending")
		}
		if d.Kind == decision.KPriority && len(e.G.Stack) == 0 {
			break
		}
		switch d.Kind {
		case decision.KPriority:
			if !responded && e.G.Obj(splitter).AttachedTo == goblin && len(e.G.Stack) > 0 {
				if got := e.Power(goblin); got != 3 {
					t.Fatalf("precondition: equipped Raging Goblin power = %d, want 3", got)
				}
				respond(e, goblin)
				responded = true
				e.priorityRound()
				continue
			}
			passFirst(t, e)
		case decision.KTarget:
			pick := -1
			order := []state.ObjID{splitter, goblin}
			if e.G.Obj(splitter).AttachedTo == goblin {
				order = []state.ObjID{giant} // the reflexive half's damage target
			}
			for _, want := range order {
				for _, o := range d.Options {
					if o.Obj == want && o.Kind != "done" {
						pick = o.Index
						break
					}
				}
				if pick >= 0 {
					break
				}
			}
			if pick < 0 {
				for _, o := range d.Options {
					if o.Kind == "done" || o.Obj == 0 {
						pick = o.Index
					}
				}
			}
			if pick < 0 {
				t.Fatalf("target ask without a pick: %+v", d.Options)
			}
			submitChoices(t, e, pick)
		case decision.KTriggerOrder:
			idx := make([]int, 0, len(d.Options))
			for _, o := range d.Options {
				idx = append(idx, o.Index)
			}
			submitChoices(t, e, idx...)
		case decision.KTriggerOptional:
			submitChoices(t, e, 0)
		default:
			t.Fatalf("unexpected decision %q: %+v", d.Kind, d.Options)
		}
	}
	if !responded {
		t.Fatal("precondition: the reflexive trigger never waited on the stack with Bonesplitter on the Goblin")
	}
	damage := int32(-1)
	for _, ev := range e.L.Events {
		if ev.Kind == events.Damage && ev.Obj == giant {
			damage = ev.Amount
		}
	}
	return e, cfg, ids, damage
}

// TestReflexiveRememberedPowerUsesLKIWhenBounced: the Goblin is returned to
// its owner's hand in response; Hill Giant takes 3, not the card's printed 1.
func TestReflexiveRememberedPowerUsesLKIWhenBounced(t *testing.T) {
	t.Parallel()
	e, cfg, ids, damage := thorinReflexive(t, func(e *Engine, goblin state.ObjID) {
		moveToHand(t, e, goblin)
	})
	goblin := ids["Raging Goblin"]
	if z := e.G.Obj(goblin).Zone; z != state.ZHand {
		t.Fatalf("precondition: Raging Goblin zone = %s at resolution, want hand", z)
	}
	if f := e.G.Obj(goblin).Face(); f == nil || f.Power() != 1 {
		t.Fatalf("precondition: Raging Goblin printed face = %+v, want power 1", f)
	}
	if damage != 3 {
		t.Fatalf("Thorin's reflexive damage = %d, want 3 (the bounced Goblin's last known power, CR 608.2h)", damage)
	}
	replayCheck(t, e, cfg)
}

// TestReflexiveRememberedPowerReadsLiveOnBattlefield: the Goblin stays and
// gets a +1/+1 counter in response; Hill Giant takes its live power, 4.
func TestReflexiveRememberedPowerReadsLiveOnBattlefield(t *testing.T) {
	t.Parallel()
	e, cfg, ids, damage := thorinReflexive(t, func(e *Engine, goblin state.ObjID) {
		e.emit(events.Event{Kind: events.CounterChange, Obj: goblin, Counter: "P1P1", Amount: 1})
		if got := e.Power(goblin); got != 4 {
			t.Fatalf("precondition: pumped Raging Goblin power = %d, want 4", got)
		}
	})
	if z := e.G.Obj(ids["Raging Goblin"]).Zone; z != state.ZBattlefield {
		t.Fatalf("precondition: Raging Goblin zone = %s, want battlefield", z)
	}
	if damage != 4 {
		t.Fatalf("Thorin's reflexive damage = %d, want 4 (the Goblin's live power)", damage)
	}
	replayCheck(t, e, cfg)
}
