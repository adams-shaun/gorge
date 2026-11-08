package pickbot_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	pb "github.com/adams-shaun/gorge/draft/limited/pickbot"
)

func TestChampionTuningIsDocumented(t *testing.T) {
	want := pb.Tuning{ColourFitMax: 8, SignalWeight: 2, Bias: [5]float64{0, 0, -2, -2, 3}}
	got := pb.ChampionTuning()
	if got != want {
		t.Fatalf("ChampionTuning = %+v, want %+v", got, want)
	}
	parsed, err := pb.ParseResearchKnobs("cfm=8;bias=G3/B-2/R-2;sig=2")
	if err != nil || parsed != got {
		t.Fatalf("parsed knobs = %+v %v, want %+v", parsed, err, got)
	}
	if got.Hash() != pb.ChampionTuning().Hash() {
		t.Fatal("Hash not stable")
	}
	got.SignalWeight = 99
	if pb.ChampionTuning() != want {
		t.Fatal("mutating the returned tuning changed the next call")
	}
}

func TestChampionPersonaResolves(t *testing.T) {
	lookup := lookupOf(corpus(t))
	d, b, ok := pb.New("champion", lookup)
	if !ok {
		t.Fatal("champion not registered")
	}
	k, isK := d.(*pb.Knobbed)
	if !isK || k.Ratings == nil || k.Tuning != pb.ChampionTuning() {
		t.Fatalf("drafter = %T %+v", d, d)
	}
	b23, isB := b.(*pb.Best23)
	if !isB || b23.Ratings == nil || b23.Tuning != (pb.Tuning{}) {
		t.Fatalf("builder = %T %+v", b, b)
	}
	have := map[string]bool{}
	for _, n := range pb.Names() {
		have[n] = true
	}
	for _, n := range []string{"champion", "rating-colour", "best23", "balanced"} {
		if !have[n] {
			t.Errorf("Names lacks %s", n)
		}
	}
	d2, b2, _ := pb.New("rating-colour", lookup)
	if rc, ok := d2.(*pb.RatingColour); !ok || rc == nil {
		t.Errorf("rating-colour drafter = %T", d2)
	}
	if bb, ok := b2.(*pb.Best23); !ok || bb.Ratings != nil {
		t.Errorf("rating-colour builder = %T", b2)
	}
	r := pb.ChampionRatings()
	if !(r.Value("Bake into a Pie") > r.Value("Aegis Turtle")) {
		t.Error("Bake into a Pie should outrate Aegis Turtle")
	}
	lo, hi := 2.0, -2.0
	for n := range pb.Champion().Cards {
		v := r.Value(n)
		lo, hi = min(lo, v), max(hi, v)
	}
	m := r.Value("no such card")
	if m != r.Value("Plains") || !(lo < m && m < hi) {
		t.Errorf("missing value %v not strictly inside (%v, %v) or differs from Plains %v", m, lo, hi, r.Value("Plains"))
	}
}

func TestChampionDraftDeterministic(t *testing.T) {
	lookup := lookupOf(corpus(t))
	run := func(name string) ([]string, []string, [][]string) {
		picks, decks, pools := knobsPod(t, 20261106,
			func() pb.Drafter { d, _, _ := pb.New(name, lookup); return d }, mustBuilder(name, lookup))
		ids := make([][]string, len(pools))
		for i, p := range pools {
			for _, c := range p {
				ids[i] = append(ids[i], c.ID)
			}
		}
		return picks, decks, ids
	}
	p1, d1, o1 := run("champion")
	p2, d2, o2 := run("champion")
	if !reflect.DeepEqual(p1, p2) || !reflect.DeepEqual(d1, d2) || !reflect.DeepEqual(o1, o2) {
		t.Fatal("champion pod not deterministic")
	}
	p3, _, _ := run("rating-colour")
	if reflect.DeepEqual(p1, p3) {
		t.Fatal("champion picks equal the default persona's; precondition broken")
	}
}

func mustBuilder(name string, lookup pb.CardLookup) pb.Builder {
	_, b, _ := pb.New(name, lookup)
	return b
}

