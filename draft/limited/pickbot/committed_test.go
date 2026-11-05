package pickbot_test

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	pb "github.com/adams-shaun/gorge/draft/limited/pickbot"
	"github.com/adams-shaun/gorge/engine/cards"
)

const (
	colW = 1 << 0
	colU = 1 << 1
	colR = 1 << 3
)

// committedRefs resolves names to pool refs whose IDs carry the prefix.
func committedRefs(t *testing.T, reg *cards.Registry, prefix string, names []string) []limited.CardRef {
	t.Helper()
	refs := fdnPool(t, reg, names)
	for i := range refs {
		refs[i].ID = prefix + itoa(i)
	}
	return refs
}

func mono(bit uint8) func(c *cards.Card, gih float64) bool {
	return func(c *cards.Card, _ float64) bool { return c.ColourIdentity() == bit }
}

func without(names []string, drop ...string) []string {
	var out []string
	for _, n := range names {
		keep := true
		for _, d := range drop {
			if n == d {
				keep = false
			}
		}
		if keep {
			out = append(out, n)
		}
	}
	return out
}

func committedRNG() *rand.Rand { return rand.New(rand.NewPCG(3, 4)) }

// TestCommittedTakesTheMissingType: a pool with no instant and 20 creatures
// takes the slightly weaker on-colour instant over the creature.
func TestCommittedTakesTheMissingType(t *testing.T) {
	reg := corpus(t)
	var x, y string
	gap := 1.0
	for _, xn := range fdnMainSetNames(t) {
		xc, ok := reg.Lookup(xn)
		xe, rated := pb.RawRating("FDN", xn)
		if !ok || !rated || xc.ColourIdentity() != colW || !xc.Faces[0].IsInstant() {
			continue
		}
		for _, yn := range fdnMainSetNames(t) {
			yc, ok := reg.Lookup(yn)
			ye, rated := pb.RawRating("FDN", yn)
			if !ok || !rated || yc.ColourIdentity() != colW || !yc.Faces[0].IsCreature() || yc.Faces[0].Cmc() != xc.Faces[0].Cmc() {
				continue
			}
			if g := ye.GihWr - xe.GihWr; g > 0 && g < 0.04 && g < gap {
				gap, x, y = g, xn, yn
			}
		}
	}
	if x == "" {
		t.Fatal("no white instant/creature pair of equal mana value within 0.04 GIH")
	}
	white := fdnPick(t, reg, mono(colW), false, 40)
	blue := fdnPick(t, reg, func(c *cards.Card, g float64) bool {
		return mono(colU)(c, g) && c.Faces[0].IsCreature()
	}, false, 10)
	var whiteCreatures []string
	for _, n := range without(white, x, y) {
		if c, _ := reg.Lookup(n); c.Faces[0].IsCreature() {
			whiteCreatures = append(whiteCreatures, n)
		}
	}
	if len(whiteCreatures) < 10 || len(blue) != 10 {
		t.Fatalf("pool precondition: %d white creatures, %d blue", len(whiteCreatures), len(blue))
	}
	picks := committedRefs(t, reg, "p", append(whiteCreatures[:10:10], without(blue, x, y)...))
	pack := committedRefs(t, reg, "k", []string{y, x})
	pack[0].ID, pack[1].ID = "y", "x"
	v := pb.PickView{Set: "FDN", Round: 2, PickNo: 7, Pack: pack, Picks: picks}

	if got := (&pb.Committed{Lookup: lookupOf(reg)}).Pick(v, committedRNG()); got != "x" {
		t.Fatalf("Committed picked %q, want the missing-type instant x (%s over %s)", got, x, y)
	}
	off := &pb.RatingColour{Lookup: lookupOf(reg), Tuning: pb.CommittedTuning}
	if got := off.Pick(v, committedRNG()); got != "y" {
		t.Fatalf("RatingColour picked %q, want the higher-rated creature y", got)
	}
}

