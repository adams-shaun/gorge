package seat

// The L9c learned-policy seat: a trained per-option scorer
// (internal/policynet) plugged in at the seat layer, exactly like the bench
// bots above. botpolicy cannot import view (dependency order,
// internal/archtest), so the encoder — which reads a projected view.View —
// is reached from here, where the view is already in hand.
//
// What the policy answers itself is ONLY what the search teacher labelled
// (cmd/searchteacher's kinds):
//
//   - KAttackers: one scored subset — an independent sigmoid per offered
//     (attacker, defender) option, admitted at sigmoid > 0.5 (score > 0),
//     then the legality repair chooseAttackersMode already does: one option
//     per attacker (CR 506.2), every required attacker declared (CR 508.1d)
//     and the Max ceiling applied required-first (CR 508.1j);
//   - KPriority: the scored cast choice — argmax over the options the
//     teacher's candidate space covered (cast / ability / pass, the exact
//     space searchprobe.Candidates drew its labelled candidates from), ties
//     on option index. The scored surface deliberately does NOT include the
//     tap ("activate") and play_land options the teacher never labelled:
//     their scores are untrained, so the kind falls back to the default bot
//     the moment no cast/ability/pass option is offered (and the default
//     bot's tap gate and land drop keep answering every decision this bot
//     delegates). ONE exception, the residual prior's inference half: when
//     the model carries a positive bot-prior residual weight
//     (Scorer.ResidualWeight, train.Config.ResidualInit), the default bot's
//     own answer is marked (Option.BotPick) and admitted into the argmax
//     even when it is a tap/land option — the untrained noise on those
//     options must not outvote the bot's own action, or the prior's
//     "starts at the bot baseline" contract breaks exactly on the decisions
//     the head was never trained for.
//
// Every other decision kind delegates to the current default bot (NewBot,
// same seed derivation) UNCHANGED — so the policy consumes no rng of its
// own, and a decision whose scored surface encodes to nothing (no options,
// no scorer, no cast/ability/pass option at priority) falls back too.
// Deterministic: no wall clock, no map-iteration order in any answer; ties
// break on option index everywhere.
//
// The residual prior at INFERENCE (the train/eval symmetry the L9b-fix2
// follow-up review demanded): for a scored kind this bot first asks the
// wrapped default bot the same question and marks its answer's options
// (Option.BotPick) before scoring, so a model trained with a positive
// ResidualInit scores the same contract it trained under and reproduces the
// bot baseline unless the learned head overrides. Neither scored kind
// consumes the default bot's rng (both answers are pure functions of the
// offered options and the board facts), so the marking changes no rng
// stream. With the prior inactive (ResidualWeight 0 — every checkpoint the
// v1 schema carries) the mark is a no-op: scores and answers are exactly
// the pre-wiring ones, and the residual is then honestly a train/eval-only
// device with the ok==false delegation path as the deployed fallback.

