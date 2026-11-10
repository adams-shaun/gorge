package dirac

import (
	"hash/fnv"
	"testing"
	"unsafe"

	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestInitNTwoSeatsEqualsInit: InitN with two decks leaves a State
// byte-identical to Init's, with and without a mulligan round, and the
// genesis it deals (toss, every library and hand, the RNG position and its
// next draws) hashes to one pinned value in every build.
func TestInitNTwoSeatsEqualsInit(t *testing.T) {
	h := fnv.New64a()
	for seed := uint64(0); seed < 200; seed++ {
		a, b := vanilla.Decks[seed%5], vanilla.Decks[(seed+1)%5]
		cfg := Config{Mulligans: uint8(seed % 3)}
		s, u := new(State), new(State)
		s.Init(vanilla.Table, a, b, seed, cfg)
		u.InitN(vanilla.Table, [MaxSeats][]CardID{a, b}, 2, seed, cfg)
		sb := unsafe.Slice((*byte)(unsafe.Pointer(s)), unsafe.Sizeof(*s))
		ub := unsafe.Slice((*byte)(unsafe.Pointer(u)), unsafe.Sizeof(*u))
		if string(sb) != string(ub) || !s.SameGame(u) {
			t.Fatalf("seed %d: InitN(2) differs from Init", seed)
		}
		buf := []byte{s.startPl, s.ctl, s.mullOrder[0], s.mullOrder[1], byte(s.nobj)}
		for p := 0; p < 2; p++ {
			pl := &s.pl[p]
			for _, o := range pl.lib[pl.libLo:pl.libHi] {
				buf = append(buf, byte(o), byte(s.objs[o].card), s.objs[o].owner())
			}
			buf = append(buf, 0xFF)
			for _, o := range pl.hand[:pl.handN] {
				buf = append(buf, byte(o), byte(s.objs[o].card), s.objs[o].owner())
			}
			buf = append(buf, 0xFF, byte(pl.life))
		}
		r := s.rng
		buf = append(buf, byte(r.Draws), byte(r.IntN(1000)), byte(r.IntN(97)), byte(r.IntN(2)))
		h.Write(buf)
	}
	const want = 0xec541494f9275116 // land before InitN, both builds
	if got := h.Sum64(); got != want {
		t.Fatalf("two-seat genesis hash %#x, pinned %#x", got, uint64(want))
	}
}
