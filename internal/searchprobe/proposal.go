package searchprobe

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"math/big"
	"math/rand/v2"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
)

const (
	seedEngine uint64 = iota + 1
	seedProposal
	seedOpponent
	seedResampling
)

// Each role owns a stream; neither call order nor worker allocation enters it.
func taggedSeed(base uint64, history [32]byte, attempt int, tag uint64, values ...uint64) [2]uint64 {
	h := sha256.New()
	h.Write(history[:])
	var b [8]byte
	for _, v := range append([]uint64{base, uint64(attempt), tag}, values...) {
		binary.LittleEndian.PutUint64(b[:], v)
		h.Write(b[:])
	}
	sum := h.Sum(nil)
	return [2]uint64{binary.LittleEndian.Uint64(sum[:8]), binary.LittleEndian.Uint64(sum[8:16])}
}

var errIncompatibleProposal = errors.New("incompatible hypothetical proposal")

type proposalState struct {
	epochs    map[epochKey]epochConstraints
	landNames map[state.PlayerID]map[string]bool
	logWeight float64
	base      uint64
	history   [32]byte
	attempt   int
	observer  *Collector
	result    *SampleResult
}

func publicLandNames(setup PublicGame) map[state.PlayerID]map[string]bool {
	out := make(map[state.PlayerID]map[string]bool, len(setup.Decks))
	for player, deck := range setup.Decks {
		names := make(map[string]bool)
		for _, card := range deck {
			if card.Faces[0].IsLand() {
				names[card.Faces[0].Name] = true
			}
		}
		out[state.PlayerID(player)] = names
	}
	return out
}

func isolationUpperDeadlines(epoch epochConstraints, handCounts map[string]int, landNames map[string]bool) ([]upperDeadlineConstraint, bool) {
	if len(epoch.LandIsolation) == 0 {
		return nil, false
	}
	names := make([]string, 0, len(landNames))
	for name := range landNames {
		names = append(names, name)
	}
	sort.Strings(names)
	var upper []upperDeadlineConstraint
	for _, fact := range epoch.LandIsolation {
		prior := make(map[string]int, len(fact.PriorExits))
		for _, exited := range fact.PriorExits {
			prior[exited.Name] = exited.Count
		}
		for _, name := range names {
			if name == fact.Name {
				continue
			}
			upper = append(upper, upperDeadlineConstraint{
				Through: fact.Through,
				Name:    name,
				Count:   prior[name] - handCounts[name],
			})
		}
	}
	return upper, true
}

func sampleLandIsolationMixture(cards []proposalCard, positions []positionConstraint, deadlines []deadlineConstraint, upper []upperDeadlineConstraint, eligible bool, r proposalRandom) ([]state.ObjID, float64, bool, bool, bool, error) {
	baseCounter, baseAvailable, err := newConstraintCounter(cards, positions, deadlines, nil)
	if err != nil {
		return nil, 0, false, false, false, err
	}
	baseCount := baseCounter.total(baseAvailable)
	if baseCount.Sign() == 0 {
		return nil, 0, false, false, false, nil
	}
	if !eligible {
		if r == nil {
			return nil, 0, false, false, false, errors.New("missing permutation randomness")
		}
		order, err := baseCounter.unrank(baseAvailable, baseCount, r)
		if err != nil {
			return nil, 0, false, false, false, err
		}
		return order, mixtureLogTargetOverProposal(len(cards), baseCount, nil, false), true, false, false, nil
	}

	isolatedCount := new(big.Int)
	var isolatedCounter *constraintCounter
	var isolatedAvailable []proposalCard
	negative := false
	for _, bound := range upper {
		if bound.Count < 0 {
			negative = true
			break
		}
	}
	if !negative {
		isolatedCounter, isolatedAvailable, err = newConstraintCounter(cards, positions, deadlines, upper)
		if err != nil {
			return nil, 0, false, false, false, err
		}
		isolatedCount = isolatedCounter.total(isolatedAvailable)
	}
	if isolatedCount.Sign() == 0 {
		if r == nil {
			return nil, 0, false, false, false, errors.New("missing permutation randomness")
		}
		order, err := baseCounter.unrank(baseAvailable, baseCount, r)
		if err != nil {
			return nil, 0, false, false, false, err
		}
		return order, mixtureLogTargetOverProposal(len(cards), baseCount, nil, false), true, false, true, nil
	}
	if r == nil {
		return nil, 0, false, false, false, errors.New("missing permutation randomness")
	}
	selected := r.Uint64()&1 == 1
	selectedCounter, selectedAvailable, selectedCount := baseCounter, baseAvailable, baseCount
	if selected {
		selectedCounter, selectedAvailable, selectedCount = isolatedCounter, isolatedAvailable, isolatedCount
	}
	order, err := selectedCounter.unrank(selectedAvailable, selectedCount, r)
	if err != nil {
		return nil, 0, false, selected, false, err
	}
	factor := mixtureLogTargetOverProposal(len(cards), baseCount, isolatedCount, isolatedCounter.contains(order))
	return order, factor, true, selected, false, nil
}

