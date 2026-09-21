package seat

// The L9c learned-policy seat's pins. Three properties, over whole games
// and crafted decisions:
//
//  1. DELEGATION: for every decision kind the scorer does not answer, the
//     PolicyNetBot submits exactly the intent the default bot would — paired
//     per decision across one WHOLE game played with a zero-weight
//     checkpoint (the checkpoint round-trip through the real loader, so the
//     geometry/encoder-hash gate runs too);
//  2. LEGALITY: every scored attackers answer is a legal declaration — one
//     option per attacker (CR 506.2), every required attacker declared
//     (CR 508.1d), at most Max (CR 508.1j) — pinned on crafted shapes AND
//     by whole games against the real engine (a Submit failure is an
//     illegal declaration, so the game loop is the validator);
//  3. NON-VACUITY: the scored kinds actually reach the scorer — a
//     random-weight checkpoint plays a DIFFERENT intent stream from the
//     default bot's, so the wiring cannot degrade into two default bots.

import (
	"bytes"
	"context"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/policynet"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// zeroCheckpoint builds a scorer from a genuinely ZERO-weight model pushed
// through the real checkpoint format (write + load, so the encoder-hash and
// geometry gates run): every option scores exactly OutB = 0, so the priority
// argmax ties and breaks to the lowest offered cast/ability/pass index and
// the attackers vote falls to the exactly-tied branch of admissionThreshold
// (the decision's own mean, inclusive — every option is declared when nothing
// distinguishes them, never an empty declaration).
func zeroCheckpoint(t *testing.T) *policynet.Scorer {
	t.Helper()
	m := policynet.NewModel(policynet.TableRows, 8, 4, rand.New(rand.NewPCG(1, 1)))
	for _, blk := range [][]float32{m.Table, m.StateW, m.StateB, m.HidW, m.HidB, m.OutW} {
		for i := range blk {
			blk[i] = 0
		}
	}
	m.OutB = 0
	var buf bytes.Buffer
	if err := policynet.WriteCheckpoint(m, &buf); err != nil {
		t.Fatalf("WriteCheckpoint: %v", err)
	}
	sc, err := policynet.LoadScorer(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("LoadScorer: %v", err)
	}
	return sc
}

// randomCheckpoint builds a scorer from a nonzero (seeded-random) model,
// round-tripped through the real checkpoint format the same way.
func randomCheckpoint(t *testing.T, seed uint64) *policynet.Scorer {
	t.Helper()
	m := policynet.NewModel(policynet.TableRows, 16, 8, rand.New(rand.NewPCG(seed, seed^0x5eed)))
	var buf bytes.Buffer
	if err := policynet.WriteCheckpoint(m, &buf); err != nil {
		t.Fatalf("WriteCheckpoint: %v", err)
	}
	sc, err := policynet.LoadScorer(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("LoadScorer: %v", err)
	}
	return sc
}

// TestPolicyNetDelegatesNonScoredKinds drives one whole game with the
// DEFAULT bot (so the trajectory is the production one) and, at every
// decision, asks the zero-checkpoint PolicyNetBot the same question: for
// every non-scored kind the two intents must be equal. The default's intent
// is the one submitted, so the game follows the production trajectory and
// the comparison stays paired for the whole stream.
func TestPolicyNetDelegatesNonScoredKinds(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 2)
	const seed = 9
	e := rules.New(rules.Config{Seed: seed, Names: names, Decks: decks})
	e.Advance()
	defs := map[state.PlayerID]*Bot{}
	pns := map[state.PlayerID]*PolicyNetBot{}
	for k := range decks {
		defs[state.PlayerID(k)] = NewBot(seed ^ uint64(k+1))
		pns[state.PlayerID(k)] = NewPolicyNetBot(seed^uint64(k+1), zeroCheckpoint(t))
	}

	compared := 0
	byKind := map[decision.Kind]int{}
	n := 0
	for !e.G.Over && e.Pending() != nil && n < 20000 {
		d := e.Pending()
		v := view.Project(e.G, e, d.Player, d)
		defIn, err := defs[d.Player].Decide(context.Background(), v, *d)
		if err != nil {
			t.Fatalf("intent %d: default bot: %v", n, err)
		}
		pnIn, err := pns[d.Player].Decide(context.Background(), v, *d)
		if err != nil {
			t.Fatalf("intent %d: policynet bot: %v", n, err)
		}
		if !scoredKind(d) {
			compared++
			byKind[d.Kind]++
			if defIn.Seq != pnIn.Seq || defIn.Player != pnIn.Player || !slices.Equal(defIn.Choices, pnIn.Choices) {
				t.Fatalf("intent %d (kind %q): policynet delegated %+v, default bot answered %+v — the delegation drifted", n, d.Kind, pnIn, defIn)
			}
		}
		if err := e.Submit(defIn); err != nil {
			t.Fatalf("intent %d: submit: %v", n, err)
		}
		n++
	}
	if !e.G.Over {
		t.Fatalf("game did not terminate after %d intents (turn %d)", n, e.G.Turn)
	}
	if compared == 0 {
		t.Fatal("the game asked no non-scored kind — the delegation pin never ran")
	}
	if len(byKind) < 2 {
		t.Fatalf("the game asked only %v — too narrow a surface to call the delegation whole-game", byKind)
	}
}

