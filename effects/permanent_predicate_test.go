package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestPermanentPredicateMatchesPermanentCards is the effects leaf for Forge's
// CardProperty.Permanent as a POSITIVE predicate (card.isPermanent()): the
// printed face is a permanent type, in any zone (CR 109.2). Before the fix the
// word was classified unknown, so Card.Permanent and every
// <base>.Permanent/<base>+Permanent spelling failed closed and matched nothing
// (22 corpus carriers: Badlands Revival's return-a-permanent-card, Deadly
// Brew's ConditionPresent$ gate, Auntie's Sentence's DiscardValid$, Six's
// retrace grant among them).
//
// The PREDICATE (Card.Permanent) and the BASE (Permanent) are two different
// readings and both are pinned here: the base stays the on-the-battlefield
// reading (TestPermanentOnlyMatchesBattlefield pins the same contract), while
// the predicate is the permanent-CARD reading in any zone.
func TestPermanentPredicateMatchesPermanentCards(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})

	// Real corpus faces: a creature card, a land card, an artifact card and
	// instant/sorcery cards. corpusObject lands them on the battlefield, so
	// move each to a non-battlefield zone to exercise the printed-face half.
	bear := corpusObject(t, reg, g, "Grizzly Bears")      // Creature Bear
	forest := corpusObject(t, reg, g, "Forest")           // Basic Land Forest
	expmap := corpusObject(t, reg, g, "Expedition Map")   // Artifact
	boltCard := corpusObject(t, reg, g, "Lightning Bolt") // Instant
	terminate := corpusObject(t, reg, g, "Terminate")     // Instant
	sorcery := corpusObject(t, reg, g, "Duress")          // Sorcery

	// Off the battlefield: a permanent CARD matches on its printed face.
	for _, o := range []*state.Object{bear, forest, expmap} {
		o.Zone = state.ZGraveyard
	}
	boltCard.Zone = state.ZGraveyard
	terminate.Zone = state.ZGraveyard
	sorcery.Zone = state.ZGraveyard

	sc := SpecContext{You: 0}
	if !MatchesObjectCtx(g, "Card.Permanent", bear, sc) {
		t.Errorf("Card.Permanent must match a creature card in the graveyard")
	}
	if !MatchesObjectCtx(g, "Card.Permanent", forest, sc) {
		t.Errorf("Card.Permanent must match a land card in the graveyard")
	}
	if !MatchesObjectCtx(g, "Card.Permanent", expmap, sc) {
		t.Errorf("Card.Permanent must match an artifact card in the graveyard")
	}
	if MatchesObjectCtx(g, "Card.Permanent", boltCard, sc) {
		t.Errorf("Card.Permanent must NOT match an instant card")
	}
	if MatchesObjectCtx(g, "Card.Permanent", terminate, sc) {
		t.Errorf("Card.Permanent must NOT match an instant card")
	}
	if MatchesObjectCtx(g, "Card.Permanent", sorcery, sc) {
		t.Errorf("Card.Permanent must NOT match a sorcery card")
	}

	// A permanent SPELL on the stack is a permanent card: the stack is still
	// not the battlefield, so this rides the printed-face half.
	bear.Zone = state.ZStack
	if !MatchesObjectCtx(g, "Card.Permanent", bear, sc) {
		t.Errorf("Card.Permanent must match a permanent spell on the stack")
	}
	boltCard.Zone = state.ZStack
	if MatchesObjectCtx(g, "Card.Permanent", boltCard, sc) {
		t.Errorf("Card.Permanent must not match an instant spell on the stack")
	}

	// A battlefield object is permanent regardless of its printed type (the
	// first clause of isPermanentCard; this is also the standalone `Permanent`
	// base's reading, which must not change).
	bear.Zone = state.ZBattlefield
	if !MatchesObjectCtx(g, "Card.Permanent", bear, sc) {
		t.Errorf("Card.Permanent must match a permanent on the battlefield")
	}

	// The predicate ANDs with the rest of the conjunction: Six's
	// Card.nonLand+Permanent shape matches a nonland permanent card and not a
	// land.
	forest.Zone = state.ZGraveyard
	if !MatchesObjectCtx(g, "Card.nonLand+Permanent", bear, sc) {
		t.Errorf("Card.nonLand+Permanent must match a nonland permanent card")
	}
	if MatchesObjectCtx(g, "Card.nonLand+Permanent", forest, sc) {
		t.Errorf("Card.nonLand+Permanent must not match a land card")
	}

	// The BASE is unchanged: a graveyard card is NOT a `Permanent` object.
	forest.Zone = state.ZGraveyard
	if MatchesObjectCtx(g, "Permanent", forest, sc) {
		t.Errorf("the bare Permanent BASE must stay battlefield-only")
	}
	if !MatchesObjectCtx(g, "Permanent", bear, sc) {
		t.Errorf("the bare Permanent BASE must match a battlefield permanent")
	}

	// Matcher and census agree on the now-recognised word.
	for _, spec := range []string{"Card.Permanent", "Card.nonLand+Permanent"} {
		if un := UnknownPredicates(spec); len(un) != 0 {
			t.Errorf("UnknownPredicates(%q) = %v, want empty", spec, un)
		}
	}
}

