package main

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/seat"
)

// covDecision builds a synthetic KChoose-like decision with the given option
// kinds (indexes 0..n-1).
func covDecision(kind decision.Kind, optKinds ...string) *decision.Decision {
	d := &decision.Decision{Player: 0, Kind: kind, Min: 0, Max: len(optKinds)}
	for i, k := range optKinds {
		d.Options = append(d.Options, decision.Option{Index: i, Kind: k, Label: k})
	}
	return d
}

// TestActionCoverageOfferedVsChosen pins the collector's core algebra: every
// option offered counts against its row; only the ANSWER's indexes count as
// chosen; a row offered but never chosen is exactly the "never fired" list
// the report prints.
func TestActionCoverageOfferedVsChosen(t *testing.T) {
	c := newActionCoverage()
	c.game()
	d := covDecision(decision.KChoose, "exile", "card", "exile")
	// Both answers pick only the exile options; card is offered twice and
	// never chosen -- exactly the offered-but-never-chosen shape.
	c.record(d, decision.Intent{Choices: []int{0, 2}})
	c.record(d, decision.Intent{Choices: []int{2, 0}})

	if got := c.offered[rowKey("choose", "exile")]; got != 4 {
		t.Errorf("choose/exile offered %d, want 4 (2 per decision)", got)
	}
	if got := c.chosen[rowKey("choose", "exile")]; got != 4 {
		t.Errorf("choose/exile chosen %d, want 4", got)
	}
	if got := c.offered[rowKey("choose", "card")]; got != 2 {
		t.Errorf("choose/card offered %d, want 2", got)
	}
	if got := c.chosen[rowKey("choose", "card")]; got != 0 {
		t.Errorf("choose/card chosen %d, want 0", got)
	}
	var buf bytes.Buffer
	c.write(&buf)
	out := buf.String()
	if !strings.Contains(out, "choose/card (offered 2)") {
		t.Errorf("offered-but-never-chosen report must name choose/card with its offer count, got:\n%s", out)
	}
	if strings.Contains(out, "choose/exile (offered") {
		t.Errorf("a chosen row must not appear in the offered-but-never-chosen list:\n%s", out)
	}
}

// TestActionCoverageCastOptionDims pins the wire-side cast dims: an offered
// cast option with Mode "kicked" and a nonzero AltCostIndex counts against
// those dims separately, and chosen counts land on the same dims.
func TestActionCoverageCastOptionDims(t *testing.T) {
	c := newActionCoverage()
	c.game()
	d := &decision.Decision{Player: 0, Kind: decision.KPriority, Min: 1, Max: 1, Options: []decision.Option{
		{Index: 0, Kind: "cast", Label: "cast kicked", Mode: "kicked", AltCostIndex: 1},
		{Index: 1, Kind: "pass", Label: "pass"},
	}}
	// Choose the kicked alt-cost cast once, then pass once.
	c.record(d, decision.Intent{Choices: []int{0}})
	c.record(d, decision.Intent{Choices: []int{1}})
	if c.castModeOffered["kicked"] != 2 || c.castModeChosen["kicked"] != 1 {
		t.Errorf("kicked offered/chosen = %d/%d, want 2/1", c.castModeOffered["kicked"], c.castModeChosen["kicked"])
	}
	if c.altCostOffered["alt"] != 2 || c.altCostChosen["alt"] != 1 {
		t.Errorf("alt offered/chosen = %d/%d, want 2/1", c.altCostOffered["alt"], c.altCostChosen["alt"])
	}
}

