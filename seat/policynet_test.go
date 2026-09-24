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
	"github.com/adams-shaun/gorge/internal/searchprobe"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// zeroCheckpoint builds a scorer from a genuinely ZERO-weight model pushed
// through the real checkpoint format (write + load, so the encoder-hash and
// geometry gates run): every option scores exactly OutB = 0, so every scored
// decision is an EXACT TIE. attackersFromScores refuses a tie (no sign, no
// ranking) and the seat falls back to the default bot's answer. The zero
// model is why the original defect shipped — a `score > 0` vote on all-zeros
// admits nothing, so the old pins only ever exercised the empty declaration.
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
		if !pns[d.Player].scoresKind(d) {
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

	// An exact tie carries NO information — not a calibrated sign (the tie is
	// one-signed) and not a ranking. The seat refuses to answer and the caller
	// falls back to the default bot's declaration, rather than declaring the
	// whole mean-inclusive subset (which on a tie is every option — the all-in
	// failure L9d exists to prevent, reached by another road).
	if _, ok := attackersFromScores(d2, []float32{3, 3}); ok {
		t.Fatal("tie: seat answered an exactly-tied decision; want delegation")
	}

	// A required attacker nothing admits still declares — its least-bad
	// option — because a declaration omitting it is illegal outright.
	d3 := &decision.Decision{Kind: decision.KAttackers, Min: 0, Max: 2, Seq: 3, Player: 0,
		Options: []decision.Option{atkOpt(0, 10, 1, true), atkOpt(1, 10, 2, true)}}
	check("required-declares-when-vetoed", d3, []float32{-5, -2}, []int{1}, true, true)

	// Two admitted free attackers, only one may swing: truncation keeps the
	// first in OFFER order. Three options, because the mean reference always
	// splits a two-option varying decision — admitting two takes a third,
	// lower-scoring option to pull the mean below the leading pair.
	d4 := &decision.Decision{Kind: decision.KAttackers, Min: 0, Max: 1, Seq: 4, Player: 0,
		Options: []decision.Option{atkOpt(0, 10, 1, false), atkOpt(1, 11, 1, false), atkOpt(2, 12, 1, false)}}
	check("max-truncates-free-first", d4, []float32{5, 5, 1}, []int{0}, true, false)

	// The same ceiling with a required attacker the ranking did NOT admit:
	// the required-first reorder keeps the requirement, not the free options.
	d5 := &decision.Decision{Kind: decision.KAttackers, Min: 0, Max: 1, Seq: 5, Player: 0,
		Options: []decision.Option{atkOpt(0, 10, 1, false), atkOpt(1, 11, 1, false), atkOpt(2, 12, 1, true)}}
	check("max-truncates-required-first", d5, []float32{5, 5, 1}, []int{2}, true, true)

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

// priorityDecision is a crafted in-distribution priority decision: two
// distinct castable objects, an ability and a pass.
func priorityDecision() decision.Decision {
	return decision.Decision{Kind: decision.KPriority, Min: 1, Max: 1, Seq: 9, Player: 0,
		Options: []decision.Option{
			{Index: 0, Kind: "cast", Obj: 1},
			{Index: 1, Kind: "cast", Obj: 2},
			{Index: 2, Kind: "ability", Obj: 3},
			{Index: 3, Kind: "pass"},
		}}
}

// withResidual rebuilds a random checkpoint's model with the given
// bot-prior residual weight, so the non-zero head AND an active prior are
// both present.
func withResidual(t *testing.T, seed uint64, w float32) *policynet.Scorer {
	t.Helper()
	m := policynet.NewModel(policynet.TableRows, 16, 8, rand.New(rand.NewPCG(seed, seed^0x5eed)))
	m.ResidualW = w
	var buf bytes.Buffer
	if err := policynet.WriteCheckpoint(m, &buf); err != nil {
		t.Fatalf("WriteCheckpoint: %v", err)
	}
	sc, err := policynet.LoadScorer(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("LoadScorer: %v", err)
	}
	if sc.ResidualWeight() != w {
		t.Fatalf("ResidualW round-trip: got %v, want %v", sc.ResidualWeight(), w)
	}
	return sc
}

// sameIntent reports exact intent equality (Seq, Player, Choices in order).
func sameIntent(a, b decision.Intent) bool {
	return a.Seq == b.Seq && a.Player == b.Player && slices.Equal(a.Choices, b.Choices)
}

// TestPolicyNetDelegatesPriority pins the two configurations under which
// KPriority must reach the DEFAULT bot, never the scorer's argmax:
//
//	(a) the DEFAULT kind selection (NewPolicyNetBot) never scores priority,
//	    whatever the checkpoint — the scored path is opt-in;
//	(b) priority selected but the checkpoint's ResidualW == 0 delegates: the
//	    residual-free scored path is the plain argmax that measured 0/1000
//	    against the default bot (checkpoint bce-big.gpol, ten approved mono
//	    pairs, 100 games/pair, seed 10000000), so that configuration must be
//	    unreachable.
//
// The checkpoint is a RANDOM one, not the zero model: its per-option scores
// genuinely differ, so a scored priority path would argmax to some
// particular cast/ability/pass option and the equality would break.
func TestPolicyNetDelegatesPriority(t *testing.T) {
	v := view.View{Viewer: 0, Active: 0, Phase: "main1"}
	d := priorityDecision()
	want, err := NewBot(3).Decide(context.Background(), v, d)
	if err != nil {
		t.Fatalf("default bot: %v", err)
	}

	// (a) default kinds: attackers only, even with an active prior.
	def := NewPolicyNetBot(3, withResidual(t, 4, 3))
	if def.scoresKind(&d) {
		t.Fatal("the default kind selection scores KPriority; it must be opt-in")
	}
	got, err := def.Decide(context.Background(), v, d)
	if err != nil {
		t.Fatalf("policynet: %v", err)
	}
	if !sameIntent(got, want) {
		t.Fatalf("default kinds: policynet %+v, default %+v — the scored path answered", got, want)
	}

	// (b) priority selected, ResidualW == 0.
	sc := randomCheckpoint(t, 4)
	if sc.ResidualWeight() != 0 {
		t.Fatalf("randomCheckpoint ResidualW = %v, want 0", sc.ResidualWeight())
	}
	opt := NewPolicyNetBotKinds(3, sc, []decision.Kind{decision.KAttackers, decision.KPriority})
	got, err = opt.Decide(context.Background(), v, d)
	if err != nil {
		t.Fatalf("policynet: %v", err)
	}
	if !sameIntent(got, want) {
		t.Fatalf("ResidualW 0: policynet %+v, default %+v — the 0/1000 configuration was reachable", got, want)
	}

	// Non-vacuity: the SAME decision with the prior active is scored, and the
	// random head is not the bot's answer on every seed — otherwise neither
	// arm above would prove anything.
	differs := false
	for seed := uint64(1); seed <= 32 && !differs; seed++ {
		sb := NewPolicyNetBotKinds(3, withResidual(t, seed, 0.001), []decision.Kind{decision.KPriority})
		got, err := sb.Decide(context.Background(), v, d)
		if err != nil {
			t.Fatalf("policynet: %v", err)
		}
		differs = !sameIntent(got, want)
	}
	if !differs {
		t.Fatal("no random head with an active prior ever overrode the bot on the in-distribution decision — the delegation arms are vacuous")
	}
}

// TestPolicyNetPriorityOutOfDistributionDelegates pins the priority
// distribution gate with a NON-zero head and an active prior: a priority
// decision the head was never trained on returns exactly the bot's intent.
// Two shapes: one castable object (searchseat.Eligible refuses it, so the
// teacher never labels it), and a bot answer of play_land (the teacher's
// candidates arm labels only a cast/ability/pass bot answer).
func TestPolicyNetPriorityOutOfDistributionDelegates(t *testing.T) {
	v := view.View{Viewer: 0, Active: 0, Phase: "main1"}
	cases := []struct {
		name string
		d    decision.Decision
	}{
		{"one castable object", decision.Decision{Kind: decision.KPriority, Min: 1, Max: 1, Seq: 4, Player: 0,
			Options: []decision.Option{
				{Index: 0, Kind: "cast", Obj: 1},
				{Index: 1, Kind: "cast", Obj: 1}, // same card, another cost
				{Index: 2, Kind: "ability", Obj: 3},
				{Index: 3, Kind: "pass"},
			}}},
		{"bot plays a land", decision.Decision{Kind: decision.KPriority, Min: 1, Max: 1, Seq: 5, Player: 0,
			Options: []decision.Option{
				{Index: 0, Kind: "play_land", Obj: 7},
				{Index: 1, Kind: "cast", Obj: 1},
				{Index: 2, Kind: "cast", Obj: 2},
				{Index: 3, Kind: "ability", Obj: 3},
				{Index: 4, Kind: "pass"},
			}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want, err := NewBot(3).Decide(context.Background(), v, tc.d)
			if err != nil {
				t.Fatalf("default bot: %v", err)
			}
			if tc.name == "bot plays a land" {
				if len(want.Choices) != 1 || tc.d.Options[want.Choices[0]].Kind != "play_land" {
					t.Fatalf("premise: the default bot answered %+v, want the play_land option", want)
				}
			}
			for seed := uint64(1); seed <= 16; seed++ {
				sc := withResidual(t, seed, 0.001)
				b := NewPolicyNetBotKinds(3, sc, []decision.Kind{decision.KAttackers, decision.KPriority})
				if priorityScorable(&tc.d, want, sc.ResidualWeight()) {
					t.Fatalf("priorityScorable admitted an out-of-distribution decision")
				}
				got, err := b.Decide(context.Background(), v, tc.d)
				if err != nil {
					t.Fatalf("policynet: %v", err)
				}
				if !sameIntent(got, want) {
					t.Fatalf("seed %d: policynet %+v, default %+v — an out-of-distribution priority decision was scored", seed, got, want)
				}
			}
		})
	}
}

// TestPolicyNetResidualReproducesTheBotInPlay pins the residual prior's
// INFERENCE half end to end. A zero-head model carrying ONLY the bot-prior
// residual weight, built with BOTH kinds scored, must answer every scored
// decision with exactly the wrapped default bot's intent over one whole
// game.
//
// Priority: every scored priority decision is in the trained distribution
// (priorityScorable), the bot's own cast/ability/pass answer carries the
// prior's +ResidualW, every other option scores the zero head's 0, so the
// argmax is the bot's answer. The premise guard requires at least one such
// in-distribution decision to have been compared, and no scored priority
// answer may be a tap/land option (those decisions are out of distribution
// and must delegate before scoring).
//
// Attackers: a zero-head model whose bot declared NO attacker marks no
// option and ties every score, and attackersFromScores must refuse that tie
// rather than declare the mean-inclusive subset (which on a tie is every
// option).
func TestPolicyNetResidualReproducesTheBotInPlay(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 2)
	const seed = 9
	e := rules.New(rules.Config{Seed: seed, Names: names, Decks: decks})
	e.Advance()

	// The zero model the delegation test uses, plus the prior, built
	// in-process (NewScorer over the live Model).
	newResidualBot := func(s uint64) *PolicyNetBot {
		m := policynet.NewModel(policynet.TableRows, 8, 4, rand.New(rand.NewPCG(1, 1)))
		for _, blk := range [][]float32{m.Table, m.StateW, m.StateB, m.HidW, m.HidB, m.OutW} {
			for i := range blk {
				blk[i] = 0
			}
		}
		m.OutB = 0
		m.ResidualW = 3
		return NewPolicyNetBotKinds(s, policynet.NewScorer(m), []decision.Kind{decision.KAttackers, decision.KPriority})
	}
	pns := map[state.PlayerID]*PolicyNetBot{}
	for k := range decks {
		pns[state.PlayerID(k)] = newResidualBot(seed ^ uint64(k+1))
	}

	compared := map[decision.Kind]int{}
	priorityInDist := 0  // scored (in-distribution) priority decisions compared
	priorityTapLand := 0 // scored priority answers OUTSIDE cast/ability/pass
	n := 0
	for !e.G.Over && e.Pending() != nil && n < 20000 {
		d := e.Pending()
		v := view.Project(e.G, e, d.Player, d)
		pn := pns[d.Player]
		pnIn, err := pn.Decide(context.Background(), v, *d)
		if err != nil {
			t.Fatalf("intent %d: policynet bot: %v", n, err)
		}
		def, err := pn.def.Decide(context.Background(), v, *d)
		if err != nil {
			t.Fatalf("intent %d: default bot: %v", n, err)
		}
		if pn.scoresKind(d) {
			compared[d.Kind]++
			// Attackers: the declaration is a set of (attacker, defender)
			// options — compare the multiset, not the submission order.
			a := append([]int(nil), pnIn.Choices...)
			b := append([]int(nil), def.Choices...)
			slices.Sort(a)
			slices.Sort(b)
			if pnIn.Seq != def.Seq || pnIn.Player != def.Player || !slices.Equal(a, b) {
				t.Fatalf("intent %d (kind %q): residual policynet answered %+v, the bot-prior contract wants the default bot's %+v — the prior did not reproduce the bot",
					n, d.Kind, pnIn, def)
			}
			if d.Kind == decision.KPriority && priorityScorable(d, def, pn.scorer.ResidualWeight()) {
				priorityInDist++
				for _, c := range pnIn.Choices {
					if c >= 0 && c < len(d.Options) {
						switch d.Options[c].Kind {
						case "cast", "ability", "pass":
						default:
							priorityTapLand++
						}
					}
				}
			}
		}
		if err := e.Submit(def); err != nil {
			t.Fatalf("intent %d: submit: %v", n, err)
		}
		n++
	}
	if !e.G.Over {
		t.Fatalf("game did not terminate after %d intents (turn %d)", n, e.G.Turn)
	}
	if compared[decision.KAttackers] == 0 {
		t.Fatalf("the game asked no scored attackers decision, so the prior is unpinned (%v)", compared)
	}
	if priorityInDist == 0 {
		t.Fatalf("the game asked no in-distribution priority decision, so the priority prior is unpinned (%v)", compared)
	}
	if priorityTapLand != 0 {
		t.Fatalf("%d scored priority answers were tap/land options — the distribution gate let the untrained surface through", priorityTapLand)
	}
	t.Logf("compared %v; %d in-distribution priority decisions scored", compared, priorityInDist)
}

