// Ticket agent-20260918T232250Z-29aed5d6: `S:Mode$ IgnoreLegendRule` is now
// honored by the CR 704.5j legend SBA. A permanent a live IgnoreLegendRule
// static's `ValidCard$` admits never joins a duplicate set, so the static's
// own controller can keep two same-named legendary permanents. The named real
// corpus carrier is Council of Reeds ("The 'legend rule' doesn't apply to
// creatures you control.", `ValidCard$ Creature.YouCtrl`); it is scoped per
// controller like the base rule, so an opponent's same-named pair is still
// asked. These tests pin the positive case, both scope boundaries (a
// noncreature and another player's creature), the exemption ending when its
// source leaves the battlefield, and a deterministic event stream.
package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// ignoreLegendTwin is a synthetic legendary creature used for the duplicate
// sets the exemption is tested against. It is deliberately NOT Council of
// Reeds: two Councils would make the carrier its own duplicate set.
const ignoreLegendTwin = "Name:Ignore Legend Twin\nManaCost:2 G\nTypes:Legendary Creature Bear\nPT:3/3\nOracle:x\n"

// ignoreLegendRelic is the noncreature scope boundary: a legendary artifact
// Council's `Creature.YouCtrl` must NOT exempt.
const ignoreLegendRelic = "Name:Ignore Legend Relic\nManaCost:2\nTypes:Legendary Artifact\nOracle:x\n"

// ignoreLegendEngine builds the two-seat constructed engine every fixture here
// uses, so seat 0 is the protagonist (see seatZeroStart) and the CR 903.9
// commander-zone replacement is out of the way.
func ignoreLegendEngine(t *testing.T) *Engine {
	t.Helper()
	return New(seatZeroStart(Config{Seed: 1, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}}))
}

// assertCouncilExemptionLive asserts the fixture precondition the positive
// assertion depends on: the corpus carrier is ON THE BATTLEFIELD, its
// IgnoreLegendRule static is collected by the canonical walk, and its
// `ValidCard$ Creature.YouCtrl` admits both candidate permanents under seat
// 0's control. Without all four the "no choice posed" assertion could pass
// vacuously (no source, or a static whose spec failed to parse).
func assertCouncilExemptionLive(t *testing.T, e *Engine, council, id1, id2 state.ObjID) {
	t.Helper()
	if o := e.G.Obj(council); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("fixture: Council of Reeds %d is not on the battlefield (zone %v)", council, o)
	}
	statics := e.activeStatics("IgnoreLegendRule")
	if len(statics) != 1 || statics[0].Source != council {
		t.Fatalf("fixture: activeStatics(\"IgnoreLegendRule\") = %d entries (want exactly the Council at %d)",
			len(statics), council)
	}
	if got := strings.TrimSpace(statics[0].Params["ValidCard"]); got != "Creature.YouCtrl" {
		t.Fatalf("fixture: Council's ValidCard$ is %q, want Creature.YouCtrl", got)
	}
	for _, id := range []state.ObjID{id1, id2} {
		if !e.legendRuleExempt(statics, id) {
			t.Fatalf("fixture: Council's Creature.YouCtrl did not exempt permanent %d it should admit", id)
		}
	}
}

// assertLegendTwinPair asserts the duplicate-set precondition: two distinct
// permanents, both legendary, same name, same controller, same zone.
func assertLegendTwinPair(t *testing.T, e *Engine, p state.PlayerID, name string, id1, id2 state.ObjID) {
	t.Helper()
	for _, id := range []state.ObjID{id1, id2} {
		o := e.G.Obj(id)
		if o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("fixture: permanent %d not on the battlefield (zone %v)", id, o)
		}
		if !o.Face().IsLegendary() || o.Face().Name != name {
			t.Fatalf("fixture: permanent %d is %q legendary=%v, want legendary %q",
				id, o.Face().Name, o.Face().IsLegendary(), name)
		}
		if o.Controller != p {
			t.Fatalf("fixture: permanent %d controlled by seat %d, want %d", id, o.Controller, p)
		}
	}
	if id1 == id2 {
		t.Fatalf("fixture: both duplicates share id %d", id1)
	}
}

