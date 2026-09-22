// The Effect-created trigger grant's transport, end to end on real corpus
// carriers (round-2 review: the first cut of this file bypassed every real
// path -- it hand-built an effects.EffectFrame and called effects.Resolve
// directly, so none of the transport it pinned was exercised).
//
// Three closed paths, one test each:
//
//  1. Palace Jailer (EffectOwner$ TargetedOwner, TriggerZones$ Command): the
//     ETB resolution registers the effect-owned ComeBack trigger; a later
//     MonarchChange queues it through checkGrantedStaticTriggersUsing's real
//     effect arm (zone identity + owner controller), it fires through the
//     stack (GrantTriggerPush), its body returns the exiled creature, and the
//     chained self-exile idiom ends the registration via the frame.
//  2. Valiant Batrider (EffectOwner$ TriggeredTarget, no TriggerZones$): the
//     damaged player owns the boon; the boon's SpellCast trigger fires on the
//     OWNER's cast (the identity's controller read), queues through the real
//     walk, and its UnlessCost ask goes to the owner -- not to the resolving
//     permanent's controller.
//  3. The replacement-body-suspends-and-resumes path (no live corpus carrier
//     has a decision inside an Effect-created replacement body -- measured 0,
//     so the fixture is an authored gatekeeper in the wildgrowth1 house
//     style, mirroring Klement, Life Acolyte's exact SVar chain): the body's
//     unless ask suspends, the resume re-binds the frame off the decision's
//     resume state, and the chained self-exile then ends the registration.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// countersOf returns how many counters of kind the object carries.
func countersOf(e *Engine, id state.ObjID, kind string) int {
	n := 0
	for _, c := range e.G.Obj(id).Counters {
		if c.Kind == kind {
			n += int(c.N)
		}
	}
	return n
}

// effectTriggerRegistrations returns the live Effect-created trigger grants
// whose source is id.
func effectTriggerRegistrations(e *Engine, id state.ObjID) []*state.ContinuousEffect {
	var out []*state.ContinuousEffect
	for i := range e.continuous {
		ce := &e.continuous[i]
		if ce.Source == id && ce.AddTrigger != nil {
			out = append(out, ce)
		}
	}
	return out
}

// effectRegistrations returns the live api:Effect registrations (trigger
// grants or live replacements) whose source is id.
func effectRegistrations(e *Engine, id state.ObjID) []*state.ContinuousEffect {
	var out []*state.ContinuousEffect
	for i := range e.continuous {
		ce := &e.continuous[i]
		if ce.Source == id && (ce.AddTrigger != nil || ce.ReplacementEvent != "") {
			out = append(out, ce)
		}
	}
	return out
}

func TestEffectOwnedTriggerJailerReturnsAndEndsOnMonarchChange(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	jailer := mustCorpusCard(t, reg, "Palace Jailer")
	bear := mustCorpusCard(t, reg, "Grizzly Bears")
	e := newSeats(t, 2)
	e.pending = nil

	// Seat 1's creature on the battlefield (the exile target), the Jailer in
	// seat 0's hand.
	b := e.G.AddObject(bear, 1)
	b.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{b.ID})
	j := e.G.AddObject(jailer, 0)
	j.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, []state.ObjID{j.ID})

	// The Jailer enters: both ETB triggers fire through the real face walk.
	// TrigExile asks its target at placement; TrigMonarch's designation
	// resolves with no registration in place yet (LIFO: the exile trigger
	// resolves first), so nothing returns early.
	e.emit(events.Event{Kind: events.MoveZone, Obj: j.ID, From: state.ZHand, To: state.ZBattlefield})
	e.priorityRound()
	d := e.Pending()
	if d != nil && d.Kind == decision.KTriggerOrder {
		// The two ETB triggers are simultaneous: the order ask decides which
		// resolves last. Put the EXILE trigger on the stack first (chosen
		// first), so the monarch designation resolves BEFORE the effect is
		// registered and cannot fire the ComeBack early.
		submitChoices(t, e, 1, 0)
		d = e.Pending()
	}
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("precondition: expected the Jailer exile target ask, got %+v", d)
	}
	bearIdx := -1
	for _, o := range d.Options {
		if o.Obj == b.ID {
			bearIdx = o.Index
		}
	}
	if bearIdx < 0 {
		t.Fatalf("precondition: seat 1's creature not offered: %+v", d.Options)
	}
	submitChoices(t, e, bearIdx)
	passUntilStackEmpty(t, e, 30)
	if e.G.Obj(b.ID).Zone != state.ZExile {
		t.Fatalf("precondition: the targeted creature zone = %s, want Exile", e.G.Obj(b.ID).Zone)
	}
	if e.G.Monarch != 0 {
		t.Fatalf("precondition: monarch = %d, want seat 0 (the Jailer's own designation)", e.G.Monarch)
	}
	grants := effectTriggerRegistrations(e, j.ID)
	if len(grants) != 1 {
		t.Fatalf("precondition: %d Effect trigger registrations on the Jailer, want 1", len(grants))
	}
	g := grants[0]
	if !g.EffectGrant {
		t.Fatal("precondition: the registered trigger grant is not marked EffectGrant")
	}
	if g.EffectOwnerPlayer != 1 {
		t.Fatalf("precondition: EffectOwnerPlayer = %d, want 1 (the exiled creature's owner)", g.EffectOwnerPlayer)
	}
	if g.AddTrigger.Mode != "BecomeMonarch" {
		t.Fatalf("precondition: granted trigger mode = %s, want BecomeMonarch", g.AddTrigger.Mode)
	}
	if len(g.Remembered) != 2 {
		t.Fatalf("precondition: the effect remembered %d objects, want the Jailer and the creature", len(g.Remembered))
	}

	// An opponent becomes the monarch: the ComeBack trigger queues through
	// the real effect arm -- the registration's source is the battlefield
	// Jailer, but the trigger lives on the Command-zone effect object, so
	// only the zone identity lets TriggerZones$ Command pass.
	e.emit(events.Event{Kind: events.MonarchChange, Player: 1})
	e.priorityRound()
	passUntilStackEmpty(t, e, 30)

	if !hasEventKind(e, events.GrantTriggerPush) {
		t.Fatal("the ComeBack trigger never reached the stack (no GrantTriggerPush)")
	}
	if z := e.G.Obj(b.ID).Zone; z != state.ZBattlefield {
		t.Fatalf("the exiled creature was never returned: zone = %s", z)
	}
	if c := e.G.Obj(b.ID).Controller; c != 1 {
		t.Fatalf("returned creature controller = %d, want its owner (seat 1)", c)
	}
	if n := len(effectTriggerRegistrations(e, j.ID)); n != 0 {
		t.Fatalf("one-shot Effect trigger registration survived the self-exile: %d left", n)
	}
}

