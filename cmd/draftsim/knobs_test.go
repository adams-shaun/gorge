package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited/pickbot"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
	"github.com/adams-shaun/gorge/engine/cards"
)

// writeKnobs writes one knob file into a fresh temp dir and returns its path.
func writeKnobs(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// deckNames is each seat's built deck as card names, seat for seat.
func deckNames(decks [][]*cards.Card) [][]string {
	out := make([][]string, len(decks))
	for i, d := range decks {
		for _, c := range d {
			out[i] = append(out[i], c.Faces[0].Name)
		}
	}
	return out
}

// TestDraftsimKnobFilePersona: a knobs:<path> value drafts with the file's
// Knobbed and builds with its Best23, the event header names the file and
// its hash, and the zero knob file drafts and builds exactly as the
// balanced drafter with the best23 builder.
func TestDraftsimKnobFilePersona(t *testing.T) {
	dir := corpusReady(t)
	path := writeKnobs(t, "k.json", `{"colour_fit_max": 8, "signal_weight": 2}`)
	want := pickbot.Tuning{ColourFitMax: 8, SignalWeight: 2}
	spec := knobsPrefix + path

	d, b, err := personaPair(spec, spec, nil)
	if err != nil {
		t.Fatalf("personaPair: %v", err)
	}
	kd, ok := d.(*pickbot.Knobbed)
	if !ok {
		t.Fatalf("drafter is %T, want *pickbot.Knobbed", d)
	}
	if !reflect.DeepEqual(kd.Tuning, want) {
		t.Fatalf("drafter Tuning %+v, want %+v", kd.Tuning, want)
	}
	if _, ok := b.(*pickbot.Best23); !ok {
		t.Fatalf("builder is %T, want *pickbot.Best23", b)
	}

	var out strings.Builder
	o := options{set: "FDN", seed: 1, events: 1, players: "bot,bot,bot,bot",
		drafter: spec + "@" + want.Hash(), builder: spec, dir: dir}
	res, err := run(o, &out)
	if err != nil {
		t.Fatalf("run with knob files: %v", err)
	}
	if len(res.events) != 1 || len(res.events[0].decks) != 4 {
		t.Fatalf("knob run produced %d events, want 1 with 4 decks", len(res.events))
	}
	header := "drafter " + spec + "@" + want.Hash() + " knobs@" + want.Hash() + ", builder " + spec + " knobs@" + want.Hash()
	if !strings.Contains(out.String(), header) {
		t.Fatalf("event header does not name the knob files; want %q in:\n%s", header, firstLine(out.String()))
	}

	zero := knobsPrefix + writeKnobs(t, "zero.json", `{}`)
	zres, err := run(options{set: "FDN", seed: 1, events: 1, players: "bot,bot,bot,bot",
		drafter: zero, builder: zero, dir: dir}, &strings.Builder{})
	if err != nil {
		t.Fatalf("run with the zero knob file: %v", err)
	}
	bres, err := run(options{set: "FDN", seed: 1, events: 1, players: "bot,bot,bot,bot",
		drafter: "balanced", builder: "best23", dir: dir}, &strings.Builder{})
	if err != nil {
		t.Fatalf("run balanced/best23: %v", err)
	}
	zd, bd := deckNames(zres.events[0].decks), deckNames(bres.events[0].decks)
	if len(zd) != 4 || len(zd[0]) == 0 {
		t.Fatalf("zero knob run built %d decks (first %d cards), want 4 non-empty", len(zd), len(zd[0]))
	}
	for s := range bd {
		if !reflect.DeepEqual(zd[s], bd[s]) {
			t.Fatalf("seat %d: zero knob deck differs from balanced/best23:\n knobs %v\n named %v", s, zd[s], bd[s])
		}
	}
	// Precondition for the equality meaning anything: the cfm-8/sig-2 file
	// builds a different pod, so the decks do depend on the knobs.
	if reflect.DeepEqual(deckNames(res.events[0].decks), bd) {
		t.Fatal("the cfm 8 / sig 2 knob file built the same decks as balanced; the comparison cannot fail")
	}
}

// TestDraftsimKnobFileRefusals: a missing file, an unknown key, an invalid
// knob set and a wrong @<hash> are refused naming the file or key, before
// the corpus opens.
func TestDraftsimKnobFileRefusals(t *testing.T) {
	noCorpus := filepath.Join(t.TempDir(), "no-corpus")
	good := writeKnobs(t, "good.json", `{"colour_fit_max": 8}`)
	cases := []struct {
		name, drafter, builder, want string
	}{
		{"missing file", knobsPrefix + filepath.Join(t.TempDir(), "absent.json"), "best23", "absent.json"},
		{"unknown key", knobsPrefix + writeKnobs(t, "unk.json", `{"colour_fit": 8}`), "best23", "colour_fit"},
		{"invalid set", "rating-colour", knobsPrefix + writeKnobs(t, "bad.json", `{"deck_size": 41}`), "deck_size"},
		{"wrong hash", knobsPrefix + good + "@000000000000", "best23", "000000000000"},
	}
	// Precondition: a valid knob file with the same missing corpus fails
	// at the corpus, so the refusals below come before it.
	if _, err := run(options{set: "FDN", seed: 1, events: 1, players: "bot", drafter: knobsPrefix + good, builder: "best23", dir: noCorpus}, &strings.Builder{}); err == nil || !strings.Contains(err.Error(), "opening corpus") {
		t.Fatalf("valid knob file with no corpus: err %v, want the corpus refusal", err)
	}
	for _, c := range cases {
		_, err := run(options{set: "FDN", seed: 1, events: 1, players: "bot", drafter: c.drafter, builder: c.builder, dir: noCorpus}, &strings.Builder{})
		if err == nil {
			t.Fatalf("%s: run accepted it", c.name)
		}
		if strings.Contains(err.Error(), "opening corpus") || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s: err %q, want a refusal naming %q before the corpus opens", c.name, err, c.want)
		}
	}
}

