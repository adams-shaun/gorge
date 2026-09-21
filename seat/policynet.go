package seat

// The L9c learned-policy seat: a trained per-option scorer
// (internal/policynet) plugged in at the seat layer, exactly like the bench
// bots above. botpolicy cannot import view (dependency order,
// internal/archtest), so the encoder — which reads a projected view.View —
// is reached from here, where the view is already in hand.
//
// What the policy answers itself is ONLY the KAttackers subset. KPriority is
// trained and fully wired, but DELEGATED — see the KPriority note below for
// the measurement that forces it.
//
//   - KAttackers: one scored subset. The per-option inclusion vote compares
//     each score against admissionThreshold's per-decision reference: the
//     calibrated boundary 0 when the decision's scores STRADDLE it, else the
//     decision's own mean. The absolute boundary is meaningful only because
//     the attackers kind is trained with a per-option binary (BCE) loss that
//     calibrates score 0 at inclusion probability 0.5 (LossBCE, the
//     -kind-loss attackers=bce default); the mean is the fallback for a
//     checkpoint whose offset is NOT trustworthy — a softmax cross-entropy
//     head is shift-invariant, so a one-signed range carries no absolute
//     information and the vote falls back to the within-decision ranking.
//     An EXACTLY TIED decision has neither a sign nor a ranking, so the seat
//     refuses it and the default bot's declaration stands. Those three arms
//     between them mean no checkpoint offset can turn a decision all-in or
//     empty, which is the L9d defect this rule replaced. The admitted subset
//     is then repaired the way chooseAttackersMode repairs it: one option per
//     attacker (CR 506.2), every required attacker declared (CR 508.1d) and
//     the Max ceiling applied required-first (CR 508.1j).
//   - KPriority: DELEGATED, and delegation is load-bearing, not a tidy-up.
//     Measured on the merged tree (this file, with the residual prior's
//     inference half below present and the attackers fix in place): with
//     priority SCORED the seat wins 0/1000 against the default bot at mean
//     13.4 turns; with priority DELEGATED and nothing else changed it wins
//     474/1000 (47.4% [44.3%, 50.5%]) against the bot self-control's 51.5%
//     [48.4%, 54.6%]. (Ten approved mono pairs, 100 games/pair, seed
//     10000000, checkpoint bce-big.gpol.) Two reasons the scored path loses:
//     its surface omits the tap ("activate") and play_land options the
//     teacher never labelled, so the argmax cannot answer the whole decision;
//     and the head is worse than the bot even at the cast/ability/pass
//     ternary it does cover (0.355-0.371 top-1 vs a 0.678 bot baseline). In
//     play it vetoes the bot's own casts and starves its development.
//
//     The residual prior (below) is the intended fix for exactly this, and
//     priorityFromScores implements its admission rule — but it can only help
//     a model with a POSITIVE ResidualW, and that weight only exists in a
//     schema-v2 checkpoint. Every checkpoint available to this branch is v1
//     and loads ResidualW == 0 (measured), so the prior is inactive, the
//     BotPick mark is a documented no-op, and priorityFromScores reduces
//     exactly to the pre-existing argmax the 0/1000 above measures. The
//     delegation therefore CANNOT be cleared on any checkpoint that exists
//     today; re-scoring priority needs a ResidualW > 0 checkpoint and a bench
//     that beats the bar above, not just the wiring. scoredKind is the single
//     switch, and TestPolicyNetDelegatesPriority pins it. The head's
//     learnability is owned by agent-20260921T012459Z-cb7a7077; see
//     docs/superpowers/plans/2026-09-19-learned-cast-profile.md.
//
// Every other decision kind delegates to the current default bot (NewBot,
// same seed derivation) UNCHANGED — so the policy consumes no rng of its
// own, and a decision whose scored surface encodes to nothing (no options,
// no scorer) or carries no information (an exactly-tied attackers decision)
// falls back too. Deterministic: no wall clock, no map-iteration order in
// any answer; ties break on option index everywhere.
//
// The residual prior at INFERENCE (the train/eval symmetry the L9b-fix2
// follow-up review demanded): for a scored kind this bot first asks the
// wrapped default bot the same question and marks its answer's options
// (Option.BotPick) before scoring, so a model trained with a positive
// ResidualInit scores the same contract it trained under and reproduces the
// bot baseline unless the learned head overrides. The default bot's answer
// is also the fallback whenever the scored surface declines to answer. No
// scored kind consumes the default bot's rng (both answers are pure
// functions of the offered options and the board facts), so the marking
// changes no rng stream. With the prior inactive (ResidualWeight 0 — every
// v1-schema checkpoint, which today is all of them) the mark is a no-op:
// scores and answers are exactly the pre-wiring ones, and the residual is
// then honestly a train/eval-only device with the delegation path as the
// deployed fallback. Since KPriority now delegates, the mark reaches only
// KAttackers in this build.