// TestPolicyNetAttackersSubsetShapes pins the scored attackers repair on
// crafted decisions, validating every answer with Decision.Validate and the
// engine-shaped rules (one option per attacker, every required attacker
// present, at most Max).
func TestPolicyNetAttackersSubsetShapes(t *testing.T) {
	atkOpt := func(idx int, obj state.ObjID, def state.PlayerID, required bool) decision.Option {
		return decision.Option{Index: idx, Kind: "attacker", Obj: obj, Player: def, Required: required}
	}
	check := func(name string, d *decision.Decision, scores []float32, want []int, distinctAtk, requireAll bool) {
		t.Helper()
		in, ok := attackersFromScores(d, scores)
		if !ok {
			t.Fatalf("%s: no intent returned", name)
		}
		if err := d.Validate(in); err != nil {
			t.Fatalf("%s: Validate rejected the repaired answer: %v", name, err)
		}
		if !slices.Equal(in.Choices, want) {
			t.Fatalf("%s: choices %v, want %v", name, in.Choices, want)
		}
		if distinctAtk {
			seen := map[state.ObjID]bool{}
			for _, c := range d.Chosen(in) {
				if seen[c.Obj] {
					t.Fatalf("%s: attacker %d declared twice", name, c.Obj)
				}
				seen[c.Obj] = true
			}
		}
		if requireAll {
			for _, o := range d.Options {
				if !o.Required {
					continue
				}
				if !slices.ContainsFunc(d.Chosen(in), func(c decision.Option) bool { return c.Obj == o.Obj }) {
					t.Fatalf("%s: required attacker %d omitted", name, o.Obj)
				}
			}
		}
	}

	// One attacker, one defender, positive score: declared.
	d1 := &decision.Decision{Kind: decision.KAttackers, Min: 0, Max: 1, Seq: 1, Player: 0,
		Options: []decision.Option{atkOpt(0, 10, 1, false)}}
	check("single-positive", d1, []float32{2}, []int{0}, true, false)

	// One attacker offered against two defenders, both admitted: the better
	// score wins (CR 506.2's one-defender repair).
	d2 := &decision.Decision{Kind: decision.KAttackers, Min: 0, Max: 2, Seq: 2, Player: 0,
		Options: []decision.Option{atkOpt(0, 10, 1, false), atkOpt(1, 10, 2, false)}}
	check("dedupe-keeps-best", d2, []float32{1, 3}, []int{1}, true, false)

	// An exact tie admits the LOWEST option index (the brief's tie rule).
	check("tie-lowest-index", d2, []float32{3, 3}, []int{0}, true, false)

	// A required attacker nothing admits still declares — its least-bad
	// option — because a declaration omitting it is illegal outright.
	d3 := &decision.Decision{Kind: decision.KAttackers, Min: 0, Max: 2, Seq: 3, Player: 0,
		Options: []decision.Option{atkOpt(0, 10, 1, true), atkOpt(1, 10, 2, true)}}
	check("required-declares-when-vetoed", d3, []float32{-5, -2}, []int{1}, true, true)

	// Two free attackers, only one may swing: truncation keeps the first.
	d4 := &decision.Decision{Kind: decision.KAttackers, Min: 0, Max: 1, Seq: 4, Player: 0,
		Options: []decision.Option{atkOpt(0, 10, 1, false), atkOpt(1, 11, 1, false)}}
	check("max-truncates-free-first", d4, []float32{1, 1}, []int{0}, true, false)

	// The same ceiling with the second attacker required: the required-first
	// reorder keeps the requirement, not the free one.
	d5 := &decision.Decision{Kind: decision.KAttackers, Min: 0, Max: 1, Seq: 5, Player: 0,
		Options: []decision.Option{atkOpt(0, 10, 1, false), atkOpt(1, 11, 1, true)}}
	check("max-truncates-required-first", d5, []float32{1, 1}, []int{1}, true, true)

	// Nothing admitted and nothing required: an empty declaration, which is
	// a legal answer at Min 0.
	d6 := &decision.Decision{Kind: decision.KAttackers, Min: 0, Max: 2, Seq: 6, Player: 0,
		Options: []decision.Option{atkOpt(0, 10, 1, false)}}
	in, ok := attackersFromScores(d6, []float32{-1})
	if !ok {
		t.Fatal("empty-subset: no intent returned")
	}
	if len(in.Choices) != 0 {
		t.Fatalf("empty-subset: choices %v, want none", in.Choices)
	}
	if err := d6.Validate(in); err != nil {
		t.Fatalf("empty-subset: Validate rejected the empty declaration: %v", err)
	}
}

