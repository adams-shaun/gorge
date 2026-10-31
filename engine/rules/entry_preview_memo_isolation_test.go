package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEntryPreviewDoesNotWriteTheLiveStaticMemo pins the entry preview's
// memo isolation (CR 611.3a: a static ability stops applying once its source
// leaves the battlefield). The census shape (dirac K1l pairs 21961
// 37823 66869 74406): Stroke of Midnight destroys Empyrean Eagle ("other
// creatures you control with flying get +1/+1") and creates a Human token in
// the same resolution, with no read in between, and Giada, Font of Hope's
// entry grant makes the token's entry build an entry preview -- a shallow
// copy of the live engine. Before the fix the preview's static-memo rebuild
// wrote into the live engine's own array (zeroing the Eagle's entry under an
// unchanged key), the live refresh then judged the Eagle's departure cold
// and re-stamped the memo, and the live active list kept the Eagle's +1/+1:
// Serra Angel stayed 5/5 while a clone read 4/4. The test folds the
// departure without an engine read (log append + Apply, as one emit inside a
// resolution leaves it), builds the token entry's preview, reads through it,
// and checks the live memo and the live Derived.
func TestEntryPreviewDoesNotWriteTheLiveStaticMemo(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	e.G.Tokens = testutil.CorpusRegistry(t).Tokens
	stashBoard(t, e, 0, corpusAlternativeCard(t, "Giada, Font of Hope"))
	eagle := stashBoard(t, e, 1, corpusAlternativeCard(t, "Empyrean Eagle"))
	angel := stashBoard(t, e, 1, corpusAlternativeCard(t, "Serra Angel"))
	if got := e.Derived(angel).Power; got != 5 {
		t.Fatalf("precondition: Serra Angel power %d with the Eagle, want 5", got)
	}
	// The destroy, folded with no read after it.
	mv := e.L.Append(events.Event{Kind: events.MoveZone, Obj: eagle, From: state.ZBattlefield, To: state.ZGraveyard, Text: "destroyed"})
	events.Apply(e.G, mv)
	before := append([]ContinuousEffect(nil), e.staticContinuous...)

	tok := events.Event{Kind: events.TokenCreate, Player: 1, Text: "w_1_1_human"}
	preview, _ := e.entryPreview(tok)
	_ = preview.Derived(angel) // the preview's own reads rebuild its memos
	e.releaseEntryPreview(preview)
	if len(e.staticContinuous) != len(before) || (len(before) > 0 && e.staticContinuous[0].Source != before[0].Source) {
		t.Fatalf("the entry preview wrote the live static memo: %d entries, first source %d (was %d)",
			len(e.staticContinuous), e.staticContinuous[0].Source, before[0].Source)
	}
	live, clone := e.Derived(angel).Power, e.Clone().Derived(angel).Power
	if live != 4 || clone != 4 {
		t.Fatalf("Serra Angel power live %d clone %d after the Eagle left, want 4/4", live, clone)
	}
}
