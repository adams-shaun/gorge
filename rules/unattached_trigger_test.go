package rules

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// The "whenever CARDNAME becomes unattached from a permanent" trigger class
// (Mode$ Unattached, CR 701.3b), pinned end to end on the real corpus card
// Grafted Exoskeleton. The trigger fires off the events.Unattached event
// rules/attach.go's attachmentSBAs emits when the bearer leaves the
// battlefield, and the body's Defined$ TriggeredObjectLKICopy referent must
// resolve to the FORMER BEARER -- never to the Equipment itself or any other
// permanent.

const unattachedBystanderSrc = "Name:Bystander Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// equipOn equips the Equipment onto the named battlefield creature for seat 0
// through the real Equip activation. Every caller asserts the resulting
// attachment itself, so a setup that silently failed to equip cannot reach an
// assertion.
func equipOn(t *testing.T, e *Engine, equip, bear state.ObjID) {
	t.Helper()
	addMana(t, e, 0, "CC")
	submitChoices(t, e, abilityOption(t, e, equip, 0).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("equip target decision: %+v", d)
	}
	idx := indexOfObjOption(d, bear)
	if idx < 0 {
		t.Fatalf("equip ask does not offer the bear: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 20)
}

// TestGraftedExoskeletonFiresWhenTheBearerLeavesAndSkipsTheGoneLKI is the
// filing card, end to end: Grafted Exoskeleton equips a creature, the
// creature leaves the battlefield, attachmentSBAs emits events.Unattached,
// the trigger fires, and its SacrificeAll body resolves
// Defined$ TriggeredObjectLKICopy to the FORMER BEARER -- which is already in
// the graveyard, so the sacrifice skips it LOUDLY (one Note naming the bear)
// instead of sacrificing the Equipment or any other permanent.
func TestGraftedExoskeletonFiresWhenTheBearerLeavesAndSkipsTheGoneLKI(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	exo := mustCorpusCard(t, reg, "Grafted Exoskeleton")
	bearCard := card(t, attachedBearSrc)
	e, cfg := tokenReplGame(t, 111, exo, bearCard)
	exoID := moveSeededCard(t, e, 0, exo, state.ZBattlefield)
	bear := moveSeededCard(t, e, 0, bearCard, state.ZBattlefield)
	passUntilStackEmpty(t, e, 20)
	equipOn(t, e, exoID, bear)

	// Precondition: the Equipment really is attached to the bear before the
	// detach is provoked; a setup that never attached would make every
	// assertion below vacuous.
	if got := e.G.Obj(exoID).AttachedTo; got != bear {
		t.Fatalf("precondition failed: Grafted Exoskeleton attached to %d, want bear %d", got, bear)
	}
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZBattlefield || o.Face().Name != "Bear" {
		t.Fatalf("precondition failed: bearer %+v, want the real Bear on the battlefield", o)
	}

	// The bearer leaves the battlefield (the real MoveZone every removal
	// path emits). attachmentSBAs then detaches the Equipment without the
	// Equipment leaving the battlefield.
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear,
		From: state.ZBattlefield, To: state.ZGraveyard})
	e.checkStateBased()
	e.pending = nil
	e.Advance()
	passUntilStackEmpty(t, e, 30)

	// The Equipment stayed on the battlefield, detached.
	if o := e.G.Obj(exoID); o == nil || o.Zone != state.ZBattlefield || o.AttachedTo != 0 {
		t.Fatalf("Equipment %+v, want battlefield and detached", o)
	}
	// The bearer really left (the "gone LKI" premise).
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("bear zone %v, want graveyard (the sacrifice must be a gone LKI copy)", o.Zone)
	}

	// The detach published a real events.Unattached naming the former bearer.
	var detach *events.Event
	for i := range e.L.Events {
		ev := &e.L.Events[i]
		if ev.Kind == events.Unattached && ev.Obj == exoID {
			detach = ev
		}
	}
	if detach == nil {
		t.Fatal("Grafted Exoskeleton left the battlefield detached but no events.Unattached was emitted")
	}
	if len(detach.IDs) == 0 || detach.IDs[0] != bear {
		t.Fatalf("events.Unattached former bearer = %v, want [%d]", detach.IDs, bear)
	}

	// The trigger's body ran: SacrificeAll's Defined$ TriggeredObjectLKICopy
	// resolved to the former bear, which is off the battlefield, so it was
	// skipped with one loud Note naming that bear.
	foundReferent := false
	for i := range e.L.Events {
		ev := &e.L.Events[i]
		if ev.Kind != events.Note || !strings.Contains(ev.Text, "SacrificeAll target") {
			continue
		}
		if !strings.Contains(ev.Text, "not on the battlefield") {
			t.Fatalf("SacrificeAll Note did not name an off-battlefield skip: %q", ev.Text)
		}
		if !strings.Contains(ev.Text, "target "+idString(bear)+" ") {
			t.Fatalf("SacrificeAll skip Note %q does not name the former bear %d -- the referent resolved to a wrong permanent", ev.Text, bear)
		}
		foundReferent = true
	}
	if !foundReferent {
		t.Fatal("the Unattached trigger's SacrificeAll body never ran (no skip Note): the trigger did not fire or the body resolved to nothing")
	}

	// Nothing was actually sacrificed: no Sacrifice marker names the
	// Equipment or any other permanent.
	for _, ev := range e.L.Events {
		if events.IsSacrifice(ev) && ev.Obj != bear {
			t.Fatalf("a wrong permanent %d was sacrificed by the Unattached trigger", ev.Obj)
		}
	}
	if o := e.G.Obj(exoID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatal("Grafted Exoskeleton was sacrificed by its own trigger")
	}
	replayCheck(t, e, cfg)
}

