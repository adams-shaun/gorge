package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// scavengeOption returns the priority option index activating id's Scavenge
// ability, or -1 when none is offered.
func scavengeOption(t *testing.T, e *Engine, id state.ObjID) int {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority decision pending: %+v", d)
	}
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == id && strings.Contains(o.Label, "Scavenge") {
			return o.Index
		}
	}
	return -1
}

// scavengeOnto activates src's Scavenge and targets the creature target.
func scavengeOnto(t *testing.T, e *Engine, src, target state.ObjID) {
	t.Helper()
	idx := scavengeOption(t, e, src)
	if idx < 0 {
		t.Fatalf("no Scavenge option for %d", src)
	}
	submitChoices(t, e, idx)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("Scavenge posed no target ask: %+v", d)
	}
	pick := -1
	for _, o := range d.Options {
		if o.Obj == target {
			pick = o.Index
		}
	}
	if pick < 0 {
		t.Fatalf("target %d not offered: %+v", target, d.Options)
	}
	submitChoices(t, e, pick)
	passUntilStackEmpty(t, e, 40)
}

// TestDodgyJalopyScavengePutsItsPowerCounters drives the real Dodgy Jalopy
// (K:Scavenge:2 G, P/T */5 with a CDA reading the greatest mana value among
// creatures you control). Its power in the graveyard is the CDA's value
// (CR 208.2a, 604.3), not the printed star's 0, and Scavenge puts that many
// +1/+1 counters on the target (CR 702.97a, 608.2h).
func TestDodgyJalopyScavengePutsItsPowerCounters(t *testing.T) {
	t.Parallel()
	e, cfg := paidCostEngine(t, []string{"Dodgy Jalopy", "Hill Giant"}, nil)
	jalopy := paidCostMoveTo(t, e, 0, "Dodgy Jalopy", state.ZGraveyard)
	giant := paidCostMoveTo(t, e, 0, "Hill Giant", state.ZBattlefield)
	o := e.G.Obj(jalopy)
	if o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: Jalopy zone %s, want graveyard", o.Zone)
	}
	if !o.Face().CharacteristicDefining() || o.Face().Power() != 0 {
		t.Fatalf("precondition: Jalopy CDA=%v printed power=%d, want CDA with 0", o.Face().CharacteristicDefining(), o.Face().Power())
	}
	if got := e.Power(jalopy); got != 4 {
		t.Fatalf("precondition: Jalopy derived power %d, want 4 (Hill Giant's mana value)", got)
	}
	addMana(t, e, 0, "CCG")
	scavengeOnto(t, e, jalopy, giant)
	if z := e.G.Obj(jalopy).Zone; z != state.ZExile {
		t.Fatalf("Jalopy zone %s after Scavenge, want exile", z)
	}
	if got := e.G.Obj(giant).Counter("P1P1"); got != 4 {
		t.Fatalf("Hill Giant +1/+1 counters = %d, want 4", got)
	}
	replayCheck(t, e, cfg)
}

// TestBoneyardMycodraxScavengeUsesItsCDA is the second CDA shape: Boneyard
// Mycodrax's power counts the OTHER creature cards in its owner's graveyard.
func TestBoneyardMycodraxScavengeUsesItsCDA(t *testing.T) {
	t.Parallel()
	e, cfg := paidCostEngine(t, []string{"Boneyard Mycodrax", "Hill Giant", "Grizzly Bears"}, nil)
	myc := paidCostMoveTo(t, e, 0, "Boneyard Mycodrax", state.ZGraveyard)
	giant := paidCostMoveTo(t, e, 0, "Hill Giant", state.ZBattlefield)
	paidCostMoveTo(t, e, 0, "Grizzly Bears", state.ZGraveyard)
	if o := e.G.Obj(myc); o.Zone != state.ZGraveyard || o.Face().Power() != 0 {
		t.Fatalf("precondition: Mycodrax zone %s printed power %d, want graveyard and 0", o.Zone, o.Face().Power())
	}
	if got := e.Power(myc); got != 1 {
		t.Fatalf("precondition: Mycodrax derived power %d, want 1", got)
	}
	addMana(t, e, 0, "CCCCB")
	scavengeOnto(t, e, myc, giant)
	if z := e.G.Obj(myc).Zone; z != state.ZExile {
		t.Fatalf("Mycodrax zone %s after Scavenge, want exile", z)
	}
	if got := e.G.Obj(giant).Counter("P1P1"); got != 1 {
		t.Fatalf("Hill Giant +1/+1 counters = %d, want 1", got)
	}
	replayCheck(t, e, cfg)
}

// TestScavengeOnlyOfferedFromTheGraveyard: CR 702.97a's activation zone is
// the graveyard; hand and battlefield offer nothing.
func TestScavengeOnlyOfferedFromTheGraveyard(t *testing.T) {
	t.Parallel()
	e, _ := paidCostEngine(t, []string{"Dodgy Jalopy", "Hill Giant"}, nil)
	paidCostMoveTo(t, e, 0, "Hill Giant", state.ZBattlefield)
	jalopy := paidCostMoveTo(t, e, 0, "Dodgy Jalopy", state.ZHand)
	addMana(t, e, 0, "CCG")
	if z := e.G.Obj(jalopy).Zone; z != state.ZHand {
		t.Fatalf("precondition: Jalopy zone %s, want hand", z)
	}
	if idx := scavengeOption(t, e, jalopy); idx >= 0 {
		t.Fatal("Scavenge offered from hand")
	}
	paidCostMoveTo(t, e, 0, "Dodgy Jalopy", state.ZBattlefield)
	if z := e.G.Obj(jalopy).Zone; z != state.ZBattlefield {
		t.Fatalf("precondition: Jalopy zone %s, want battlefield", z)
	}
	if idx := scavengeOption(t, e, jalopy); idx >= 0 {
		t.Fatal("Scavenge offered from the battlefield")
	}
	paidCostMoveTo(t, e, 0, "Dodgy Jalopy", state.ZGraveyard)
	if idx := scavengeOption(t, e, jalopy); idx < 0 {
		t.Fatal("Scavenge not offered from the graveyard")
	}
}

// TestScavengePrimitiveIsRegistered pins the census half: kw:Scavenge is a
// supported primitive and Dodgy Jalopy no longer names it as missing.
func TestScavengePrimitiveIsRegistered(t *testing.T) {
	t.Parallel()
	supported := effects.Supported()
	if !supported["kw:Scavenge"] {
		t.Fatal(`effects.Supported() is missing "kw:Scavenge"`)
	}
	reg := testutil.CorpusRegistry(t)
	c, ok := reg.Lookup("Dodgy Jalopy")
	if !ok {
		t.Fatal("corpus lacks Dodgy Jalopy")
	}
	if missing := reg.Unsupported(c, supported); len(missing) != 0 {
		t.Fatalf("Dodgy Jalopy still reports missing primitives: %v", missing)
	}
}
