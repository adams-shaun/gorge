// Package searchseat is the PIMC search teacher's decision function, shared by
// the label-corpus generator (cmd/searchteacher) and any seat that plays the
// teacher's answer.
//
// It exists so the two cannot drift. Before it, the whole of candidate
// building, world sampling, teacher scoring and root matching lived inside
// cmd/searchteacher's teach(), entangled with label emission -- so a seat that
// wanted to PLAY the teacher's answer had to reimplement it, and a
// reimplementation that disagreed anywhere would invalidate the corpus the
// student learns from. Here the decision is one function; the generator adds
// label emission around it and reads the diagnostics off Trace.
//
// # What a caller must supply, and why a plain seat cannot
//
// Choose needs a searchprobe.History, and History is not something a
// seat.Seat can build. Measured against the sampler's actual requirements:
//
//   - searchprobe.Collector.Capture takes a *rules.Engine. Its own doc says
//     "Collector alone can read a source engine": it projects the actor's view
//     off e.G, reads e.L.Events for the round fold and e.Pending() for the
//     decision, and needs the raw []events.Event burst since the previous
//     capture. A seat is handed a PROJECTED view.View and a decision.Decision
//     -- never the engine, never the event log, never the burst.
//   - Capture runs at EVERY decision, every player's, not just the actor's
//     (cmd/searchteacher's loop). A seat is invoked only at its own decisions,
//     so it could not assemble the same Frames list even with engine access.
//
// So the history must come from whoever drives the engine, which is why
// Choose takes the history, the collector and the engine as arguments instead
// of reading them off a view. Wiring a driver to maintain them and feed a
// seat is the same idiom seat.BoardSeat already uses: a seat variant the
// driver detects and feeds richer inputs than the plain Seat interface
// carries. That wiring is not in this package.
//
// The alternative -- a seat reconstructing history from successive views --
// was rejected on measurement, not taste: without the event bursts there are
// no epoch constraints, so Sample would draw from the wrong world
// distribution and the teacher's measured edge would not survive.
package searchseat

