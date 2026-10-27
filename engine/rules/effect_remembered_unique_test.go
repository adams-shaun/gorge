package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// mayPlayGrantFor returns the Effect-created may-play grant that remembers id
// and whose key differs from skipKey ("" skips nothing), or nil.
func mayPlayGrantFor(e *Engine, id state.ObjID, skipKey string) *state.ContinuousEffect {
	for i := range e.continuous {
		ce := &e.continuous[i]
		if ce.MayPlay && objIDIn(ce.Remembered, id) && (skipKey == "" || mayPlayEffectKey(ce) != skipKey) {
			return ce
		}
	}
	return nil
}

// mayPlayGrantByKey returns the may-play grant with the given key, or nil.
func mayPlayGrantByKey(e *Engine, key string) *state.ContinuousEffect {
	for i := range e.continuous {
		ce := &e.continuous[i]
		if ce.MayPlay && mayPlayEffectKey(ce) == key {
			return ce
		}
	}
	return nil
}

// TestFlameskullEffectRememberedIsUniqueAcrossRecasts pins that Flameskull's
// Rejuvenation Effect registers its remembered set without a duplicate (the
// trigger capture plus RememberChanged$ True hold the card twice in the
// resolution), so the ForgetOnMoved$ sweep fully forgets the recast card and
// a later recast is charged to the newer grant rather than the first one.
func TestFlameskullEffectRememberedIsUniqueAcrossRecasts(t *testing.T) {
	reg := choiceCorpusRegistry(t)
	fl := choiceCorpusCard(t, "Flameskull")
	shock := choiceCorpusCard(t, "Shock")
	e := corpusEngine(t, reg, []*cards.Card{shock, shock}, nil)
	fs := onBoardCard(t, e, 0, fl)

	addMana(t, e, 0, "R")
	castAt(t, e, findCardObj(t, e, 0, "Shock", state.ZHand), fs)
	if e.G.Obj(fs).Zone != state.ZExile {
		t.Fatalf("Flameskull not exiled after first death: zone %v", e.G.Obj(fs).Zone)
	}
	first := mayPlayGrantFor(e, fs, "")
	if first == nil {
		t.Fatalf("no may-play grant remembers Flameskull after the first death")
	}
	firstKey := mayPlayEffectKey(first)
	if n := countObjID(first.Remembered, fs); n != 1 {
		t.Fatalf("first grant holds Flameskull %d times, want 1: %v", n, first.Remembered)
	}
	seen := map[state.ObjID]bool{}
	for _, id := range first.Remembered {
		if seen[id] {
			t.Fatalf("first grant remembered set has duplicate %d: %v", id, first.Remembered)
		}
		seen[id] = true
	}

	addMana(t, e, 0, "RRR")
	perStaticCast(t, e, fs)
	if e.G.Obj(fs).Zone != state.ZBattlefield {
		t.Fatalf("Flameskull not recast: zone %v", e.G.Obj(fs).Zone)
	}
	ce := mayPlayGrantByKey(e, firstKey)
	if ce == nil {
		t.Fatalf("first grant %q vanished", firstKey)
	}
	if objIDIn(ce.Remembered, fs) {
		t.Fatalf("first grant still remembers Flameskull after it left exile: %v", ce.Remembered)
	}

	driveToStep(t, e, e.G.Turn+2, 0, state.StepMain1)

	addMana(t, e, 0, "R")
	castAt(t, e, findCardObj(t, e, 0, "Shock", state.ZHand), fs)
	if e.G.Obj(fs).Zone != state.ZExile {
		t.Fatalf("Flameskull not exiled after second death: zone %v", e.G.Obj(fs).Zone)
	}
	second := mayPlayGrantFor(e, fs, firstKey)
	if second == nil {
		t.Fatalf("no second may-play grant remembers Flameskull")
	}
	if n := countObjID(second.Remembered, fs); n != 1 {
		t.Fatalf("second grant holds Flameskull %d times, want 1: %v", n, second.Remembered)
	}
	secondKey := mayPlayEffectKey(second)
	if secondKey == firstKey {
		t.Fatalf("second grant shares the first grant's key %q", firstKey)
	}

	addMana(t, e, 0, "RRR")
	perStaticCast(t, e, fs)
	var last *events.Event
	for i := range e.L.Events {
		if ev := &e.L.Events[i]; ev.Kind == events.CastInfo && ev.Obj == fs {
			last = ev
		}
	}
	if last == nil {
		t.Fatalf("no CastInfo for Flameskull")
	}
	if want := "mayplay,perm=" + secondKey; last.Counter != want {
		t.Fatalf("second recast counter = %q, want %q (charged to the wrong grant)", last.Counter, want)
	}
}

// TestObjIDWithoutRemovesEveryOccurrence pins objIDWithout's all-occurrence
// semantics and that it never rewrites the caller's backing array (the move
// sweep's activeBuf copies share it).
func TestObjIDWithoutRemovesEveryOccurrence(t *testing.T) {
	ids := []state.ObjID{6, 35, 6}
	got := objIDWithout(ids, 6)
	if !slices.Equal(got, []state.ObjID{35}) {
		t.Fatalf("objIDWithout(%v, 6) = %v, want [35]", ids, got)
	}
	if !slices.Equal(ids, []state.ObjID{6, 35, 6}) {
		t.Fatalf("input was mutated: %v", ids)
	}
	got = objIDWithout(ids, 99)
	if &got[0] != &ids[0] || len(got) != len(ids) {
		t.Fatalf("absent id must return the input slice unchanged: %v", got)
	}
}

// countObjID counts the occurrences of id in ids.
func countObjID(ids []state.ObjID, id state.ObjID) int {
	n := 0
	for _, x := range ids {
		if x == id {
			n++
		}
	}
	return n
}
