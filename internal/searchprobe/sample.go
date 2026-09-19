package searchprobe

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"sort"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
)

// PublicGame has intentionally no original seed, intents, log, or engine.
// Definitions are immutable corpus data for the declared public deck lists.
type PublicGame struct {
	Names        []string
	Decks        [][]*cards.Card
	Tokens       map[string]*cards.Card
	StartingLife int32
}
type History struct {
	Actor   state.PlayerID
	Frames  []Frame
	Answers map[int][]Action
}
type SampleOptions struct {
	Seed                         uint64
	Attempts, Worlds, MaxSubmits int
	// MinESS is the effective-sample-size gate for resampling Worlds. Zero
	// keeps the calibration contract (ESS >= Worlds). A positive value lets a
	// caller (the search-teacher spike) resample Worlds with replacement from a
	// thinner accepted pool; duplicates are then counted in Duplicates.
	MinESS float64
}
type World struct {
	Config   rules.Config
	Engine   *rules.Engine
	Observer *Collector
}
type SampleResult struct {
	Worlds                                                                                 []World `json:"-"`
	Attempts, Accepted, PrefixRejected, BudgetExhausted, Submits, Duplicates               int
	ESS                                                                                    float64
	FirstRejection                                                                         string
	Rejections                                                                             []RejectionBucket
	GuidedGenesis, GuidedLater, ArrangeWindows, UnguidedConstraints, IncompatibleProposals int
	WeightDiagnostics                                                                      WeightDiagnostics
	HandToStackCauses                                                                      HandToStackCauses
	StackRejectionContexts                                                                 []StackRejectionContext
}
type RejectionBucket struct {
	Frame            int
	Component, Shape string
	Count            int
}

type HandToStackCauses struct {
	PolicyCompetition, ObserverReference, ObservedCastMissing, HypotheticalExtraCast, Other int
}

type StackRejectionContext struct {
	Cause, Step, ExpectedAction, Constraint string
	Count                                   int
}