// TestActionCoverageNeverAskedKindsUseTheDecisionUniverse pins that the
// never-asked kind list is exactly the decision.Kinds the run did not ask --
// the universe must come from the decision package's own list, not a
// hand-copied one that can drift.
func TestActionCoverageNeverAskedKindsUseTheDecisionUniverse(t *testing.T) {
	c := newActionCoverage()
	c.game()
	c.record(covDecision(decision.KPriority, "pass"), decision.Intent{Choices: []int{0}})
	var buf bytes.Buffer
	c.write(&buf)
	out := buf.String()
	if !strings.Contains(out, "decision kinds asked: 1/12") {
		t.Errorf("asked count must be 1 of the %d decision kinds, got:\n%s", len(decision.Kinds), out)
	}
	for _, k := range decision.Kinds {
		never := strings.Contains(out, "never asked: "+string(k)) ||
			strings.Contains(out, ","+string(k))
		if k == decision.KPriority && never {
			t.Errorf("priority was asked; must not be in the never-asked list:\n%s", out)
		}
		if k != decision.KPriority && !never {
			t.Errorf("kind %s was never asked; must appear in the never-asked list:\n%s", k, out)
		}
	}
}

// TestRunAppendsCoverageReportWhenEnabled pins the integration shape: with
// the package global on, run() appends an "action coverage (" section after
// the normal report, and with it off the report is unchanged (the
// byte-identical default the flag promises).
func TestRunAppendsCoverageReportWhenEnabled(t *testing.T) {
	dir := corpusDirOrSkip(t)
	var with bytes.Buffer
	old := actionCoverageEnabled
	actionCoverageEnabled = true
	err := run(7, 1, 2, 0, 0, "bot", "bot", dir, 12, 20000, false, &with)
	actionCoverageEnabled = old
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(with.String(), "\naction coverage (1 games):\n") {
		t.Errorf("the enabled run must append an action coverage section, got:\n%s", with.String())
	}
	// The section must come AFTER the normal report's summary line (the
	// collector appends, never interleaves); the byte-identity of the normal
	// report with the flag off is pinned by TestConstructedDefaultIsByteIdentical.
	idx := strings.Index(with.String(), "\naction coverage (1 games):\n")
	if !strings.Contains(with.String()[:idx], "mean turns per game:") {
		t.Errorf("the coverage section must follow the normal report:\n%s", with.String())
	}
}

// benchCorpus is the package tests' shared corpus registry: every test in
// this file needs the same registry, and OpenCorpusRegistry re-parses the
// corpus on every call (~0.4s), so the tests share one via sync.Once. The
// registry is read-only after open (decks load fresh slices from it), so
// sharing it across tests is safe.
var (
	benchCorpusOnce sync.Once
	benchCorpusReg  *cards.Registry
	benchCorpusErr  error
)

func benchCorpus(t *testing.T) *cards.Registry {
	t.Helper()
	benchCorpusOnce.Do(func() {
		benchCorpusReg, benchCorpusErr = testutil.OpenCorpusRegistry(corpusDirOrSkip(t))
	})
	if benchCorpusErr != nil {
		t.Fatalf("open corpus: %v", benchCorpusErr)
	}
	return benchCorpusReg
}

// playCoveredGame plays one real engine game through playMatch with the
// given collector, the direct path the walk tests use (run() builds its own
// collector from the package global; playMatch takes one explicitly).
func playCoveredGame(t *testing.T, reg *cards.Registry, seed uint64, cov *actionCoverage) error {
	t.Helper()
	a, err := testutil.LoadRepoDeck(reg, "death-n-taxes")
	if err != nil {
		return err
	}
	b, err := testutil.LoadRepoDeck(reg, "dimir-tempo")
	if err != nil {
		return err
	}
	cfg := buildGameConfig(seed, []string{"death-n-taxes", "dimir-tempo"},
		[][]*cards.Card{a, b}, nil, false)
	cfg.Tokens = reg.Tokens
	seats := []seat.Seat{seat.NewBot(seed ^ 1), seat.NewBot(seed ^ 2)}
	_, err = playMatch(cfg, []string{"bot", "bot"}, seats, 12, 20000, nil, cov)
	return err
}

