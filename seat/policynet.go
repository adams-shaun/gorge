package seat

// The L9c learned-policy seat: a trained per-option scorer
// (internal/policynet) plugged in at the seat layer, exactly like the bench
// bots above. botpolicy cannot import view (dependency order,
// internal/archtest), so the encoder — which reads a projected view.View —
// is reached from here, where the view is already in hand.
//
// Which kinds the policy answers itself is a per-seat selection
// (NewPolicyNetBotKinds; botbench -policynet-kinds). The DEFAULT is the
// KAttackers subset only; KPriority, KBlockers and KTarget are opt-in, and
// the single-choice kinds are gated to the distribution the head was trained
// on.
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
//   - KPriority (opt-in): scored ONLY when priorityScorable holds — the
//     checkpoint's ResidualW > 0, the decision offers >= 2 distinct castable
//     objects (botpolicy.CastableObjects, the count searchseat.Eligible gates
//     the teacher's labels on) and the default bot's own answer is one
//     cast/ability/pass option (the only bot answers the teacher labels).
//     Every other priority decision returns the bot's answer unscored.
//
//     Why the gate, measured (pn01; ten approved mono pairs, 100 games/pair,
//     seed 10000000, 1000 games per arm; control bot-vs-bot 50.1%
//     [47.0%, 53.2%], 15.4 turns): priority scored UNGATED on a ResidualW 0
//     checkpoint wins 0/1000 at 13.3 turns, reproducing the historical
//     0/1000 (bce-big.gpol, merge e9945e79). Every one of its ~13
//     disagreements per game with the bot was OUT of distribution — a
//     one-castable-object window or a play_land answer — where the scored
//     argmax (which cannot pick the untrained tap/land surface) passed
//     instead of casting or playing the land; the head never even reached an
//     in-distribution decision, because it starved its own development. With
//     the gate, the ResidualW 0 configuration is unreachable and the
//     out-of-distribution windows (~190 of every ~195 priority decisions per
//     game) belong to the bot.
//
//     What the gated path does NOT yet do is beat the bot: on the three
//     trained checkpoints measured (dev corpus ResidualInit 2 and 8, oracle
//     corpus ResidualInit 2) the head overrode the bot's answer at ZERO of
//     ~5,150 scored in-distribution priority decisions, so every
//     priority-scored arm played byte-identically to the same checkpoint's
//     attackers-only arm (dev r2 46.3% [43.2%, 49.4%]; dev r8 40.5%; oracle
//     r2 47.1%). The prior is inert-safe, not a source of edge; any gap to
//     the control is the attackers path's.
//   - KBlockers (opt-in): a subset kind trained with the per-option BCE loss
//     (-kind-loss blockers=bce), so it follows the attackers contract, not
//     the priority gate: no ResidualW requirement, the same
//     admissionThreshold vote and the same exact-tie refusal. The admitted
//     set keeps one (blocker, attacker) pair per blocker (the option Group,
//     CR 509.1a), is ordered bot-picked pairs first in the bot's own
//     declaration order (the engine reads that order for CR 510.1c), then
//     repaired by decision.FitRequired, the bot's block guard
//     (botpolicy.LegalBlockChoices: CR 509.1a MinMaxBlocker bounds and
//     CR 509.1b charges, over boardFromView) and Clamp. An answer that still
//     fails Validate or misses the Required quota is refused, and the bot's
//     declaration stands.
//   - KTarget (opt-in): a single-choice CE kind, so the priority rule
//     applies: scored only in the shape the teacher labels (singleTarget:
//     Min == Max == 1, no budget, >= 2 options, searchprobe.SingleTarget's
//     test; the bot answered one choice) and only when ResidualW > 0. The
//     answer is the argmax, ties on the lowest option index.
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
// bot baseline unless the learned head overrides. Checkpoints are schema v2
// and carry ResidualW (policytrain -residual-init); a checkpoint trained
// without it loads ResidualW == 0, the mark is then a no-op for attackers and
// KPriority is never scored. The default bot's answer is also the fallback
// whenever the scored surface declines to answer. No scored kind consumes
// the default bot's rng (both answers are pure functions of the offered
// options and the board facts), so the marking changes no rng stream.

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"

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
	// attackers / priority / blockers / target are the scored-kind
	// selection (NewPolicyNetBotKinds). Named switches rather than a set so
	// the dispatch never ranges a map.
	attackers bool
	priority  bool
	blockers  bool
	target    bool
	// record, when set (SetRecorder), receives every decision this seat
	// SCORED and answered from its scores (ticket pn13's on-policy corpus).
	// nil records nothing and costs nothing.
	record func(PolicyNetDecision)
	// signAdmission selects the subset kinds' SIGN admission vote
	// (SetAdmission(AdmissionSign)); false is the default AdmissionAuto.
	signAdmission bool
	// sampleT > 0 switches the scored kinds from the deterministic vote to
	// SAMPLING (SetSampling, ticket pn14's stochastic collection): softmax of
	// score/T for the single-choice kinds, a per-option Bernoulli of
	// σ(score/T) for the subset kinds. sampleRng is the seat's own stream,
	// seeded from the game seed; nothing else reads it.
	sampleT   float64
	sampleRng *rand.Rand
}

