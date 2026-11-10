package paymirror

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestRecordedFindingsMirror (TestRecordedFindingsMirrorChunkNN, sharded per
// findings_chunks_test.go) replays recorded paymirror finding games
// (testdata/findings_recorded.jsonl: repo deck names or random card-name lists, no
// script text) end to end: no cast is a mismatch, no planned cast falls back
// or misses its witness, the live-vs-clone control is equivalent, and each
// named cast gets its root-caused verdict. A seq moves whenever a rule change
// shifts a game's log or the bots' trajectory; a move keeps the card and the
// verdict.
//
//   - 12468, random4: a clean, control-equivalent game with an empty pin. Its
//     finding, Veiled Crocodile's CR 603.8 state trigger remembering the land
//     tapped in run A's CR 601.2g window, is pinned by
//     rules/state_trigger_remembered_test.go.
//   - 11828 (Vorinclex, Voice of Hunger), random2: the plan's Fanatic of Rhonas
//     step held only while Vorinclex was in hand; CR 601.2a moves it to the stack
//     first. The offer proves every planned step with the card on the stack
//     (paymentPlanHoldsOnStack), so no planned cast in the game falls back or
//     mismatches.
//   - 12603 seq 4634 (Ashnod's Altar), commander4: cast from exile under Party
//     Thrasher, which grants convoke only to a spell cast from exile, so the plan
//     prices all of {3}; the seat's convoke tap then has the engine plan the cast
//     again (CR 702.51a), and run A pays by that witness (Report.ARearm).
//     Equivalent. The game no longer reaches its earlier finding, Treasonous
//     Ogre's life payment read past an opponent's speed gain (CR 702.179d,
//     lifePaymentConsequences), which TestWitnessReadsLifePaymentPastSpeedGain pins.
//   - 10877 seq 2842 (Infernal Plunge), commander4: the plan's Ogre paid life for R
//     and was then the creature sacrificed for the spell's additional cost
//     (CR 601.2h), which the witness must not read as its activation
//     (activationProduction). Equivalent.
//
// recordedWant holds those pins.
var recordedWant = map[uint64]map[uint64]string{ // seed -> seq -> verdict key ("" = equivalent)
	12468: {},
	11828: {},
	12603: {4634: ""},
	10877: {2842: ""},
}

const (
	recordedFile   = "testdata/findings_recorded.jsonl"
	recordedChunks = 4
)

func TestRecordedFindingsMirrorChunk00(t *testing.T) { runRecordedChunk(t, 0) }
func TestRecordedFindingsMirrorChunk01(t *testing.T) { runRecordedChunk(t, 1) }
func TestRecordedFindingsMirrorChunk02(t *testing.T) { runRecordedChunk(t, 2) }
func TestRecordedFindingsMirrorChunk03(t *testing.T) { runRecordedChunk(t, 3) }

// TestRecordedFindingsMirrorChunksPartitionTheSpecs: the chunks play every
// spec exactly once (checkFindingPartition).
func TestRecordedFindingsMirrorChunksPartitionTheSpecs(t *testing.T) {
	checkFindingPartition(t, findingSpecs(t, recordedFile, 4), recordedChunks, []func(*testing.T){
		TestRecordedFindingsMirrorChunk00, TestRecordedFindingsMirrorChunk01,
		TestRecordedFindingsMirrorChunk02, TestRecordedFindingsMirrorChunk03})
}

// runRecordedChunk is one shard of TestRecordedFindingsMirror.
func runRecordedChunk(t *testing.T, i int) {
	t.Parallel()
	d := findingDecks(t)
	for _, spec := range findingChunk(findingSpecs(t, recordedFile, 4), recordedChunks, i) {
		reports := findingGame(t, d, spec)
		seen := map[uint64]bool{}
		for _, r := range reports {
			st, key := r.Verdict()
			if st == Mismatch || (st == Unmirrorable && !r.ExpectedUnmirrorable()) {
				t.Errorf("seed %d seq %d %q: %s %s", spec.Seed, r.Seq, r.Card, st, key)
			}
			if r.AFallback != "" || r.AWitness != "" {
				t.Errorf("seed %d seq %d %q: fallback %q witness %q", spec.Seed, r.Seq, r.Card, r.AFallback, r.AWitness)
			}
			if r.Control == nil || r.Control.Status != Equivalent {
				t.Errorf("seed %d seq %d %q: control %+v", spec.Seed, r.Seq, r.Card, r.Control)
			}
			w, ok := recordedWant[spec.Seed][r.Seq]
			if !ok {
				continue
			}
			seen[r.Seq] = true
			if key != w {
				t.Errorf("seed %d seq %d %q: verdict %s %q, want %q", spec.Seed, r.Seq, r.Card, st, key, w)
			}
		}
		for seq := range recordedWant[spec.Seed] {
			if !seen[seq] {
				t.Errorf("seed %d: no planned cast at seq %d (the game no longer reaches the finding)", spec.Seed, seq)
			}
		}
		if len(reports) == 0 {
			t.Errorf("seed %d: no planned cast at all", spec.Seed)
		}
	}
}

// TestWitnessReadsLifePaymentPastSpeedGain pins lifePaymentConsequences: an
// opponent's speed gain the life payment folds inline sits between a
// pay-life-only source's payment and its mana, and is passed over; any other
// event there still breaks the start.
func TestWitnessReadsLifePaymentPastSpeedGain(t *testing.T) {
	const payer state.PlayerID = 1
	ogre := decision.PaymentActivation{Source: 107, Produces: decision.ManaAmount{0, 0, 0, 1, 0, 0},
		Consequence: &decision.PaymentConsequence{Life: 3}}
	plan := decision.PaymentPlan{Activations: []decision.PaymentActivation{ogre}}
	life := events.Event{Kind: events.LifeChange, Player: payer, Amount: -3}
	speed := events.Event{Kind: events.SpeedChange, Player: 3, Amount: 1, Text: "speed"}
	red := events.Event{Kind: events.ManaAdd, Player: payer, Amount: 1, Counter: "R"}
	evs := []events.Event{life, speed, red}
	if s := lifeOnlyStarts(evs, plan, payer); s[0] != 0 {
		t.Fatalf("starts = %v, want [0] past the speed gain", s)
	}
	if got := manaRunAfter(evs, 0); got != ogre.Produces {
		t.Fatalf("mana after the payment = %v, want R", got)
	}
	if v := witnessOver(t, evs, plan, payer); v != "" {
		t.Fatalf("witness %q, want none", v)
	}
	broken := []events.Event{life, {Kind: events.Draw, Player: payer}, red}
	if s := lifeOnlyStarts(broken, plan, payer); s[0] != -1 {
		t.Fatalf("a draw between payment and mana read as a start: %v", s)
	}
}

// TestWitnessPassesSourceSacrificedForTheSpell pins activationProduction:
// a pay-life-only source sacrificed afterwards for the spell's own
// additional cost (CR 601.2h) is still read through its life payment, while
// a source that tapped and made nothing is still wrong_production.
func TestWitnessPassesSourceSacrificedForTheSpell(t *testing.T) {
	const payer state.PlayerID = 1
	ogre := decision.PaymentActivation{Source: 107, Produces: decision.ManaAmount{0, 0, 0, 1, 0, 0},
		Consequence: &decision.PaymentConsequence{Life: 3}}
	plan := decision.PaymentPlan{Activations: []decision.PaymentActivation{ogre}}
	evs := []events.Event{
		{Kind: events.PutOnStack, Obj: 109, Player: payer},
		{Kind: events.LifeChange, Player: payer, Amount: -3},
		{Kind: events.ManaAdd, Player: payer, Amount: 1, Counter: "R"},
		{Kind: events.ManaAdd, Player: payer, Amount: -1, Counter: "R"},
		{Kind: events.MoveZone, Obj: 107, From: state.ZBattlefield, To: state.ZGraveyard, Text: "sacrificed"},
		{Kind: events.TriggerPush, Obj: 138, Player: payer},
	}
	if v := witnessOver(t, evs, plan, payer); v != "" {
		t.Fatalf("witness %q, want none", v)
	}
	tapped := []events.Event{
		{Kind: events.Tap, Obj: 107},
		{Kind: events.LifeChange, Player: payer, Amount: -3},
		{Kind: events.Note, Player: payer},
		{Kind: events.LifeChange, Player: payer, Amount: -3},
		{Kind: events.ManaAdd, Player: payer, Amount: 1, Counter: "R"},
	}
	if v := witnessOver(t, tapped, plan, payer); v == "" {
		t.Fatal("a tapped source that made nothing passed through another life payment")
	}
}

// witnessOver runs witnessViolation over a fixture engine whose log since the
// fork is exactly evs (the witness reads only a.L.Events[fork:], object names
// and the cast object's zone).
func witnessOver(t *testing.T, evs []events.Event, plan decision.PaymentPlan, payer state.PlayerID) string {
	t.Helper()
	e := fixtureEngine(t)
	fork := len(e.L.Events)
	e.L.Events = append(e.L.Events, evs...)
	return witnessViolation(e, fork, plan, payer)
}