// zeroResidualBot is the residual-prior-only seat: a zero-head model (every
// option scores 0) carrying ResidualW, so a bot-picked option scores
// ResidualW and every other option 0.
func zeroResidualBot(s uint64, w float32, kinds []decision.Kind) *PolicyNetBot {
	m := policynet.NewModel(policynet.TableRows, 8, 4, rand.New(rand.NewPCG(1, 1)))
	for _, blk := range [][]float32{m.Table, m.StateW, m.StateB, m.HidW, m.HidB, m.OutW} {
		for i := range blk {
			blk[i] = 0
		}
	}
	m.OutB = 0
	m.ResidualW = w
	return NewPolicyNetBotKinds(s, policynet.NewScorer(m), kinds)
}

// TestPolicyNetDefaultKindsUnchanged pins the default scored-kind selection:
// NewPolicyNetBot scores KAttackers and nothing else — the opt-in kinds this
// ticket added (blockers, target) and priority stay off.
func TestPolicyNetDefaultKindsUnchanged(t *testing.T) {
	b := NewPolicyNetBot(1, nil)
	for _, k := range []decision.Kind{decision.KAttackers, decision.KPriority, decision.KBlockers, decision.KTarget, decision.KChoose} {
		d := decision.Decision{Kind: k}
		if got, want := b.scoresKind(&d), k == decision.KAttackers; got != want {
			t.Errorf("default selection scoresKind(%q) = %v, want %v", k, got, want)
		}
	}
	if !slices.Equal(PolicyNetKindNames, []string{"attackers", "priority", "blockers", "target"}) {
		t.Errorf("PolicyNetKindNames = %v", PolicyNetKindNames)
	}
	kinds, err := ParsePolicyNetKinds("attackers,blockers, target")
	if err != nil || !slices.Equal(kinds, []decision.Kind{decision.KAttackers, decision.KBlockers, decision.KTarget}) {
		t.Errorf("ParsePolicyNetKinds(attackers,blockers, target) = %v, %v", kinds, err)
	}
	for _, bad := range []string{"blocker", "targets", "blockers,blockers"} {
		if _, err := ParsePolicyNetKinds(bad); err == nil {
			t.Errorf("ParsePolicyNetKinds(%q) accepted", bad)
		}
	}
}