// TestPermanentPredicateGateAndTargetSpecs pins two real corpus carrier
// specs whose surrounding primitive is implemented: a
// ConditionPresent$ Card.Permanent+YouCtrl battlefield gate (the shape Deadly
// Brew, Rise of the Witch-king, Pep, Nurturing Pixie and Yarus carry, resolved
// by conditionMetBattlefield, which consults the shared matcher) and Badlands
// Revival's ValidTgts$ Card.Permanent+YouOwn (the sub-ability's target spec,
// evaluated by the same matcher). Both specs previously failed closed, so the
// gate was unresolved and the target leg had no legal target.
func TestPermanentPredicateGateAndTargetSpecs(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	h := newHost(t, 2)

	// A real SA carrying the corpus gate shape (Deadly Brew's own gate also
	// names ConditionDefined$ Remembered, which routes to the remembered-count
	// path; the battlefield presence gate is the primitive this predicate
	// feeds, so the SA carries exactly the two keys that reach it).
	gateSA := sa(t, "DB$ ChangeZone | ConditionPresent$ Card.Permanent+YouCtrl | ConditionCompare$ GE1")
	src := h.g.AddObject(mkCard(t, "Name:Src\nTypes:Sorcery\nOracle:x\n"), 0)
	gateCtx := &Ctx{Controller: 0, Source: src.ID}
	// No permanent on the battlefield: the Presence gate is unmet but
	// RESOLVED (the fail-open/fail-closed distinction the condition gate
	// draws: an unknown predicate is unresolved, which silently stops the
	// sub).
	if met, resolved := conditionMet(h, gateCtx, gateSA); met || !resolved {
		t.Fatalf("Card.Permanent+YouCtrl gate with an empty battlefield: met=%v resolved=%v, want false true", met, resolved)
	}
	// A permanent under the gate's controller: the gate now holds.
	permanent := corpusObject(t, reg, h.g, "Grizzly Bears")
	permanent.Controller = 0
	if met, resolved := conditionMet(h, gateCtx, gateSA); !met || !resolved {
		t.Fatalf("Card.Permanent+YouCtrl gate with a controlled permanent: met=%v resolved=%v, want true true", met, resolved)
	}
	// An instant parked on the battlefield would not satisfy it, but an
	// opponent's permanent is already out of YouCtrl.
	permanent.Controller = 1
	if met, resolved := conditionMet(h, gateCtx, gateSA); met || !resolved {
		t.Fatalf("Card.Permanent+YouCtrl gate with only an opponent's permanent: met=%v resolved=%v, want false true", met, resolved)
	}

	// Badlands Revival's sub-ability target spec: a permanent card in the
	// graveyard is a legal target, an instant card is not.
	_, saBad := corpusSA(t, "Badlands Revival", "DBReturn")
	spec := saBad.Params["ValidTgts"]
	if spec != "Card.Permanent+YouOwn" {
		t.Fatalf("Badlands Revival ValidTgts = %q, want Card.Permanent+YouOwn", spec)
	}
	gravePermanent := corpusObject(t, reg, h.g, "Llanowar Elves")
	gravePermanent.Zone = state.ZGraveyard
	gravePermanent.Owner = 0
	graveInstant := corpusObject(t, reg, h.g, "Lightning Bolt")
	graveInstant.Zone = state.ZGraveyard
	graveInstant.Owner = 0
	if !MatchesObjectCtx(h.g, spec, gravePermanent, SpecContext{You: 0}) {
		t.Errorf("Badlands Revival target spec must match a permanent card in your graveyard")
	}
	if MatchesObjectCtx(h.g, spec, graveInstant, SpecContext{You: 0}) {
		t.Errorf("Badlands Revival target spec must not match an instant card")
	}
	// A permanent card owned by the other seat is out of YouOwn.
	theirs := corpusObject(t, reg, h.g, "Grizzly Bears")
	theirs.Zone = state.ZGraveyard
	theirs.Owner = 1
	if MatchesObjectCtx(h.g, spec, theirs, SpecContext{You: 0}) {
		t.Errorf("Badlands Revival target spec must not match another owner's card")
	}
}

