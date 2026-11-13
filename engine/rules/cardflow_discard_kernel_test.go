package rules

// Who chooses a discard, and that the answer is honoured,
// driven through the resolution kernel on a real engine.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// discardBoard: seat 0 is the caster (a sorcery source), seat 1 the
// target whose hand is exactly hand.
func discardBoard(t *testing.T, hand ...string) (*Engine, func() *effects.Ctx, []state.ObjID) {
	t.Helper()
	e := fixtureEngine(t, 2)
	src := placeSrc(t, e, 0, "Name:Thoughtseize\nTypes:Sorcery\nOracle:x\n", state.ZHand)
	ids := setHandSrcs(t, e, 1, hand...)
	mk := func() *effects.Ctx {
		return &effects.Ctx{Source: src, Controller: 0,
			Targets: []state.Target{{Player: 1, IsPlayer: true}}, TargetsOffered: true}
	}
	return e, mk, ids
}

// tgtChooseSA finds a corpus card's Mode$ TgtChoose Discard SA (walking
// printed abilities and SVars) -- the old realTgtChooseSA.
func tgtChooseSA(t *testing.T, name string) *cards.SA {
	t.Helper()
	c := corpusCardNamed(t, name)
	var walk func(sa *cards.SA, d int) *cards.SA
	walk = func(sa *cards.SA, d int) *cards.SA {
		if sa == nil || d > 32 {
			return nil
		}
		if sa.API == "Discard" && sa.Params["Mode"] == "TgtChoose" {
			return sa
		}
		return walk(sa.Sub, d+1)
	}
	for _, f := range c.Faces {
		for _, sa := range f.Abilities {
			if got := walk(sa, 0); got != nil {
				return got
			}
		}
	}
	t.Fatalf("card %q has no TgtChoose Discard ability", name)
	return nil
}

// TestDiscardRevealYouChooseAsksTheCasterNotTheTargetKernel: for Mode$
// RevealYouChoose the CASTER chooses (a KModes 1/1 over the target's hand),
// and the chosen card -- deliberately not hand[0] -- is the one discarded.
func TestDiscardRevealYouChooseAsksTheCasterNotTheTargetKernel(t *testing.T) {
	t.Parallel()
	e, mk, ids := discardBoard(t, vanillaCreatureSrc("Frog"), vanillaCreatureSrc("Bird"))
	d := runProbeSA(t, e, parseFixtureSA(t, "SP$ Discard | ValidTgts$ Player | Mode$ RevealYouChoose | DiscardValid$ Card | NumCards$ 1"), mk, nil)
	if d == nil {
		t.Fatal("RevealYouChoose posed no decision")
	}
	if d.Player != 0 || d.Kind != decision.KModes || d.Min != 1 || d.Max != 1 || len(d.Options) != 2 {
		t.Fatalf("decision = %+v, want the caster's 1/1 KModes over 2 cards", d)
	}
	answerAsk(t, e, optionIdxForObj(t, d, ids[1]))
	if !objectInZone(e, state.ZGraveyard, 1, ids[1]) || objectInZone(e, state.ZHand, 1, ids[1]) {
		t.Fatal("the chosen card (bird) was not discarded")
	}
	if !objectInZone(e, state.ZHand, 1, ids[0]) {
		t.Fatal("the un-chosen card (frog) left the hand -- the choice was ignored")
	}
}

// TestDiscardHandOptionalWithAskableHostPosesTheElectionKernel: Optional$
// True Mode$ Hand poses the may-discard election to the discarding target;
// nothing moves before it, and a decline discards nothing.
func TestDiscardHandOptionalWithAskableHostPosesTheElectionKernel(t *testing.T) {
	t.Parallel()
	e, mk, ids := discardBoard(t, vanillaCreatureSrc("Frog"), vanillaCreatureSrc("Bird"))
	d := runProbeSA(t, e, parseFixtureSA(t, "SP$ Discard | ValidTgts$ Player | Mode$ Hand | Optional$ True"), mk, nil)
	if d == nil || d.Player != 1 {
		t.Fatalf("election = %+v, want a decision to the discarding seat 1", d)
	}
	for i, id := range ids {
		if !objectInZone(e, state.ZHand, 1, id) {
			t.Fatalf("hand card %d left the hand before the election was answered", i)
		}
	}
	answerAsk(t, e, optionKindIndex(t, d, "no"))
	for i, id := range ids {
		if !objectInZone(e, state.ZHand, 1, id) {
			t.Fatalf("hand card %d was discarded despite a DECLINED election", i)
		}
	}
}

