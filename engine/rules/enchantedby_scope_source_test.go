package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
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
	if !e.lifeGainForbidden(1) {
		t.Error("the enchanted seat can gain life, want it forbidden")
	}
	if e.lifeGainForbidden(0) {
		t.Error("the unenchanted seat is forbidden to gain life, want it allowed")
	}
}
