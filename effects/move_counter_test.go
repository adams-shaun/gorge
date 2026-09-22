package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Task movecounter1: api:MoveCounter (Forge's MoveCounterEffect). The
// primitive was unregistered, so every `AB$ MoveCounter` / `DB$ MoveCounter` /
// `SP$ MoveCounter` line (33 raw SA lines over 32 corpus files) degraded to
// one "unimplemented API MoveCounter" Note and moved nothing -- and the
// ACTIVATED carriers were not even offered, because the offer gate reads the
// registered API. These effects-level leaves pin the primitive's two-set
// resolution (FROM and TO) on synthetic scripts; the end-to-end real-corpus
// carriers live in rules/move_counter_test.go.

// moveCounterSA builds a `DB$ MoveCounter` SA with the given extra parameters.
func moveCounterSA(t *testing.T, extra string) *cards.SA {
	t.Helper()
	line := "DB$ MoveCounter"
	if extra != "" {
		line += " | " + extra
	}
	return sa(t, line)
}

// creature adds one battlefield creature for owner and returns its id. The id
// (never the *state.Object) is what callers keep: Game.AddObject appends to
// Game.Objs, so a pointer returned before a later AddObject can be stale.
func mcCreature(t *testing.T, g *state.Game, name string, owner state.PlayerID) state.ObjID {
	t.Helper()
	o := g.AddObject(mkCard(t, "Name:"+name+"\nTypes:Creature\nPT:1/1\nOracle:x\n"), owner)
	o.Zone = state.ZBattlefield
	g.SetZone(state.ZBattlefield, owner, append(g.Zone(state.ZBattlefield, owner), o.ID))
	return o.ID
}

// artifact adds one battlefield artifact for owner.
func mcArtifact(t *testing.T, g *state.Game, name string, owner state.PlayerID) state.ObjID {
	t.Helper()
	o := g.AddObject(mkCard(t, "Name:"+name+"\nTypes:Artifact\nOracle:x\n"), owner)
	o.Zone = state.ZBattlefield
	g.SetZone(state.ZBattlefield, owner, append(g.Zone(state.ZBattlefield, owner), o.ID))
	return o.ID
}

// Source$ Self | ValidTgts$ Creature: the counter leaves the SOURCE and lands
// on the CHOSEN target (Weapon Rack, Diamond City, Explorer's Cache).
func TestMoveCounterSourceSelfToChosenTarget(t *testing.T) {
	h := &fakeHost{g: state.NewGame(names(2))}
	alpha := mcArtifact(t, h.g, "Alpha", 0)
	beta := mcCreature(t, h.g, "Beta", 0)
	h.g.Obj(alpha).AddCounter("P1P1", 3)
	Resolve(h, &Ctx{Controller: 0, Source: alpha,
		Targets: []state.Target{{Obj: beta}}, TargetsOffered: true},
		moveCounterSA(t, "Source$ Self | ValidTgts$ Creature | CounterType$ P1P1 | CounterNum$ 1"))
	if got := h.g.Obj(alpha).Counter("P1P1"); got != 2 {
		t.Fatalf("source P1P1 = %d, want 2 (one moved off)", got)
	}
	if got := h.g.Obj(beta).Counter("P1P1"); got != 1 {
		t.Fatalf("target P1P1 = %d, want 1 (one landed)", got)
	}
	assertCounterPair(t, h.log, alpha, beta, "P1P1", 1)
}

// ValidTgts$ Creature | Defined$ Self: the counter leaves the CHOSEN target
// and lands on the SOURCE (Cytoplast Root-Kin, Arcbound Fiend) -- the mirror
// of the preceding shape, and the case the origin rule's "no Source$ =>
// chosen targets" branch exists for.
func TestMoveCounterChosenTargetToDefinedSelf(t *testing.T) {
	h := &fakeHost{g: state.NewGame(names(2))}
	alpha := mcArtifact(t, h.g, "Alpha", 0)
	beta := mcCreature(t, h.g, "Beta", 0)
	h.g.Obj(beta).AddCounter("P1P1", 2)
	Resolve(h, &Ctx{Controller: 0, Source: alpha,
		Targets: []state.Target{{Obj: beta}}, TargetsOffered: true},
		moveCounterSA(t, "Defined$ Self | ValidTgts$ Creature | CounterType$ P1P1 | CounterNum$ 1"))
	if got := h.g.Obj(beta).Counter("P1P1"); got != 1 {
		t.Fatalf("chosen target P1P1 = %d, want 1 (one moved off)", got)
	}
	if got := h.g.Obj(alpha).Counter("P1P1"); got != 1 {
		t.Fatalf("source P1P1 = %d, want 1 (one landed)", got)
	}
	assertCounterPair(t, h.log, beta, alpha, "P1P1", 1)
}

// ValidTgts$ Creature | TargetMin$ 2 | TargetMax$ 2: the 2-target shape --
// target 0 is the ORIGIN and the remaining targets are the destinations
// (Bioshift, Fate Transfer, Daghatar).
func TestMoveCounterTwoTargetsFirstIsOrigin(t *testing.T) {
	h := &fakeHost{g: state.NewGame(names(2))}
	alpha := mcCreature(t, h.g, "Alpha", 0)
	beta := mcCreature(t, h.g, "Beta", 0)
	h.g.Obj(alpha).AddCounter("P1P1", 4)
	Resolve(h, &Ctx{Controller: 0, Source: alpha,
		Targets: []state.Target{{Obj: alpha}, {Obj: beta}}, TargetsOffered: true},
		moveCounterSA(t, "ValidTgts$ Creature | TargetMin$ 2 | TargetMax$ 2 | CounterType$ P1P1 | CounterNum$ 1"))
	if got := h.g.Obj(alpha).Counter("P1P1"); got != 3 {
		t.Fatalf("target 0 P1P1 = %d, want 3 (origin, lost one)", got)
	}
	if got := h.g.Obj(beta).Counter("P1P1"); got != 1 {
		t.Fatalf("target 1 P1P1 = %d, want 1 (destination, gained one)", got)
	}
	assertCounterPair(t, h.log, alpha, beta, "P1P1", 1)
}

// ValidSource$ <filter> | Defined$ Self: the sweep shape -- EVERY battlefield
// object the filter admits is an origin and the Defined$ Self is the single
// destination (Aetherborn Marauder, Spike Cannibal).
func TestMoveCounterValidSourceSweepToDefinedSelf(t *testing.T) {
	h := &fakeHost{g: state.NewGame(names(2))}
	theirs := mcCreature(t, h.g, "Theirs", 1)
	mine := mcCreature(t, h.g, "Mine", 0)
	sink := mcCreature(t, h.g, "Sink", 0)
	h.g.Obj(theirs).AddCounter("P1P1", 2)
	h.g.Obj(mine).AddCounter("P1P1", 1)
	Resolve(h, &Ctx{Controller: 0, Source: sink},
		moveCounterSA(t, "ValidSource$ Creature | Defined$ Self | CounterType$ P1P1 | CounterNum$ All"))
	if got := h.g.Obj(theirs).Counter("P1P1"); got != 0 {
		t.Fatalf("opponent creature P1P1 = %d, want 0 (swept, all moved)", got)
	}
	if got := h.g.Obj(mine).Counter("P1P1"); got != 0 {
		t.Fatalf("own creature P1P1 = %d, want 0 (swept, all moved)", got)
	}
	if got := h.g.Obj(sink).Counter("P1P1"); got != 3 {
		t.Fatalf("sink P1P1 = %d, want 3 (2 + 1 swept in)", got)
	}
}

// Source$ Self | ValidDefined$ Creature.Other: the destination is a sweep
// (Forgotten Ancient's "onto other creatures"); every matching creature gains
// the counter, the source loses it once.
func TestMoveCounterSourceSelfToValidDefinedSweep(t *testing.T) {
	h := &fakeHost{g: state.NewGame(names(2))}
	alpha := mcCreature(t, h.g, "Alpha", 0)
	beta := mcCreature(t, h.g, "Beta", 0)
	h.g.Obj(alpha).AddCounter("P1P1", 2)
	Resolve(h, &Ctx{Controller: 0, Source: alpha},
		moveCounterSA(t, "Source$ Self | ValidDefined$ Creature.Other | CounterType$ P1P1 | CounterNum$ 1"))
	if got := h.g.Obj(alpha).Counter("P1P1"); got != 1 {
		t.Fatalf("source P1P1 = %d, want 1 (one moved off)", got)
	}
	if got := h.g.Obj(beta).Counter("P1P1"); got != 1 {
		t.Fatalf("other creature P1P1 = %d, want 1 (the sweep destination)", got)
	}
}

// CounterType$ All moves every kind the origin holds; CounterNum$ All moves
// all of each (Fate Transfer, The Ozolith).
func TestMoveCounterTypeAllNumAllMovesEveryKindAndCount(t *testing.T) {
	h := &fakeHost{g: state.NewGame(names(2))}
	alpha := mcArtifact(t, h.g, "Alpha", 0)
	beta := mcCreature(t, h.g, "Beta", 0)
	h.g.Obj(alpha).AddCounter("P1P1", 2)
	h.g.Obj(alpha).AddCounter("CHARGE", 3)
	Resolve(h, &Ctx{Controller: 0, Source: alpha, Targets: []state.Target{{Obj: beta}}},
		moveCounterSA(t, "Source$ Self | ValidDefined$ Creature | CounterType$ All | CounterNum$ All"))
	if got := h.g.Obj(alpha).Counter("P1P1"); got != 0 {
		t.Fatalf("source P1P1 = %d, want 0", got)
	}
	if got := h.g.Obj(alpha).Counter("CHARGE"); got != 0 {
		t.Fatalf("source CHARGE = %d, want 0", got)
	}
	if got := h.g.Obj(beta).Counter("P1P1"); got != 2 {
		t.Fatalf("target P1P1 = %d, want 2", got)
	}
	if got := h.g.Obj(beta).Counter("CHARGE"); got != 3 {
		t.Fatalf("target CHARGE = %d, want 3", got)
	}
}

// CounterType$ EachNotOn: each kind the origin holds that the destination
// does NOT already have (Goldberry's first ability). A kind the destination
// already carries is not moved.
func TestMoveCounterEachNotOnSkipsKindsTheDestinationHas(t *testing.T) {
	h := &fakeHost{g: state.NewGame(names(2))}
	alpha := mcArtifact(t, h.g, "Alpha", 0)
	beta := mcCreature(t, h.g, "Beta", 0)
	h.g.Obj(alpha).AddCounter("P1P1", 2)
	h.g.Obj(alpha).AddCounter("CHARGE", 1)
	h.g.Obj(beta).AddCounter("P1P1", 5) // destination already has P1P1
	Resolve(h, &Ctx{Controller: 0, Source: alpha, Targets: []state.Target{{Obj: beta}}},
		moveCounterSA(t, "Source$ Self | ValidTgts$ Creature | CounterType$ EachNotOn | CounterNum$ 1"))
	if got := h.g.Obj(alpha).Counter("P1P1"); got != 2 {
		t.Fatalf("source P1P1 = %d, want 2 (destination already had P1P1: not moved)", got)
	}
	if got := h.g.Obj(alpha).Counter("CHARGE"); got != 0 {
		t.Fatalf("source CHARGE = %d, want 0 (destination lacked it: moved)", got)
	}
	if got := h.g.Obj(beta).Counter("CHARGE"); got != 1 {
		t.Fatalf("target CHARGE = %d, want 1", got)
	}
	if got := h.g.Obj(beta).Counter("P1P1"); got != 5 {
		t.Fatalf("target P1P1 = %d, want 5 (untouched)", got)
	}
}

// CounterType$ Any asks which single kind to move when the origin holds more
// than one; the answered kind is what moves. One kind only: no ask.
func TestMoveCounterTypeAnyAsksForTheKind(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	src := mcArtifact(t, h.g, "Src", 0)
	dst := mcCreature(t, h.g, "Dst", 0)
	h.g.Obj(src).AddCounter("P1P1", 2)
	h.g.Obj(src).AddCounter("CHARGE", 3)

	Resolve(h, &Ctx{Controller: 0, Source: src, Targets: []state.Target{{Obj: dst}}, TargetsOffered: true},
		moveCounterSA(t, "Source$ Self | ValidTgts$ Creature | CounterType$ Any | CounterNum$ 1"))
	if h.asked == nil {
		t.Fatal("no CounterType$ Any kind decision was posed")
	}
	d := h.asked
	if d.Kind != decision.KChoose || d.Min != 1 || d.Max != 1 {
		t.Fatalf("decision = %+v, want a Min==Max==1 KChoose", d)
	}
	if d.ResumeKind != "move_counter_kind" {
		t.Fatalf("ResumeKind = %q, want move_counter_kind", d.ResumeKind)
	}
	if len(d.Options) != 2 {
		t.Fatalf("kind options = %+v, want the two distinct kinds", d.Options)
	}
	// Nothing moved yet.
	if got := h.g.Obj(src).Counter("P1P1"); got != 2 {
		t.Fatalf("source P1P1 = %d before the answer, want 2", got)
	}
	// Re-entry with the SECOND offered kind (CHARGE) picked, to prove the
	// answer is honoured rather than the first-kind stand-in.
	Resolve(h, &Ctx{Controller: 0, Source: src, Targets: []state.Target{{Obj: dst}}, TargetsOffered: true,
		MoveCounterKind: "CHARGE", MoveCounterKindDone: true},
		moveCounterSA(t, "Source$ Self | ValidTgts$ Creature | CounterType$ Any | CounterNum$ 1"))
	if got := h.g.Obj(src).Counter("CHARGE"); got != 2 {
		t.Fatalf("source CHARGE = %d, want 2 (one CHARGE moved)", got)
	}
	if got := h.g.Obj(src).Counter("P1P1"); got != 2 {
		t.Fatalf("source P1P1 = %d, want 2 (untouched -- a different kind was chosen)", got)
	}
	if got := h.g.Obj(dst).Counter("CHARGE"); got != 1 {
		t.Fatalf("target CHARGE = %d, want 1", got)
	}
}

// A single-kind origin under CounterType$ Any takes that kind with no ask
// (the strict-supersets convention).
func TestMoveCounterTypeAnySingleKindDoesNotAsk(t *testing.T) {
	h := &fakeHost{g: state.NewGame(names(2))}
	src := mcArtifact(t, h.g, "Src", 0)
	dst := mcCreature(t, h.g, "Dst", 0)
	h.g.Obj(src).AddCounter("P1P1", 2)
	Resolve(h, &Ctx{Controller: 0, Source: src, Targets: []state.Target{{Obj: dst}}},
		moveCounterSA(t, "Source$ Self | ValidTgts$ Creature | CounterType$ Any | CounterNum$ 1"))
	if got := h.g.Obj(dst).Counter("P1P1"); got != 1 {
		t.Fatalf("target P1P1 = %d, want 1 (the sole kind moved with no ask)", got)
	}
}

// CounterNum$ Any asks how many to move, Min 0 / Max the origin's count; the
// answered amount is exactly what moves, and a Min-0 decline (Ctx carries 0)
// moves nothing.
func TestMoveCounterNumAnyAsksAndHonoursTheAmount(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	src := mcArtifact(t, h.g, "Src", 0)
	dst := mcCreature(t, h.g, "Dst", 0)
	h.g.Obj(src).AddCounter("P1P1", 4)

	Resolve(h, &Ctx{Controller: 0, Source: src, Targets: []state.Target{{Obj: dst}}, TargetsOffered: true},
		moveCounterSA(t, "Source$ Self | ValidTgts$ Creature | CounterType$ P1P1 | CounterNum$ Any"))
	if h.asked == nil {
		t.Fatal("no CounterNum$ Any amount decision was posed")
	}
	d := h.asked
	if d.Kind != decision.KChoose || d.Min != 0 || d.Max != 4 {
		t.Fatalf("decision = %+v, want a Min 0 / Max 4 KChoose", d)
	}
	if d.ResumeKind != "move_counter" {
		t.Fatalf("ResumeKind = %q, want move_counter", d.ResumeKind)
	}
	if len(d.Options) != 5 {
		t.Fatalf("amount options = %d, want 5 (0..4)", len(d.Options))
	}

	// Answer 2: exactly two move.
	Resolve(h, &Ctx{Controller: 0, Source: src, Targets: []state.Target{{Obj: dst}}, TargetsOffered: true,
		MoveCounterN: 2, MoveCounterNDone: true},
		moveCounterSA(t, "Source$ Self | ValidTgts$ Creature | CounterType$ P1P1 | CounterNum$ Any"))
	if got := h.g.Obj(src).Counter("P1P1"); got != 2 {
		t.Fatalf("source P1P1 = %d, want 2 (answered 2 of 4)", got)
	}
	if got := h.g.Obj(dst).Counter("P1P1"); got != 2 {
		t.Fatalf("target P1P1 = %d, want 2", got)
	}
}

func TestMoveCounterNumAnyDeclineMovesNothing(t *testing.T) {
	h := &fakeHost{g: state.NewGame(names(2))}
	src := mcArtifact(t, h.g, "Src", 0)
	dst := mcCreature(t, h.g, "Dst", 0)
	h.g.Obj(src).AddCounter("P1P1", 3)
	// The answered Min-0 decline: a re-entry with the done marker and N = 0.
	Resolve(h, &Ctx{Controller: 0, Source: src, Targets: []state.Target{{Obj: dst}}, TargetsOffered: true,
		MoveCounterN: 0, MoveCounterNDone: true},
		moveCounterSA(t, "Source$ Self | ValidTgts$ Creature | CounterType$ P1P1 | CounterNum$ Any"))
	if got := h.g.Obj(src).Counter("P1P1"); got != 3 {
		t.Fatalf("source P1P1 = %d, want 3 (declined: nothing moved)", got)
	}
	if got := h.g.Obj(dst).Counter("P1P1"); got != 0 {
		t.Fatalf("target P1P1 = %d, want 0 (declined: nothing moved)", got)
	}
}

// RememberPut$ True remembers the DESTINATION that received a counter
// (Goldberry's SVar:X:Remembered$Amount gate).
func TestMoveCounterRememberPutRemembersTheDestination(t *testing.T) {
	h := &fakeHost{g: state.NewGame(names(2))}
	src := mcArtifact(t, h.g, "Src", 0)
	dst := mcCreature(t, h.g, "Dst", 0)
	h.g.Obj(src).AddCounter("P1P1", 2)
	c := &Ctx{Controller: 0, Source: src, Targets: []state.Target{{Obj: dst}}, TargetsOffered: true}
	Resolve(h, c, moveCounterSA(t, "Source$ Self | ValidTgts$ Creature | CounterType$ P1P1 | CounterNum$ 1 | RememberPut$ True"))
	if len(c.Remembered) != 1 || c.Remembered[0].Obj != dst {
		t.Fatalf("Remembered = %+v, want the destination %d once", c.Remembered, dst)
	}
}

// RememberAmount$ True remembers the moved AMOUNT through the engine's list
// channel: Count$RememberedNumber reads len(Ctx.Remembered), so the origin id
// is appended once per counter moved (Black Panther's gain-that-much-life).
func TestMoveCounterRememberAmountRecordsTheAmount(t *testing.T) {
	h := &fakeHost{g: state.NewGame(names(2))}
	src := mcArtifact(t, h.g, "Src", 0)
	dst := mcCreature(t, h.g, "Dst", 0)
	h.g.Obj(src).AddCounter("P1P1", 3)
	c := &Ctx{Controller: 0, Source: src, Targets: []state.Target{{Obj: dst}}, TargetsOffered: true}
	Resolve(h, c, moveCounterSA(t, "Source$ Self | ValidTgts$ Creature | CounterType$ P1P1 | CounterNum$ 2 | RememberAmount$ True"))
	if got := len(c.Remembered); got != 2 {
		t.Fatalf("Remembered length = %d, want 2 (the moved amount)", got)
	}
}

// TestBlackPantherRememberAmountFeedsGainLife follows Black Panther, Wakandan
// King's compiled DBMove -> DBGainLife chain: the move remembers one entry per
// counter (`RememberAmount$ True`), and the chained payoff reads that context
// through `SVar:X:Count$RememberedNumber` with `ConditionCheckSVar$ X`, so
// deleting the RememberedNumber count-head dispatch degrades X to zero and
// suppresses the life gain.
func TestBlackPantherRememberAmountFeedsGainLife(t *testing.T) {
	card, dbMove := corpusSA(t, "Black Panther, Wakandan King", "DBMove")
	_, dbGainLife := corpusSA(t, "Black Panther, Wakandan King", "DBGainLife")
	if dbMove.API != "MoveCounter" || dbGainLife.API != "GainLife" {
		t.Fatalf("compiled SVars = %s -> %s, want MoveCounter -> GainLife", dbMove.API, dbGainLife.API)
	}
	if dbMove.Sub == nil || dbMove.Sub.API != dbGainLife.API {
		t.Fatalf("DBMove sub-ability = %#v, want compiled DBGainLife", dbMove.Sub)
	}

	h := newHost(t, 2)
	pantherID := h.g.AddObject(card, 0).ID
	landID := h.g.AddObject(mkCard(t, "Name:Vibranium Land\nTypes:Land\nOracle:x\n"), 0).ID
	creatureID := h.g.AddObject(mkCard(t, "Name:Target Creature\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0).ID
	for _, id := range []state.ObjID{pantherID, landID, creatureID} {
		h.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})
	}
	panther := h.g.Obj(pantherID)
	land := h.g.Obj(landID)
	creature := h.g.Obj(creatureID)
	h.Emit(events.Event{Kind: events.CounterChange, Obj: landID, Counter: "P1P1", Amount: 2})
	h.g.Players[0].Life = 20

	if panther.Zone != state.ZBattlefield || land.Zone != state.ZBattlefield || creature.Zone != state.ZBattlefield {
		t.Fatalf("precondition: objects must be on battlefield: panther=%v land=%v creature=%v", panther.Zone, land.Zone, creature.Zone)
	}
	if land.Counter("P1P1") != 2 || creature.Counter("P1P1") != 0 || land.ID == creature.ID {
		t.Fatalf("precondition: land=%d counters, creature=%d counters, ids=%d/%d; want 2/0 and distinct", land.Counter("P1P1"), creature.Counter("P1P1"), land.ID, creature.ID)
	}
	beforeLife := h.g.Players[0].Life
	c := &Ctx{
		Controller:      0,
		Source:          pantherID,
		SVars:           panther.Face().SVars,
		Targets:         []state.Target{{Obj: landID}},
		TargetsPick:     []state.Target{{Obj: creatureID}},
		TargetsPickDone: true,
	}
	// Resolve the two compiled SVars separately so the later compiled
	// DBDraw/DBCleanup tail does not erase Remembered before we inspect it.
	moveOnly := *dbMove
	moveOnly.Sub = nil
	gainOnly := *dbGainLife
	gainOnly.Sub = nil
	Resolve(h, c, &moveOnly)

	if got := land.Counter("P1P1"); got != 0 {
		t.Fatalf("land P1P1 = %d, want 0 after moving both counters", got)
	}
	if got := creature.Counter("P1P1"); got != 2 {
		t.Fatalf("creature P1P1 = %d, want 2 after receiving both counters", got)
	}
	if got := len(c.Remembered); got != 2 {
		t.Fatalf("Remembered length = %d, want 2 moved-counter entries", got)
	}
	Resolve(h, c, &gainOnly)
	if got := h.g.Players[0].Life; got != beforeLife+2 {
		t.Fatalf("life = %d, want %d from compiled DBGainLife's X", got, beforeLife+2)
	}
	for _, ev := range h.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "unimplemented API MoveCounter") {
			t.Fatalf("compiled MoveCounter did not run: %+v", ev)
		}
	}
}

