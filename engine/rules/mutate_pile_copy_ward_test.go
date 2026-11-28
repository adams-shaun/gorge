package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMutatedPileThatBecomesACopyDropsTheUnderWardTrigger: ward is a keyword
// ability (CR 702.21a), so a copy effect newer than the merge takes the under
// card's ward trigger with its keyword line (CR 707.2, 613.2a, 730.2a): only
// the copied face's own ward and a granted one trigger. When the copy ends the
// under card's ward triggers again.
func TestMutatedPileThatBecomesACopyDropsTheUnderWardTrigger(t *testing.T) {
	t.Parallel()
	const warden = "Name:Copied Warden\nManaCost:2 W\nTypes:Creature Soldier\nPT:2/4\nK:Ward:1\nOracle:x\n"
	cases := []struct {
		name, copied  string
		grant         bool
		want, wantEnd int // triggers while copied, and after the copy ends
	}{
		{"no ward left", copiedPileWurmSrc, false, 0, 1},
		{"the copied face's ward", warden, false, 1, 1},
		// The grant equals the under card's line: one instance while copied
		// (the grant), two once the under card's line is back (CR 113.2c).
		{"a granted ward", copiedPileWurmSrc, true, 1, 2},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, _, pile := mutatedKeywordPile(t, 481+uint64(i), "K:Ward:2\n")
			src := onBoard(t, e, 0, tc.copied)
			if tc.grant {
				onBoard(t, e, 0, mutateGrantLordSrc("Ward:2"))
			}
			target := func(libIdx, want int, when string) {
				t.Helper()
				e.pendingTriggers = nil
				cause := e.G.Zone(state.ZLibrary, 1)[libIdx]
				e.emit(events.Event{Kind: events.PutOnStack, Obj: cause, Player: 1, From: state.ZLibrary, To: state.ZStack})
				from := len(e.L.Events)
				e.emit(events.Event{Kind: events.TargetsChosen, Obj: cause, IDs: []state.ObjID{pile}})
				if got := queuedSince(e, pile, from); got != want {
					t.Fatalf("%s: targeting the pile queued %d ward triggers, want %d (derived %q)",
						when, got, want, e.Derived(pile).Keywords)
				}
			}
			becomeCopyOf(t, e, pile, src)
			if n := len(e.G.Obj(pile).MergedCards); n != 1 {
				t.Fatalf("precondition: the copy effect left %d under cards, want 1", n)
			}
			target(0, tc.want, "copied")

			e.emit(events.Event{Kind: events.ClonePermanent, Obj: pile, Player: 0})
			if o := e.G.Obj(pile); o.CopyFace != nil || o.Face().Name != "Keyword Crown" {
				t.Fatalf("precondition: the copy effect did not end: %+v", o)
			}
			target(0, tc.wantEnd, "copy ended")
		})
	}
}

// TestMergeUnderACopyKeepsTheUnderWardTrigger is the control for the order: a
// merge newer than the copy effect gives the pile the under card's ward,
// though it still shows the copied face (CR 730.2a, 613.2a).
func TestMergeUnderACopyKeepsTheUnderWardTrigger(t *testing.T) {
	t.Parallel()
	const under = "Name:Keyword Burrower\nManaCost:2 G\nTypes:Creature Beast\nPT:3/3\n" +
		"K:Mutate:1 G\nK:Ward:2\nOracle:x\n"
	e, cfg := tokenReplGame(t, 484, card(t, under), card(t, copiedPileWurmSrc), card(t, mutateBearSrc))
	mut := moveSeededCard(t, e, 0, card(t, under), state.ZHand)
	pile := moveSeeded(t, e, 0, mutateBearSrc, state.ZBattlefield)
	wurm := moveSeeded(t, e, 0, copiedPileWurmSrc, state.ZBattlefield)
	becomeCopyOf(t, e, pile, wurm)
	addMana(t, e, 0, "GG")
	mutateCastOnto(t, e, mutatedCastOption(t, e, mut), pile, false)
	mutateDrain(t, e, 40)
	if o := e.G.Obj(pile); o.CopyFace == nil || o.Face().Name != "Copied Wurm" || len(o.MergedCards) != 1 {
		t.Fatalf("precondition: want the Burrower under the Wurm copy, got %+v", o)
	}
	replayCheck(t, e, cfg)
	cause := e.G.Zone(state.ZLibrary, 1)[0]
	e.emit(events.Event{Kind: events.PutOnStack, Obj: cause, Player: 1, From: state.ZLibrary, To: state.ZStack})
	from := len(e.L.Events)
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: cause, IDs: []state.ObjID{pile}})
	if got := queuedSince(e, pile, from); got != 1 {
		t.Fatalf("targeting the pile queued %d ward triggers, want the under card's 1", got)
	}
}
