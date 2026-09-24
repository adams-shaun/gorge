package main

import (
	"bytes"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/policynet"
	"github.com/adams-shaun/gorge/internal/testutil"
)

// TestOnPolicyCorpusLeavesTheBenchUnchanged pins -onpolicy-corpus (ticket
// pn13): the bench report is byte-identical with and without it, the corpus
// is byte-identical across worker counts, every record loads back through
// policynet.LoadOnPolicy with an outcome, and the flag's front door refuses
// a run without a policynet side or with an existing destination.
func TestOnPolicyCorpusLeavesTheBenchUnchanged(t *testing.T) {
	dir := corpusDirOrSkip(t)
	defer func(m *policynet.Model, k []decision.Kind, p string) {
		policynetModel, policynetKinds, onpolicyCorpusPath = m, k, p
	}(policynetModel, policynetKinds, onpolicyCorpusPath)
	m := policynet.NewModel(policynet.TableRows, 16, 8, rand.New(rand.NewPCG(8, 9)))
	m.ResidualW = 0.5
	policynetModel = m
	policynetKinds = []decision.Kind{decision.KAttackers, decision.KPriority}
	pairs, err := parsePairs("mono-red-prowess:mono-green-stompy,mono-white-equipment:mono-black-aggro", testutil.RepoDeckNames())
	if err != nil {
		t.Fatal(err)
	}
	bench := func(workers int, corpus string) []byte {
		onpolicyCorpusPath = corpus
		var b bytes.Buffer
		if err := runMatrix(500, 3, 2, "policynet", "bot", dir, "json", pairs, workers, 200, 20000, false, nil, &b, io.Discard); err != nil {
			t.Fatalf("runMatrix(workers %d, corpus %q): %v", workers, corpus, err)
		}
		return b.Bytes()
	}
	tmp := t.TempDir()
	plain := bench(2, "")
	c1, c4 := filepath.Join(tmp, "w1.jsonl"), filepath.Join(tmp, "w4.jsonl")
	if got := bench(1, c1); !bytes.Equal(got, plain) {
		t.Fatal("-onpolicy-corpus changed the bench report")
	}
	bench(4, c4)
	a, _ := os.ReadFile(c1)
	b, _ := os.ReadFile(c4)
	if len(a) == 0 || !bytes.Equal(a, b) {
		t.Fatalf("corpus differs across worker counts (%d vs %d bytes)", len(a), len(b))
	}
	exs, recs, stats, err := policynet.LoadOnPolicy(c1)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Records == 0 || stats.WithOutcome != stats.Records || len(exs) != len(recs) {
		t.Fatalf("stats %+v", stats)
	}
	for i, r := range recs {
		if r.Pair != pairs[r.PairIndex].String() || r.Deck != [2]string{pairs[r.PairIndex].a, pairs[r.PairIndex].b}[r.Seat] {
			t.Fatalf("record %d: pair/deck metadata %+v", i, r)
		}
		if got := m.Score(exs[i].State, exs[i].Options); !slicesEqual(got, exs[i].PPO.OldScores) {
			t.Fatalf("record %d: the recorded scores are not the checkpoint's", i)
		}
	}
	if err := checkOnPolicyDestination(c1); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("existing destination: %v", err)
	}
	onpolicyCorpusPath = filepath.Join(tmp, "x.jsonl")
	if code := mainExit("bot", "bot", 1, 0, 2, 0, "mono-red-prowess:mono-green-stompy", "constructed", "json", 0,
		200, 20000, dir, "", false, false, "", 0, 0, "", "", "", "", ""); code == 0 {
		t.Fatal("-onpolicy-corpus without a policynet side must exit non-zero")
	}
}

func slicesEqual(a, b []float32) bool {
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

// TestSampledCollectionAndOppMix pins ticket pn14's collection flags:
// -policynet-temperature makes the policynet seat sample (every record
// carries its temperature and behaviour log-probability, the corpus is a
// pure function of the flags across worker counts, and the play differs
// from the greedy run), -opp-mix swaps side B per game as a pure function of
// the game seed, and the parser refuses bad specs.
func TestSampledCollectionAndOppMix(t *testing.T) {
	dir := corpusDirOrSkip(t)
	defer func(m *policynet.Model, k []decision.Kind, p string, temp float64, mix []oppMixEntry) {
		policynetModel, policynetKinds, onpolicyCorpusPath, policynetTemperature, oppMix = m, k, p, temp, mix
	}(policynetModel, policynetKinds, onpolicyCorpusPath, policynetTemperature, oppMix)
	m := policynet.NewModel(policynet.TableRows, 16, 8, rand.New(rand.NewPCG(8, 9)))
	m.ResidualW = 0.5
	policynetModel = m
	policynetKinds = []decision.Kind{decision.KAttackers, decision.KPriority}
	pairs, err := parsePairs("mono-red-prowess:mono-green-stompy", testutil.RepoDeckNames())
	if err != nil {
		t.Fatal(err)
	}
	bench := func(workers int, corpus string) []byte {
		onpolicyCorpusPath = corpus
		var b bytes.Buffer
		if err := runMatrix(700, 4, 2, "policynet", "bot", dir, "json", pairs, workers, 200, 20000, false, nil, &b, io.Discard); err != nil {
			t.Fatalf("runMatrix: %v", err)
		}
		return b.Bytes()
	}
	tmp := t.TempDir()
	policynetTemperature, oppMix = 0, nil
	greedy := bench(2, filepath.Join(tmp, "g.jsonl"))
	policynetTemperature = 1
	if oppMix, err = parseOppMix("explore:0.5"); err != nil {
		t.Fatal(err)
	}
	s1, s3 := filepath.Join(tmp, "s1.jsonl"), filepath.Join(tmp, "s3.jsonl")
	sampled := bench(1, s1)
	if again := bench(3, s3); !bytes.Equal(again, sampled) {
		t.Fatal("sampled bench report differs across worker counts")
	}
	a, _ := os.ReadFile(s1)
	b, _ := os.ReadFile(s3)
	if len(a) == 0 || !bytes.Equal(a, b) {
		t.Fatal("sampled corpus differs across worker counts")
	}
	g, _ := os.ReadFile(filepath.Join(tmp, "g.jsonl"))
	if bytes.Contains(g, []byte("logp_behaviour")) || bytes.Equal(g, a) || bytes.Equal(greedy, sampled) {
		t.Fatal("greedy and sampled runs must differ, and only the sampled one carries behaviour fields")
	}
	_, recs, stats, err := policynet.LoadOnPolicy(s1)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Sampled != stats.Records || stats.Records == 0 {
		t.Fatalf("stats %+v", stats)
	}
	for _, r := range recs {
		if r.Temperature != 1 || r.LogPBehaviour == nil || *r.LogPBehaviour > 0 {
			t.Fatalf("record without a valid behaviour log-probability: %+v", r)
		}
	}
	mixed := 0
	for s := uint64(0); s < 1000; s++ {
		if p := oppMixPick(s); p != oppMixPick(s) {
			t.Fatal("oppMixPick is not a pure function of the seed")
		} else if p == "explore" {
			mixed++
		}
	}
	if mixed < 400 || mixed > 600 {
		t.Fatalf("explore:0.5 picked %d of 1000 games", mixed)
	}
	for _, bad := range []string{"explore", "explore:0", "explore:1.5", "nosuch:0.2", "policynet:0.2", "explore:0.6,legacy:0.6"} {
		if _, err := parseOppMix(bad); err == nil {
			t.Fatalf("parseOppMix(%q) accepted", bad)
		}
	}
}