// SetSampling makes the seat SAMPLE its scored answers at temperature temp
// (> 0) from a PCG stream seeded by seed — the collection-only exploration
// mode of ticket pn14. temp <= 0 restores the default argmax/vote seat. The
// stream is the seat's own (the wrapped default bot's rng is untouched), so
// a sampling game is a pure function of the game seed and the flags.
//
//   - priority, target: one draw u, the option at u on the inverse CDF of
//     softmax(score/T) over the scored action space (option order);
//   - attackers, blockers: one draw per option, included when
//     u < σ(score/T); the included set is then repaired exactly as the vote's
//     admitted set is (one option per attacker/blocker, requirements, Max,
//     the block guard), and a repaired answer the engine would refuse falls
//     back to the bot as before.
//
// The recorder then also reports the behaviour log-probability of the
// answer actually played (policynet.TemperedLogProb), which is what PPO's
// importance ratio must divide by.
func (b *PolicyNetBot) SetSampling(temp float64, seed uint64) {
	if temp <= 0 {
		b.sampleT, b.sampleRng = 0, nil
		return
	}
	b.sampleT = temp
	b.sampleRng = rand.New(rand.NewPCG(seed^0x70e14c0115ec7ed5, seed+0x9e3779b97f4a7c15))
}

// Subset admission votes (SetAdmission). AdmissionAuto is the default
// admissionThreshold rule: the calibrated sign when a decision's scores
// straddle 0, else the decision's own mean, refusing an exact tie.
// AdmissionSign (opt-in, ticket pn13) always votes on the calibrated sign,
// score > 0 — the per-option inclusion probability σ(score) > ½ the BCE head
// is trained under and the on-policy PPO objective models. Measured on the
// pn11 gen-0 checkpoint's own games (pn13): 93% of the auto rule's attackers
// "overrides" came from ALL-POSITIVE multi-option decisions, where the mean
// fallback drops every below-mean attacker the head (and the residual prior)
// voted to include; no weight update can reach those, because the mean rule
// is shift-invariant and admits all options only on an exact tie.
const (
	AdmissionAuto = "auto"
	AdmissionSign = "sign"
)

// AdmissionNames is the admission vocabulary, in listing order.
var AdmissionNames = []string{AdmissionAuto, AdmissionSign}

// SetAdmission selects the subset kinds' admission vote (AdmissionAuto or
// AdmissionSign); any other value is an error and changes nothing.
func (b *PolicyNetBot) SetAdmission(mode string) error {
	switch mode {
	case AdmissionAuto:
		b.signAdmission = false
	case AdmissionSign:
		b.signAdmission = true
	default:
		return fmt.Errorf("unknown policynet admission %q (want %s)", mode, strings.Join(AdmissionNames, " or "))
	}
	return nil
}

// admission reports the seat's admission mode name.
func (b *PolicyNetBot) admission() string {
	if b.signAdmission {
		return AdmissionSign
	}
	return AdmissionAuto
}