// TestCommittedStaysInItsPair: a white-blue pool takes the white card over a
// slightly better red one, where the default tuning takes the red.
func TestCommittedStaysInItsPair(t *testing.T) {
	reg := corpus(t)
	creature := func(c *cards.Card) bool { return c.Faces[0].IsCreature() }
	var w, r string
	best := 1.0
	for _, wn := range fdnMainSetNames(t) {
		wc, ok := reg.Lookup(wn)
		we, rated := pb.RawRating("FDN", wn)
		if !ok || !rated || wc.ColourIdentity() != colW || !creature(wc) {
			continue
		}
		for _, rn := range fdnMainSetNames(t) {
			rc, ok := reg.Lookup(rn)
			re, rated := pb.RawRating("FDN", rn)
			if !ok || !rated || rc.ColourIdentity() != colR || !creature(rc) {
				continue
			}
			g := re.GihWr - we.GihWr
			if g < 0.05 || g > 0.08 {
				continue
			}
			if d := g - 0.065; d*d < best {
				best, w, r = d*d, wn, rn
			}
		}
	}
	if w == "" {
		t.Fatal("no white/red creature pair 0.05-0.08 GIH apart")
	}
	white := without(fdnPick(t, reg, mono(colW), false, 12), w, r)[:10]
	blue := without(fdnPick(t, reg, mono(colU), false, 12), w, r)[:10]
	picks := committedRefs(t, reg, "p", append(white[:10:10], blue...))
	pack := committedRefs(t, reg, "k", []string{w, r})
	pack[0].ID, pack[1].ID = "w", "r"
	v := pb.PickView{Set: "FDN", Round: 2, PickNo: 7, Pack: pack, Picks: picks}

	if got := (&pb.Committed{Lookup: lookupOf(reg)}).Pick(v, committedRNG()); got != "w" {
		t.Fatalf("Committed picked %q, want the in-pair white card w", got)
	}
	if got := (&pb.RatingColour{Lookup: lookupOf(reg)}).Pick(v, committedRNG()); got != "r" {
		t.Fatalf("RatingColour picked %q, want the red card r", got)
	}
}

// TestCommittedMatchesRatingColourEarly: before the 10th pick there is no
// needs term, so the scores are RatingColour's under the committed tuning.
func TestCommittedMatchesRatingColourEarly(t *testing.T) {
	reg := corpus(t)
	names := fdnPick(t, reg, nil, false, 19)
	if len(names) != 19 {
		t.Fatalf("got %d names, want 19", len(names))
	}
	v := pb.PickView{
		Set: "FDN", Round: 1, PickNo: 6,
		Pack:  committedRefs(t, reg, "k", names[5:]),
		Picks: committedRefs(t, reg, "p", names[:5]),
	}
	got := (&pb.Committed{Lookup: lookupOf(reg)}).Scores(v, nil)
	want := (&pb.RatingColour{Lookup: lookupOf(reg), Tuning: pb.CommittedTuning}).Scores(v, nil)
	if len(got) != 14 || len(want) != 14 {
		t.Fatalf("score counts %d, %d, want 14", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("score %d = %v, want %v", i, got[i], want[i])
		}
	}
}

// TestCommittedWithoutLookup: with no Lookup it is the committed-tuning
// rating drafter and still picks from the pack.
func TestCommittedWithoutLookup(t *testing.T) {
	reg := corpus(t)
	names := fdnPick(t, reg, nil, false, 15)
	pack := committedRefs(t, reg, "k", names[:3])
	v := pb.PickView{Set: "FDN", Round: 2, PickNo: 4, Pack: pack, Picks: committedRefs(t, reg, "p", names[3:])}
	got := (&pb.Committed{}).Pick(v, committedRNG())
	for _, c := range pack {
		if c.ID == got {
			return
		}
	}
	t.Fatalf("picked %q, not in the pack", got)
}

func TestCommittedTuningDefaults(t *testing.T) {
	want := pb.DefaultTuning
	want.ColourFitMax = 24
	if pb.CommittedTuning != want {
		t.Fatalf("CommittedTuning = %+v, want DefaultTuning with ColourFitMax 24", pb.CommittedTuning)
	}
	if pb.DefaultNeedWeight != 8 {
		t.Fatalf("DefaultNeedWeight = %v, want 8", pb.DefaultNeedWeight)
	}
}
