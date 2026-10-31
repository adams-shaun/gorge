package refagent

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/draftlab/extern/wire"
	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/server/protocol"

	"github.com/adams-shaun/gorge/draft/limited/build"
)

func card(id, name, colors, typeLine string) protocol.DraftCard {
	return protocol.DraftCard{ID: id, Name: name, Set: "FDN", Number: "1", Rarity: "C",
		Colors: colors, TypeLine: typeLine}
}

func gorgeHello() wire.Message {
	return wire.Message{V: wire.Version, Type: wire.THello, Protocol: wire.Protocol,
		Kinds: []string{wire.KindPick, wire.KindBuild, wire.KindDecide}}
}

func decideItem(id, seed uint64, d *decision.Decision) wire.Item {
	view, err := json.Marshal(map[string]any{"decision": d})
	if err != nil {
		panic(err)
	}
	return wire.Item{ID: id, Kind: wire.KindDecide, Seed: seed, Stream: "g1.0", View: view}
}

func priorityDecision(min, max int, options ...decision.Option) *decision.Decision {
	return &decision.Decision{Seq: 7, Player: 1, Kind: decision.KPriority, Prompt: "Your priority.",
		Min: min, Max: max, Options: options}
}

// serve runs Serve on the given config over an io.Pipe pair and returns
// the driver's write and read ends plus Serve's error channel.
func serve(t *testing.T, cfg Config) (io.WriteCloser, io.Reader, <-chan error) {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	errc := make(chan error, 1)
	go func() { errc <- Serve(inR, outW, cfg) }()
	t.Cleanup(func() { inW.Close(); outR.Close() })
	return inW, outR, errc
}

// TestServeHandshakeBatchBye runs a whole session: gorge hello → agent
// hello carrying the Config's fields → a batch of one pick, one build and
// one decide → one result answering every id in order → bye → nil.
func TestServeHandshakeBatchBye(t *testing.T) {
	inW, outR, errc := serve(t, Config{Policy: "random", MaxBatch: 16, ViewOmit: []string{"library"},
		ObserveBatch: func(n int) {
			if n != 3 {
				t.Errorf("ObserveBatch saw %d items, want 3", n)
			}
		},
		ObserveItem: func(it wire.Item) {
			if it.Kind != wire.KindPick && it.Kind != wire.KindBuild && it.Kind != wire.KindDecide {
				t.Errorf("ObserveItem saw kind %q", it.Kind)
			}
		}})
	writeMsg := func(m wire.Message) {
		t.Helper()
		if err := wire.NewWriter(inW).Write(m); err != nil {
			t.Fatalf("write %s: %v", m.Type, err)
		}
	}
	readMsg := func() wire.Message {
		t.Helper()
		m, err := wire.NewReader(outR, 0).Read()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		return m
	}
	writeMsg(gorgeHello())
	hello := readMsg()
	if err := hello.Validate(wire.FromAgent); err != nil {
		t.Fatalf("agent hello invalid: %v", err)
	}
	if hello.Type != wire.THello || hello.Agent != "random" || hello.ModelVersion != "1" ||
		hello.MaxBatch != 16 || !reflect.DeepEqual(hello.ViewOmit, []string{"library"}) {
		t.Errorf("agent hello does not carry the Config's fields: %+v", hello)
	}
	pack := []protocol.DraftCard{card("c1", "Bear", "G", "Creature — Bear"), card("c2", "Bolt", "R", "Instant")}
	pool := []protocol.DraftCard{card("c1", "Bear", "G", "Creature — Bear"), card("c3", "Clue", "", "Artifact")}
	batch := wire.Message{V: wire.Version, Type: wire.TBatch, Batch: 1, Items: []wire.Item{
		{ID: 11, Kind: wire.KindPick, Seed: 1, Set: "FDN", Round: 1, PickNo: 1, Pack: pack},
		{ID: 12, Kind: wire.KindBuild, Seed: 2, Set: "FDN", DeckSize: 40, Pool: pool},
		decideItem(13, 3, priorityDecision(1, 1,
			decision.Option{Index: 0, Kind: "pass", Label: "Pass"},
			decision.Option{Index: 1, Kind: "cast", Label: "Cast Bear"},
			decision.Option{Index: 2, Kind: "concede", Label: "Concede"})),
	}}
	writeMsg(batch)
	result := readMsg()
	if err := result.Validate(wire.FromAgent); err != nil {
		t.Fatalf("agent result invalid: %v", err)
	}
	if result.Type != wire.TResult || result.Batch != 1 {
		t.Fatalf("got %s batch %d, want result batch 1", result.Type, result.Batch)
	}
	if len(result.Answers) != 3 {
		t.Fatalf("%d answers, want 3", len(result.Answers))
	}
	for i, wantID := range []uint64{11, 12, 13} {
		if got := result.Answers[i].ID; got != wantID {
			t.Errorf("answer %d has id %d, want %d (item order)", i, got, wantID)
		}
	}
	a0, a1, a2 := result.Answers[0], result.Answers[1], result.Answers[2]
	if a0.Pick != "c1" && a0.Pick != "c2" {
		t.Errorf("pick answer %q is not in the pack", a0.Pick)
	}
	if a1.Deck == nil || len(a1.Deck.Main) == 0 {
		t.Errorf("build answer carries no deck: %+v", a1)
	}
	if a2.Intent == nil || len(a2.Intent.Choices) != 1 {
		t.Fatalf("decide answer carries no single choice: %+v", a2)
	}
	if a2.Intent.Choices[0] == 2 {
		t.Errorf("random policy conceded")
	}
	if a2.Intent.Seq != 0 || a2.Intent.Player != 0 {
		t.Errorf("agent filled seq/player; gorge owns them")
	}
	writeMsg(wire.Message{V: wire.Version, Type: wire.TBye})
	if err := <-errc; err != nil {
		t.Errorf("Serve returned %v, want nil", err)
	}
}