// Out-of-scope shapes are loud-degraded: one Note naming the shape, nothing
// moves.
func TestMoveCounterExoticShapesStayLoud(t *testing.T) {
	for _, tc := range []struct{ name, extra string }{
		{"player origin", "Source$ You | Defined$ Self | CounterType$ P1P1 | CounterNum$ 1"},
		{"tgt zone", "Source$ Self | Defined$ Self | TgtZone$ Exile | CounterType$ P1P1 | CounterNum$ 1"},
		{"multi-kind any-number", "Source$ Self | ValidDefined$ Creature.Other | CounterType$ All | CounterNum$ Any"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &fakeHost{g: state.NewGame(names(2))}
			src := mcArtifact(t, h.g, "Src", 0)
			dst := mcCreature(t, h.g, "Dst", 0)
			h.g.Obj(src).AddCounter("P1P1", 2)
			Resolve(h, &Ctx{Controller: 0, Source: src, Targets: []state.Target{{Obj: dst}}},
				moveCounterSA(t, tc.extra))
			if got := h.g.Obj(src).Counter("P1P1"); got != 2 {
				t.Fatalf("source P1P1 = %d, want 2 (exotic shape must not move)", got)
			}
			found := false
			for _, ev := range h.log {
				if ev.Kind == events.Note && strings.HasPrefix(ev.Text, "unimplemented MoveCounter shape:") {
					found = true
				}
			}
			if !found {
				t.Fatalf("no unimplemented-MoveCounter Note: %+v", h.log)
			}
		})
	}
}