// TestIgnoreLegendRuleExemptsMatchingCreatures is the headline: with Council
// of Reeds live under seat 0, two same-named legendary creatures it controls
// are exempt from CR 704.5j -- NO legend choice is posed and both stay on the
// battlefield. Without the fix legendGroups gathers them and parks a choice,
// so this fails with the hunk reverted.
func TestIgnoreLegendRuleExemptsMatchingCreatures(t *testing.T) {
	t.Parallel()
	e := ignoreLegendEngine(t)
	council := onBoardCard(t, e, 0, corpusCard(t, "Council of Reeds"))
	id1 := onBoard(t, e, 0, ignoreLegendTwin)
	id2 := onBoard(t, e, 0, ignoreLegendTwin)
	assertCouncilExemptionLive(t, e, council, id1, id2)
	assertLegendTwinPair(t, e, 0, "Ignore Legend Twin", id1, id2)

	e.checkStateBased()

	if d := e.Pending(); d != nil {
		t.Fatalf("a decision %s is pending under a live IgnoreLegendRule exemption", d.Kind)
	}
	if e.legendBatch != nil {
		t.Fatalf("a legend batch is parked under a live IgnoreLegendRule exemption")
	}
	for _, id := range []state.ObjID{id1, id2} {
		if o := e.G.Obj(id); o.Zone != state.ZBattlefield {
			t.Fatalf("exempt permanent %d left the battlefield for %v; the legend rule must not apply", id, o.Zone)
		}
	}
	// A "nothing happens" test must also prove the feature's handler ran: the
	// exemption walk is the only reason no choice was posed, and no
	// unimplemented-note stands in for it.
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "IgnoreLegendRule") {
			t.Fatalf("the exemption degraded to a Note instead of filtering: %q", ev.Text)
		}
	}
}

// TestIgnoreLegendRuleDoesNotExemptNoncreatures is the scope boundary: the
// carrier's `Creature.YouCtrl` does not cover a legendary NONCREATURE, so two
// same-named legendary artifacts under seat 0 still form a CR 704.5j set and
// the controller is asked -- while an exempt creature pair on the same board
// is not. The exempt pair makes this fail with the hunk reverted too (the
// reverted build would ask over the creature pair first).
func TestIgnoreLegendRuleDoesNotExemptNoncreatures(t *testing.T) {
	t.Parallel()
	e := ignoreLegendEngine(t)
	council := onBoardCard(t, e, 0, corpusCard(t, "Council of Reeds"))
	// An exempt creature pair is present first and must NOT be the set asked.
	c1 := onBoard(t, e, 0, ignoreLegendTwin)
	c2 := onBoard(t, e, 0, ignoreLegendTwin)
	a1 := onBoard(t, e, 0, ignoreLegendRelic)
	a2 := onBoard(t, e, 0, ignoreLegendRelic)
	assertCouncilExemptionLive(t, e, council, c1, c2)
	assertLegendTwinPair(t, e, 0, "Ignore Legend Relic", a1, a2)

	e.checkStateBased()
	statics := e.activeStatics("IgnoreLegendRule")
	for _, id := range []state.ObjID{a1, a2} {
		if e.legendRuleExempt(statics, id) {
			t.Fatalf("fixture: Creature.YouCtrl exempted the legendary artifact %d -- the boundary would be untested", id)
		}
	}
	d := legendPending(t, e, 0, a1, a2)
	submitKeep(t, e, d, 0)
	if o := e.G.Obj(a1); o.Zone != state.ZBattlefield {
		t.Fatalf("the kept artifact left the battlefield (zone %v)", o.Zone)
	}
	if o := e.G.Obj(a2); o.Zone != state.ZGraveyard {
		t.Fatalf("the unchosen artifact is in %v, want its owner's graveyard", o.Zone)
	}
	for _, id := range []state.ObjID{c1, c2} {
		if o := e.G.Obj(id); o.Zone != state.ZBattlefield {
			t.Fatalf("the exempt creature %d is in %v, want on the battlefield", id, o.Zone)
		}
	}
}

