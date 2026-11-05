package pickbot_test

import (
	"encoding/json"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/booster"
	"github.com/adams-shaun/gorge/draft/limited/draft"
	pb "github.com/adams-shaun/gorge/draft/limited/pickbot"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// knobsPod drafts one seeded 8-seat FDN pod with d for every seat (each
// seat its own seeded rng) and builds every pool with b, returning every
// pick in order, every deck and every pool.
func knobsPod(t *testing.T, seed uint64, d func() pb.Drafter, b pb.Builder) (picks []string, decks []string, pools [][]limited.CardRef) {
	t.Helper()
	spec, err := setdata.Load("FDN")
	if err != nil {
		t.Fatalf("setdata FDN: %v", err)
	}
	packs, err := booster.OpenPod(spec, seed, 8, 3, &booster.IDs{}, nil)
	if err != nil {
		t.Fatalf("OpenPod: %v", err)
	}
	pod, err := draft.NewPod(seed, packs)
	if err != nil {
		t.Fatalf("NewPod: %v", err)
	}
	drafters := make([]pb.Drafter, 8)
	rngs := make([]*rand.Rand, 8)
	for s := range drafters {
		drafters[s] = d()
		rngs[s] = rand.New(rand.NewPCG(seed, uint64(s)))
	}
	for !pod.Done() {
		progressed := false
		for s := 0; s < 8; s++ {
			v, ok := pod.Next(s)
			if !ok {
				continue
			}
			id := drafters[s].Pick(v, rngs[s])
			if err := pod.Pick(s, v.PickNo, id); err != nil {
				t.Fatalf("seat %d pick %d: %v", s, v.PickNo, err)
			}
			picks = append(picks, id)
			progressed = true
		}
		if !progressed {
			t.Fatal("pod stalled")
		}
	}
	for s := 0; s < 8; s++ {
		pools = append(pools, pod.Pool(s))
		raw, err := json.Marshal(b.Build("FDN", pod.Pool(s)))
		if err != nil {
			t.Fatal(err)
		}
		decks = append(decks, string(raw))
	}
	return picks, decks, pools
}

// TestZeroKnobsByteIdentical: the zero Tuning (NewTuned and a "{}" knob
// file) drafts exactly as Committed and builds exactly as Best23 over four
// seeded pods, every pick and every deck.
func TestZeroKnobsByteIdentical(t *testing.T) {
	lookup := lookupOf(corpus(t))
	fromFile, err := pb.ParseTuning([]byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if fromFile != (pb.Tuning{}) {
		t.Fatalf("ParseTuning({}) = %+v, want the zero Tuning", fromFile)
	}
	if got := (pb.Tuning{}).Filled(); got != pb.CommittedTuning {
		t.Fatalf("Filled(zero) = %+v, want CommittedTuning", got)
	}
	differ := false
	for _, seed := range []uint64{11, 23, 37, 41} {
		wantPicks, wantDecks, _ := knobsPod(t, seed,
			func() pb.Drafter { return &pb.Committed{Lookup: lookup} }, &pb.Best23{Lookup: lookup})
		if len(wantPicks) < 8*3*13 {
			t.Fatalf("seed %d: %d picks, want a full pod", seed, len(wantPicks))
		}
		_, tb := pb.NewTuned(fromFile, lookup)
		gotPicks, gotDecks, _ := knobsPod(t, seed,
			func() pb.Drafter { d, _ := pb.NewTuned(fromFile, lookup); return d }, tb)
		if !reflect.DeepEqual(gotPicks, wantPicks) {
			t.Errorf("seed %d: zero-knob picks differ from Committed", seed)
		}
		if !reflect.DeepEqual(gotDecks, wantDecks) {
			t.Errorf("seed %d: zero-knob decks differ from Best23", seed)
		}
		// Precondition: the comparison can see a knob change at all.
		k := pb.Tuning{SignalWeight: 4, Deny: 5, Bias: [5]float64{3, 0, -2, -2, 0}}
		kPicks, _, _ := knobsPod(t, seed, func() pb.Drafter { d, _ := pb.NewTuned(k, lookup); return d }, tb)
		if !reflect.DeepEqual(kPicks, wantPicks) {
			differ = true
		}
	}
	if !differ {
		t.Fatal("non-zero knobs drafted identically in every pod: the comparison cannot fail")
	}
}

// TestKnobsJSONRoundTrip: every knob survives Marshal then ParseTuning
// exactly, and the hash is content-addressed over the filled knobs.
func TestKnobsJSONRoundTrip(t *testing.T) {
	in := pb.Tuning{
		ColourFitMax: 8, ColourFitRamp: 7, Need: 0.1 + 0.2, SignalWeight: 2, SignalStart: 3,
		Bias:     [5]float64{0, 0, -2, -2, 3},
		PairBias: [10]float64{9: 1.5, 0: -0.25},
		Deny:     1, MainSize: 24, Lands: 16, MinCreatures: 15, CreatureMax: 18, MaxHighValue: 2,
		CurveMin: [6]int{0, 4, 3, 0, 0, 0}, CurveMax: [6]int{3, 0, 0, 0, 5, 2},
		ShortPairPenalty: -7.125,
	}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := pb.ParseTuning(raw)
	if err != nil {
		t.Fatalf("ParseTuning(%s): %v", raw, err)
	}
	if out != in {
		t.Fatalf("round trip:\n got %+v\nwant %+v", out, in)
	}
	again, _ := json.Marshal(out)
	if string(again) != string(raw) {
		t.Fatalf("re-marshal differs:\n%s\n%s", again, raw)
	}
	if h := in.Hash(); len(h) != 12 || h == (pb.Tuning{}).Hash() {
		t.Fatalf("Hash = %q (zero %q): want 12 hex digits differing from the zero knobs", h, (pb.Tuning{}).Hash())
	}
	if (pb.Tuning{}).Hash() != pb.CommittedTuning.Hash() {
		t.Fatal("the zero knobs and their filled form hash differently")
	}
	// The research string reads onto the same knobs.
	r, err := pb.ParseResearchKnobs("cfm=8;need=4;sig=2;deny=1;bias=G+3/B-2;pbias=RG+1.5")
	if err != nil {
		t.Fatal(err)
	}
	want := pb.Tuning{ColourFitMax: 8, Need: 4, SignalWeight: 2, Deny: 1, Bias: [5]float64{2: -2, 4: 3}, PairBias: [10]float64{9: 1.5}}
	if r != want {
		t.Fatalf("ParseResearchKnobs = %+v, want %+v", r, want)
	}
}

// TestKnobsUnknownKeyRefused: a key Tuning does not have is refused by
// name, as are trailing data and inconsistent knobs.
func TestKnobsUnknownKeyRefused(t *testing.T) {
	if _, err := pb.ParseTuning([]byte(`{"need": 4}`)); err != nil {
		t.Fatalf("known key refused: %v", err)
	}
	_, err := pb.ParseTuning([]byte(`{"need": 4, "colour_fit_maxx": 9}`))
	if err == nil || !strings.Contains(err.Error(), "colour_fit_maxx") {
		t.Fatalf("unknown key: err = %v, want one naming colour_fit_maxx", err)
	}
	for _, bad := range []string{`{} {}`, `{"lands": 16, "main_size": 23}`, `{"creature_max": 10}`, `{"curve_min": [9,9,9,0,0,0]}`} {
		if _, err := pb.ParseTuning([]byte(bad)); err == nil {
			t.Errorf("ParseTuning(%s) accepted", bad)
		}
	}
	if _, err := pb.ParseResearchKnobs("cfm=8;wat=1"); err == nil || !strings.Contains(err.Error(), "wat") {
		t.Fatalf("research unknown key: err = %v, want one naming wat", err)
	}
}

// TestBest23HonoursBuilderKnobs: CreatureMax, CurveMax and CurveMin bound
// the main a knob set builds, where the default build breaks them.
func TestBest23HonoursBuilderKnobs(t *testing.T) {
	reg := corpus(t)
	lookup := lookupOf(reg)
	_, kb := pb.NewTuned(pb.Tuning{MinCreatures: 12, CreatureMax: 13, CurveMax: [6]int{5: 1}}, lookup)
	_, fb := pb.NewTuned(pb.Tuning{CurveMin: [6]int{1: 8}}, lookup)
	_, _, pools := knobsPod(t, 5, func() pb.Drafter { return &pb.Committed{Lookup: lookup} }, &pb.Best23{Lookup: lookup})
	measure := func(b pb.Builder, pool []limited.CardRef) (creatures int, buckets [6]int) {
		names := map[string]string{}
		for _, r := range pool {
			names[r.ID] = r.Name
		}
		for _, id := range b.Build("FDN", pool).Main {
			f := lookupCard(t, reg, names[id]).Faces[0]
			if f.IsCreature() {
				creatures++
			}
			mv := int(f.Cmc())
			switch {
			case mv <= 1:
				buckets[0]++
			case mv >= 6:
				buckets[5]++
			default:
				buckets[mv-1]++
			}
		}
		return creatures, buckets
	}
	broke, raised := false, false
	for s, pool := range pools {
		dc, db := measure(&pb.Best23{Lookup: lookup}, pool)
		if dc > 13 || db[5] > 1 {
			broke = true
		}
		c, bk := measure(kb, pool)
		if c > 13 || bk[5] > 1 {
			t.Errorf("seat %d: %d creatures, %d 6+ cards; caps are 13 and 1", s, c, bk[5])
		}
		_, fk := measure(fb, pool)
		if fk[1] < db[1] {
			t.Errorf("seat %d: %d two-drops under a floor of 8, default plays %d", s, fk[1], db[1])
		}
		if db[1] < 8 && fk[1] > db[1] {
			raised = true
		}
	}
	if !broke {
		t.Fatal("the default build already meets every cap: the test cannot fail")
	}
	if !raised {
		t.Fatal("curve_min raised no seat's two-drops")
	}
}
