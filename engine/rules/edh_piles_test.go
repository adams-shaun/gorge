package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// edhPilesBears are the vanilla creatures every EDH-piles board is built from.
var edhPilesBears = []string{"Alpha", "Beta", "Gamma", "Delta"}

func edhPilesBearDeck(t *testing.T) []*cards.Card {
	t.Helper()
	srcs := make([]string, len(edhPilesBears))
	for i, n := range edhPilesBears {
		srcs[i] = creatureLineSrc(n)
	}
	return cardsOfSrcs(t, srcs...)
}

// edhPilesLibraryTop puts seat p's named cards on top of its library in the
// given order (a card the opening hand drew goes back first).
func edhPilesLibraryTop(t *testing.T, e *Engine, p state.PlayerID, names ...string) []state.ObjID {
	t.Helper()
	ids := make([]state.ObjID, 0, len(names))
	for _, n := range names {
		id := findNamedInZone(e, p, state.ZLibrary, n)
		if id == 0 {
			id = findNamedInZone(e, p, state.ZHand, n)
			if id == 0 {
				t.Fatalf("seat %d has no %q in library or hand", p, n)
			}
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
		}
		ids = append(ids, id)
	}
	libraryTopOrder(t, e, p, ids...)
	return ids
}

func edhPilesZone(e *Engine, id state.ObjID) state.Zone { return e.G.Obj(id).Zone }

// TestEDHPilesMakeAnExampleFourSeats plays Make an Example at four seats:
// each opponent separates their creatures (a per-opponent split ask to that
// opponent), the caster picks a pile per opponent, and exactly the chosen
// piles are sacrificed. Seat 1 splits 1/1 and the caster takes pile A; seat 2
// puts both creatures in pile A and the caster takes the (empty) pile B;
// seat 3 puts nothing in pile A, so pile B is both creatures.
func TestEDHPilesMakeAnExampleFourSeats(t *testing.T) {
	t.Parallel()
	bears := edhPilesBearDeck(t)[:2]
	e, cfg := fixtureGame(t, 7001,
		[]*cards.Card{corpusAlternativeCard(t, "Make an Example")}, bears, bears, bears)
	board := map[state.PlayerID][]state.ObjID{}
	for p := state.PlayerID(1); p <= 3; p++ {
		for _, n := range []string{"Alpha", "Beta"} {
			board[p] = append(board[p], moveNamedTo(t, e, p, n, state.ZBattlefield))
		}
	}
	moveNamedTo(t, e, 0, "Make an Example", state.ZHand)
	d := castNamedSpell(t, e, "Make an Example", "RRRRB", -1)

	// answers[p] are the split picks for seat p (option indexes), pick[p] the
	// caster's pile choice for that opponent.
	splits := map[state.PlayerID][]int{1: {0}, 2: {0, 1}, 3: {}}
	pick := map[state.PlayerID]string{1: "pile-a", 2: "pile-b", 3: "pile-b"}
	for p := state.PlayerID(1); p <= 3; p++ {
		if d == nil || d.ResumeKind != "twopiles_split" || d.Player != p {
			t.Fatalf("seat %d split ask: pending %+v", p, d)
		}
		if len(d.Options) != 2 || d.Options[0].Obj != board[p][0] || d.Options[1].Obj != board[p][1] {
			t.Fatalf("seat %d split offered %+v, want its two creatures", p, d.Options)
		}
		submitChoices(t, e, splits[p]...)
		d = e.Pending()
		if d == nil || d.ResumeKind != "twopiles_pick" || d.Player != 0 {
			t.Fatalf("seat %d pile pick: pending %+v, want the caster", p, d)
		}
		idx := d.Options[0].Index
		if pick[p] == "pile-b" {
			idx = d.Options[1].Index
		}
		submitChoices(t, e, idx)
		d = nextPending(t, e)
	}
	if d != nil {
		t.Fatalf("unexpected pending decision after the last pile pick: %+v", d)
	}
	want := map[state.ObjID]state.Zone{
		board[1][0]: state.ZGraveyard, board[1][1]: state.ZBattlefield,
		board[2][0]: state.ZBattlefield, board[2][1]: state.ZBattlefield,
		board[3][0]: state.ZGraveyard, board[3][1]: state.ZGraveyard,
	}
	for id, z := range want {
		if got := edhPilesZone(e, id); got != z {
			t.Errorf("creature %d zone = %s, want %s", id, got, z)
		}
	}
	replayCheck(t, e, cfg)
}