// PolicyNetDecision is one decision the seat scored and answered from its
// scores — the on-policy corpus's unit (policynet.OnPolicyRecord). A decision
// the seat delegated to the default bot (an unscored kind, a closed
// distribution gate, an exactly-tied or refused scored answer) is never
// reported. Options carry the residual BotPick mark exactly as scored; the
// slices are the seat's own, handed over (the seat keeps no reference).
type PolicyNetDecision struct {
	Kind   decision.Kind
	Seq    uint64
	Player state.PlayerID
	Turn   int32
	State  policynet.State
	// Options parallel the decision's options; InSpace marks the scored
	// action space (every option for attackers, blockers and target; the
	// cast/ability/pass options priorityFromScores argmaxes over).
	Options []policynet.Option
	InSpace []bool
	// Subset is true for the subset kinds (attackers, blockers), whose
	// policy is the per-option inclusion vote; false for the single-choice
	// kinds (priority, target), whose policy is the argmax.
	Subset bool
	// Scores are the full scores the answer was read from (head + residual).
	Scores []float32
	// Chosen and BotChosen are option POSITIONS: the seat's answer and the
	// wrapped default bot's answer to the same decision.
	Chosen    []int
	BotChosen []int
	// Value is the checkpoint's V(s) for the deciding seat (HasValue false:
	// the checkpoint has no value head).
	Value    float32
	HasValue bool
	// Admission is the subset kinds' admission vote the answer was read
	// with (AdmissionAuto or AdmissionSign).
	Admission string
	// Sampled is true when the answer was SAMPLED (SetSampling) at
	// Temperature; LogPBehaviour is then log π_T(Chosen) over the InSpace
	// options (policynet.TemperedLogProb).
	Sampled       bool
	Temperature   float64
	LogPBehaviour float64
}

// SetRecorder installs fn to receive every decision this seat scores (nil
// removes it). Recording reads the scores the answer was built from and asks
// the value head once more; it never changes an answer or consumes rng.
func (b *PolicyNetBot) SetRecorder(fn func(PolicyNetDecision)) { b.record = fn }

// compile-time assertions: PolicyNetBot is a Seat and deliberately NOT a
// BoardSeat (its encoder needs the projected View).
var (
	_ Seat = (*PolicyNetBot)(nil)
)

// PolicyNetKindNames is the scored-kind vocabulary, in the order a caller
// lists it: the decision kinds NewPolicyNetBotKinds accepts, by the names
// botbench's -policynet-kinds flag spells them. KAttackers is the default;
// KPriority, KBlockers and KTarget are opt-in (see the package comment for
// their gates).
var PolicyNetKindNames = []string{"attackers", "priority", "blockers", "target"}

// policyNetKind maps a vocabulary name to its decision kind.
func policyNetKind(name string) (decision.Kind, bool) {
	switch name {
	case "attackers":
		return decision.KAttackers, true
	case "priority":
		return decision.KPriority, true
	case "blockers":
		return decision.KBlockers, true
	case "target":
		return decision.KTarget, true
	}
	return "", false
}

// ParsePolicyNetKinds parses a comma list of scored-kind names
// (PolicyNetKindNames) into decision kinds. Empty entries are an error, as
// are unknown or repeated names and an empty list: the flag's value is a
// deliberate selection, and a typo must not silently fall back to a default.
func ParsePolicyNetKinds(list string) ([]decision.Kind, error) {
	var out []decision.Kind
	for _, name := range strings.Split(list, ",") {
		name = strings.TrimSpace(name)
		k, ok := policyNetKind(name)
		if !ok {
			return nil, fmt.Errorf("unknown policynet kind %q (want a comma list of %s)", name, strings.Join(PolicyNetKindNames, ","))
		}
		if slices.Contains(out, k) {
			return nil, fmt.Errorf("policynet kind %q listed twice", name)
		}
		out = append(out, k)
	}
	return out, nil
}

// NewPolicyNetBot wraps the default bot (same PCG seed derivation as
// NewBot, so the delegation path consumes exactly the rng the default bot
// would) with the given scorer, scoring the DEFAULT kind set: KAttackers
// only. The scorer is owned by the caller; a Scorer is single-threaded, so a
// bench worker builds one per seat per game over a shared read-only Model.
func NewPolicyNetBot(seed uint64, sc *policynet.Scorer) *PolicyNetBot {
	return NewPolicyNetBotKinds(seed, sc, []decision.Kind{decision.KAttackers})
}