// TestPolicyNetAttackersAdmissionIsShiftInvariant is the L9d regression pin:
// a checkpoint whose scores share a large offset — a softmax-CE head's output
// is shift-invariant, measured at [11575, 15052] for every labelled attack
// option — must not make the seat declare every legal attacker (all-in), and
// the mirrored all-negative range must not make it declare none (empty). The
// old rule (sigmoid(score) > 0.5, i.e. score > 0) admitted 100% of a positive
// range, which is how the seat emptied its board into bad attacks and lost
// 1000/1000 against the default bot.
//
// The scores are the controller's measured CE level (base 12000), not a zero
// model: a zero model scores 0.0 and 0 > 0 is false, which is exactly why the
// bug shipped past the pre-existing pins.
func TestPolicyNetAttackersAdmissionIsShiftInvariant(t *testing.T) {
	atkOpt := func(idx int, obj state.ObjID, def state.PlayerID, required bool) decision.Option {
		return decision.Option{Index: idx, Kind: "attacker", Obj: obj, Player: def, Required: required}
	}
	// Four independent attackers against one defender, none required: the
	// decision is a genuine subset selection, so "all-in" and "empty" are both
	// observable non-answers.
	d := &decision.Decision{Kind: decision.KAttackers, Min: 0, Max: 4, Seq: 11, Player: 0,
		Options: []decision.Option{
			atkOpt(0, 10, 1, false), atkOpt(1, 11, 1, false),
			atkOpt(2, 12, 1, false), atkOpt(3, 13, 1, false),
		}}
	const base = 12000 // the CE checkpoint's measured level (L9d evidence)

	pos := []float32{base, base + 1, base + 2, base + 3}
	in, ok := attackersFromScores(d, pos)
	if !ok {
		t.Fatal("all-positive: no intent returned")
	}
	if err := d.Validate(in); err != nil {
		t.Fatalf("all-positive: Validate rejected: %v", err)
	}
	if len(in.Choices) == len(d.Options) {
		t.Fatalf("all-positive: admitted all %d options — the offset was trusted as a calibrated sign (the L9d bug)", len(in.Choices))
	}
	if len(in.Choices) == 0 {
		t.Fatalf("all-positive: admitted none of %d options — the shift-invariant fallback over-corrected", len(d.Options))
	}

	neg := []float32{-base - 3, -base - 2, -base - 1, -base}
	in, ok = attackersFromScores(d, neg)
	if !ok {
		t.Fatal("all-negative: no intent returned")
	}
	if err := d.Validate(in); err != nil {
		t.Fatalf("all-negative: Validate rejected: %v", err)
	}
	if len(in.Choices) == 0 {
		t.Fatalf("all-negative: admitted none of %d options — the offset was trusted as a calibrated sign (the mirrored L9d bug)", len(d.Options))
	}
	if len(in.Choices) == len(d.Options) {
		t.Fatalf("all-negative: admitted all %d options", len(in.Choices))
	}
}

