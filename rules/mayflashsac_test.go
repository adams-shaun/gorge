package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// K:MayFlashSac (CR 702.8) is pinned end to end: the "you may cast this as
// though it had flash" permission (granted for free, with no additional cost
// -- unlike the adjacent K:MayFlashCost) and the keyword's own consequence,
// "if you cast it any time a sorcery couldn't have been cast, the controller
// of the permanent it becomes sacrifices it at the beginning of the next
// cleanup step". The core behaviour is pinned on a synthetic enchantment
// (below) and the filing card itself (Necromancy) at the bottom.

// mayflashsacEnchantSrc is the minimal carrier: an enchantment with the
// keyword, no ETB, no cost complications, so the tests measure only the
// permission and the cleanup rider.
const mayflashsacEnchantSrc = "Name:Flashsac Enchant\nManaCost:1 G\nTypes:Enchantment\nK:MayFlashSac\nOracle:x\n"

// mayflashsacTargetSrc is a creature card seeded to a graveyard as a legal
// target for Necromancy's ETB reanimation.
const mayflashsacTargetSrc = "Name:Flashsac Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// sawMayFlashSacFlag reports whether the log carries a pay-time CastInfo for
// id whose flags include state.FlagMayFlashSac.
func sawMayFlashSacFlag(e *Engine, id state.ObjID) bool {
	for _, ev := range e.L.Events {
		if ev.Kind == events.CastInfo && ev.Obj == id && events.FlagsFrom(ev.Counter)&state.FlagMayFlashSac != 0 {
			return true
		}
	}
	return false
}

// sawMayFlashSacCleanupRegister reports whether the log carries a
// DelayedRegister for id at exactly the cleanup step with the keyword's
// builtin SVar -- the registration the ETB hook emits.
func sawMayFlashSacCleanupRegister(e *Engine, id state.ObjID) bool {
	for _, ev := range e.L.Events {
		if ev.Kind == events.DelayedRegister && ev.Obj == id &&
			ev.Step == state.StepCleanup && ev.Counter == "__kwMayFlashSacrifice" {
			return true
		}
	}
	return false
}

// passUntilTarget answers "pass" priority decisions (for whichever seat
// holds them) until a KTarget decision appears, and returns it. A resolution
// that asks its target only once the spell is resolving needs the caster and
// every responder to pass first.
func passUntilTarget(t *testing.T, e *Engine, limit int) *decision.Decision {
	t.Helper()
	for i := 0; i < limit; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			d = e.Pending()
		}
		if d == nil {
			t.Fatal("no decision pending while waiting for a target ask")
		}
		if d.Kind == decision.KTarget {
			return d
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("decision %+v while waiting for a target ask", d)
		}
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("priority decision with no pass option: %+v", d)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
			t.Fatalf("submit pass: %v", err)
		}
	}
	t.Fatalf("no target ask within %d passes", limit)
	return nil
}

// resolveStackFor answers any target ask with preferred (when it is among the
// offered candidates) and passes every priority decision until the stack is
// empty. It bounds the Necromancy cast's resolution without assuming the
// reanimation asks a target (a lone graveyard candidate may be taken
// silently).
func resolveStackFor(t *testing.T, e *Engine, preferred state.ObjID, limit int) {
	t.Helper()
	for i := 0; i < limit && !e.G.Over && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			d = e.Pending()
		}
		if d == nil {
			t.Fatal("no decision while resolving")
		}
		switch d.Kind {
		case decision.KPriority:
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("priority decision with no pass option: %+v", d)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
				t.Fatalf("submit pass: %v", err)
			}
		case decision.KTarget:
			idx := indexOfObjOption(d, preferred)
			if idx < 0 {
				idx = d.Options[0].Index
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
				t.Fatalf("submit target: %v", err)
			}
		default:
			t.Fatalf("unexpected decision %+v while resolving", d)
		}
	}
}

