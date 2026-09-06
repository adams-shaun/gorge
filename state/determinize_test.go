package state

// Task D1: property tests for Determinize. The whole value of the feature is
// the invariant:
//
//	No card visible to the searching seat may move, and the multiset of
//	unknown cards must be preserved.
//
// So it is tested as a property, over many seeded games and every seat, not as
// a handful of hand-written cases. All seeds are fixed constants -- a flaky
// property test is worse than none.

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/cards"
)

// determFixture builds a 4-seat game with distinct cards distributed across
// hidden AND public zones, plus a couple of objects on the shared stack. Card
// identity is unique per object (two differing ObjIDs never share a card), so
// "card multiset" and "ObjID multiset" are the same check.
func determFixture(t *testing.T) *Game {
	t.Helper()
	g := NewGame([]string{"alice", "bob", "carol", "dave"})
	n := 0
	card := func() *cards.Card {
		c := testCard(t, fmt.Sprintf("Name:C%02d\nManaCost:1 G\nTypes:Creature Bear\nPT:1/1\nOracle:x\n", n))
		n++
		return c
	}
	add := func(p PlayerID, z Zone, count int) []ObjID {
		ids := make([]ObjID, 0, count)
		for i := 0; i < count; i++ {
			ids = append(ids, g.AddObject(card(), p).ID)
		}
		g.SetZone(z, p, ids)
		return ids
	}
	for p := PlayerID(0); p < 4; p++ {
		add(p, ZLibrary, 6)
		add(p, ZHand, 3)
		add(p, ZBattlefield, 2)
		add(p, ZGraveyard, 3)
	}

	// Two spells on the shared stack. The stack is not per-seat; pass owner 0.
	add(0, ZStack, 2)
	return g
}