// TestPolicyNetAttackersAdmissionKeepsCalibratedSign pins the other half of
// the rule: when the decision's scores straddle zero (a per-option binary
// head's calibrated output), admission is the trained sign — positive
// declared, negative dropped — not a relative ranking.
func TestPolicyNetAttackersAdmissionKeepsCalibratedSign(t *testing.T) {
	atkOpt := func(idx int, obj state.ObjID, def state.PlayerID) decision.Option {
		return decision.Option{Index: idx, Kind: "attacker", Obj: obj, Player: def}
	}
	d := &decision.Decision{Kind: decision.KAttackers, Min: 0, Max: 2, Seq: 12, Player: 0,
		Options: []decision.Option{atkOpt(0, 10, 1), atkOpt(1, 11, 1)}}
	in, ok := attackersFromScores(d, []float32{2, -2})
	if !ok {
		t.Fatal("straddle: no intent returned")
	}
	if !slices.Equal(in.Choices, []int{0}) {
		t.Fatalf("straddle: choices %v, want [0] (the calibrated positive sign)", in.Choices)
	}
	in, ok = attackersFromScores(d, []float32{-2, 2})
	if !ok {
		t.Fatal("straddle (reversed): no intent returned")
	}
	if !slices.Equal(in.Choices, []int{1}) {
		t.Fatalf("straddle (reversed): choices %v, want [1]", in.Choices)
	}
}

// priority decision offering only options the teacher never labelled
// (an "activate" tap) delegates to the default bot verbatim.
func TestPolicyNetEmptySurfaceDelegates(t *testing.T) {
	b := NewPolicyNetBot(3, randomCheckpoint(t, 4))
	def := NewBot(3)
	d := decision.Decision{Kind: decision.KPriority, Min: 1, Max: 1, Seq: 7, Player: 0,
		Options: []decision.Option{{Index: 0, Kind: "activate", Obj: 1}, {Index: 1, Kind: "pass"}}}
	v := view.View{Viewer: 0, Active: 0, Phase: "main1"}
	got, err := b.Decide(context.Background(), v, d)
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	want, err := def.Decide(context.Background(), v, d)
	if err != nil {
		t.Fatalf("default bot: %v", err)
	}
	if got.Seq != want.Seq || got.Player != want.Player || !slices.Equal(got.Choices, want.Choices) {
		t.Fatalf("activate-only priority: policynet %+v, default %+v — the fallback did not delegate", got, want)
	}
}

// TestPolicyNetDelegatesOtherKinds pins the kind switch on a scored-nil and
// a non-scored kind: KBlockers and KChoose always delegate, with or without
// options.
func TestPolicyNetDelegatesOtherKinds(t *testing.T) {
	b := NewPolicyNetBot(3, randomCheckpoint(t, 4))
	def := NewBot(3)
	v := view.View{Viewer: 0, Active: 0}
	for _, k := range []decision.Kind{decision.KBlockers, decision.KChoose, decision.KMulligan, decision.KTarget} {
		d := decision.Decision{Kind: k, Min: 0, Max: 1, Seq: 8, Player: 0,
			Options: []decision.Option{{Index: 0, Kind: "block", Obj: 1}, {Index: 1, Kind: "x"}}}
		got, err := b.Decide(context.Background(), v, d)
		if err != nil {
			t.Fatalf("kind %q: %v", k, err)
		}
		want, err := def.Decide(context.Background(), v, d)
		if err != nil {
			t.Fatalf("kind %q: default bot: %v", k, err)
		}
		if got.Seq != want.Seq || got.Player != want.Player || !slices.Equal(got.Choices, want.Choices) {
			t.Fatalf("kind %q: policynet %+v, default %+v", k, got, want)
		}
	}
}

// TestPolicyNetPlaysWholeGamesLegalAndDistinct runs whole games with the
// random-checkpoint bot against the default bot: no submit may EVER fail
// (the engine is the attackers validator), and the intent stream must
// differ from the default-vs-default one (the scored kinds reach the scorer).
func TestPolicyNetPlaysWholeGamesLegalAndDistinct(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 2)
	sc := randomCheckpoint(t, 77)
	for _, seed := range []uint64{100, 101, 102, 103} {
		defRun := driveBotGame(t, seed, names, decks, func(p state.PlayerID) answerer {
			return NewBot(seed ^ uint64(p+1))
		})
		pnRun := driveBotGame(t, seed, names, decks, func(p state.PlayerID) answerer {
			return NewPolicyNetBot(seed^uint64(p+1), sc)
		})
		if slices.EqualFunc(defRun, pnRun, func(a, b decision.Intent) bool {
			return a.Seq == b.Seq && a.Player == b.Player && slices.Equal(a.Choices, b.Choices)
		}) {
			t.Fatalf("seed %d: the policynet run reproduced the default bot's intent stream exactly — the scored kinds never reached the scorer", seed)
		}
	}
}

