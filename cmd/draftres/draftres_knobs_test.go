package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/draftlab"
	"github.com/adams-shaun/gorge/draft/limited/pickbot"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func openLab(t *testing.T) *draftlab.Lab {
	t.Helper()
	lab, err := draftlab.Open(testutil.CorpusDirOrSkip(t), draftlab.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return lab
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestKnobFileBuilderSetsBest23Knobs(t *testing.T) {
	lab := openLab(t)
	dir := t.TempDir()
	kf := filepath.Join(dir, "b.json")
	writeFile(t, kf, `{"min_creatures": 13, "max_high_value": 2}`)
	nb, err := parseBuilder(lab, "knobs:"+kf, "FDN")
	if err != nil {
		t.Fatalf("knobs:%s: %v", kf, err)
	}
	b23, ok := nb.Builder.(*pickbot.Best23)
	if !ok {
		t.Fatalf("knobs: builder is %T, want *pickbot.Best23", nb.Builder)
	}
	if b23.Tuning.MinCreatures != 13 || b23.Tuning.MaxHighValue != 2 {
		t.Fatalf("knobs: tuning min_creatures %d max_high_value %d, want 13/2",
			b23.Tuning.MinCreatures, b23.Tuning.MaxHighValue)
	}
	// An explicit version equal to the file's hash is accepted; a stale
	// one is refused (draftlab's rule for knob refs).
	hnb, err := parseBuilder(lab, "knobs:"+kf+"@"+nb.Ref.Version, "FDN")
	if err != nil {
		t.Fatalf("knobs with matching hash: %v", err)
	}
	if hnb.Ref.Version != nb.Ref.Version {
		t.Fatalf("resolved ref version %q, want %q", hnb.Ref.Version, nb.Ref.Version)
	}
	_, err = parseBuilder(lab, "knobs:"+kf+"@deadbeefcafe", "FDN")
	if err == nil {
		t.Fatal("knobs with a wrong hash: want an error, got none")
	}
	// +r swaps the built-in Best23's ratings.
	rf := filepath.Join(dir, "r.json")
	writeFile(t, rf, `{"AAA": {"gih": 0.62}, "BBB": {"gih": 0.44}}`)
	rnb, err := parseBuilder(lab, "knobs:"+kf+"+r="+rf+"#gih", "FDN")
	if err != nil {
		t.Fatalf("knobs+r: %v", err)
	}
	if rnb.Builder.(*pickbot.Best23).Ratings == nil {
		t.Fatal("knobs+r: Best23.Ratings is nil")
	}
	// An unknown key is refused naming it.
	bad := filepath.Join(dir, "bad.json")
	writeFile(t, bad, `{"min_creatures": 13, "nope": 1}`)
	_, err = parseBuilder(lab, "knobs:"+bad, "FDN")
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("unknown key: err %v, want one naming nope", err)
	}
	// balanced: shape knobs still work.
	bnb, err := parseBuilder(lab, "balanced:main=22", "FDN")
	if err != nil {
		t.Fatalf("balanced: %v", err)
	}
	bal, ok := bnb.Builder.(*pickbot.Balanced)
	if !ok {
		t.Fatalf("balanced: builder is %T, want *pickbot.Balanced", bnb.Builder)
	}
	if bal.Shape.MainSize != 22 {
		t.Fatalf("balanced: Shape.MainSize %d, want 22", bal.Shape.MainSize)
	}
}

func TestKnobFileVariantMatchesInlineKnobs(t *testing.T) {
	lab := openLab(t)
	lookup := func(name string) *cards.Card { c, _ := lab.Card(name); return c }
	dir := t.TempDir()
	kf := filepath.Join(dir, "d.json")
	writeFile(t, kf, `{"colour_fit_max": 8, "signal_weight": 2}`)
	d1, err := newDrafter("knobs:"+kf, lookup, "FDN")
	if err != nil {
		t.Fatalf("knobs variant: %v", err)
	}
	d2, err := newDrafter("knob:cfm=8;sig=2", lookup, "FDN")
	if err != nil {
		t.Fatalf("knob variant: %v", err)
	}
	k1, ok1 := d1.(*pickbot.Knobbed)
	k2, ok2 := d2.(*pickbot.Knobbed)
	if !ok1 || !ok2 {
		t.Fatalf("drafters are %T and %T, want *pickbot.Knobbed", d1, d2)
	}
	// Precondition: both forms really carry the knobs (so equality below
	// is not two silent zero tunings).
	if k1.Tuning.ColourFitMax != 8 || k1.Tuning.SignalWeight != 2 {
		t.Fatalf("file tuning cfm %v sig %v, want 8/2", k1.Tuning.ColourFitMax, k1.Tuning.SignalWeight)
	}
	if !reflect.DeepEqual(k1.Tuning, k2.Tuning) {
		t.Fatalf("tunings differ:\n%#v\n%#v", k1.Tuning, k2.Tuning)
	}
	packs, err := lab.OpenPacks("FDN", "", 11, 1)
	if err != nil {
		t.Fatal(err)
	}
	v := pickbot.PickView{Set: "FDN", Round: 1, PickNo: 1, Pack: packs[0], Seats: 8}
	if len(v.Pack) < 5 {
		t.Fatalf("precondition: pack has %d cards, want a real pack", len(v.Pack))
	}
	s1 := k1.Scores(v, nil)
	s2 := k2.Scores(v, nil)
	if !slices.Equal(s1, s2) {
		t.Fatalf("scores differ over the same pack:\n%v\n%v", s1, s2)
	}
	if _, err := newDrafter("nope:1", lookup, "FDN"); err == nil {
		t.Fatal("unknown variant: want an error, got none")
	}
}

func TestFourSeatPodsDraftAndPair(t *testing.T) {
	lab := openLab(t)
	lookup := func(name string) *cards.Card { c, _ := lab.Card(name); return c }
	nb, err := parseBuilder(lab, "balanced", "FDN")
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{
		lab: lab, lookup: lookup, set: "FDN", seed: 5, games: 1,
		variants: []string{"committed", "random"}, bnames: []string{"balanced"},
		builders: []draftlab.NamedBuilder{nb}, seats: 4,
	}
	dir := t.TempDir()
	if err := h.draftChunk(chunkPlan{First: 0, Last: 2}, dir); err != nil {
		t.Fatal(err)
	}
	decks, err := os.ReadDir(filepath.Join(dir, "decks"))
	if err != nil {
		t.Fatal(err)
	}
	if len(decks) != 8 {
		t.Fatalf("wrote %d deck files, want 8", len(decks))
	}
	b, err := os.ReadFile(filepath.Join(dir, "decks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	type dk struct {
		ID    string `json:"id"`
		Pod   int    `json:"pod"`
		Seat  int    `json:"seat"`
		Label string `json:"label"`
	}
	byID := map[string]dk{}
	var dks []dk
	for line := range strings.SplitSeq(string(b), "\n") {
		if line == "" {
			continue
		}
		var d dk
		if err := json.Unmarshal([]byte(line), &d); err != nil {
			t.Fatalf("decks.jsonl %q: %v", line, err)
		}
		if d.Seat >= 4 {
			t.Fatalf("deck %s has seat %d, want < 4", d.ID, d.Seat)
		}
		byID[d.ID] = d
		dks = append(dks, d)
	}
	if len(dks) != 8 {
		t.Fatalf("decks.jsonl holds %d decks, want 8", len(dks))
	}
	pb, err := os.ReadFile(filepath.Join(dir, "pairs.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for line := range strings.SplitSeq(string(pb), "\n") {
		if line == "" {
			continue
		}
		n++
		f := strings.Split(line, "\t")
		if len(f) != 3 {
			t.Fatalf("pair line %q: %d fields, want 3", line, len(f))
		}
		a, bok := byID[f[0]]
		d2, dok := byID[f[1]]
		if !bok || !dok {
			t.Fatalf("pair line %q names unknown decks", line)
		}
		if a.Pod != d2.Pod {
			t.Fatalf("pair %s vs %s crosses pods %d and %d", f[0], f[1], a.Pod, d2.Pod)
		}
		if a.Label == d2.Label {
			t.Fatalf("pair %s vs %s has the same label %s", f[0], f[1], a.Label)
		}
	}
	if n == 0 {
		t.Fatal("pairs.tsv is empty: the 2-variant 4-seat pods should pair every deck")
	}
}