// TestPolicyNetSingleTargetMatchesTeacher pins the seat's restated shape
// test to the teacher's (searchprobe.SingleTarget) over crafted shapes, so
// the scored distribution and the labelled one cannot drift apart.
func TestPolicyNetSingleTargetMatchesTeacher(t *testing.T) {
	two := []decision.Option{{Index: 0, Kind: "target", Obj: 1}, {Index: 1, Kind: "target", Obj: 2}}
	shapes := []*decision.Decision{
		nil,
		{Kind: decision.KTarget, Min: 1, Max: 1, Options: two},
		{Kind: decision.KTarget, Min: 1, Max: 2, Options: two},
		{Kind: decision.KTarget, Min: 0, Max: 1, Options: two},
		{Kind: decision.KTarget, Min: 1, Max: 1, Options: two[:1]},
		{Kind: decision.KTarget, Min: 1, Max: 1, MaxSum: 3, Options: two},
		{Kind: decision.KChoose, Min: 1, Max: 1, Options: two},
	}
	for i, d := range shapes {
		if got, want := singleTarget(d), searchprobe.SingleTarget(d); got != want {
			t.Errorf("shape %d: singleTarget %v, searchprobe.SingleTarget %v", i, got, want)
		}
	}
}

// TestPolicyNetTargetOutOfShapeDelegates: a KTarget decision outside the
// teacher's single-choice shape (Max 2, or Min 0), or any target decision on
// a ResidualW 0 checkpoint, returns exactly the default bot's intent — with a
// random head that WOULD pick something else when scored (the non-vacuity
// arm proves the in-shape decision is scored).
func TestPolicyNetTargetOutOfShapeDelegates(t *testing.T) {
	v := view.View{Viewer: 0, Active: 0, Phase: "main1"}
	opts := []decision.Option{
		{Index: 0, Kind: "target", Obj: 1}, {Index: 1, Kind: "target", Obj: 2},
		{Index: 2, Kind: "target", Obj: 3}, {Index: 3, Kind: "target", Obj: 4},
	}
	kinds := []decision.Kind{decision.KTarget}
	for _, d := range []decision.Decision{
		{Kind: decision.KTarget, Min: 1, Max: 2, Seq: 3, Player: 0, Options: opts},
		{Kind: decision.KTarget, Min: 0, Max: 1, Seq: 3, Player: 0, Options: opts},
	} {
		want, err := NewBot(3).Decide(context.Background(), v, d)
		if err != nil {
			t.Fatalf("default bot: %v", err)
		}
		for seed := uint64(1); seed <= 16; seed++ {
			b := NewPolicyNetBotKinds(3, withResidual(t, seed, 0.001), kinds)
			got, err := b.Decide(context.Background(), v, d)
			if err != nil {
				t.Fatalf("policynet: %v", err)
			}
			if !sameIntent(got, want) {
				t.Fatalf("Min %d Max %d seed %d: policynet %+v, default %+v — an out-of-shape target decision was scored", d.Min, d.Max, seed, got, want)
			}
		}
		if _, ok := targetFromScores(&d, []float32{0, 1, 2, 3}); ok {
			t.Fatalf("Min %d Max %d: targetFromScores answered an out-of-shape decision", d.Min, d.Max)
		}
	}

	in := decision.Decision{Kind: decision.KTarget, Min: 1, Max: 1, Seq: 4, Player: 0, Options: opts}
	want, err := NewBot(3).Decide(context.Background(), v, in)
	if err != nil {
		t.Fatalf("default bot: %v", err)
	}
	// ResidualW 0: the plain argmax is unreachable.
	got, err := NewPolicyNetBotKinds(3, randomCheckpoint(t, 4), kinds).Decide(context.Background(), v, in)
	if err != nil {
		t.Fatalf("policynet: %v", err)
	}
	if !sameIntent(got, want) {
		t.Fatalf("ResidualW 0: policynet %+v, default %+v — the target argmax was reachable without the prior", got, want)
	}
	// Non-vacuity: in shape with an active prior, some random head overrides.
	differs := false
	for seed := uint64(1); seed <= 32 && !differs; seed++ {
		got, err := NewPolicyNetBotKinds(3, withResidual(t, seed, 0.001), kinds).Decide(context.Background(), v, in)
		if err != nil {
			t.Fatalf("policynet: %v", err)
		}
		if err := in.Validate(got); err != nil {
			t.Fatalf("seed %d: scored target answer invalid: %v", seed, err)
		}
		differs = !sameIntent(got, want)
	}
	if !differs {
		t.Fatal("no random head ever overrode the bot on an in-shape target decision — the delegation arms are vacuous")
	}
	// Ties break on the lowest option index.
	if got, ok := targetFromScores(&in, []float32{1, 5, 5, 2}); !ok || !slices.Equal(got.Choices, []int{1}) {
		t.Fatalf("tie: targetFromScores = %+v, %v; want [1]", got, ok)
	}
}