// TestServeRefusesBadHello answers a wrong-protocol hello with an error
// message and returns an error instead of serving.
func TestServeRefusesBadHello(t *testing.T) {
	inW, outR, errc := serve(t, Config{})
	bad := gorgeHello()
	bad.Protocol = "gorge-agent/2"
	if err := wire.NewWriter(inW).Write(bad); err != nil {
		t.Fatal(err)
	}
	m := readOne(t, outR)
	if m.Type != wire.TError || m.Code != wire.CodeUnsupported {
		t.Errorf("got %s code %q, want error unsupported", m.Type, m.Code)
	}
	if err := <-errc; err == nil {
		t.Error("Serve returned nil on a bad hello")
	}
}

func readOne(t *testing.T, r io.Reader) wire.Message {
	t.Helper()
	m, err := wire.NewReader(r, 0).Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return m
}

// TestServeDieAfter answers the configured number of batches, then ends
// the session with an error when the next batch arrives (before it is
// answered).
func TestServeDieAfter(t *testing.T) {
	inW, outR, errc := serve(t, Config{DieAfter: 1})
	if err := wire.NewWriter(inW).Write(gorgeHello()); err != nil {
		t.Fatal(err)
	}
	readOne(t, outR) // agent hello
	batch := wire.Message{V: wire.Version, Type: wire.TBatch, Batch: 1, Items: []wire.Item{
		{ID: 1, Kind: wire.KindPick, Seed: 1, Set: "FDN", Round: 1, PickNo: 1,
			Pack: []protocol.DraftCard{card("c1", "Bear", "G", "Creature — Bear")}},
	}}
	if err := wire.NewWriter(inW).Write(batch); err != nil {
		t.Fatal(err)
	}
	result := readOne(t, outR)
	if result.Type != wire.TResult || len(result.Answers) != 1 {
		t.Fatalf("batch was not answered: %+v", result)
	}
	// Batch 2 arrives: Serve dies before answering it, so no result is
	// written for it.
	if err := wire.NewWriter(inW).Write(batch); err != nil {
		t.Fatal(err)
	}
	err := <-errc
	if err == nil {
		t.Fatal("Serve returned nil after DieAfter batches")
	}
	if !strings.Contains(err.Error(), "died after 1 batches") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestUnservedKindIsRefused answers an item of a kind outside the Config's
// kinds with a refused error, not with a policy answer.
func TestUnservedKindIsRefused(t *testing.T) {
	inW, outR, errc := serve(t, Config{Kinds: []string{wire.KindDecide}})
	if err := wire.NewWriter(inW).Write(gorgeHello()); err != nil {
		t.Fatal(err)
	}
	readOne(t, outR)
	batch := wire.Message{V: wire.Version, Type: wire.TBatch, Batch: 1, Items: []wire.Item{
		{ID: 1, Kind: wire.KindPick, Seed: 1, Set: "FDN", Round: 1, PickNo: 1,
			Pack: []protocol.DraftCard{card("c1", "Bear", "G", "Creature — Bear")}},
	}}
	if err := wire.NewWriter(inW).Write(batch); err != nil {
		t.Fatal(err)
	}
	result := readOne(t, outR)
	if len(result.Answers) != 1 || result.Answers[0].Error == nil ||
		result.Answers[0].Error.Code != wire.CodeRefused {
		t.Errorf("want one refused answer, got %+v", result.Answers)
	}
	if err := wire.NewWriter(inW).Write(wire.Message{V: wire.Version, Type: wire.TBye}); err != nil {
		t.Fatal(err)
	}
	if err := <-errc; err != nil {
		t.Errorf("Serve: %v", err)
	}
}

// TestRandomPickInPack answers a pick with a pack card, and different
// seeds spread over more than one of them.
func TestRandomPickInPack(t *testing.T) {
	pack := []protocol.DraftCard{
		card("c1", "Bear", "G", "Creature — Bear"), card("c2", "Bolt", "R", "Instant"),
		card("c3", "Wall", "W", "Creature — Wall"), card("c4", "Ox", "", "Creature — Ox"),
		card("c5", "Bird", "U", "Creature — Bird"),
	}
	seen := map[string]bool{}
	for seed := uint64(0); seed < 50; seed++ {
		a := Answer(Config{}, wire.Item{ID: 1, Kind: wire.KindPick, Seed: seed, Pack: pack})
		if a.Error != nil || a.Pick == "" {
			t.Fatalf("seed %d: %+v", seed, a)
		}
		inPack := false
		for _, c := range pack {
			if c.ID == a.Pick {
				inPack = true
			}
		}
		if !inPack {
			t.Fatalf("seed %d: pick %q not in pack", seed, a.Pick)
		}
		seen[a.Pick] = true
	}
	if len(seen) < 2 {
		t.Errorf("50 seeds produced only %d distinct picks; the seed is not mixing", len(seen))
	}
}

// TestRandomBuildValidates builds a deck from a synthetic 45-card pool and
// build.Validate accepts it, converted to build.Deck.
func TestRandomBuildValidates(t *testing.T) {
	var pool []protocol.DraftCard
	var refs []limited.CardRef
	add := func(n int, idPrefix, colors string) {
		for i := 0; i < n; i++ {
			c := card(fmt.Sprintf("%s%d", idPrefix, i), "Card "+idPrefix, colors, "Creature — Beast")
			if colors == "" {
				c.TypeLine = "Artifact"
			}
			pool = append(pool, c)
			refs = append(refs, limited.CardRef{ID: c.ID, Name: c.Name, Set: c.Set, Number: c.Number, Rarity: limited.Common})
		}
	}
	add(15, "w", "W")
	add(15, "u", "U")
	add(15, "r", "R")
	a := Answer(Config{}, wire.Item{ID: 1, Kind: wire.KindBuild, Seed: 42, DeckSize: 40, Pool: pool})
	if a.Deck == nil {
		t.Fatal("no deck in the answer")
	}
	deck := build.Deck{Main: a.Deck.Main, Basics: a.Deck.Basics}
	if err := build.Validate(refs, deck); err != nil {
		t.Fatalf("build.Validate: %v", err)
	}
	if deck.Total() != 40 {
		t.Errorf("deck has %d cards, want 40", deck.Total())
	}
	// The pool's colours tie at 15/15/15, so WUBRG order picks W,U; main is
	// 23 of the 30 matched cards and 17 basics split 9/8 (first gets the
	// ceiling).
	if deck.Basics != [5]int{9, 8, 0, 0, 0} {
		t.Errorf("basics = %v, want [9 8 0 0 0]", deck.Basics)
	}
	if len(deck.Main) != 23 {
		t.Errorf("main has %d cards, want 23", len(deck.Main))
	}
}

// TestRandomDecideNeverConcedes answers a priority decision over 200
// seeds: never the concede index, always a legal count of distinct in-range
// option indexes.
func TestRandomDecideNeverConcedes(t *testing.T) {
	// Priority: exactly one choice, never the conceded option.
	d := priorityDecision(1, 1,
		decision.Option{Index: 0, Kind: "pass", Label: "Pass"},
		decision.Option{Index: 1, Kind: "cast", Label: "Cast"},
		decision.Option{Index: 2, Kind: "concede", Label: "Concede"})
	pickedCast := false
	for seed := uint64(0); seed < 200; seed++ {
		a := Answer(Config{}, decideItem(1, seed, d))
		if a.Intent == nil {
			t.Fatalf("seed %d: %+v", seed, a)
		}
		if len(a.Intent.Choices) != 1 {
			t.Fatalf("seed %d: choices %v, want exactly one", seed, a.Intent.Choices)
		}
		if a.Intent.Choices[0] == 2 {
			t.Fatalf("seed %d: conceded", seed)
		}
		if a.Intent.Choices[0] == 1 {
			pickedCast = true
		}
	}
	if !pickedCast {
		t.Error("200 seeds never chose the cast option; the seed is not mixing")
	}
	// A wider decision: the count stays within [Min, Max], the indexes stay
	// distinct and in range.
	wide := priorityDecision(1, 3,
		decision.Option{Index: 0, Kind: "a"}, decision.Option{Index: 1, Kind: "b"},
		decision.Option{Index: 2, Kind: "c"}, decision.Option{Index: 3, Kind: "d"})
	counts := map[int]bool{}
	for seed := uint64(0); seed < 200; seed++ {
		a := Answer(Config{}, decideItem(1, seed, wide))
		got := a.Intent.Choices
		if len(got) < 1 || len(got) > 3 {
			t.Fatalf("seed %d: %d choices, want 1..3", seed, len(got))
		}
		seen := map[int]bool{}
		for _, c := range got {
			if c < 0 || c > 3 {
				t.Fatalf("seed %d: index %d out of range", seed, c)
			}
			if seen[c] {
				t.Fatalf("seed %d: duplicate index %d", seed, c)
			}
			seen[c] = true
		}
		counts[len(got)] = true
	}
	if !counts[1] || !counts[2] || !counts[3] {
		t.Errorf("choice counts over 200 seeds: %v; want 1, 2 and 3 all reachable", counts)
	}
}

// TestAnswerIsPureFunctionOfItem answers the same item twice and gets
// identical bytes — the seed alone drives the rng.
func TestAnswerIsPureFunctionOfItem(t *testing.T) {
	it := decideItem(9, 777, priorityDecision(1, 2,
		decision.Option{Index: 0, Kind: "pass"}, decision.Option{Index: 1, Kind: "cast"},
		decision.Option{Index: 2, Kind: "concede"}))
	first, err := json.Marshal(Answer(Config{}, it))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		again, err := json.Marshal(Answer(Config{}, it))
		if err != nil {
			t.Fatal(err)
		}
		if string(first) != string(again) {
			t.Fatalf("answer %d differs:\n %s\n %s", i, again, first)
		}
	}
}

