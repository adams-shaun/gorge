package rules

// This file pins the end-to-end real-card path that the bare `Attached` and the
// contextual `AttachedTo <ref>` filter predicates unlock: Arna Kennerüd,
// Skycaptain's real CopyPermanent source filter
// `Defined$ Valid Permanent.!token+AttachedTo TriggeredAttackerLKICopy`.
//
// Before the filter work, both predicates failed closed, so the copy rider was
// only reachable by substituting a resolvable `Defined$ Valid Permanent`
// (the older TestArnaCopy in copypermanent_grants_test.go documents that
// substitution). This test drives Arna's UNMODIFIED SA and binds the trigger's
// remembered attacker the way a real Attacks trigger does, proving the real
// source filter now selects the attached Equipment.
//
// The trigger that STARTS Arna in a real game is still unreachable for a
// separate reason: `ValidCard$ Creature.modified+YouCtrl` needs the `modified`
// CardProperty (CR 700.9), which this build does not implement, so the trigger
// never fires. That gap is filed as an issue, not closed here; this test
// reaches the rider through Arna's real Execute body, which is the part the
// brief names.

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestArnaRealSourceFilterReachesCopyRider drives Arna's DBCopyPermanents body
// with its own `Defined` parameter intact. The triggering attacker is bound via
// Ctx.Remembered -- the exact set effects' Defined$ TriggeredAttackerLKICopy
// resolves -- so the real filter must select the Equipment attached to that
// attacker and the rider must mint an attached copy of it. A second, unattached
// Equipment must not be copied: that is the negative half of the source filter,
// and it fails if `AttachedTo TriggeredAttackerLKICopy` degrades to always-true
// (which a bare `Attached`-style predicate that ignored the referent would).
func TestArnaRealSourceFilterReachesCopyRider(t *testing.T) {
	reg := searchTestRegistry(t)
	arnaCard := lookup(t, reg, "Arna Kennerüd, Skycaptain")
	e, cfg := corpusEngineCfg(t, reg,
		[]*cards.Card{arnaCard, lookup(t, reg, "Grizzly Bears"), lookup(t, reg, "Bonesplitter"), lookup(t, reg, "Bonesplitter")},
		[]*cards.Card{})

	arna := moveByName(t, e, 0, "Arna Kennerüd, Skycaptain", state.ZBattlefield)
	bear := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	equip := moveByName(t, e, 0, "Bonesplitter", state.ZBattlefield)
	free := moveByName(t, e, 0, "Bonesplitter", state.ZBattlefield)
	e.emit(events.Event{Kind: events.Attach, Obj: equip, IDs: []state.ObjID{bear}})

	// Preconditions: the attached Equipment really is attached to the bound
	// attacker, the second Equipment really is unattached, and all three are on
	// the battlefield. A vacuous setup must fail here, not pass below.
	if got := e.G.Obj(equip).AttachedTo; got != bear {
		t.Fatalf("precondition failed: attached Equipment %d AttachedTo = %d, want attacker %d", equip, got, bear)
	}
	if got := e.G.Obj(free).AttachedTo; got != 0 {
		t.Fatalf("precondition failed: second Equipment %d AttachedTo = %d, want 0 (unattached)", free, got)
	}
	if e.G.Obj(equip).Zone != state.ZBattlefield || e.G.Obj(bear).Zone != state.ZBattlefield || e.G.Obj(free).Zone != state.ZBattlefield {
		t.Fatalf("precondition failed: zones equip=%s bear=%s free=%s, want all battlefield",
			e.G.Obj(equip).Zone, e.G.Obj(bear).Zone, e.G.Obj(free).Zone)
	}
	if equip == free {
		t.Fatalf("precondition failed: both Equipment share id %d", equip)
	}
	equipCard := e.G.Obj(equip).Card

	// Arna's real SVar body, with its real Defined$ and AttachedTo$ parameters.
	sa := resolveSourceFaceSA(t, e, arna, "DBCopyPermanents")
	if defined := sa.Params["Defined"]; defined != "Valid Permanent.!token+AttachedTo TriggeredAttackerLKICopy" {
		t.Fatalf("precondition failed: Arna DBCopyPermanents Defined = %q, want the real source filter", defined)
	}

	// Bind the triggering attacker the way the Attacks trigger does (Remembered)
	// and resolve. No `Defined` override: the real filter must do the selection.
	ctx := &effects.Ctx{Source: arna, Controller: 0, Remembered: []state.Target{{Obj: bear}}}
	effects.Resolve(e, ctx, sa)
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
		t.Fatalf("Arna's real source filter posed an unexpected ask: %+v", d)
	}

	// The attached Equipment was copied and the copy entered attached to the
	// attacker (Arna's real AttachedTo$ TriggeredAttackerLKICopy endpoint).
	copyID := findTokenCopyOf(t, e, equipCard, equip)
	if e.G.Obj(copyID).Zone != state.ZBattlefield {
		t.Fatalf("precondition failed: the Equipment copy is not on the battlefield (zone %s)", e.G.Obj(copyID).Zone)
	}
	if got := e.G.Obj(copyID).AttachedTo; got != bear {
		t.Fatalf("the Equipment copy AttachedTo = %d, want the bound attacker %d", got, bear)
	}

	// The unattached second Equipment must NOT have been copied: exactly one
	// token copy of Bonesplitter exists.
	copies := 0
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.IsCopy && o.Card == equipCard {
			copies++
		}
	}
	if copies != 1 {
		t.Fatalf("Bonesplitter copies = %d, want exactly 1 (the unattached second Equipment must be filtered out)", copies)
	}
	noCopyPermanentModNote(t, e, "AttachedTo")
	replayCheck(t, e, cfg)
}