// zonesEqual reports element-for-element equality of two zone id slices.
func zonesEqual(a, b []ObjID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func concat(a, b []ObjID) []ObjID {
	out := make([]ObjID, 0, len(a)+len(b))
	out = append(out, a...)
	out = append(out, b...)
	return out
}

// multisetEqual reports whether the two id lists hold the same multiset.
func multisetEqual(a, b []ObjID) bool {
	if len(a) != len(b) {
		return false
	}
	aa := append([]ObjID(nil), a...)
	bb := append([]ObjID(nil), b...)
	sort.Slice(aa, func(i, j int) bool { return aa[i] < aa[j] })
	sort.Slice(bb, func(i, j int) bool { return bb[i] < bb[j] })
	for i := range aa {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}

// orderKey renders an id slice in its current order (for detecting ORDER
// variation across seeds; use sortedKey for multiset comparison).
func orderKey(ids []ObjID) string { return fmt.Sprint(ids) }

func sortedKey(ids []ObjID) string {
	s := append([]ObjID(nil), ids...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return fmt.Sprint(s)
}

func distinct(keys []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, k := range keys {
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	return out
}

// allIDs collects every id in every zone (per-seat five zones plus the shared
// stack) as one multiset.
func allIDs(g *Game) []ObjID {
	var out []ObjID
	for p := PlayerID(0); p < PlayerID(len(g.Players)); p++ {
		for _, z := range []Zone{ZLibrary, ZHand, ZBattlefield, ZGraveyard, ZExile} {
			out = concat(out, g.Zone(z, p))
		}
	}
	return concat(out, g.Stack)
}

// fullEqual reports complete structural equality of two games. Used only to
// prove the receiver was not mutated (a determinizer must be a pure function
// of its inputs). Deterministic: it compares slices, never iterates a map.
func fullEqual(a, b *Game) bool {
	if len(a.Players) != len(b.Players) || !reflect.DeepEqual(a.Players, b.Players) {
		return false
	}
	if len(a.Objs) != len(b.Objs) {
		return false
	}
	for i := range a.Objs {
		if !reflect.DeepEqual(a.Objs[i], b.Objs[i]) {
			return false
		}
	}
	if !reflect.DeepEqual(a.Stack, b.Stack) {
		return false
	}
	if len(a.zones) != len(b.zones) {
		return false
	}
	for i := range a.zones {
		if !reflect.DeepEqual(a.zones[i], b.zones[i]) {
			return false
		}
	}
	return a.Turn == b.Turn && a.Active == b.Active && a.Priority == b.Priority &&
		a.Step == b.Step && a.Passes == b.Passes && a.Over == b.Over &&
		a.Winner == b.Winner && a.Draw == b.Draw && a.NextID == b.NextID &&
		a.Clock == b.Clock
}

// checkDeterminize asserts every per-determinization part of the invariant.
func checkDeterminize(t *testing.T, g, d *Game, seat PlayerID) {
	t.Helper()

	// 1. Every public zone byte-identical, element for element.
	for p := PlayerID(0); p < PlayerID(len(g.Players)); p++ {
		for _, z := range []Zone{ZBattlefield, ZGraveyard, ZExile} {
			if !zonesEqual(d.Zone(z, p), g.Zone(z, p)) {
				t.Fatalf("public zone %s of seat %d changed", z, p)
			}
		}
	}
	if !zonesEqual(d.Stack, g.Stack) {
		t.Fatal("stack changed")
	}

	// 2. fromSeat's own hand identical, element for element.
	if !zonesEqual(d.Zone(ZHand, seat), g.Zone(ZHand, seat)) {
		t.Fatal("fromSeat's own hand changed")
	}

	// fromSeat's own library: composition preserved (order-variation is the
	// separate multi-seed assertion in the caller).
	if !multisetEqual(d.Zone(ZLibrary, seat), g.Zone(ZLibrary, seat)) {
		t.Fatal("fromSeat's own library lost or gained a card")
	}

	// 4. Each opponent's per-zone counts unchanged; 5. hidden multiset intact.
	for p := PlayerID(0); p < PlayerID(len(g.Players)); p++ {
		if p == seat {
			continue
		}
		if len(d.Zone(ZHand, p)) != len(g.Zone(ZHand, p)) {
			t.Fatalf("opponent %d hand count %d, source %d", p, len(d.Zone(ZHand, p)), len(g.Zone(ZHand, p)))
		}
		if len(d.Zone(ZLibrary, p)) != len(g.Zone(ZLibrary, p)) {
			t.Fatalf("opponent %d library count changed", p)
		}
		dh := concat(d.Zone(ZHand, p), d.Zone(ZLibrary, p))
		gh := concat(g.Zone(ZHand, p), g.Zone(ZLibrary, p))
		if !multisetEqual(dh, gh) {
			t.Fatalf("opponent %d hidden multiset changed", p)
		}
	}

	// No card may change owner: the re-deal is per owner.
	for p := PlayerID(0); p < PlayerID(len(g.Players)); p++ {
		for _, z := range []Zone{ZHand, ZLibrary} {
			for _, id := range d.Zone(z, p) {
				if o := d.Obj(id); o == nil || o.Owner != p {
					t.Fatalf("card id %d changed owner (zone %s seat %d): determinization crosses owners", id, z, p)
				}
			}
		}
	}

	// 6. Arena invariant and no ObjID created/destroyed/duplicated.
	if len(d.Objs) != len(g.Objs) {
		t.Fatalf("arena size %d, source %d", len(d.Objs), len(g.Objs))
	}
	for i := range d.Objs {
		if d.Objs[i].ID != ObjID(i+1) {
			t.Fatalf("arena not dense: Objs[%d].ID = %d", i, d.Objs[i].ID)
		}
	}
	if !multisetEqual(allIDs(d), allIDs(g)) {
		t.Fatal("multiset of all ids across all zones changed (creation/loss/duplication)")
	}
}

// TestDeterminizeAllBounds is the main property test: for every seat and many
// fixed-seed determinizations, every part of the invariant holds, and over the
// seed set the uncertain orders demonstrably vary while the certain ones never
// do. The receiver is verified unmodified at the end.
func TestDeterminizeAllBounds(t *testing.T) {
	for _, seat := range []PlayerID{0, 1, 2, 3} {
		seat := seat
		t.Run(fmt.Sprintf("seat%d", seat), func(t *testing.T) {
			g := determFixture(t)
			snap := g.Clone()

			var libOrders, oppHands []string
			const seeds = 64
			for s := uint64(0); s < seeds; s++ {
				rng := rand.New(rand.NewPCG(s, s^0x9e3779b97f4a7c15))
				d := g.Determinize(seat, rng)
				checkDeterminize(t, g, d, seat)

				libOrders = append(libOrders, orderKey(d.Zone(ZLibrary, seat)))
				for p := PlayerID(0); p < PlayerID(len(g.Players)); p++ {
					if p != seat {
						oppHands = append(oppHands, orderKey(d.Zone(ZHand, p)))
					}
				}
			}

			// 3 (second half): fromSeat's library is order-randomised, not an
			// identity -- over the seed set it must take more than one order
			// and at least once differ from the source order.
			if variants := len(distinct(libOrders)); variants < 2 {
				t.Fatalf("fromSeat library took only %d order(s) across %d seeds -- never randomised", variants, seeds)
			}
			if allSame(libOrders, orderKey(g.Zone(ZLibrary, seat))) {
				t.Fatal("fromSeat library never differs from its original order across seeds")
			}
			// Opponent hand contents re-deal too.
			if variants := len(distinct(oppHands)); variants < 2 {
				t.Fatalf("opponent hand contents took only %d value(s) across %d seeds", variants, seeds)
			}

			// 7. Receiver unmodified.
			if !fullEqual(g, snap) {
				t.Fatal("receiver game was mutated by Determinize")
			}
		})
	}
}

func allSame(keys []string, want string) bool {
	for _, k := range keys {
		if k != want {
			return false
		}
	}
	return true
}

// TestDeterminizeDeterministic proves same seed -> same game, and different
// seeds -> different game.
func TestDeterminizeDeterministic(t *testing.T) {
	g := determFixture(t)
	seed := func(s, h uint64) *Game {
		return g.Determinize(0, rand.New(rand.NewPCG(s, h)))
	}
	a1 := seed(42, 43)
	a2 := seed(42, 43)
	if !fullEqual(a1, a2) {
		t.Fatal("same seed produced different determinizations")
	}
	b := seed(99, 1)
	if fullEqual(a1, b) {
		t.Fatal("different seeds produced identical determinizations -- rng not consumed")
	}
}

// TestDeterminizeReceiverUnmodified — focused proof that Determinize is pure.
// Breaks if a mutation were applied to the receiver instead of the clone.
func TestDeterminizeReceiverUnmodified(t *testing.T) {
	g := determFixture(t)
	snap := g.Clone()
	// Mix seats and seeds so the receiver is exercised from every angle.
	for _, seat := range []PlayerID{0, 1, 2, 3} {
		for s := uint64(0); s < 8; s++ {
			g.Determinize(seat, rand.New(rand.NewPCG(s, s+1)))
		}
	}
	if !fullEqual(g, snap) {
		t.Fatal("receiver mutated by Determinize")
	}
}

// TestDeterminizePublicZonesByteIdentical — breaks if a public zone (here the
// graveyard) is re-randomised. Loops seats and seeds so a shuffle that ever
// reorders a multi-card public zone is caught; all seeds are fixed.
func TestDeterminizePublicZonesByteIdentical(t *testing.T) {
	g := determFixture(t)
	for _, seat := range []PlayerID{0, 1, 2, 3} {
		for s := uint64(0); s < 16; s++ {
			d := g.Determinize(seat, rand.New(rand.NewPCG(s, s)))
			for p := PlayerID(0); p < 4; p++ {
				for _, z := range []Zone{ZBattlefield, ZGraveyard, ZExile} {
					if !zonesEqual(d.Zone(z, p), g.Zone(z, p)) {
						t.Fatalf("determinization moved a card in public zone %s of seat %d (seed %d, seat %d)", z, p, s, seat)
					}
				}
			}
			if !zonesEqual(d.Stack, g.Stack) {
				t.Fatal("determinization moved the stack")
			}
		}
	}
}

// TestDeterminizePreservesOpponentCompositionByDesign pins the determinization
// assumption by name. Re-dealing an opponent's hidden zones (hand + library)
// preserves the true card multiset EXACTLY -- composition, not merely counts.
// This is not an incidental property but the load-bearing strategic assumption,
// and the reason SearchingSeatKnowsOpponentComposition is true: a
// determinization models a searching seat that knows each opponent's decklist
// AND has perfect recall of every card that has left it, so it can reconstruct
// precisely which cards remain hidden. It is deliberately not a pool-sampling
// model (gorge has no card-pool model); if the re-deal ever dropped or gained a
// card, this test fails by name.
func TestDeterminizePreservesOpponentCompositionByDesign(t *testing.T) {
	g := determFixture(t)
	for _, seat := range []PlayerID{0, 1, 2, 3} {
		for s := uint64(0); s < 16; s++ {
			d := g.Determinize(seat, rand.New(rand.NewPCG(s, s^0xdeadbeef)))
			// Opponent hidden zones, every seat but the searcher's, as one
			// multiset. Composition must be preserved even though the searching
			// seat sees only per-zone totals of these zones.
			var got, want []ObjID
			for p := PlayerID(0); p < PlayerID(len(g.Players)); p++ {
				if p == seat {
					continue
				}
				got = append(got, concat(d.Zone(ZHand, p), d.Zone(ZLibrary, p))...)
				want = append(want, concat(g.Zone(ZHand, p), g.Zone(ZLibrary, p))...)
			}
			if !multisetEqual(got, want) {
				t.Fatalf("seat %d seed %d: opponent hidden-zone composition changed -- the re-deal must randomise location, not composition", seat, s)
			}
		}
	}
}

// TestDeterminizeOpponentZoneCountsPreserved — breaks if a card moves between
// an opponent's hand and library (per-zone counts change).
func TestDeterminizeOpponentZoneCountsPreserved(t *testing.T) {
	g := determFixture(t)
	for _, seat := range []PlayerID{0, 1, 2, 3} {
		for s := uint64(0); s < 16; s++ {
			d := g.Determinize(seat, rand.New(rand.NewPCG(s, s)))
			for p := PlayerID(0); p < 4; p++ {
				if p == seat {
					continue
				}
				if len(d.Zone(ZHand, p)) != len(g.Zone(ZHand, p)) {
					t.Fatalf("seat %d seeing seat %d: hand %d != %d", seat, p, len(d.Zone(ZHand, p)), len(g.Zone(ZHand, p)))
				}
				if len(d.Zone(ZLibrary, p)) != len(g.Zone(ZLibrary, p)) {
					t.Fatalf("seat %d seeing seat %d: library %d != %d", seat, p, len(d.Zone(ZLibrary, p)), len(g.Zone(ZLibrary, p)))
				}
			}
		}
	}
}

// TestDeterminizeFromSeatHandUntouched — breaks if fromSeat's own hand moves.
func TestDeterminizeFromSeatHandUntouched(t *testing.T) {
	g := determFixture(t)
	for _, seat := range []PlayerID{0, 1, 2, 3} {
		for s := uint64(0); s < 16; s++ {
			d := g.Determinize(seat, rand.New(rand.NewPCG(s, s)))
			if !zonesEqual(d.Zone(ZHand, seat), g.Zone(ZHand, seat)) {
				t.Fatalf("seat %d: own hand was moved by determinization (seed %d)", seat, s)
			}
		}
	}
}

// TestDeterminizeFromSeatLibraryPermutes — breaks if fromSeat's own library is
// left in its original order (the "helpful" bug the task warns against).
func TestDeterminizeFromSeatLibraryPermutes(t *testing.T) {
	g := determFixture(t)
	var orders []string
	for s := uint64(0); s < 64; s++ {
		d := g.Determinize(0, rand.New(rand.NewPCG(s, s+3)))
		if !multisetEqual(d.Zone(ZLibrary, 0), g.Zone(ZLibrary, 0)) {
			t.Fatal("seat 0 library composition changed")
		}
		orders = append(orders, orderKey(d.Zone(ZLibrary, 0)))
	}
	if len(distinct(orders)) < 2 {
		t.Fatal("seat 0 library never randomised across seeds (kept original order?)")
	}
	if allSame(orders, orderKey(g.Zone(ZLibrary, 0))) {
		t.Fatal("seat 0 library always identical to source order -- not determinized")
	}
}

// TestDeterminizeCardsStayWithOwner — breaks if a card changes hands (the
// re-deal is per owner, never pooled across owners).
func TestDeterminizeCardsStayWithOwner(t *testing.T) {
	g := determFixture(t)
	for _, seat := range []PlayerID{0, 1, 2, 3} {
		for s := uint64(0); s < 16; s++ {
			d := g.Determinize(seat, rand.New(rand.NewPCG(s, s)))
			for p := PlayerID(0); p < 4; p++ {
				for _, z := range []Zone{ZHand, ZLibrary} {
					for _, id := range d.Zone(z, p) {
						if o := d.Obj(id); o == nil || o.Owner != p {
							t.Fatalf("card %d left owner %d (seed %d, seat %d)", id, p, s, seat)
						}
					}
				}
			}
		}
	}
}

// TestDeterminizeArenaInvariant — breaks if an ObjID is invented, dropped or
// duplicated, or the dense arena contract is violated.
func TestDeterminizeArenaInvariant(t *testing.T) {
	g := determFixture(t)
	for _, seat := range []PlayerID{0, 1, 2, 3} {
		for s := uint64(0); s < 8; s++ {
			d := g.Determinize(seat, rand.New(rand.NewPCG(s, s)))
			if len(d.Objs) != len(g.Objs) {
				t.Fatalf("arena size %d != %d", len(d.Objs), len(g.Objs))
			}
			for i := range d.Objs {
				if d.Objs[i].ID != ObjID(i+1) {
					t.Fatalf("arena not dense: Objs[%d].ID = %d", i, d.Objs[i].ID)
				}
			}
			if !multisetEqual(allIDs(d), allIDs(g)) {
				t.Fatal("id multiset changed")
			}
		}
	}
}