import (
	"errors"
	"fmt"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/policynet"
	"github.com/adams-shaun/gorge/internal/searchprobe"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// Options are the search knobs, defaulted by Defaults() to cmd/searchteacher's
// own flag defaults so a seat and the generator search identically unless a
// caller deliberately differs.
type Options struct {
	// Kinds gates which decision kinds the teacher answers. The implemented
	// set is "attackers", "blockers", "mana" (alternative bare mana
	// sources at priority), "cast" (a KPriority decision
	// offering two or more distinct castable objects) and "target" (a
	// single-choice, unbudgeted KTarget, searchprobe.SingleTarget); every
	// other decision delegates. Defaults leaves "blockers" and "target" off.
	Kinds map[string]bool
	// Worlds is K, the sampled worlds per decision; Attempts the sampler's
	// proposal attempts; MinESS the effective-sample-size gate (0 keeps the
	// calibration contract ESS >= Worlds).
	Worlds, Attempts int
	MinESS           float64
	// Limit caps candidates per decision, the bot's own answer first.
	Limit int
	// Margin is the mean-value margin a candidate must beat the bot's answer
	// by before the teacher overrides it.
	Margin float64
	// HorizonTurns is the rollout horizon in engine turns after the root; 0
	// rolls to game end. It is int32 to match TeacherOptions, whose turn
	// counts are engine turns (state.Game.Turn is int32). MaxSubmits caps
	// submits per rollout and per sample attempt.
	HorizonTurns int32
	MaxSubmits   int
	// Value, when non-nil and carrying a value head (Model.HasValue), scores
	// every non-terminal rollout leaf with the value head
	// (searchprobe.TeacherOptions.Leaf) instead of the frozen material
	// heuristic. Nil is off. A model WITHOUT a value head is a configuration
	// error the caller must reject up front (cmd/searchteacher does); Choose
	// ignores it rather than score every leaf 0.5. It only matters with
	// HorizonTurns > 0 or a MaxSubmits cap: a game-end rollout has no
	// non-terminal leaf. The Model is shared read-only across rollout
	// goroutines, which Model.Value allows.
	//
	// Value must read the redacted view: a model of a diagnostic (oracle)
	// feature set here is a configuration error (Validate), because the
	// redacted leaf has no opponent hand to give it.
	Value *policynet.Model
	// OracleValue (ticket pn17-a1), when non-nil, is an ORACLE value model: a
	// FeaturesMZOppHand checkpoint (policynet.LoadOracleCheckpointFile) whose
	// value head scores every non-terminal rollout leaf from the OMNISCIENT
	// projection of that leaf (searchprobe.TeacherOptions.LeafOmniscient),
	// the opponent's hand read off it as the Diag the model was trained with
	// (policynet.SplitOmniscientView). This is legitimate at deploy time only
	// because every leaf lives in a SAMPLED world: the hand it reads is the
	// sampled one, never the real opponent's. Hence it refuses Clairvoyant,
	// whose one world is a clone of the real engine.
	//
	// Validate refuses, besides Clairvoyant: a model without a value head, a
	// non-diagnostic feature set (a redacted model has no use for the
	// omniscient view and would silently ignore it), FeaturesMZOracle (its
	// next-draw tokens are library ORDER, which no view carries, so every
	// leaf would be off its training distribution), and Value set too. Nil
	// is off, and today's search bit for bit.
	OracleValue *policynet.Model
	// SampleSeed is the fixed sampler seed base. The teacher's own seed is
	// derived from it per decision exactly as cmd/searchteacher derives it, so
	// a seat and the generator score a given decision identically.
	SampleSeed uint64
	// AfterSample and AfterSearch, when non-nil, are called immediately after
	// the two expensive phases. They exist so a caller can TIME the phases
	// without this package reading a clock: internal/archtest's
	// TestTimeIsImportedOnlyByTheHost allows the time import in host,
	// host/httpapi and cmd/gorged only, and a search helper is none of those.
	// The cost per searched decision is the number that decides whether the
	// teacher's edge is affordable, so the split has to be measurable
	// somewhere -- it is measured by whoever already owns a clock.
	//
	// They are diagnostics only. Choose's answer is a pure function of its
	// inputs whether or not they are set, which is what lets a seat play this
	// and stay replayable.
	AfterSample, AfterSearch func()
	// Parallelism is how many goroutines one decision's sampling attempts and
	// rollouts may use (<=1: sequential). It buys latency for a caller that
	// answers one decision at a time -- a live seat -- and nothing for one that
	// already fills its cores with whole games. It never changes the answer:
	// searchprobe folds the parallel work in its sequential order.
	Parallelism int
	// NoLandExclusion is searchprobe.SampleOptions.NoLandExclusion: a
	// measurement switch that restores the sampler's pre-exclusion proposal.
	// A playing seat leaves it false.
	NoLandExclusion bool
	// ComparePotentialActions is searchprobe.SampleOptions'
	// ComparePotentialActions: a measurement switch that restores the
	// replay's potential-action walk and counts the rejections it alone
	// decides (Trace.BoardPotentialActionsOnly). A playing seat leaves it false.
	ComparePotentialActions bool
	// Redeal turns on searchprobe's redeal fallback (pn21): when the sampler
	// starves, the seat's unknown hidden cards are redealt from clones of the
	// engine it is deciding in, pinning every card its observation history
	// knows (searchprobe.RedealBase documents what is and is not read). Off
	// by default; off changes nothing.
	Redeal bool
	// Clairvoyant searches one clone of the ACTUAL engine instead of sampled
	// worlds. It cheats by construction and exists only as a measurement
	// ceiling (cmd/searchteacher's -oracle); a playing seat must leave it
	// false.
	Clairvoyant bool
	// Prior, when non-nil, is a trained policynet checkpoint that chooses
	// WHICH candidates get the rollout budget (pn10): the attackers and cast
	// arms enumerate up to PriorWiden candidates, the non-bot candidates are
	// ranked by the policy head scored on the actor's own view, and the top
	// PriorTopK are kept behind the bot's answer (see applyPrior). nil is
	// today's fixed, hand-ordered list, byte for byte. The Model is only read
	// (Model.Score allocates), so one Model may be shared across goroutines.
	Prior *policynet.Model
	// PriorTopK is how many non-bot candidates a Prior keeps; 0 means
	// Limit-1, the unguided list's own non-bot budget.
	PriorTopK int
	// PriorWiden is the enumeration cap (bot answer included) used when a
	// Prior is set; 0 means max(Limit, 16).
	PriorWiden int
}

// Defaults are cmd/searchteacher's flag defaults, which are also the knobs the
// +5.80pp +/- 1.25 paired-dev measurement was taken with. A seat starts here
// so "the seat plays the teacher" means the teacher that was measured.
func Defaults() Options {
	return Options{
		Kinds:      map[string]bool{"attackers": true, "cast": true},
		Worlds:     8,
		Attempts:   64,
		MinESS:     0,
		Limit:      6,
		Margin:     0,
		MaxSubmits: 5000,
		SampleSeed: 54321,
	}
}

// Trace is everything a caller might want to record about one Choose call. It
// is diagnostics only: nothing here feeds back into the decision, so a caller
// that ignores it plays identically to one that records all of it.
//
// Covered reports that the teacher actually scored the decision. Fallback
// names why it did not, when it did not -- the strings are the same ones
// cmd/searchteacher has always written to its DecisionRecord, so its corpus
// is unchanged by the extraction.
type Trace struct {
	Kind       string
	Covered    bool
	Fallback   string
	Candidates [][]searchprobe.Action
	Worlds     int

	// Sample diagnostics, zero when the clairvoyant path skipped sampling.
	Attempts, Accepted, PrefixRejected, Duplicates int
	ESS                                            float64
	// Rejections is the complete deterministic sampler rejection census for
	// this decision. It is copied from SampleResult rather than retained by
	// reference so diagnostics cannot alias a sampler result.
	Rejections                []searchprobe.RejectionBucket
	TopRejection              string
	HandToStack               searchprobe.HandToStackCauses
	CompetitionExclusions     int
	CompetitionResidual       int
	CompetitionUnguided       int
	BoardPotentialActionsOnly int
	IncompatibleProposals     int

	// Teacher result. Index 0 is the bot's own answer.
	Index    int
	Values   []float64
	Rollouts int
	Submits  int
	Terminal int
	Capped   int
	HasValue bool

	// Prior diagnostics (Options.Prior set, attackers or cast arm only).
	// PriorEnumerated is how many candidates the widened enumeration produced,
	// PriorKept how many the prior kept (bot answer included), PriorRanked
	// whether the policy head actually ranked them (false: a cast decision
	// outside the head's trained distribution, or a candidate that could not
	// be mapped back to option indices -- today's list was kept), and
	// PriorChanged whether the kept set differs from the same-budget unguided
	// list (the first PriorKept candidates in enumeration order).
	PriorEnumerated, PriorKept int
	PriorRanked, PriorChanged  bool
}

// Eligible reports whether Choose would attempt this decision at all, without
// paying for sampling. A caller that must decide cheaply whether a decision is
// worth observing (or whether to charge a search budget) asks this first.
// It is deliberately the same test Choose applies.
func Eligible(d *decision.Decision, opts Options) bool {
	switch {
	case d.Kind == decision.KAttackers && opts.Kinds["attackers"]:
		return true
	case d.Kind == decision.KBlockers && opts.Kinds["blockers"]:
		return true
	case opts.Kinds["target"] && searchprobe.SingleTarget(d):
		return true
	case d.Kind == decision.KPriority && opts.Kinds["cast"] && CastOptions(d) >= 2:
		return true
	case d.Kind == decision.KPriority && opts.Kinds["mana"] && bareManaSources(d) >= 2:
		return true
	}
	return false
}

// bareManaSources counts source taps the bot can compare without putting a
// non-mana cost (life, sacrifice, pool mana) into the root candidate set.
func bareManaSources(d *decision.Decision) int {
	n := 0
	for _, o := range d.Options {
		if o.Kind == "activate" && o.Cost == "" {
			n++
		}
	}
	return n
}

// CastOptions counts the DISTINCT castable objects a priority decision offers.
// Distinct objects, not options: one card can be offered several ways (an
// alternative cost, a kicked mode), and those are the same choice of card for
// candidate purposes. It IS botpolicy.CastableObjects, the definition the
// learned seat's priority gate (seat.PolicyNetBot) also reads, so the
// teacher's label distribution and the seat's scored distribution share one
// eligibility test.
func CastOptions(d *decision.Decision) int {
	return botpolicy.CastableObjects(d)
}

// Choose runs the teacher at one decision and returns the intent to play.
//
// It returns (bot, false, trace) whenever the teacher does not cover the
// decision or anything fails -- an ineligible kind, fewer than two
// candidates, a sampler failure, no accepted worlds, a teacher error, or a
// root match that cannot be translated back into an intent. Every failure
// path is a DELEGATION to the wrapped bot's own answer, never a dropped or
// invented decision: a search teacher that cannot search must still play
// legally.
//
// h is passed by value because Sample takes it by value; the caller keeps
// ownership of the frame history and keeps appending to it.
func Choose(
	setup searchprobe.PublicGame,
	h searchprobe.History,
	collector *searchprobe.Collector,
	e *rules.Engine,
	d *decision.Decision,
	bot decision.Intent,
	f searchprobe.Frame,
	opts Options,
) (decision.Intent, bool, Trace) {
	var tr Trace

	cands, kind, ok := candidates(collector, e, d, bot, f, opts)
	if ok {
		cands = applyPrior(collector, e, d, bot, kind, cands, opts, &tr)
	}
	tr.Kind, tr.Candidates = kind, cands
	if !ok || len(cands) < 2 {
		return bot, false, tr
	}
	if err := opts.Validate(); err != nil {
		tr.Fallback = "options: " + err.Error()
		return bot, false, tr
	}

	worlds, sample, err := sampleWorlds(setup, h, collector, e, opts)
	if opts.AfterSample != nil {
		opts.AfterSample()
	}
	recordSample(&tr, sample)
	if err != nil {
		tr.Fallback = sampleFallback(err)
		return bot, false, tr
	}
	if len(worlds) == 0 {
		tr.Fallback = "insufficient worlds/ESS"
		return bot, false, tr
	}
	tr.Worlds = len(worlds)

	leaf := valueLeaf(opts.Value)
	if opts.OracleValue != nil {
		leaf = oracleLeaf(opts.OracleValue)
	}
	res, err := searchprobe.TeacherChoice(worlds, cands, searchprobe.TeacherOptions{
		Seed:           teacherSeed(opts.SampleSeed, e),
		HorizonTurns:   opts.HorizonTurns,
		MaxSubmits:     opts.MaxSubmits,
		Margin:         opts.Margin,
		Clairvoyant:    opts.Clairvoyant,
		Parallelism:    opts.Parallelism,
		Leaf:           leaf,
		LeafOmniscient: opts.OracleValue != nil,
	})
	if opts.AfterSearch != nil {
		opts.AfterSearch()
	}
	if err != nil {
		tr.Fallback = "teacher error: " + err.Error()
		return bot, false, tr
	}
	tr.Covered = true
	tr.HasValue = true
	tr.Index, tr.Values, tr.Rollouts = res.Index, res.Values, res.Rollouts
	tr.Submits, tr.Terminal, tr.Capped = res.Submits, res.Terminal, res.Capped

	// Index 0 IS the bot's answer, so the teacher agreeing is not an override
	// and must return the bot's own intent rather than a re-matched copy of
	// it: the two are equal in effect, and returning the original keeps the
	// no-override path byte-identical to a game the teacher never touched.
	if res.Index == 0 {
		return bot, false, tr
	}
	in, err := collector.Match(d, cands[res.Index])
	if err != nil {
		tr.Fallback = "root match: " + err.Error()
		return bot, false, tr
	}
	return in, true, tr
}

// valueLeaf is the rollout leaf evaluator for a value model: the value head
// on the leaf encoded exactly as a training state (policynet.EncodeState of
// the deciding seat's redacted view). Nil -- the heuristic leaf -- for no
// model or a model without a value head.
func valueLeaf(m *policynet.Model) func(view.View, state.PlayerID) float64 {
	if m == nil || !m.HasValue() {
		return nil
	}
	return func(v view.View, actor state.PlayerID) float64 {
		return float64(m.Value(policynet.EncodeStateWith(m.Features, v, actor, nil)))
	}
}

// oracleLeaf is the leaf evaluator for an oracle value model. Its view is the
// OMNISCIENT projection of the leaf (TeacherOptions.LeafOmniscient), which it
// splits back into a training record's shape -- the actor's view plus the
// opponents' hands as the Diag -- before encoding, so the model sees exactly
// the features it was trained on. Nil for no model or no value head.
func oracleLeaf(m *policynet.Model) func(view.View, state.PlayerID) float64 {
	if m == nil || !m.HasValue() {
		return nil
	}
	return func(v view.View, actor state.PlayerID) float64 {
		own, diag := policynet.SplitOmniscientView(v, actor)
		return float64(m.Value(policynet.EncodeStateWith(m.Features, own, actor, diag)))
	}
}

// Validate reports a leaf configuration Choose must not search with; every
// Choose call checks it and delegates to the bot on an error, and a caller
// that builds Options from flags should call it up front. The zero Options
// are valid.
func (o Options) Validate() error {
	if o.Value != nil && o.Value.Features.Diagnostic() {
		return fmt.Errorf("value model of feature set %s reads hidden information: it is an oracle model, usable only as OracleValue (an omniscient leaf)", o.Value.Features)
	}
	m := o.OracleValue
	if m == nil {
		return nil
	}
	switch {
	case o.Value != nil:
		return errors.New("Value and OracleValue are exclusive: one leaf evaluator per search")
	case o.Clairvoyant:
		return errors.New("OracleValue cannot be combined with Clairvoyant: the ceiling's one world is a clone of the REAL engine, so an omniscient leaf there would read the real opponent's hand")
	case !m.HasValue():
		return errors.New("OracleValue model has no value head")
	case m.Features == policynet.FeaturesMZOracle:
		return fmt.Errorf("OracleValue feature set %s reads library order, which no view carries; only %s is supported", m.Features, policynet.FeaturesMZOppHand)
	case !m.Features.Diagnostic():
		return fmt.Errorf("OracleValue model of feature set %s reads no hidden information: it cannot use the omniscient leaf (use Value)", m.Features)
	}
	return nil
}

// teacherSeed derives the per-decision teacher seed. It is deliberately the
// expression cmd/searchteacher has always used, turn included, so extracting
// this function changed no scored decision.
func teacherSeed(base uint64, e *rules.Engine) uint64 {
	return base ^ 0x5eed ^ uint64(e.G.Turn)
}

// candidates builds the candidate list, the bot's own answer first. The
// attackers arm enumerates attack subsets; the blockers arm enumerates
// single-pair edits of the bot's declaration, kept only when the bot's own
// block guard (read off the deciding seat's board, exactly what the bot
// reads) accepts them unchanged; the target arm compares the bot's single
// pick against every other option (searchprobe.TargetCandidates); the cast arm asks searchprobe for
// alternatives to the bot's single chosen action. A collector that cannot
// translate the bot's own intent into actions is a hard stop: without the
// baseline at index 0 the teacher has nothing to beat.
func candidates(collector *searchprobe.Collector, e *rules.Engine, d *decision.Decision, bot decision.Intent, f searchprobe.Frame, opts Options) ([][]searchprobe.Action, string, bool) {
	switch {
	case d.Kind == decision.KAttackers && opts.Kinds["attackers"]:
		var out [][]searchprobe.Action
		for _, in := range searchprobe.AttackCandidates(d, bot, opts.enumLimit()) {
			a, err := collector.Actions(d, in)
			if err != nil {
				return nil, "attackers", false
			}
			out = append(out, a)
		}
		return out, "attackers", true
	case d.Kind == decision.KBlockers && opts.Kinds["blockers"]:
		b := botpolicy.BoardFromGame(e.G, e, d.Player)
		legal := func(choices []int) []int { return botpolicy.LegalBlockChoices(b, d, choices) }
		var out [][]searchprobe.Action
		for _, in := range searchprobe.BlockCandidates(d, bot, opts.Limit, legal) {
			a, err := collector.Actions(d, in)
			if err != nil {
				return nil, "blockers", false
			}
			out = append(out, a)
		}
		return out, "blockers", true
	case opts.Kinds["target"] && searchprobe.SingleTarget(d):
		var out [][]searchprobe.Action
		for _, in := range searchprobe.TargetCandidates(d, bot, opts.Limit) {
			a, err := collector.Actions(d, in)
			if err != nil {
				return nil, "target", false
			}
			out = append(out, a)
		}
		return out, "target", true
	case d.Kind == decision.KPriority && opts.Kinds["cast"] && CastOptions(d) >= 2:
		a, err := collector.Actions(d, bot)
		if err != nil || len(a) != 1 {
			return nil, "cast", false
		}
		var out [][]searchprobe.Action
		for _, c := range searchprobe.Candidates(f.Decision, a[0], opts.enumLimit()) {
			out = append(out, []searchprobe.Action{c})
		}
		return out, "cast", true
	case d.Kind == decision.KPriority && opts.Kinds["mana"] && bareManaSources(d) >= 2:
		if len(bot.Choices) != 1 || bot.Choices[0] < 0 || bot.Choices[0] >= len(d.Options) {
			return nil, "mana", false
		}
		chosen := d.Options[bot.Choices[0]]
		if chosen.Kind != "activate" || chosen.Cost != "" {
			return nil, "mana", false
		}
		out := make([][]searchprobe.Action, 0, opts.Limit)
		for _, idx := range append([]int{bot.Choices[0]}, bareManaAlternatives(d, bot.Choices[0])...) {
			in := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}
			a, err := collector.Actions(d, in)
			if err != nil || len(a) != 1 {
				return nil, "mana", false
			}
			out = append(out, a)
			if len(out) >= opts.Limit {
				break
			}
		}
		return out, "mana", len(out) >= 2
	}
	return nil, "", false
}

