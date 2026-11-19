package rules

// The EDH deck-census replacement classes (cmd/forgec/edh_replacements_test.go
// pins the registry side): behaviour tests that play the printed text
// through a real game on the REAL corpus cards.
//
//   - Tekuthal, Inquiry Dominus (R:Event$ Proliferate): one proliferate
//     becomes two, each a full action with its own choice round and its own
//     completed marker (CR 701.34a's per-action shape).
//   - Fear of Sleep Paralysis (R:Event$ RemoveCounter, Layer$ CantHappen):
//     a stun counter on an opponent's permanent is not removed when the
//     permanent would untap (CR 122.1d's substitution is itself blocked),
//     while the same lock never touches the controller's own permanents.
//
// Cascade (Averna, the Chaos Bloom, R:Event$ Cascade) is registered beside
// the machinery the existing averna_cascade_test.go already drives end to
// end; no new behaviour test belongs here.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEDHReplacementEventsProliferateDoubles is Tekuthal's leaf: with it on
// the battlefield, Karn's Bastion's one proliferate poses TWO choice rounds
// and the carrier chosen in both takes +1 of each kind from each (1 -> 3).
// The registered key without this behaviour would be worse than
// unsupported, so the precondition asserts the R: line and the battlefield
// source explicitly.
func TestEDHReplacementEventsProliferateDoubles(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := proliferateEngine(t, reg, "Karn's Bastion", "Tekuthal, Inquiry Dominus", "Grizzly Bears")
	tek := putNamedOnBattlefield(t, e, "Tekuthal, Inquiry Dominus")
	karn := searchMoveByName(t, e, "Karn's Bastion", state.ZBattlefield)
	putNamedOnBattlefield(t, e, "Grizzly Bears")
	carrier := putCountersOn(t, e, 0, "Grizzly Bears", "P1P1", 1)

	// Precondition: the replacement source really is the battlefield
	// Tekuthal and really carries an R: line whose Event$ is Proliferate
	// scoped to the controller (ValidPlayer$ You).
	teko := e.G.Obj(tek)
	if teko == nil || teko.Zone != state.ZBattlefield || teko.Controller != 0 {
		t.Fatalf("precondition: Tekuthal is not on seat 0's battlefield: %+v", teko)
	}
	found := false
	for i := range teko.Face().Repls {
		r := &teko.Face().Repls[i]
		if r.Event == "Proliferate" && r.ParamStr(cards.PKValidPlayer) == "You" {
			found = true
		}
	}
	if !found {
		t.Fatal("precondition: Tekuthal carries no R:Event$ Proliferate line")
	}
	markersBefore := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.Proliferate {
			markersBefore++
		}
	}

	// Karn's Bastion prints two ABs: index 0 is the mana ability, index 1 is
	// the {4},{T} proliferate (the TestKarnsBastionActivationProliferates
	// precedent).
	addMana(t, e, 0, "CCCC")
	opt, ok := findAbilityOption(e, karn, 1)
	if !ok {
		t.Fatalf("Karn's Bastion proliferate ability not offered: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	d := passUntilNonPriority(t, e, 60)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "proliferate" {
		t.Fatalf("decision = %+v, want the first proliferate ask", d)
	}
	carrierOpt := -1
	for i, o := range d.Options {
		if o.Obj == carrier {
			carrierOpt = i
		}
	}
	if carrierOpt < 0 {
		t.Fatalf("carrier not offered: %+v", d.Options)
	}
	if got := e.G.Obj(carrier).Counter("P1P1"); got != 1 {
		t.Fatalf("precondition broken at the first ask: carrier P1P1 = %d, want 1", got)
	}
	d = answerProliferate(t, e, d, carrierOpt)
	if d == nil || d.ResumeKind != "proliferate" {
		t.Fatalf("decision = %+v, want the second proliferate ask (Tekuthal's double)", d)
	}
	if got := e.G.Obj(carrier).Counter("P1P1"); got != 2 {
		t.Fatalf("carrier P1P1 = %d after the first proliferation, want 2 (+1)", got)
	}
	answerProliferate(t, e, d, carrierOpt)
	if got := e.G.Obj(carrier).Counter("P1P1"); got != 3 {
		t.Fatalf("carrier P1P1 = %d, want 3 (one proliferate doubled by Tekuthal)", got)
	}
	// Two actions, two completed markers.
	markersAfter := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.Proliferate {
			markersAfter++
		}
	}
	if markersAfter-markersBefore != 2 {
		t.Fatalf("the doubled proliferate completed %d marker(s), want 2", markersAfter-markersBefore)
	}
	replayCheck(t, e, cfg)
}

