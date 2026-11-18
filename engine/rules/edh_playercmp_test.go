package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// edh_playercmp_test.go pins the PlayerCount board/life comparison family
// through real games (the printed text, cast and resolved): a seat's count
// is the number of opponents whose board or life compares against the
// resolving controller's at the moment the ability resolves (CR 608.2h), so
// moving a land or creature across the table moves the count the next
// resolution reads.

// edhPlayerCompareBear is the vanilla creature fixture every board in these
// tests is built from.
func edhPlayerCompareBear(t *testing.T) *cards.Card {
	return card(t, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
}

// edhPlayerCompareLands moves n Mountains from p's library onto its
// battlefield (the eventless zone moves the placement helpers use, with the
// pending decision re-asked so later casts read the new board).
func edhPlayerCompareLands(t *testing.T, e *Engine, p state.PlayerID, n int) {
	t.Helper()
	given := 0
	for _, id := range e.G.Zone(state.ZLibrary, p) {
		if given == n {
			break
		}
		o := e.G.Obj(id)
		if o != nil && o.Face() != nil && o.Face().Name == "Mountain" {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZBattlefield})
			given++
		}
	}
	if given != n {
		t.Fatalf("seat %d's library held only %d Mountains, want %d", p, given, n)
	}
	e.pending = nil
	e.Advance()
}

// edhPlayerCompareCount counts one seat's cards named name in a zone.
func edhPlayerCompareCount(e *Engine, p state.PlayerID, z state.Zone, name string) int {
	n := 0
	for _, id := range e.G.Zone(z, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			n++
		}
	}
	return n
}

// edhPlayerCompareCastSearch casts the named card in seat 0's hand paying
// mana, passes priority through resolution, and answers the "search your
// library for up to X" pick ask the ETB resolution poses with EXACTLY
// searchMax cards, asserting the ask's own Max is searchMax -- the count
// read lives in that bound, not only in the moved cards. The stack must be
// empty when the drain ends.
func edhPlayerCompareCastSearch(t *testing.T, e *Engine, name, mana string, searchMax int) {
	t.Helper()
	id := searchMoveByName(t, e, name, state.ZHand)
	addMana(t, e, 0, mana)
	d := e.Pending()
	if d == nil {
		t.Fatal("no decision pending")
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for %s", name)
	}
	submitChoices(t, e, idx)
	answered := false
	for n := 0; n < 80; n++ {
		d := e.Pending()
		if d == nil {
			if len(e.G.Stack) == 0 && len(e.pendingTriggers) == 0 {
				if !answered {
					t.Fatalf("%s's search ask never came", name)
				}
				return
			}
			e.priorityRound()
			continue
		}
		if d.Kind == decision.KChoose && d.ResumeKind == "search" && d.Player == 0 {
			if d.Max != searchMax {
				t.Fatalf("%s's search ask = Min %d Max %d, want Max %d", name, d.Min, d.Max, searchMax)
			}
			picks := make([]int, 0, searchMax)
			for _, o := range d.Options {
				if len(picks) == searchMax {
					break
				}
				picks = append(picks, o.Index)
			}
			if len(picks) != searchMax {
				t.Fatalf("search ask offered %d candidates, want at least %d", len(picks), searchMax)
			}
			submitChoices(t, e, picks...)
			answered = true
			continue
		}
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 && len(e.pendingTriggers) == 0 {
				if !answered {
					t.Fatalf("%s's search ask never came", name)
				}
				return
			}
			pass := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					pass = o.Index
				}
			}
			if pass < 0 {
				t.Fatalf("priority decision with no pass option")
			}
			submitChoices(t, e, pass)
			continue
		}
		t.Fatalf("unexpected decision %s/%s while draining %s's resolution", d.Kind, d.ResumeKind, name)
	}
	t.Fatalf("did not settle within 80 steps")
}