// TestFirstPolicyPasses passes priority, picks the first pack card and
// takes the first Min options elsewhere.
func TestFirstPolicyPasses(t *testing.T) {
	cfg := Config{Policy: "first"}
	d := priorityDecision(1, 1,
		decision.Option{Index: 0, Kind: "pass", Label: "Pass"},
		decision.Option{Index: 1, Kind: "cast", Label: "Cast"},
		decision.Option{Index: 2, Kind: "concede", Label: "Concede"})
	a := Answer(cfg, decideItem(1, 5, d))
	if a.Intent == nil || !reflect.DeepEqual(a.Intent.Choices, []int{0}) {
		t.Errorf("priority: %+v, want choices [0]", a)
	}
	pack := []protocol.DraftCard{card("c1", "Bear", "G", "Creature — Bear"), card("c2", "Bolt", "R", "Instant")}
	if a := Answer(cfg, wire.Item{ID: 2, Kind: wire.KindPick, Seed: 5, Pack: pack}); a.Pick != "c1" {
		t.Errorf("pick: %q, want the first pack card", a.Pick)
	}
	target := &decision.Decision{Seq: 1, Kind: decision.KTarget, Min: 1, Max: 1, Options: []decision.Option{
		{Index: 0, Kind: "card", Label: "Bear"}, {Index: 1, Kind: "card", Label: "Bolt"}}}
	if a := Answer(cfg, decideItem(3, 5, target)); !reflect.DeepEqual(a.Intent.Choices, []int{0}) {
		t.Errorf("target: %+v, want choices [0]", a)
	}
	// The first policy's build matches the random policy's shape and stays
	// legal.
	var pool []protocol.DraftCard
	var refs []limited.CardRef
	for i := 0; i < 15; i++ {
		c := card(fmt.Sprintf("w%d", i), "W", "W", "Creature — Beast")
		pool = append(pool, c)
		refs = append(refs, limited.CardRef{ID: c.ID, Name: c.Name, Set: c.Set, Number: c.Number, Rarity: limited.Common})
	}
	for i := 0; i < 15; i++ {
		c := card(fmt.Sprintf("u%d", i), "U", "U", "Creature — Beast")
		pool = append(pool, c)
		refs = append(refs, limited.CardRef{ID: c.ID, Name: c.Name, Set: c.Set, Number: c.Number, Rarity: limited.Common})
	}
	aDeck := Answer(cfg, wire.Item{ID: 4, Kind: wire.KindBuild, Seed: 5, DeckSize: 40, Pool: pool})
	deck := build.Deck{Main: aDeck.Deck.Main, Basics: aDeck.Deck.Basics}
	if err := build.Validate(refs, deck); err != nil {
		t.Errorf("first policy build: %v", err)
	}
}