// TestMayFlashSacSorceryCastIsNotSacrificed is the negative half: cast at
// NORMAL sorcery timing (main phase, empty stack), the ordinary cast is
// offered, no FlagMayFlashSac is stamped, no cleanup registration is made,
// and the permanent survives its own cleanup step.
func TestMayFlashSacSorceryCastIsNotSacrificed(t *testing.T) {
	e, cfg, id := newFixtureDeck(t, 501, mayflashsacEnchantSrc, mayflashsacTargetSrc)
	addMana(t, e, 0, "GC") // {1}{G}
	// Precondition: the cast is offered at plain sorcery timing (Mode "") --
	// without this an off-timing fixture would make the assertions vacuous.
	opt := plainCastOption(t, e, id)
	submitChoices(t, e, opt.Index)
	passUntilStackEmpty(t, e, 30)

	o := e.G.Obj(id)
	if o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: enchantment zone %s, want battlefield after the sorcery-speed cast", o.Zone)
	}
	if sawMayFlashSacFlag(e, id) {
		t.Fatal("a sorcery-timed cast must not stamp FlagMayFlashSac")
	}
	if sawMayFlashSacCleanupRegister(e, id) {
		t.Fatal("a sorcery-timed cast must register no cleanup sacrifice")
	}
	// Drive through this turn's cleanup into the next turn: the permanent
	// must still be on the battlefield (nothing sacrifices it).
	driveToStep(t, e, e.G.Turn+1, 1, state.StepMain1)
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield {
		t.Fatalf("enchantment zone %s after cleanup, want battlefield (sorcery-timed cast is not sacrificed)", o.Zone)
	}
	replayCheck(t, e, cfg)
}

// TestMayFlashSacInstantWindowCastSacrificedAtCleanup is the filing defect's
// fix leaf: in a priority window that is NOT seat 0's main phase (so a
// sorcery could not have been cast), the card is offered through its
// MayFlashSac permission at no extra cost, the off-sorcery provenance is
// stamped, the permanent's entry registers the cleanup-step delayed
// sacrifice, and the permanent is gone by the time the next turn begins.
func TestMayFlashSacInstantWindowCastSacrificedAtCleanup(t *testing.T) {
	e, cfg, id := newFixtureDeck(t, 502, mayflashsacEnchantSrc, mayflashsacTargetSrc)
	// Stop in begin combat: seat 0 still has priority there, but it is not a
	// main phase, so spellTimingOK withholds the card unless the MayFlashSac
	// permission grants the flash window.
	e.askPriority(0)
	driveToStep(t, e, e.G.Turn, 0, state.StepBeginCombat)

	// Precondition: the ordinary sorcery-speed offer is absent off-main --
	// otherwise the permission would be untested (this is exactly the shape
	// that failed before the fix).
	if plainCastOptionExists(e.Pending().Options, id) {
		t.Fatal("precondition: plain cast offered off-main; the permission is untested")
	}
	// Fund {1}{G} after the drive (mana empties across step boundaries).
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "G", Amount: 1})
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "C", Amount: 1})
	e.pending = nil
	e.askPriority(0)

	opt := plainCastOption(t, e, id)
	if opt.Mode != "" {
		t.Fatalf("MayFlashSac cast mode %q, want the ordinary cast (the permission carries no mode and no extra cost)", opt.Mode)
	}
	submitChoices(t, e, opt.Index)
	passUntilStackEmpty(t, e, 30)

	if o := e.G.Obj(id); o.Zone != state.ZBattlefield {
		t.Fatalf("enchantment zone %s, want battlefield after the off-sorcery cast", o.Zone)
	}
	if !sawMayFlashSacFlag(e, id) {
		t.Fatal("off-sorcery MayFlashSac cast did not stamp FlagMayFlashSac")
	}
	if !sawMayFlashSacCleanupRegister(e, id) {
		t.Fatal("off-sorcery MayFlashSac cast did not register the cleanup sacrifice")
	}

	// It must survive the rest of the turn (not sacrificed at end of turn)
	// and be gone once the next turn is underway.
	driveToStep(t, e, e.G.Turn, 0, state.StepEnd)
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield {
		t.Fatalf("enchantment zone %s at the end step, want battlefield (the rider fires at cleanup, not end of turn)", o.Zone)
	}
	driveToStep(t, e, e.G.Turn+1, 1, state.StepMain1)
	if o := e.G.Obj(id); o.Zone != state.ZGraveyard {
		t.Fatalf("enchantment zone %s after the cleanup, want graveyard (sacrificed at the next cleanup step)", o.Zone)
	}
	if len(e.G.Delayed) != 0 {
		t.Fatalf("MayFlashSac registration not consumed: %d", len(e.G.Delayed))
	}
	replayCheck(t, e, cfg)
}