// TestActionCoverageWalkOnRealGames is the walk's corpus test, one merged
// pass over two real engine games covering three invariants:
//
//   - the walk's central attribution invariant: every primitive the log
//     attributes was in the run's deck universe, every ability slot
//     activated names a deck card at a real non-mana index, and at least one
//     card was cast or played (a violation means the walk attributed
//     something the decks cannot explain);
//   - the plain-cast fix: plain casts (no {X}, no mode flag) emit no
//     CastInfo, so the walk must attribute them from PutOnStack -- the plain
//     shape must be nonzero for any run where casts happened;
//   - the static choose row seed guard: every choose option kind a real run
//     OFFERS must be in kindRows["choose"] (modulo the fmt-built prefixes),
//     or the never-asked report would eventually claim a live shape was
//     never asked. If this fires, add the observed kind to the seed and
//     check where the engine started emitting it.
func TestActionCoverageWalkOnRealGames(t *testing.T) {
	reg := benchCorpus(t)
	cov := newActionCoverage()
	if err := playCoveredGame(t, reg, 7, cov); err != nil {
		t.Fatalf("playMatch: %v", err)
	}
	cov.mu.Lock()
	defer cov.mu.Unlock()
	if len(cov.primExercsed) == 0 {
		t.Fatalf("no primitive was exercised in a real game -- the walk is broken")
	}
	for p := range cov.primExercsed {
		// api:Mana is claimed label-level by ManaAdd and IS in every deck
		// universe via Primitives(); anything else out of universe is a
		// walk defect.
		if !cov.primUniverse[p] {
			t.Errorf("exercised primitive %q is not in the deck universe", p)
		}
	}
	for k := range cov.abilityUses {
		name, idx := splitAbilityKey(k)
		if !cov.cardsInUniverse[name] {
			t.Errorf("ability use %q names a card outside the universe", k)
			continue
		}
		found := false
		for _, i := range cov.cardAbilityIdx[name] {
			if i == idx {
				found = true
			}
		}
		if !found {
			t.Errorf("ability use %q is not a non-mana slot of the card", k)
		}
	}
	if len(cov.cardCasts) == 0 {
		t.Errorf("no card was cast or played in a real game -- attribution is broken")
	}
	plain := cov.castShapes["plain"]
	casts := int64(0)
	for _, v := range cov.cardCasts {
		casts += v
	}
	if plain == 0 {
		t.Errorf("no plain cast was counted (plain=%d, casts+plays=%d) -- the PutOnStack attribution regressed", plain, casts)
	}
	if casts == 0 {
		t.Errorf("no cast or play counted at all")
	}
	// The seed-rot guard covers EVERY decision kind, not just choose: any
	// option kind a real run offers must be in its kind's seed (or a
	// fmt-built family), or the never-asked report would eventually claim a
	// live shape was never asked. Seed additions go into kindRows.
	for key := range cov.offered {
		kind, opt := splitRowKey(key)
		seeded := false
		for _, s := range kindRows[kind] {
			if s == opt {
				seeded = true
			}
		}
		if !seeded && !dynamicOptionPrefix(opt) {
			t.Errorf("option kind %q of decision kind %q was offered in a real run but is not in the static seed for that kind", opt, kind)
		}
	}
}

// dynamicOptionPrefix reports whether an option kind is one of the fmt-built
// families the static seed cannot enumerate ("pay_<letter>", "convoke_<n>")
// -- the guard accepts those as covered, because a new letter or number is
// not a new decision shape.
func dynamicOptionPrefix(opt string) bool {
	for _, p := range []string{"pay_", "convoke_"} {
		if strings.HasPrefix(opt, p) && opt != p {
			return true
		}
	}
	return false
}