func bareManaAlternatives(d *decision.Decision, chosen int) []int {
	var out []int
	for i, o := range d.Options {
		if i != chosen && o.Kind == "activate" && o.Cost == "" {
			out = append(out, i)
		}
	}
	return out
}

// sampleWorlds returns the worlds to search. The clairvoyant ceiling skips
// sampling entirely and searches one clone of the real engine; the honest path
// samples hidden worlds consistent with the observed history.
func sampleWorlds(setup searchprobe.PublicGame, h searchprobe.History, collector *searchprobe.Collector, e *rules.Engine, opts Options) ([]searchprobe.World, searchprobe.SampleResult, error) {
	if opts.Clairvoyant {
		return []searchprobe.World{{Engine: e.Clone(), Observer: collector}}, searchprobe.SampleResult{}, nil
	}
	sr, err := searchprobe.Sample(setup, h, searchprobe.SampleOptions{
		Seed:        opts.SampleSeed,
		Attempts:    opts.Attempts,
		Worlds:      opts.Worlds,
		MaxSubmits:  opts.MaxSubmits,
		MinESS:      opts.MinESS,
		Parallelism: opts.Parallelism,

		NoLandExclusion:         opts.NoLandExclusion,
		ComparePotentialActions: opts.ComparePotentialActions,
		Redeal:                  redealBase(e, collector, opts),
	})
	// The result is returned even on error: its rejection buckets are the
	// diagnostics that explain the failure, and dropping them would make a
	// sampler fallback unexplainable.
	return sr.Worlds, sr, err
}

