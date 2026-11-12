package rules

// The staged TwoPiles pins, driven through the real Fact or Fiction,
// Split the Spoils and Brilliant Ultimatum: the separator's split, the
// chooser's pick, and where each pile goes.

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

var pileNames = []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon"}

// pileBoard is a 2-seat game whose seat 0 holds the corpus card spell and
// whose library's top five are Alpha..Epsilon, in order (returned).
func pileBoard(t *testing.T, seed uint64, spell string) (*Engine, Config, []state.ObjID) {
	t.Helper()
	deck := []*cards.Card{corpusAlternativeCard(t, spell)}
	for _, n := range pileNames {
		deck = append(deck, card(t, creatureLineSrc(n)))
	}
	e, cfg := fixtureGame(t, seed, deck, nil)
	moveNamedTo(t, e, 0, spell, state.ZHand)
	var ids []state.ObjID
	for _, n := range pileNames {
		ids = append(ids, moveNamedTo(t, e, 0, n, state.ZLibrary))
	}
	libraryTopOrder(t, e, 0, ids...)
	return e, cfg, ids
}

// factOrFictionSplit casts Fact or Fiction and returns its posed split ask.
func factOrFictionSplit(t *testing.T, seed uint64) (*Engine, Config, []state.ObjID, *decision.Decision) {
	t.Helper()
	e, cfg, ids := pileBoard(t, seed, "Fact or Fiction")
	d := castNamedSpell(t, e, "Fact or Fiction", "UUUU", -1)
	return e, cfg, ids, d
}

func optionIndexes(t *testing.T, d *decision.Decision, ids ...state.ObjID) []int {
	t.Helper()
	out := []int{}
	for _, id := range ids {
		out = append(out, objOptionIndex(t, d, id))
	}
	return out
}

// TestTwoPilesSplitAskPosesToTheSeparatorAndSuspends: the separator (the
// opponent) is posed a KChoose over the five revealed cards, Min 0 (piles
// can be empty) Max 5, in reveal order, before anything moves.
func TestTwoPilesSplitAskPosesToTheSeparatorAndSuspends(t *testing.T) {
	t.Parallel()
	e, _, ids, d := factOrFictionSplit(t, 231)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "twopiles_split" || d.Player != 1 {
		t.Fatalf("split ask = %+v, want the separator's (seat 1) twopiles_split KChoose", d)
	}
	if d.Min != 0 || d.Max != 5 {
		t.Fatalf("bounds = %d..%d, want 0..5", d.Min, d.Max)
	}
	if got := optionObjIDs(d); !slices.Equal(got, ids) {
		t.Fatalf("options = %v, want the revealed cards in order %v", got, ids)
	}
	for _, id := range ids {
		if zoneOfObjID(e, id) != state.ZLibrary {
			t.Fatalf("card %d moved before the split was answered", id)
		}
	}
}

// TestTwoPilesAnsweredSplitAsksThePick: the split answer poses the chooser's
// (Defined$ You) two-option pile pick, not a second split, with nothing moved.
func TestTwoPilesAnsweredSplitAsksThePick(t *testing.T) {
	t.Parallel()
	e, _, ids, d := factOrFictionSplit(t, 232)
	mark := len(e.L.Events)
	d = answerChoices(t, e, optionIndexes(t, d, ids[0], ids[2])...)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "twopiles_pick" || d.Player != 0 {
		t.Fatalf("pick ask = %+v, want seat 0's twopiles_pick", d)
	}
	if d.Min != 1 || d.Max != 1 || len(d.Options) != 2 || d.Options[0].Kind != "pile-a" || d.Options[1].Kind != "pile-b" {
		t.Fatalf("pick options = %+v, want pile-a/pile-b at Min=Max=1", d.Options)
	}
	if n := countKindFrom(e, mark, events.MoveZone); n != 0 {
		t.Fatalf("%d card(s) moved before the pick was answered", n)
	}
}

// pilesAfterAnswer answers FoF's split with pileA and the pick with kind, and
// checks that pileA's cards went to want and every other card to the other
// of hand/graveyard.
func pilesAfterAnswer(t *testing.T, seed uint64, pileA []int, kind string, aZone, bZone state.Zone) {
	t.Helper()
	e, cfg, ids, d := factOrFictionSplit(t, seed)
	var a []state.ObjID
	for _, i := range pileA {
		a = append(a, ids[i])
	}
	d = answerChoices(t, e, optionIndexes(t, d, a...)...)
	if d == nil || d.ResumeKind != "twopiles_pick" {
		t.Fatalf("pick ask = %+v", d)
	}
	if d := answerChoices(t, e, optionKindOrFail(t, d, kind)); d != nil {
		t.Fatalf("a fully answered TwoPiles posed %+v", d)
	}
	for _, id := range ids {
		want := bZone
		if slices.Contains(a, id) {
			want = aZone
		}
		if got := zoneOfObjID(e, id); got != want {
			t.Fatalf("card %d zone = %v, want %v", id, got, want)
		}
	}
	replayCheck(t, e, cfg)
}

