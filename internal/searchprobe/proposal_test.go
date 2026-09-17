package searchprobe

import (
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"math/rand/v2"
	"reflect"
	"runtime"
	"testing"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
)

type sequenceProposalRandom struct {
	values []uint64
	at     int
}

func (r *sequenceProposalRandom) Uint64() uint64 {
	value := r.values[r.at]
	r.at++
	return value
}

func TestLandIsolationUpperDeadlines(t *testing.T) {
	tests := []struct {
		name       string
		epoch      epochConstraints
		handCounts map[string]int
		landNames  map[string]bool
		want       []upperDeadlineConstraint
		eligible   bool
	}{
		{
			name:      "no supported facts",
			landNames: map[string]bool{"A": true, "B": true},
		},
		{
			name:      "different land excluded",
			epoch:     epochConstraints{LandIsolation: []landIsolationConstraint{{Through: 7, Name: "A"}}},
			landNames: map[string]bool{"B": true, "A": true},
			want:      []upperDeadlineConstraint{{Through: 7, Name: "B", Count: 0}},
			eligible:  true,
		},
		{
			name: "prior exit permits one copy",
			epoch: epochConstraints{LandIsolation: []landIsolationConstraint{{
				Through: 7, Name: "A", PriorExits: []nameCount{{Name: "B", Count: 1}},
			}}},
			landNames: map[string]bool{"A": true, "B": true},
			want:      []upperDeadlineConstraint{{Through: 7, Name: "B", Count: 1}},
			eligible:  true,
		},
		{
			name:       "boundary hand makes isolation empty",
			epoch:      epochConstraints{LandIsolation: []landIsolationConstraint{{Through: 7, Name: "A"}}},
			handCounts: map[string]int{"B": 1},
			landNames:  map[string]bool{"A": true, "B": true},
			want:       []upperDeadlineConstraint{{Through: 7, Name: "B", Count: -1}},
			eligible:   true,
		},
		{
			name: "successive plays",
			epoch: epochConstraints{LandIsolation: []landIsolationConstraint{
				{Through: 7, Name: "A"},
				{Through: 8, Name: "B", PriorExits: []nameCount{{Name: "A", Count: 1}}},
			}},
			landNames: map[string]bool{"B": true, "A": true},
			want: []upperDeadlineConstraint{
				{Through: 7, Name: "B", Count: 0},
				{Through: 8, Name: "A", Count: 1},
			},
			eligible: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, eligible := isolationUpperDeadlines(tc.epoch, tc.handCounts, tc.landNames)
			if eligible != tc.eligible || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("bounds=%+v eligible=%v, want %+v eligible=%v", got, eligible, tc.want, tc.eligible)
			}
		})
	}
}

func TestPublicLandNamesUsesPlayableFrontFace(t *testing.T) {
	land := syntheticCard(t, "Name:A\nTypes:Land\nOracle:Fixture.\nALTERNATE\nName:A Back\nTypes:Sorcery\nOracle:Fixture.\n")
	backOnly := syntheticCard(t, "Name:Front Spell\nTypes:Sorcery\nOracle:Fixture.\nALTERNATE\nName:Back Land\nTypes:Land\nOracle:Fixture.\n")
	setup := PublicGame{Decks: [][]*cards.Card{{land, land, backOnly}, {backOnly}}}
	want := map[state.PlayerID]map[string]bool{0: {"A": true}, 1: {}}
	if got := publicLandNames(setup); !reflect.DeepEqual(got, want) {
		t.Fatalf("public land names = %#v, want %#v", got, want)
	}
}

func TestLandIsolationMixturePreservesSupportAndExactWeights(t *testing.T) {
	cards := []proposalCard{{ID: 1, Name: "A"}, {ID: 2, Name: "A"}, {ID: 3, Name: "B"}, {ID: 4, Name: "C"}}
	upper := []upperDeadlineConstraint{{Through: 2, Name: "B", Count: 0}}
	baseCount, isolatedCount := big.NewInt(24), big.NewInt(12)
	baseOrders := make(map[string]bool)
	baseReachedIsolation := false
	for rank := uint64(0); rank < 24; rank++ {
		rng := &sequenceProposalRandom{values: []uint64{0, rank}}
		order, factor, compatible, selected, empty, err := sampleLandIsolationMixture(cards, nil, nil, upper, true, rng)
		if err != nil || !compatible || selected || empty {
			t.Fatalf("base rank %d: compatible=%v selected=%v empty=%v err=%v", rank, compatible, selected, empty, err)
		}
		inside := permutationSatisfies(order, cards, nil, nil, upper)
		baseReachedIsolation = baseReachedIsolation || inside
		want := mixtureLogTargetOverProposal(4, baseCount, isolatedCount, inside)
		if math.Abs(factor-want) > 1e-12 {
			t.Fatalf("base rank %d weight=%g want=%g", rank, factor, want)
		}
		baseOrders[keyIDs(order)] = true
	}
	if len(baseOrders) != 24 || !baseReachedIsolation {
		t.Fatalf("base support=%d reached isolation=%v", len(baseOrders), baseReachedIsolation)
	}
	isolatedOrders := make(map[string]bool)
	for rank := uint64(0); rank < 12; rank++ {
		rng := &sequenceProposalRandom{values: []uint64{1, rank}}
		order, factor, compatible, selected, empty, err := sampleLandIsolationMixture(cards, nil, nil, upper, true, rng)
		if err != nil || !compatible || !selected || empty {
			t.Fatalf("isolated rank %d: compatible=%v selected=%v empty=%v err=%v", rank, compatible, selected, empty, err)
		}
		if !permutationSatisfies(order, cards, nil, nil, upper) {
			t.Fatalf("isolation component emitted %v outside L", order)
		}
		want := mixtureLogTargetOverProposal(4, baseCount, isolatedCount, true)
		if math.Abs(factor-want) > 1e-12 {
			t.Fatalf("isolated rank %d weight=%g want=%g", rank, factor, want)
		}
		isolatedOrders[keyIDs(order)] = true
	}
	if len(isolatedOrders) != 12 {
		t.Fatalf("isolated support=%d, want 12 distinct physical orders", len(isolatedOrders))
	}
	for order := range isolatedOrders {
		if !baseOrders[order] {
			t.Fatalf("isolated order %s absent from base support", order)
		}
	}
}