// TestIgnoreLegendRuleDoesNotExemptOtherPlayersCreatures is the second scope
// boundary: Council of Reeds' `YouCtrl` is resolved against the static's own
// controller, so a same-named legendary creature pair controlled by the
// OPPONENT is not exempt and still asks that opponent's seat. An exempt
// creature pair under seat 0 sits FIRST in battlefield order, so the reverted
// build would ask over it and fail here too.
func TestIgnoreLegendRuleDoesNotExemptOtherPlayersCreatures(t *testing.T) {
	t.Parallel()
	e := ignoreLegendEngine(t)
	council := onBoardCard(t, e, 0, corpusCard(t, "Council of Reeds"))
	c1 := onBoard(t, e, 0, ignoreLegendTwin)
	c2 := onBoard(t, e, 0, ignoreLegendTwin)
	id1 := onBoard(t, e, 1, ignoreLegendTwin)
	id2 := onBoard(t, e, 1, ignoreLegendTwin)
	assertCouncilExemptionLive(t, e, council, c1, c2)
	assertLegendTwinPair(t, e, 1, "Ignore Legend Twin", id1, id2)
	statics := e.activeStatics("IgnoreLegendRule")
	for _, id := range []state.ObjID{id1, id2} {
		if e.legendRuleExempt(statics, id) {
			t.Fatalf("fixture: seat 0's Creature.YouCtrl exempted seat 1's creature %d -- the boundary would be untested", id)
		}
	}

	e.checkStateBased()
	d := legendPending(t, e, 1, id1, id2)
	submitKeep(t, e, d, 1)
	if o := e.G.Obj(id2); o.Zone != state.ZBattlefield {
		t.Fatalf("the kept opponent creature left the battlefield (zone %v)", o.Zone)
	}
	if o := e.G.Obj(id1); o.Zone != state.ZGraveyard {
		t.Fatalf("the unchosen opponent creature is in %v, want its owner's graveyard", o.Zone)
	}
	for _, id := range []state.ObjID{c1, c2} {
		if o := e.G.Obj(id); o.Zone != state.ZBattlefield {
			t.Fatalf("the exempt seat-0 creature %d is in %v, want on the battlefield", id, o.Zone)
		}
	}
}

// TestIgnoreLegendRuleHonorsConditionTrue: Brothers Yamazaki's exemption is
// GATED -- `IsPresent$ Permanent.namedBrothers Yamazaki | PresentCompare$
// EQ2`, "If there are exactly two permanents named Brothers Yamazaki on the
// battlefield". With exactly two on the battlefield the gate holds and both
// are exempt, so no duplicate set forms and no choice is posed. The fixture
// asserts the gate actually evaluates true (not that the walk was empty).
func TestIgnoreLegendRuleHonorsConditionTrue(t *testing.T) {
	t.Parallel()
	e := ignoreLegendEngine(t)
	b1 := onBoardCard(t, e, 0, corpusCard(t, "Brothers Yamazaki"))
	b2 := onBoardCard(t, e, 0, corpusCard(t, "Brothers Yamazaki"))
	assertLegendTwinPair(t, e, 0, "Brothers Yamazaki", b1, b2)

	statics := e.activeStatics("IgnoreLegendRule")
	if len(statics) != 2 {
		t.Fatalf("fixture: activeStatics(\"IgnoreLegendRule\") = %d entries, want 2 Brothers Yamazaki", len(statics))
	}
	// Precondition: the condition is genuinely live for this board -- the
	// canonical walk has the static, its IsPresent$ parses, PresentCompare$ is
	// EQ2 and the count is exactly 2. Without that, the "no choice" assertion
	// below would pass for the wrong reason (a broken/absent gate reads false
	// and the test then proves the opposite of its name).
	if got := strings.TrimSpace(statics[0].Params["PresentCompare"]); got != "EQ2" {
		t.Fatalf("fixture: Brothers Yamazaki PresentCompare$ = %q, want EQ2", got)
	}
	if !e.continuousGateHolds(statics[0]) {
		t.Fatalf("fixture: the EQ2 gate does not hold with exactly two Brothers Yamazaki on the battlefield")
	}
	for _, id := range []state.ObjID{b1, b2} {
		if !e.legendRuleExempt(statics, id) {
			t.Fatalf("fixture: the live EQ2 exemption did not exempt permanent %d", id)
		}
	}

	e.checkStateBased()
	if d := e.Pending(); d != nil {
		t.Fatalf("a decision %s is pending although exactly two Brothers Yamazaki meet the EQ2 gate", d.Kind)
	}
	if e.legendBatch != nil {
		t.Fatalf("a legend batch is parked although the EQ2 exemption is live")
	}
	for _, id := range []state.ObjID{b1, b2} {
		if o := e.G.Obj(id); o.Zone != state.ZBattlefield {
			t.Fatalf("exempt permanent %d left the battlefield for %v", id, o.Zone)
		}
	}
}