func TestEffectOwnerTriggeredTargetBoonFiresOnTheOwnersCast(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	batrider := mustCorpusCard(t, reg, "Valiant Batrider")
	e := newSeats(t, 2)
	e.pending = nil
	br := e.G.AddObject(batrider, 0)
	br.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{br.ID})

	// The Batrider deals combat damage to seat 1: the printed DamageDone
	// trigger resolves TrigBoon, and the effect must be owned by the DAMAGED
	// player (EffectOwner$ TriggeredTarget), not by the Batrider's controller.
	e.damaging = br.ID
	e.combatDamaging = true
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
	e.combatDamaging = false
	e.damaging = 0
	e.priorityRound()
	passUntilStackEmpty(t, e, 30)
	grants := effectTriggerRegistrations(e, br.ID)
	if len(grants) != 1 {
		t.Fatalf("precondition: %d Effect trigger registrations on the Batrider, want 1", len(grants))
	}
	if grants[0].EffectOwnerPlayer != 1 {
		t.Fatalf("precondition: EffectOwnerPlayer = %d, want 1 (the damaged player)", grants[0].EffectOwnerPlayer)
	}
	if grants[0].AddTrigger.Mode != "SpellCast" {
		t.Fatalf("precondition: granted trigger mode = %s, want SpellCast", grants[0].AddTrigger.Mode)
	}

	// The boon's owner casts a noncreature spell: the SpellCast trigger must
	// fire on seat 1's cast (ValidActivatingPlayer$ You reads the effect
	// owner), not on the Batrider controller's. The spell is an instant, so
	// no timing setup is needed.
	instant := card(t, "Name:Slow Ritual\nManaCost:0\nTypes:Instant\nA:SP$ Draw | Defined$ You\nOracle:x\n")
	s := e.G.AddObject(instant, 1)
	s.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 1, []state.ObjID{s.ID})
	hand0 := len(e.G.Zone(state.ZHand, 0)) // seat 0 does not cast here
	e.G.Active, e.G.Priority = 1, 1
	e.askPriority(1)
	submitChoices(t, e, passToCast(t, e, s.ID))

	// The trigger's UnlessCost ask goes to the effect OWNER (seat 1): the
	// payer declines from an empty pool, and each opponent of the owner
	// (seat 0) draws one.
	e.priorityRound()
	pay := drainUntilUnlessPay(t, e, 40)
	if pay == nil {
		t.Fatal("the boon's pay-{1} ask was never posed")
	}
	if pay.Player != 1 {
		t.Fatalf("unless payer = seat %d, want the effect owner (seat 1), not the Batrider's controller", pay.Player)
	}
	submitChoices(t, e, pay.Options[len(pay.Options)-1].Index) // decline
	passUntilStackEmpty(t, e, 30)
	if n := len(e.G.Zone(state.ZHand, 0)); n != hand0+1 {
		t.Fatalf("seat 0's hand = %d, want %d after the declined boon drew one: the owner-relative Defined$ Opponent misread", n, hand0+1)
	}
	if n := len(effectTriggerRegistrations(e, br.ID)); n != 1 {
		t.Fatalf("the boon registration %d after its first use, want 1 (Duration$ Permanent)", n)
	}
}

