package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Grievous Wound's CantGainLife static scopes itself with
// `ValidPlayer$ Player.EnchantedBy`, a source-relative read (the Aura names
// its own enchanted player). The scope read must carry the static's source:
// the enchanted seat cannot gain life, the other seat can.
func TestEnchantedByStaticScopeCarriesSource(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	wound := mustCorpusCard(t, reg, "Grievous Wound")
	e := handEngine(t, wound)
	id := e.G.Zone(state.ZHand, 0)[0]
	addMana(t, e, 0, "BBBBB")
	found := false
	for _, opt := range castOptions(t, e) {
		if opt.Obj == id {
			submitChoices(t, e, opt.Index)
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Grievous Wound cast not offered")
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected player target decision: %+v", d)
	}
	index := -1
	for _, o := range d.Options {
		if o.Player == 1 && o.Kind == "player" {
			index = o.Index
		}
	}
	if index < 0 {
		t.Fatalf("opponent target missing: %+v", d.Options)
	}
	submitChoices(t, e, index)
	passUntilStackEmpty(t, e, 20)
	o := e.G.Obj(id)
	if o.Zone != state.ZBattlefield || !o.HasAttachedPlayer || o.AttachedPlayer != 1 {
		t.Fatalf("precondition: Grievous Wound not attached to seat 1: %+v", o)
	}
	// A second, unrelated player-attached Aura sits on seat 0. A source-less
	// any-enchantment scan would now see seat 0 as "enchanted" too and
	// forbid its life gain; the source-bound read must not.
	other := onBoard(t, e, 1, playerEnchantAuraScript)
	e.emit(events.Event{Kind: events.Attach, Obj: other, Player: 0, Text: "attach to player"})
	if o := e.G.Obj(other); !o.HasAttachedPlayer || o.AttachedPlayer != 0 {
		t.Fatalf("precondition: second Aura not attached to seat 0: %+v", o)
	}
	if !e.lifeGainForbidden(1) {
		t.Error("the enchanted seat can gain life, want it forbidden")
	}
	if e.lifeGainForbidden(0) {
		t.Error("the unenchanted seat is forbidden to gain life, want it allowed")
	}
}

// The CantDraw and CantChangeLife scope reads thread the static's source the
// same way: a static Aura on seat 1 scopes to seat 1 only, even with an
// unrelated Aura attached to seat 0.
func TestEnchantedByDrawAndLifeChangeScopeCarriesSource(t *testing.T) {
	t.Parallel()
	const noDraw = "Name:Test Muzzle\nTypes:Enchantment Aura\nK:Enchant:Player\n" +
		"S:Mode$ CantDraw | ValidPlayer$ Player.EnchantedBy | Description$ x\nOracle:x\n"
	const noLife = "Name:Test Lockdown\nTypes:Enchantment Aura\nK:Enchant:Player\n" +
		"S:Mode$ CantChangeLife | ValidPlayer$ Player.EnchantedBy | Description$ x\nOracle:x\n"
	e := handEngine(t)
	muzzle := onBoard(t, e, 0, noDraw)
	lockdown := onBoard(t, e, 0, noLife)
	other := onBoard(t, e, 1, playerEnchantAuraScript)
	e.emit(events.Event{Kind: events.Attach, Obj: muzzle, Player: 1, Text: "attach to player"})
	e.emit(events.Event{Kind: events.Attach, Obj: lockdown, Player: 1, Text: "attach to player"})
	e.emit(events.Event{Kind: events.Attach, Obj: other, Player: 0, Text: "attach to player"})
	for _, id := range []state.ObjID{muzzle, lockdown} {
		if o := e.G.Obj(id); !o.HasAttachedPlayer || o.AttachedPlayer != 1 {
			t.Fatalf("precondition: Aura %d not attached to seat 1: %+v", id, o)
		}
	}
	if o := e.G.Obj(other); !o.HasAttachedPlayer || o.AttachedPlayer != 0 {
		t.Fatalf("precondition: unrelated Aura not attached to seat 0: %+v", o)
	}
	if !e.drawForbidden(1) {
		t.Error("the enchanted seat can draw, want it forbidden")
	}
	if e.drawForbidden(0) {
		t.Error("the unenchanted seat is forbidden to draw, want it allowed")
	}
	if !e.playerLifeChangeForbidden(1) {
		t.Error("the enchanted seat can change life, want it forbidden")
	}
	if e.playerLifeChangeForbidden(0) {
		t.Error("the unenchanted seat is forbidden to change life, want it allowed")
	}
}