import (
	"context"
	"math"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/policynet"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// PolicyNetBot answers decisions with a trained scorer, delegating every
// kind the scorer does not cover to the default bot it wraps. It implements
// seat.Seat only — never BoardSeat — because its scored surface is encoded
// from the projected view.View; the host's BoardSeat fast path would skip
// the projection the encoder reads.
type PolicyNetBot struct {
	def    *Bot
	scorer *policynet.Scorer
}

// compile-time assertions: PolicyNetBot is a Seat and deliberately NOT a
// BoardSeat (its encoder needs the projected View).
var (
	_ Seat = (*PolicyNetBot)(nil)
)

// NewPolicyNetBot wraps the default bot (same PCG seed derivation as
// NewBot, so the delegation path consumes exactly the rng the default bot
// would) with the given scorer. The scorer is owned by the caller; a Scorer
// is single-threaded, so a bench worker builds one per seat per game over a
// shared read-only Model.
func NewPolicyNetBot(seed uint64, sc *policynet.Scorer) *PolicyNetBot {
	return &PolicyNetBot{def: NewBot(seed), scorer: sc}
}

// scoredKinds reports whether the scorer answers this decision kind.
func scoredKind(d *decision.Decision) bool {
	return d.Kind == decision.KAttackers || d.Kind == decision.KPriority
}

// encode runs the fixed encoder over the seat's view and the offered
// options. ok is false when the scored surface is empty — no scorer, or a
// decision with no options at all — which is the documented fall-back-to-
// the-default-bot shape ("a decision whose options encode to nothing").
func (b *PolicyNetBot) encode(v view.View, d *decision.Decision) (policynet.State, []policynet.Option, bool) {
	if b.scorer == nil || len(d.Options) == 0 {
		return policynet.State{}, nil, false
	}
	st := policynet.EncodeState(v, d.Player)
	opts := make([]policynet.Option, len(d.Options))
	for i := range d.Options {
		opts[i] = policynet.EncodeOption(v, d.Player, d.Kind, d.Options[i], i, len(d.Options))
	}
	return st, opts, true
}

// Decide answers d: the two scored kinds above, the default bot for
// everything else. For a scored kind the wrapped default bot is asked FIRST
// (its answer marks the residual prior's BotPick and is the fallback when
// the scored surface cannot answer) — see the residual-prior paragraph
// above for the determinism and no-op-when-inactive contract.
func (b *PolicyNetBot) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	if scoredKind(&d) {
		botIn, err := b.def.Decide(ctx, v, d)
		if err != nil {
			return decision.Intent{}, err
		}
		st, opts, ok := b.encode(v, &d)
		if ok {
			markBotPicks(&d, opts, botIn)
			scores := b.scorer.Score(st, opts)
			var in decision.Intent
			var scored bool
			switch d.Kind {
			case decision.KAttackers:
				in, scored = attackersFromScores(&d, scores)
			case decision.KPriority:
				in, scored = priorityFromScores(&d, opts, scores, b.scorer.ResidualWeight() > 0)
			}
			if scored {
				return in, nil
			}
		}
		return botIn, nil
	}
	return b.def.Decide(ctx, v, d)
}

// markBotPicks sets Option.BotPick on every option whose Index the given
// intent chose — the inference half of the residual prior's "the bot's own
// answer" definition (loader.botPicks is the training half, from the label
// corpus's bot candidate). Choices carry option Index values (rules/legal.go
// builds every option with Index == its position and both bots answer in
// those terms), so a membership set over the intent's choices is the whole
// map. Deterministic: the map is only ever probed, never ranged.
func markBotPicks(d *decision.Decision, opts []policynet.Option, in decision.Intent) {
	picked := make(map[int]bool, len(in.Choices))
	for _, c := range in.Choices {
		picked[c] = true
	}
	for i := range opts {
		if picked[d.Options[i].Index] {
			opts[i].BotPick = true
		}
	}
}

