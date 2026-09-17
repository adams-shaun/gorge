package rules

import (
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

func chanceConfig(t *testing.T) Config {
	t.Helper()
	return Config{Seed: 73, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 12), mountainDeck(t, 12)}}
}

// Losing the opt-in boundary must not silently change ordinary replay.
func TestHypotheticalUnconditionedMatchesNormal(t *testing.T) {
	cfg := chanceConfig(t)
	normal := New(cfg)
	hyp, err := NewHypothetical(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if hyp.L.Head() != normal.L.Head() || hyp.RNGDraws() != normal.RNGDraws() {
		t.Fatal("unconditioned hypothetical changed normal genesis")
	}
	if got := normal.ChanceTranscript(); got != nil {
		t.Fatal("normal game recorded hypothetical transcript")
	}
	if got := hyp.ChanceTranscript(); len(got) != 23 || got[0].Bound != 2 || got[1].Bound != 12 {
		t.Fatalf("missing toss or genesis shuffle draws: %v", got)
	}
}

// A forced shuffle must happen during genesis, not via fabricated later moves.
func TestHypotheticalForcesGenesisAndOwnsPrefix(t *testing.T) {
	cfg := chanceConfig(t)
	prefix := []ChanceDraw{{Bound: 2, Value: 1}}
	for p := 0; p < 2; p++ {
		for n := 12; n > 1; n-- {
			prefix = append(prefix, ChanceDraw{Bound: n, Value: n - 1})
		}
	}
	e, err := NewHypothetical(cfg, prefix)
	if err != nil {
		t.Fatal(err)
	}
	if e.G.StartingPlayer != 1 {
		t.Fatalf("starter = %d", e.G.StartingPlayer)
	}
	// Identity permutations deal the first seven physical cards to each seat.
	if got := e.G.Zone(state.ZHand, 0); !reflect.DeepEqual(got, []state.ObjID{1, 2, 3, 4, 5, 6, 7}) {
		t.Fatalf("hand = %v", got)
	}
	prefix[0].Value = 0
	got := e.ChanceTranscript()
	if got[0].Value != 1 {
		t.Fatal("caller prefix aliases the transcript")
	}
	got[0].Value = 0
	if e.ChanceTranscript()[0].Value != 1 {
		t.Fatal("returned transcript aliases the engine")
	}
}

func TestHypotheticalRejectsMalformedChance(t *testing.T) {
	for _, tc := range []struct {
		name string
		draw ChanceDraw
		want string
	}{
		{"zero bound", ChanceDraw{Bound: 0, Value: 0}, "bound"},
		{"negative value", ChanceDraw{Bound: 2, Value: -1}, "value"},
		{"large value", ChanceDraw{Bound: 2, Value: 2}, "value"},
		{"wrong site", ChanceDraw{Bound: 3, Value: 1}, "bound"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, err := NewHypothetical(chanceConfig(t), []ChanceDraw{tc.draw})
			if err == nil || !strings.Contains(err.Error(), tc.want) || e != nil {
				t.Fatalf("got engine=%v error=%v", e != nil, err)
			}
		})
	}
}

func TestHypotheticalReplayAndCloneOwnChanceState(t *testing.T) {
	cfg := chanceConfig(t)
	cfg.Mulligans = 1
	e, err := NewHypothetical(cfg, []ChanceDraw{{Bound: 2, Value: 0}})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.AdvanceHypothetical(); err != nil {
		t.Fatal(err)
	}
	clone := e.Clone()
	d := clone.Pending()
	choice := -1
	for i, o := range d.Options {
		if o.Kind == "mulligan" {
			choice = i
		}
	}
	if choice < 0 {
		t.Fatalf("no mulligan: %+v", d)
	}
	intent := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{choice}}
	head, draws := e.L.Head(), e.RNGDraws()
	if err := clone.SubmitHypothetical(intent); err != nil {
		t.Fatal(err)
	}
	if clone.RNGDraws() <= draws {
		t.Fatal("mulligan did not consume chance")
	}
	if e.L.Head() != head || e.RNGDraws() != draws || len(e.ChanceTranscript()) != int(draws) {
		t.Fatal("clone changed source log or chance state")
	}
	if err := e.SubmitHypothetical(intent); err != nil {
		t.Fatal(err)
	}
	if e.L.Head() != clone.L.Head() || !reflect.DeepEqual(e.ChanceTranscript(), clone.ChanceTranscript()) {
		t.Fatal("clone lost chance position")
	}
	replay, err := NewHypothetical(cfg, e.ChanceTranscript())
	if err != nil {
		t.Fatal(err)
	}
	if err := replay.AdvanceHypothetical(); err != nil {
		t.Fatal(err)
	}
	if err := replay.SubmitHypothetical(intent); err != nil {
		t.Fatal(err)
	}
	if replay.L.Head() != e.L.Head() || replay.RNGDraws() != e.RNGDraws() {
		t.Fatal("chance+intent replay diverged")
	}
}