// TestIgnoreLegendRuleHonorsConditionFalse is the finding's break case: with
// THREE Brothers Yamazaki on the battlefield the `PresentCompare$ EQ2` gate is
// FALSE, so the exemption does NOT apply and the ordinary CR 704.5j choice
// must be posed. Before the condition gate was wired the static exempted
// unconditionally, so no choice appeared and all three survived.
func TestIgnoreLegendRuleHonorsConditionFalse(t *testing.T) {
	t.Parallel()
	e := ignoreLegendEngine(t)
	b1 := onBoardCard(t, e, 0, corpusCard(t, "Brothers Yamazaki"))
	b2 := onBoardCard(t, e, 0, corpusCard(t, "Brothers Yamazaki"))
	b3 := onBoardCard(t, e, 0, corpusCard(t, "Brothers Yamazaki"))
	for _, id := range []state.ObjID{b1, b2, b3} {
		o := e.G.Obj(id)
		if o == nil || o.Zone != state.ZBattlefield || !o.Face().IsLegendary() || o.Face().Name != "Brothers Yamazaki" {
			t.Fatalf("fixture: permanent %d is not a legendary Brothers Yamazaki on the battlefield", id)
		}
	}
	statics := e.activeStatics("IgnoreLegendRule")
	if len(statics) != 3 {
		t.Fatalf("fixture: activeStatics(\"IgnoreLegendRule\") = %d entries, want 3 Brothers Yamazaki", len(statics))
	}
	// Precondition: the EQ2 gate is genuinely FALSE for three copies -- a
	// vacuous "three copies" (e.g. the gate silently missing) would make the
	// choice assertion untested.
	if e.continuousGateHolds(statics[0]) {
		t.Fatalf("fixture: the EQ2 gate holds with three Brothers Yamazaki on the battlefield -- the false case is not exercised")
	}
	for _, id := range []state.ObjID{b1, b2, b3} {
		if e.legendRuleExempt(statics, id) {
			t.Fatalf("fixture: the false EQ2 gate still exempted permanent %d", id)
		}
	}

	e.checkStateBased()
	d := legendPending(t, e, 0, b1, b2, b3)
	submitKeep(t, e, d, 0)
	if o := e.G.Obj(b1); o.Zone != state.ZBattlefield {
		t.Fatalf("the kept Yamazaki left the battlefield (zone %v)", o.Zone)
	}
	for _, id := range []state.ObjID{b2, b3} {
		if o := e.G.Obj(id); o.Zone != state.ZGraveyard {
			t.Fatalf("unexempted duplicate %d is in %v, want its owner's graveyard", id, o.Zone)
		}
	}
}

// TestIgnoreLegendRuleExemptionEndsWhenSourceLeaves: the exemption is live
// only while its source is. Moving Council of Reeds off the battlefield
// removes the static (asserted on the canonical walk) and the ordinary CR
// 704.5j choice returns for the pair it had been exempting.
func TestIgnoreLegendRuleExemptionEndsWhenSourceLeaves(t *testing.T) {
	t.Parallel()
	e := ignoreLegendEngine(t)
	council := onBoardCard(t, e, 0, corpusCard(t, "Council of Reeds"))
	id1 := onBoard(t, e, 0, ignoreLegendTwin)
	id2 := onBoard(t, e, 0, ignoreLegendTwin)
	assertCouncilExemptionLive(t, e, council, id1, id2)

	// While the source is live: no choice.
	e.checkStateBased()
	if d := e.Pending(); d != nil {
		t.Fatalf("a decision %s is pending while the exemption is live", d.Kind)
	}

	// The source leaves through the real event path. Assert the static is
	// actually gone before re-running the SBA, so the "choice returns"
	// assertion cannot pass on an unrelated failure to remove it.
	e.emit(events.Event{Kind: events.MoveZone, Obj: council, From: state.ZBattlefield, To: state.ZGraveyard})
	if o := e.G.Obj(council); o.Zone != state.ZGraveyard {
		t.Fatalf("fixture: Council of Reeds is in %v, want the graveyard", o.Zone)
	}
	if n := len(e.activeStatics("IgnoreLegendRule")); n != 0 {
		t.Fatalf("fixture: %d IgnoreLegendRule statics still live after the source left", n)
	}

	e.checkStateBased()
	d := legendPending(t, e, 0, id1, id2)
	submitKeep(t, e, d, 1)
	if o := e.G.Obj(id2); o.Zone != state.ZBattlefield {
		t.Fatalf("the kept twin left the battlefield (zone %v) after the exemption ended", o.Zone)
	}
	if o := e.G.Obj(id1); o.Zone != state.ZGraveyard {
		t.Fatalf("the unchosen twin is in %v, want its owner's graveyard once the exemption ended", o.Zone)
	}
}

