package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 113.7a: a triggered ability exists apart from its source. A trigger's own
// Sac<X> cost reads the SVar:X of the ability, so a source that leaves with the
// trigger on the stack -- a transformed card now front face up in the
// graveyard, a permanent that was a copy -- still lets the payer announce X,
// sacrifice X and draw X. Inline synthetic fixtures only.

// sacXDFCScript's front face transforms as it enters and defines an unrelated
// SVar:X; the back face carries the Sac<X> trigger, fired by another creature
// entering.
const sacXDFCScript = "Name:SacX Carrier\nManaCost:B\nTypes:Creature Human\nPT:2/2\n" +
	"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ CarrierFlip\n" +
	"SVar:CarrierFlip:DB$ SetState | Mode$ Transform | Defined$ Self\n" +
	"SVar:X:Count$ResolvedThisTurn\n" +
	"AlternateMode:DoubleFaced\nOracle:x\n" +
	"\nALTERNATE\n\n" +
	"Name:SacX Carrier Angel\nManaCost:no cost\nColors:black\nTypes:Creature Angel\nPT:3/3\n" +
	"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Creature.Other | TriggerZones$ Battlefield | Execute$ CarrierDraw\n" +
	"SVar:CarrierDraw:AB$ Draw | NumCards$ X | Cost$ Sac<X/Creature.Other/another creature>\n" +
	"SVar:X:Count$xPaid\nOracle:x\n"

const (
	sacXDepartedBearA = "Name:SacX Departed Bear A\nManaCost:1\nTypes:Creature Bear\nPT:1/1\nOracle:x\n"
	sacXDepartedBearB = "Name:SacX Departed Bear B\nManaCost:1\nTypes:Creature Bear\nPT:1/1\nOracle:x\n"
)

// payDepartedSacX asserts the X ask offers 0..2, pays X = 2 and asserts both
// bears were sacrificed and two cards drawn.
func payDepartedSacX(t *testing.T, e *Engine, cfg *Config, bears []state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected the trigger-cost X ask, got %+v", d)
	}
	if len(d.Options) != 3 {
		t.Fatalf("X ask options = %+v, want X = 0..2", d.Options)
	}
	hand := len(e.G.Zone(state.ZHand, 0))
	paySacX(t, e, d, 2, bears...)
	drainEvidenceXTriggers(t, e)
	for _, id := range bears {
		if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
			t.Fatalf("creature %d in %s, want graveyard (sacrificed)", id, z)
		}
	}
	if got := len(e.G.Zone(state.ZHand, 0)) - hand; got != 2 {
		t.Fatalf("cards drawn = %d, want 2", got)
	}
	if cfg != nil {
		replayCheck(t, e, *cfg)
	}
}

func TestTriggerSacXDepartedTransformedSource(t *testing.T) {
	t.Parallel()
	e, cfg, src := newFixtureDeck(t, 1, sacXDFCScript, sacXDepartedBearA, sacXDepartedBearB)
	e.pending = nil
	bears := []state.ObjID{
		namedCardInZones(t, e, 0, "SacX Departed Bear A", state.ZHand, state.ZLibrary),
		namedCardInZones(t, e, 0, "SacX Departed Bear B", state.ZHand, state.ZLibrary),
	}
	// One bear first, so the front face's ETB flip resolves with nothing else
	// to trigger; the second bear then fires the back face's trigger.
	e.emit(events.Event{Kind: events.MoveZone, Obj: bears[0], From: e.G.Obj(bears[0]).Zone, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.MoveZone, Obj: src, From: e.G.Obj(src).Zone, To: state.ZBattlefield})
	e.putTriggersOnStack()
	e.resolveTop()
	if name := e.G.Obj(src).Face().Name; name != "SacX Carrier Angel" {
		t.Fatalf("source face = %q, want the back face after its ETB transform", name)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: bears[1], From: e.G.Obj(bears[1]).Zone, To: state.ZBattlefield})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack = %v, want the back face's trigger alone", e.G.Stack)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: src, From: state.ZBattlefield, To: state.ZGraveyard})
	// The precondition: the departed card shows its front face, whose SVar:X
	// is not the ability's.
	if body := e.G.Obj(src).Face().SVars["X"]; body != "Count$ResolvedThisTurn" {
		t.Fatalf("departed source SVar:X = %q, want the front face's", body)
	}
	e.resolveTop()
	payDepartedSacX(t, e, &cfg, bears)
}

// sacXModelScript is a one-faced carrier of the same trigger, for a permanent
// to become a copy of.
const sacXModelScript = "Name:SacX Model\nManaCost:B\nTypes:Creature Human\nPT:2/2\n" +
	"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Creature.Other | TriggerZones$ Battlefield | Execute$ ModelDraw\n" +
	"SVar:ModelDraw:AB$ Draw | NumCards$ X | Cost$ Sac<X/Creature.Other/another creature>\n" +
	"SVar:X:Count$xPaid\nOracle:x\n"

// TestTriggerSacXDepartedCopySource: a permanent that was a copy of the Sac<X>
// carrier leaves with the trigger on the stack and reverts to its own card,
// which has no SVar:X at all.
func TestTriggerSacXDepartedCopySource(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, sacXDepartedBearB))
	src := copyOnBoard(t, e, sacXModelScript)
	// Only the copy is left to trigger.
	for _, id := range append([]state.ObjID(nil), e.G.Zone(state.ZBattlefield, 1)...) {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZExile})
	}
	bears := []state.ObjID{onBoard(t, e, 0, sacXDepartedBearA), e.G.Zone(state.ZHand, 0)[0]}
	e.pending, e.pendingTriggers = nil, nil
	e.emit(events.Event{Kind: events.MoveZone, Obj: bears[1], From: state.ZHand, To: state.ZBattlefield})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 || e.G.Obj(e.G.Stack[0]).Source != src {
		t.Fatalf("stack = %v, want the copy's trigger alone", e.G.Stack)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: src, From: state.ZBattlefield, To: state.ZGraveyard})
	if body, ok := e.G.Obj(src).Face().SVars["X"]; ok {
		t.Fatalf("departed copy still defines SVar:X = %q", body)
	}
	e.resolveTop()
	payDepartedSacX(t, e, nil, bears)
}
