package dirac

import (
	"testing"

	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestMarksCoverEveryMutation: over random games of every deck pairing,
// with an epoch re-based every few actions (so marks accumulate over
// several actions, as along a search path) and the search redeal applied
// at each base, every object without a mark hashes as it did at the base,
// and ViewHash read through the base equals ViewHash recomputed.
func TestMarksCoverEveryMutation(t *testing.T) {
	n := 600
	if testing.Short() {
		n = 100
	}
	var s State
	var base [MaxObj]uint64
	var hc ObjHashCache
	var d Decision
	actions, rebases := 0, 0
	for g := 0; g < n; g++ {
		nd := len(v.Decks)
		s.Init(v.Table, v.Decks[g%nd], v.Decks[(g/nd)%nd], uint64(g)*0x51ed27, Config{Mulligans: uint8(g % 3), NoElide: g%4 == 0})
		pol := NewRandomPolicy(uint64(g) ^ 0x77)
		r := NewRand(uint64(g), 3)
		every := 1 + g%5
		var since uint32
		rebase := func() {
			since = s.Touches()
			s.SetEpoch(uint32(rebases + 1))
			rebases++
			for o := 0; o < int(s.nobj); o++ {
				base[o] = s.ObjHash(Obj(o))
			}
			s.SetLine(uint64(rebases))
		}
		rebase()
		for i := 0; i < 4000; i++ {
			s.StepInto(&d)
			if d.Kind == DkNone {
				break
			}
			if i%every == 0 {
				rebase()
				if g%2 == 0 {
					// the search's redeal: the seat's own library, the
					// opponent's hidden cards
					seat := d.Player
					s.RedealOwnLibrary(seat, &r)
					var pool [2 * MaxDeck]CardID
					m := s.HiddenCards(1-seat, pool[:])
					s.DealHidden(1-seat, &r, pool[:m])
					s.StepInto(&d)
					if d.Kind == DkNone {
						break
					}
				}
			}
			s.Apply(pol.Pick(&d))
			actions++
			for o := 0; o < int(s.nobj); o++ {
				if s.ObjHash(Obj(o)) != base[o] && !s.ChangedSince(Obj(o), since) {
					t.Fatalf("game %d action %d: object %d changed but its stamp says not since %d (now %d)", g, i, o, since, s.Touches())
				}
			}
			if o, ok := CheckMarks(&s, &base); !ok {
				t.Fatalf("game %d action %d: object %d (card %d, zone %d) changed without a mark", g, i, o, s.objs[o].card, s.objs[o].zone)
			}
			for seat := uint8(0); seat < 2; seat++ {
				if a, b := s.ViewHashCached(seat, &base, &hc), s.ViewHash(seat, nil); a != b {
					t.Fatalf("game %d action %d: ViewHash through the base %x, recomputed %x", g, i, a, b)
				}
			}
		}
	}
	t.Logf("%d games, %d actions checked, %d epochs", n, actions, rebases)
}

// ChangedSince is exact for fewer than 256 writes and conservative beyond.
func TestChangedSince(t *testing.T) {
	var s State
	for n := 0; n < 1000; n++ {
		since := s.Touches()
		s.touch(Obj(n % 5))
		for k := 0; k < n%300; k++ {
			s.touch(Obj(5 + k%3))
		}
		if !s.ChangedSince(Obj(n%5), since) {
			t.Fatalf("n=%d: touched object reported unchanged", n)
		}
		other := Obj((n + 1) % 5)
		if got, want := s.ChangedSince(other, since), n%300 >= 255; got && !want {
			// a stale stamp may alias into the window only when the window wraps
			if uint8(s.stamp[other]-uint8(since))-1 >= uint8(s.touchN-since) {
				t.Fatalf("n=%d: untouched object reported changed", n)
			}
		}
	}
}

// A clone carries its base's epoch and marks; ViewHash tells two seats'
// views apart and is unchanged by hidden cards it cannot see.
func TestViewHashSeesOnlyTheSeatsView(t *testing.T) {
	s := NewConfig(v.Table, v.Decks[0], v.Decks[1], 9, Config{})
	pol := NewRandomPolicy(4)
	PlayRandom(s, &pol, 30)
	s.SetEpoch(7)
	c := new(State)
	c.CopyFrom(s)
	if c.Epoch() != 7 || c.ChangedMask() != ([ObjWords]uint64{}) {
		t.Fatal("clone lost its epoch or marks")
	}
	h0, h1 := s.ViewHash(0, nil), s.ViewHash(1, nil)
	if h0 == h1 {
		t.Fatal("both seats hash alike")
	}
	// re-deal seat 1's hidden cards: seat 0's view is unchanged
	r := NewRand(1, 2)
	var pool [2 * MaxDeck]CardID
	m := c.HiddenCards(1, pool[:])
	for i := 0; i < m/2; i++ {
		pool[i], pool[m-1-i] = pool[m-1-i], pool[i]
	}
	c.DealHidden(1, &r, pool[:m])
	if c.ViewHash(0, nil) != h0 {
		t.Fatal("seat 0's view hash read seat 1's hidden cards")
	}
	if c.HandSize(1) > 0 && c.ViewHash(1, nil) == h1 {
		t.Log("seat 1's re-dealt hand happened to match (possible, not checked)")
	}
}

func BenchmarkViewHash(b *testing.B) {
	s := NewConfig(v.Table, v.Decks[0], v.Decks[1], 9, Config{})
	pol := NewRandomPolicy(4)
	PlayRandom(s, &pol, 60)
	s.SetEpoch(1)
	var base [MaxObj]uint64
	for o := 0; o < int(s.nobj); o++ {
		base[o] = s.ObjHash(Obj(o))
	}
	b.Run("base", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			s.ViewHash(uint8(i&1), &base)
		}
	})
	b.Run("recompute", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			s.ViewHash(uint8(i&1), nil)
		}
	})
}

// TestViewHashCachedEverySeat: along one line of play, the cached view hash
// of every seat of the game equals the uncached one, at two seats and, in
// the dirac_mp build, at three and four: the cache keeps one object part per
// seat's view (a part cache of two seats panicked at seat 2, and folding a
// seat onto another would hand it the other's hand).
func TestViewHashCachedEverySeat(t *testing.T) {
	var c ObjHashCache
	for g := uint64(1); g <= 6; g++ {
		n := uint8(2 + g%uint64(MaxSeats-1))
		var decks [MaxSeats][]CardID
		for i := uint8(0); i < n; i++ {
			decks[i] = v.Decks[(int(g)+int(i))%len(v.Decks)]
		}
		var s State
		s.InitN(v.Table, decks, n, g, Config{})
		s.SetLine(g)
		pol := NewRandomPolicy(g)
		var d Decision
		for i := 0; i < 300; i++ {
			s.StepInto(&d)
			if d.Kind == DkNone {
				break
			}
			for seat := uint8(0); seat < n; seat++ {
				want := s.ViewHash(seat, nil)
				for k := 0; k < 2; k++ { // computed, then read from the cache
					if got := s.ViewHashCached(seat, nil, &c); got != want {
						t.Fatalf("game %d (%d seats) decision %d seat %d read %d: cached %x, uncached %x", g, n, i, seat, k, got, want)
					}
				}
			}
			s.Apply(pol.Pick(&d))
		}
	}
}