func TestHypotheticalSubmitFailurePoisonsOnlyThatBranch(t *testing.T) {
	cfg := chanceConfig(t)
	cfg.Mulligans = 1
	base, err := NewHypothetical(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := base.AdvanceHypothetical(); err != nil {
		t.Fatal(err)
	}
	prefix := append(base.ChanceTranscript(), ChanceDraw{Bound: 999, Value: 0})
	e, err := NewHypothetical(cfg, prefix)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.AdvanceHypothetical(); err != nil {
		t.Fatal(err)
	}
	d := e.Pending()
	choice := -1
	for i, o := range d.Options {
		if o.Kind == "mulligan" {
			choice = i
		}
	}
	if choice < 0 {
		t.Fatal("missing mulligan")
	}
	in := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{choice}}
	if err := e.SubmitHypothetical(in); err == nil || !strings.Contains(err.Error(), "bound") {
		t.Fatalf("error = %v", err)
	}
	head := e.L.Head()
	if err := e.SubmitHypothetical(in); err == nil {
		t.Fatal("accepted input after chance failure")
	}
	if head != e.L.Head() {
		t.Fatal("failed branch kept mutating")
	}
	if err := base.SubmitHypothetical(in); err != nil {
		t.Fatal(err)
	}
	if err := New(cfg).SubmitHypothetical(in); err == nil {
		t.Fatal("hypothetical entry accepted a normal engine")
	}
}

// A full tape must restore not just the recorded prefix but the same seeded
// continuation. Skipping the PCG advance at forced draws breaks that property.
func TestHypotheticalFullTapePreservesRandomContinuation(t *testing.T) {
	cfg := chanceConfig(t)
	original, err := NewHypothetical(cfg, []ChanceDraw{{Bound: 2, Value: 1}})
	if err != nil {
		t.Fatal(err)
	}
	replay, err := NewHypothetical(cfg, original.ChanceTranscript())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 40; i++ {
		if a, b := original.Rand(1000+i), replay.Rand(1000+i); a != b {
			t.Fatalf("continuation %d: %d != %d", i, a, b)
		}
	}
}

// Exercise Host.Rand inside a real resolving Shuffle spell, not only the
// engine's separate genesis/mulligan Fisher-Yates loops.
func TestHypotheticalEffectsShuffleReplaysAndRejectsWrongBound(t *testing.T) {
	cfg := chanceConfig(t)
	spell := card(t, "Name:Hypothesis Shuffle\nManaCost:0\nTypes:Sorcery\nA:SP$ Shuffle | Defined$ You\nOracle:Test shuffle.\n")
	for i := range cfg.Decks[0] {
		cfg.Decks[0][i] = spell
	}
	e, err := NewHypothetical(cfg, []ChanceDraw{{Bound: 2, Value: 0}})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.AdvanceHypothetical(); err != nil {
		t.Fatal(err)
	}
	genesisDraws := e.RNGDraws()
	cast := false
	for i := 0; i < 100 && e.RNGDraws() == genesisDraws; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("unexpected decision %+v", d)
		}
		choice := -1
		for j, o := range d.Options {
			if o.Kind == "pass" {
				choice = j
			}
		}
		if !cast {
			for j, o := range d.Options {
				if o.Kind == "cast" {
					choice = j
					cast = true
					break
				}
			}
		}
		if choice < 0 {
			t.Fatal("missing cast/pass")
		}
		if err := e.SubmitHypothetical(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{choice}}); err != nil {
			t.Fatal(err)
		}
	}
	if !cast || e.RNGDraws() != genesisDraws+4 {
		t.Fatalf("effect shuffle draws=%d, genesis=%d, cast=%v", e.RNGDraws(), genesisDraws, cast)
	}
	tape := e.ChanceTranscript()
	replay, err := NewHypothetical(cfg, tape)
	if err != nil {
		t.Fatal(err)
	}
	if err := replay.AdvanceHypothetical(); err != nil {
		t.Fatal(err)
	}
	for _, in := range e.L.Intents {
		if err := replay.SubmitHypothetical(in); err != nil {
			t.Fatal(err)
		}
	}
	if replay.L.Head() != e.L.Head() || replay.RNGDraws() != e.RNGDraws() {
		t.Fatal("effects chance replay diverged")
	}
	tape[genesisDraws].Bound++
	bad, err := NewHypothetical(cfg, tape)
	if err != nil {
		t.Fatal(err)
	}
	if err := bad.AdvanceHypothetical(); err != nil {
		t.Fatal(err)
	}
	for _, in := range e.L.Intents {
		if err = bad.SubmitHypothetical(in); err != nil {
			break
		}
	}
	if err == nil || !strings.Contains(err.Error(), "bound") {
		t.Fatalf("effects error=%v", err)
	}
}
