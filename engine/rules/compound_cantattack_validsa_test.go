package rules

// cr-g204: the compound `S:Mode$ CantAttack,CantBlock,CantBeActivated |
// ValidCard$ Card.EnchantedBy | ValidSA$ Activated.!ManaAbility` line on
// Realmbreaker's Grasp and Intercessor's Arrest. The parse-time split
// (engine/cards/parse.go, commit f81f996e) gives each mode its own static but
// shares ONE Params map, so the CantAttack half carries ValidSA$, which belongs
// to the sibling CantBeActivated mode. CantAttackParamsReadableForRules now
// admits ValidSA$ as INERT (Forge's StaticAbilityCantAttackBlock never reads
// it), so the attack half binds instead of being skipped whole. The CantBlock
// half already bound (BlockRestricted reads no whitelist) and the
// CantBeActivated half is unchanged: mana abilities stay available.
//
// Every fixture mutation goes through e.emit, so every test ends
// replay-verified.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// compoundCantAttackCarrier asserts the corpus card still prints the compound
// split this ticket fixes: one CantAttack, one CantBlock and one
// CantBeActivated static, with the CantAttack half carrying the sibling mode's
// ValidSA$ value through the shared Params map.
func compoundCantAttackCarrier(t *testing.T, c *cards.Card) {
	t.Helper()
	var attack *cards.Static
	modes := map[string]bool{}
	for _, f := range c.Faces {
		for i := range f.Statics {
			st := &f.Statics[i]
			modes[st.Mode] = true
			if st.Mode == "CantAttack" {
				attack = st
			}
		}
	}
	if !modes["CantAttack"] || !modes["CantBlock"] || !modes["CantBeActivated"] {
		t.Fatalf("%s lacks the split CantAttack/CantBlock/CantBeActivated statics: %v", c.Faces[0].Name, modes)
	}
	if got := attack.ParamStr(cards.PKValidSA); got != "Activated.!ManaAbility" {
		t.Fatalf("%s: CantAttack half ValidSA$ = %q, want the shared Activated.!ManaAbility; statics %+v",
			c.Faces[0].Name, got, c.Faces[0].Statics)
	}
}

// TestRealmbreakersGraspStopsAttackAndBlock is the probed board: the real
// corpus Aura on seat 1's bear stops the attack (the regression -- before the
// whitelist admitted ValidSA$ the CantAttack static was skipped whole while
// the CantBlock half bound) and the block, and the enchanted bear is absent
// from the KAttackers option list.
func TestRealmbreakersGraspStopsAttackAndBlock(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	grasp := lookup(t, reg, "Realmbreaker's Grasp")
	compoundCantAttackCarrier(t, grasp)
	bear := card(t, staticBearFixture)
	enchanted := card(t, staticBearFixture)
	e, cfg := restrictionGame(t, 6204,
		[][]*cards.Card{{grasp}, nil, nil},
		[][]*cards.Card{nil, {bear, enchanted}, {bear}})
	bear1 := bearOnBoard(t, e, 1, bear)
	victim := bearOnBoard(t, e, 1, enchanted)
	bear2 := bearOnBoard(t, e, 2, bear)

	graspID := findAndMoveToHand(t, e, 0, "Realmbreaker's Grasp")
	addMana(t, e, 0, "CW")
	castFromPriority(t, e, graspID)
	answerKTarget(t, e, victim)
	passUntilStackEmpty(t, e, 60)
	if got := e.G.Obj(graspID); got == nil || got.AttachedTo != victim {
		t.Fatalf("grasp AttachedTo = %+v, want the enchanted bear %d", got, victim)
	}

	// Direct reads: the CantAttack half binds against every defender, the
	// CantBlock half still binds, and the control bear (the precondition that
	// the two really differ) binds neither.
	if !e.attackBlocked(victim, 0, 0) || !e.attackBlocked(victim, 2, 0) {
		t.Fatalf("enchanted bear attack pairs not blocked: (→0)=%v (→2)=%v",
			e.attackBlocked(victim, 0, 0), e.attackBlocked(victim, 2, 0))
	}
	if !e.blockRestricted(victim, bear2) {
		t.Fatal("enchanted bear not restricted from blocking")
	}
	if e.attackBlocked(bear1, 0, 0) || e.blockRestricted(bear1, bear2) {
		t.Fatal("control bear (precondition) is restricted — the fixture does not isolate the enchanted one")
	}

	// The real offer path: seat 1's KAttackers must offer the control bear a
	// pair and never the enchanted one.
	driveToStep(t, e, 3, 1, state.StepDeclareAttackers)
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("expected the attackers decision at seat 1's declare step, got %+v", d)
	}
	if opts := attackerOptionsFor(e, victim); len(opts) != 0 {
		t.Fatalf("enchanted bear still offered as an attacker: %+v", opts)
	}
	if opts := attackerOptionsFor(e, bear1); len(opts) == 0 {
		t.Fatal("precondition failed: the control bear has no attack pair to offer")
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{}}); err != nil {
		t.Fatalf("declare no attackers: %v", err)
	}
	replayCheck(t, e, cfg)
}