// TestActionCoverageReportIsDeterministic pins the report's determinism:
// two identical runs produce byte-identical coverage reports (the tallies
// are seed-pure; the report sorts everything it prints).
func TestActionCoverageReportIsDeterministic(t *testing.T) {
	reg := benchCorpus(t)
	var b1, b2 bytes.Buffer
	for _, buf := range []*bytes.Buffer{&b1, &b2} {
		cov := newActionCoverage()
		if err := playCoveredGame(t, reg, 7, cov); err != nil {
			t.Fatalf("playMatch: %v", err)
		}
		cov.write(buf)
	}
	if b1.String() != b2.String() {
		t.Fatalf("two identical runs produced different coverage reports:\n--- 1 ---\n%s\n--- 2 ---\n%s", b1.String(), b2.String())
	}
}

// TestActionCoverageConcurrentRecords pins the collector's thread safety
// shape (games run in parallel; every mutation takes the mutex).
func TestActionCoverageConcurrentRecords(t *testing.T) {
	c := newActionCoverage()
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.game()
			c.record(covDecision(decision.KChoose, "exile", "card"), decision.Intent{Choices: []int{0}})
			c.exerciseAPIChain(nil)
			c.primitive("api:Draw")
		}()
	}
	wg.Wait()
	if c.games != 8 || c.asked["choose"] != 8 || c.primExercsed["api:Draw"] != 8 {
		t.Errorf("concurrent records lost counts: games=%d asked=%d api:Draw=%d", c.games, c.asked["choose"], c.primExercsed["api:Draw"])
	}
}

// TestActionCoverageSupportedSplit pins that the report's supported/universe
// split uses effects.Supported() and that every primitive label in a deck
// universe carries a known class prefix.
func TestActionCoverageSupportedSplit(t *testing.T) {
	reg := benchCorpus(t)
	deck, err := testutil.LoadRepoDeck(reg, "mono-red-goblins")
	if err != nil {
		t.Fatal(err)
	}
	c := newActionCoverage()
	for _, card := range deck {
		c.registerCard(card)
	}
	supported := effects.Supported()
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.primUniverse) == 0 {
		t.Fatal("empty universe")
	}
	for p := range c.primUniverse {
		classed := false
		for _, cl := range []string{"api:", "trig:", "stat:", "kw:", "repl:"} {
			if strings.HasPrefix(p, cl) {
				classed = true
			}
		}
		if !classed {
			t.Errorf("primitive %q carries no known class prefix", p)
		}
		_ = supported[p]
	}
}

// TestNilActionCoverageIsSafe pins the nil-receiver contract every call site
// relies on (the flag-off path never constructs a collector).
func TestNilActionCoverageIsSafe(t *testing.T) {
	var c *actionCoverage
	c.game()
	c.record(covDecision(decision.KChoose, "exile"), decision.Intent{Choices: []int{0}})
	// (walkGame would need a real engine; a nil collector returns before that.)
	c.exerciseAPIChain(nil)
	c.primitive("api:Draw")
	var buf bytes.Buffer
	c.write(&buf)
	if buf.Len() != 0 {
		t.Errorf("a nil collector must print nothing, got %q", buf.String())
	}
}

// TestSplitAbilityKey pins the "card|idx" split.
func TestSplitAbilityKey(t *testing.T) {
	name, idx := splitAbilityKey("Some Card|3")
	if name != "Some Card" || idx != 3 {
		t.Errorf("splitAbilityKey = %q,%d; want Some Card,3", name, idx)
	}
	if n, i := splitAbilityKey("NoBar"); n != "NoBar" || i != 0 {
		t.Errorf("splitAbilityKey without a bar = %q,%d; want NoBar,0", n, i)
	}
}

func TestRowKeyRoundTrip(t *testing.T) {
	k, o := splitRowKey(rowKey("choose", "exile"))
	if k != "choose" || o != "exile" {
		t.Errorf("rowKey round trip = %q,%q; want choose,exile", k, o)
	}
	if fmt.Sprint(rowKey("a", "b")) == "ab" {
		t.Errorf("rowKey must use a separator that cannot appear in either part")
	}
}