func redealBase(e *rules.Engine, collector *searchprobe.Collector, opts Options) *searchprobe.RedealBase {
	if !opts.Redeal {
		return nil
	}
	return &searchprobe.RedealBase{Engine: e, Observer: collector}
}

// sampleFallback classifies a sampler error into the fallback string. A
// structured searchprobe.Failure reports its own Kind, which is what makes a
// fallback census (the "insufficient worlds/ESS" tally the generator prints)
// aggregatable; anything else keeps its message verbatim.
func sampleFallback(err error) string {
	var fl *searchprobe.Failure
	if errors.As(err, &fl) {
		return "sample " + fl.Kind
	}
	return "sample error: " + err.Error()
}

func recordSample(tr *Trace, sr searchprobe.SampleResult) {
	tr.Attempts, tr.Accepted, tr.PrefixRejected = sr.Attempts, sr.Accepted, sr.PrefixRejected
	tr.ESS, tr.Duplicates = sr.ESS, sr.Duplicates
	tr.Rejections = append([]searchprobe.RejectionBucket(nil), sr.Rejections...)
	tr.HandToStack = sr.HandToStackCauses
	tr.CompetitionExclusions, tr.CompetitionResidual = sr.CompetitionExclusions, sr.CompetitionResidual
	tr.CompetitionUnguided, tr.IncompatibleProposals = sr.CompetitionUnguided, sr.IncompatibleProposals
	tr.BoardPotentialActionsOnly = sr.BoardPotentialActionsOnly
	top := 0
	for _, b := range sr.Rejections {
		if b.Count > top {
			top = b.Count
			tr.TopRejection = b.Component + "/" + b.Shape
		}
	}
}