// TestEDHPilesSauronsRansomPutsOnePileInHandAndOneInGraveyard casts Sauron's
// Ransom: the opponent splits the top four cards of the caster's library, the
// caster picks a pile, the chosen pile goes to hand and the other to the
// graveyard.
func TestEDHPilesSauronsRansomPutsOnePileInHandAndOneInGraveyard(t *testing.T) {
	t.Parallel()
	deck0 := append([]*cards.Card{corpusAlternativeCard(t, "Sauron's Ransom")}, edhPilesBearDeck(t)...)
	e, cfg := fixtureGame(t, 7002, deck0, nil)
	moveNamedTo(t, e, 0, "Sauron's Ransom", state.ZHand)
	top := edhPilesLibraryTop(t, e, 0, edhPilesBears...)
	d := castNamedSpell(t, e, "Sauron's Ransom", "1UB", -1)
	if d == nil || d.ResumeKind != "twopiles_split" || d.Player != 1 {
		t.Fatalf("split ask: pending %+v, want the opponent's", d)
	}
	if len(d.Options) != 4 {
		t.Fatalf("split offered %d cards, want the top four", len(d.Options))
	}
	for i, o := range d.Options {
		if o.Obj != top[i] {
			t.Fatalf("split option %d = %d, want top card %d", i, o.Obj, top[i])
		}
	}
	submitChoices(t, e, 0, 2) // pile A = Alpha, Gamma
	d = e.Pending()
	if d == nil || d.ResumeKind != "twopiles_pick" || d.Player != 0 {
		t.Fatalf("pile pick: pending %+v, want the caster's", d)
	}
	submitChoices(t, e, d.Options[1].Index) // pile B = Beta, Delta
	nextPending(t, e)
	for i, z := range []state.Zone{state.ZGraveyard, state.ZHand, state.ZGraveyard, state.ZHand} {
		if got := edhPilesZone(e, top[i]); got != z {
			t.Errorf("%s zone = %s, want %s", edhPilesBears[i], got, z)
		}
	}
	replayCheck(t, e, cfg)
}

// TestEDHPilesWhimsOfTheFatesSacrificesOneRandomPilePerPlayer casts Whims of
// the Fates over seeds: each of two players separates three creatures into
// piles of 2/1/0, and each loses exactly one of those piles (never a mix),
// the draw coming from the seeded rng. Across the seeds more than one pile is
// drawn, so the test cannot pass on a Whims that moves nothing.
func TestEDHPilesWhimsOfTheFatesSacrificesOneRandomPilePerPlayer(t *testing.T) {
	t.Parallel()
	lost := map[int]bool{}
	for seed := uint64(7100); seed < 7112; seed++ {
		deck := edhPilesBearDeck(t)[:3]
		d0 := append([]*cards.Card{corpusAlternativeCard(t, "Whims of the Fates")}, deck...)
		e, cfg := fixtureGame(t, seed, d0, deck)
		board := map[state.PlayerID][]state.ObjID{}
		for p := state.PlayerID(0); p <= 1; p++ {
			for _, n := range edhPilesBears[:3] {
				board[p] = append(board[p], moveNamedTo(t, e, p, n, state.ZBattlefield))
			}
		}
		moveNamedTo(t, e, 0, "Whims of the Fates", state.ZHand)
		d := castNamedSpell(t, e, "Whims of the Fates", "RRRRRR", -1)
		for p := state.PlayerID(0); p <= 1; p++ {
			if d == nil || d.ResumeKind != "multiplepiles_split" || d.Player != p || len(d.Options) != 3 {
				t.Fatalf("seed %d seat %d first split ask: %+v", seed, p, d)
			}
			submitChoices(t, e, 0, 1) // pile 1 = Alpha, Beta
			d = e.Pending()
			if d == nil || d.ResumeKind != "multiplepiles_split" || d.Player != p || len(d.Options) != 1 {
				t.Fatalf("seed %d seat %d second split ask: %+v", seed, p, d)
			}
			submitChoices(t, e, 0) // pile 2 = Gamma, pile 3 empty
			d = nextPending(t, e)
		}
		for p := state.PlayerID(0); p <= 1; p++ {
			gone := 0
			for i, id := range board[p] {
				if edhPilesZone(e, id) != state.ZBattlefield {
					gone |= 1 << i
				}
			}
			switch gone {
			case 0b011, 0b100, 0:
				lost[gone] = true
			default:
				t.Fatalf("seed %d seat %d lost creature mask %03b, want one whole pile (011, 100 or none)", seed, p, gone)
			}
		}
		if seed == 7100 {
			replayCheck(t, e, cfg)
		}
	}
	if len(lost) < 2 {
		t.Fatalf("over 12 seeds only %v piles were ever sacrificed", lost)
	}
}