// NewPolicyNetBotKinds is NewPolicyNetBot with an explicit scored-kind
// selection: kinds may name KAttackers, KPriority, KBlockers and KTarget
// (any other kind panics — a programming error; ParsePolicyNetKinds is the
// validated front door). A kind left out delegates to the default bot
// exactly as every unscored kind does. KPriority or KTarget in the set is
// necessary but not sufficient for such a decision to be scored: its
// distribution gate (priorityScorable, targetScorable) must also hold.
func NewPolicyNetBotKinds(seed uint64, sc *policynet.Scorer, kinds []decision.Kind) *PolicyNetBot {
	b := &PolicyNetBot{def: NewBot(seed), scorer: sc}
	for _, k := range kinds {
		switch k {
		case decision.KAttackers:
			b.attackers = true
		case decision.KPriority:
			b.priority = true
		case decision.KBlockers:
			b.blockers = true
		case decision.KTarget:
			b.target = true
		default:
			panic(fmt.Sprintf("seat: NewPolicyNetBotKinds: kind %q is not a scored policynet kind", k))
		}
	}
	return b
}

// scoresKind reports whether this seat's scored-kind selection covers d's
// kind. For KPriority and KTarget the per-decision gates (priorityScorable,
// targetScorable) still apply.
func (b *PolicyNetBot) scoresKind(d *decision.Decision) bool {
	switch d.Kind {
	case decision.KAttackers:
		return b.attackers
	case decision.KPriority:
		return b.priority
	case decision.KBlockers:
		return b.blockers
	case decision.KTarget:
		return b.target
	}
	return false
}

// singleTarget is the one KTarget shape the target teacher labels: exactly
// one pick (Min == Max == 1), no MaxSum budget and at least two options. It
// is searchprobe.SingleTarget's test, restated here because searchprobe
// imports rules and the seat must not (seat/policynet_test.go pins the two
// equal over crafted shapes).
func singleTarget(d *decision.Decision) bool {
	return d != nil && d.Kind == decision.KTarget && d.Min == 1 && d.Max == 1 &&
		!d.HasBudget() && len(d.Options) >= 2
}

// targetScorable is the target distribution gate, ticket 01's single-choice
// CE rule: the residual prior is active (ResidualW > 0, so the zero-residual
// plain argmax is unreachable), the decision has the teacher's singleTarget
// shape, and the bot answered with exactly one choice (the teacher labels
// only such a decision: searchprobe.TargetCandidates).
func targetScorable(d *decision.Decision, botIn decision.Intent, residualW float32) bool {
	return residualW > 0 && singleTarget(d) && len(botIn.Choices) == 1
}

// priorityScorable is the priority distribution gate: a priority decision
// is scored only when it has the shape the head was TRAINED on, and
// otherwise the default bot's answer stands. All of:
//
//   - the residual prior is active (residualW > 0). With ResidualW 0 the
//     scored path is the plain argmax that measured 0/1000 in play, so that
//     configuration is unreachable;
//   - the decision offers >= 2 distinct castable objects
//     (botpolicy.CastableObjects — the same count searchseat.Eligible gates
//     the teacher's priority labels on);
//   - the bot's own answer is exactly one option of kind cast, ability or
//     pass — the teacher labels a priority decision only when the bot's
//     answer is a single candidate-kind action (searchseat's candidates arm,
//     searchprobe.Candidates), so a play_land or tap ("activate") answer, or
//     a multi-choice one, is a decision the head never saw.
func priorityScorable(d *decision.Decision, botIn decision.Intent, residualW float32) bool {
	if residualW <= 0 || botpolicy.CastableObjects(d) < 2 || len(botIn.Choices) != 1 {
		return false
	}
	for i := range d.Options {
		if d.Options[i].Index != botIn.Choices[0] {
			continue
		}
		switch d.Options[i].Kind {
		case "cast", "ability", "pass":
			return true
		}
		return false
	}
	return false
}

// PriorityInDistribution is priorityScorable exported: the priority
// distribution gate, for a caller outside this package that scores priority
// options with a policynet head (searchseat's candidate prior, pn10) and must
// score only the shape the head was trained on, exactly as this seat does.
func PriorityInDistribution(d *decision.Decision, botIn decision.Intent, residualW float32) bool {
	return priorityScorable(d, botIn, residualW)
}

// MarkBotPicks is markBotPicks exported: the inference half of the residual
// prior's BotPick contract, for a caller outside this package that scores a
// decision's options with a residual checkpoint (searchseat's candidate
// prior). opts must parallel d.Options.
func MarkBotPicks(d *decision.Decision, opts []policynet.Option, in decision.Intent) {
	markBotPicks(d, opts, in)
}