func TestLandIsolationEmptySetConsumesNoSelectionAndKeepsPrior(t *testing.T) {
	cards := []proposalCard{{ID: 1, Name: "A"}, {ID: 2, Name: "B"}, {ID: 3, Name: "C"}}
	for _, upper := range [][]upperDeadlineConstraint{
		{{Through: 3, Name: "B", Count: 0}},
		{{Through: 1, Name: "B", Count: -1}},
	} {
		rng := &sequenceProposalRandom{values: []uint64{2}}
		order, factor, compatible, selected, empty, err := sampleLandIsolationMixture(cards, nil, nil, upper, true, rng)
		if err != nil || !compatible || selected || !empty || rng.at != 1 {
			t.Fatalf("empty L: compatible=%v selected=%v empty=%v draws=%d err=%v", compatible, selected, empty, rng.at, err)
		}
		wantOrder, wantFactor, wantCompatible, err := sampleConstrainedPermutation(cards, nil, nil, nil, &rankRandom{value: 2})
		if err != nil || !wantCompatible || !reflect.DeepEqual(order, wantOrder) || factor != wantFactor {
			t.Fatalf("empty L changed prior: order=%v/%v factor=%g/%g err=%v", order, wantOrder, factor, wantFactor, err)
		}
	}
}

// A single playable land among nineteen uncastable spells is in an ordinary
// opening hand only 7/20 of the time. The public play must guide that deadline.
func TestProposalGuidesOpponentPublicLand(t *testing.T) {
	land := syntheticCard(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:Fixture.\n")
	spell := syntheticCard(t, "Name:Uncastable\nManaCost:99\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:Fixture.\n")
	decks := [][]*cards.Card{repeatCard(land, 20), repeatCard(spell, 20)}
	decks[1][0] = land
	setup, h := proposalHistory(t, decks, 1, func(e *rules.Engine) bool { return len(e.G.Zone(state.ZBattlefield, 1)) == 1 })
	result, err := Sample(setup, h, SampleOptions{Seed: 37, Attempts: 16, Worlds: 4, MaxSubmits: 500})
	if err != nil {
		t.Fatal(err)
	}
	if result.Accepted != 16 || len(result.Worlds) != 4 || result.ESS < 15.999 {
		t.Fatalf("opponent public land was not fully guided: %+v", result)
	}
	for _, w := range result.Worlds {
		if err := VerifyWorld(w); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLandIsolationKeepsFrozenOpponentPolicyAndReplays(t *testing.T) {
	plains := syntheticCard(t, "Name:Plains\nTypes:Basic Land Plains\nOracle:Fixture.\n")
	island := syntheticCard(t, "Name:Island\nTypes:Basic Land Island\nOracle:Fixture.\n")
	blueNeed := syntheticCard(t, "Name:Blue Need\nManaCost:U\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:Fixture.\n")
	filler := syntheticCard(t, "Name:Filler\nManaCost:99\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:Fixture.\n")
	mountain := syntheticCard(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:Fixture.\n")
	decks := [][]*cards.Card{repeatCard(mountain, 8), {plains, blueNeed, filler, filler, filler, filler, filler, island}}
	setup, h := proposalHistory(t, decks, 1, func(e *rules.Engine) bool {
		return len(e.G.Zone(state.ZBattlefield, 1)) == 1
	})

	outside := []string{"Plains", "Island", "Blue Need", "Filler", "Filler", "Filler", "Filler", "Filler"}
	matched, got, want, _ := replayWithOpponentGenesisOrder(t, setup, h, outside)
	if matched || rejectionBucket(len(h.Frames)-1, got, want).Shape != "hand_to_battlefield" {
		t.Fatalf("competing-land base world did not reject at the land play: matched=%v got=%+v want=%+v", matched, got, want)
	}
	inside := []string{"Plains", "Blue Need", "Filler", "Filler", "Filler", "Filler", "Filler", "Island"}
	matched, got, want, world := replayWithOpponentGenesisOrder(t, setup, h, inside)
	if !matched {
		t.Fatalf("isolated world failed the complete prefix: got=%+v want=%+v", got, want)
	}
	if err := VerifyWorld(world); err != nil {
		t.Fatalf("isolated world did not replay without a planner: %v", err)
	}

	result, err := Sample(setup, h, SampleOptions{Seed: 137, Attempts: 32, Worlds: 4, MaxSubmits: 500})
	if err != nil {
		t.Fatal(err)
	}
	if result.LandIsolationEligible != 32 || result.LandIsolationSelected == 0 || result.LandIsolationEmpty != 0 || result.LandIsolationUnsupported != 0 {
		t.Fatalf("land mixture diagnostics = %+v", result)
	}
	if result.Accepted < result.LandIsolationSelected || result.PrefixRejected == 0 || len(result.Worlds) != 4 {
		t.Fatalf("mixture did not retain isolated worlds and base rejection: accepted=%d selected=%d rejected=%d worlds=%d first=%s", result.Accepted, result.LandIsolationSelected, result.PrefixRejected, len(result.Worlds), result.FirstRejection)
	}
	for _, world := range result.Worlds {
		if err := VerifyWorld(world); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProposalReplaysNamelessActivatedAbilityHistory(t *testing.T) {
	card := syntheticCard(t, "Name:Fixture Fountain\nTypes:Land Creature Construct\nPT:1/1\nK:Haste\nA:AB$ GainLife | Cost$ T | Defined$ You | LifeAmount$ 1\nOracle:Fixture.\n")
	decks := [][]*cards.Card{repeatCard(card, 20), repeatCard(card, 20)}
	setup, h := proposalHistory(t, decks, 0, func(e *rules.Engine) bool { return len(e.G.Stack) > 0 })
	assertNamelessAbilityHistorySamples(t, setup, h, events.AbilityPush)
}

func TestProposalReplaysNamelessTriggeredAbilityHistory(t *testing.T) {
	card := syntheticCard(t, "Name:Fixture Spring\nTypes:Land\nT:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ Gain\nSVar:Gain:DB$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:Fixture.\n")
	decks := [][]*cards.Card{repeatCard(card, 20), repeatCard(card, 20)}
	setup, h := proposalHistory(t, decks, 0, func(e *rules.Engine) bool { return len(e.G.Stack) > 0 })
	assertNamelessAbilityHistorySamples(t, setup, h, events.TriggerPush)
}

func assertNamelessAbilityHistorySamples(t *testing.T, setup PublicGame, h History, kind events.Kind) {
	t.Helper()
	nameless, pushed := false, false
	for _, f := range h.Frames {
		for _, id := range f.Identities {
			nameless = nameless || id.ID != 0 && id.Name == ""
		}
		for _, ev := range f.Events {
			pushed = pushed || ev.Kind == kind
		}
	}
	if !nameless || !pushed {
		t.Fatalf("fixture missing actual nameless stack ability: nameless=%v push=%v", nameless, pushed)
	}
	if _, err := compileEpochs(h); err != nil {
		t.Fatalf("valid stack ability rejected by compiler: %v", err)
	}
	result, err := Sample(setup, h, SampleOptions{Seed: 799, Attempts: 8, Worlds: 4, MaxSubmits: 500})
	if err != nil {
		t.Fatalf("valid stack ability rejected by sampling: %v", err)
	}
	if result.Accepted != 8 || len(result.Worlds) != 4 {
		t.Fatalf("ability reconstruction accepted=%d worlds=%d first=%s", result.Accepted, len(result.Worlds), result.FirstRejection)
	}
	for _, world := range result.Worlds {
		if err := VerifyWorld(world); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProposalOpponentWeightMatchesExhaustivePhysicalPermutations(t *testing.T) {
	land := syntheticCard(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:Fixture.\n")
	b := syntheticCard(t, "Name:Uncastable\nManaCost:99\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:Fixture.\n")
	a := syntheticCard(t, "Name:Public Spell\nManaCost:0\nTypes:Sorcery\nA:SP$ GainLife | LifeAmount$ 1\nOracle:Fixture.\n")
	decks := [][]*cards.Card{repeatCard(land, 8), repeatCard(b, 8)}
	decks[1][0] = a
	setup, h := proposalHistory(t, decks, 1, func(e *rules.Engine) bool { return len(e.G.Zone(state.ZGraveyard, 1)) == 1 })
	epochs, err := compileEpochs(h)
	if err != nil {
		t.Fatal(err)
	}
	tape, tossWeight, err := publicToss(setup, h)
	if err != nil {
		t.Fatal(err)
	}
	p := &proposalState{epochs: epochs, observer: NewCollector(0), result: &SampleResult{}, logWeight: tossWeight}
	e, err := rules.NewHypotheticalPlanned(rules.Config{Seed: 91, Names: setup.Names, Decks: decks}, tape, p.plan)
	if err != nil {
		t.Fatal(err)
	}
	// Card zero is the single spell. Enumerate all eight distinct physical
	// objects, independently of the constrained sampler and its DP counts.
	order := []int{0, 1, 2, 3, 4, 5, 6, 7}
	total, compatible := 0, 0
	var walk func(int)
	walk = func(pos int) {
		if pos == len(order) {
			total++
			for _, card := range order[:7] {
				if card == 0 {
					compatible++
					break
				}
			}
			return
		}
		for j := pos; j < len(order); j++ {
			order[pos], order[j] = order[j], order[pos]
			walk(pos + 1)
			order[pos], order[j] = order[j], order[pos]
		}
	}
	walk(0)
	if total != 40320 || compatible != 35280 {
		t.Fatalf("bad exhaustive fixture: %d/%d", compatible, total)
	}
	want := float64(compatible) / float64(total) / 2
	if math.Abs(math.Exp(p.logWeight)-want) > 1e-12 {
		t.Fatalf("actual engine proposal weight=%g want %g", math.Exp(p.logWeight), want)
	}
	found := false
	for _, id := range e.G.Zone(state.ZHand, 1) {
		if e.G.Obj(id).Card == a {
			found = true
		}
	}
	if !found {
		t.Fatal("real genesis failed the public spell deadline")
	}
}

// With the required land already in hand, requiring it AGAIN from the later
// library would falsely reject that otherwise compatible hypothetical past.
func TestProposalLaterShuffleCreditsOpponentHand(t *testing.T) {
	land := syntheticCard(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:Fixture.\n")
	spell := syntheticCard(t, "Name:Uncastable\nManaCost:99\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:Fixture.\n")
	shuffle := syntheticCard(t, "Name:Shuffle Opponent\nManaCost:0\nTypes:Sorcery\nA:SP$ Shuffle | Defined$ Opponent\nOracle:Fixture.\n")
	decks := [][]*cards.Card{repeatCard(land, 20), repeatCard(spell, 20)}
	decks[0][0], decks[1][0] = shuffle, land
	setup, h := proposalHistory(t, decks, 0, func(e *rules.Engine) bool { return len(e.G.Zone(state.ZBattlefield, 1)) == 1 })
	result, err := Sample(setup, h, SampleOptions{Seed: 91, Attempts: 16, Worlds: 1, MaxSubmits: 500})
	if err != nil {
		t.Fatal(err)
	}
	if result.Accepted != 16 || result.IncompatibleProposals != 0 || result.GuidedLater != 16 {
		t.Fatalf("pre-shuffle hand was not credited: accepted=%d incompatible=%d later=%d first=%s", result.Accepted, result.IncompatibleProposals, result.GuidedLater, result.FirstRejection)
	}
	for _, w := range result.Worlds {
		if err := VerifyWorld(w); err != nil {
			t.Fatal(err)
		}
	}
	result, err = Sample(setup, h, SampleOptions{Seed: 91, Attempts: 16, Worlds: 16, MaxSubmits: 500})
	if err != nil {
		t.Fatal(err)
	}
	if result.Accepted != 16 || result.ESS >= 16 || len(result.Worlds) != 0 {
		t.Fatalf("ESS fallback bypassed: accepted=%d ESS=%g worlds=%d", result.Accepted, result.ESS, len(result.Worlds))
	}
}

func TestProposalLaterDrawsPreserveKnownDuplicateObjects(t *testing.T) {
	land := syntheticCard(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:Fixture.\n")
	spell := syntheticCard(t, "Name:Return Shuffle Draw\nManaCost:0\nTypes:Sorcery\nA:SP$ ChangeZoneAll | Origin$ Hand | Destination$ Library | ChangeType$ Card.YouOwn | SubAbility$ Mix\nSVar:Mix:DB$ Shuffle | Defined$ You | SubAbility$ Pull\nSVar:Pull:DB$ Draw | Defined$ You | NumCards$ 5\nOracle:Fixture.\n")
	decks := [][]*cards.Card{repeatCard(land, 12), repeatCard(land, 12)}
	decks[0][0] = spell
	setup, h := proposalHistory(t, decks, 0, func(e *rules.Engine) bool { return len(e.G.Zone(state.ZGraveyard, 0)) == 1 })
	epochs, err := compileEpochs(h)
	if err != nil {
		t.Fatal(err)
	}
	known := 0
	for _, p := range epochs[epochKey{Player: 0, Ordinal: 1}].Positions {
		if p.Ref != 0 {
			known++
		}
	}
	if known == 0 {
		t.Fatal("fixture did not redraw a known object")
	}
	result, err := Sample(setup, h, SampleOptions{Seed: 219, Attempts: 16, Worlds: 4, MaxSubmits: 500})
	if err != nil {
		t.Fatal(err)
	}
	// Previously-known objects are exact constraints; unseen-name constraints
	// may still choose a different known duplicate and fail the full prefix.
	if len(result.Worlds) != 4 || result.GuidedLater != 16 {
		t.Fatalf("known physical redraw not guided: accepted=%d later=%d rejected=%d first=%s", result.Accepted, result.GuidedLater, result.PrefixRejected, result.FirstRejection)
	}
	for _, w := range result.Worlds {
		if err := VerifyWorld(w); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProposalIncompatibleLaterLibraryRejectsOnlyAttempt(t *testing.T) {
	epoch := epochConstraints{Positions: []epochPosition{{Index: 3, Name: "A"}}}
	_, result, _, err := runLaterProposal(t, epoch)
	if !errors.Is(err, errIncompatibleProposal) || result.IncompatibleProposals != 1 {
		t.Fatalf("too-short hypothetical library: err=%v incompatible=%d", err, result.IncompatibleProposals)
	}
}

func TestProposalDuplicateHandDeficitsHaveExactWeights(t *testing.T) {
	for _, tc := range []struct {
		name         string
		deadlines    []deadlineConstraint
		weight       float64
		incompatible bool
	}{
		{"already in hand", []deadlineConstraint{{Through: 0, Name: "A", Count: 1}}, 1, false},
		{"one of two in hand", []deadlineConstraint{{Through: 0, Name: "A", Count: 1}, {Through: 1, Name: "A", Count: 2}}, 1.0 / 3, false},
		{"no completion", []deadlineConstraint{{Through: 1, Name: "A", Count: 3}}, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, result, logWeight, err := runLaterProposal(t, epochConstraints{Deadlines: tc.deadlines})
			if tc.incompatible {
				if !errors.Is(err, errIncompatibleProposal) || result.IncompatibleProposals != 1 {
					t.Fatalf("err=%v diagnostics=%+v", err, result)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			// Enumerate all 3! physical permutations independently: ABB has six
			// orders, of which exactly two put the remaining A in the first slot.
			if math.Abs(math.Exp(logWeight)-tc.weight) > 1e-12 {
				t.Fatalf("weight=%g want %g", math.Exp(logWeight), tc.weight)
			}
			if tc.weight < 1 && e.G.Obj(e.G.Zone(state.ZLibrary, 1)[0]).Card.Faces[0].Name != "A" {
				t.Fatal("deadline not installed by Shuffle event")
			}
		})
	}
}

func TestProposalExecutesLaterShuffleAndArrangeDrawHistory(t *testing.T) {
	for _, tc := range []struct {
		name, body     string
		later, arrange int
	}{
		{"later shuffle draw", "Shuffle | Defined$ You | SubAbility$ Pull", 16, 0},
		{"arrange reordered draw", "RearrangeTopOfLibrary | Defined$ You | NumCards$ 3 | SubAbility$ Pull", 0, 16},
	} {
		t.Run(tc.name, func(t *testing.T) {
			land := syntheticCard(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:Fixture.\n")
			spell := syntheticCard(t, "Name:Look Shuffle Draw\nManaCost:0\nTypes:Sorcery\nA:SP$ "+tc.body+"\nSVar:Pull:DB$ Draw | Defined$ You | NumCards$ 1\nOracle:Fixture.\n")
			decks := [][]*cards.Card{repeatCard(land, 12), repeatCard(land, 12)}
			decks[0][0] = spell
			for i := 7; i < 12; i++ {
				decks[0][i] = syntheticCard(t, "Name:"+string(rune('A'+i-7))+"\nTypes:Land\nOracle:Fixture.\n")
			}
			setup, h := proposalHistory(t, decks, 0, func(e *rules.Engine) bool { return len(e.G.Zone(state.ZGraveyard, 0)) == 1 })
			if tc.arrange > 0 {
				found := false
				for i, f := range h.Frames {
					if f.Decision != nil && f.Decision.Kind == decision.KArrange {
						found = true
						if h.Answers[i][0].Obj != f.Decision.Options[2].Action.Obj {
							t.Fatal("fixture failed to reverse arrangement")
						}
					}
				}
				if !found {
					t.Fatal("missing arrange window")
				}
			}
			result, err := Sample(setup, h, SampleOptions{Seed: 619, Attempts: 16, Worlds: 4, MaxSubmits: 500})
			if err != nil {
				t.Fatal(err)
			}
			if result.Accepted != 16 || result.GuidedLater != tc.later || result.ArrangeWindows != tc.arrange {
				t.Fatalf("accepted=%d later=%d arrange=%d first=%s", result.Accepted, result.GuidedLater, result.ArrangeWindows, result.FirstRejection)
			}
			for _, w := range result.Worlds {
				if err := VerifyWorld(w); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestProposalGuidesUnseenDrawPastBottomedArrangeWindow(t *testing.T) {
	land := syntheticCard(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:Fixture.\n")
	spell := syntheticCard(t, "Name:Look Then Draw\nManaCost:0\nTypes:Sorcery\nA:SP$ Scry | Defined$ You | ScryNum$ 3 | SubAbility$ Pull\nSVar:Pull:DB$ Draw | Defined$ You | NumCards$ 2\nOracle:Fixture.\n")
	decks := [][]*cards.Card{repeatCard(land, 16), repeatCard(land, 16)}
	decks[0][0] = spell
	for i := 7; i < 16; i++ {
		decks[0][i] = syntheticCard(t, "Name:"+string(rune('A'+i-7))+"\nTypes:Land\nOracle:Fixture.\n")
	}
	setup, h := proposalHistory(t, decks, 0, func(e *rules.Engine) bool { return len(e.G.Zone(state.ZGraveyard, 0)) == 1 }, func(d *decision.Decision, in decision.Intent) decision.Intent {
		if d.Kind == decision.KArrange {
			in.Choices = []int{0}
		}
		return in
	})
	epochs, err := compileEpochs(h)
	if err != nil {
		t.Fatal(err)
	}
	ep := epochs[epochKey{Player: 0}]
	// Opening seven, then look at A/B/C; keep A and bottom B/C. The second
	// subsequent draw is previously unseen D at ORIGINAL index 10, not 8.
	found := false
	for _, p := range ep.Positions {
		if p.Name == "D" {
			found = p.Index == 10
		}
	}
	if !found || len(ep.Unguided) != 0 {
		t.Fatalf("post-arrange original position not guided: %+v", ep)
	}
	result, err := Sample(setup, h, SampleOptions{Seed: 883, Attempts: 8, Worlds: 4, MaxSubmits: 500})
	if err != nil {
		t.Fatal(err)
	}
	if result.Accepted != 8 || len(result.Worlds) != 4 {
		t.Fatalf("post-arrange guidance accepted=%d worlds=%d first=%s", result.Accepted, len(result.Worlds), result.FirstRejection)
	}
	for _, world := range result.Worlds {
		if err := VerifyWorld(world); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProposalSamplerContinuesAfterIncompatiblePast(t *testing.T) {
	land := syntheticCard(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:Fixture.\n")
	spell := syntheticCard(t, "Name:Uncastable\nManaCost:99\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:Fixture.\n")
	a := syntheticCard(t, "Name:Public Spell\nManaCost:0\nTypes:Sorcery\nA:SP$ GainLife | LifeAmount$ 1\nOracle:Fixture.\n")
	shuffle := syntheticCard(t, "Name:Shuffle Opponent\nManaCost:0\nTypes:Sorcery\nA:SP$ Shuffle | Defined$ Opponent\nOracle:Fixture.\n")
	decks := [][]*cards.Card{repeatCard(land, 20), repeatCard(spell, 20)}
	decks[0][0], decks[1][0], decks[1][1] = shuffle, a, a
	setup, h := proposalHistory(t, decks, 0, func(e *rules.Engine) bool { return len(e.G.Zone(state.ZGraveyard, 1)) == 2 })
	result, err := Sample(setup, h, SampleOptions{Seed: 43, Attempts: 24, Worlds: 1, MaxSubmits: 500})
	if err != nil {
		t.Fatal(err)
	}
	if result.Accepted == 0 || result.IncompatibleProposals == 0 || result.Attempts != 24 || len(result.Worlds) != 1 {
		t.Fatalf("failed to continue: accepted=%d incompatible=%d attempts=%d worlds=%d", result.Accepted, result.IncompatibleProposals, result.Attempts, len(result.Worlds))
	}
	if result.Accepted+result.IncompatibleProposals+result.PrefixRejected+result.BudgetExhausted != result.Attempts {
		t.Fatal("attempt accounting drift")
	}
	for _, w := range result.Worlds {
		if err := VerifyWorld(w); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProposalWorkerSchedulingPreservesWorlds(t *testing.T) {
	setup, h := samplingHistory(t)
	opts := SampleOptions{Seed: 83, Attempts: 8, Worlds: 4, MaxSubmits: 500}
	previous := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(previous)
	want, err := Sample(setup, h, opts)
	if err != nil {
		t.Fatal(err)
	}
	runtime.GOMAXPROCS(4)
	type outcome struct {
		result SampleResult
		err    error
	}
	results := make(chan outcome, 3)
	for range 3 {
		go func() { got, err := Sample(setup, h, opts); results <- outcome{got, err} }()
	}
	for range 3 {
		got := <-results
		if got.err != nil {
			t.Fatal(got.err)
		}
		assertSameProposalWorlds(t, want, got.result)
	}
}

func TestProposalIgnoresSourceSecretsAndPrivateIntentIndices(t *testing.T) {
	a, b := observationEngine(t, 17), observationEngine(t, 83)
	b.Rand(37)
	a.L.Intents = append(a.L.Intents, decision.Intent{Player: 1, Choices: []int{0}})
	b.L.Intents = append(b.L.Intents, decision.Intent{Player: 1, Choices: []int{19}})
	if a.L.Head() == b.L.Head() || a.RNGDraws() == b.RNGDraws() {
		t.Fatal("fixture needs distinct source secrets")
	}
	fa, err := NewCollector(0).Capture(a, a.L.Events)
	if err != nil {
		t.Fatal(err)
	}
	fb, err := NewCollector(0).Capture(b, b.L.Events)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fa, fb) {
		t.Fatal("source secrets reached owned observation")
	}
	setup, _ := samplingHistory(t)
	opts := SampleOptions{Seed: 89, Attempts: 8, Worlds: 4, MaxSubmits: 500}
	x, err := Sample(setup, History{Actor: 0, Frames: []Frame{fa}}, opts)
	if err != nil {
		t.Fatal(err)
	}
	y, err := Sample(setup, History{Actor: 0, Frames: []Frame{fb}}, opts)
	if err != nil {
		t.Fatal(err)
	}
	assertSameProposalWorlds(t, x, y)
}

func assertSameProposalWorlds(t *testing.T, a, b SampleResult) {
	t.Helper()
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	if string(aj) != string(bj) || len(a.Worlds) != 4 || len(b.Worlds) != 4 {
		t.Fatal("proposal diagnostics differ or worlds missing")
	}
	for i := range a.Worlds {
		x, y := a.Worlds[i], b.Worlds[i]
		if x.Engine.L.Head() != y.Engine.L.Head() || !reflect.DeepEqual(x.Engine.ChanceTranscript(), y.Engine.ChanceTranscript()) || !reflect.DeepEqual(x.Engine.L.Intents, y.Engine.L.Intents) {
			t.Fatal("proposal depends on source secret or worker schedule")
		}
	}
}

func TestProposalExplicitlyUnguidedEpochUsesOrdinaryPrior(t *testing.T) {
	card := syntheticCard(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:Fixture.\n")
	cfg := rules.Config{Seed: 87, Names: []string{"a", "b"}, Decks: [][]*cards.Card{repeatCard(card, 12), repeatCard(card, 12)}}
	p := &proposalState{epochs: map[epochKey]epochConstraints{{Player: 0}: {Unguided: []string{"library_mutation"}}}, observer: NewCollector(0), result: &SampleResult{}}
	a, err := rules.NewHypotheticalPlanned(cfg, nil, p.plan)
	if err != nil {
		t.Fatal(err)
	}
	b, err := rules.NewHypothetical(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if a.L.Head() != b.L.Head() || a.RNGDraws() != b.RNGDraws() || !reflect.DeepEqual(a.ChanceTranscript(), b.ChanceTranscript()) {
		t.Fatal("unguided epoch changed prior chance")
	}
	if p.logWeight != 0 || p.result.UnguidedConstraints != 1 || p.result.GuidedGenesis != 0 {
		t.Fatalf("unguided diagnostics=%+v weight=%g", p.result, p.logWeight)
	}
}

func TestProposalDuplicateResamplesOwnMutableState(t *testing.T) {
	setup, h := samplingHistory(t)
	for seed := uint64(0); seed < 20; seed++ {
		result, err := Sample(setup, h, SampleOptions{Seed: seed, Attempts: 4, Worlds: 4, MaxSubmits: 500})
		if err != nil {
			t.Fatal(err)
		}
		for i, a := range result.Worlds {
			for j, b := range result.Worlds {
				if i >= j || a.Config.Seed != b.Config.Seed {
					continue
				}
				beforeHead, beforeLife := b.Engine.L.Head(), b.Engine.G.Players[0].Life
				a.Engine.Emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: -3})
				if a.Engine.G.Players[0].Life != beforeLife-3 || b.Engine.G.Players[0].Life != beforeLife || b.Engine.L.Head() != beforeHead {
					t.Fatal("duplicate engines share mutable state")
				}
				id := a.Engine.G.Zone(state.ZLibrary, 0)[0]
				if a.Observer.ref(id) != 0 || b.Observer.ref(id) != 0 {
					t.Fatal("fixture card already known")
				}
				a.Observer.introduce(a.Engine, id)
				if a.Observer.ref(id) == 0 || b.Observer.ref(id) != 0 {
					t.Fatal("duplicate observer dictionaries alias")
				}
				return
			}
		}
	}
	t.Fatal("fixture never resampled a duplicate")
}

// Drive a real spell through the checked engine boundary. The opponent's
// actual hypothetical hand is ABBBBBB and the shuffled library is ABB.
func runLaterProposal(t *testing.T, epoch epochConstraints) (*rules.Engine, SampleResult, float64, error) {
	t.Helper()
	a := syntheticCard(t, "Name:A\nManaCost:99\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:Fixture.\n")
	b := syntheticCard(t, "Name:B\nManaCost:99\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:Fixture.\n")
	shuffle := syntheticCard(t, "Name:Shuffle Opponent\nManaCost:0\nTypes:Sorcery\nA:SP$ Shuffle | Defined$ Opponent\nOracle:Fixture.\n")
	decks := [][]*cards.Card{repeatCard(b, 10), repeatCard(b, 10)}
	decks[0][0], decks[1][0], decks[1][7] = shuffle, a, a
	tape := []rules.ChanceDraw{{Bound: 2, Value: 0}}
	for range decks {
		for n := 10; n > 1; n-- {
			tape = append(tape, rules.ChanceDraw{Bound: n, Value: n - 1})
		}
	}
	var result SampleResult
	c := NewCollector(0)
	p := &proposalState{epochs: map[epochKey]epochConstraints{{Player: 1, Ordinal: 1}: epoch}, observer: c, result: &result}
	e, err := rules.NewHypotheticalPlanned(rules.Config{Seed: 8, Names: []string{"a", "b"}, Decks: decks}, tape, p.plan)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.AdvanceHypothetical(); err != nil {
		t.Fatal(err)
	}
	r := rand.New(rand.NewPCG(17, 29))
	pos := 0
	for i := 0; i < 30; i++ {
		if _, err := c.Capture(e, e.L.Events[pos:]); err != nil {
			t.Fatal(err)
		}
		if len(e.G.Zone(state.ZGraveyard, 0)) > 0 {
			return e, result, p.logWeight, nil
		}
		d := e.Pending()
		if d == nil {
			t.Fatal("fixture ended before shuffle")
		}
		in := botpolicy.Decide(botpolicy.BoardFromGame(e.G, e, d.Player), d, r)
		pos = len(e.L.Events)
		if err := e.SubmitHypothetical(in); err != nil {
			return e, result, p.logWeight, err
		}
	}
	t.Fatal("fixture did not shuffle")
	return nil, result, p.logWeight, nil
}

func repeatCard(card *cards.Card, n int) []*cards.Card {
	deck := make([]*cards.Card, n)
	for i := range deck {
		deck[i] = card
	}
	return deck
}

func proposalHistory(t *testing.T, decks [][]*cards.Card, toss int, stop func(*rules.Engine) bool, answers ...func(*decision.Decision, decision.Intent) decision.Intent) (PublicGame, History) {
	t.Helper()
	setup := PublicGame{Names: []string{"a", "b"}, Decks: decks}
	tape := []rules.ChanceDraw{{Bound: 2, Value: toss}}
	for _, deck := range decks {
		for n := len(deck); n > 1; n-- {
			tape = append(tape, rules.ChanceDraw{Bound: n, Value: n - 1})
		}
	}
	e, err := rules.NewHypothetical(rules.Config{Seed: 1, Names: setup.Names, Decks: decks}, tape)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.AdvanceHypothetical(); err != nil {
		t.Fatal(err)
	}
	c := NewCollector(0)
	h := History{Actor: 0, Answers: make(map[int][]Action)}
	r := rand.New(rand.NewPCG(4, 8))
	pos := 0
	for i := 0; i < 500; i++ {
		frame, err := c.Capture(e, e.L.Events[pos:])
		if err != nil {
			t.Fatal(err)
		}
		h.Frames = append(h.Frames, frame)
		if stop(e) {
			return setup, h
		}
		d := e.Pending()
		if d == nil {
			t.Fatal("fixture ended before root")
		}
		in := botpolicy.Decide(botpolicy.BoardFromGame(e.G, e, d.Player), d, r)
		if d.Player == 0 {
			if d.Kind == decision.KArrange {
				in.Choices = nil
				for j := len(d.Options) - 1; j >= 0; j-- {
					in.Choices = append(in.Choices, j)
				}
			}
			for _, answer := range answers {
				in = answer(d, in)
			}
			h.Answers[i], err = c.Actions(d, in)
			if err != nil {
				t.Fatal(err)
			}
		}
		pos = len(e.L.Events)
		if err := e.SubmitHypothetical(in); err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("fixture never reached root")
	return setup, h
}

func replayWithOpponentGenesisOrder(t *testing.T, setup PublicGame, h History, names []string) (bool, Frame, Frame, World) {
	t.Helper()
	config := rules.Config{Seed: 1, Names: setup.Names, Decks: setup.Decks, Tokens: setup.Tokens, StartingLife: setup.StartingLife}
	tape, _, err := publicToss(setup, h)
	if err != nil {
		t.Fatal(err)
	}
	planner := func(ctx rules.ShuffleContext) ([]state.ObjID, error) {
		if ctx.Ordinal != 0 || ctx.Player != 1 {
			out := make([]state.ObjID, len(ctx.Library))
			for i, card := range ctx.Library {
				out[i] = card.ID
			}
			return out, nil
		}
		used := make([]bool, len(ctx.Library))
		out := make([]state.ObjID, 0, len(ctx.Library))
		for _, name := range names {
			found := -1
			for i, card := range ctx.Library {
				if !used[i] && card.Name == name {
					found = i
					break
				}
			}
			if found < 0 {
				return nil, errors.New("named genesis order is not a permutation")
			}
			used[found] = true
			out = append(out, ctx.Library[found].ID)
		}
		for i, card := range ctx.Library {
			if !used[i] {
				out = append(out, card.ID)
			}
		}
		return out, nil
	}
	e, err := rules.NewHypotheticalPlanned(config, tape, planner)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.AdvanceHypothetical(); err != nil {
		t.Fatal(err)
	}
	observer := NewCollector(h.Actor)
	bot := rand.New(rand.NewPCG(4, 8))
	pos := 0
	for i, want := range h.Frames {
		got, err := observer.Capture(e, e.L.Events[pos:])
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			return false, got, want, World{}
		}
		if i == len(h.Frames)-1 {
			e.ClearHypotheticalPlanner()
			return true, Frame{}, Frame{}, World{Config: config, Engine: e, Observer: observer}
		}
		d := e.Pending()
		if d == nil {
			t.Fatal("planned replay ended before the recorded prefix")
		}
		var in decision.Intent
		if d.Player == h.Actor {
			in, err = observer.Match(d, h.Answers[i])
			if err != nil {
				t.Fatal(err)
			}
		} else {
			in = botpolicy.Decide(botpolicy.BoardFromGame(e.G, e, d.Player), d, bot)
		}
		pos = len(e.L.Events)
		if err := e.SubmitHypothetical(in); err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("empty history")
	return false, Frame{}, Frame{}, World{}
}