// TestPermanentPredicateReachesGrantAndStaticSpecs pins the two `Affected$`
// specs the fix was filed for, through MatchesSpecFrom -- the matcher call the
// may-play and keyword-grant paths use -- so the class-wide reach is measured
// on the real carriers rather than assumed from the target leg alone: Wrenn and
// Realmbreaker's `Affected$ Card.Permanent+YouOwn` emblem (the
// MayPlay$ True | AffectedZone$ Graveyard grant) and Six's
// `Affected$ Card.nonLand+Permanent+YouOwn` retrace static (the one repo-deck
// carrier, `pro-shaper`). Delivery of the grant is separate machinery; what
// this asserts is the match the filter fix owns.
func TestPermanentPredicateReachesGrantAndStaticSpecs(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	h := newHost(t, 2)

	// Wrenn's emblem static is the real corpus line (its Affected$ spec plus
	// MayPlay$ True); the static is delivered through the emblem's AB$ Effect
	// StaticAbilities$ PermanentRecycle SVar, so read the real spec text from
	// that SVar rather than a literal, and fail if the script changes.
	wrenn, ok := reg.Lookup("Wrenn and Realmbreaker")
	if !ok {
		t.Fatal("corpus has no Wrenn and Realmbreaker")
	}
	permanentSpec := ""
	if body := wrenn.Faces[0].SVars["PermanentRecycle"]; body != "" {
		for _, part := range strings.Split(body, "|") {
			k, v, found := strings.Cut(part, "$")
			if found && strings.TrimSpace(k) == "Affected" {
				permanentSpec = strings.TrimSpace(v)
			}
		}
	}
	if permanentSpec != "Card.Permanent+YouOwn" {
		t.Fatalf("Wrenn emblem Affected$ = %q, want Card.Permanent+YouOwn", permanentSpec)
	}
	permanent := corpusObject(t, reg, h.g, "Llanowar Elves")
	permanent.Zone = state.ZGraveyard
	permanent.Owner = 0
	if !MatchesSpecFrom(h.g, permanentSpec, permanent.ID, 0, 0) {
		t.Errorf("Wrenn emblem Affected$ must match a permanent card in its owner's graveyard")
	}
	instant := corpusObject(t, reg, h.g, "Lightning Bolt")
	instant.Zone = state.ZGraveyard
	instant.Owner = 0
	if MatchesSpecFrom(h.g, permanentSpec, instant.ID, 0, 0) {
		t.Errorf("Wrenn emblem Affected$ must not match an instant card")
	}

	// Six's retrace static spec.
	six, ok := reg.Lookup("Six")
	if !ok {
		t.Fatal("corpus has no Six")
	}
	sixSpec := ""
	for _, st := range six.Faces[0].Statics {
		if st.Params["AddKeyword"] == "Retrace" {
			sixSpec = st.Params["Affected"]
		}
	}
	if sixSpec != "Card.nonLand+Permanent+YouOwn" {
		t.Fatalf("Six retrace Affected$ = %q, want Card.nonLand+Permanent+YouOwn", sixSpec)
	}
	if !MatchesSpecFrom(h.g, sixSpec, permanent.ID, 0, 0) {
		t.Errorf("Six retrace Affected$ must match a nonland permanent card in your graveyard")
	}
	forest := corpusObject(t, reg, h.g, "Forest")
	forest.Zone = state.ZGraveyard
	forest.Owner = 0
	if MatchesSpecFrom(h.g, sixSpec, forest.ID, 0, 0) {
		t.Errorf("Six retrace Affected$ must not match a land card")
	}
}