// answerer is the per-seat policy the driver asks.
type answerer interface {
	Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error)
}

// driveBotGame plays one full game with per-seat bots, recording every
// submitted intent. The engine's livelock watcher panics inside Submit; the
// wrapper converts that into a clean stop (a stall, not an error) so a
// random-weight policy's pathological loop ends the GAME, not the test.
func driveBotGame(t *testing.T, seed uint64, names []string, decks [][]*cards.Card, newSeat func(state.PlayerID) answerer) []decision.Intent {
	t.Helper()
	e := rules.New(rules.Config{Seed: seed, Names: names, Decks: decks})
	e.Advance()
	bots := map[state.PlayerID]answerer{}
	for k := range decks {
		bots[state.PlayerID(k)] = newSeat(state.PlayerID(k))
	}
	var intents []decision.Intent
	n := 0
	for !e.G.Over && e.Pending() != nil && n < 20000 && e.G.Turn < 200 {
		d := e.Pending()
		in, err := bots[d.Player].Decide(context.Background(), view.Project(e.G, e, d.Player, d), *d)
		if err != nil {
			t.Fatalf("seed %d intent %d: %v", seed, n, err)
		}
		if err := submitSafe(e, in); err != nil {
			var livelock *rules.LivelockError
			if asLivelock(err, &livelock) {
				break // a stall: the random-weight policy looped; legality is not in question
			}
			t.Fatalf("seed %d intent %d: submit: %v", seed, n, err)
		}
		intents = append(intents, in)
		n++
	}
	return intents
}

// submitSafe isolates the livelock panic so it can be told from a real
// error (an illegal intent), which must fail the test.
func submitSafe(e *rules.Engine, in decision.Intent) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if le, ok := r.(*rules.LivelockError); ok {
				err = le
				return
			}
			panic(r)
		}
	}()
	return e.Submit(in)
}

func asLivelock(err error, target **rules.LivelockError) bool {
	le, ok := err.(*rules.LivelockError)
	if ok {
		*target = le
	}
	return ok
}

// TestPolicyNetDelegatesPriority pins the L9d bench result as a behaviour:
// KPriority must reach the DEFAULT bot, not the scorer's argmax. This is not
// tidy-up — it is the difference between winning 0/1000 and 474/1000 against
// the default bot on the ten approved mono pairs (checkpoint bce-big.gpol,
// 100 games/pair, seed 10000000; the 2x2 isolating it from the attackers
// admission rule is in the L9d report). The priority head is trained (argmax
// CE) and the scored path is a few lines away, so without a pin it is an easy
// thing to switch back on by accident; re-enable it only behind a bench that
// clears that bar.
//
// The checkpoint is a RANDOM one, not the zero model: its per-option scores
// genuinely differ, so a scored priority path would argmax to some particular
// cast/ability/pass option and this equality would break (verified: with
// KPriority restored to scoredKind, this test fails).
func TestPolicyNetDelegatesPriority(t *testing.T) {
	if scoredKind(&decision.Decision{Kind: decision.KPriority}) {
		t.Fatal("scoredKind admits KPriority: the scored priority path measures 0/1000 in play")
	}
	b := NewPolicyNetBot(3, randomCheckpoint(t, 4))
	def := NewBot(3)
	v := view.View{Viewer: 0, Active: 0, Phase: "main1"}
	d := decision.Decision{Kind: decision.KPriority, Min: 1, Max: 1, Seq: 9, Player: 0,
		Options: []decision.Option{
			{Index: 0, Kind: "cast", Obj: 1},
			{Index: 1, Kind: "cast", Obj: 2},
			{Index: 2, Kind: "ability", Obj: 3},
			{Index: 3, Kind: "pass"},
		}}
	got, err := b.Decide(context.Background(), v, d)
	if err != nil {
		t.Fatalf("policynet: %v", err)
	}
	want, err := def.Decide(context.Background(), v, d)
	if err != nil {
		t.Fatalf("default bot: %v", err)
	}
	if got.Seq != want.Seq || got.Player != want.Player || !slices.Equal(got.Choices, want.Choices) {
		t.Fatalf("priority: policynet %+v, default %+v — the scored path answered", got, want)
	}
}