// TestDiscardLookYouChooseAsksTheCasterNotTheTargetKernel: LookYouChoose's
// decision goes to the caster; the answered (second) card moves.
func TestDiscardLookYouChooseAsksTheCasterNotTheTargetKernel(t *testing.T) {
	t.Parallel()
	e, mk, ids := discardBoard(t, vanillaCreatureSrc("Frog"), vanillaCreatureSrc("Bird"))
	d := runProbeSA(t, e, parseFixtureSA(t, "SP$ Discard | ValidTgts$ Player | Mode$ LookYouChoose | NumCards$ 1"), mk, nil)
	if d == nil || d.Player != 0 || d.ResumeKind != "discard" {
		t.Fatalf("decision = %+v, want the caster's discard ask", d)
	}
	answerAsk(t, e, optionIdxForObj(t, d, ids[1]))
	if !objectInZone(e, state.ZGraveyard, 1, ids[1]) {
		t.Fatal("the chosen card (bird) was not discarded")
	}
	if !objectInZone(e, state.ZHand, 1, ids[0]) {
		t.Fatal("the un-chosen card (frog) left the hand -- the choice was ignored")
	}
}

// TestDiscardYouChooseChooserIsTheCasterKernel: Mode$ YouChoose | Defined$
// Targeted -- the target's hand is the pool, the caster chooses.
func TestDiscardYouChooseChooserIsTheCasterKernel(t *testing.T) {
	t.Parallel()
	e, mk, ids := discardBoard(t, vanillaCreatureSrc("Frog"), vanillaCreatureSrc("Bird"))
	d := runProbeSA(t, e, parseFixtureSA(t, "SP$ Discard | Defined$ Targeted | Mode$ YouChoose | NumCards$ 1"), mk, nil)
	if d == nil || d.Player != 0 || d.Min != 1 || d.Max != 1 {
		t.Fatalf("decision = %+v, want the caster's 1/1 ask", d)
	}
	answerAsk(t, e, optionIdxForObj(t, d, ids[0]))
	if !objectInZone(e, state.ZGraveyard, 1, ids[0]) {
		t.Fatal("the chosen card was not discarded from the TARGET's hand")
	}
}

// TestDiscardRevealTgtChooseChooserIsTheTargetKernel: Rakdos Augermage's
// RevealTgtChoose -- the caster discards, the targeted opponent chooses,
// the pool is the caster's hand.
func TestDiscardRevealTgtChooseChooserIsTheTargetKernel(t *testing.T) {
	t.Parallel()
	e := fixtureEngine(t, 2)
	src := placeSrc(t, e, 0, "Name:Rakdos Augermage\nTypes:Creature\nPT:2/2\nOracle:x\n", state.ZBattlefield)
	ids := setHandSrcs(t, e, 0, vanillaCreatureSrc("Frog"), vanillaCreatureSrc("Bird"))
	d := runProbeSA(t, e, parseFixtureSA(t, "SP$ Discard | Defined$ You | ValidTgts$ Opponent | Mode$ RevealTgtChoose | NumCards$ 1"), func() *effects.Ctx {
		return &effects.Ctx{Source: src, Controller: 0,
			Targets: []state.Target{{Player: 1, IsPlayer: true}}, TargetsOffered: true}
	}, nil)
	if d == nil || d.Player != 1 || len(d.Options) != 2 {
		t.Fatalf("decision = %+v, want the TARGET seat 1 choosing over the caster's 2 cards", d)
	}
	answerAsk(t, e, optionIdxForObj(t, d, ids[1]))
	if !objectInZone(e, state.ZGraveyard, 0, ids[1]) {
		t.Fatal("the opponent's pick (bird) was not discarded from the CASTER's hand")
	}
	if !objectInZone(e, state.ZHand, 0, ids[0]) {
		t.Fatal("the un-chosen card (frog) left the caster's hand")
	}
}

// twoHandsBoard sets seat 0's and seat 1's hands and returns a Ctx maker for a
// plain source owned by seat 0.
func twoHandsBoard(t *testing.T, h0, h1 []string) (*Engine, func() *effects.Ctx, []state.ObjID, []state.ObjID) {
	t.Helper()
	e := fixtureEngine(t, 2)
	src := placeSrc(t, e, 0, "Name:Wheel\nTypes:Sorcery\nOracle:x\n", state.ZBattlefield)
	a := setHandSrcs(t, e, 0, h0...)
	b := setHandSrcs(t, e, 1, h1...)
	return e, func() *effects.Ctx { return &effects.Ctx{Source: src, Controller: 0} }, a, b
}

