package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// inExile places a card in its owner's exile the eventless way onBoard places
// one on the battlefield, staling the derived memos a real move would bump.
func inExile(t testing.TB, e *Engine, p state.PlayerID, src string) state.ObjID {
	t.Helper()
	o := e.G.AddObject(card(t, src), p)
	o.Zone = state.ZExile
	e.G.SetZone(state.ZExile, p, append(e.G.Zone(state.ZExile, p), o.ID))
	e.staticEpoch = -1
	e.activeEpoch = -1
	e.typesEpoch = -1
	return o.ID
}

// TestImprintedCDAPowerAndToughnessAgreeEveryZone pins CR 604.3/208.2a on the
// <Ref>$<Property> read behind a P/T CDA off the battlefield: a characteristic
// P/T functions in EVERY zone, so an Imprinted$CardPower and
// Imprinted$CardToughness pair over the same exiled card cannot disagree.
// Phyrexian Ingester's "+X/+Y, where X is the exiled creature card's power and
// Y is its toughness" over an exiled counting CDA (Abominable Treefolk, a */*
// equal to the number of snow permanents you control) once read the power
// through the layer walk and the toughness off the printed star, so the
// Ingester sat at 4/3 over a 1/1 exile while the kernel read 4/4.
func TestImprintedCDAPowerAndToughnessAgreeEveryZone(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	yeti := onBoard(t, e, 0, "Name:Yeti\nManaCost:3 G\nTypes:Snow Creature Yeti\nPT:3/3\nOracle:x\n")
	tree := inExile(t, e, 0, "Name:Treefolk\nManaCost:2 G U\nTypes:Snow Creature Treefolk\nPT:*/*\nK:Trample\n"+
		"S:Mode$ Continuous | SetToughness$ N | SetPower$ N | CharacteristicDefining$ True\n"+
		"SVar:N:Count$Valid Permanent.YouCtrl+Snow\nOracle:x\n")
	ing := onBoard(t, e, 0, "Name:Ingester\nManaCost:4 G G\nTypes:Creature Beast\nPT:3/3\n"+
		"S:Mode$ Continuous | AddPower$ P | AddToughness$ T\n"+
		"SVar:T:Imprinted$CardToughness\nSVar:P:Imprinted$CardPower\nOracle:x\n")
	e.G.Obj(ing).Imprinted = []state.ObjID{tree}

	// Preconditions: the exile really holds the counting CDA (one snow
	// permanent, the Yeti -- the Treefolk itself left the battlefield), the
	// card really carries it, and the association really binds.
	to := e.G.Obj(tree)
	if to == nil || to.Face() == nil || !to.Face().CharacteristicDefining() {
		t.Fatal("precondition: the exiled card carries no P/T CDA")
	}
	td := e.Derived(tree)
	if td.Power != 1 || td.Toughness != 1 {
		t.Fatalf("precondition: exiled Treefolk in exile = %d/%d, want 1/1 (the Yeti alone)", td.Power, td.Toughness)
	}
	if e.G.Obj(ing).Imprinted == nil || e.G.Obj(ing).Imprinted[0] != tree || e.G.Obj(yeti) == nil {
		t.Fatal("precondition: the imprint association or the snow permanent is missing")
	}

	d := e.Derived(ing)
	// Both heads read the SAME 1/1 the card carries in exile: 3/3 + 1/1. The
	// power head alone cannot be right (4) while the toughness head stays
	// printed (3) -- the asymmetric 4/3 is the bug.
	if d.Power != 4 || d.Toughness != 4 {
		t.Fatalf("Ingester over the exiled CDA = %d/%d, want 4/4 (CR 604.3: the CDA works in exile for both heads)", d.Power, d.Toughness)
	}
}
