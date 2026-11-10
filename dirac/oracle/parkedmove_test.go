package oracle

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// newSynthPlayerOf deals the synthetic pool name once and returns a function that
// plays one of its pairs in lockstep, with the pair's matchup and seed, and the error
// of a pair the set refuses. Only recordPins plays pairs by number: a test pins a
// recorded game (pinnedGames), never a pair number, which a re-deal moves.
func newSynthPlayerOf(t *testing.T, name string, rc RunConfig, mulligans int) func(pair int) (*Matchup, uint64, Result, error) {
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
	return func(i int) (*Matchup, uint64, Result, error) {
		p := PairingAt(pool, i, 1)
		m, tbl, err := SetMatchup(set, c, p.A, p.B, mulligans)
		if err != nil {
			return nil, 0, Result{}, err
		}
		k, err := SetSide(set, tbl, cfg)(m, p.Seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), rc)
		CloseSide(k)
		return m, p.Seed, res, nil
	}
}

// parkedPairs plays recorded games with two Rests in Peace out and a replacement
// order ask posed, and fails any in which a card reaches a graveyard under both.
func parkedPairs(t *testing.T, also ...func(ref *GorgeSide) bool) {
	t.Helper()
	rc := RunConfig{Config: Config{GorgeAutopaySurface: true}, Check: func(ref *GorgeSide, _ Side) *Divergence {
		if msg, ok := gravesAfterAnswers(ref); ok && msg != "" {
			return &Divergence{What: "parked move", Field: "graveyard", Detail: msg}
		}
		return nil
	}}
	settled := func(ref *GorgeSide) bool { _, ok := gravesAfterAnswers(ref); return ok }
	pinnedGamesRun(t, "std-big", rc, 1, append([]func(ref *GorgeSide) bool{settled, replacementAsk}, also...)...)
}

// replacementAsk is the condition that a replacement order ask is pending.
func replacementAsk(ref *GorgeSide) bool {
	if d := ref.Pending(); d != nil {
		for _, o := range d.Options {
			if o.Kind == "replacement" {
				return true
			}
		}
	}
	return false
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
	parkedPairs(t)
}

// A casualty's Equipment stays attached to it until the state-based pass ends, and
// the next casualty's order ask is still that pass (CR 704.3): Lost Jitte's bearer is
// in exile with the Jitte still on it when the ask comes.
func TestParkedPassKeepsAttachments(t *testing.T) {
	t.Parallel()
	parkedPairs(t, cardOut("Lost Jitte"))
}

// A move made while a resolution runs waits too, and the resolution with it: a
// spell going to a graveyard stays on the stack through the ask, and so does the
// ability whose effect sacrificed or discarded.
func TestParkedMoveResolve(t *testing.T) {
	t.Parallel()
	parkedPairs(t)
}

// A copy's exceptions are part of it as it enters (CR 707.9b): Nexus of
// Becoming's 3/3 Golem artifact creature token is a creature entering under
// Torpor Orb (CR 603.2g), so its entry triggers nothing.
func TestCopyExceptionsReadByEntryTriggers(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "std-big", Config{GorgeAutopaySurface: true}, cardOut("Nexus of Becoming"), onBattlefield("Torpor Orb"))
}

// A copy of an Equipment that its exception makes a creature can't stay attached
// to anything (CR 704.5p): Nexus of Becoming's copy of a Sword is activated, and
// the state-based pass takes it off again.
func TestCopiedEquipmentCreatureUnattaches(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "std-big", Config{GorgeAutopaySurface: true}, cardOut("Nexus of Becoming"))
}

// Gorge's planner prices no spell with a tap-permanents additional cost, so the
// autopay surface offers Guardian of the Great Door only when the pool pays it.
func TestTapAdditionalCostOutsideThePlanner(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "std-lci", Config{GorgeAutopaySurface: true}, cardSeen("Guardian of the Great Door"))
}

// Krovod Haunch's "you may pay {1}{W}" is one question: the decider is the
// controller of the card triggering on itself, the one who pays, so gorge folds
// the yes/no into the pay/decline.
func TestSelfTriggerDeciderFoldsIntoCost(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "std-mkm", Config{GorgeAutopaySurface: true}, cardOut("Krovod Haunch"))
}

// A Treasure sacrificed while a cost is tapped for poses the order ask with the
// earlier mana floating and its own not yet made (CR 605.3b).
func TestManaSacrificeAsksMidPayment(t *testing.T) {
	parkedPairs(t)
}

// A discard to hand size under two exile-instead replacements poses the order ask,
// and the last answer finishes the cleanup step.
func TestCleanupDiscardAsksReplacementOrder(t *testing.T) {
	parkedPairs(t)
}

// Several discards take all their picks before the first order ask.
func TestCleanupDiscardPicksBeforeOrderAsks(t *testing.T) {
	parkedPairs(t)
}