// resumeProbeSrc mirrors Klement, Life Acolyte's exact SVar chain (the
// corpus's Effect -> ReplacementEffects$ ETB-counter -> self-exile carrier)
// with the one change under test: the replacement body's sub chain carries a
// mid-resolution unless ask BEFORE the self-exile, so the registration can
// only end if the resume re-binds the frame off the decision's resume state.
// No live corpus carrier has a decision inside an Effect-created replacement
// body (measured over the corpus pin: 0 Effect bodies with ReplacementEffects$
// carry any ask-shaped parameter), so the fixture is authored in the
// wildgrowth1 house style: a real Forge script shape, inline, never a
// .cards/ .txt.
const resumeProbeSrc = "Name:Resume Probe\nManaCost:1 G\nTypes:Creature Human Soldier\nPT:2/2\n" +
	"T:Mode$ SpellCast | ValidCard$ Creature | ValidActivatingPlayer$ You | TriggerZones$ Battlefield | Execute$ TrigEffect | TriggerDescription$ Whenever you cast a creature spell, the next time it or a remembered card enters, put a +1/+1 counter on it, then you may pay {1}. If you don't, draw a card, and the effect ends.\n" +
	"SVar:TrigEffect:DB$ Effect | RememberObjects$ TriggeredCard | ReplacementEffects$ ETBCreat | Duration$ Permanent\n" +
	"SVar:ETBCreat:Event$ Moved | ValidCard$ Card.IsRemembered | Destination$ Battlefield | ReplaceWith$ DBPutAndAsk | ReplacementResult$ Updated\n" +
	"SVar:DBPutAndAsk:DB$ PutCounter | Defined$ ReplacedCard | CounterType$ P1P1 | ETB$ True | CounterNum$ 1 | SubAbility$ DBAskThenExile\n" +
	"SVar:DBAskThenExile:DB$ Draw | Defined$ You | UnlessCost$ {1} | NumCards$ 1 | SubAbility$ ExileEffect\n" +
	"SVar:ExileEffect:DB$ ChangeZone | Defined$ Self | Origin$ Command | Destination$ Exile\n" +
	"Oracle:x\n"

const resumeWolfSrc = "Name:Fixture Wolf\nManaCost:0\nTypes:Creature Wolf\nPT:2/2\nOracle:x\n"

func TestEffectReplacementBodySuspendsAndResumesWithTheFrame(t *testing.T) {
	probe := card(t, resumeProbeSrc)
	wolf := card(t, resumeWolfSrc)
	e := newSeats(t, 2)
	e.pending = nil
	pr := e.G.AddObject(probe, 0)
	pr.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{pr.ID})
	w := e.G.AddObject(wolf, 0)
	w.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, []state.ObjID{w.ID})
	hand0 := len(e.G.Zone(state.ZHand, 0)) - 1 // the spell leaves on the cast

	// Seat 0 casts the Wolf: the SpellCast trigger resolves TrigEffect, which
	// registers the effect remembering the cast spell.
	e.G.Active, e.G.Priority = 0, 0
	e.askPriority(0)
	submitChoices(t, e, passToCast(t, e, w.ID))
	e.priorityRound()
	passUntilStackEmpty(t, e, 30)
	grants := effectRegistrations(e, pr.ID)
	if len(grants) != 1 {
		t.Fatalf("precondition: %d Effect registrations on the probe, want 1", len(grants))
	}
	if grants[0].ReplacementEvent != "Moved" || grants[0].ReplacementBody == "" {
		t.Fatalf("precondition: the effect's replacement is not live: %+v", grants[0])
	}

	// The spell resolves and enters: the live replacement applies the move,
	// runs the body's PutCounter, and the sub's unless ask suspends
	// mid-replacement.
	d := e.Pending()
	if d == nil || d.Kind != decision.KModes || d.ResumeKind != "unless_pay" {
		t.Fatalf("the replacement body's pay-{1} ask was never posed (the body did not suspend): %+v", d)
	}
	submitChoices(t, e, d.Options[len(d.Options)-1].Index) // decline
	passUntilStackEmpty(t, e, 30)

	if e.G.Obj(w.ID).Zone != state.ZBattlefield {
		t.Fatalf("precondition: the creature zone = %s, want Battlefield (the replacement applied the move)", e.G.Obj(w.ID).Zone)
	}
	if n := countersOf(e, w.ID, "P1P1"); n != 1 {
		t.Fatalf("the body's PutCounter placed %d counters, want 1", n)
	}
	if n := len(e.G.Zone(state.ZHand, 0)); n != hand0+1 {
		t.Fatalf("the declined unless drew %d card(s) (hand %d, want %d): the body did not run after the resume", n-hand0, n, hand0+1)
	}
	if n := len(effectRegistrations(e, pr.ID)); n != 0 {
		t.Fatalf("the resumed self-exile did not end the registration: %d left -- the frame was not carried across the resume", n)
	}
}
