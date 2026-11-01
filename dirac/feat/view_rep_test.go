package feat

import (
	"math/bits"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

func repPool(t *testing.T) *Encoder {
	t.Helper()
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(
		[]cardset.Entry{{Name: "Mountain", Count: 1}},
		[]cardset.Entry{{Name: "Mountain", Count: 1}},
	)
	if err != nil {
		t.Fatal(err)
	}
	return NewEncoder(NewPool(tbl, make([]CardSpec, len(tbl.Cards))))
}

// TestEachKwListsRepeatedGrant: a granted keyword with a repeat is listed
// once for the grant and once per repeat (gorge lists Flashback once per
// granting effect; two Sphinx of Forgotten Lore list it twice).
func TestEachKwListsRepeatedGrant(t *testing.T) {
	e := repPool(t)
	b := bits.TrailingZeros32(uint32(kop.KwFlashback))
	var reps kwReps
	reps[b] = 1
	vc := vcore{kw: kop.KwFlashback, kwRep: &reps}
	c := vcard{vcore: &vc}
	n := 0
	e.eachKw(&c, func(k *kwInfo) {
		if string(k.raw) == "Flashback" {
			n++
		}
	})
	if n != 2 {
		t.Fatalf("Flashback listed %d times, want 2 (the grant and its repeat)", n)
	}
}

// TestHintUseMarksCacheWay: a hint hit names a content-cache way but does
// not itself count as a use in cachedCard's intra-build eviction guard.
// useView must mark the way, or a later card of the same set can evict it
// while a listed card still points at its vcore. An overflow view is a
// build-local buffer and must not be reused by a hint.
func TestHintUseMarksCacheWay(t *testing.T) {
	e := repPool(t)
	e.vc = new(vcCache)
	key := uint64(0x123456789abcdef1)
	si := int(key >> (64 - vcSetBits))
	e.vc.set[si].key[0] = key
	e.vc.set[si].vc[0].key = key
	e.vc.use = 7
	e.vc.start = e.vc.use + 1
	if vc := e.useView(key); vc != &e.vc.set[si].vc[0] {
		t.Fatalf("useView returned %p, want the cached way", vc)
	}
	if got := e.vc.set[si].use[0]; got < e.vc.start {
		t.Fatalf("hint use stamp %d < build start %d: cachedCard may evict a listed view", got, e.vc.start)
	}
	e.vc.set[si].key[0] = 0
	e.vc.ovf[0].vc.key = key
	if vc := e.useView(key); vc != nil {
		t.Fatal("useView accepted an overflow view, which a later build may overwrite")
	}
}

// TestCardViewMarksHintWays: a second encode of an unchanged position hits
// the per-object line hints. Every hinted content-cache way must be marked
// used by the current build (cardView's useView), or a later card of the
// same set can evict it while an already listed vcard still points at its
// vcore. TestHintUseMarksCacheWay guards the helper; this guards the
// cardView integration.
func TestCardViewMarksHintWays(t *testing.T) {
	set := cardsettest.Set(t, "fdn")
	tbl, da, db, err := set.Matchup(
		[]cardset.Entry{{Name: "Mountain", Count: 20}, {Name: "Healer's Hawk", Count: 20}},
		[]cardset.Entry{{Name: "Mountain", Count: 20}, {Name: "Healer's Hawk", Count: 20}},
	)
	if err != nil {
		t.Fatal(err)
	}
	e := NewEncoder(NewPool(tbl, make([]CardSpec, len(tbl.Cards))))
	s := dirac.New(tbl, da, db, 7)
	s.SetLine(1)
	d := s.Step()
	if d.Kind == dirac.DkNone {
		t.Fatal("no decision to encode")
	}
	e.EncodeStateFor(s, &d, d.Player)
	e.EncodeStateFor(s, &d, d.Player)
	hits := 0
	for o := range e.ohint {
		h := &e.ohint[o]
		if h.line == 0 || h.vc == nil {
			continue
		}
		c := &e.vc.set[h.key>>(64-vcSetBits)]
		for i := range c.key {
			if c.key[i] != h.key {
				continue
			}
			hits++
			if c.use[i] < e.vc.start {
				t.Fatalf("object %d hint way use %d < build start %d: cardView did not mark it, so cachedCard can evict a listed view", o, c.use[i], e.vc.start)
			}
		}
	}
	if hits == 0 {
		t.Fatal("no hint hit: the test does not exercise the cardView hint path")
	}
}

// TestSpelledRepsMerges: two calls for the same bit add up (a granted-off
// repeat and a static pump's repeat both count), saturating at 255.
func TestSpelledRepsMerges(t *testing.T) {
	e := repPool(t)
	b := bits.TrailingZeros32(uint32(kop.KwFlashback))
	var rep, dst kwReps
	rep[b] = 1
	if !e.spelledReps(&rep, &dst) || !e.spelledReps(&rep, &dst) {
		t.Fatal("spelledReps reported no repeat")
	}
	if dst[b] != 2 {
		t.Fatalf("merged repeats %d, want 2", dst[b])
	}
	rep[b] = 255
	if !e.spelledReps(&rep, &dst) {
		t.Fatal("spelledReps reported no repeat")
	}
	if dst[b] != 255 {
		t.Fatalf("saturated repeats %d, want 255", dst[b])
	}
}