// The sub's own pre-ask answer beats the parent's target list: a MoveCounter
// sub the generic ValidTgts$ pre-ask asked (Nesting Grounds' `Source$
// ParentTarget | ValidTgts$ Permanent`, Rikku's, Black Panther's) receives
// Ctx.PickedTargets as its destination, never c.Targets (the parent's
// target) -- the established PickedTargets convention (effects/context.go
// Defined, effects/damage.go, effects/zone.go). Without the preference the
// -1 and +1 land on the same object and cancel.
func TestMoveCounterSubTargetBeatsParentTargets(t *testing.T) {
	h := &fakeHost{g: state.NewGame(names(2))}
	parent := mcCreature(t, h.g, "Parent", 0)
	sub := mcCreature(t, h.g, "Sub", 0)
	h.g.Obj(parent).AddCounter("P1P1", 2)
	Resolve(h, &Ctx{Controller: 0, Source: sub,
		Targets: []state.Target{{Obj: parent}}, // the parent's chosen target (Source$ ParentTarget reads it)
		// The sub's own pre-ask answer, in the transport the machinery
		// delivers it in (Ctx.TargetsPick consumed by chosenTargetsFor into
		// PickedTargets -- the exact shape of a re-entry after the ask).
		TargetsPick: []state.Target{{Obj: sub}}, TargetsPickDone: true},
		moveCounterSA(t, "Source$ ParentTarget | ValidTgts$ Creature | CounterType$ P1P1 | CounterNum$ 1"))
	if got := h.g.Obj(parent).Counter("P1P1"); got != 1 {
		t.Fatalf("parent P1P1 = %d, want 1 (the origin lost one)", got)
	}
	if got := h.g.Obj(sub).Counter("P1P1"); got != 1 {
		t.Fatalf("sub target P1P1 = %d, want 1 (the sub's OWN chosen destination gained it)", got)
	}
	assertCounterPair(t, h.log, parent, sub, "P1P1", 1)
}

