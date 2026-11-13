package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TargetsWithRelatedProperty$ (Spawnbroker LEPower, Puca's Mischief LECMC):
// the candidate's power / mana value may not exceed the ParentTarget's. The
// census post-filter and the resolution recheck share relatedPropertyAdmits.

// relatedCorpusLink returns the printed DBExchange link of a corpus card and
// the card itself, asserting the link carries the key under test.
func relatedCorpusLink(t *testing.T, name, want string) (*cards.Card, *cards.SA) {
	t.Helper()
	c, ok := testutil.CorpusRegistry(t).Lookup(name)
	if !ok {
		t.Fatalf("%s missing from corpus", name)
	}
	sa := cards.ResolveSVar(c.Faces[0].SVars, "DBExchange")
	if sa == nil || sa.ParamStr(cards.PKTargetsWithRelatedProperty) != want {
		t.Fatalf("precondition: %s DBExchange lost TargetsWithRelatedProperty$ %s", name, want)
	}
	return c, sa
}

func relatedOffered(t *testing.T, e *Engine, src state.ObjID, sa *cards.SA, id state.ObjID) bool {
	t.Helper()
	e.pending = nil
	e.askTarget(0, src, sa)
	d := e.Pending()
	if d == nil {
		t.Fatal("no target decision posed")
	}
	return indexOfObjOption(d, id) != -1
}

func TestTargetRelatedPropertySpawnbroker(t *testing.T) {
	t.Parallel()
	c, sa := relatedCorpusLink(t, "Spawnbroker", "LEPower")
	e := newSeats(t, 2)
	src := e.G.AddObject(c, 0).ID
	e.emit(events.Event{Kind: events.MoveZone, Obj: src, From: state.ZLibrary, To: state.ZBattlefield})
	own := putBattlefield(t, e, 0, "Name:Mine\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	small := putBattlefield(t, e, 1, "Name:Small\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	big := putBattlefield(t, e, 1, "Name:Big\nTypes:Creature Bear\nPT:5/5\nOracle:x\n")
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: src, IDs: []state.ObjID{own}})
	if e.Power(big) <= e.Power(own) || e.Power(small) > e.Power(own) || e.Power(own) == 0 {
		t.Fatalf("precondition: powers own=%d small=%d big=%d", e.Power(own), e.Power(small), e.Power(big))
	}
	if !relatedOffered(t, e, src, sa, small) {
		t.Fatal("equal-power creature was not offered")
	}
	if relatedOffered(t, e, src, sa, big) {
		t.Fatal("greater-power creature was offered despite TargetsWithRelatedProperty$ LEPower")
	}
	kept := e.legalTargets([]state.Target{{Obj: small}, {Obj: big}}, sa, targetZones(sa), 0, src, src)
	if len(kept) != 1 || kept[0].Obj != small {
		t.Fatalf("recheck = %+v, want only %d", kept, small)
	}
}

func TestTargetRelatedPropertyPucasMischief(t *testing.T) {
	t.Parallel()
	c, sa := relatedCorpusLink(t, "Puca's Mischief", "LECMC")
	e := newSeats(t, 2)
	src := e.G.AddObject(c, 0).ID
	e.emit(events.Event{Kind: events.MoveZone, Obj: src, From: state.ZLibrary, To: state.ZBattlefield})
	own := putBattlefield(t, e, 0, "Name:Mine\nManaCost:2\nTypes:Artifact\nOracle:x\n")
	cheap := putBattlefield(t, e, 1, "Name:Cheap\nManaCost:1\nTypes:Artifact\nOracle:x\n")
	dear := putBattlefield(t, e, 1, "Name:Dear\nManaCost:5\nTypes:Artifact\nOracle:x\n")
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: src, IDs: []state.ObjID{own}})
	cmc := func(id state.ObjID) int32 { return e.G.Obj(id).Face().Cmc() }
	if cmc(own) != 2 || cmc(cheap) != 1 || cmc(dear) != 5 {
		t.Fatalf("precondition: cmc own=%d cheap=%d dear=%d", cmc(own), cmc(cheap), cmc(dear))
	}
	if !relatedOffered(t, e, src, sa, cheap) {
		t.Fatal("lesser-MV permanent was not offered")
	}
	if relatedOffered(t, e, src, sa, dear) {
		t.Fatal("greater-MV permanent was offered despite TargetsWithRelatedProperty$ LECMC")
	}
	kept := e.legalTargets([]state.Target{{Obj: cheap}, {Obj: dear}}, sa, targetZones(sa), 0, src, src)
	if len(kept) != 1 || kept[0].Obj != cheap {
		t.Fatalf("recheck = %+v, want only %d", kept, cheap)
	}
}