// TestIntercessorsArrestStopsAttack is the same rule on the other Standard
// carrier, whose line also lists CantCrew (cr-g182's half): the attack half
// must bind, not only the block half.
func TestIntercessorsArrestStopsAttack(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	arrest := lookup(t, reg, "Intercessor's Arrest")
	compoundCantAttackCarrier(t, arrest)
	bear := card(t, staticBearFixture)
	enchanted := card(t, staticBearFixture)
	e, cfg := restrictionGame(t, 6205,
		[][]*cards.Card{{arrest}, nil, nil},
		[][]*cards.Card{nil, {bear, enchanted}, {bear}})
	bear1 := bearOnBoard(t, e, 1, bear)
	victim := bearOnBoard(t, e, 1, enchanted)
	bear2 := bearOnBoard(t, e, 2, bear)

	arrestID := findAndMoveToHand(t, e, 0, "Intercessor's Arrest")
	addMana(t, e, 0, "CCW")
	castFromPriority(t, e, arrestID)
	answerKTarget(t, e, victim)
	passUntilStackEmpty(t, e, 60)
	if got := e.G.Obj(arrestID); got == nil || got.AttachedTo != victim {
		t.Fatalf("arrest AttachedTo = %+v, want the enchanted bear %d", got, victim)
	}

	if !e.attackBlocked(victim, 0, 0) || !e.attackBlocked(victim, 2, 0) {
		t.Fatalf("enchanted bear attack pairs not blocked: (→0)=%v (→2)=%v",
			e.attackBlocked(victim, 0, 0), e.attackBlocked(victim, 2, 0))
	}
	if !e.blockRestricted(victim, bear2) {
		t.Fatal("enchanted bear not restricted from blocking")
	}
	if e.attackBlocked(bear1, 0, 0) {
		t.Fatal("control bear (precondition) is restricted — the fixture does not isolate the enchanted one")
	}

	driveToStep(t, e, 3, 1, state.StepDeclareAttackers)
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("expected the attackers decision at seat 1's declare step, got %+v", d)
	}
	if opts := attackerOptionsFor(e, victim); len(opts) != 0 {
		t.Fatalf("enchanted bear still offered as an attacker: %+v", opts)
	}
	if opts := attackerOptionsFor(e, bear1); len(opts) == 0 {
		t.Fatal("precondition failed: the control bear has no attack pair to offer")
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{}}); err != nil {
		t.Fatalf("declare no attackers: %v", err)
	}
	replayCheck(t, e, cfg)
}

// TestGraspStillAllowsManaAbility pins the CantBeActivated half of the same
// split line against the attack-half fix: the enchanted creature's mana
// ability is still offered (ValidSA$ Activated.!ManaAbility spares it) while
// its non-mana activated ability is not. The pre-attach offer is the
// precondition that the missing non-mana option is the restriction's doing,
// not an unpayable/unofferable ability.
func TestGraspStillAllowsManaAbility(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	grasp := lookup(t, reg, "Realmbreaker's Grasp")
	compoundCantAttackCarrier(t, grasp)
	druid := lookup(t, reg, "Incubation Druid")
	mountain := lookup(t, reg, "Mountain")
	e, cfg := restrictionGame(t, 6206,
		[][]*cards.Card{{grasp}, nil},
		[][]*cards.Card{{druid, mountain}, nil})
	druidID := bearOnBoard(t, e, 0, druid)

	// The {3}{G}{G} Adapt ability is the non-mana half; the {T} mana ability
	// is the spared half. Before the Aura both are on the table.
	addMana(t, e, 0, "GGCCC")
	if _, ok := findAbilityOption(e, druidID, 0); !ok {
		t.Fatalf("precondition failed: Adapt (ability 0) is not offered before the Aura: %+v", e.Pending().Options)
	}
	if !hasActivateOption(e, druidID) {
		t.Fatalf("precondition failed: the mana ability is not offered before the Aura: %+v", e.Pending().Options)
	}

	graspID := findAndMoveToHand(t, e, 0, "Realmbreaker's Grasp")
	addMana(t, e, 0, "CW")
	castFromPriority(t, e, graspID)
	answerKTarget(t, e, druidID)
	passUntilStackEmpty(t, e, 60)
	if got := e.G.Obj(graspID); got == nil || got.AttachedTo != druidID {
		t.Fatalf("grasp AttachedTo = %+v, want the enchanted druid %d", got, druidID)
	}

	if !hasActivateOption(e, druidID) {
		t.Fatalf("the enchanted druid's mana ability is no longer offered: %+v", e.Pending().Options)
	}
	if _, ok := findAbilityOption(e, druidID, 0); ok {
		t.Fatalf("the enchanted druid's non-mana Adapt ability is still offered: %+v", e.Pending().Options)
	}
	replayCheck(t, e, cfg)
}
