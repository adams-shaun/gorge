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
//   - KAttackers: one scored subset — an independent per-option inclusion
//     vote (admissionThreshold: the calibrated sigmoid boundary 0 when the
//     decision's scores straddle it, else the decision's own mean, so the
//     vote never degenerates to all-in or empty on an untrusted offset),
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
//     delegates).
//
// Every other decision kind delegates to the current default bot (NewBot,
// same seed derivation) UNCHANGED — so the policy consumes no rng of its
// own, and a decision whose scored surface encodes to nothing (no options,
// no scorer, no cast/ability/pass option at priority) falls back too.
// Deterministic: no wall clock, no map-iteration order in any answer; ties
// break on option index everywhere.

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
// everything else.
func (b *PolicyNetBot) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	if scoredKind(&d) {
		st, opts, ok := b.encode(v, &d)
		if ok {
			scores := b.scorer.Score(st, opts)
			var in decision.Intent
			var ok bool
			switch d.Kind {
			case decision.KAttackers:
				in, ok = attackersFromScores(&d, scores)
			case decision.KPriority:
				in, ok = priorityFromScores(&d, scores)
			}
			if ok {
				return in, nil
			}
		}
	}
	return b.def.Decide(ctx, v, d)
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
	// inclusive is true only on the mean fallback: an exactly-tied decision has
	// no ranking to break the tie, so an option at the mean is declared rather
	// than dropped (the calibrated sign stays strict, so an exactly-zero vote
	// is not declared).
	threshold, inclusive := admissionThreshold(scores)

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

// priorityFromScores turns the per-option scores into the scored cast
// choice: argmax over the options the teacher's candidate space covered
// (cast / ability / pass — searchprobe.Candidates' pool), ties on the
// lowest option index. ok is false when the decision offers none of those —
// the untrained-surface shape that delegates to the default bot (whose tap
// gate and land drop then answer, as they do for every delegated kind).
func priorityFromScores(d *decision.Decision, scores []float32) (decision.Intent, bool) {
	if len(d.Options) == 0 || len(scores) != len(d.Options) {
		return decision.Intent{}, false
	}
	best := -1
	var bestScore float32
	for i := range d.Options {
		switch d.Options[i].Kind {
		case "cast", "ability", "pass":
		default:
			continue
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

// sigmoid is the logistic function in float64 over the float32 score —
// deterministic. Retained for callers that reason about the calibrated
// probability directly (and for the tests that pin the 0.5 boundary); the
// admission rule itself now compares against admissionThreshold rather than
// an absolute zero.
func sigmoid(x float32) float64 {
	return 1 / (1 + math.Exp(-float64(x)))
}
