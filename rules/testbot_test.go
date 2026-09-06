package rules

import (
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/decision"
)

// testBot drives the rules package's own fuzz and acceptance harnesses with
// the same policy seat.Bot uses, through the same botpolicy.Decide (Ruling
// F7). Before botpolicy existed, this file carried a line-for-line copy of
// seat/bot.go's botDecide and clamp -- rules/fuzz_test.go is package rules,
// and importing seat -- which imports view -- runs the declared dependency
// order (cards -> state -> decision -> events -> effects -> rules -> view
// -> seat -> replay -> cmd/*) backwards into the package under test. There
// is one policy now, in botpolicy, and this file is its game-shaped
// adapter: the caller computes isMain from the engine's own step on the
// line before calling answer (this package has no View), and answer feeds
// botpolicy.Decide the same Board a seat.Bot would build from the projected
// View. seat/integration_test.go's TestBotAdaptersAgree* pins the two
// halves to the same Board for the same game facts.
type testBot struct {
	r *rand.Rand
}

func newTestBot(seed uint64) *testBot {
	return &testBot{r: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))}
}

// answer is the fuzz-driver interface this package's callers have always
// used -- the call shape is unchanged from the mirror it replaces:
// rules/fuzz_test.go, acceptance_test.go, priority_phase_test.go and
// clone_test.go all compute isMain from e.G.Step.IsMain() (or their local
// step) on the line before calling, exactly as seat.Bot derives the same
// fact from the View inside Decide. The policy behaviour itself -- and its
// doc, per case -- now lives in botpolicy.Decide; see there for the
// rationale of each decision.Kind. Every access into d.Options is guarded
// against the list being empty there, and clamp (Ruling T25-c) is the last
// thing every return does, so the intent this forwards always validates
// against d for any Min/Max the wire format allows, not only today's
// shapes.
func (b *testBot) answer(isMain bool, d *decision.Decision) decision.Intent {
	return botpolicy.Decide(botpolicy.Board{IsMain: isMain}, d, b.r)
}

// TestTestBotDelegatesToBotPolicy pins answer's wiring: for representative
// shapes of the decisions the fuzz gate throws at it, answer must deliver
// exactly what botpolicy.Decide would -- same Board, same rng stream, same
// consumption points. This is the successor of the mirror tests
// (TestTestBotChoosePolicy and friends), which died with the mirror: the
// policy's behaviour is now tested once, in botpolicy/policy_test.go, and
// this file only has to prove it forwards unchanged.
func TestTestBotDelegatesToBotPolicy(t *testing.T) {
	shapes := []decision.Decision{
		{Seq: 1, Player: 0, Kind: decision.KPriority, Min: 1, Max: 1,
			Options: []decision.Option{
				{Index: 0, Kind: "activate", Obj: 1},
				{Index: 1, Kind: "pass"},
			}},
		// A KChoose shape and a rng-consuming kind (KBlockers), so a
		// regression that skips the rng forwarding -- or feeds the policy a
		// different Board -- shows up here, not only in a game.
		{Seq: 2, Player: 0, Kind: decision.KChoose, Min: 1, Max: 1,
			Options: []decision.Option{
				{Index: 0, Kind: "x"},
				{Index: 1, Kind: "x"},
			}},
		{Seq: 3, Player: 0, Kind: decision.KBlockers, Min: 0, Max: 2,
			Options: []decision.Option{
				{Index: 0, Kind: "block", Obj: 10, Attacker: 20},
				{Index: 1, Kind: "block", Obj: 11, Attacker: 20},
			}},
	}
	for _, isMain := range []bool{false, true} {
		for _, d := range shapes {
			b := newTestBot(99)
			got := b.answer(isMain, &d)
			r := rand.New(rand.NewPCG(99, 99^0x9e3779b97f4a7c15))
			want := botpolicy.Decide(botpolicy.Board{IsMain: isMain}, &d, r)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("isMain=%v kinds=%q: answer = %+v, botpolicy.Decide = %+v",
					isMain, d.Kind, got, want)
			}
			if err := d.Validate(got); err != nil {
				t.Errorf("isMain=%v kinds=%q: answer intents failed Validate: %v", isMain, d.Kind, err)
			}
		}
	}
}
