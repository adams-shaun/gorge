package rules

// CR 613.7e/f/g: an Aura, Equipment or Fortification receives a new timestamp
// each time it becomes attached (701.3c: to a different object), and a
// permanent receives one each time it turns face up or face down or
// transforms. The new timestamp orders that permanent's static abilities in
// their layer (613.7a). The observable here is layer 6: Witness Protection's
// "loses all abilities" removes a keyword grant OLDER than it and keeps one
// NEWER than it (abilityloss.go's grantLostTo). Every card is a real corpus
// card.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// restampActivate activates obj's ability idx, answers every non-priority
// ask (the equip target) with its first option, and resolves the stack.
func restampActivate(t *testing.T, e *Engine, obj state.ObjID, idx int) {
	t.Helper()
	submitChoices(t, e, abilityOption(t, e, obj, idx).Index)
	for i := 0; i < 5 && len(e.G.Stack) == 0; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack %v after activating %d, want the ability", e.G.Stack, obj)
	}
	passUntilStackEmpty(t, e, 20)
}

// Swiftfoot Boots is on the battlefield, unattached, BEFORE Witness
// Protection enchants the Bears; then the Boots are equipped to the Bears.
// Equipping gives the Boots a new timestamp (CR 613.7e), later than Witness
// Protection's, so the Boots' grant applies after the ability removal and the
// Bears have hexproof and haste.
func TestEquipAfterAbilityLossRestampsTheEquipment(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Swiftfoot Boots"), lookup(t, reg, "Grizzly Bears"),
		lookup(t, reg, "Witness Protection")}, nil)
	boots := moveCorpusCard(t, e, "Swiftfoot Boots", 0, state.ZBattlefield)
	bears := moveCorpusCard(t, e, "Grizzly Bears", 0, state.ZBattlefield)
	lossAttach(t, e, "Witness Protection", 0, bears)
	entry := e.G.Obj(boots).Timestamp
	addMana(t, e, 0, "G")
	restampActivate(t, e, boots, 0)
	if got := e.G.Obj(boots).AttachedTo; got != bears {
		t.Fatalf("Swiftfoot Boots attached to %d, want the Bears %d", got, bears)
	}
	if !e.HasKeyword(bears, "Hexproof") || !e.HasKeyword(bears, "Haste") {
		t.Errorf("CR 613.7e: Boots equipped after Witness Protection must keep hexproof and haste; Bears keywords %v", e.Keywords(bears))
	}
	// The ENTRY timestamp is the object's battlefield-stint identity (control
	// grants' ObjStamp, the legend-rule youngest pick): attaching must not
	// move it.
	if got := e.G.Obj(boots).Timestamp; got != entry {
		t.Errorf("equipping moved the Boots' entry Timestamp %d -> %d", entry, got)
	}
	replayCheck(t, e, cfg)
}

// The guard: Boots equipped BEFORE Witness Protection are older than it, so
// the Bears lose hexproof and haste (correct today). An Attach to the bearer
// the Boots already equip is not "attached to a different object" (CR 701.3c)
// and must not restamp them.
func TestEquipBeforeAbilityLossStaysLost(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Swiftfoot Boots"), lookup(t, reg, "Grizzly Bears"),
		lookup(t, reg, "Witness Protection")}, nil)
	boots := moveCorpusCard(t, e, "Swiftfoot Boots", 0, state.ZBattlefield)
	bears := moveCorpusCard(t, e, "Grizzly Bears", 0, state.ZBattlefield)
	addMana(t, e, 0, "G")
	restampActivate(t, e, boots, 0)
	if !e.HasKeyword(bears, "Hexproof") || !e.HasKeyword(bears, "Haste") {
		t.Fatalf("precondition: equipped Bears keywords %v, want hexproof and haste", e.Keywords(bears))
	}
	lossAttach(t, e, "Witness Protection", 0, bears)
	if e.HasKeyword(bears, "Hexproof") || e.HasKeyword(bears, "Haste") {
		t.Errorf("Boots older than Witness Protection: Bears keywords %v, want neither hexproof nor haste", e.Keywords(bears))
	}
	e.emit(events.Event{Kind: events.Attach, Obj: boots, IDs: []state.ObjID{bears}})
	if e.HasKeyword(bears, "Hexproof") || e.HasKeyword(bears, "Haste") {
		t.Errorf("a same-bearer Attach restamped the Boots: Bears keywords %v", e.Keywords(bears))
	}
	replayCheck(t, e, cfg)
}

// Ulvenwald Oddity is on the battlefield before Witness Protection enchants
// the Bears; then it transforms into Ulvenwald Behemoth ("Other creatures you
// control get +1/+1 and have trample and haste"). Transforming gives it a new
// timestamp (CR 613.7g), later than Witness Protection's, so the Bears have
// trample and haste.
func TestTransformRestampsThePermanent(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Ulvenwald Oddity"), lookup(t, reg, "Grizzly Bears"),
		lookup(t, reg, "Witness Protection")}, nil)
	oddity := moveCorpusCard(t, e, "Ulvenwald Oddity", 0, state.ZBattlefield)
	bears := moveCorpusCard(t, e, "Grizzly Bears", 0, state.ZBattlefield)
	lossAttach(t, e, "Witness Protection", 0, bears)
	addMana(t, e, 0, "GGGGGGG")
	restampActivate(t, e, oddity, 0)
	if f := e.G.Obj(oddity).Face(); f == nil || f.Name != "Ulvenwald Behemoth" {
		t.Fatalf("Ulvenwald Oddity did not transform: face %v", f)
	}
	if !e.HasKeyword(bears, "Trample") || !e.HasKeyword(bears, "Haste") {
		t.Errorf("CR 613.7g: Behemoth transformed after Witness Protection must grant trample and haste; Bears keywords %v", e.Keywords(bears))
	}
	replayCheck(t, e, cfg)
}

// Tannuk, Steadfast Second ("Other creatures you control have haste") is face
// down on the battlefield before Witness Protection enchants the Bears; then
// it is turned face up. Turning face up gives it a new timestamp (CR 613.7f),
// later than Witness Protection's, so the Bears have haste.
func TestTurnFaceUpRestampsThePermanent(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Tannuk, Steadfast Second"), lookup(t, reg, "Grizzly Bears"),
		lookup(t, reg, "Witness Protection")}, nil)
	tannuk := moveCorpusCard(t, e, "Tannuk, Steadfast Second", 0, state.ZBattlefield)
	e.emit(events.Event{Kind: events.TurnFaceDown, Obj: tannuk})
	bears := moveCorpusCard(t, e, "Grizzly Bears", 0, state.ZBattlefield)
	lossAttach(t, e, "Witness Protection", 0, bears)
	e.emit(events.Event{Kind: events.TurnFaceUp, Obj: tannuk})
	if e.G.Obj(tannuk).FaceDown {
		t.Fatal("precondition: Tannuk is still face down")
	}
	if !e.HasKeyword(bears, "Haste") {
		t.Errorf("CR 613.7f: Tannuk turned face up after Witness Protection must grant haste; Bears keywords %v", e.Keywords(bears))
	}
	replayCheck(t, e, cfg)
}