// TestPolicyNetBlockersShapes pins blockersFromScores on crafted decisions:
// one pair per blocker (the option Group), the bot's declaration order kept
// first, the exact-tie refusal, and the CR 509.1a MinBlockers guard.
func TestPolicyNetBlockersShapes(t *testing.T) {
	blk := func(idx int, blocker, attacker state.ObjID) decision.Option {
		return decision.Option{Index: idx, Kind: "block", Obj: blocker, Attacker: attacker, Player: 0,
			Group: "blocker:" + string(rune('0'+blocker))}
	}
	board := boardFromView(view.View{Viewer: 0, Active: 1, Phase: "combat"})
	// Blockers 1 and 2, each able to block attackers 10 and 11.
	d := &decision.Decision{Kind: decision.KBlockers, Min: 0, Max: 4, Seq: 5, Player: 0,
		Options: []decision.Option{blk(0, 1, 10), blk(1, 1, 11), blk(2, 2, 10), blk(3, 2, 11)}}

	// Straddling scores: blocker 1's better admitted pair (index 1), blocker
	// 2's only admitted pair (index 2); offer order when the bot declared
	// neither.
	in, ok := blockersFromScores(d, []float32{1, 3, 2, -1}, board, decision.Intent{})
	if !ok || !slices.Equal(in.Choices, []int{1, 2}) {
		t.Fatalf("one-per-blocker: %+v, %v; want [1 2]", in, ok)
	}
	if err := d.Validate(in); err != nil {
		t.Fatalf("one-per-blocker: Validate: %v", err)
	}
	// The bot's own pairs lead, in the bot's order.
	in, ok = blockersFromScores(d, []float32{1, 3, 2, -1}, board, decision.Intent{Choices: []int{2, 1}})
	if !ok || !slices.Equal(in.Choices, []int{2, 1}) {
		t.Fatalf("bot-order: %+v, %v; want [2 1]", in, ok)
	}
	// An exact tie refuses.
	if _, ok := blockersFromScores(d, []float32{0, 0, 0, 0}, board, decision.Intent{}); ok {
		t.Fatal("tie: answered an exactly-tied blockers decision; want delegation")
	}
	// Nothing admitted: the empty declaration is a legal answer.
	in, ok = blockersFromScores(d, []float32{-1, -2, -3, -4}, board, decision.Intent{})
	if !ok || d.Validate(in) != nil {
		t.Fatalf("all-negative: %+v, %v", in, ok)
	}
	if len(in.Choices) == 0 || len(in.Choices) == len(d.Options) {
		// the mean fallback ranks: some but not all options
		t.Fatalf("all-negative: choices %v, want a proper nonempty subset", in.Choices)
	}

	// Attacker 10 has menace (MinBlockers 2): a lone admitted blocker on it
	// is dropped by the guard, the other pair stays.
	m := &decision.Decision{Kind: decision.KBlockers, Min: 0, Max: 4, Seq: 6, Player: 0,
		Options: []decision.Option{blk(0, 1, 10), blk(1, 2, 11), blk(2, 3, 11)}}
	m.Options[0].MinBlockers, m.Options[0].MaxBlockers = 2, 0
	in, ok = blockersFromScores(m, []float32{3, 1, -5}, board, decision.Intent{})
	if !ok || !slices.Equal(in.Choices, []int{1}) {
		t.Fatalf("menace guard: %+v, %v; want [1]", in, ok)
	}
}

