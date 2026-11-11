package azmcts

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// commanderZoneSeed is the first game seed (from testSeed) at which the default
// bot meets a commander-zone ask in foundations-tramplesaurus-rex vs
// foundations-wretched-ranks (re-measured for the official FDC lists: seed
// testSeed itself has none).
const commanderZoneSeed = testSeed + 3

// commanderTestConfig is a two-seat Commander game: the decks' resolved
// cards, each file's commander indices, the format's 40 life and seed.
func commanderTestConfig(t testing.TB, a, b string, seed uint64) rules.Config {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	da, err := testutil.LoadRepoDeck(reg, a)
	if err != nil {
		t.Fatal(err)
	}
	db, err := testutil.LoadRepoDeck(reg, b)
	if err != nil {
		t.Fatal(err)
	}
	fa, err := testutil.LoadRepoDeckFile(a)
	if err != nil {
		t.Fatal(err)
	}
	fb, err := testutil.LoadRepoDeckFile(b)
	if err != nil {
		t.Fatal(err)
	}
	return rules.Config{
		Seed:         seed,
		Names:        []string{a, b},
		Decks:        [][]*cards.Card{da, db},
		Commanders:   [][]int{fa.CommanderIndices(), fb.CommanderIndices()},
		Tokens:       reg.Tokens,
		Format:       rules.FormatCommander,
		StartingLife: 40,
	}
}

// commanderZonePosition plays the default bot from genesis to the pinned
// commander-zone ask and returns the engine there with the bot's answer,
// unsubmitted.
func commanderZonePosition(t testing.TB) (*rules.Engine, *decision.Decision, decision.Intent) {
	t.Helper()
	e, d, bot, ok := botDecision(
		commanderTestConfig(t, "foundations-tramplesaurus-rex", "foundations-wretched-ranks", commanderZoneSeed),
		20000,
		func(d *decision.Decision) bool { return d.Kind == decision.KCommanderZone },
	)
	if !ok {
		t.Fatalf("seed %d: no commander-zone ask", commanderZoneSeed)
	}
	return e, d, bot
}

// TestCommanderZoneEnumeratesBothAnswers pins the searched kind's candidates
// at a real ask: the bot's answer first, then the other option, both legal.
// FullKinds must not search it, or every existing search would change.
func TestCommanderZoneEnumeratesBothAnswers(t *testing.T) {
	e, d, bot := commanderZonePosition(t)
	if d.Min != 1 || d.Max != 1 || len(d.Options) != 2 {
		t.Fatalf("commander-zone ask is min/max %d/%d over %d options, want 1/1 over 2", d.Min, d.Max, len(d.Options))
	}
	if d.Options[0].Kind != "command_zone" || d.Options[1].Kind != "leave" {
		t.Fatalf("commander-zone options are %q then %q, want command_zone then leave", d.Options[0].Kind, d.Options[1].Kind)
	}
	if len(bot.Choices) != 1 {
		t.Fatalf("the bot's answer is %v, want one option", bot.Choices)
	}
	chosen := bot.Choices[0]

	cands, kind, ok := enumerate(searchprobe.NewCollector(d.Player), e, d, bot, Kinds{CommanderZone: true}, 6)
	if !ok || kind != "commander_zone" {
		t.Fatalf("kind %q ok %v, want commander_zone searched", kind, ok)
	}
	if len(cands) != 2 {
		t.Fatalf("%d candidates, want exactly 2", len(cands))
	}
	if !reflect.DeepEqual(cands[0].in, bot) {
		t.Fatalf("candidate 0 is %+v, want the bot's answer %+v", cands[0].in, bot)
	}
	if got := cands[1].in.Choices; len(got) != 1 || got[0] == chosen {
		t.Fatalf("candidate 1 is %v, want the other option than %d", got, chosen)
	}
	for i, c := range cands {
		if err := d.Validate(c.in); err != nil {
			t.Fatalf("candidate %d is invalid: %v", i, err)
		}
	}
	if cands[0].key == cands[1].key {
		t.Fatal("both candidates have the same key")
	}

	for _, kinds := range []Kinds{AllKinds(), FullKinds()} {
		if _, kind, ok := enumerate(searchprobe.NewCollector(d.Player), e, d, bot, kinds, 6); ok || kind != "" {
			t.Fatalf("searched as %q under kinds %+v, want not searched", kind, kinds)
		}
	}
}

// TestSearchWeighsTheCommanderZone runs the tree at the pinned commander-zone
// ask: every simulation completes, the answer is one of the two options, the
// result is deterministic, and the live engine is never written.
func TestSearchWeighsTheCommanderZone(t *testing.T) {
	e, d, bot := commanderZonePosition(t)
	if d.Kind != decision.KCommanderZone || len(d.Options) != 2 ||
		d.Options[0].Kind != "command_zone" || d.Options[1].Kind != "leave" {
		t.Fatalf("pinned decision is %s over %d options, want a two-option commander-zone ask", d.Kind, len(d.Options))
	}
	head := e.L.Head()

	opts := DefaultOptions()
	opts.Sims, opts.Seed, opts.Kinds = 32, 41, Kinds{CommanderZone: true}
	res := searchAt(t, e, d, bot, nil, opts)
	if res.Kind != "commander_zone" {
		t.Fatalf("result kind %q, want commander_zone", res.Kind)
	}
	st := res.Stats
	if st.Searched != 1 || st.Simulations != 32 || st.Completed != 32 || st.AllFailed != 0 {
		t.Fatalf("stats %+v, want 32 completed simulations", st)
	}
	if st.Panics+st.SubmitErrors+st.BadWorlds+st.NoWorld+st.ChanceFailures != 0 {
		t.Fatalf("failures: panics %d, submit %d, bad world %d, no world %d, chance %d",
			st.Panics, st.SubmitErrors, st.BadWorlds, st.NoWorld, st.ChanceFailures)
	}
	if len(res.Candidates) != 2 {
		t.Fatalf("%d candidates, want exactly 2", len(res.Candidates))
	}
	if len(res.Intent.Choices) != 1 {
		t.Fatalf("chosen intent %+v, want one option", res.Intent)
	}
	if c := res.Intent.Choices[0]; c < 0 || c >= len(d.Options) {
		t.Fatalf("chosen option %d, want one of the two offered", c)
	}
	if err := d.Validate(res.Intent); err != nil {
		t.Fatalf("chosen intent is invalid: %v", err)
	}

	again := searchAt(t, e, d, bot, nil, opts)
	if diff := sameResult(res, again); diff != "" || again.Stats != res.Stats {
		t.Fatalf("the search is not deterministic: %s", diff)
	}
	if e.L.Head() != head {
		t.Fatal("the search wrote the live engine")
	}
}