// TestDraftsimRatingsFileWithKnobDrafter: -ratings-file reaches a knob
// file's Knobbed drafter and Best23 builder; a balanced drafter is still
// refused.
func TestDraftsimRatingsFileWithKnobDrafter(t *testing.T) {
	spec, err := setdata.Load("FDN")
	if err != nil {
		t.Fatal(err)
	}
	names := sheetNames(spec)
	if len(names) < 2 {
		t.Fatalf("FDN's sheets name %d cards, want at least 2", len(names))
	}
	body, _ := json.Marshal(map[string]map[string]float64{names[0]: {"gih": 0.6}, names[1]: {"gih": 0.5}})
	ratings := writeKnobs(t, "ratings.json", string(body))
	knobs := knobsPrefix + writeKnobs(t, "k.json", `{"signal_weight": 2}`)
	o := options{set: "FDN", ratingsFile: ratings, ratingsField: "gih"}

	r := &runner{}
	if r.drafter, r.builder, err = personaPair(knobs, "best23", nil); err != nil {
		t.Fatal(err)
	}
	if err := applyRatingsOverride(o, r, &strings.Builder{}); err != nil {
		t.Fatalf("ratings override with a knob drafter: %v", err)
	}
	if d := r.drafter.(*pickbot.Knobbed); d.Ratings == nil {
		t.Fatal("Knobbed drafter has no Ratings after the override")
	}
	if b := r.builder.(*pickbot.Best23); b.Ratings == nil {
		t.Fatal("Best23 builder has no Ratings after the override")
	}

	r = &runner{}
	if r.drafter, r.builder, err = personaPair("balanced", "best23", nil); err != nil {
		t.Fatal(err)
	}
	if err := applyRatingsOverride(o, r, &strings.Builder{}); err == nil || !strings.Contains(err.Error(), "drafter") {
		t.Fatalf("ratings override with a balanced drafter: err %v, want the drafter refusal", err)
	}
}
