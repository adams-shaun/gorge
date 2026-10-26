package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// castToTargetAsk casts the hand's only spell with R mana floating and
// returns the KTarget decision it poses (CR 601.2c).
func castToTargetAsk(t *testing.T, e *Engine) *decision.Decision {
	t.Helper()
	e.G.Players[0].Pool[state.MR] = 1
	e.askPriority(0)
	castFirst(t, e, "cast")
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.TargetEffect == nil {
		t.Fatalf("pending = %+v, want a KTarget with a TargetEffect", d)
	}
	return d
}

// TestValidTgtsDescRidesTheTargetDecision: ValidTgtsDesc$ is the script's noun
// for the legal set (CR 115.1's "target" + noun phrase); the ask carries it.
func TestValidTgtsDescRidesTheTargetDecision(t *testing.T) {
	t.Parallel()
	spell := card(t, "Name:Hound Fixture\nManaCost:R\nTypes:Instant\n"+
		"A:SP$ Pump | ValidTgts$ Creature.YouCtrl | ValidTgtsDesc$ another creature you control | NumAtt$ +1\nOracle:x\n")
	e := handEngine(t, spell)
	bear := e.G.AddObject(card(t, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	bear.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{bear.ID})
	d := castToTargetAsk(t, e)
	if got := d.TargetEffect.Desc; got != "another creature you control" {
		t.Fatalf("TargetEffect.Desc = %q", got)
	}
}

// TestValidTgtsDescAbsentLeavesDescEmpty: no script noun, no synthesised one.
func TestValidTgtsDescAbsentLeavesDescEmpty(t *testing.T) {
	t.Parallel()
	bolt := card(t, "Name:Lightning Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n")
	e := handEngine(t, bolt)
	d := castToTargetAsk(t, e)
	if got := d.TargetEffect.Desc; got != "" {
		t.Fatalf("TargetEffect.Desc = %q, want empty", got)
	}
}

// TestValidTgtsDescChangeZoneNoLongerNotes: a ChangeZone carrier resolves
// without the "ignores unread parameter(s) ValidTgtsDesc$" Note.
func TestValidTgtsDescChangeZoneNoLongerNotes(t *testing.T) {
	t.Parallel()
	e, cfg, _ := newFixtureDeck(t, 1, "Name:Return Fixture\nManaCost:R\nTypes:Instant\n"+
		"A:SP$ ChangeZone | Origin$ Graveyard | Destination$ Battlefield | ValidTgts$ Creature.YouOwn | "+
		"ValidTgtsDesc$ creature card in your graveyard\nOracle:x\n",
		"Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	bear := moveToGraveyardFromLibrary(t, e, "Bear")
	addMana(t, e, 0, "R")
	castFirst(t, e, "cast")
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.TargetEffect == nil {
		t.Fatalf("pending = %+v, want a KTarget with a TargetEffect", d)
	}
	if d.TargetEffect.API != "ChangeZone" || d.TargetEffect.Desc != "creature card in your graveyard" {
		t.Fatalf("TargetEffect = %+v", d.TargetEffect)
	}
	answerTargetAsk(t, e, []state.ObjID{bear})
	passUntilStackEmpty(t, e, 40)
	if got := e.G.Obj(bear).Zone; got != state.ZBattlefield {
		t.Fatalf("bear in zone %v, want the battlefield (the carrier did not resolve)", got)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "ValidTgtsDesc$") {
			t.Fatalf("spurious Note: %q", ev.Text)
		}
	}
	replayCheck(t, e, cfg)
}