import (
	"context"

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
	return d.Kind == decision.KAttackers
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
// declaration: the per-option inclusion vote (dropping every option at or
// below the decision's admission threshold) plus the legality repair
// chooseAttackersMode already does. ok is false only when no option was
// scored (the empty decision).
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
	// The per-decision admission threshold: the calibrated sigmoid boundary
	// (0) when the decision holds options on BOTH sides of it — the head can
	// and does separate included from excluded — else the decision's own mean,
	// a reference shift-invariant like the loss, so a checkpoint whose scores
	// are all one sign (the measured failure this rule exists to stop) ranks
	// options against each other instead of admitting every one or none.
	threshold, inclusive := admissionThreshold(scores)

	// An EXACTLY TIED multi-option decision carries no information at all:
	// not a calibrated sign (the tie is one-signed by construction) and not a
	// ranking. Declaring the mean-inclusive subset there means declaring EVERY
	// option — the all-in failure this whole rule exists to prevent, reached
	// by a different road. So refuse to answer and let the caller use the
	// default bot's declaration, which is the best available evidence about a
	// board the head has nothing to say about. This is also what makes the
	// residual prior's contract hold at attackers: a zero-head model whose bot
	// picked NO attacker marks no option, ties every score, and must reproduce
	// the bot's empty declaration rather than swing the whole board.
	if len(scores) > 1 && inclusive && allEqual(scores) {
		return decision.Intent{}, false
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
		// The per-option inclusion vote, relative to a shift-invariant
		// reference (admissionThreshold): an absolute score > 0 means "the
		// teacher includes this option" only for a head whose loss trains the
		// score LEVEL (lossBCE); a head trained with argmax CE is a softmax
		// logit — shift-invariant, absolute level untrained — so when the whole
		// decision lands on one side of zero the sign carries no information
		// and the vote falls back to the decision's own ranking.
		if inclusive {
			// The mean fallback on a VARYING one-signed range: inclusive so a
			// decision whose scores cluster at the mean still declares. An
			// exact tie never reaches here (it delegated above).
			return s >= threshold
		}
		return s > threshold
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

// priorityFromScores is RETAINED BUT CURRENTLY UNREACHABLE: scoredKind no
// longer admits KPriority (the scored path measures 0/1000 in play — see the
// package comment), so nothing calls this today. It is kept, not deleted,
// because it implements the residual prior's priority admission rule and is
// the exact code a ResidualW > 0 checkpoint needs; deleting it would make
// re-enabling the kind a rewrite rather than a one-line scoredKind change.
// Its behaviour is pinned by TestPolicyNetResidualReproducesTheBotInPlay
// only for the arms that reach it.
//
// It turns the per-option scores into the scored cast choice: argmax over the options the teacher's candidate space covered
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

// admissionThreshold returns the reference score for the per-option admission
// vote and whether the comparison against it is inclusive.
//
// The seat's admission is a per-option inclusion vote. Its historical form —
// sigmoid(score) > 0.5, i.e. score > 0 — is correct only when the scorer's
// absolute level is trained: a per-option binary (BCE) head calibrates score
// 0 at the inclusion probability 0.5. A softmax cross-entropy head trains
// only the ORDER inside a decision (its loss is shift-invariant), so its
// output has no calibrated zero and score > 0 admits everything — measured
// on the controller's dev2 checkpoint, 100% of 9,016 labelled attack options
// scored in [11575, 15052], so the seat declared every legal attacker every
// combat.
//
// The threshold is therefore chosen to be a no-op under shift:
//
//   - a decision whose options straddle zero uses the calibrated boundary 0
//     (strict), which is the trained vote for a BCE head and a harmless
//     ordering centre for any head;
//   - a decision whose options all share one sign uses the decision's own
//     mean (inclusive), so an uncalibrated offset shifts every option equally
//     and the vote is decided by the within-decision ranking instead of the
//     untrusted absolute level. Inclusive because an exactly-tied decision
//     has no ranking to break the tie, so its options are declared rather
//     than dropped.
//
// Both branches use only within-decision structure, so neither can admit
// every option of a multi-option decision solely because the checkpoint's
// offset is positive, nor admit none solely because it is negative.
func admissionThreshold(scores []float32) (float32, bool) {
	if len(scores) <= 1 {
		// A single option offers no within-decision ranking, so the calibrated
		// sign is the only vote there is: a lone positive score is declared and
		// a lone negative one is not (the shapes seat/policynet_test.go pins).
		return 0, false
	}
	pos, neg := 0, 0
	for _, s := range scores {
		if s > 0 {
			pos++
		} else {
			neg++
		}
	}
	if pos > 0 && neg > 0 {
		return 0, false
	}
	sum := float64(0)
	for _, s := range scores {
		sum += float64(s)
	}
	return float32(sum / float64(len(scores))), true
}

// allEqual reports whether every score is bit-for-bit the same value — the
// "no ranking whatsoever" case attackersFromScores refuses to answer.
func allEqual(scores []float32) bool {
	for _, s := range scores {
		if s != scores[0] {
			return false
		}
	}
	return true
}
