package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const (
	grantedHarmonizeMulch = "Name:Mulch Probe\nManaCost:1 G\nTypes:Instant\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"
	grantedHarmonizeMage  = "Name:Songcrafter Probe\nManaCost:G U R\nTypes:Creature Human Bard\nPT:3/2\nK:Flash\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigGrant | TriggerDescription$ x\n" +
		"SVar:TrigGrant:DB$ Pump | ValidTgts$ Instant.YouCtrl,Sorcery.YouCtrl | TgtZone$ Graveyard | KW$ Harmonize | PumpZone$ Graveyard\nOracle:x\n"
)

// grantedHarmonizeFixture resolves the Songcrafter-shaped ETB onto a graveyard
// instant and returns the engine and that instant.
func grantedHarmonizeFixture(t *testing.T) (*Engine, state.ObjID) {
	t.Helper()
	e, _, mage := newFixtureDeck(t, 13, grantedHarmonizeMage, grantedHarmonizeMulch)
	id := addToGraveyard(t, e, 0, grantedHarmonizeMulch)
	f := e.G.Obj(id).Face()
	if f.HasKeyword("Harmonize") || e.hasKeywordH(id, kwhHarmonize) {
		t.Fatal("precondition: Harmonize present before the grant")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: mage, From: state.ZHand, To: state.ZBattlefield})
	e.Advance()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget || len(d.Options) != 1 || d.Options[0].Obj != id {
		t.Fatalf("target decision %+v", d)
	}
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 50)
	if e.G.Obj(id).Zone != state.ZGraveyard || e.G.Obj(id).Face().HasKeyword("Harmonize") || !e.hasKeywordH(id, kwhHarmonize) {
		t.Fatal("precondition: derived Harmonize on a graveyard card whose printed face lacks it")
	}
	return e, id
}

func grantedHarmonizeOffered(e *Engine, id state.ObjID) bool {
	for _, o := range e.legalActions(0) {
		if o.Kind == "cast" && o.Obj == id && o.Mode == "harmonize" {
			return true
		}
	}
	return false
}

func TestGrantedHarmonizeOffered(t *testing.T) {
	e, id := grantedHarmonizeFixture(t)
	if c, ok := e.harmonizeCostFor(id); !ok || c.Generic != 1 {
		t.Fatalf("harmonize cost = %+v ok=%v, want the mana cost {1}{G}", c, ok)
	}
	c, _ := e.harmonizeCostFor(id)
	// The 3-power Songcrafter itself is the untapped creature; tapped, it
	// credits nothing, so the two prices differ.
	var mage state.ObjID
	for _, cid := range e.G.Zone(state.ZBattlefield, 0) {
		if e.IsCreature(cid) {
			mage = cid
		}
	}
	if mage == 0 {
		t.Fatal("precondition: no creature on the battlefield")
	}
	e.G.Obj(mage).Tapped = true
	if base, _ := e.harmonizePayment(0, id, c); base.Generic != 1 {
		t.Fatalf("precondition: a tapped creature credits the cost: %+v", base)
	}
	e.G.Obj(mage).Tapped = false
	e.G.Players[0].Pool[state.MG] = 1
	if got, _ := e.harmonizePayment(0, id, c); got.Generic != 0 {
		t.Fatalf("3-power creature did not credit the generic: %+v", got)
	}
	if !grantedHarmonizeOffered(e, id) {
		t.Fatalf("granted harmonize not offered: %v", optKinds(e.legalActions(0)))
	}
}

func TestGrantedHarmonizeCastChargesManaCostAndExiles(t *testing.T) {
	e, id := grantedHarmonizeFixture(t)
	e.G.Players[0].Pool[state.MG], e.G.Players[0].Pool[state.MC] = 1, 1
	pool := e.G.Players[0].Pool
	for _, cid := range e.G.Zone(state.ZBattlefield, 0) {
		e.G.Obj(cid).Tapped = true // no creature credit: the full mana cost is due
	}
	if !grantedHarmonizeOffered(e, id) {
		t.Fatalf("precondition: not offered: %v", optKinds(e.legalActions(0)))
	}
	castMode(t, e, id, "harmonize")
	if e.G.Obj(id).Zone != state.ZStack {
		t.Fatalf("spell zone %s, want stack (pending %+v)", e.G.Obj(id).Zone, e.Pending())
	}
	if e.G.Players[0].Pool[state.MG] != 0 || e.G.Players[0].Pool[state.MC] != 0 {
		t.Fatalf("pool %v after cast, was %v: mana cost not charged", e.G.Players[0].Pool, pool)
	}
	passUntilStackEmpty(t, e, 50)
	if z := e.G.Obj(id).Zone; z != state.ZExile {
		t.Fatalf("harmonized spell ended in %s, want exile", z)
	}
}

func TestGrantedHarmonizeExpiresAtCleanup(t *testing.T) {
	e, id := grantedHarmonizeFixture(t)
	e.G.Players[0].Pool[state.MG], e.G.Players[0].Pool[state.MC] = 1, 1
	if !grantedHarmonizeOffered(e, id) {
		t.Fatal("precondition: not offered before cleanup")
	}
	passUntilTurn(t, e, e.G.Turn+1)
	if e.G.Obj(id).Zone != state.ZGraveyard {
		t.Fatalf("precondition: card left the graveyard: %s", e.G.Obj(id).Zone)
	}
	if e.hasKeywordH(id, kwhHarmonize) {
		t.Fatal("grant outlived the turn")
	}
	e.G.Players[0].Pool[state.MG], e.G.Players[0].Pool[state.MC] = 1, 1
	if grantedHarmonizeOffered(e, id) {
		t.Fatal("harmonize still offered after cleanup")
	}
}

const quietHarmonizeGrantSrc = "Name:Quiet Harmonize Grant\nTypes:Artifact\n" +
	"S:Mode$ Continuous | Affected$ Card.YouOwn+nonLand | AffectedZone$ Graveyard | AddKeyword$ Harmonize | Description$ x\n" +
	"Oracle:x\n"

func TestQuietDerivedGraveRouteBlocksGrantedHarmonize(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	spell := lookup(t, reg, "Divination")
	if spell.Faces[0].HasKeyword("Harmonize") {
		t.Fatal("precondition: Divination prints Harmonize")
	}
	grant := card(t, quietHarmonizeGrantSrc)
	e := quietBaseLandWith(t, reg, "Island", []*cards.Card{grant, spell})
	addZone(t, e, 0, grant, state.ZBattlefield)
	id := addZone(t, e, 0, spell, state.ZGraveyard)
	if !e.hasKeywordH(id, kwhHarmonize) {
		t.Fatal("precondition: the Harmonize grant did not reach the graveyard card")
	}
	if got := e.quietBlocker(0); got != qbGraveRoute {
		t.Fatalf("quietBlocker = %s, want %s", quietBlockerNames[got], quietBlockerNames[qbGraveRoute])
	}
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MU] = 2, 1
	e.priorityRound()
	if !grantedHarmonizeOffered(e, id) {
		t.Fatalf("walk did not offer granted harmonize: %v", optKinds(e.legalActions(0)))
	}
}