// encode runs the fixed encoder over the seat's view and the offered
// options. ok is false when the scored surface is empty — no scorer, or a
// decision with no options at all — which is the documented fall-back-to-
// the-default-bot shape ("a decision whose options encode to nothing").
func (b *PolicyNetBot) encode(v view.View, d *decision.Decision) (policynet.State, []policynet.Option, bool) {
	if b.scorer == nil || len(d.Options) == 0 {
		return policynet.State{}, nil, false
	}
	// The checkpoint's feature set (pn12): FeaturesV1 is the pinned encoder
	// byte for byte; a non-v1 checkpoint is scored under the set it trained on.
	fs := b.scorer.Features()
	st := policynet.EncodeStateWith(fs, v, d.Player, nil)
	opts := make([]policynet.Option, len(d.Options))
	for i := range d.Options {
		opts[i] = policynet.EncodeOptionWith(fs, v, d.Player, d.Kind, d.Options[i], i, len(d.Options))
	}
	return st, opts, true
}

// Decide answers d: the selected scored kinds above, the default bot for
// everything else. For a scored kind the wrapped default bot is asked FIRST
// (its answer marks the residual prior's BotPick, gates a priority decision
// and is the fallback when the scored surface cannot answer) — see the
// residual-prior paragraph above for the determinism contract.
func (b *PolicyNetBot) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	if !b.scoresKind(&d) {
		return b.def.Decide(ctx, v, d)
	}
	botIn, err := b.def.Decide(ctx, v, d)
	if err != nil {
		return decision.Intent{}, err
	}
	if d.Kind == decision.KPriority && (b.scorer == nil || !priorityScorable(&d, botIn, b.scorer.ResidualWeight())) {
		return botIn, nil
	}
	if d.Kind == decision.KTarget && (b.scorer == nil || !targetScorable(&d, botIn, b.scorer.ResidualWeight())) {
		return botIn, nil
	}
	st, opts, ok := b.encode(v, &d)
	if !ok {
		return botIn, nil
	}
	markBotPicks(&d, opts, botIn)
	scores := b.scorer.Score(st, opts)
	var in decision.Intent
	var scored bool
	if b.sampleT > 0 {
		in, scored = b.sampleAnswer(v, &d, scores, botIn)
		if scored && b.record != nil {
			b.report(v, &d, st, opts, scores, in, botIn)
		}
		if scored {
			return in, nil
		}
		return botIn, nil
	}
	switch d.Kind {
	case decision.KAttackers:
		in, scored = attackersFromScoresVote(&d, scores, b.signAdmission)
	case decision.KPriority:
		in, scored = priorityFromScores(&d, scores)
	case decision.KBlockers:
		in, scored = blockersFromScoresVote(&d, scores, boardFromView(v), botIn, b.signAdmission)
	case decision.KTarget:
		in, scored = targetFromScores(&d, scores)
	}
	if scored {
		if b.record != nil {
			b.report(v, &d, st, opts, scores, in, botIn)
		}
		return in, nil
	}
	return botIn, nil
}

// report builds and hands one scored decision to the recorder.
func (b *PolicyNetBot) report(v view.View, d *decision.Decision, st policynet.State, opts []policynet.Option, scores []float32, in, botIn decision.Intent) {
	pos := make(map[int]int, len(d.Options)) // option Index -> position; probed only
	for i := range d.Options {
		pos[d.Options[i].Index] = i
	}
	positions := func(choices []int) []int {
		out := make([]int, 0, len(choices))
		for _, c := range choices {
			if p, ok := pos[c]; ok {
				out = append(out, p)
			}
		}
		return out
	}
	rec := PolicyNetDecision{
		Kind: d.Kind, Seq: d.Seq, Player: d.Player, Turn: v.Turn,
		State: st, Options: opts, InSpace: make([]bool, len(opts)),
		Subset: d.Kind == decision.KAttackers || d.Kind == decision.KBlockers,
		Scores: scores, Chosen: positions(in.Choices), BotChosen: positions(botIn.Choices),
		Admission: b.admission(),
	}
	for i := range d.Options {
		switch d.Kind {
		case decision.KPriority:
			switch d.Options[i].Kind {
			case "cast", "ability", "pass":
				rec.InSpace[i] = true
			}
		default:
			rec.InSpace[i] = true
		}
	}
	if b.sampleT > 0 {
		chosen := make([]bool, len(opts))
		for _, p := range rec.Chosen {
			chosen[p] = true
		}
		rec.Sampled, rec.Temperature = true, b.sampleT
		rec.LogPBehaviour, _ = policynet.TemperedLogProb(scores, rec.InSpace, chosen, rec.Subset, b.sampleT)
	}
	if b.scorer.HasValue() {
		rec.Value, rec.HasValue = b.scorer.Value(st), true
	}
	b.record(rec)
}