// TestDiscardHandOptionalAsksEachPlayerAndDeclineDiscardsNothingKernel: the
// whole-hand wheel's may-discard election asks each player in turn; a
// decline discards nothing, an acceptance that player's whole hand.
func TestDiscardHandOptionalAsksEachPlayerAndDeclineDiscardsNothingKernel(t *testing.T) {
	t.Parallel()
	e, mk, h0, h1 := twoHandsBoard(t, []string{vanillaCreatureSrc("Frog")}, []string{vanillaCreatureSrc("Bird")})
	frog, bird := h0[0], h1[0]
	d := runProbeSA(t, e, parseFixtureSA(t, "SP$ Discard | Mode$ Hand | Defined$ Player | Optional$ True | RememberDiscardingPlayers$ True"), mk, nil)
	if d == nil || d.Player != 0 || d.ResumeKind != "discard_hand" || len(d.Options) != 2 {
		t.Fatalf("first election = %+v, want seat 0's yes/no discard_hand", d)
	}
	d = answerAsk(t, e, optionKindIndex(t, d, "no"))
	if d == nil || d.Player != 1 || d.ResumeKind != "discard_hand" {
		t.Fatalf("second election = %+v, want seat 1's own election", d)
	}
	if !objectInZone(e, state.ZHand, 0, frog) {
		t.Fatal("a DECLINED election still discarded the hand")
	}
	answerAsk(t, e, optionKindIndex(t, d, "yes"))
	if !objectInZone(e, state.ZHand, 0, frog) {
		t.Fatal("the declining player's card left the hand")
	}
	if !objectInZone(e, state.ZGraveyard, 1, bird) {
		t.Fatal("the accepting player's card was not discarded")
	}
}

// TestDiscardTgtChooseMultiTargetAsksEveryTargetKernel: a TgtChoose discard
// over `Defined$ You & Opponent` asks each acting player over their own
// hand, in order, and applies each answer to the player who gave it.
func TestDiscardTgtChooseMultiTargetAsksEveryTargetKernel(t *testing.T) {
	t.Parallel()
	e, mk, h0, h1 := twoHandsBoard(t,
		[]string{vanillaCreatureSrc("Frog"), vanillaCreatureSrc("Cat")},
		[]string{vanillaCreatureSrc("Bird"), vanillaCreatureSrc("Dog")})
	frog, cat, bird, dog := h0[0], h0[1], h1[0], h1[1]
	d := runProbeSA(t, e, parseFixtureSA(t, "SP$ Discard | Defined$ You & Opponent | Mode$ TgtChoose | NumCards$ 1"), mk, nil)
	if d == nil || d.Player != 0 {
		t.Fatalf("first ask = %+v, want seat 0", d)
	}
	d = answerAsk(t, e, optionIdxForObj(t, d, cat))
	if d == nil || d.Player != 1 {
		t.Fatalf("second ask = %+v, want seat 1 -- target 1 was never asked", d)
	}
	if !objectInZone(e, state.ZHand, 0, cat) {
		t.Fatal("target 0's card left the hand before target 1 chose")
	}
	answerAsk(t, e, optionIdxForObj(t, d, dog))
	if !objectInZone(e, state.ZGraveyard, 0, cat) {
		t.Fatal("target 0's answer was not applied")
	}
	if !objectInZone(e, state.ZGraveyard, 1, dog) {
		t.Fatal("target 1's answer was not applied to target 1's hand")
	}
	if !objectInZone(e, state.ZHand, 0, frog) || !objectInZone(e, state.ZHand, 1, bird) {
		t.Fatal("a card nobody chose left a hand")
	}
	if len(e.G.Zone(state.ZHand, 0)) != 1 || len(e.G.Zone(state.ZHand, 1)) != 1 {
		t.Fatalf("hand sizes = %d/%d, want 1/1", len(e.G.Zone(state.ZHand, 0)), len(e.G.Zone(state.ZHand, 1)))
	}
}