// TestUnattachedLiveBearerIsSacrificed pins the other half of the referent:
// when the bearer is STILL on the battlefield at detach time, the trigger's
// Defined$ TriggeredObjectLKICopy sacrifices it for real. The detach event is
// the same shape attachmentSBAs emits (Obj = the Equipment, IDs[0] = the
// former bearer); driving it directly keeps this test about the referent and
// the live sacrifice, not about which SBA arm produced it (the real SBA arm
// is pinned by the bearer-left test above).
func TestUnattachedLiveBearerIsSacrificed(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	exo := mustCorpusCard(t, reg, "Grafted Exoskeleton")
	bearCard := card(t, attachedBearSrc)
	bystanderCard := card(t, unattachedBystanderSrc)
	e, cfg := tokenReplGame(t, 112, exo, bearCard, bystanderCard)
	exoID := moveSeededCard(t, e, 0, exo, state.ZBattlefield)
	bear := moveSeededCard(t, e, 0, bearCard, state.ZBattlefield)
	other := moveSeededCard(t, e, 0, bystanderCard, state.ZBattlefield)
	passUntilStackEmpty(t, e, 20)
	equipOn(t, e, exoID, bear)

	if got := e.G.Obj(exoID).AttachedTo; got != bear {
		t.Fatalf("precondition failed: attached to %d, want bear %d", got, bear)
	}
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition failed: bear zone %v, want battlefield (the sacrifice must have a live target)", o.Zone)
	}

	// The detach with the bearer still alive on the battlefield -- the shape
	// the "bearer is no longer a creature" / bestowed SBA arms emit.
	e.emit(events.Event{Kind: events.Unattached, Obj: exoID, IDs: []state.ObjID{bear},
		Text: "test detach with a live bearer"})
	e.pending = nil
	e.Advance()
	passUntilStackEmpty(t, e, 30)

	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("live former bearer zone %v, want graveyard (the trigger must sacrifice it)", o.Zone)
	}
	sacrificed := false
	for _, ev := range e.L.Events {
		if events.IsSacrifice(ev) && ev.Obj == bear {
			sacrificed = true
		}
	}
	if !sacrificed {
		t.Fatal("no Sacrifice marker for the live former bearer -- the referent did not resolve to it")
	}
	if o := e.G.Obj(other); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("the bystander was sacrificed too: zone %v, want battlefield", o.Zone)
	}
	if o := e.G.Obj(exoID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("the Equipment was sacrificed instead of the bearer: zone %v", o.Zone)
	}
	replayCheck(t, e, cfg)
}

// idString renders an object id the way the effects Note does (%d).
func idString(id state.ObjID) string { return strconv.Itoa(int(id)) }