// A gift trigger is the keyword's, not one of the card's printed triggers: the two
// engines name it alike on the stack (CR 702.174c; Starforged Sword, Scrapshooter).
func TestGiftTriggerOnTheStack(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "std-blb", Config{}, anyOut("Starforged Sword", "Scrapshooter"))
}

// A cast with an either-or additional cost is lowered only on a witness from the
// planner's census, which has no source with a fee: gorge doesn't offer Deadly
// Precision or Redirect Lightning when the colour comes only through Barrels of
// Blasting Jelly's {1}.
func TestEitherOrLoweredOnAWitness(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "std-tla", Config{}, anyOf(cardSeen("Deadly Precision"), cardSeen("Redirect Lightning")), onBattlefield("Barrels of Blasting Jelly"))
	pinnedGames(t, "std-lci", Config{}, anyOf(cardSeen("Bitter Triumph"), cardSeen("Daring Buccaneer"),
		cardSeen("Dusk Rose Reliquary"), cardSeen("Soaring Stoneglider"), cardSeen("Thunderherd Migration")))
}

// Two Rests in Peace: a Treasure in a resolution's mana payment waits for its order
// ask, the plan's next source waits for that, and a mass destroy and a mill ask once a
// card, each going where its answer says.
func TestParkedMassAndPayment(t *testing.T) {
	settled := func(ref *GorgeSide) bool { _, ok := gravesAfterAnswers(ref); return ok }
	pinnedGamesRun(t, "std-big", RunConfig{Config: Config{GorgeAutopaySurface: true}}, 1, settled)
}

// An Aura whose host dies goes in the next state-based pass, behind that pass's
// casualties and a token ceasing: Bard the Bowman, who loses Bard's Company's +1/+1
// with 3 damage marked, ahead of Enchanted River's Grasp (std-hob 161), and an Aura
// whose host dies ahead of one that was already unattached (std-dft).
func TestAuraLeavesInThePassAfterItsHostLockstep(t *testing.T) {
	pinnedGames(t, "std-hob", Config{GorgeAutopaySurface: true}, onBattlefield("Bard the Bowman"), cardOut("Enchanted River's Grasp"))
	pinnedGames(t, "std-dft", Config{}, auraLeftBattlefield)
}

// An enchantment animated with a granted set-P/T static (Beorn's Hospitality) that then
// loses all abilities to Enchanted River's Grasp is a 0/0 Bear: the later layer-6 loss
// removes the granted static, so it dies and the Aura goes with it (CR 613.1f, 613.7,
// 704.5f; std-hob 66, 276).
func TestLoseAllAbilitiesEndsAnAnimateGrantedStatic(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "std-hob", Config{GorgeAutopaySurface: true}, onBattlefield("Beorn's Hospitality"), cardOut("Enchanted River's Grasp"))
}

// A reduction for targeting your own permanent reads every announced target, a second
// one too, and none when an "up to" takes none (std-otj 18, 71: This Town Ain't Big
// Enough).
func TestReductionReadsTheAnnouncedTargets(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "std-otj", Config{}, cardSeen("This Town Ain't Big Enough"))
}

// A Gift spell is lowered only on the witness of its composed cost, which has no source
// with a fee: gorge doesn't offer Cruelclaw's Heist or Dawn's Truce through Hidden
// Grotto's {1}.
func TestGiftLoweredOnAWitness(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "std-blb", Config{}, anyOf(cardSeen("Cruelclaw's Heist"), cardSeen("Dawn's Truce")), onBattlefield("Hidden Grotto"))
}

// Copying each nontoken creature is one batch of tokens entering: Mister Fantastic's
// "one or more tokens" triggers once (Multiversal Incursion).
func TestCopyEachEntersAsOneBatch(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "std-msh", Config{}, cardOut("Multiversal Incursion"))
}

// A behold cast is lowered on the composed plan's witness too: gorge doesn't offer
// Champion of the Path when only Springleaf Drum's fee reaches its cost (std-ecl 53).
func TestBeholdLoweredOnAWitness(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "std-ecl", Config{}, onBattlefield("Springleaf Drum"), cardSeen("Champion of the Path"))
}

// A return during a resolution asks the card's as-enters choice with the card still in
// its zone, and it enters once answered (CR 614.12): Tune Up on Lifecraft Engine.
func TestReturnAsksItsChoiceBeforeItMoves(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "std-dft", Config{}, cardOut("Tune Up"), onBattlefield("Lifecraft Engine"))
}

// A card's own leaves-the-battlefield trigger reads the counters it left with (CR
// 608.2h): Selfless Police Captain puts its +1/+1 counters on the target.
func TestLeavesTriggerReadsItsOwnCounters(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "std-spm", Config{}, cardOut("Selfless Police Captain"))
}
