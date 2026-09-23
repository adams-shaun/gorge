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

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
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

// TestIgnoreLegendRuleExemptionEndsWhenSourceLeaves: the exemption is live
// only while its source is. Moving Council of Reeds off the battlefield
// removes the static (asserted on the canonical walk) and the ordinary CR
// 704.5j choice returns for the pair it had been exempting.
func TestIgnoreLegendRuleExemptionEndsWhenSourceLeaves(t *testing.T) {
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
