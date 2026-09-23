package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestDigUntilAuraCanEnchantOpponentsCreature(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	own := h.g.AddObject(mkCard(t, "Name:Own Bear\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0).ID
	opp := h.g.AddObject(mkCard(t, "Name:Opponent Bear\nTypes:Creature\nPT:3/3\nOracle:x\n"), 1).ID
	aura := h.g.AddObject(mkCard(t, "Name:Halo\nManaCost:W\nTypes:Enchantment Aura\nK:Enchant:Creature\nOracle:x\n"), 0).ID
	h.g.SetZone(state.ZLibrary, 0, []state.ObjID{own, aura})
	h.g.SetZone(state.ZLibrary, 1, []state.ObjID{opp})
	h.Emit(events.Event{Kind: events.MoveZone, Obj: own, From: state.ZLibrary, To: state.ZBattlefield})
	h.Emit(events.Event{Kind: events.MoveZone, Obj: opp, From: state.ZLibrary, To: state.ZBattlefield})

	// The enemy creature is on its own battlefield, not the caster's;
	// both creatures are legal but distinct bearer choices.
	if h.g.Obj(aura).Zone != state.ZLibrary || h.g.Obj(opp).Zone != state.ZBattlefield ||
		h.g.Obj(opp).Controller != 1 || h.g.Obj(own).Zone != state.ZBattlefield ||
		own == opp || !MatchesSpecFrom(h.g, "Creature", own, 0, aura) ||
		!MatchesSpecFrom(h.g, "Creature", opp, 0, aura) {
		t.Fatal("setup: expected a library Aura and two distinct eligible creatures controlled by opposing seats")
	}
	ability := sa(t, "SP$ DigUntil | Valid$ Aura | FoundDestination$ Battlefield")
	Resolve(h, &Ctx{Controller: 0}, ability)
	if h.asked == nil || h.asked.Kind != decision.KChoose || h.asked.ResumeKind != "diguntil_aura" {
		t.Fatalf("bearer decision = %+v, want diguntil_aura choice", h.asked)
	}
	if len(h.asked.Options) != 2 || h.asked.Options[0].Obj != own || h.asked.Options[1].Obj != opp {
		t.Fatalf("bearer options = %+v, want own %d then opponent %d", h.asked.Options, own, opp)
	}
	if h.g.Obj(aura).Zone != state.ZLibrary {
		t.Fatalf("Aura moved before choice: %s", h.g.Obj(aura).Zone)
	}
	Resolve(h, &Ctx{Controller: 0, DigUntilAuraBearer: opp, DigUntilAuraDone: true}, ability)
	if o := h.g.Obj(aura); o.Zone != state.ZBattlefield || o.AttachedTo != opp {
		t.Fatalf("Aura zone/attachment = %s/%d, want battlefield/%d", o.Zone, o.AttachedTo, opp)
	}
}
