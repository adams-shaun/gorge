package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestDefenderCtrlForRemembered plays Mercadia's Downfall against two
// attackers aimed at different defenders: each attacker's X counts the
// nonbasic lands ITS defender controls (Forge's DefenderCtrlForRemembered,
// read per RepeatEach iteration).
func TestDefenderCtrlForRemembered(t *testing.T) {
	if got := UnknownPredicates("Land.nonBasic+DefenderCtrlForRemembered"); len(got) != 0 {
		t.Fatalf("UnknownPredicates = %v, want none", got)
	}
	const script = `Name:Mercadia's Downfall
ManaCost:2 R
Types:Instant
A:SP$ RepeatEach | RepeatSubAbility$ Boost | RepeatCards$ Creature.attacking
SVar:Boost:DB$ Pump | NumAtt$ +N | Defined$ Remembered
SVar:N:Count$Valid Land.nonBasic+DefenderCtrlForRemembered
Oracle:x
`
	card, diags := cards.ParseBytes("mercadias-downfall.txt", []byte(script))
	if len(diags) != 0 {
		t.Fatalf("parse fixture: %v", diags)
	}
	card.Link()

	h := newHost(t, 4)
	spell := h.g.AddObject(card, 0)
	atkA := h.g.AddObject(mkCard(t, "Name:A\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0)
	atkB := h.g.AddObject(mkCard(t, "Name:B\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0)
	nonbasic := func(seat state.PlayerID) state.ObjID {
		return h.g.AddObject(mkCard(t, "Name:Nonbasic\nTypes:Land\nOracle:x\n"), seat).ID
	}
	basic := h.g.AddObject(mkCard(t, "Name:Basic\nTypes:Basic Land\nOracle:x\n"), 2).ID
	lands := []state.ObjID{nonbasic(1), nonbasic(1), nonbasic(2), nonbasic(3), nonbasic(3), nonbasic(3), basic}
	for _, id := range append([]state.ObjID{atkA.ID, atkB.ID}, lands...) {
		h.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})
	}
	h.Emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{atkA.ID}})
	h.Emit(events.Event{Kind: events.DeclareAttackers, Player: 2, IDs: []state.ObjID{atkB.ID}})
	if a, b := h.g.Obj(atkA.ID), h.g.Obj(atkB.ID); !a.IsAttacking || !b.IsAttacking || a.Attacking == b.Attacking {
		t.Fatalf("precondition: two attackers must attack different defenders: %+v %+v", a, b)
	}

	ctx := &Ctx{Source: spell.ID, Controller: 0, SVars: card.Faces[0].SVars}
	Resolve(h, ctx, card.Faces[0].Abilities[0])

	got := map[state.ObjID]int32{}
	for _, ce := range h.continuous {
		got[ce.Source] += ce.AddPower
	}
	// Seat 1 controls two nonbasic lands, seat 2 one nonbasic (and a basic);
	// seat 3's three are not attacked and must count for nobody.
	if got[atkA.ID] != 2 || got[atkB.ID] != 1 {
		t.Fatalf("pump per attacker = %v, want A(+2 vs seat 1) B(+1 vs seat 2); effects %+v", got, h.continuous)
	}

	// A remembered object that is not attacking names no defender: the
	// token is refused under '!' as well, never a match-everything.
	idle := h.g.AddObject(mkCard(t, "Name:Idle\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	sc := SpecContext{You: 0, Remembered: []state.Target{{Obj: idle.ID}}}
	for _, spec := range []string{"Land.DefenderCtrlForRemembered", "Land.!DefenderCtrlForRemembered"} {
		if MatchesObjectCtx(h.g, spec, h.g.Obj(lands[0]), sc) {
			t.Errorf("%s matched with no attacking remembered object", spec)
		}
	}
}