// TestIgnoreLegendRuleEventStreamIsDeterministic runs the same mixed board
// (Council + an exempt creature pair + a non-exempt artifact pair, which
// forces a real choice and its move) twice in fresh engines and compares the
// chain head and the event kind stream. Membership maps in legendGroups are
// never iterated, so the exemption walk must not perturb the stream.
func TestIgnoreLegendRuleEventStreamIsDeterministic(t *testing.T) {
	t.Parallel()
	run := func() (*Engine, []events.Kind) {
		e := ignoreLegendEngine(t)
		onBoardCard(t, e, 0, corpusCard(t, "Council of Reeds"))
		// Exempt pair: filtered out entirely.
		onBoard(t, e, 0, ignoreLegendTwin)
		onBoard(t, e, 0, ignoreLegendTwin)
		// Non-exempt pair: the choice and its binned member.
		a1 := onBoard(t, e, 0, ignoreLegendRelic)
		a2 := onBoard(t, e, 0, ignoreLegendRelic)
		e.checkStateBased()
		d := legendPending(t, e, 0, a1, a2)
		submitKeep(t, e, d, 1)
		kinds := make([]events.Kind, len(e.L.Events))
		for i, ev := range e.L.Events {
			kinds[i] = ev.Kind
		}
		return e, kinds
	}
	e1, k1 := run()
	e2, k2 := run()
	if len(k1) != len(k2) {
		t.Fatalf("event count differs run to run: %d vs %d", len(k1), len(k2))
	}
	for i := range k1 {
		if k1[i] != k2[i] {
			t.Fatalf("event %d kind differs run to run: %v vs %v", i, k1[i], k2[i])
		}
	}
	if e1.L.Head() != e2.L.Head() {
		t.Fatalf("chain head differs run to run: %s vs %s", e1.L.Head(), e2.L.Head())
	}
	// The legend batch must be settled in both runs (the Submit tail leaves a
	// priority round pending, which is expected and not the batch).
	if e1.legendBatch != nil || e2.legendBatch != nil {
		t.Fatalf("a legend batch is still parked after the scenario settled")
	}
}

// echoPermissionSrc is the Hall of Echoes {5} ability's Effect body, resolved
// directly: a permanent carrying `DB$ Effect | StaticAbilities$
// LegendPermission` whose LegendPermission SVar is the card's exact
// `Mode$ IgnoreLegendRule | ValidCard$ Permanent.YouCtrl` line. The real card
// delivers it as the SubAbility$ of its AB$ Clone; this source isolates the
// registration and its lifetime for the scope and cleanup tests.
const echoPermissionSrc = "Name:Echo Permission\nTypes:Enchantment\n" +
	"SVar:DBEffect:DB$ Effect | StaticAbilities$ LegendPermission\n" +
	"SVar:LegendPermission:Mode$ IgnoreLegendRule | ValidCard$ Permanent.YouCtrl | Description$ The \"legend rule\" doesn't apply to permanents you control this turn.\n" +
	"Oracle:x\n"