// A multi-destination sweep DISTRIBUTES the moved total across the
// destinations (CR 122.5 conservation): 2 moved over 2 other creatures is 1
// and 1 -- never 2 and 2, which would mint two counters out of nothing
// (the Forgotten Ancient defect). A moved 1 lands wholly on the first
// destination in order.
func TestMoveCounterSweepDistributesMovedAcrossDestinations(t *testing.T) {
	for _, tc := range []struct {
		name         string
		moved        string
		wantA, wantB int32
		wantSrc      int32
		srcStart     int32
	}{{
		name: "2 over 2", moved: "2", srcStart: 3, wantSrc: 1, wantA: 1, wantB: 1,
	}, {
		name: "1 over 2", moved: "1", srcStart: 1, wantSrc: 0, wantA: 1, wantB: 0,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			h := &fakeHost{g: state.NewGame(names(2))}
			src := mcCreature(t, h.g, "Src", 0)
			otherA := mcCreature(t, h.g, "OtherA", 0)
			otherB := mcCreature(t, h.g, "OtherB", 0)
			h.g.Obj(src).AddCounter("P1P1", tc.srcStart)
			Resolve(h, &Ctx{Controller: 0, Source: src},
				moveCounterSA(t, "Source$ Self | ValidDefined$ Creature.Other | CounterType$ P1P1 | CounterNum$ "+tc.moved))
			total := h.g.Obj(src).Counter("P1P1") + h.g.Obj(otherA).Counter("P1P1") + h.g.Obj(otherB).Counter("P1P1")
			if total != tc.srcStart {
				t.Fatalf("total counters = %d, want %d (CR 122.5: a move never mints)", total, tc.srcStart)
			}
			if got := h.g.Obj(src).Counter("P1P1"); got != tc.wantSrc {
				t.Fatalf("source P1P1 = %d, want %d", got, tc.wantSrc)
			}
			if got := h.g.Obj(otherA).Counter("P1P1"); got != tc.wantA {
				t.Fatalf("otherA P1P1 = %d, want %d", got, tc.wantA)
			}
			if got := h.g.Obj(otherB).Counter("P1P1"); got != tc.wantB {
				t.Fatalf("otherB P1P1 = %d, want %d", got, tc.wantB)
			}
		})
	}
}

