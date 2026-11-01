package oracle

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

func shuffleCtx(player state.PlayerID, ordinal, n int) rules.ShuffleContext {
	ctx := rules.ShuffleContext{Player: player, Ordinal: ordinal}
	for i := 1; i <= n; i++ {
		ctx.Library = append(ctx.Library, rules.ShuffleCard{ID: state.ObjID(i)})
	}
	return ctx
}

// TestChanceShufflePlanner pins the transcript-to-order translation: runs of
// descending bounds ending at 2 are shuffles, handed out in order one per
// (player, ordinal); the toss and a stray draw are skipped; a repeat (a
// probe's clone) gets the same order; a wrong-sized request gets nil and
// moves nothing.
func TestChanceShufflePlanner(t *testing.T) {
	tr := []ChanceDraw{
		{2, 0},                         // the toss
		{5, 2}, {4, 0}, {3, 1}, {2, 1}, // a 5-card shuffle
		{7, 3},         // a random discard
		{3, 1}, {2, 0}, // a 3-card shuffle
	}
	plan := ChanceShufflePlanner(tr)
	call := func(player state.PlayerID, ordinal, n int) []state.ObjID {
		t.Helper()
		got, err := plan(shuffleCtx(player, ordinal, n))
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	want5 := []state.ObjID{4, 5, 2, 1, 3}
	if got := call(0, 0, 5); !reflect.DeepEqual(got, want5) {
		t.Fatalf("first shuffle = %v, want %v", got, want5)
	}
	if got := call(0, 0, 5); !reflect.DeepEqual(got, want5) {
		t.Fatalf("repeat of the same shuffle = %v, want %v", got, want5)
	}
	if got := call(1, 0, 4); got != nil {
		t.Fatalf("a 4-card request against the next recorded 3-card shuffle = %v, want nil", got)
	}
	if got, want := call(1, 0, 3), []state.ObjID{3, 1, 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("second shuffle = %v, want %v", got, want)
	}
	if got := call(0, 1, 5); got != nil {
		t.Fatalf("a shuffle past the record = %v, want nil", got)
	}
	if got, want := call(1, 1, 1), []state.ObjID{1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("a 1-card shuffle = %v, want %v (it draws nothing)", got, want)
	}
}