func publicToss(setup PublicGame, h History) ([]rules.ChanceDraw, float64, error) {
	for _, f := range h.Frames {
		for _, ev := range f.Events {
			if ev.Kind == events.Note && strings.HasSuffix(ev.Text, " won the toss") {
				if int(ev.Player) >= len(setup.Names) {
					return nil, 0, fail("contradictory", "invalid public toss")
				}
				return []rules.ChanceDraw{{Bound: len(setup.Names), Value: int(ev.Player)}}, -math.Log(float64(len(setup.Names))), nil
			}
		}
	}
	return nil, 0, fail("invariant", "missing public genesis toss")
}

// Before there is a hypothetical past, impossible genesis requirements are
// contradictions in the public input, not unlucky proposals.
func validateGenesis(setup PublicGame, epochs map[epochKey]epochConstraints) error {
	for player, deck := range setup.Decks {
		ep := epochs[epochKey{Player: state.PlayerID(player)}]
		cards := make([]proposalCard, len(deck))
		for i, card := range deck {
			cards[i] = proposalCard{ID: state.ObjID(i + 1), Name: card.Faces[0].Name}
		}
		positions := make([]positionConstraint, len(ep.Positions))
		for i, p := range ep.Positions {
			positions[i] = positionConstraint{Index: p.Index, Name: p.Name}
		}
		counter, available, err := newConstraintCounter(cards, positions, ep.Deadlines, nil)
		if err != nil {
			return fail("contradictory", "invalid genesis constraints: %v", err)
		}
		if counter.count(0, counter.initialFree, len(available)-sumInts(counter.initialFree)).Sign() == 0 {
			return fail("contradictory", "genesis constraints have no public-deck completion for player %d", player)
		}
	}
	return nil
}

func (p *proposalState) plan(ctx rules.ShuffleContext) ([]state.ObjID, error) {
	ep := p.epochs[epochKey{Player: ctx.Player, Ordinal: ctx.Ordinal}]
	p.result.UnguidedConstraints += len(ep.Unguided)
	if len(ep.Positions) == 0 && len(ep.Deadlines) == 0 && len(ep.LandIsolation) == 0 {
		return nil, nil
	}
	cards := make([]proposalCard, len(ctx.Library))
	byID := make(map[state.ObjID]string, len(ctx.Library))
	for i, card := range ctx.Library {
		cards[i] = proposalCard{ID: card.ID, Name: card.Name}
		byID[card.ID] = card.Name
	}
	positions := make([]positionConstraint, len(ep.Positions))
	used := make(map[state.ObjID]bool)
	for i, pos := range ep.Positions {
		positions[i] = positionConstraint{Index: pos.Index, Name: pos.Name}
		// Only this attempt's prefix-aligned observer can translate a reference.
		// Future introductions have no mapping yet and remain name constraints.
		positions[i].Obj = p.observer.object(pos.Ref)
		obj := positions[i].Obj
		if pos.Index >= len(cards) || obj != 0 && (byID[obj] != pos.Name || used[obj]) {
			return p.incompatible()
		}
		if obj != 0 {
			used[obj] = true
		}
	}
	// Requirements are cumulative by name, so each deadline subtracts the same
	// pre-shuffle hand count, not a greedily consumed sequence of copies.
	handCounts := make(map[string]int)
	if ctx.Player != p.observer.actor {
		for _, card := range ctx.Hand {
			handCounts[card.Name]++
		}
	}
	deadlines := append([]deadlineConstraint(nil), ep.Deadlines...)
	for i := range deadlines {
		deadlines[i].Count = max(0, deadlines[i].Count-handCounts[deadlines[i].Name])
		if deadlines[i].Through > len(cards) {
			return p.incompatible()
		}
	}
	upper, isolationEligible := isolationUpperDeadlines(ep, handCounts, p.landNames[ctx.Player])
	seed := taggedSeed(p.base, p.history, p.attempt, seedProposal, uint64(ctx.Player), uint64(ctx.Ordinal))
	order, factor, compatible, isolationSelected, isolationEmpty, err := sampleLandIsolationMixture(cards, positions, deadlines, upper, isolationEligible, rand.New(rand.NewPCG(seed[0], seed[1])))
	if err != nil {
		return nil, err
	}
	if !compatible {
		return p.incompatible()
	}
	if isolationEligible {
		p.result.LandIsolationEligible++
	}
	if isolationSelected {
		p.result.LandIsolationSelected++
	}
	if isolationEmpty {
		p.result.LandIsolationEmpty++
	}
	p.logWeight += factor
	if ctx.Ordinal == 0 {
		p.result.GuidedGenesis++
	} else {
		p.result.GuidedLater++
	}
	p.result.ArrangeWindows += ep.ArrangeWindows
	return order, nil
}

func (p *proposalState) incompatible() ([]state.ObjID, error) {
	p.result.IncompatibleProposals++
	return nil, errIncompatibleProposal
}