// TestMayFlashSacIsRegistered pins the coverage census: kw:MayFlashSac is
// registered as a supported primitive, so the report's ratchet defends
// every carrier. Without the RegisterNonAPI call the coverage walk
// (cards/primitive.go's Primitives, which lists kw:MayFlashSac off the
// K: line) would still count the carriers unsupported.
func TestMayFlashSacIsRegistered(t *testing.T) {
	if !effects.Supported()["kw:MayFlashSac"] {
		t.Fatal("effects.Supported() does not report kw:MayFlashSac")
	}
	// The permission read and the coverage walk must agree about which cards
	// carry the keyword.
	src := "Name:Has It\nManaCost:1 G\nTypes:Enchantment\nK:MayFlashSac\nOracle:x\n"
	if !mayFlashSacFace(card(t, src).Faces[0]) {
		t.Fatal("mayFlashSacFace missed a printed K:MayFlashSac")
	}
	none := card(t, "Name:Has Not\nManaCost:1 G\nTypes:Enchantment\nOracle:x\n")
	if mayFlashSacFace(none.Faces[0]) {
		t.Fatal("mayFlashSacFace matched a card with no keyword")
	}
	if mayFlashSacFace(nil) {
		t.Fatal("mayFlashSacFace(nil) must be false")
	}
}

// TestNecromancyOffSorceryCastSacrificesItselfAtCleanup is the filing card's
// own corpus leaf: the real Necromancy script carries K:MayFlashSac and its
// reanimation body. Cast off-sorcery it resolves (reanimating the graveyard
// creature and becoming an Aura), then the keyword's rider sacrifices it at
// the next cleanup step.
func TestNecromancyOffSorceryCastSacrificesItselfAtCleanup(t *testing.T) {
	necro := corpusCardText(t, "n/necromancy.txt")
	if !mayFlashSacFace(card(t, necro).Faces[0]) {
		t.Fatal("setup: the corpus Necromancy script does not carry K:MayFlashSac")
	}
	e, cfg, id := newFixtureDeck(t, 503, necro, mayflashsacTargetSrc)
	bear := moveSeeded(t, e, 0, mayflashsacTargetSrc, state.ZGraveyard)
	e.askPriority(0)
	driveToStep(t, e, e.G.Turn, 0, state.StepBeginCombat)

	// Fund {2}{B} after the drive.
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "B", Amount: 1})
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "C", Amount: 2})
	e.pending = nil
	e.askPriority(0)

	if !plainCastOptionExists(e.Pending().Options, id) {
		t.Fatalf("Necromancy not offered through K:MayFlashSac off-main: %+v", e.Pending().Options)
	}
	submitChoices(t, e, plainCastOption(t, e, id).Index)

	// Resolve the spell and its ETB reanimation, answering the reanimation's
	// graveyard-creature target with the seeded bear if it asks.
	resolveStackFor(t, e, bear, 60)
	passUntilStackEmpty(t, e, 30)

	if o := e.G.Obj(id); o.Zone != state.ZBattlefield {
		t.Fatalf("Necromancy zone %s, want battlefield after resolving", o.Zone)
	}
	if !sawMayFlashSacFlag(e, id) {
		t.Fatal("off-sorcery Necromancy cast did not stamp FlagMayFlashSac")
	}
	if !sawMayFlashSacCleanupRegister(e, id) {
		t.Fatal("off-sorcery Necromancy cast did not register the cleanup sacrifice")
	}

	driveToStep(t, e, e.G.Turn+1, 1, state.StepMain1)
	if o := e.G.Obj(id); o.Zone != state.ZGraveyard {
		t.Fatalf("Necromancy zone %s after cleanup, want graveyard (the rider sacrifices the permanent it became)", o.Zone)
	}
	replayCheck(t, e, cfg)
}