func Sample(setup PublicGame, h History, opts SampleOptions) (out SampleResult, err error) {
	defer func() {
		var f *Failure
		if err != nil && !errors.As(err, &f) {
			err = fail("invariant", "%v", err)
		}
	}()
	var result SampleResult
	defer func() {
		out = result
	}()
	if len(setup.Names) != len(setup.Decks) || len(setup.Names) < 2 || int(h.Actor) >= len(setup.Names) || len(h.Frames) == 0 || opts.Attempts < 1 || opts.Worlds < 1 || opts.MaxSubmits < 1 {
		return result, fmt.Errorf("invalid sampling configuration/history")
	}
	for _, deck := range setup.Decks {
		if len(deck) < 7 {
			return result, fail("unsupported", "undersized genesis deck")
		}
		for _, card := range deck {
			if card == nil || len(card.Faces) == 0 || card.Faces[0] == nil {
				return result, fmt.Errorf("invalid public card definition")
			}
		}
	}
	encoded, err := json.Marshal(h)
	if err != nil {
		return result, err
	}
	digest := sha256.Sum256(encoded)
	epochs, err := compileEpochs(h)
	if err != nil {
		return result, err
	}
	if err := validateGenesis(setup, epochs); err != nil {
		return result, err
	}
	stackConstraints := stackConstraintContexts(h, epochs)
	tape, tossWeight, err := publicToss(setup, h)
	if err != nil {
		return result, err
	}
	var proposals []World
	var logs []float64
	plans := newConstraintPlanCache()
	prefixEvents := observedPrefixEvents(h)
	for attempt := 0; attempt < opts.Attempts; attempt++ {
		result.Attempts++
		seed := taggedSeed(opts.Seed, digest, attempt, seedEngine)
		cfg := rules.Config{Seed: seed[0], Names: setup.Names, Decks: setup.Decks, Tokens: setup.Tokens, StartingLife: setup.StartingLife}
		observer := NewCollector(h.Actor)
		proposal := &proposalState{epochs: epochs, logWeight: tossWeight, base: opts.Seed, history: digest, attempt: attempt, observer: observer, result: &result, plans: plans}
		e, err := rules.NewHypotheticalPlanned(cfg, tape, proposal.plan)
		if errors.Is(err, errIncompatibleProposal) {
			continue
		}
		if err != nil {
			return result, err
		}
		e.L.Reserve(prefixEvents)
		if err := e.AdvanceHypothetical(); err != nil {
			if errors.Is(err, errIncompatibleProposal) {
				continue
			}
			return result, err
		}
		bots := make([]*rand.Rand, len(setup.Names))
		for i := range bots {
			seed := taggedSeed(opts.Seed, digest, attempt, seedOpponent, uint64(i))
			bots[i] = rand.New(rand.NewPCG(seed[0], seed[1]))
		}
		boards := newOpponentBoards(len(setup.Names))
		pos, submits := 0, 0
		accepted := true
		knownGot := make(map[uint32]Identity)
		knownWant := make(map[uint32]Identity)
		for i, want := range h.Frames {
			got, err := observer.Capture(e, e.L.Events[pos:])
			if err != nil {
				return result, err
			}
			for _, identity := range got.Identities {
				knownGot[identity.ID] = identity
			}
			for _, identity := range want.Identities {
				knownWant[identity.ID] = identity
			}
			if !reflect.DeepEqual(got, want) {
				result.PrefixRejected++
				bucket := rejectionBucket(i, got, want)
				addRejection(&result, bucket)
				if bucket.Component == "identities" && bucket.Shape == "hand_to_stack" {
					result.HandToStackCauses.add(handToStackCause(got, want, knownGot, knownWant))
					addStackRejectionContext(&result, stackRejectionContext(got, want, knownGot, knownWant, stackConstraints[i]))
				}
				if result.FirstRejection == "" {
					result.FirstRejection = frameDifference(i, got, want)
				}
				accepted = false
				break
			}
			if i == len(h.Frames)-1 {
				break
			}
			if submits >= opts.MaxSubmits {
				result.BudgetExhausted++
				accepted = false
				break
			}
			d := e.Pending()
			if d == nil {
				result.PrefixRejected++
				addRejection(&result, RejectionBucket{Frame: i, Component: "decision", Shape: "missing"})
				if result.FirstRejection == "" {
					result.FirstRejection = fmt.Sprintf("frame %d missing pending decision", i)
				}
				accepted = false
				break
			}
			var in decision.Intent
			if d.Player == h.Actor {
				actions, ok := h.Answers[i]
				if !ok {
					return result, fmt.Errorf("missing actor answer at frame %d", i)
				}
				in, err = observer.Match(d, actions)
				if err != nil {
					result.PrefixRejected++
					addRejection(&result, RejectionBucket{Frame: i, Component: "action", Shape: "mismatch"})
					if result.FirstRejection == "" {
						result.FirstRejection = fmt.Sprintf("frame %d action mismatch: %v", i, err)
					}
					accepted = false
					break
				}
			} else {
				in = botpolicy.Decide(opponentBoard(e, d.Player, boards), d, bots[d.Player])
			}
			pos = len(e.L.Events)
			submits++
			result.Submits++
			if err := e.SubmitHypothetical(in); err != nil {
				if errors.Is(err, errIncompatibleProposal) {
					accepted = false
					break
				}
				return result, fmt.Errorf("reconstruction submit: %w", err)
			}
		}
		if accepted {
			e.ClearHypotheticalPlanner()
			result.Accepted++
			proposals = append(proposals, World{Config: cfg, Engine: e, Observer: observer})
			logs = append(logs, proposal.logWeight)
		}
	}
	if len(proposals) == 0 {
		return result, nil
	}
	weights, ess, err := normalizeWeights(logs)
	if err != nil {
		return result, err
	}
	result.ESS = ess
	result.WeightDiagnostics = summarizeWeightDiagnostics(logs, weights)
	minESS := float64(opts.Worlds)
	if opts.MinESS > 0 {
		minESS = opts.MinESS
	} else if len(proposals) < opts.Worlds {
		return result, nil
	}
	if ess+1e-10 < minESS {
		return result, nil
	}
	seed := taggedSeed(opts.Seed, digest, 0, seedResampling)
	r := rand.New(rand.NewPCG(seed[0], seed[1]))
	indices, err := weightedIndices(weights, opts.Worlds, r)
	if err != nil {
		return result, err
	}
	seen := make(map[int]bool)
	for _, i := range indices {
		if seen[i] {
			result.Duplicates++
		}
		seen[i] = true
		w := proposals[i]
		w.Engine = w.Engine.Clone()
		w.Observer = w.Observer.clone()
		result.Worlds = append(result.Worlds, w)
	}
	return result, nil
}

func observedPrefixEvents(h History) int {
	total := 0
	for _, frame := range h.Frames {
		total += len(frame.Events)
	}
	return total
}

