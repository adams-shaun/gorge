package oracle

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// synthPairs plays pairs of a synthetic pool in lockstep on the autopay surface
// and checks each runs clean.
func synthPairs(t *testing.T, name string, pairs []int, check func(pair int, res *Result)) {
	t.Helper()
	synthPairsOn(t, name, Config{GorgeAutopaySurface: true}, pairs, check)
}

// synthPairsOn is synthPairs on the surface cfg names.
func synthPairsOn(t *testing.T, name string, cfg Config, pairs []int, check func(pair int, res *Result)) {
	t.Helper()
	synthPairsRun(t, name, RunConfig{Config: cfg}, pairs, check)
}

// synthPairsRun is synthPairsOn with rc's Check at every decision.
func synthPairsRun(t *testing.T, name string, rc RunConfig, pairs []int, check func(pair int, res *Result)) {
	t.Helper()
	synthPairsPlay(t, name, rc, pairs, func(i int, res *Result) {
		if res.Div != nil {
			t.Errorf("pair %d diverged at %d: %s %s %s %v %v", i, res.Div.At, res.Div.What, res.Div.Field, res.Div.Detail, res.Div.RefOptions, res.Div.KerOptions)
		}
		if res.Stall != "" {
			t.Errorf("pair %d stalled: %s", i, res.Stall)
		}
		if check != nil {
			check(i, res)
		}
	})
}

// synthPairsPlay plays each pair in lockstep and hands the result to each, judging nothing.
func synthPairsPlay(t *testing.T, name string, rc RunConfig, pairs []int, each func(pair int, res *Result)) {
	t.Helper()
	cfg := rc.Config
	rc.MaxDecisions, rc.MaxTurns = 20000, 100
	c := &Corpus{Reg: synthReg(t), decks: map[string][]*cards.Card{}}
	set := cardsettest.Synth(t, name)
	pool, files, err := SynthPool(c.Reg, set, 1, SynthDecks)
	if err != nil {
		t.Fatal(err)
	}
	c.AddDecks(files)
	for _, i := range pairs {
		p := PairingAt(pool, i, 1)
		m, tbl, err := SetMatchup(set, c, p.A, p.B, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, p.Seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), rc)
		CloseSide(k)
		each(i, &res)
	}
}

// parkedPairs is synthPairs requiring a replacement order ask in each game.
func parkedPairs(t *testing.T, pairs []int) {
	t.Helper()
	bad, read := "", false
	rc := RunConfig{Config: Config{GorgeAutopaySurface: true}, Check: func(ref *GorgeSide, _ Side) *Divergence {
		if msg, ok := gravesAfterAnswers(ref); ok {
			read = true
			if bad == "" {
				bad = msg
			}
		}
		return nil
	}}
	synthPairsRun(t, "std-big", rc, pairs, func(pair int, res *Result) {
		if bad != "" {
			t.Errorf("pair %d: %s", pair, bad)
		}
		bad = ""
		for _, a := range res.Actions {
			if a.Kind() == ActReplacement {
				return
			}
		}
		t.Errorf("pair %d no longer poses a replacement order ask", pair)
	})
	if !read {
		t.Error("no game reached a settled board under two Rests in Peace")
	}
}

// gravesAfterAnswers reads the board at a priority with the stack empty and two
// Rests in Peace out: the first's enter trigger emptied the graveyards, so whichever
// way each order ask was answered, a card there means a parked move went wrong. ok
// says the board was such a one.
func gravesAfterAnswers(ref *GorgeSide) (msg string, ok bool) {
	g := ref.E.G
	if len(g.Stack) != 0 || ref.E.Pending() == nil || ref.E.Pending().Kind != decision.KPriority {
		return "", false
	}
	rests := 0
	for p := range g.Players {
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
			if f := g.Obj(id).Face(); f != nil && f.Name == "Rest in Peace" {
				rests++
			}
		}
	}
	if rests < 2 {
		return "", false
	}
	for p := range g.Players {
		for _, id := range g.Zone(state.ZGraveyard, state.PlayerID(p)) {
			return fmt.Sprintf("turn %d: %s reached a graveyard under two Rests in Peace", g.Turn, g.Obj(id).Face().Name), true
		}
	}
	return "", true
}