// countingSeat wraps an answerer and counts the kinds it answered.
type countingSeat struct {
	inner answerer
	kinds map[decision.Kind]int
}

func (c *countingSeat) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	c.kinds[d.Kind]++
	return c.inner.Decide(ctx, v, d)
}

// TestPolicyNetBlockersTargetsResidualReproducesTheBot: a zero head with
// ResidualW 3 scoring blockers and target answers every scored decision of
// those kinds with EXACTLY the default bot's intent (declaration order
// included) over whole games. Premise guards: at least one blockers decision
// the scored path ANSWERED (a bot declaration with a block — a no-block bot
// answer ties and delegates) and at least one in-shape target decision.
func TestPolicyNetBlockersTargetsResidualReproducesTheBot(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 2)
	kinds := []decision.Kind{decision.KBlockers, decision.KTarget}
	blockAnswered, blockCompared, targetInShape := 0, 0, 0
	for _, seed := range []uint64{9, 10, 11, 12} {
		e := rules.New(rules.Config{Seed: seed, Names: names, Decks: decks})
		e.Advance()
		pns := map[state.PlayerID]*PolicyNetBot{}
		for k := range decks {
			pns[state.PlayerID(k)] = zeroResidualBot(seed^uint64(k+1), 3, kinds)
		}
		n := 0
		for !e.G.Over && e.Pending() != nil && n < 20000 {
			d := e.Pending()
			v := view.Project(e.G, e, d.Player, d)
			pn := pns[d.Player]
			pnIn, err := pn.Decide(context.Background(), v, *d)
			if err != nil {
				t.Fatalf("seed %d intent %d: policynet: %v", seed, n, err)
			}
			def, err := pn.def.Decide(context.Background(), v, *d)
			if err != nil {
				t.Fatalf("seed %d intent %d: default bot: %v", seed, n, err)
			}
			if pn.scoresKind(d) {
				if !sameIntent(pnIn, def) {
					t.Fatalf("seed %d intent %d (kind %q): residual policynet %+v, default bot %+v — the prior did not reproduce the bot", seed, n, d.Kind, pnIn, def)
				}
				switch d.Kind {
				case decision.KBlockers:
					blockCompared++
					if len(def.Choices) > 0 {
						blockAnswered++
					}
				case decision.KTarget:
					if targetScorable(d, def, 3) {
						targetInShape++
					}
				}
			}
			if err := e.Submit(def); err != nil {
				t.Fatalf("seed %d intent %d: submit: %v", seed, n, err)
			}
			n++
		}
	}
	if blockAnswered == 0 {
		t.Fatalf("no blockers decision with a bot block was compared (%d blockers decisions in all) — the blockers prior is unpinned", blockCompared)
	}
	if targetInShape == 0 {
		t.Fatal("no in-shape target decision was compared — the target prior is unpinned")
	}
	t.Logf("blockers compared %d (bot blocked in %d); in-shape target decisions %d", blockCompared, blockAnswered, targetInShape)
}

