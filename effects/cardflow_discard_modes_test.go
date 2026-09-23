package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The Mode$ families the "Discard takes the front card" approximation row
// named: Random (CR 701.8b's random discard), the choosing modes the row
// grouped with it (LookYouChoose / YouChoose / RevealTgtChoose), the Mode$
// Hand | Optional$ True may-discard election, and the multi-target walk's
// per-target cursor. The helpers come from cardflow_discard_test.go
// (discardBoard, creature, land, inZone) and primitives_test.go (mkCard, sa,
// askHost): a 2-seat board whose seat 0 is the caster and seat 1 the target.

// discardRngHost is an askHost whose Rand replays a scripted sequence of raw
// [0,n) draws, so a Random discard's picks are known. Exhausted scripts
// degrade to 0 (the same convention dice_test's scriptedRollHost uses).
type discardRngHost struct {
	askHost
	seq []int
	i   int
}

func (h *discardRngHost) Rand(n int) int {
	if h.i < len(h.seq) {
		v := h.seq[h.i]
		h.i++
		return v % n
	}
	return 0
}

// standinNotes counts the R-9 "no engine host to ask" stand-in Notes in a
// host's log.
func standinNotes(log []events.Event) int {
	n := 0
	for _, ev := range log {
		if ev.Kind == events.Note && ev.Text == "discards its first card (no engine host to ask)" {
			n++
		}
	}
	return n
}

// TestDiscardRandomUsesTheEngineRng pins CR 701.8b: a Mode$ Random discard
// does NOT take the front card — the engine's own seeded RNG picks
// NumCards$ cards out of the DiscardValid$-filtered hand. The script [1, 1]
// draws bird (index 1 of [frog, bird, cat]) and then cat (index 1 of the
// remaining [frog, cat]): two DISTINCT cards, without replacement, and the
// front card (frog) and the land stay. No seat is asked and no stand-in Note
// is recorded — the randomness is the rule, not a missing ask.
func TestDiscardRandomUsesTheEngineRng(t *testing.T) {
	ah, c, ids := discardBoard(t,
		creature(t, "Frog"), land(t, "Islet"), creature(t, "Bird"), creature(t, "Cat"))
	rng := &discardRngHost{askHost: askHost{fakeHost: fakeHost{g: ah.g, log: ah.log}}, seq: []int{1, 1}}
	s := sa(t, "SP$ Discard | ValidTgts$ Player | Mode$ Random | NumCards$ 2 | DiscardValid$ Card.nonLand")

	if rng.g.Zone(state.ZHand, 1)[0] != ids[0] {
		t.Fatal("precondition: the hand's front card is not frog — the no-replacement assertion below would not discriminate")
	}
	effDiscard(rng, c, s)

	if rng.asked != nil {
		t.Fatal("a Random discard posed a decision — randomness is not a choice")
	}
	if !inZone(rng.g, state.ZGraveyard, 1, ids[2]) {
		t.Fatal("the RNG's first pick (bird, index 1) was not discarded — the discard took the front card instead")
	}
	if !inZone(rng.g, state.ZGraveyard, 1, ids[3]) {
		t.Fatal("the RNG's second pick (cat, index 1 of the remaining pool) was not discarded — the draw replaced")
	}
	if inZone(rng.g, state.ZGraveyard, 1, ids[0]) {
		t.Fatal("the front card (frog) was discarded — Random still takes hand[0]")
	}
	if !inZone(rng.g, state.ZHand, 1, ids[0]) || !inZone(rng.g, state.ZHand, 1, ids[1]) {
		t.Fatal("a card the RNG never picked (frog or the land) left the hand")
	}
	if standinNotes(rng.log) != 0 {
		t.Fatal("the Random arm recorded the R-9 stand-in Note — randomness is the rule, not a degradation")
	}
}