// TestTwoPilesAnsweredPickMovesThePiles: pile A chosen -> its cards to hand,
// pile B to the graveyard.
func TestTwoPilesAnsweredPickMovesThePiles(t *testing.T) {
	t.Parallel()
	pilesAfterAnswer(t, 233, []int{0, 2}, "pile-a", state.ZHand, state.ZGraveyard)
}

// TestTwoPilesPickBChoosesTheSecondPile: pile B chosen -> B to hand, A to
// the graveyard.
func TestTwoPilesPickBChoosesTheSecondPile(t *testing.T) {
	t.Parallel()
	pilesAfterAnswer(t, 234, []int{0, 2}, "pile-b", state.ZGraveyard, state.ZHand)
}

// TestTwoPilesEmptyPileAnswerMovesEverythingToUnchosen: an empty split (Min
// 0 makes it legal) with the empty pile A chosen sends all five cards to
// the graveyard.
func TestTwoPilesEmptyPileAnswerMovesEverythingToUnchosen(t *testing.T) {
	t.Parallel()
	pilesAfterAnswer(t, 235, nil, "pile-a", state.ZHand, state.ZGraveyard)
}

// TestTwoPilesSplitTheSpoilsTargetedChooserOpponent: the real Split the
// Spoils reads its targets (DefinedCards$ Targeted) and swaps the roles --
// the caster (Separator$ You) splits the three exiled targets, the opponent
// (Chooser$ Opponent) picks, and the chosen pile goes from exile to hand,
// the other to the graveyard.
func TestTwoPilesSplitTheSpoilsTargetedChooserOpponent(t *testing.T) {
	t.Parallel()
	e, cfg, ids := pileBoard(t, 236, "Split the Spoils")
	gy := ids[:3]
	for _, id := range gy {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZGraveyard})
	}
	e.pending = nil
	addMana(t, e, 0, "GGG")
	submitChoices(t, e, castOptionFor(t, e, findNamedInZone(e, 0, state.ZHand, "Split the Spoils")).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("decision = %+v, want the up-to-five graveyard target ask", d)
	}
	d = answerChoices(t, e, optionIndexes(t, d, gy...)...)
	if d == nil || d.ResumeKind != "twopiles_split" || d.Player != 0 || len(d.Options) != 3 {
		t.Fatalf("split ask = %+v, want the caster's split over the three exiled targets", d)
	}
	d = answerChoices(t, e, objOptionIndex(t, d, gy[1]))
	if d == nil || d.ResumeKind != "twopiles_pick" || d.Player != 1 {
		t.Fatalf("pick ask = %+v, want Chooser$ Opponent (seat 1)", d)
	}
	if d := answerChoices(t, e, optionKindOrFail(t, d, "pile-a")); d != nil {
		t.Fatalf("unexpected follow-up %+v", d)
	}
	if z := zoneOfObjID(e, gy[1]); z != state.ZHand {
		t.Fatalf("chosen pile card zone = %v, want hand", z)
	}
	for _, id := range []state.ObjID{gy[0], gy[2]} {
		if z := zoneOfObjID(e, id); z != state.ZGraveyard {
			t.Fatalf("unchosen pile card %d zone = %v, want graveyard", id, z)
		}
	}
	replayCheck(t, e, cfg)
}

// TestTwoPilesPlayBodyHandsThePileToPlay: Brilliant Ultimatum's ChosenPile$
// is a DB$ Play body; the chosen pile is handed to Play, whose play ask is
// posed over exactly that pile's cards.
func TestTwoPilesPlayBodyHandsThePileToPlay(t *testing.T) {
	t.Parallel()
	e, _, ids := pileBoard(t, 237, "Brilliant Ultimatum")
	d := castNamedSpell(t, e, "Brilliant Ultimatum", "WWUUUBB", -1)
	if d == nil || d.ResumeKind != "twopiles_split" || d.Player != 1 {
		t.Fatalf("split ask = %+v, want the opponent's split", d)
	}
	d = answerChoices(t, e, optionIndexes(t, d, ids[0], ids[1])...)
	if d == nil || d.ResumeKind != "twopiles_pick" || d.Player != 0 {
		t.Fatalf("pick ask = %+v, want the caster's pick", d)
	}
	d = answerChoices(t, e, optionKindOrFail(t, d, "pile-a"))
	if d == nil || d.Kind != decision.KModes || d.ResumeKind != "play" {
		t.Fatalf("decision = %+v, want the play KModes over the chosen pile", d)
	}
	played := map[state.ObjID]bool{}
	for _, o := range d.Options {
		if o.Obj != 0 {
			played[o.Obj] = true
		}
	}
	if len(played) != 2 || !played[ids[0]] || !played[ids[1]] {
		t.Fatalf("play options = %+v, want exactly the chosen pile's two cards", d.Options)
	}
}