// TestPolicyNetBlockersTargetsRandomHeadLegal: a random head with an active
// prior scoring attackers, blockers and target plays whole games against
// the default bot without one illegal intent (the engine's Submit is the
// validator), reaches both new kinds, and plays a different stream from
// the default bot (the scored kinds reach the scorer).
func TestPolicyNetBlockersTargetsRandomHeadLegal(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 2)
	kinds := []decision.Kind{decision.KAttackers, decision.KBlockers, decision.KTarget}
	seen := map[decision.Kind]int{}
	for _, seed := range []uint64{200, 201, 202, 203} {
		sc := withResidual(t, seed, 0.001)
		defRun := driveBotGame(t, seed, names, decks, func(p state.PlayerID) answerer {
			return NewBot(seed ^ uint64(p+1))
		})
		pnRun := driveBotGame(t, seed, names, decks, func(p state.PlayerID) answerer {
			return &countingSeat{inner: NewPolicyNetBotKinds(seed^uint64(p+1), sc, kinds), kinds: seen}
		})
		if slices.EqualFunc(defRun, pnRun, sameIntent) {
			t.Fatalf("seed %d: the policynet run reproduced the default bot's intent stream exactly", seed)
		}
	}
	if seen[decision.KBlockers] == 0 || seen[decision.KTarget] == 0 {
		t.Fatalf("the games never asked both new kinds: %v", seen)
	}
	t.Logf("kinds answered: %v", seen)
}

