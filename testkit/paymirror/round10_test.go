package paymirror

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestRoundTenFindingsMirror (TestRoundTenFindingsMirrorChunkNN, sharded per
// findings_chunks_test.go) replays the round-10 paymirror finding games
// (testdata/round10.jsonl: repo deck names or random card-name lists, no
// script text) end to end: no cast is a mismatch, no planned cast falls back
// or misses its witness, the live-vs-clone control is equivalent, and each
// named cast gets its root-caused verdict.
//
//   - 12468, random4: the state_differs finding on
//     G.Objs[*].Remembered[*].Obj (Veiled Crocodile's CR 603.8 state trigger
//     remembered the Island tapped in run A's CR 601.2g window but itself on
//     the float route) has been displaced from the game entirely. A rules fix
//     (stateTriggerRemembered) first moved it to the Homarid Spawning Bed
//     cast, and ticket agent-20261006T052735Z-5c29adb5 then shifted the seed's
//     trajectory again: a Defined$ TopOfLibrary/BottomOfLibrary fetch is a
//     position read and no longer emits a mandatory shuffle (CR 401.4), which
//     changes the libraries the bots draw through, so the Homarid Spawning Bed
//     cast (seq 6402 after the October 5 main merge) is no longer reached.
//     The seed keeps an empty pin and asserts a clean, control-equivalent
//     game; the root cause stays pinned by the unit test
//     rules/state_trigger_remembered_test.go.
//   - 11828 seq 4243 (Vorinclex, Voice of Hunger), random2
//     a_fallback=source_changed: the plan's Fanatic of Rhonas step ("Activate
//     only if you control a creature with power 4 or greater") held only
//     through Syr Elenora (power = cards in hand) while Vorinclex was still
//     in hand; CR 601.2a moved it to the stack first, the step fell back and
//     the cast reversed. A rules fix: the offer proves every planned step
//     with the card on the stack (paymentPlanHoldsOnStack). The plan is no
//     longer offered there, so the game moves; the pin is that no planned
//     cast in it falls back or mismatches. (The report's control route was
//     equivalent: the source_changed was run A's fallback, not the clone.)
//   - 12603 seq 2291 (Mana Vault), commander4: Treasonous Ogre's "Pay 3
//     life: Add {R}" cast plan remains equivalent after the speed gain moved
//     to its triggered-ability timing (CR 702.179d), shifting the cast
//     earlier; the lifePaymentConsequences harness gap is fixed in
//     paymirror.go, so the witness no longer trips on it. The paid-cost census
//     default-on flip (cli-20261012T170859Z) then moved this cast from
//     seq 2298 to 2291, and botmanual (2026-10-14: the auto-pay bot's tap gate reads the engine's cast census, and flashback and fixed-discard casts get payment plans) to
//     seq 2250, and the combat priority fixes (rules e7aaf1530 and
//     93b6057e4, CR 510.4/117.3a: more priority windows in combat) to seq
//     2393, and the CR 510.3 post-damage round (rules 24bbba791) to seq
//     2409, verdict still equivalent.
//   - 10877 seq 2873 (Infernal Plunge), commander4-r9 a_witness:wrong_production:
//     the plan's Ogre paid life for R and was then the creature sacrificed
//     for the spell's additional cost (CR 601.2h); the witness read that
//     sacrifice as its activation (harness gap, activationProduction).
//     botmanual (2026-10-14: the auto-pay bot's tap gate reads the engine's cast census, and flashback and fixed-discard casts get payment plans) moves the cast to seq 4900,
//     the combat priority fixes (rules e7aaf1530, 93b6057e4) to seq 5134,
//     and the CR 510.3 post-damage round (rules 24bbba791) to seq 5198, and
//     the mandatory library-search shuffle (cr-g09: the may-shuffle ask is
//     gone, two fewer events) to seq 5196, where run A's witness is clean and the float route is the expected
//     cast_blocked_by_float_trigger (floating at priority puts a mana
//     trigger on the stack first). cr-g125 (2026-10-26: an untapped
//     RestrictValid$ source, ulalek-eldrazi's Eldrazi Temple here, now funds
//     a planned cast its restriction admits) moves the bots' trajectory:
//     the Infernal Plunge cast is at seq 2879, no longer blocked by a
//     float trigger, so its verdict is plain equivalent (confirmed by
//     reverting the planner hunk: seq 5196 and the expected float verdict
//     return).
//
// fb-20260927T163321Z-69285807 moved the commander seeds here (12468, 11828,
// 12603, 10877) with the command-zone payment-plan fix: a commander in the
// command zone now gets a plan, the auto-pay bots cast it through one, and
// those games move. 10877 keeps the same Infernal Plunge pin at its new seq;
// the other three already carry empty or unchanged pins.
//
// The speed-trigger fix (CR 702.179d) then moved 12603's Ogre cast to seq
// 2298 and fixed the lifePaymentConsequences harness gap in paymirror.go;
// the merged tree was re-measured to that seq, verdict equivalent. The
// crew-tracking fix (tmt-crewedthisturn, CR 702.122) emits one Crew event per
// crewing creature, so seed 12468's trajectory renumbers: the Homarid
// Spawning Bed cast moves from seq 7211 to 7227, verdict unchanged. The
// token-entry fix (agent-20261001T043732Z-8f099a06) then moves seed 12468's
// trajectory again, from seq 7227 to 5830; verdict unchanged. After the
// October 5 main merge it reaches seq 6402, with the same card and verdict,
// and ticket agent-20261006T052735Z-5c29adb5 then removes the cast from the
// game (see the 12468 entry above), so the seed's pin is now empty.
//
// roundTenWant holds those pins.
var roundTenWant = map[uint64]map[uint64]string{ // seed -> seq -> verdict key ("" = equivalent)
	12468: {},
	11828: {},
	12603: {2409: ""},
	10877: {2879: ""},
}

const (
	roundTenFile   = "testdata/round10.jsonl"
	roundTenChunks = 4
)

func TestRoundTenFindingsMirrorChunk00(t *testing.T) { runRoundTenChunk(t, 0) }
func TestRoundTenFindingsMirrorChunk01(t *testing.T) { runRoundTenChunk(t, 1) }
func TestRoundTenFindingsMirrorChunk02(t *testing.T) { runRoundTenChunk(t, 2) }
func TestRoundTenFindingsMirrorChunk03(t *testing.T) { runRoundTenChunk(t, 3) }

// TestRoundTenFindingsMirrorChunksPartitionTheSpecs: the chunks play every
// round-10 spec exactly once (checkFindingPartition).
func TestRoundTenFindingsMirrorChunksPartitionTheSpecs(t *testing.T) {
	checkFindingPartition(t, findingSpecs(t, roundTenFile, 4), roundTenChunks, []func(*testing.T){
		TestRoundTenFindingsMirrorChunk00, TestRoundTenFindingsMirrorChunk01,
		TestRoundTenFindingsMirrorChunk02, TestRoundTenFindingsMirrorChunk03})
}

// runRoundTenChunk is one shard of TestRoundTenFindingsMirror.
func runRoundTenChunk(t *testing.T, i int) {
	t.Parallel()
	d := findingDecks(t)
	for _, spec := range findingChunk(findingSpecs(t, roundTenFile, 4), roundTenChunks, i) {
		reports := round6Game(t, d, spec)
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
			w, ok := roundTenWant[spec.Seed][r.Seq]
			if !ok {
				continue
			}
			seen[r.Seq] = true
			if key != w {
				t.Errorf("seed %d seq %d %q: verdict %s %q, want %q", spec.Seed, r.Seq, r.Card, st, key, w)
			}
		}
		for seq := range roundTenWant[spec.Seed] {
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