// TestEDHReplacementEventsStunCounterNotRemoved is Fear's leaf: an untap
// that would remove an opponent's stun counter (CR 122.1d) removes nothing
// and untaps nothing, while the same lock never touches Fear's controller's
// own stunned permanent -- the control case is the live baseline the
// blocking case is measured against.
func TestEDHReplacementEventsStunCounterNotRemoved(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	bear := card(t, bearSrc)
	e, cfg := corpusEngineCfg(t, reg,
		[]*cards.Card{lookup(t, reg, "Fear of Sleep Paralysis"), bear},
		[]*cards.Card{bear})
	fear := moveByName(t, e, 0, "Fear of Sleep Paralysis", state.ZBattlefield)
	own := moveByName(t, e, 0, "Bear", state.ZBattlefield)
	theirs := moveByName(t, e, 1, "Bear", state.ZBattlefield)

	// Precondition: the source really is the battlefield Fear and really
	// carries the Layer$ CantHappen R: line.
	fo := e.G.Obj(fear)
	if fo == nil || fo.Zone != state.ZBattlefield || fo.Controller != 0 {
		t.Fatalf("precondition: Fear is not on seat 0's battlefield: %+v", fo)
	}
	found := false
	for i := range fo.Face().Repls {
		r := &fo.Face().Repls[i]
		if r.Event == "RemoveCounter" && r.ParamStr(cards.PKLayer) == "CantHappen" &&
			r.ParamStr(cards.PKValidCounterType) == "STUN" {
			found = true
		}
	}
	if !found {
		t.Fatal("precondition: Fear carries no R:Event$ RemoveCounter CantHappen line")
	}
	tapStun := func(id state.ObjID) {
		e.emit(events.Event{Kind: events.Tap, Obj: id})
		e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "STUN", Amount: 1})
	}
	tapStun(own)
	tapStun(theirs)
	for _, id := range []state.ObjID{own, theirs} {
		o := e.G.Obj(id)
		if o == nil || !o.Tapped || o.Counter("STUN") != 1 {
			t.Fatalf("precondition: fixture %d is not a tapped one-stun permanent: %+v", id, o)
		}
	}

	// Control arm: the controller's OWN stunned permanent untaps and the
	// stun counter comes off (CR 122.1d unblocked). This is the baseline the
	// blocked arm must differ from. The substitution removes the counter
	// first; the untap it stood in for happens on the next untap attempt,
	// exactly as the untap step re-runs.
	effects.TryUntap(e, own)
	if o := e.G.Obj(own); !o.Tapped || o.Counter("STUN") != 0 {
		t.Fatalf("control arm first attempt: tapped=%v stun=%d, want still tapped with the stun counter removed", o.Tapped, o.Counter("STUN"))
	}
	effects.TryUntap(e, own)
	o := e.G.Obj(own)
	if o.Tapped || o.Counter("STUN") != 0 {
		t.Fatalf("control arm: own bear tapped=%v stun=%d, want untapped with the stun removed", o.Tapped, o.Counter("STUN"))
	}

	// Blocked arm: the opponent's stunned permanent keeps both.
	effects.TryUntap(e, theirs)
	o = e.G.Obj(theirs)
	if !o.Tapped || o.Counter("STUN") != 1 {
		t.Fatalf("blocked arm: opponent bear tapped=%v stun=%d, want tapped with the stun kept", o.Tapped, o.Counter("STUN"))
	}
	sawPrevent := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && ev.Text == "prevented: counter removal cannot happen" {
			sawPrevent = true
		}
	}
	if !sawPrevent {
		t.Fatal("the blocked arm logged no prevented-removal Note")
	}
	replayCheck(t, e, cfg)
}