// attackersFromScores turns the per-option scores into a legal attack
// declaration: the sigmoid subset (score > 0 per option) plus the legality
// repair chooseAttackersMode already does. ok is false only when no option
// was scored (the empty decision).
func attackersFromScores(d *decision.Decision, scores []float32) (decision.Intent, bool) {
	if len(d.Options) == 0 || len(scores) != len(d.Options) {
		return decision.Intent{}, false
	}
	// CR 508.1d requirements travel on the options (Option.Required): every
	// option naming a required attacker carries the mark (rules/combat.go's
	// offer loop).
	required := make(map[state.ObjID]bool, len(d.Options))
	for i := range d.Options {
		if d.Options[i].Required {
			required[d.Options[i].Obj] = true
		}
	}
	// Group the options by attacker in first-seen (offer) order; the map is
	// only ever read by key, never ranged.
	type atk struct{ opts []int }
	byID := make(map[state.ObjID]*atk, len(d.Options))
	var order []state.ObjID
	for i := range d.Options {
		at, ok := byID[d.Options[i].Obj]
		if !ok {
			at = &atk{}
			byID[d.Options[i].Obj] = at
			order = append(order, d.Options[i].Obj)
		}
		at.opts = append(at.opts, i)
	}

	// The scored subset, repaired to one option per attacker (CR 506.2 —
	// the engine rejects a declaration naming one attacker twice): each
	// attacker keeps its best-scoring ADMITTED option, ties on the lowest
	// option index. A required attacker with nothing admitted declares its
	// best-scoring option outright (the repair; a required 0-admission
	// attacker that stayed home would make the whole declaration illegal).
	bestOf := func(ois []int, admitted func(float32) bool) int {
		best, bestScore := -1, float32(0)
		for _, oi := range ois {
			s := scores[oi]
			if !admitted(s) {
				continue
			}
			if best == -1 || s > bestScore {
				best, bestScore = oi, s
			}
		}
		if best == -1 {
			for _, oi := range ois {
				if best == -1 || scores[oi] > bestScore {
					best, bestScore = oi, scores[oi]
				}
			}
		}
		return best
	}
	admitted := func(s float32) bool {
		// The per-option sigmoid gate, at the 0.5 boundary: the trained
		// scorer's independent per-option inclusion vote.
		return sigmoid(s) > 0.5
	}
	chosen := make([]int, 0, len(order))
	for _, obj := range order {
		at := byID[obj]
		oi := bestOf(at.opts, admitted)
		if !admitted(scores[oi]) && !required[obj] {
			continue
		}
		chosen = append(chosen, oi)
	}

	// The Max ceiling (CR 508.1j), required-first — the chooseAttackersMode
	// shape, so the truncation never drops a required attacker while a free
	// one stays. chosen is in the attackers' first-seen order, so the
	// re-ordering is a stable partition, not a sort.
	if d.Max < len(chosen) && d.Max >= 0 {
		ordered := make([]int, 0, len(chosen))
		for _, oi := range chosen {
			if required[d.Options[oi].Obj] {
				ordered = append(ordered, oi)
			}
		}
		for _, oi := range chosen {
			if !required[d.Options[oi].Obj] {
				ordered = append(ordered, oi)
			}
		}
		chosen = ordered[:d.Max]
	}
	return botpolicy.Clamp(d, decision.Intent{Seq: d.Seq, Player: d.Player, Choices: chosen}), true
}

// priorityFromScores turns the per-option scores into the scored cast
// choice: argmax over the options the teacher's candidate space covered
// (cast / ability / pass — searchprobe.Candidates' pool), ties on the
// lowest option index. When admitBotPicks is set (the residual prior is
// active), an option marked BotPick — the wrapped default bot's own answer,
// marked by markBotPicks — is admissible in the argmax EVEN when its Kind is
// the untrained tap/land surface: with the prior on, the bot's own action
// carries the prior and wins unless the head overrides it; without the
// prior, the untrained surface stays excluded exactly as before. ok is
// false when the decision offers none of those — the untrained-surface
// shape that falls back to the already-computed default-bot answer (whose
// tap gate and land drop answer, as they do for every delegated kind).
func priorityFromScores(d *decision.Decision, opts []policynet.Option, scores []float32, admitBotPicks bool) (decision.Intent, bool) {
	if len(d.Options) == 0 || len(scores) != len(d.Options) {
		return decision.Intent{}, false
	}
	best := -1
	var bestScore float32
	for i := range d.Options {
		switch d.Options[i].Kind {
		case "cast", "ability", "pass":
		default:
			if !(admitBotPicks && opts[i].BotPick) {
				continue
			}
		}
		if best == -1 || scores[i] > bestScore {
			best, bestScore = i, scores[i]
		}
	}
	if best == -1 {
		return decision.Intent{}, false
	}
	in := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{d.Options[best].Index}}
	return botpolicy.Clamp(d, in), true
}

// sigmoid is the logistic function in float64 over the float32 score —
// deterministic, and exactly 0.5 at score 0 (so the > 0.5 admission gate
// excludes an exactly-zero vote: an option the trained scorer cannot
// distinguish from nothing is not declared).
func sigmoid(x float32) float64 {
	return 1 / (1 + math.Exp(-float64(x)))
}