// resolveEchoPermission resolves the DB$ Effect body on source exactly the
// way TestCantPlayLandHandLocked's registration recipe does, then asserts the
// fixture precondition the caller's assertions depend on: one live
// IgnoreLegendRule exemption is collected by legendExemptions (the
// registration, not a printed static -- the source carries no S: line).
func resolveEchoPermission(t *testing.T, e *Engine, source state.ObjID) {
	t.Helper()
	grant := e.G.Obj(source)
	effects.Resolve(e, &effects.Ctx{Source: source, Controller: grant.Controller, SVars: grant.Face().SVars},
		cards.ResolveSVar(grant.Face().SVars, "DBEffect"))
	exempt := e.legendExemptions()
	if len(exempt) != 1 || exempt[0].Source != source || exempt[0].Params == nil {
		t.Fatalf("fixture: the DB$ Effect did not register one live IgnoreLegendRule exemption (got %d views)", len(exempt))
	}
	if got := strings.TrimSpace(exempt[0].Params["ValidCard"]); got != "Permanent.YouCtrl" {
		t.Fatalf("fixture: the registered ValidCard$ is %q, want Permanent.YouCtrl", got)
	}
}

// TestEffectIgnoreLegendRuleHallOfEchoes is the reported card end to end:
// Hall of Echoes' {5} ability targets a legendary creature you control, the
// land becomes a copy of it, and the resolved ability's DB$ Effect registers
// the "the 'legend rule' doesn't apply to permanents you control this turn"
// exemption -- so the two same-named legendary permanents under seat 0 keep
// BOTH members: no legend choice posed, no legend batch parked, no
// IgnoreLegendRule Note. Without the fix the body Notes
// "IgnoreLegendRule unimplemented" and the next state-based check kills the
// copy, so this fails at the pending decision.
func TestEffectIgnoreLegendRuleHallOfEchoes(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := searchEngine(t, reg, "Hall of Echoes", "Isamaru, Hound of Konda")
	hall := searchMoveByName(t, e, "Hall of Echoes", state.ZBattlefield)
	isamaru := searchMoveByName(t, e, "Isamaru, Hound of Konda", state.ZBattlefield)

	addMana(t, e, 0, "CCCCC")
	opt := abilityOption(t, e, hall, 1)
	submitChoices(t, e, opt.Index)
	idx := cloneTargetOption(t, e, isamaru)
	if idx < 0 {
		t.Fatalf("the clone target ask offers no option for Isamaru %d: %+v", isamaru, e.Pending())
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 40)

	// The clone landed: Hall of Echoes is now a copy of Isamaru -- same
	// derived name, legendary -- so the two really are a CR 704.5j set.
	f := e.G.Obj(hall).Face()
	if f == nil || f.Name != "Isamaru, Hound of Konda" {
		t.Fatalf("Hall of Echoes did not become a copy (face %v)", f)
	}
	if !f.IsLegendary() {
		t.Fatal("the copy is not legendary; the legend rule would have nothing to fire on")
	}
	assertLegendTwinPair(t, e, 0, "Isamaru, Hound of Konda", isamaru, hall)
	// The registration is live and its Permanent.YouCtrl admits both members.
	exempt := e.legendExemptions()
	if len(exempt) != 1 {
		t.Fatalf("fixture: %d IgnoreLegendRule exemptions live, want exactly Hall of Echoes' registration", len(exempt))
	}
	for _, id := range []state.ObjID{isamaru, hall} {
		if !e.legendRuleExempt(exempt, id) {
			t.Fatalf("fixture: Permanent.YouCtrl did not exempt permanent %d", id)
		}
	}

	e.checkStateBased()

	if d := e.Pending(); d != nil && d.Kind != decision.KPriority {
		t.Fatalf("a decision %s is pending under Hall of Echoes' live exemption", d.Kind)
	}
	if e.legendBatch != nil {
		t.Fatalf("a legend batch is parked under Hall of Echoes' live exemption")
	}
	for _, id := range []state.ObjID{isamaru, hall} {
		if o := e.G.Obj(id); o.Zone != state.ZBattlefield {
			t.Fatalf("permanent %d is in %v; the legend rule must not apply this turn", id, o.Zone)
		}
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "IgnoreLegendRule") {
			t.Fatalf("the Effect body degraded to a Note instead of registering: %q", ev.Text)
		}
	}
	replayCheck(t, e, cfg)
}