// priorityInSpace marks the priority action space (cast / ability / pass).
func priorityInSpace(d *decision.Decision) []bool {
	in := make([]bool, len(d.Options))
	for i := range d.Options {
		switch d.Options[i].Kind {
		case "cast", "ability", "pass":
			in[i] = true
		}
	}
	return in
}

// sampleAnswer is Decide's scored step under SetSampling: draw the answer
// from the tempered policy, then repair it exactly as the vote's answer is
// repaired. ok false means the repaired answer was refused (the bot's
// declaration stands, as for the vote).
func (b *PolicyNetBot) sampleAnswer(v view.View, d *decision.Decision, scores []float32, botIn decision.Intent) (decision.Intent, bool) {
	t := b.sampleT
	switch d.Kind {
	case decision.KPriority, decision.KTarget:
		inSpace := make([]bool, len(d.Options))
		if d.Kind == decision.KPriority {
			inSpace = priorityInSpace(d)
		} else {
			for i := range inSpace {
				inSpace[i] = true
			}
		}
		p := policynet.SampleSoftmax(scores, inSpace, t, b.sampleRng.Float64())
		if p < 0 {
			return decision.Intent{}, false
		}
		in := botpolicy.Clamp(d, decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{d.Options[p].Index}})
		if d.Kind == decision.KTarget && d.Validate(in) != nil {
			return decision.Intent{}, false
		}
		return in, true
	case decision.KAttackers, decision.KBlockers:
		// The draw becomes a pseudo-score the sign vote reads: an included
		// option lands far above 0, an excluded one far below, each keeping
		// its own score's order so the per-attacker / per-blocker "best
		// admitted option" repair still prefers the higher-scored pair.
		pseudo := make([]float32, len(scores))
		for i, s := range scores {
			c := s
			if c > 100 {
				c = 100
			} else if c < -100 {
				c = -100
			}
			if policynet.SampleInclusion(s, t, b.sampleRng.Float64()) {
				pseudo[i] = 1000 + c
			} else {
				pseudo[i] = -1000 + c
			}
		}
		if d.Kind == decision.KAttackers {
			return attackersFromScoresVote(d, pseudo, true)
		}
		return blockersFromScoresVote(d, pseudo, boardFromView(v), botIn, true)
	}
	return decision.Intent{}, false
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
	return attackersFromScoresVote(d, scores, false)
}

