package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// chapterPushed reports whether the log holds a DelayedPush of chapter SVar
// name for object id.
func chapterPushed(e *Engine, id state.ObjID, name string) bool {
	for _, ev := range e.L.Events {
		if ev.Kind == events.DelayedPush && ev.Obj == id && ev.Counter == name {
			return true
		}
	}
	return false
}

// TestEchoingDeepsCopiesUrzasSagaWithEntryLore: a permanent that enters as a
// copy of a Saga enters with the Saga's lore counter and reaches chapter I
// (CR 707.2, CR 702.151a), though the copy basis is installed only after the
// entry move folds.
func TestEchoingDeepsCopiesUrzasSagaWithEntryLore(t *testing.T) {
	t.Parallel()
	t.Run("echoing-deeps", func(t *testing.T) {
		raw := `{"name":"echoing-deeps-saga","cr":["707.2","702.151a"],"why":"copied saga lore","setup":{` +
			`"p0":{"hand":["Echoing Deeps"]},"p1":{"graveyard":["Urza's Saga"]}},` +
			`"steps":[{"op":"play","seat":0,"card":"p0:Echoing Deeps","answers":[` +
			`{"kind":"choose","pick":["p1:Urza's Saga"]}]}],"expect":[]}`
		e := runETBCloneScenario(t, raw).e
		id := findOn(t, e, state.ZBattlefield, 0, "Urza's Saga")
		der := e.Derived(id)
		for _, ty := range []string{"Saga", "Cave"} {
			if !slices.Contains(der.Types, ty) {
				t.Fatalf("copy lacks type %q: %v", ty, der.Types)
			}
		}
		n, names := e.sagaChapters(e.G.Obj(id))
		if n != 3 || !slices.Contains(names, "Animate1") {
			t.Fatalf("copy basis not installed: chapters %d %v", n, names)
		}
		if got := e.G.Obj(id).Counter("LORE"); got != 1 {
			t.Fatalf("LORE = %d, want 1", got)
		}
		if !chapterPushed(e, id, "Animate1") {
			t.Fatal("chapter I (Animate1) never reached the stack")
		}
	})
	t.Run("clever-impersonator", func(t *testing.T) {
		raw := `{"name":"clever-impersonator-saga","cr":["707.2","702.151a"],"why":"copied saga lore","setup":{` +
			`"p0":{"hand":["Clever Impersonator"]},"p1":{"battlefield":["The Eldest Reborn"]}},` +
			`"steps":[{"op":"cast","seat":0,"card":"p0:Clever Impersonator","mana":"CCUU"},` +
			`{"op":"resolve","answers":[{"kind":"choose","pick":["p1:The Eldest Reborn"]}]}],"expect":[]}`
		e := runETBCloneScenario(t, raw).e
		orig := findOn(t, e, state.ZBattlefield, 1, "The Eldest Reborn")
		if got := e.G.Obj(orig).Counter("LORE"); got != 1 {
			t.Fatalf("original LORE = %d, want 1", got)
		}
		cp := findOn(t, e, state.ZBattlefield, 0, "The Eldest Reborn")
		if n, names := e.sagaChapters(e.G.Obj(cp)); n != 3 || len(names) != 3 {
			t.Fatalf("copy chapters %d %v", n, names)
		}
		if got := e.G.Obj(cp).Counter("LORE"); got != 1 {
			t.Fatalf("copy LORE = %d, want 1", got)
		}
		if !chapterPushed(e, cp, "DBSacrifice") {
			t.Fatal("chapter I (DBSacrifice) never reached the stack")
		}
	})
}
