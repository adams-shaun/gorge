package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// seekHost is fakeHost with a scripted RNG queue: it proves the seek draws
// are seeded and distinct, which the always-zero fakeHost cannot (every draw
// would land on candidate 0). A short queue falls back to 0 (the fakeHost
// default); a script out of range clamps into [0,n).
type seekHost struct {
	*fakeHost
	draws []int
}

func (h *seekHost) Rand(n int) int {
	h.n++
	if len(h.draws) == 0 {
		return 0
	}
	v := h.draws[0]
	h.draws = h.draws[1:]
	if v < 0 {
		v = 0
	}
	if v >= n {
		v = n - 1
	}
	return v
}

func newSeekHost(t *testing.T, seats int, draws ...int) *seekHost {
	t.Helper()
	return &seekHost{fakeHost: newHost(t, seats), draws: draws}
}

// seekSource adds a battlefield source so ImprintFound$/RememberFound$ have a
// persistent object to bind to.
func seekSource(t *testing.T, h *fakeHost) state.ObjID {
	t.Helper()
	src := h.g.AddObject(mkCard(t, "Name:Seeker\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{src.ID})
	return src.ID
}

// seekCards adds distinct card fixtures on seat p's library in order (index 0
// is the top) and returns their ids.
func seekCards(t *testing.T, h *fakeHost, p state.PlayerID, cardsInOrder ...*cards.Card) []state.ObjID {
	t.Helper()
	var ids []state.ObjID
	for _, c := range cardsInOrder {
		ids = append(ids, h.g.AddObject(c, p).ID)
	}
	h.g.SetZone(state.ZLibrary, p, ids)
	return ids
}

func seekNotes(h *fakeHost) []string {
	var out []string
	for _, e := range h.log {
		if e.Kind == events.Note {
			out = append(out, e.Text)
		}
	}
	return out
}

func seekMarkers(h *fakeHost) []events.Event {
	var out []events.Event
	for _, e := range h.log {
		if e.Kind == events.Seek {
			out = append(out, e)
		}
	}
	return out
}

func seekHasNoteContaining(h *fakeHost, sub string) bool {
	for _, n := range seekNotes(h) {
		if len(n) >= len(sub) {
			for i := 0; i+len(sub) <= len(n); i++ {
				if n[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

func TestSeekRandomlyMovesMatchingCardsWithoutShuffleOrReveal(t *testing.T) {
	h := newSeekHost(t, 2, 2, 0)
	src := seekSource(t, h.fakeHost)
	creA := mkCard(t, "Name:Alpha\nTypes:Creature\nPT:1/1\nOracle:x\n")
	creB := mkCard(t, "Name:Beta\nTypes:Creature\nPT:1/1\nOracle:x\n")
	creC := mkCard(t, "Name:Gamma\nTypes:Creature\nPT:1/1\nOracle:x\n")
	land := mkCard(t, "Name:Waste\nTypes:Land\nOracle:x\n")
	// Top first: A, B, C, land. Eligible = [A,B,C]; draws 2 then 0 pick C
	// then A, leaving B and the land.
	ids := seekCards(t, h.fakeHost, 0, creA, creB, creC, land)
	if len(h.g.Zone(state.ZLibrary, 0)) != 4 {
		t.Fatalf("precondition: library = %d, want 4", len(h.g.Zone(state.ZLibrary, 0)))
	}

	Resolve(h, &Ctx{Controller: 0, Source: src}, sa(t, "SP$ Seek | Type$ Creature | Num$ 2"))

	if h.n != 2 {
		t.Fatalf("Rand calls = %d, want exactly 2", h.n)
	}
	hand := h.g.Zone(state.ZHand, 0)
	if len(hand) != 2 {
		t.Fatalf("hand = %v, want exactly two moved cards", hand)
	}
	wantMoved := map[state.ObjID]bool{ids[2]: true, ids[0]: true}
	for _, id := range hand {
		if !wantMoved[id] {
			t.Fatalf("hand contains %d, want exactly the scripted distinct draws {%d,%d}", id, ids[2], ids[0])
		}
	}
	if h.g.Obj(ids[1]).Zone != state.ZLibrary {
		t.Fatalf("ineligible-for-script card %d left the library (zone %s)", ids[1], h.g.Obj(ids[1]).Zone)
	}
	if h.g.Obj(ids[3]).Zone != state.ZLibrary {
		t.Fatalf("ineligible land %d left the library (zone %s)", ids[3], h.g.Obj(ids[3]).Zone)
	}
	for _, e := range h.log {
		if e.Kind == events.Shuffle {
			t.Fatalf("seek emitted a Shuffle: %+v", e)
		}
		if e.Kind == events.Note && len(e.IDs) != 0 {
			t.Fatalf("seek emitted a public ID-bearing reveal Note: %+v", e)
		}
	}
	if len(seekMarkers(h.fakeHost)) != 1 {
		t.Fatalf("seek markers = %d, want exactly 1 for the completing action", len(seekMarkers(h.fakeHost)))
	}
}

func TestSeekEmptyAndRememberedImprintedRiders(t *testing.T) {
	// Precondition the seek handler runs even when it finds nothing: assert
	// the resolver's unimplemented-API Note never appears.
	h := newSeekHost(t, 2)
	src := seekSource(t, h.fakeHost)
	creature := mkCard(t, "Name:Alpha\nTypes:Creature\nPT:1/1\nOracle:x\n")
	creatureID := seekCards(t, h.fakeHost, 0, creature)[0]
	Resolve(h, &Ctx{Controller: 0, Source: src}, sa(t, "SP$ Seek | Type$ Artifact"))
	if seekHasNoteContaining(h.fakeHost, "unimplemented API Seek") {
		t.Fatal("empty seek reached the unimplemented API fallback")
	}
	if len(seekMarkers(h.fakeHost)) != 0 {
		t.Fatalf("no-match seek emitted %d markers, want 0", len(seekMarkers(h.fakeHost)))
	}
	if len(h.g.Zone(state.ZHand, 0)) != 0 || h.g.Obj(creatureID).Zone != state.ZLibrary {
		t.Fatal("no-match seek moved a card")
	}

	// RememberFound$: both the resolution Ctx binding and the source's
	// persistent (event-backed) remembered list must carry the found card.
	rememberCtx := &Ctx{Controller: 0, Source: src}
	Resolve(h, rememberCtx, sa(t, "SP$ Seek | Type$ Creature | RememberFound$ True"))
	if len(h.g.Zone(state.ZHand, 0)) != 1 {
		t.Fatalf("remembered seek moved %d cards, want 1", len(h.g.Zone(state.ZHand, 0)))
	}
	found := h.g.Zone(state.ZHand, 0)[0]
	got := h.g.Obj(src).Remembered
	if len(got) != 1 || got[0].IsPlayer || got[0].Obj != found {
		t.Fatalf("source Remembered = %v, want [%d] (the found card)", got, found)
	}
	// The live resolving context, not a test-authored replacement, must bind
	// the found card for a chained Defined$ Remembered consumer.
	if len(rememberCtx.Remembered) != 1 || rememberCtx.Remembered[0].IsPlayer || rememberCtx.Remembered[0].Obj != found {
		t.Fatalf("resolving Ctx.Remembered = %+v, want the found card %d", rememberCtx.Remembered, found)
	}
	rem := Defined(h, rememberCtx, &cards.SA{Params: map[string]string{"Defined": "Remembered"}})
	if len(rem) != 1 || rem[0].Obj != found {
		t.Fatalf("Defined$ Remembered = %+v, want the found card %d", rem, found)
	}

	// ImprintFound$: the source's SeekFound association must hold the found
	// card AND a chained `Defined$ Imprinted | Origin$ Hand` body -- the real
	// Spawning Pod / Gitrog continuation shape -- must move it on.
	h2 := newSeekHost(t, 2)
	src2 := seekSource(t, h2.fakeHost)
	cre2 := mkCard(t, "Name:Alpha\nTypes:Creature\nPT:1/1\nOracle:x\n")
	seekCards(t, h2.fakeHost, 0, cre2)
	Resolve(h2, &Ctx{Controller: 0, Source: src2},
		sa(t, "SP$ Seek | Type$ Creature | ImprintFound$ True"))
	if len(h2.g.Zone(state.ZHand, 0)) != 1 {
		t.Fatalf("imprint seek moved %d cards, want 1", len(h2.g.Zone(state.ZHand, 0)))
	}
	found2 := h2.g.Zone(state.ZHand, 0)[0]
	if sf := h2.g.Obj(src2).SeekFound; len(sf) != 1 || sf[0] != found2 {
		t.Fatalf("source SeekFound = %v, want [%d]", sf, found2)
	}
	// The reader must resolve `Defined$ Imprinted` while the card is in hand.
	imp := Defined(h2, &Ctx{Controller: 0, Source: src2},
		&cards.SA{Params: map[string]string{"Defined": "Imprinted"}})
	if len(imp) != 1 || imp[0].Obj != found2 {
		t.Fatalf("Defined$ Imprinted = %+v, want the hand-held found card %d", imp, found2)
	}
	Resolve(h2, &Ctx{Controller: 0, Source: src2},
		sa(t, "DB$ ChangeZone | Origin$ Hand | Destination$ Battlefield | Defined$ Imprinted"))
	if h2.g.Obj(found2).Zone != state.ZBattlefield {
		t.Fatalf("chained Defined$ Imprinted continuation left the found card in %s, want battlefield",
			h2.g.Obj(found2).Zone)
	}
}

func TestSeekTypesAndTopTenPool(t *testing.T) {
	// Types$ is one independent random pick per alternative, and a card
	// selected by an earlier alternative cannot be selected again by a later
	// one that also matches it. Library [creature, land]; Types Creature,Card:
	// the creature is drawn by the first alternative, so the second (which
	// matches both) must take the land.
	h := newSeekHost(t, 1)
	creature := mkCard(t, "Name:Alpha\nTypes:Creature\nPT:1/1\nOracle:x\n")
	land := mkCard(t, "Name:Waste\nTypes:Land\nOracle:x\n")
	ids := seekCards(t, h.fakeHost, 0, creature, land)
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ Seek | Types$ Creature,Card"))
	if len(h.g.Zone(state.ZHand, 0)) != 2 || h.n != 2 {
		t.Fatalf("Types seek hand/Rand = %d/%d, want 2/2", len(h.g.Zone(state.ZHand, 0)), h.n)
	}
	seen := map[state.ObjID]bool{}
	for _, id := range h.g.Zone(state.ZHand, 0) {
		if seen[id] {
			t.Fatalf("Types seek selected %d twice", id)
		}
		seen[id] = true
	}
	if !seen[ids[0]] || !seen[ids[1]] {
		t.Fatalf("Types seek moved %v, want both alternatives' distinct cards", seen)
	}

	// Top_10_OfLibrary: a matching card at index 10 is out of the eligible
	// prefix and cannot be selected, while one inside the prefix can.
	h2 := newSeekHost(t, 1)
	topCard := mkCard(t, "Name:Top Bear\nTypes:Creature\nPT:1/1\nOracle:x\n")
	belowCard := mkCard(t, "Name:Deep Bear\nTypes:Creature\nPT:1/1\nOracle:x\n")
	pool := make([]*cards.Card, 0, 11)
	for i := 0; i < 9; i++ {
		pool = append(pool, mkCard(t, "Name:Waste\nTypes:Land\nOracle:x\n"))
	}
	pool = append(pool, topCard)
	pool = append(pool, belowCard)
	ids2 := seekCards(t, h2.fakeHost, 0, pool...)
	if len(ids2) != 11 {
		t.Fatalf("precondition: pool = %d, want 11", len(ids2))
	}
	Resolve(h2, &Ctx{Controller: 0}, sa(t, "SP$ Seek | Type$ Creature | DefinedCards$ Top_10_OfLibrary"))
	if h2.g.Obj(ids2[10]).Zone != state.ZLibrary {
		t.Fatalf("Top_10_OfLibrary selected the below-prefix card %d", ids2[10])
	}
	if h2.g.Obj(ids2[9]).Zone != state.ZHand {
		t.Fatalf("Top_10_OfLibrary missed the in-prefix matching card %d (zone %s)", ids2[9], h2.g.Obj(ids2[9]).Zone)
	}
}

func TestSeekDefinedEachPlayer(t *testing.T) {
	h := newSeekHost(t, 2)
	creature := mkCard(t, "Name:Alpha\nTypes:Creature\nPT:1/1\nOracle:x\n")
	c0 := h.g.AddObject(creature, 0).ID
	c1 := h.g.AddObject(creature, 1).ID
	h.g.SetZone(state.ZLibrary, 0, []state.ObjID{c0})
	h.g.SetZone(state.ZLibrary, 1, []state.ObjID{c1})
	if len(h.g.Zone(state.ZLibrary, 0)) != 1 || len(h.g.Zone(state.ZLibrary, 1)) != 1 {
		t.Fatal("precondition: each player must have one eligible library card")
	}

	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ Seek | Defined$ Player | Type$ Creature"))
	if len(h.g.Zone(state.ZHand, 0)) != 1 || len(h.g.Zone(state.ZHand, 1)) != 1 {
		t.Fatalf("player seeks hand sizes = %d/%d, want 1/1",
			len(h.g.Zone(state.ZHand, 0)), len(h.g.Zone(state.ZHand, 1)))
	}
	if h.g.Obj(c0).Zone != state.ZHand || h.g.Obj(c1).Zone != state.ZHand {
		t.Fatalf("each player's own sought card not in hand: %s/%s",
			h.g.Obj(c0).Zone, h.g.Obj(c1).Zone)
	}
	markers := seekMarkers(h.fakeHost)
	if len(markers) != 2 {
		t.Fatalf("markers = %d, want one per completed player seek", len(markers))
	}
	if markers[0].Player != 0 || markers[1].Player != 1 {
		t.Fatalf("marker player order = %d,%d, want deterministic 0,1", markers[0].Player, markers[1].Player)
	}
}