// attackersFromScoresVote is attackersFromScores under the chosen admission
// vote (sign: the calibrated sign alone; see SetAdmission).
func attackersFromScoresVote(d *decision.Decision, scores []float32, sign bool) (decision.Intent, bool) {
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
	threshold, inclusive := admissionVote(scores, sign)

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

// priorityFromScores turns the per-option scores into the scored cast
// choice: argmax over the options the teacher's candidate space covers
// (cast / ability / pass — searchprobe.Candidates' pool), ties on the lowest
// option index. It is reached only through priorityScorable, so the bot's
// own answer is always one of those options and carries the residual
// prior's BotPick bonus: it wins unless the learned head overrides it. ok is
// false when the decision offers none of those options.
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

// blockersFromScores turns the per-option scores into a legal block
// declaration: the attackers contract applied to KBlockers (the kind is
// trained with the per-option BCE loss).
//
//   - The per-option vote against admissionThreshold, refusing an exactly
//     tied multi-option decision (no sign, no ranking: the zero-head model
//     whose bot declared no block ties every score there and must reproduce
//     the bot).
//   - One pair per blocker (the option Group the engine publishes per
//     blocker, CR 509.1a): the blocker's best admitted option, ties on the
//     lowest index.
//   - Ordered: the pairs the bot's own declaration (botIn) contains first,
//     in the bot's order, then the rest in offer order. The engine reads the
//     declaration order for CR 510.1c damage assignment and the bot's order
//     is its damage-order heuristic, so a head that agrees with the bot
//     reproduces the bot's intent byte for byte.
//   - Repaired by decision.FitRequired (the KBlockers-aware requirement
//     solver), then botpolicy.LegalBlockChoices over the seat's board
//     (CR 509.1a MinMaxBlocker bounds, CR 509.1b charges) and Clamp.
//
// ok is false when the decision is empty or tied, or when the repaired
// answer still fails Validate or the Required quota (the guard can drop a
// required pair); the caller then uses the bot's declaration.
func blockersFromScores(d *decision.Decision, scores []float32, board botpolicy.Board, botIn decision.Intent) (decision.Intent, bool) {
	return blockersFromScoresVote(d, scores, board, botIn, false)
}

// blockersFromScoresVote is blockersFromScores under the chosen admission
// vote (see SetAdmission).
func blockersFromScoresVote(d *decision.Decision, scores []float32, board botpolicy.Board, botIn decision.Intent, sign bool) (decision.Intent, bool) {
	if len(d.Options) == 0 || len(scores) != len(d.Options) {
		return decision.Intent{}, false
	}
	threshold, inclusive := admissionVote(scores, sign)
	if len(scores) > 1 && inclusive && allEqual(scores) {
		return decision.Intent{}, false
	}
	admitted := func(s float32) bool {
		if inclusive {
			return s >= threshold
		}
		return s > threshold
	}
	// Each blocker's best admitted option, keyed by Group (an option with no
	// Group is its own group). The map is only probed, never ranged; groups
	// are visited in first-seen offer order.
	best := make(map[string]int, len(d.Options))
	var order []string
	for i := range d.Options {
		if !admitted(scores[i]) {
			continue
		}
		g := d.Options[i].Group
		if g == "" {
			g = fmt.Sprintf("#%d", i)
		}
		cur, ok := best[g]
		if !ok {
			order = append(order, g)
			best[g] = i
			continue
		}
		if scores[i] > scores[cur] {
			best[g] = i
		}
	}
	// Choices carry option Index values (rules builds Index == position, but
	// the answer is spelt in Index terms like every other arm's).
	pending := make(map[int]bool, len(order)) // Index membership only
	for _, g := range order {
		pending[d.Options[best[g]].Index] = true
	}
	chosen := make([]int, 0, len(order))
	for _, c := range botIn.Choices {
		if pending[c] {
			chosen = append(chosen, c)
			pending[c] = false
		}
	}
	for _, g := range order {
		if idx := d.Options[best[g]].Index; pending[idx] {
			chosen = append(chosen, idx)
			pending[idx] = false
		}
	}
	chosen = d.FitRequired(chosen)
	chosen = botpolicy.LegalBlockChoices(board, d, chosen)
	in := botpolicy.Clamp(d, decision.Intent{Seq: d.Seq, Player: d.Player, Choices: chosen})
	if d.Validate(in) != nil || d.RequiredChosen(in.Choices) < d.RequiredQuota() {
		return decision.Intent{}, false
	}
	return in, true
}

// targetFromScores is the scored single-target pick: argmax over the
// options, ties on the lowest option index, Clamped. It answers only the
// singleTarget shape (Decide's targetScorable gate is the front door; the
// shape test is repeated so a direct caller cannot score an
// out-of-distribution decision) and refuses an answer Validate rejects.
func targetFromScores(d *decision.Decision, scores []float32) (decision.Intent, bool) {
	if !singleTarget(d) || len(scores) != len(d.Options) {
		return decision.Intent{}, false
	}
	best := 0
	for i := 1; i < len(scores); i++ {
		if scores[i] > scores[best] {
			best = i
		}
	}
	in := botpolicy.Clamp(d, decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{d.Options[best].Index}})
	if d.Validate(in) != nil {
		return decision.Intent{}, false
	}
	return in, true
}

// admissionVote is the admission reference under the seat's vote: the
// calibrated boundary 0 (strict) for the sign vote, admissionThreshold for
// the default auto vote. The sign vote is never inclusive, so it never
// refuses an exact tie: a zero head with a positive residual admits exactly
// the bot's own picks.
func admissionVote(scores []float32, sign bool) (float32, bool) {
	if sign {
		return 0, false
	}
	return admissionThreshold(scores)
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