func newOpponentBoards(players int) []botpolicy.Board {
	boards := make([]botpolicy.Board, players)
	for i := range boards {
		boards[i] = botpolicy.NewBoard(players)
	}
	return boards
}

func opponentBoard(e *rules.Engine, player state.PlayerID, boards []botpolicy.Board) botpolicy.Board {
	return botpolicy.BoardFromGameInto(e.G, e, player, &boards[player])
}

func (c *HandToStackCauses) add(other HandToStackCauses) {
	c.PolicyCompetition += other.PolicyCompetition
	c.ObserverReference += other.ObserverReference
	c.ObservedCastMissing += other.ObservedCastMissing
	c.HypotheticalExtraCast += other.HypotheticalExtraCast
	c.Other += other.Other
}

func handToStackCause(got, want Frame, knownGot, knownWant map[uint32]Identity) HandToStackCauses {
	for _, identity := range got.Identities {
		knownGot[identity.ID] = identity
	}
	for _, identity := range want.Identities {
		knownWant[identity.ID] = identity
	}
	stackName := func(frame Frame, known map[uint32]Identity) (string, bool) {
		for _, event := range frame.Events {
			if event.From == state.ZHand && event.To == state.ZStack {
				identity, ok := known[event.Obj]
				if !ok || identity.Name == "" {
					return "", false
				}
				return identity.Name, true
			}
		}
		return "", false
	}
	gotName, gotOK := stackName(got, knownGot)
	wantName, wantOK := stackName(want, knownWant)
	if gotOK && wantOK {
		if gotName == wantName {
			return HandToStackCauses{ObserverReference: 1}
		}
		return HandToStackCauses{PolicyCompetition: 1}
	}
	if wantOK && !gotOK {
		return HandToStackCauses{ObservedCastMissing: 1}
	}
	if gotOK && !wantOK {
		return HandToStackCauses{HypotheticalExtraCast: 1}
	}
	return HandToStackCauses{Other: 1}
}

func stackRejectionContext(got, want Frame, knownGot, knownWant map[uint32]Identity, constraint string) StackRejectionContext {
	cause := handToStackCause(got, want, knownGot, knownWant)
	causeName := "other"
	switch {
	case cause.PolicyCompetition > 0:
		causeName = "policy_competition"
	case cause.ObserverReference > 0:
		causeName = "observer_reference"
	case cause.ObservedCastMissing > 0:
		causeName = "observed_cast_missing"
	case cause.HypotheticalExtraCast > 0:
		causeName = "hypothetical_extra_cast"
	}
	if constraint == "" {
		if cause.HypotheticalExtraCast > 0 {
			constraint = "no_observed_cast"
		} else {
			constraint = "unknown"
		}
	}
	step := "unknown"
	var board struct {
		Step string `json:"step"`
	}
	if json.Unmarshal(want.Board, &board) == nil && board.Step != "" {
		step = board.Step
	}
	return StackRejectionContext{Cause: causeName, Step: step, ExpectedAction: frameActionShape(want), Constraint: constraint, Count: 1}
}

func frameActionShape(frame Frame) string {
	for _, event := range frame.Events {
		if event.From == state.ZHand && event.To == state.ZStack {
			return "cast"
		}
	}
	for _, event := range frame.Events {
		switch event.Kind {
		case events.LandPlayed:
			return "land_play"
		case events.AbilityPush:
			return "ability"
		case events.DeclareAttackers, events.DeclareBlockers:
			return "combat"
		case events.Priority:
			return "pass"
		case events.Resolve:
			return "resolve"
		}
	}
	return "other"
}

func stackConstraintContexts(h History, epochs map[epochKey]epochConstraints) []string {
	out := make([]string, len(h.Frames))
	ordinals := make(map[state.PlayerID]int)
	names := make(map[uint32]Identity)
	for frameIndex, frame := range h.Frames {
		for _, identity := range frame.Identities {
			names[identity.ID] = identity
		}
		for _, event := range frame.Events {
			if event.Kind == events.Shuffle {
				ordinals[event.Player]++
				continue
			}
			if event.From != state.ZHand || event.To != state.ZStack {
				continue
			}
			identity, ok := names[event.Obj]
			if !ok || identity.Name == "" || ordinals[identity.Owner] == 0 {
				out[frameIndex] = "unknown"
				break
			}
			if identity.Owner == h.Actor {
				out[frameIndex] = "actor"
				break
			}
			epoch := epochs[epochKey{Player: identity.Owner, Ordinal: ordinals[identity.Owner] - 1}]
			out[frameIndex] = "unconstrained"
			for _, deadline := range epoch.Deadlines {
				if deadline.Name == identity.Name {
					out[frameIndex] = "supported"
					break
				}
			}
			if out[frameIndex] == "unconstrained" && len(epoch.Unguided) > 0 {
				out[frameIndex] = "unguided"
			}
			break
		}
	}
	return out
}