// Two Rests in Peace compete for every move to a graveyard. A move paid as a
// cost, or a state-based death, waits on the CR 616.1 ask with the object where
// it was: the discarded or sacrificed card sits in its zone with the ability
// already on the stack.
func TestParkedMoveCostAndSBA(t *testing.T) {
	t.Parallel()
	parkedPairs(t, []int{121, 178, 338, 339, 342, 349, 353, 383, 402, 463})
}

// A casualty's Equipment stays attached to it until the state-based pass ends, and
// the next casualty's order ask is still that pass (CR 704.3): Lost Jitte's bearer is
// in exile with the Jitte still on it when the ask comes.
func TestParkedPassKeepsAttachments(t *testing.T) {
	t.Parallel()
	parkedPairs(t, []int{31, 160, 185, 257, 281})
}

// A move made while a resolution runs waits too, and the resolution with it: a
// spell going to a graveyard stays on the stack through the ask, and so does the
// ability whose effect sacrificed or discarded.
func TestParkedMoveResolve(t *testing.T) {
	t.Parallel()
	parkedPairs(t, []int{30, 49, 52, 170, 177, 273, 275, 290, 310, 315, 329, 344, 405, 441, 495})
}

// A copy's exceptions are part of it as it enters (CR 707.9b): Nexus of
// Becoming's 3/3 Golem artifact creature token is a creature entering under
// Torpor Orb (CR 603.2g), so its entry triggers nothing.
func TestCopyExceptionsReadByEntryTriggers(t *testing.T) {
	t.Parallel()
	synthPairs(t, "std-big", []int{7, 10, 25, 29, 34, 40, 41, 50, 58, 59}, nil)
}

// A copy of an Equipment that its exception makes a creature can't stay attached
// to anything (CR 704.5p): Nexus of Becoming's copy of a Sword is activated, and
// the state-based pass takes it off again.
func TestCopiedEquipmentCreatureUnattaches(t *testing.T) {
	t.Parallel()
	synthPairs(t, "std-big", []int{2, 20, 78, 141, 204}, nil)
}

// Gorge's planner prices no spell with a tap-permanents additional cost, so the
// autopay surface offers Guardian of the Great Door only when the pool pays it.
func TestTapAdditionalCostOutsideThePlanner(t *testing.T) {
	t.Parallel()
	synthPairs(t, "std-lci", []int{31, 42, 62, 69, 98, 114}, nil)
}

// Krovod Haunch's "you may pay {1}{W}" is one question: the decider is the
// controller of the card triggering on itself, the one who pays, so gorge folds
// the yes/no into the pay/decline.
func TestSelfTriggerDeciderFoldsIntoCost(t *testing.T) {
	t.Parallel()
	synthPairs(t, "std-mkm", []int{2, 29, 35, 103, 116}, nil)
}

// A Treasure sacrificed while a cost is tapped for poses the order ask with the
// earlier mana floating and its own not yet made (CR 605.3b).
func TestManaSacrificeAsksMidPayment(t *testing.T) {
	parkedPairs(t, []int{114, 190, 244, 328, 429})
}

// A discard to hand size under two exile-instead replacements poses the order ask,
// and the last answer finishes the cleanup step.
func TestCleanupDiscardAsksReplacementOrder(t *testing.T) {
	parkedPairs(t, []int{36, 175})
}

// Several discards take all their picks before the first order ask.
func TestCleanupDiscardPicksBeforeOrderAsks(t *testing.T) {
	parkedPairs(t, []int{341, 666})
}

// A gift trigger is the keyword's, not one of the card's printed triggers: the two
// engines name it alike on the stack (CR 702.174c; Starforged Sword, Scrapshooter).
func TestGiftTriggerOnTheStack(t *testing.T) {
	t.Parallel()
	synthPairsOn(t, "std-blb", Config{}, []int{12, 15, 24}, nil)
}