// TestEffectIgnoreLegendRuleEndsAtCleanup pins the exemption's lifetime: the
// body carries no Duration$, so the registration is an end-of-turn one (CR
// 611.2a). This turn the pair is exempt and no choice is asked; after the
// turn's cleanup the registration is gone and the CR 704.5j choice is posed.
func TestEffectIgnoreLegendRuleEndsAtCleanup(t *testing.T) {
	t.Parallel()
	e := ignoreLegendEngine(t)
	source := onBoardGrant(t, e, 0, echoPermissionSrc)
	id1 := onBoard(t, e, 0, ignoreLegendTwin)
	id2 := onBoard(t, e, 0, ignoreLegendTwin)
	assertLegendTwinPair(t, e, 0, "Ignore Legend Twin", id1, id2)
	resolveEchoPermission(t, e, source)

	// This turn: no choice, no batch, both members stay.
	e.checkStateBased()
	if d := e.Pending(); d != nil {
		t.Fatalf("a decision %s is pending while the exemption is live", d.Kind)
	}
	if e.legendBatch != nil {
		t.Fatalf("a legend batch is parked while the exemption is live")
	}

	// Drive through this turn's cleanup, passing every priority round. The
	// choice must return exactly once the registration is gone (CR 514.2/514.3:
	// cleanup ends the until-end-of-turn effect, then the repeated SBA check
	// gathers the pair again).
	turn := e.G.Turn
	e.pending = nil
	e.Advance()
	for i := 0; i < 200; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while driving to cleanup")
		}
		if d.Kind != decision.KPriority {
			break
		}
		submitPass(t, e)
	}
	if n := len(e.legendExemptions()); n != 0 {
		t.Fatalf("%d IgnoreLegendRule exemptions still live after the turn's cleanup", n)
	}
	if e.G.Turn != turn || e.G.Step != state.StepCleanup {
		t.Fatalf("the choice returned at turn %d step %s, want the same turn's cleanup step", e.G.Turn, e.G.Step)
	}
	d := legendPending(t, e, 0, id1, id2)
	submitKeep(t, e, d, 0)
	if o := e.G.Obj(id1); o.Zone != state.ZBattlefield {
		t.Fatalf("the kept twin left the battlefield (zone %v)", o.Zone)
	}
	if o := e.G.Obj(id2); o.Zone != state.ZGraveyard {
		t.Fatalf("the unchosen twin is in %v, want its owner's graveyard once the exemption ended", o.Zone)
	}
}

// TestEffectIgnoreLegendRuleOnlyYourPermanents is the registration's scope
// boundary: the effect's `Permanent.YouCtrl` resolves against the EFFECT's
// controller, so seat 1's same-named pair is still gathered and still asked
// while seat 0's exemption is live.
func TestEffectIgnoreLegendRuleOnlyYourPermanents(t *testing.T) {
	t.Parallel()
	e := ignoreLegendEngine(t)
	source := onBoardGrant(t, e, 0, echoPermissionSrc)
	c1 := onBoard(t, e, 0, ignoreLegendTwin)
	c2 := onBoard(t, e, 0, ignoreLegendTwin)
	id1 := onBoard(t, e, 1, ignoreLegendTwin)
	id2 := onBoard(t, e, 1, ignoreLegendTwin)
	assertLegendTwinPair(t, e, 0, "Ignore Legend Twin", c1, c2)
	assertLegendTwinPair(t, e, 1, "Ignore Legend Twin", id1, id2)
	resolveEchoPermission(t, e, source)
	exempt := e.legendExemptions()
	for _, id := range []state.ObjID{c1, c2} {
		if !e.legendRuleExempt(exempt, id) {
			t.Fatalf("fixture: Permanent.YouCtrl did not exempt seat 0's permanent %d", id)
		}
	}
	for _, id := range []state.ObjID{id1, id2} {
		if e.legendRuleExempt(exempt, id) {
			t.Fatalf("fixture: seat 0's Permanent.YouCtrl exempted seat 1's permanent %d", id)
		}
	}

	e.checkStateBased()
	d := legendPending(t, e, 1, id1, id2)
	submitKeep(t, e, d, 1)
	if o := e.G.Obj(id2); o.Zone != state.ZBattlefield {
		t.Fatalf("the kept opponent creature left the battlefield (zone %v)", o.Zone)
	}
	if o := e.G.Obj(id1); o.Zone != state.ZGraveyard {
		t.Fatalf("the unchosen opponent creature is in %v, want its owner's graveyard", o.Zone)
	}
	for _, id := range []state.ObjID{c1, c2} {
		if o := e.G.Obj(id); o.Zone != state.ZBattlefield {
			t.Fatalf("the exempt seat-0 creature %d is in %v, want on the battlefield", id, o.Zone)
		}
	}
}
