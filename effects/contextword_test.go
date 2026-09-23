package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestContextWordPredicates is the effects leaf for the pc1 object/game-context
// predicate families the row named and this task closed. Each family reads
// provenance the object alone does not carry, but the engine already tracks
// it, so none needed a new tracker or a callable on SpecContext:
//
//   - wasDealtDamageThisTurn reads state.Object.WasDealtDamageThisTurn (the
//     per-turn provenance events.Apply's Damage case sets and TurnChange
//     clears). Corpus carriers: Wicked Akuba's "destroy target creature that
//     was dealt damage this turn" activation, and the Count$Valid walks.
//   - IsImprinted reads the SOURCE object's persistent imprint association
//     (state.Object.Imprinted/ImprintTokens/SeekFound) -- the same pile
//     Defined$ Imprinted resolves. Corpus carrier: Knowledge Pool's
//     `Valid$ Card.IsImprinted+!IsRemembered+nonLand` play gate.
//   - DefenderCtrl reads the defending player the resolving combat trigger
//     captured (TriggerContext.DefendingPlayer). Corpus carrier: Kjeldoran
//     Guard's `IsPresent$ Land.Snow+DefenderCtrl`.
//   - NotDefinedTargeted reads the resolving ability's recorded targets
//     (SpecContext.ResolutionTargets while Resolving). Corpus carrier: Wake
//     of Destruction's `Land.NotDefinedTargeted+sharesNameWith Targeted`.
//   - Opponent is the bare object twin of the player grammar's `Opponent`
//     base: controlled by an opponent of the evaluating controller.
//
// Every leaf asserts its precondition (the two candidate objects actually
// differ in the state the predicate reads) and pins the UNBOUND case for
// every context-bound family: with no binding the predicate fails closed for
// BOTH the positive and the leading-'!' negated spelling, so a recognised
// body can never be inverted into an always-true match. Each word is also
// asserted recognised by UnknownPredicates, so matcher and census agree.
func TestContextWordPredicates(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})

	// Leaf 1: wasDealtDamageThisTurn -- the object's own per-turn flag.
	damaged := g.Obj(corpusObject(t, reg, g, "Grizzly Bears").ID)
	damaged.WasDealtDamageThisTurn = true
	undamaged := g.Obj(corpusObject(t, reg, g, "Grizzly Bears").ID)
	if damaged.WasDealtDamageThisTurn == undamaged.WasDealtDamageThisTurn {
		t.Fatalf("precondition failed: the two candidates must differ in WasDealtDamageThisTurn")
	}
	if !MatchesObjectCtx(g, "Creature.wasDealtDamageThisTurn", damaged, SpecContext{You: 0}) {
		t.Errorf("Creature.wasDealtDamageThisTurn must match a creature dealt damage this turn")
	}
	if MatchesObjectCtx(g, "Creature.wasDealtDamageThisTurn", undamaged, SpecContext{You: 0}) {
		t.Errorf("Creature.wasDealtDamageThisTurn must not match an undamaged creature")
	}
	if got := UnknownPredicates("Creature.wasDealtDamageThisTurn"); len(got) != 0 {
		t.Errorf("UnknownPredicates(Creature.wasDealtDamageThisTurn) = %v, want empty", got)
	}

	// Leaf 2: IsImprinted -- membership in the SOURCE's imprint pile. The
	// source is Knowledge Pool (the corpus carrier); the imprinted card and a
	// bystander card are both real corpus cards. Precondition: exactly one of
	// them is in the source's Imprinted list. Every object is re-fetched by ID
	// after the last AddObject, because g.Objs reallocates as objects are added
	// and an earlier *state.Object pointer is then stale (the same discipline
	// TestGameAwarePredicates keeps).
	sourceID := corpusObject(t, reg, g, "Knowledge Pool").ID
	imprintedID := corpusObject(t, reg, g, "Grizzly Bears").ID
	bystanderID := corpusObject(t, reg, g, "Grizzly Bears").ID
	source := g.Obj(sourceID)
	imprintedCard := g.Obj(imprintedID)
	bystander := g.Obj(bystanderID)
	source.Imprinted = []state.ObjID{imprintedID}
	inList := func(id state.ObjID) bool {
		for _, x := range g.Obj(sourceID).Imprinted {
			if x == id {
				return true
			}
		}
		return false
	}
	if !inList(imprintedID) || inList(bystanderID) {
		t.Fatalf("precondition failed: Imprinted list must contain one candidate and not the other")
	}
	scImprint := SpecContext{You: 0, Source: sourceID}
	if !MatchesObjectCtx(g, "Card.IsImprinted", imprintedCard, scImprint) {
		t.Errorf("Card.IsImprinted must match a card in the source's imprint pile")
	}
	if MatchesObjectCtx(g, "Card.IsImprinted", bystander, scImprint) {
		t.Errorf("Card.IsImprinted must not match a card outside the source's imprint pile")
	}
	// A token imprinted through ImprintTokens$ True is in the same pile.
	tokenID := corpusObject(t, reg, g, "Grizzly Bears").ID
	token := g.Obj(tokenID)
	token.IsToken = true
	g.Obj(sourceID).ImprintTokens = []state.ObjID{tokenID}
	if !MatchesObjectCtx(g, "Card.IsImprinted", token, scImprint) {
		t.Errorf("Card.IsImprinted must match an ImprintTokens-imprinted token")
	}
	// Unbound (no source): the positive AND the negated spelling fail closed.
	if MatchesObjectCtx(g, "Card.IsImprinted", imprintedCard, SpecContext{You: 0}) {
		t.Errorf("Card.IsImprinted with no source must match nothing")
	}
	if MatchesObjectCtx(g, "Card.!IsImprinted", imprintedCard, SpecContext{You: 0}) {
		t.Errorf("Card.!IsImprinted with no source must also match nothing (unbound is not invertible)")
	}
	if got := UnknownPredicates("Card.IsImprinted"); len(got) != 0 {
		t.Errorf("UnknownPredicates(Card.IsImprinted) = %v, want empty", got)
	}

	// Leaf 3: DefenderCtrl -- controlled by the trigger's captured defending
	// player. Precondition: one candidate is controlled by seat 1, the other
	// by seat 0.
	defender := g.Obj(corpusObject(t, reg, g, "Grizzly Bears").ID)
	defender.Controller = 1
	notDefender := g.Obj(corpusObject(t, reg, g, "Grizzly Bears").ID)
	notDefender.Controller = 0
	if defender.Controller == notDefender.Controller {
		t.Fatalf("precondition failed: the two candidates must be controlled by different seats")
	}
	scCombat := SpecContext{You: 0, TriggerContext: TriggerContext{
		DefendingPlayer: state.Target{IsPlayer: true, Player: 1},
	}}
	if !MatchesObjectCtx(g, "Creature.DefenderCtrl", defender, scCombat) {
		t.Errorf("Creature.DefenderCtrl must match a creature the defending player controls")
	}
	if MatchesObjectCtx(g, "Creature.DefenderCtrl", notDefender, scCombat) {
		t.Errorf("Creature.DefenderCtrl must not match a creature another player controls")
	}
	// Unbound (no combat trigger): positive and negated both fail closed.
	if MatchesObjectCtx(g, "Creature.DefenderCtrl", defender, SpecContext{You: 0}) {
		t.Errorf("Creature.DefenderCtrl outside combat must match nothing")
	}
	if MatchesObjectCtx(g, "Creature.!DefenderCtrl", defender, SpecContext{You: 0}) {
		t.Errorf("Creature.!DefenderCtrl outside combat must also match nothing")
	}
	if got := UnknownPredicates("Creature.DefenderCtrl"); len(got) != 0 {
		t.Errorf("UnknownPredicates(Creature.DefenderCtrl) = %v, want empty", got)
	}

	// Leaf 4: NotDefinedTargeted -- NOT one of the resolving ability's
	// targets. Precondition: one candidate is the recorded target and the
	// other is not.
	targeted := g.Obj(corpusObject(t, reg, g, "Grizzly Bears").ID)
	untargeted := g.Obj(corpusObject(t, reg, g, "Grizzly Bears").ID)
	if targeted.ID == untargeted.ID {
		t.Fatalf("precondition failed: the candidates must be distinct objects")
	}
	scResolve := SpecContext{You: 0, Resolving: true,
		ResolutionTargets: []state.Target{{Obj: targeted.ID}}}
	if MatchesObjectCtx(g, "Permanent.NotDefinedTargeted", targeted, scResolve) {
		t.Errorf("Permanent.NotDefinedTargeted must not match the ability's own target")
	}
	if !MatchesObjectCtx(g, "Permanent.NotDefinedTargeted", untargeted, scResolve) {
		t.Errorf("Permanent.NotDefinedTargeted must match a permanent outside the target list")
	}
	// A resolving ability with NO targets admits every permanent (nothing was
	// targeted), so an empty non-nil list is not a fail-closed case.
	scNoTargets := SpecContext{You: 0, Resolving: true, ResolutionTargets: []state.Target{}}
	if !MatchesObjectCtx(g, "Permanent.NotDefinedTargeted", targeted, scNoTargets) {
		t.Errorf("Permanent.NotDefinedTargeted with an empty target list must match")
	}
	// Unbound (not resolving): positive and negated both fail closed.
	if MatchesObjectCtx(g, "Permanent.NotDefinedTargeted", untargeted, SpecContext{You: 0}) {
		t.Errorf("Permanent.NotDefinedTargeted outside resolution must match nothing")
	}
	if MatchesObjectCtx(g, "Permanent.!NotDefinedTargeted", untargeted, SpecContext{You: 0}) {
		t.Errorf("Permanent.!NotDefinedTargeted outside resolution must also match nothing")
	}
	if got := UnknownPredicates("Permanent.NotDefinedTargeted"); len(got) != 0 {
		t.Errorf("UnknownPredicates(Permanent.NotDefinedTargeted) = %v, want empty", got)
	}

	// Leaf 5: the bare Opponent object predicate -- controlled by an opponent
	// of the evaluating controller. Precondition: one candidate is controlled
	// by seat 1 (an opponent of You==0), the other by seat 0.
	opp := g.Obj(corpusObject(t, reg, g, "Grizzly Bears").ID)
	opp.Controller = 1
	mine := g.Obj(corpusObject(t, reg, g, "Grizzly Bears").ID)
	mine.Controller = 0
	if opp.Controller == mine.Controller {
		t.Fatalf("precondition failed: the two candidates must be controlled by different seats")
	}
	if !MatchesObjectCtx(g, "Creature.Opponent", opp, SpecContext{You: 0}) {
		t.Errorf("Creature.Opponent must match a creature an opponent controls")
	}
	if MatchesObjectCtx(g, "Creature.Opponent", mine, SpecContext{You: 0}) {
		t.Errorf("Creature.Opponent must not match a creature You control")
	}
	// A narrowed context (You==1) flips the two -- the predicate is evaluated
	// from the caller's seat, not the object's controller alone.
	if !MatchesObjectCtx(g, "Creature.Opponent", mine, SpecContext{You: 1}) {
		t.Errorf("Creature.Opponent from seat 1 must match a creature seat 0 controls")
	}
	if got := UnknownPredicates("Creature.Opponent"); len(got) != 0 {
		t.Errorf("UnknownPredicates(Creature.Opponent) = %v, want empty", got)
	}

	// Leaf 6 (regression): ExiledWithSource is the row's already-closed
	// family; pin it in the same leaf so a future edit to this file cannot
	// silently drop it.
	exiled := g.Obj(corpusObject(t, reg, g, "Grizzly Bears").ID)
	exiled.ExiledWith = sourceID
	if !MatchesObjectCtx(g, "Card.ExiledWithSource", exiled, SpecContext{You: 0, Source: sourceID}) {
		t.Errorf("Card.ExiledWithSource must match a card exiled with the source")
	}
	if got := UnknownPredicates("Card.ExiledWithSource"); len(got) != 0 {
		t.Errorf("UnknownPredicates(Card.ExiledWithSource) = %v, want empty", got)
	}
}