// TestIllegalPolicyIsIllegal answers every kind with an answer the
// validator (or the pool) refuses — the test half of the refusal path.
func TestIllegalPolicyIsIllegal(t *testing.T) {
	cfg := Config{Policy: "illegal"}
	pack := []protocol.DraftCard{card("c1", "Bear", "G", "Creature — Bear")}
	if a := Answer(cfg, wire.Item{ID: 1, Kind: wire.KindPick, Seed: 1, Pack: pack}); a.Pick != "no-such-card" {
		t.Errorf("pick: %q, want no-such-card", a.Pick)
	}
	if a := Answer(cfg, wire.Item{ID: 2, Kind: wire.KindBuild, Seed: 1, DeckSize: 40, Pool: pack}); a.Deck == nil ||
		len(a.Deck.Main) != 0 || a.Deck.Basics != [5]int{} {
		t.Errorf("build: %+v, want an empty deck", a.Deck)
	}
	d := priorityDecision(1, 1,
		decision.Option{Index: 0, Kind: "pass"}, decision.Option{Index: 1, Kind: "cast"},
		decision.Option{Index: 2, Kind: "concede"})
	if a := Answer(cfg, decideItem(3, 1, d)); a.Intent == nil ||
		!reflect.DeepEqual(a.Intent.Choices, []int{3}) {
		t.Errorf("decide: %+v, want choices [3] (one past the max index)", a)
	}
}

// The empty-config default policy is random: its decide answers never
// concede (the same assertion the served path relies on).
func TestDefaultPolicyIsRandom(t *testing.T) {
	d := priorityDecision(1, 1,
		decision.Option{Index: 0, Kind: "pass"}, decision.Option{Index: 1, Kind: "cast"},
		decision.Option{Index: 2, Kind: "concede"})
	a := Answer(Config{}, decideItem(1, 3, d))
	if a.Intent == nil || a.Intent.Choices[0] == 2 {
		t.Errorf("default policy answered %+v; want a random non-concede choice", a)
	}
}