// TestDiscardLookYouChooseAsksTheCasterNotTheTarget pins the LookYouChoose
// chooser: "Target player reveals their hand. You choose X cards from it."
// The decision goes to the CASTER (seat 0), never the player whose hand it
// is, and the answered card (deliberately the second) is the one that moves.
func TestDiscardLookYouChooseAsksTheCasterNotTheTarget(t *testing.T) {
	ah, ctx, ids := discardBoard(t, creature(t, "Frog"), creature(t, "Bird"))
	s := sa(t, "SP$ Discard | ValidTgts$ Player | Mode$ LookYouChoose | NumCards$ 1")

	effDiscard(ah, ctx, s)
	if ah.asked == nil {
		t.Fatal("LookYouChoose posed no decision")
	}
	if ah.asked.Player != 0 {
		t.Fatalf("chooser = seat %d, want the caster seat 0 (the target must not pick)", ah.asked.Player)
	}
	if ah.asked.ResumeKind != "discard" || ah.asked.ResumeTarget != 0 {
		t.Fatalf("resume routing = %q/%d, want \"discard\"/0", ah.asked.ResumeKind, ah.asked.ResumeTarget)
	}

	// Simulate the engine's resume: the continuation set Ctx.Discard to the
	// chosen card and Ctx.DiscardTarget to the asking target's index.
	ctx.Discard = []state.ObjID{ids[1]}
	ctx.DiscardTarget = 0
	effDiscard(ah, ctx, s)

	if !inZone(ah.g, state.ZGraveyard, 1, ids[1]) {
		t.Fatal("the chosen card (bird) was not discarded")
	}
	if !inZone(ah.g, state.ZHand, 1, ids[0]) {
		t.Fatal("the un-chosen card (frog) left the hand — the choice was ignored")
	}
}

// TestDiscardYouChooseChooserIsTheCaster pins YouChoose's split (the corpus's
// "Mode$ YouChoose | Defined$ Targeted" shape): the TARGET's hand is the
// pool, the CASTER is the chooser.
func TestDiscardYouChooseChooserIsTheCaster(t *testing.T) {
	ah, ctx, ids := discardBoard(t, creature(t, "Frog"), creature(t, "Bird"))
	s := sa(t, "SP$ Discard | Defined$ Targeted | Mode$ YouChoose | NumCards$ 1")

	effDiscard(ah, ctx, s)
	if ah.asked == nil {
		t.Fatal("YouChoose posed no decision")
	}
	if ah.asked.Player != 0 {
		t.Fatalf("chooser = seat %d, want the caster seat 0", ah.asked.Player)
	}
	if ah.asked.Min != 1 || ah.asked.Max != 1 {
		t.Fatalf("Min/Max = %d/%d, want 1/1", ah.asked.Min, ah.asked.Max)
	}

	ctx.Discard = []state.ObjID{ids[0]}
	ctx.DiscardTarget = 0
	effDiscard(ah, ctx, s)
	if !inZone(ah.g, state.ZGraveyard, 1, ids[0]) {
		t.Fatal("the chosen card was not discarded from the TARGET's hand")
	}
}