// TestEDHPilesTruthOrTaleOpponentPicksThePile casts Truth or Tale: the caster
// separates the five revealed cards (DefinedCards$ Imprinted), the opponent
// picks the pile, the caster takes a card of that pile into hand and the other
// four go to the bottom of the library.
func TestEDHPilesTruthOrTaleOpponentPicksThePile(t *testing.T) {
	t.Parallel()
	bears := edhPilesBearDeck(t)
	deck0 := append([]*cards.Card{corpusAlternativeCard(t, "Truth or Tale"),
		card(t, creatureLineSrc("Epsilon"))}, bears...)
	e, cfg := fixtureGame(t, 7003, deck0, nil)
	moveNamedTo(t, e, 0, "Truth or Tale", state.ZHand)
	top := edhPilesLibraryTop(t, e, 0, "Alpha", "Beta", "Gamma", "Delta", "Epsilon")
	d := castNamedSpell(t, e, "Truth or Tale", "1U", -1)
	if d == nil || d.ResumeKind != "twopiles_split" || d.Player != 0 || len(d.Options) != 5 {
		t.Fatalf("split ask: pending %+v, want the caster's over five cards", d)
	}
	submitChoices(t, e, 0, 1) // pile A = Alpha, Beta
	d = e.Pending()
	if d == nil || d.ResumeKind != "twopiles_pick" || d.Player != 1 {
		t.Fatalf("pile pick: pending %+v, want the opponent's", d)
	}
	submitChoices(t, e, d.Options[0].Index) // the opponent picks pile A
	d = nextPending(t, e)
	if d == nil || len(d.Options) != 2 || d.Options[0].Obj != top[0] || d.Options[1].Obj != top[1] {
		t.Fatalf("card pick: pending %+v, want the two cards of the chosen pile", d)
	}
	submitChoices(t, e, d.Options[1].Index) // Beta to hand
	nextPending(t, e)
	if z := edhPilesZone(e, top[1]); z != state.ZHand {
		t.Fatalf("Beta zone = %s, want hand", z)
	}
	lib := e.G.Zone(state.ZLibrary, 0)
	for _, id := range []state.ObjID{top[0], top[2], top[3], top[4]} {
		if z := edhPilesZone(e, id); z != state.ZLibrary {
			t.Fatalf("card %d zone = %s, want library bottom", id, z)
		}
	}
	tail := lib[len(lib)-4:]
	for _, id := range []state.ObjID{top[0], top[2], top[3], top[4]} {
		if !contains(tail, id) {
			t.Fatalf("card %d not among the bottom four of the library", id)
		}
	}
	replayCheck(t, e, cfg)
}

// TestEDHPilesPhyrexianPortalTargetOpponentSeparates activates Phyrexian
// Portal: the targeted opponent separates the top ten cards into two piles
// (Separator$ Targeted, FaceDown$ True), the caster picks one, searches it for
// a card to hand and the other pile is exiled.
func TestEDHPilesPhyrexianPortalTargetOpponentSeparates(t *testing.T) {
	t.Parallel()
	deck0 := append([]*cards.Card{corpusAlternativeCard(t, "Phyrexian Portal")}, edhPilesBearDeck(t)...)
	e, cfg := fixtureGame(t, 7004, deck0, nil)
	portal := moveNamedTo(t, e, 0, "Phyrexian Portal", state.ZBattlefield)
	top := edhPilesLibraryTop(t, e, 0, edhPilesBears...)
	topTen := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 0)[:10]...)
	e.pending = nil
	addMana(t, e, 0, "RRR")
	submitChoices(t, e, abilityOption(t, e, portal, 0).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending %+v, want the opponent target ask", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == 1 {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("seat 1 not offered: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	d = nextPending(t, e)
	if d != nil && d.ResumeKind == "look_ack" {
		// PeekAndReveal still poses its look acknowledgement to the caster
		// even with NoPeek$ True (see the report's Issues).
		submitChoices(t, e, d.Options[0].Index)
		d = nextPending(t, e)
	}
	if d == nil || d.ResumeKind != "twopiles_split" || d.Player != 1 || len(d.Options) != 10 {
		t.Fatalf("split ask: pending %+v, want the target opponent over ten cards", d)
	}
	submitChoices(t, e, 0, 1, 2, 3) // pile A = Alpha..Delta
	d = e.Pending()
	if d == nil || d.ResumeKind != "twopiles_pick" || d.Player != 0 {
		t.Fatalf("pile pick: pending %+v, want the caster's", d)
	}
	submitChoices(t, e, d.Options[0].Index)
	d = nextPending(t, e)
	if d == nil || len(d.Options) == 0 {
		t.Fatalf("search pick: pending %+v, want a choice among pile A", d)
	}
	pickIdx := -1
	for _, o := range d.Options {
		if o.Obj == top[2] {
			pickIdx = o.Index
		}
	}
	if pickIdx < 0 {
		t.Fatalf("Gamma not offered: %+v", d.Options)
	}
	submitChoices(t, e, pickIdx)
	nextPending(t, e)
	if z := edhPilesZone(e, top[2]); z != state.ZHand {
		t.Fatalf("Gamma zone = %s, want hand", z)
	}
	for _, id := range topTen[4:] {
		if z := edhPilesZone(e, id); z != state.ZExile {
			t.Fatalf("pile B card %d zone = %s, want exile", id, z)
		}
	}
	for _, id := range []state.ObjID{top[0], top[1], top[3]} {
		if z := edhPilesZone(e, id); z != state.ZLibrary {
			t.Fatalf("shuffled-back card %d zone = %s, want library", id, z)
		}
	}
	replayCheck(t, e, cfg)
}