// TestDiscardChooseModeMultiTargetCursorDoesNotStallKernel: LookYouChoose
// over two acting players -- the caster chooses for each hand; the second
// ask is about the second hand (never re-offering the first), and both
// answers apply.
func TestDiscardChooseModeMultiTargetCursorDoesNotStallKernel(t *testing.T) {
	t.Parallel()
	e, mk, h0, h1 := twoHandsBoard(t, []string{vanillaCreatureSrc("Frog")}, []string{vanillaCreatureSrc("Bird")})
	frog, bird := h0[0], h1[0]
	d := runProbeSA(t, e, parseFixtureSA(t, "SP$ Discard | Defined$ You & Opponent | Mode$ LookYouChoose | NumCards$ 1"), mk, nil)
	if d == nil || d.Player != 0 {
		t.Fatalf("first ask = %+v, want seat 0", d)
	}
	d = answerAsk(t, e, optionIdxForObj(t, d, frog))
	if d == nil || d.Player != 0 || d.ResumeTarget != 1 {
		t.Fatalf("second ask = %+v, want the caster's ask for target 1", d)
	}
	for _, o := range d.Options {
		if o.Obj == frog {
			t.Fatal("target 1 was offered target 0's hand -- the cursor did not move")
		}
	}
	answerAsk(t, e, optionIdxForObj(t, d, bird))
	if !objectInZone(e, state.ZGraveyard, 0, frog) {
		t.Fatal("target 0's answer was not applied")
	}
	if !objectInZone(e, state.ZGraveyard, 1, bird) {
		t.Fatal("target 1's answer was not applied")
	}
}

// TestDiscardTgtChooseAsksTheDiscardingPlayerKernel: Mind Rot's real
// TgtChoose shape -- the DISCARDING player chooses (KModes 2/2 over three
// cards) and the chosen cards, not the front of the hand, leave it.
func TestDiscardTgtChooseAsksTheDiscardingPlayerKernel(t *testing.T) {
	t.Parallel()
	s := tgtChooseSA(t, "Mind Rot")
	e, mk, ids := discardBoard(t, vanillaCreatureSrc("Frog"), vanillaCreatureSrc("Bird"), vanillaCreatureSrc("Cat"))
	d := runProbeSA(t, e, s, mk, nil)
	if d == nil || d.Player != 1 || d.Kind != decision.KModes || d.Min != 2 || d.Max != 2 || len(d.Options) != 3 {
		t.Fatalf("decision = %+v, want seat 1's 2/2 KModes over 3 cards", d)
	}
	answerAsk(t, e, optionIdxForObj(t, d, ids[1]), optionIdxForObj(t, d, ids[2]))
	for _, id := range ids[1:] {
		if !objectInZone(e, state.ZGraveyard, 1, id) {
			t.Fatalf("chosen card %d was not discarded", id)
		}
	}
	if !objectInZone(e, state.ZHand, 1, ids[0]) {
		t.Fatal("the un-chosen front card (frog) left the hand")
	}
}

// TestDiscardTgtChooseSubAbilityFiresExactlyOnceKernel: Davriel's
// Shadowfugue (TgtChoose discard, SubAbility$ DBLoseLife 2): the chained
// life loss does not run before the choice and runs exactly once after.
func TestDiscardTgtChooseSubAbilityFiresExactlyOnceKernel(t *testing.T) {
	t.Parallel()
	s := tgtChooseSA(t, "Davriel's Shadowfugue")
	e, mk, ids := discardBoard(t, vanillaCreatureSrc("Frog"), vanillaCreatureSrc("Bird"), vanillaCreatureSrc("Cat"))
	life := e.G.Players[1].Life
	start := len(e.L.Events)
	d := runProbeSA(t, e, s, mk, nil)
	if d == nil {
		t.Fatal("TgtChoose posed no decision")
	}
	if e.G.Players[1].Life != life {
		t.Fatal("the chained SubAbility ran before the discard choice was made")
	}
	answerAsk(t, e, optionIdxForObj(t, d, ids[1]), optionIdxForObj(t, d, ids[2]))
	losses := 0
	for _, ev := range eventsSince(e, start) {
		if ev.Kind == events.LifeChange && ev.Player == 1 && ev.Amount < 0 {
			losses++
		}
	}
	if losses != 1 || e.G.Players[1].Life != life-2 {
		t.Fatalf("life-loss events = %d, life = %d; want exactly one loss of 2 (from %d)", losses, e.G.Players[1].Life, life)
	}
	for _, id := range ids[1:] {
		if !objectInZone(e, state.ZGraveyard, 1, id) {
			t.Fatalf("chosen card %d not discarded", id)
		}
	}
	if !objectInZone(e, state.ZHand, 1, ids[0]) {
		t.Fatal("un-chosen front card (frog) left the hand")
	}
}