// TestDiscardRevealTgtChooseChooserIsTheTarget pins Rakdos Augermage's
// RevealTgtChoose shape: "Reveal your hand and discard a card of target
// opponent's choice" — the DISCARDER is the caster (Defined$ You), the
// CHOOSER is the first player target (seat 1), and the pool is seat 0's hand.
func TestDiscardRevealTgtChooseChooserIsTheTarget(t *testing.T) {
	ah := &askHost{}
	ah.g = state.NewGame(names(2))
	src := ah.g.AddObject(mkCard(t, "Name:Rakdos Augermage\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0)
	frog := ah.g.AddObject(creature(t, "Frog"), 0)
	bird := ah.g.AddObject(creature(t, "Bird"), 0)
	ah.g.SetZone(state.ZHand, 0, []state.ObjID{frog.ID, bird.ID})
	ctx := &Ctx{Source: src.ID, Controller: 0,
		Targets: []state.Target{{Player: 1, IsPlayer: true}}, TargetsOffered: true}
	s := sa(t, "SP$ Discard | Defined$ You | ValidTgts$ Opponent | Mode$ RevealTgtChoose | NumCards$ 1")

	effDiscard(ah, ctx, s)
	if ah.asked == nil {
		t.Fatal("RevealTgtChoose posed no decision")
	}
	if ah.asked.Player != 1 {
		t.Fatalf("chooser = seat %d, want the TARGET seat 1 (Rakdos Augermage's opponent picks)", ah.asked.Player)
	}
	if len(ah.asked.Options) != 2 {
		t.Fatalf("options = %d, want the 2 cards of the CASTER's hand", len(ah.asked.Options))
	}

	// Simulate the resume with the answered pick.
	ctx.Discard = []state.ObjID{bird.ID}
	ctx.DiscardTarget = 0
	effDiscard(ah, ctx, s)
	if !inZone(ah.g, state.ZGraveyard, 0, bird.ID) {
		t.Fatal("the opponent's pick (bird) was not discarded from the CASTER's hand")
	}
	if !inZone(ah.g, state.ZHand, 0, frog.ID) {
		t.Fatal("the un-chosen card (frog) left the caster's hand — the choice was ignored")
	}
}

// TestDiscardHandOptionalAsksEachPlayerAndDeclineDiscardsNothing pins the
// whole-hand wheel's may-discard election (the real corpus shape `SP$ Discard
// | Mode$ Hand | Defined$ Player | Optional$ True | RememberDiscardingPlayers$
// True`): each acting player gets their own yes/no, a decline discards
// NOTHING (the old stand-in discarded the whole hand), an acceptance discards
// that player's whole hand, and the cursor keeps each answer attached to the
// player who gave it.
func TestDiscardHandOptionalAsksEachPlayerAndDeclineDiscardsNothing(t *testing.T) {
	ah := &askHost{}
	ah.g = state.NewGame(names(2))
	src := ah.g.AddObject(mkCard(t, "Name:Wheel\nTypes:Sorcery\nOracle:x\n"), 0)
	frog := ah.g.AddObject(creature(t, "Frog"), 0)
	bird := ah.g.AddObject(creature(t, "Bird"), 1)
	ah.g.Obj(frog.ID).Zone = state.ZHand
	ah.g.Obj(bird.ID).Zone = state.ZHand
	ah.g.SetZone(state.ZHand, 0, []state.ObjID{frog.ID})
	ah.g.SetZone(state.ZHand, 1, []state.ObjID{bird.ID})
	ctx := &Ctx{Source: src.ID, Controller: 0}
	s := sa(t, "SP$ Discard | Mode$ Hand | Defined$ Player | Optional$ True | RememberDiscardingPlayers$ True")

	effDiscard(ah, ctx, s)
	if ah.asked == nil {
		t.Fatal("the may-discard election posed no decision")
	}
	if ah.asked.Player != 0 || ah.asked.ResumeKind != "discard_hand" || ah.asked.ResumeTarget != 0 {
		t.Fatalf("first election = player %d kind %q target %d, want seat 0 / \"discard_hand\" / 0",
			ah.asked.Player, ah.asked.ResumeKind, ah.asked.ResumeTarget)
	}
	if len(ah.asked.Options) != 2 {
		t.Fatalf("election options = %d, want a yes and a no", len(ah.asked.Options))
	}

	// Seat 0 declines.
	ctx.DiscardVote = "no"
	ctx.DiscardTarget = 0
	effDiscard(ah, ctx, s)
	if inZone(ah.g, state.ZGraveyard, 0, frog.ID) {
		t.Fatal("a DECLINED may-discard election still discarded the whole hand — the stand-in is running")
	}
	if !inZone(ah.g, state.ZHand, 0, frog.ID) {
		t.Fatal("the declining player's card left the hand")
	}

	// Seat 1's own election is then posed (the cursor moved on).
	if ah.asked.Player != 1 || ah.asked.ResumeTarget != 1 {
		t.Fatalf("second election = player %d target %d, want seat 1 / 1", ah.asked.Player, ah.asked.ResumeTarget)
	}
	ctx.DiscardVote = "yes"
	ctx.DiscardTarget = 1
	effDiscard(ah, ctx, s)
	if !inZone(ah.g, state.ZGraveyard, 1, bird.ID) {
		t.Fatal("the accepting player's card was not discarded")
	}
	if inZone(ah.g, state.ZHand, 1, bird.ID) {
		t.Fatal("the accepting player's card is still in hand")
	}
}

// TestDiscardTgtChooseMultiTargetAsksEveryTarget pins the multi-target walk:
// a TgtChoose discard whose acting-player list holds TWO players asks each
// one in order, and each answer is applied to the target that gave it — the
// old walk applied target 0's answer to every later target and abandoned
// targets 2..n. The acting list here is `Defined$ You & Opponent` (caster
// seat 0 first, opponent seat 1 second), the same deterministic list the
// engine's cursor indexes.
func TestDiscardTgtChooseMultiTargetAsksEveryTarget(t *testing.T) {
	ah := &askHost{}
	ah.g = state.NewGame(names(2))
	src := ah.g.AddObject(mkCard(t, "Name:Mind Rot Wheel\nTypes:Sorcery\nOracle:x\n"), 0)
	frog := ah.g.AddObject(creature(t, "Frog"), 0)
	cat := ah.g.AddObject(creature(t, "Cat"), 0)
	bird := ah.g.AddObject(creature(t, "Bird"), 1)
	dog := ah.g.AddObject(creature(t, "Dog"), 1)
	for _, o := range []*state.Object{frog, cat, bird, dog} {
		ah.g.Obj(o.ID).Zone = state.ZHand
	}
	ah.g.SetZone(state.ZHand, 0, []state.ObjID{frog.ID, cat.ID})
	ah.g.SetZone(state.ZHand, 1, []state.ObjID{bird.ID, dog.ID})
	ctx := &Ctx{Source: src.ID, Controller: 0}
	s := sa(t, "SP$ Discard | Defined$ You & Opponent | Mode$ TgtChoose | NumCards$ 1")

	// Target 0 (the caster, seat 0) is asked first, over its own hand.
	effDiscard(ah, ctx, s)
	if ah.asked == nil || ah.asked.Player != 0 {
		t.Fatalf("first ask = %+v, want a decision to seat 0", ah.asked)
	}

	// Target 0 answers (cat — deliberately not the front card).
	ctx.Discard = []state.ObjID{cat.ID}
	ctx.DiscardTarget = 0
	effDiscard(ah, ctx, s)
	if !inZone(ah.g, state.ZGraveyard, 0, cat.ID) {
		t.Fatal("target 0's answer was not applied to target 0's hand")
	}
	if ah.asked.Player != 1 || ah.asked.ResumeTarget != 1 {
		t.Fatalf("second ask = player %d target %d, want seat 1 / 1 — target 1 was never asked",
			ah.asked.Player, ah.asked.ResumeTarget)
	}

	// Target 1 answers (dog).
	ctx.Discard = []state.ObjID{dog.ID}
	ctx.DiscardTarget = 1
	effDiscard(ah, ctx, s)
	if !inZone(ah.g, state.ZGraveyard, 1, dog.ID) {
		t.Fatal("target 1's answer was not applied to target 1's hand — the walk abandoned targets 2..n")
	}
	if !inZone(ah.g, state.ZHand, 0, frog.ID) || !inZone(ah.g, state.ZHand, 1, bird.ID) {
		t.Fatal("a card nobody chose left a hand")
	}
	if len(ah.g.Zone(state.ZHand, 0)) != 1 || len(ah.g.Zone(state.ZHand, 1)) != 1 {
		t.Fatalf("hand sizes after both answers = %d/%d, want 1/1",
			len(ah.g.Zone(state.ZHand, 0)), len(ah.g.Zone(state.ZHand, 1)))
	}
}

// TestDiscardChooseModesPoseNoAskWhenNothingIsEligible guards the ask-shape's
// empty-pool edge (the ask_empty contract): a choosing-mode discard into a
// hand with zero DiscardValid$-eligible cards resolves silently — no decision
// (it would be unanswerable), no stand-in Note (skipping an unanswerable ask
// is correct resolution, not a degradation).
func TestDiscardChooseModesPoseNoAskWhenNothingIsEligible(t *testing.T) {
	ah, ctx, _ := discardBoard(t, land(t, "Islet"))
	s := sa(t, "SP$ Discard | ValidTgts$ Player | Mode$ LookYouChoose | NumCards$ 1 | DiscardValid$ Card.nonLand")

	effDiscard(ah, ctx, s)
	if ah.asked != nil {
		t.Fatal("a LookYouChoose over an all-land hand posed an unanswerable decision")
	}
	if standinNotes(ah.log) != 0 {
		t.Fatal("the empty-pool skip recorded the R-9 stand-in Note")
	}
	if len(ah.g.Zone(state.ZHand, 1)) != 1 {
		t.Fatal("the land left the hand although nothing was eligible")
	}
}

// TestDiscardChooseModeMultiTargetCursorDoesNotStall pins the choosing
// modes' own cursor: two acting players, each with a hand — the first
// answer is consumed at target 0 only, and target 1 gets its own ask (the
// old "every target sees the SAME answered list" behaviour re-asked target 1
// forever and never discarded its card).
func TestDiscardChooseModeMultiTargetCursorDoesNotStall(t *testing.T) {
	ah := &askHost{}
	ah.g = state.NewGame(names(2))
	src := ah.g.AddObject(mkCard(t, "Name:Look Wheel\nTypes:Sorcery\nOracle:x\n"), 0)
	frog := ah.g.AddObject(creature(t, "Frog"), 0)
	bird := ah.g.AddObject(creature(t, "Bird"), 1)
	ah.g.SetZone(state.ZHand, 0, []state.ObjID{frog.ID})
	ah.g.SetZone(state.ZHand, 1, []state.ObjID{bird.ID})
	ctx := &Ctx{Source: src.ID, Controller: 0}
	s := sa(t, "SP$ Discard | Defined$ You & Opponent | Mode$ LookYouChoose | NumCards$ 1")

	effDiscard(ah, ctx, s)
	if ah.asked == nil || ah.asked.Player != 0 {
		t.Fatalf("first ask = %+v, want a decision to seat 0", ah.asked)
	}

	ctx.Discard = []state.ObjID{frog.ID}
	ctx.DiscardTarget = 0
	effDiscard(ah, ctx, s)
	if !inZone(ah.g, state.ZGraveyard, 0, frog.ID) {
		t.Fatal("target 0's answer was not applied")
	}
	// The CHOSER for a LookYouChoose is the caster for every target's hand;
	// the cursor (ResumeTarget 1) says WHICH hand this answer is about.
	if ah.asked == nil || ah.asked.Player != 0 || ah.asked.ResumeTarget != 1 {
		t.Fatalf("second ask = %+v, want the caster's ask with ResumeTarget 1 — target 1 was re-asked or skipped", ah.asked)
	}
	for _, o := range ah.asked.Options {
		if o.Obj == frog.ID {
			t.Fatal("target 1 was offered target 0's hand — the answer cursor did not move")
		}
	}

	ctx.Discard = []state.ObjID{bird.ID}
	ctx.DiscardTarget = 1
	effDiscard(ah, ctx, s)
	if !inZone(ah.g, state.ZGraveyard, 1, bird.ID) {
		t.Fatal("target 1's answer was not applied")
	}
}

// TestDiscardDefinedAppliesTheRememberRiders pins the Mode$ Defined arm on the
// SHARED discard-and-remember path every other mode uses. DefinedCards$ names
// the cards (Breathstealer's Crypt's "that player discards it"), and because
// the arm routes through discardAndRemember it applies RememberDiscarded$ and
// RememberDiscardingPlayers$ per card -- both the resolution's Ctx.Remembered
// set and the source object's event-backed remembered list. Emitting
// events.Discard directly, as the arm used to, moves the card but records
// neither, so a chained "for each card discarded this way" reads nothing.
func TestDiscardDefinedAppliesTheRememberRiders(t *testing.T) {
	ah, ctx, ids := discardBoard(t, creature(t, "Frog"), creature(t, "Bird"), creature(t, "Cat"))
	s := sa(t, "SP$ Discard | ValidTgts$ Player | Mode$ Defined | DefinedCards$ Remembered"+
		" | RememberDiscarded$ True | RememberDiscardingPlayers$ True")
	// The named cards are deliberately NOT the front of hand, so a front-card
	// discard could not pass, and there are two of them, so a stale hand slice
	// would show up as a skipped second card.
	ctx.Remembered = []state.Target{{Obj: ids[1]}, {Obj: ids[2]}}
	if ah.g.Zone(state.ZHand, 1)[0] != ids[0] {
		t.Fatal("precondition: the hand's front card is not frog — the named-card assertions would not discriminate")
	}

	effDiscard(ah, ctx, s)

	for _, want := range []state.ObjID{ids[1], ids[2]} {
		if !inZone(ah.g, state.ZGraveyard, 1, want) {
			t.Fatalf("the DefinedCards$ card %d was not discarded", want)
		}
	}
	if !inZone(ah.g, state.ZHand, 1, ids[0]) {
		t.Fatal("the un-named front card (frog) was discarded — Defined ignored DefinedCards$")
	}
	// RememberDiscarded$: both halves. Ctx.Remembered already held the two
	// named cards, so the discriminating half is the event-backed one the
	// source object carries.
	var remembered []state.ObjID
	for _, ev := range ah.log {
		if ev.Kind == events.Choose && ev.Counter == "remembered" && ev.Obj == ctx.Source {
			remembered = append(remembered, ev.IDs...)
		}
	}
	if len(remembered) != 2 || remembered[0] != ids[1] || remembered[1] != ids[2] {
		t.Fatalf("RememberDiscarded$ recorded %v, want the two discarded cards %v — the Defined arm bypassed discardAndRemember",
			remembered, []state.ObjID{ids[1], ids[2]})
	}
	// RememberDiscardingPlayers$: the discarding player joins the set once.
	seen := 0
	for _, tg := range ctx.Remembered {
		if tg.IsPlayer && tg.Player == 1 {
			seen++
		}
	}
	if seen != 1 {
		t.Fatalf("RememberDiscardingPlayers$ recorded the discarder %d time(s), want exactly 1", seen)
	}
}