func addStackRejectionContext(result *SampleResult, bucket StackRejectionContext) {
	for i := range result.StackRejectionContexts {
		got := &result.StackRejectionContexts[i]
		if got.Cause == bucket.Cause && got.Step == bucket.Step && got.ExpectedAction == bucket.ExpectedAction && got.Constraint == bucket.Constraint {
			got.Count++
			return
		}
	}
	result.StackRejectionContexts = append(result.StackRejectionContexts, bucket)
	sort.Slice(result.StackRejectionContexts, func(i, j int) bool {
		a, b := result.StackRejectionContexts[i], result.StackRejectionContexts[j]
		if a.Cause != b.Cause {
			return a.Cause < b.Cause
		}
		if a.Step != b.Step {
			return a.Step < b.Step
		}
		if a.ExpectedAction != b.ExpectedAction {
			return a.ExpectedAction < b.ExpectedAction
		}
		return a.Constraint < b.Constraint
	})
}

func addRejection(result *SampleResult, bucket RejectionBucket) {
	for i := range result.Rejections {
		got := &result.Rejections[i]
		if got.Frame == bucket.Frame && got.Component == bucket.Component && got.Shape == bucket.Shape {
			got.Count++
			return
		}
	}
	bucket.Count = 1
	result.Rejections = append(result.Rejections, bucket)
	sort.Slice(result.Rejections, func(i, j int) bool {
		a, b := result.Rejections[i], result.Rejections[j]
		if a.Frame != b.Frame {
			return a.Frame < b.Frame
		}
		if a.Component != b.Component {
			return a.Component < b.Component
		}
		return a.Shape < b.Shape
	})
}

func rejectionBucket(frame int, got, want Frame) RejectionBucket {
	if !reflect.DeepEqual(got.Identities, want.Identities) {
		return RejectionBucket{Frame: frame, Component: "identities", Shape: identityDifferenceShape(got, want)}
	}
	if !reflect.DeepEqual(got.Events, want.Events) {
		limit := len(got.Events)
		if len(want.Events) < limit {
			limit = len(want.Events)
		}
		for i := 0; i < limit; i++ {
			if !reflect.DeepEqual(got.Events[i], want.Events[i]) {
				return RejectionBucket{Frame: frame, Component: "events", Shape: got.Events[i].Kind.String() + "_to_" + want.Events[i].Kind.String()}
			}
		}
		return RejectionBucket{Frame: frame, Component: "events", Shape: "count"}
	}
	if string(got.Board) != string(want.Board) {
		return RejectionBucket{Frame: frame, Component: "board", Shape: "state"}
	}
	return RejectionBucket{Frame: frame, Component: "decision", Shape: "fields"}
}

func identityDifferenceShape(got, want Frame) string {
	declarations := [2]map[uint32]Identity{make(map[uint32]Identity), make(map[uint32]Identity)}
	frames := []Frame{want, got}
	for i, f := range frames {
		for _, id := range f.Identities {
			declarations[i][id.ID] = id
		}
	}
	for i, f := range frames {
		for _, identity := range f.Identities {
			if other, exists := declarations[1-i][identity.ID]; exists && other == identity {
				continue
			}
			for _, ev := range f.Events {
				if ev.Obj != identity.ID || ev.From != state.ZHand {
					continue
				}
				switch ev.To {
				case state.ZBattlefield:
					return "hand_to_battlefield"
				case state.ZStack:
					return "hand_to_stack"
				case state.ZExile:
					return "hand_to_exile"
				default:
					return "hand_to_other"
				}
			}
		}
	}
	return "other"
}

func frameDifference(i int, got, want Frame) string {
	part := "decision"
	if !reflect.DeepEqual(got.Identities, want.Identities) {
		part = "identities"
	} else if !reflect.DeepEqual(got.Events, want.Events) {
		part = "events"
	} else if string(got.Board) != string(want.Board) {
		part = "board"
	}
	return fmt.Sprintf("frame %d %s", i, part)
}
