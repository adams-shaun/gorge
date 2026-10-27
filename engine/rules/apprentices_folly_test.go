package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// follyCastToChapterOneTarget casts The Apprentice's Folly with the Bear (and,
// when extra is non-empty, a second nontoken creature) on seat 0's
// battlefield and passes until chapter I's target ask is pending.
func follyCastToChapterOneTarget(t *testing.T, extra []string) (e *Engine, cfg Config, bear state.ObjID) {
	t.Helper()
	e, cfg, _ = altCostEngine(t, 742, []string{"The Apprentice's Folly"}, append([]string{altBearSrc}, extra...), nil)
	bear = findCardObj(t, e, 0, "Bear", state.ZBattlefield)
	for _, n := range extra {
		_ = n
		findCardObj(t, e, 0, "Test Dragon", state.ZBattlefield)
	}
	saga := findCardObj(t, e, 0, "The Apprentice's Folly", state.ZHand)
	addMana(t, e, 0, "CCUR")
	submitChoices(t, e, castModeOption(t, e, saga, ""))
	passUntilFollyTarget(t, e, e.G.Turn)
	return e, cfg, bear
}

// passUntilFollyTarget passes priority until a KTarget ask is pending, failing
// if the game reaches turn limit's end first.
func passUntilFollyTarget(t *testing.T, e *Engine, turn int32) *decision.Decision {
	t.Helper()
	for i := 0; i < 400 && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision pending while waiting for chapter's target ask")
		}
		if d.Kind == decision.KTarget {
			return d
		}
		if e.G.Turn > turn {
			break
		}
		if answerIfDiscard(t, e) {
			continue
		}
		if d.Kind == decision.KAttackers || d.Kind == decision.KBlockers {
			submitChoices(t, e)
			continue
		}
		passOnceP(t, e)
	}
	t.Fatalf("no KTarget ask before the end of turn %d (turn %d, step %s)", turn, e.G.Turn, e.G.Step)
	return nil
}

func follyOfferIndex(d *decision.Decision, id state.ObjID) int {
	for _, o := range d.Options {
		if o.Obj == id {
			return o.Index
		}
	}
	return -1
}

func TestApprenticesFollyChapterOneCopiesNontokenCreature(t *testing.T) {
	e, cfg, bear := follyCastToChapterOneTarget(t, nil)
	d := e.Pending()
	pick := follyOfferIndex(d, bear)
	if pick < 0 {
		t.Fatalf("chapter I does not offer the Bear: %+v", d.Options)
	}
	submitChoices(t, e, pick)
	passUntilStackEmpty(t, e, 40)
	var tok state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.IsToken && o.Face().Name == "Bear" {
			tok = id
		}
	}
	if tok == 0 {
		t.Fatal("no token copy of the Bear on seat 0's battlefield")
	}
	if !e.HasKeyword(tok, "Haste") {
		t.Errorf("the copy lacks Haste: %v", e.Derived(tok).Keywords)
	}
	types := e.Derived(tok).Types
	hasType := func(w string) bool {
		for _, x := range types {
			if x == w {
				return true
			}
		}
		return false
	}
	if !hasType("Reflection") {
		t.Errorf("the copy is not a Reflection: %v", types)
	}
	if hasType("Legendary") {
		t.Errorf("the copy is legendary: %v", types)
	}
	replayCheck(t, e, cfg)
}

func TestApprenticesFollyChapterTwoRefusesNameSharedWithToken(t *testing.T) {
	e, _, bear := follyCastToChapterOneTarget(t, []string{altDragonSrc})
	d := e.Pending()
	dragon := state.ObjID(0)
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Test Dragon" {
			dragon = id
		}
	}
	if dragon == 0 || follyOfferIndex(d, bear) < 0 || follyOfferIndex(d, dragon) < 0 {
		t.Fatalf("precondition: chapter I must offer the Bear (%d) and the Dragon (%d): %+v", bear, dragon, d.Options)
	}
	submitChoices(t, e, follyOfferIndex(d, bear))
	passUntilStackEmpty(t, e, 40)
	// Chapter II triggers at seat 0's next precombat main.
	d2 := passUntilFollyTarget(t, e, e.G.Turn+2)
	if e.G.Active != 0 || e.G.Step != state.StepMain1 {
		t.Fatalf("precondition: chapter II ask at seat %d step %s, want seat 0 main1", e.G.Active, e.G.Step)
	}
	if follyOfferIndex(d2, bear) >= 0 {
		t.Errorf("chapter II offers the Bear although a token named Bear exists: %+v", d2.Options)
	}
	if follyOfferIndex(d2, dragon) < 0 {
		t.Errorf("chapter II does not offer the differently named Dragon: %+v", d2.Options)
	}
}