// TestPolicyNetRecorderObservesWithoutChangingPlay pins the on-policy
// recorder (ticket pn13): whole games played with a recorder installed
// submit exactly the intent stream of the same seat without one; every
// record is a decision the seat answered FROM ITS SCORES (its Chosen is the
// submitted intent, in option positions); the recorded scores are the
// scorer's own on the recorded state and options; and the action space and
// distribution shape follow the kind.
func TestPolicyNetRecorderObservesWithoutChangingPlay(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 2)
	kinds := []decision.Kind{decision.KAttackers, decision.KPriority, decision.KBlockers, decision.KTarget}
	seenKinds := map[decision.Kind]int{}
	for _, seed := range []uint64{300, 301, 302} {
		sc := withResidual(t, seed, 0.5)
		plain := driveBotGame(t, seed, names, decks, func(p state.PlayerID) answerer {
			return NewPolicyNetBotKinds(seed^uint64(p+1), sc, kinds)
		})
		var recs []PolicyNetDecision
		recorded := driveBotGame(t, seed, names, decks, func(p state.PlayerID) answerer {
			b := NewPolicyNetBotKinds(seed^uint64(p+1), sc, kinds)
			b.SetRecorder(func(d PolicyNetDecision) { recs = append(recs, d) })
			return b
		})
		if !slices.EqualFunc(plain, recorded, sameIntent) {
			t.Fatalf("seed %d: installing the recorder changed the intent stream", seed)
		}
		if len(recs) == 0 {
			t.Fatalf("seed %d: nothing recorded", seed)
		}
		bySeq := map[[2]uint64]decision.Intent{}
		for _, in := range recorded {
			bySeq[[2]uint64{in.Seq, uint64(in.Player)}] = in
		}
		for _, r := range recs {
			seenKinds[r.Kind]++
			in, ok := bySeq[[2]uint64{r.Seq, uint64(r.Player)}]
			if !ok {
				t.Fatalf("seed %d: record seq %d has no submitted intent", seed, r.Seq)
			}
			// Index == position for engine-built options (rules/legal.go).
			if !slices.Equal(r.Chosen, in.Choices) {
				t.Fatalf("seed %d seq %d %s: recorded chosen %v, submitted %v", seed, r.Seq, r.Kind, r.Chosen, in.Choices)
			}
			if got := sc.Score(r.State, r.Options); !slices.Equal(got, r.Scores) {
				t.Fatalf("seed %d seq %d: recorded scores differ from a re-score", seed, r.Seq)
			}
			if r.Subset != (r.Kind == decision.KAttackers || r.Kind == decision.KBlockers) || len(r.InSpace) != len(r.Options) {
				t.Fatalf("seed %d seq %d %s: subset %v / in-space %d of %d", seed, r.Seq, r.Kind, r.Subset, len(r.InSpace), len(r.Options))
			}
			for i, b := range r.InSpace {
				if r.Kind != decision.KPriority && !b {
					t.Fatalf("seed %d seq %d %s: option %d outside the action space", seed, r.Seq, r.Kind, i)
				}
			}
			for _, c := range r.BotChosen {
				if !r.Options[c].BotPick {
					t.Fatalf("seed %d seq %d: bot answer position %d not BotPick-marked", seed, r.Seq, c)
				}
			}
		}
	}
	if seenKinds[decision.KAttackers] == 0 || seenKinds[decision.KPriority] == 0 {
		t.Fatalf("the recorder never saw attackers and priority: %v", seenKinds)
	}
	t.Logf("recorded kinds: %v", seenKinds)
}

