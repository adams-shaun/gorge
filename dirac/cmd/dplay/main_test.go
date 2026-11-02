package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/oracle"
)

// actionHash is a game's action list as one number.
func actionHash(acts []dirac.Action) uint64 {
	h := fnv.New64a()
	for _, a := range acts {
		fmt.Fprintf(h, "%08x", uint32(a))
	}
	return h.Sum64()
}

// sliceDecks extracts the committed FDN slice and returns its directory.
func sliceDecks(t *testing.T) string {
	t.Helper()
	dir, remove, err := oracle.ExtractSlice("fdn")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(remove)
	return dir
}

// TestDplayMatchesKfuzzFollower plays the slice's first pairs with random seats through
// dplay, and the same seeds through the call kfuzz -lead kernel -pass-elision makes, with
// gorge following and the leader answering by dplay's random rule. Gorge following without
// a divergence and the two action lists being equal make dplay's game a verified one.
func TestDplayMatchesKfuzzFollower(t *testing.T) {
	set := cardsettest.Set(t, "fdn")
	cards, err := cardset.CorpusDir()
	if err != nil {
		t.Fatal(err)
	}
	dir := sliceDecks(t)
	o := &options{set: "fdn", a: seatRandom, b: seatRandom, mullPolicy: "coin", maxTurns: 100, maxIntents: 20000, firstPair: 0, pairs: 60,
		seed: oracle.SliceSeed, pairsFile: filepath.Join(dir, "pairs.tsv"), decks: filepath.Join(dir, "decks"),
		explicit: map[string]bool{"pairs": true}}
	ps, decksDir, remove, err := o.dealtPairs()
	defer remove()
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != o.pairs {
		t.Fatalf("%d pairs, want %d", len(ps), o.pairs)
	}
	decks, err := readDecks(set, ps, decksDir)
	if err != nil {
		t.Fatal(err)
	}
	corp, err := oracle.OpenCorpus(cards, decksDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, config := range []string{configAutopay, configFull} {
		t.Run(config, func(t *testing.T) {
			o.config = config
			r, err := newRunner(o, set, ps, decks, decksDir, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			w := r.newWorker()
			w.record = true
			for i := range ps {
				p := &ps[i]
				var got row
				w.play(p, &got)
				if got.err != "" || got.refused != "" {
					t.Fatalf("pair %d: %+v", p.Index, got)
				}
				want, res := kfuzzLeg(t, set, corp, p, config)
				if res.Div != nil {
					t.Fatalf("pair %d: gorge did not follow: %v", p.Index, res.Div)
				}
				if actionHash(w.acts) != actionHash(want) {
					t.Fatalf("pair %d: %d actions with hash %x, the lockstep leg %d with hash %x", p.Index, len(w.acts), actionHash(w.acts), len(want), actionHash(want))
				}
				if got.stall == "" && got.winner != res.Winner || (got.stall == "") != (res.Stall == "") {
					t.Fatalf("pair %d: winner %d stall %q, the lockstep leg %d %q", p.Index, got.winner, got.stall, res.Winner, res.Stall)
				}
			}
		})
	}
}

// kfuzzLeg plays p led by dirac with gorge following, under dplay's random rule, and
// returns the actions dirac took.
func kfuzzLeg(t *testing.T, set cardset.Set, corp *oracle.Corpus, p *oracle.Pairing, config string) ([]dirac.Action, oracle.Result) {
	t.Helper()
	m, tbl, err := oracle.SetMatchup(set, corp, p.A, p.B, 0)
	if err != nil {
		t.Fatal(err)
	}
	cfg := oracle.Config{PassElision: true, GorgeAutopaySurface: config == configAutopay}
	lead, err := oracle.SetSide(set, tbl, cfg)(m, p.Seed)
	if err != nil {
		t.Fatal(err)
	}
	follow, err := oracle.NewGorgeSide(m, p.Seed, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	rnd := dirac.NewRandomPolicy(p.Seed ^ randomSalt)
	var acts []dirac.Action
	pol := oracle.PolicyFunc(func(*oracle.PickCtx) oracle.Canon {
		_, d, opts := oracle.KernelStateOf(lead)
		a := rnd.Pick(d)
		acts = append(acts, a)
		for i, x := range d.Options[:d.N] {
			if x == a {
				return opts[i]
			}
		}
		panic("the random rule picked no option")
	})
	return acts, oracle.LockstepLed(m, p.Seed, lead, follow, pol, oracle.RunConfig{Config: cfg, MaxDecisions: 20000, MaxTurns: 100})
}

// refusedCard is a card set blb will not play, with the reason it gives, and a card it
// plays.
func refusedCard(t *testing.T) (card, reason, good string) {
	t.Helper()
	set := cardsettest.Set(t, "blb")
	supported := set.Supported()
	for i, n := range set.Names()[:set.NumCards()] {
		switch {
		case !supported[i] && card == "":
			card, reason = n, set.Reason(i)
		case supported[i] && good == "":
			good = n
		}
	}
	if card == "" || good == "" {
		t.Fatalf("blb refuses %q and plays %q: a test needs both", card, good)
	}
	return card, reason, good
}

// writeDeck writes a deck of 60 copies of card and returns its path.
func writeDeck(t *testing.T, dir, name, card string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"name": name, "cards": []map[string]any{{"name": card, "count": 60}}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name+".json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDplayRefusesARefusedCardByName(t *testing.T) {
	card, reason, good := refusedCard(t)
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run([]string{"-set", "blb", "-a", seatRandom, "-b", seatRandom, "-pairs", "1",
		"-deck-a", writeDeck(t, dir, "good", good), "-deck-b", writeDeck(t, dir, "bad", card)}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit %d, want 1; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), fmt.Sprintf("%q", card)) || !strings.Contains(stderr.String(), reason) {
		t.Fatalf("stderr does not name %q and why (%s): %s", card, reason, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("a refused pair played: %s", stdout.String())
	}
}

func TestDplaySkipRefusedCounts(t *testing.T) {
	card, _, good := refusedCard(t)
	dir := t.TempDir()
	writeDeck(t, dir, "good", good)
	writeDeck(t, dir, "bad", card)
	pairs := filepath.Join(dir, "pairs.tsv")
	if err := os.WriteFile(pairs, []byte("good\tgood\t1\ngood\tbad\t2\nbad\tgood\t3\nbad\tbad\t4\ngood\tgood\t5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	sumPath := filepath.Join(dir, "summary.json")
	var stderr bytes.Buffer
	code := run([]string{"-set", "blb", "-a", seatRandom, "-b", seatRandom, "-skip-refused", "-decks", dir, "-pairs-file", pairs, "-summary", sumPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	raw, err := os.ReadFile(sumPath)
	if err != nil {
		t.Fatal(err)
	}
	var s summary
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	if s.Games != 2 || s.RefusedPairs != 3 || s.RefusedCards[card] != 4 {
		t.Fatalf("games %d, refused pairs %d, %q in %d decks; want 2, 3 and 4", s.Games, s.RefusedPairs, card, s.RefusedCards[card])
	}
	rows := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(rows) != 2 || strings.Contains(stdout.String(), `"bad"`) {
		t.Fatalf("rows: %s", stdout.String())
	}
}

func TestDplaySampleReplayReexecutesInGorge(t *testing.T) {
	cardsettest.Require(t, "fdn")
	cards, err := cardset.CorpusDir()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	var stderr bytes.Buffer
	code := run([]string{"-pairs", "10", "-a", seatRandom, "-b", seatRandom, "-sample-replay", "1", "-out", filepath.Join(dir, "games.jsonl"), "-summary", filepath.Join(dir, "s.json")}, io.Discard, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	files, err := filepath.Glob(filepath.Join(dir, "games.jsonl.replays", "*.json"))
	if err != nil || len(files) != 10 {
		t.Fatalf("%d records: %v", len(files), err)
	}
	corp, err := oracle.OpenCorpus(cards, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		g, err := oracle.ReadKernelGame(f)
		if err != nil {
			t.Fatal(err)
		}
		if r := oracle.Replay(corp.Reg, g); r.DivergedAt != -1 || r.Verified != nil {
			t.Errorf("%s: diverged at %d (%s: %s), verified %v", f, r.DivergedAt, r.Reason, r.Detail, r.Verified)
		}
	}
}

// TestDplayFlagsDocumented holds the flag table in dirac/docs/running.md to the flags.
func TestDplayFlagsDocumented(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "running.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, table, ok := strings.Cut(string(raw), "## Playing games with dplay")
	if !ok {
		t.Fatal("running.md has no dplay section")
	}
	table, _, _ = strings.Cut(table, "\n## ")
	documented := map[string]bool{}
	for _, m := range regexp.MustCompile("(?m)^\\| `-([a-z-]+)`").FindAllStringSubmatch(table, -1) {
		documented[m[1]] = true
	}
	o := options{}
	o.flags(io.Discard).VisitAll(func(f *flag.Flag) {
		if !documented[f.Name] {
			t.Errorf("-%s is not in running.md's flag table", f.Name)
		}
		delete(documented, f.Name)
	})
	for name := range documented {
		t.Errorf("running.md documents -%s, which dplay does not have", name)
	}
}
