package rules

import (
	"github.com/adams-shaun/gorge/engine/events"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// edhCloneCast casts the named hand card (seat 0) with a template of the
// given name on the battlefield under tplSeat, takes the as-enters copy
// election for that template and returns the entered object and template.
func edhCloneCast(t *testing.T, name, tplName string, tplSeat state.PlayerID, pool map[int]int32) (e *Engine, id, tpl state.ObjID) {
	t.Helper()
	e = goadGrantEngine(t, corpusAlternativeCard(t, name))
	tpl = onBoardReadyCard(t, e, tplSeat, corpusAlternativeCard(t, tplName))
	for k, n := range pool {
		e.G.Players[0].Pool[k] = n
	}
	id = e.G.Zone(state.ZHand, 0)[0]
	castMode(t, e, id, "")
	e.resolveTop()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "etb" {
		t.Fatalf("%s: expected the ETB copy election, got %+v", name, d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Obj == tpl {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("%s: template %s not offered: %+v", name, tplName, d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 8)
	return e, id, tpl
}

func edhHasKeyword(o *state.Object, head string) bool {
	for _, kw := range o.Face().Keywords {
		if cards.KeywordHead(kw) == head {
			return true
		}
	}
	return false
}

// TestEDHETBCloneRidersCursedMirrorHastyUntilEOT: Cursed Mirror enters as a
// copy of a creature with haste, and at the cleanup step is itself again.
func TestEDHETBCloneRidersCursedMirrorHastyUntilEOT(t *testing.T) {
	t.Parallel()
	e, id, _ := edhCloneCast(t, "Cursed Mirror", "Grizzly Bears", 1,
		map[int]int32{state.MC: 2, state.MR: 1})
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.Face().Name != "Grizzly Bears" {
		t.Fatalf("mirror = %+v, want a Grizzly Bears copy on the battlefield", o)
	}
	if !e.IsCreature(id) {
		t.Fatal("the copy is not a creature")
	}
	if !edhHasKeyword(o, "Haste") && !e.hasKeywordH(id, kwhHaste) {
		t.Fatal("the copy lacks the excepted haste")
	}
	e.pending = nil
	e.setStep(state.StepCleanup)
	e.priorityRound()
	if o := e.G.Obj(id); o.Face() == nil || o.Face().Name != "Cursed Mirror" || e.IsCreature(id) {
		t.Fatalf("after cleanup the mirror is %v (creature %v), want Cursed Mirror", o.Face(), e.IsCreature(id))
	}
}

// TestEDHETBCloneRidersImposterMechIsNoncreatureVehicle: Imposter Mech copies
// an opponent's creature but is a Vehicle artifact with crew 3 and loses
// every other card type.
func TestEDHETBCloneRidersImposterMechIsNoncreatureVehicle(t *testing.T) {
	t.Parallel()
	e, id, _ := edhCloneCast(t, "Imposter Mech", "Grizzly Bears", 1,
		map[int]int32{state.MC: 1, state.MU: 1})
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.Face().Name != "Grizzly Bears" {
		t.Fatalf("mech = %+v, want a Grizzly Bears copy on the battlefield", o)
	}
	if e.IsCreature(id) {
		t.Fatal("the Vehicle copy is a creature")
	}
	if !e.hasType(o, "Artifact") || !e.hasType(o, "Vehicle") {
		t.Fatalf("copy is not a Vehicle artifact: %v", o.Face().Types)
	}
	if e.hasType(o, "Bear") {
		t.Fatal("copy kept the Bear subtype (RemoveSubTypes$)")
	}
	if !edhHasKeyword(o, "Crew") {
		t.Fatalf("copy lacks crew 3: %v", o.Face().Keywords)
	}
}

// TestEDHETBCloneRidersMachineGodsEffigyIsArtifactManaRock: the copy is a
// noncreature artifact that taps for {U}.
func TestEDHETBCloneRidersMachineGodsEffigyIsArtifactManaRock(t *testing.T) {
	t.Parallel()
	e, id, _ := edhCloneCast(t, "Machine God's Effigy", "Grizzly Bears", 1,
		map[int]int32{state.MC: 4})
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.Face().Name != "Grizzly Bears" {
		t.Fatalf("effigy = %+v, want a Grizzly Bears copy on the battlefield", o)
	}
	if e.IsCreature(id) || !e.hasType(o, "Artifact") {
		t.Fatalf("copy is not a noncreature artifact: %v", o.Face().Types)
	}
	mana := false
	for _, ab := range o.Face().Abilities {
		if ab.API == "Mana" {
			mana = true
		}
	}
	if !mana {
		t.Fatal("copy lacks the granted {T}: Add {U} ability")
	}
}

// TestEDHETBCloneRidersSparkDoubleCounterAndNonlegendary: Spark Double copying
// a legendary creature you control enters with one additional +1/+1 counter
// and is not legendary (the template keeps its supertype and counters).
func TestEDHETBCloneRidersSparkDoubleCounterAndNonlegendary(t *testing.T) {
	t.Parallel()
	e, id, tpl := edhCloneCast(t, "Spark Double", "Isamaru, Hound of Konda", 0,
		map[int]int32{state.MC: 3, state.MU: 1})
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.Face().Name != "Isamaru, Hound of Konda" {
		t.Fatalf("double = %+v, want an Isamaru copy on the battlefield", o)
	}
	if !e.hasType(e.G.Obj(tpl), "Legendary") {
		t.Fatal("precondition: the template must be legendary")
	}
	if e.hasType(o, "Legendary") {
		t.Fatal("the copy is still legendary (NonLegendary$)")
	}
	if got := o.Counter("P1P1"); got != 1 {
		t.Fatalf("copy entered with %d +1/+1 counters, want 1", got)
	}
	if got := e.G.Obj(tpl).Counter("P1P1"); got != 0 {
		t.Fatalf("template gained %d +1/+1 counters, want none", got)
	}
	if o.Counter("LOYALTY") != 0 {
		t.Fatal("a creature copy took a loyalty counter")
	}
	if got := e.Power(id); got != 3 {
		t.Fatalf("copy power = %d, want 3 (2/2 plus the counter)", got)
	}
}

// TestEDHETBCloneRidersSparkDoublePlaneswalkerLoyalty: copying a planeswalker
// enters with an additional loyalty counter and no +1/+1 counter.
func TestEDHETBCloneRidersSparkDoublePlaneswalkerLoyalty(t *testing.T) {
	t.Parallel()
	e, id, tpl := edhCloneCast(t, "Spark Double", "Jace Beleren", 0,
		map[int]int32{state.MC: 3, state.MU: 1})
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.Face().Name != "Jace Beleren" {
		t.Fatalf("double = %+v, want a Jace Beleren copy on the battlefield", o)
	}
	if e.G.Obj(tpl) == nil {
		t.Fatal("template vanished")
	}
	// Jace Beleren's printed loyalty is 3; the rider adds one.
	if got := o.Counter("LOYALTY"); got != 4 {
		t.Fatalf("copy entered with %d loyalty, want 4 (printed 3 plus one)", got)
	}
	if o.Counter("P1P1") != 0 {
		t.Fatal("a planeswalker copy took a +1/+1 counter")
	}
}

// TestEDHETBCloneRidersMoritteTwoCountersOnlyForCreatures: Moritte of the
// Frost copying a creature enters with two +1/+1 counters, legendary and
// snow; copying a noncreature permanent it takes none.
func TestEDHETBCloneRidersMoritteTwoCountersOnlyForCreatures(t *testing.T) {
	t.Parallel()
	pool := map[int]int32{state.MC: 2, state.MG: 1, state.MU: 2}
	e, id, _ := edhCloneCast(t, "Moritte of the Frost", "Grizzly Bears", 0, pool)
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.Face().Name != "Grizzly Bears" {
		t.Fatalf("moritte = %+v, want a Grizzly Bears copy on the battlefield", o)
	}
	if got := o.Counter("P1P1"); got != 2 {
		t.Fatalf("creature copy entered with %d +1/+1 counters, want 2", got)
	}
	if !e.hasType(o, "Legendary") || !e.hasType(o, "Snow") {
		t.Fatalf("copy is not legendary and snow: %v", o.Face().Types)
	}
	e2, id2, _ := edhCloneCast(t, "Moritte of the Frost", "Sol Ring", 0, pool)
	o2 := e2.G.Obj(id2)
	if o2 == nil || o2.Zone != state.ZBattlefield || o2.Face().Name != "Sol Ring" {
		t.Fatalf("moritte = %+v, want a Sol Ring copy on the battlefield", o2)
	}
	if got := o2.Counter("P1P1"); got != 0 {
		t.Fatalf("noncreature copy took %d +1/+1 counters, want none", got)
	}
}

// TestEDHETBCloneRidersWallOfStolenIdentityLocksTheOriginal: the Wall enters
// as a defender Wall copy of an opponent's creature, taps it, and that
// creature stays tapped through its controller's untap step.
func TestEDHETBCloneRidersWallOfStolenIdentityLocksTheOriginal(t *testing.T) {
	t.Parallel()
	e, id, tpl := edhCloneCast(t, "Wall of Stolen Identity", "Grizzly Bears", 1,
		map[int]int32{state.MC: 3, state.MU: 1})
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.Face().Name != "Grizzly Bears" {
		t.Fatalf("wall = %+v, want a Grizzly Bears copy on the battlefield", o)
	}
	if !e.hasType(o, "Wall") || !edhHasKeyword(o, "Defender") {
		t.Fatalf("copy is not a defender Wall: %v %v", o.Face().Types, o.Face().Keywords)
	}
	e.putTriggersOnStack()
	pushed := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.DelayedPush {
			pushed = true
		}
	}
	if !pushed {
		t.Fatal("no reflexive trigger was put on the stack")
	}
	for i := 0; i < 8 && len(e.G.Stack) > 0; i++ {
		resolveTopProbe(e)
		settleDrain(e)
	}
	if !e.G.Obj(tpl).Tapped {
		t.Fatal("the copied creature was not tapped")
	}
	// The opponent's untap step: the locked creature stays tapped.
	e.G.Active = 1
	e.setStep(state.StepUntap)
	e.untapTurnPermanent(tpl)
	if !e.G.Obj(tpl).Tapped {
		t.Fatal("the copied creature untapped during its controller's untap step")
	}
}