// TestPolicyNetSignAdmission pins the opt-in sign vote (ticket pn13): on an
// ALL-POSITIVE multi-option attackers decision the default auto vote falls
// back to the decision mean and drops the below-mean attackers, while the
// sign vote admits every positive option; a straddling decision votes the
// same under both; an exact positive tie, which auto refuses, is admitted
// whole; and an unknown mode is refused.
func TestPolicyNetSignAdmission(t *testing.T) {
	atkOpt := func(idx int, obj state.ObjID) decision.Option {
		return decision.Option{Index: idx, Kind: "attacker", Obj: obj, Player: 1}
	}
	d := &decision.Decision{Kind: decision.KAttackers, Min: 0, Max: 3, Seq: 12, Player: 0,
		Options: []decision.Option{atkOpt(0, 10), atkOpt(1, 11), atkOpt(2, 12)}}
	auto, _ := attackersFromScoresVote(d, []float32{2.5, 1.0, 2.0}, false)
	sign, _ := attackersFromScoresVote(d, []float32{2.5, 1.0, 2.0}, true)
	if !slices.Equal(auto.Choices, []int{0, 2}) || !slices.Equal(sign.Choices, []int{0, 1, 2}) {
		t.Fatalf("all-positive: auto %v sign %v, want [0 2] and [0 1 2]", auto.Choices, sign.Choices)
	}
	a2, _ := attackersFromScoresVote(d, []float32{1, -1, 0.5}, false)
	s2, _ := attackersFromScoresVote(d, []float32{1, -1, 0.5}, true)
	if !slices.Equal(a2.Choices, s2.Choices) || !slices.Equal(s2.Choices, []int{0, 2}) {
		t.Fatalf("straddle: auto %v sign %v", a2.Choices, s2.Choices)
	}
	if _, ok := attackersFromScoresVote(d, []float32{2, 2, 2}, false); ok {
		t.Fatal("auto must refuse an exact tie")
	}
	if in, ok := attackersFromScoresVote(d, []float32{2, 2, 2}, true); !ok || len(in.Choices) != 3 {
		t.Fatalf("sign on a positive tie: %v %v", in.Choices, ok)
	}
	b := NewPolicyNetBot(1, nil)
	if err := b.SetAdmission("mean"); err == nil || b.admission() != AdmissionAuto {
		t.Fatal("an unknown admission must be refused and leave auto in place")
	}
	if err := b.SetAdmission(AdmissionSign); err != nil || b.admission() != AdmissionSign {
		t.Fatal("sign admission not set")
	}
}

// TestPolicyNetSignAdmissionReproducesTheBotWithAZeroHead: with the sign
// vote, a zero-head model with an active residual answers attackers exactly
// as the default bot does over whole games (the residual prior's contract,
// now without the auto vote's tie refusal).
func TestPolicyNetSignAdmissionReproducesTheBotWithAZeroHead(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 2)
	m := policynet.NewModel(policynet.TableRows, 8, 4, rand.New(rand.NewPCG(1, 1)))
	for _, blk := range [][]float32{m.Table, m.StateW, m.StateB, m.HidW, m.HidB, m.OutW} {
		for i := range blk {
			blk[i] = 0
		}
	}
	m.ResidualW = 2
	sc := policynet.NewScorer(m)
	for _, seed := range []uint64{400, 401} {
		defRun := driveBotGame(t, seed, names, decks, func(p state.PlayerID) answerer { return NewBot(seed ^ uint64(p+1)) })
		pnRun := driveBotGame(t, seed, names, decks, func(p state.PlayerID) answerer {
			b := NewPolicyNetBot(seed^uint64(p+1), sc)
			if err := b.SetAdmission(AdmissionSign); err != nil {
				t.Fatal(err)
			}
			return b
		})
		if !slices.EqualFunc(defRun, pnRun, sameIntent) {
			t.Fatalf("seed %d: a zero head under the sign vote diverged from the default bot", seed)
		}
	}
}