// An origin whose every destination left the battlefield (or is the origin
// itself -- a move from a permanent to itself is a null move) keeps its
// counters: nothing is emitted, nothing is lost to the void.
func TestMoveCounterDeadDestinationKeepsTheCounters(t *testing.T) {
	h := &fakeHost{g: state.NewGame(names(2))}
	src := mcCreature(t, h.g, "Src", 0)
	gone := mcCreature(t, h.g, "Gone", 0)
	h.g.Obj(src).AddCounter("P1P1", 2)
	// gone is not on the battlefield (mcCreature put it there; move it off).
	g := h.g
	g.SetZone(state.ZBattlefield, 0, nil)
	g.Obj(gone).Zone = state.ZGraveyard
	g.SetZone(state.ZGraveyard, 0, []state.ObjID{gone})
	Resolve(h, &Ctx{Controller: 0, Source: src},
		moveCounterSA(t, "Source$ Self | ValidTgts$ Creature | CounterType$ P1P1 | CounterNum$ 1"))
	if got := h.g.Obj(src).Counter("P1P1"); got != 2 {
		t.Fatalf("source P1P1 = %d, want 2 (no live destination: nothing moved)", got)
	}
}

// assertCounterPair finds the -n origin and +n destination CounterChange pair
// the move emitted.
func assertCounterPair(t *testing.T, log []events.Event, from, to state.ObjID, kind string, n int32) {
	t.Helper()
	minus, plus := 0, 0
	for _, ev := range log {
		if ev.Kind != events.CounterChange || ev.Counter != kind {
			continue
		}
		if ev.Obj == from && ev.Amount == -n {
			minus++
		}
		if ev.Obj == to && ev.Amount == n {
			plus++
		}
	}
	if minus != 1 || plus != 1 {
		t.Fatalf("CounterChange pair for %s: origin -%d x%d, dest +%d x%d; want one each (log %+v)",
			kind, n, minus, n, plus, log)
	}
}