func TestChampionValuesValidate(t *testing.T) {
	v, err := pb.ParseChampionValues(pb.ChampionFile())
	if err != nil || len(v.Cards) != 270 {
		t.Fatalf("parse: %d cards, %v", len(v.Cards), err)
	}
	if _, err := v.Ratings(); err != nil {
		t.Fatal(err)
	}
	enc := func(mut func(*pb.ChampionValues)) []byte {
		c := v
		c.Cards = map[string]pb.ChampionCard{}
		for k, x := range v.Cards {
			c.Cards[k] = x
		}
		mut(&c)
		b, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	firstCard := func() string {
		names := make([]string, 0)
		for n := range v.Cards {
			names = append(names, n)
		}
		sort.Strings(names)
		return names[0]
	}()
	good := enc(func(*pb.ChampionValues) {})
	cases := []struct {
		name string
		in   []byte
		want string
	}{
		{"extra key", bytes.Replace(good, []byte("{"), []byte(`{"extra":1,`), 1), "extra"},
		{"trailing", append(append([]byte(nil), good...), []byte(" {}")...), "trailing"},
		{"set", enc(func(c *pb.ChampionValues) { c.Set = "BLB" }), "set"},
		{"persona", enc(func(c *pb.ChampionValues) { c.Persona = "x" }), "persona"},
		{"metric", enc(func(c *pb.ChampionValues) { c.Metric = "gih" }), "metric"},
		{"license", enc(func(c *pb.ChampionValues) { c.License = "MIT" }), "license"},
		{"attribution", enc(func(c *pb.ChampionValues) { c.Attribution = "someone" }), "attribution"},
		{"provenance", enc(func(c *pb.ChampionValues) { c.Provenance = "" }), "provenance"},
		{"cards", enc(func(c *pb.ChampionValues) {
			for n := range c.Cards {
				if len(c.Cards) == 199 {
					break
				}
				delete(c.Cards, n)
			}
		}), "cards"},
		{"unknown card", enc(func(c *pb.ChampionValues) { c.Cards["Not A Card"] = pb.ChampionCard{DV: 0.5, N: 5} }), "Not A Card"},
		{"dv", enc(func(c *pb.ChampionValues) { c.Cards[firstCard] = pb.ChampionCard{DV: 1.5, N: 5} }), "dv"},
		{"n", enc(func(c *pb.ChampionValues) { c.Cards[firstCard] = pb.ChampionCard{DV: 0.5, N: 0} }), "n"},
	}
	for _, tc := range cases {
		_, err := pb.ParseChampionValues(tc.in)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want containing %q", tc.name, err, tc.want)
		}
	}
	if _, err := pb.ParseChampionValues(good); err != nil {
		t.Fatalf("re-marshalled good table refused: %v", err)
	}
}

func TestChampionValuesNamesResolveFDNSetList(t *testing.T) {
	in := map[string]bool{}
	for _, n := range fdnMainSetNames(t) {
		in[n] = true
	}
	for n := range pb.Champion().Cards {
		if !in[n] {
			t.Errorf("%q is not on the FDN set list", n)
		}
	}
}

func TestChampionValuesDigest(t *testing.T) {
	c := pb.Champion().Cards
	if len(c) != 270 {
		t.Fatalf("%d cards, want 270", len(c))
	}
	names := make([]string, 0, len(c))
	for n := range c {
		names = append(names, n)
	}
	sort.Strings(names)
	var sb strings.Builder
	for _, n := range names {
		fmt.Fprintf(&sb, "%s\t%.12f\n", n, c[n].DV)
	}
	sum := sha256.Sum256([]byte(sb.String()))
	const want = "5a681e520590ccd2aad24fcfcb2ddd5faff8dfc82792f71dba8b916978480929"
	if got := hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("digest = %s, want %s", got, want)
	}
}

func TestChampionPersonaSetGuard(t *testing.T) {
	if err := pb.CheckPersonaSet("champion", "FDN"); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"BLB", "TRK"} {
		err := pb.CheckPersonaSet("champion", s)
		if err == nil || !strings.Contains(err.Error(), "champion") || !strings.Contains(err.Error(), "FDN") {
			t.Errorf("set %s: err = %v", s, err)
		}
	}
	if pb.CheckPersonaSet("rating-colour", "BLB") != nil || pb.CheckPersonaSet("knobs:/x.json", "TRK") != nil {
		t.Error("other personas must pass any set")
	}
	if pb.PersonaSet("champion") != "FDN" || pb.PersonaSet("best23") != "" {
		t.Error("PersonaSet wrong")
	}
}