// A cast with an either-or additional cost is lowered only on a witness from the
// planner's census, which has no source with a fee: gorge doesn't offer Deadly
// Precision or Redirect Lightning when the colour comes only through Barrels of
// Blasting Jelly's {1}.
func TestEitherOrLoweredOnAWitness(t *testing.T) {
	t.Parallel()
	synthPairsOn(t, "std-tla", Config{}, []int{4, 96, 144, 286}, nil)
	synthPairsOn(t, "std-lci", Config{}, []int{40, 425}, nil)
}

// Two Rests in Peace: a Treasure in a resolution's mana payment waits for its order
// ask, the plan's next source waits for that, and a mass destroy and a mill ask once a
// card, each going where its answer says.
func TestParkedMassAndPayment(t *testing.T) {
	synthPairs(t, "std-big", []int{35, 306, 370, 375, 859, 869, 1544, 1938, 2207, 2238, 2287}, func(pair int, res *Result) {
		if res.Stall != "" {
			t.Errorf("pair %d stalled: %s", pair, res.Stall)
		}
	})
}

// An Aura whose host dies goes in the next state-based pass, behind that pass's
// casualties and a token ceasing: Bard the Bowman, who loses Bard's Company's +1/+1
// with 3 damage marked, ahead of Enchanted River's Grasp (std-hob 161), and an Aura
// whose host dies ahead of one that was already unattached (std-dft 469).
func TestAuraLeavesInThePassAfterItsHostLockstep(t *testing.T) {
	synthPairs(t, "std-hob", []int{161}, nil)
	synthPairsOn(t, "std-dft", Config{}, []int{469}, nil)
}

// A reduction for targeting your own permanent reads every announced target, a second
// one too, and none when an "up to" takes none (std-otj 18, 71: This Town Ain't Big
// Enough).
func TestReductionReadsTheAnnouncedTargets(t *testing.T) {
	t.Parallel()
	synthPairsOn(t, "std-otj", Config{}, []int{18, 31, 71, 129}, nil)
}

// A Gift spell is lowered only on the witness of its composed cost, which has no source
// with a fee: gorge doesn't offer Cruelclaw's Heist or Dawn's Truce through Hidden
// Grotto's {1}.
func TestGiftLoweredOnAWitness(t *testing.T) {
	t.Parallel()
	synthPairsOn(t, "std-blb", Config{}, []int{96, 109, 122, 299}, nil)
}

// Copying each nontoken creature is one batch of tokens entering: Mister Fantastic's
// "one or more tokens" triggers once (std-msh 70, Multiversal Incursion).
func TestCopyEachEntersAsOneBatch(t *testing.T) {
	t.Parallel()
	synthPairsOn(t, "std-msh", Config{}, []int{70, 305, 388, 415}, nil)
}

// A behold cast is lowered on the composed plan's witness too: gorge doesn't offer
// Champion of the Path when only Springleaf Drum's fee reaches its cost (std-ecl 53).
func TestBeholdLoweredOnAWitness(t *testing.T) {
	t.Parallel()
	synthPairsOn(t, "std-ecl", Config{}, []int{53, 238}, nil)
}

// A return during a resolution asks the card's as-enters choice with the card still in
// its zone, and it enters once answered (CR 614.12): Tune Up on Lifecraft Engine.
func TestReturnAsksItsChoiceBeforeItMoves(t *testing.T) {
	t.Parallel()
	synthPairsOn(t, "std-dft", Config{}, []int{227, 349, 381}, nil)
}

// A card's own leaves-the-battlefield trigger reads the counters it left with (CR
// 608.2h): Selfless Police Captain puts its +1/+1 counters on the target.
func TestLeavesTriggerReadsItsOwnCounters(t *testing.T) {
	t.Parallel()
	synthPairsOn(t, "std-spm", Config{}, []int{20, 99, 219, 358}, nil)
}