// TestEDHPlayerCompareOreskosExplorerCountsMoreLands plays Oreskos
// Explorer's ETB ("search your library for up to X Plains cards, where X is
// the number of players who control more lands than you") in a 4-seat game:
// two of the three opponents control more lands than seat 0, so the search
// takes exactly 2, and the count moves to 3 once a third opponent's land
// enters.
func TestEDHPlayerCompareOreskosExplorerCountsMoreLands(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	explorer := mustCorpusCard(t, reg, "Oreskos Explorer")
	// Precondition: the corpus card really carries the shape under test.
	if got := explorer.Faces[0].SVars["X"]; got != "PlayerCountPropertywithAtLeast1MoreLandsThanYou$Amount" {
		t.Fatalf("corpus Oreskos Explorer SVar X = %q, want the at-least-1-more-lands body", got)
	}
	plains := card(t, "Name:Plains\nTypes:Basic Land Plains\nOracle:x\n")
	deck0 := []*cards.Card{explorer, explorer}
	for i := 0; i < 40; i++ {
		deck0 = append(deck0, plains)
	}
	cfg := seatZeroStart(Config{Seed: 11, Names: []string{"a", "b", "c", "d"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{
			deck0,
			mountainDeck(t, 40),
			mountainDeck(t, 40),
			mountainDeck(t, 40),
		}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	// Seats 1 (two lands) and 2 (one land) out-land seat 0's empty board;
	// seat 3 has none yet.
	edhPlayerCompareLands(t, e, 1, 2)
	edhPlayerCompareLands(t, e, 2, 1)

	before := edhPlayerCompareCount(e, 0, state.ZHand, "Plains")
	edhPlayerCompareCastSearch(t, e, "Oreskos Explorer", "CCW", 2)
	if got := edhPlayerCompareCount(e, 0, state.ZHand, "Plains") - before; got != 2 {
		t.Fatalf("Explorer searched %d Plains, want 2 (seats 1 and 2 control more lands than seat 0)", got)
	}
	// The count is the live comparison, not a snapshot: seat 3 gaining a
	// land makes a third player qualify for the next resolution.
	edhPlayerCompareLands(t, e, 3, 1)
	before = edhPlayerCompareCount(e, 0, state.ZHand, "Plains")
	edhPlayerCompareCastSearch(t, e, "Oreskos Explorer", "CCW", 3)
	if got := edhPlayerCompareCount(e, 0, state.ZHand, "Plains") - before; got != 3 {
		t.Fatalf("second Explorer searched %d Plains, want 3 (seats 1, 2 and 3 now control more lands)", got)
	}
}

// TestEDHPlayerCompareFewerCreaturesCounted plays Voice of Many's ETB
// ("draw a card for each opponent who controls fewer creatures than you")
// in a 4-seat game — the head Birth of the Imperium's chapter III reads
// with a /Times.2 suffix (its corpus body is pinned below, so the pin dies
// loudly if the corpus shape moves). Only seat 2 controls fewer creatures
// in play than seat 0's one (the entering Voice itself counts for its
// controller), so the first draw is exactly 1, and it falls to 0 once three
// creatures enter for seat 2 and lift it past the second resolution's two.
func TestEDHPlayerCompareFewerCreaturesCounted(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	voice := mustCorpusCard(t, reg, "Voice of Many")
	if got := voice.Faces[0].SVars["X"]; got != "PlayerCountOpponents$HasPropertyhasFewerCreaturesInPlayThanYou" {
		t.Fatalf("corpus Voice of Many SVar X = %q, want the fewer-creatures-in-play body", got)
	}
	birth := mustCorpusCard(t, reg, "Birth of the Imperium")
	if got := birth.Faces[0].SVars["Y"]; got != "PlayerCountOpponents$HasPropertyhasFewerCreaturesInPlayThanYou/Times.2" {
		t.Fatalf("corpus Birth of the Imperium SVar Y = %q, want the /Times.2 sibling", got)
	}
	bear := edhPlayerCompareBear(t)
	// A full-length library for seat 0 too: the deck's 40th+ card is filler;
	// the two Voices and the Bear reach the opening hand either drawn or via
	// searchMoveByName's hand-then-library move.
	deck0 := []*cards.Card{voice, voice, bear}
	deck0 = append(deck0, mountainDeck(t, 37)...)
	cfg := seatZeroStart(Config{Seed: 13, Names: []string{"a", "b", "c", "d"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{
			deck0,
			mountainDeck(t, 40),
			mountainDeck(t, 40),
			mountainDeck(t, 40),
		}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	// Seat 0's "creatures in play" at a resolution is its Voices — the
	// entering Voice itself is on the battlefield when its own ETB trigger
	// resolves. Seat 1 holds four creatures, seat 3 three, seat 2 none.
	for i := 0; i < 4; i++ {
		onBoardCard(t, e, 1, bear)
	}
	for i := 0; i < 3; i++ {
		onBoardCard(t, e, 3, bear)
	}

	// First resolution: you control one (the Voice itself), so only seat 2
	// (zero) sits below.
	before := len(e.G.Zone(state.ZHand, 0))
	castVanilla(t, e, "Voice of Many", "CCGG")
	if got := len(e.G.Zone(state.ZHand, 0)) - before; got != 1 {
		t.Fatalf("Voice of Many drew %d cards, want 1 (only seat 2 controls fewer creatures than your one)", got)
	}
	// The comparison is live: three creatures entering for seat 2 lift its
	// board past your second resolution's two, so nobody sits below and the
	// draw is zero.
	for i := 0; i < 3; i++ {
		onBoardCard(t, e, 2, bear)
	}
	before = len(e.G.Zone(state.ZHand, 0))
	castVanilla(t, e, "Voice of Many", "CCGG")
	if got := len(e.G.Zone(state.ZHand, 0)) - before; got != 0 {
		t.Fatalf("second Voice of Many drew %d cards, want 0 (no opponent controls fewer than your two)", got)
	}
}
