package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Codsworth, Handy Helper's {T}: "Attach target Aura or Equipment you control
// to target creature you control" -- the attached object is the first target
// (Object$ ParentTarget), so the source creature is a legal second target. The
// attach-source exclusion belongs to the attach that fastens the source itself
// (Equip, CR 301.5c).
func TestAttachParentTargetDestinationMayBeTheSource(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _, ids := dsBoard(t, reg, "Codsworth, Handy Helper", "Bonesplitter")
	cods, sword := ids["Codsworth, Handy Helper"], ids["Bonesplitter"]
	for _, id := range []state.ObjID{cods, sword} {
		if e.G.Obj(id).Zone != state.ZBattlefield {
			t.Fatalf("precondition: object %d must be on the battlefield", id)
		}
	}
	e.G.Obj(cods).SummonSick = false
	e.askPriority(0)
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("expected seat 0's priority, got %+v", d)
	}
	activate := -1
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == cods && o.Ability == 1 {
			activate = o.Index
		}
	}
	if activate < 0 {
		t.Fatalf("precondition: Codsworth's attach ability was not offered: %+v", d.Options)
	}
	submitChoices(t, e, activate)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected the Equipment target ask, got %+v", d)
	}
	pick := -1
	for _, o := range d.Options {
		if o.Obj == sword {
			pick = o.Index
		}
	}
	if pick < 0 {
		t.Fatalf("the Equipment target ask does not offer Bonesplitter: %+v", d.Options)
	}
	submitChoices(t, e, pick)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected the creature target ask, got %+v", d)
	}
	for _, o := range d.Options {
		if o.Obj == cods {
			return
		}
	}
	t.Fatalf("the creature target ask does not offer Codsworth: %+v", d.Options)
}
