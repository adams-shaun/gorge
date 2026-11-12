package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestChromaSourceCount pins the ChromaSource count head:
// Forge's AbilityUtils Chroma block, whose ChromaSource form counts the
// coloured mana symbols of the PASSED-IN source card's printed mana cost
// (CardLists.getTotalChroma over ManaCostShard.isColor). Before the
// evaluator every spelling answered zero, so Namor the Sub-Mariner's
// TriggeredCard$ChromaSource.Blue made no Merfolk and Light from Within's
// AffectedX added nothing. The ref and Sacrificed spellings ride the same
// per-object read (Forge's handlePaid and CostSacrifice's "Sacrificed" paid
// list).
func TestChromaSourceCount(t *testing.T) {
	h := newHost(t, 2)
	// "2 W W W/U U/P": White is W, W and the W/U hybrid -- 3; Blue is the
	// W/U and the U/P Phyrexian -- 2; every colour is those same 4 SYMBOLS
	// once each, deliberately not the naive per-letter count (5): a hybrid
	// is one ManaCostShard and Forge counts it once per query.
	carrier := mkCard(t, "Name:Chroma Carrier\nTypes:Creature Elemental\nPT:2/2\nManaCost:2 W W W/U U/P\nOracle:x\n")
	other := mkCard(t, "Name:Blue Carrier\nTypes:Creature Elemental\nPT:1/1\nManaCost:1 U U\nOracle:x\n")
	plain := mkCard(t, "Name:Plain Bear\nTypes:Creature Bear\nPT:1/1\nManaCost:2 R\nOracle:x\n")
	hidden := mkCard(t, "Name:Hidden Morph\nTypes:Creature Bear\nPT:2/2\nManaCost:1 U U\nOracle:x\n")
	co := h.g.AddObject(carrier, 0)
	oo := h.g.AddObject(other, 0)
	po := h.g.AddObject(plain, 0)
	fo := h.g.AddObject(hidden, 1)
	fo.Zone = state.ZBattlefield
	fo.FaceDown = true
	c := &Ctx{Source: co.ID, Controller: 0,
		Remembered: []state.Target{{Obj: oo.ID}},
		Sacrificed: []state.SacrificedInfo{{Obj: po.ID, ManaCost: "2 R"}}}
	for _, tc := range []struct {
		expr string
		want int32
	}{
		// The plain head reads the resolving SOURCE (Forge's passed-in c;
		// Light from Within's AffectedX anchors Ctx.Source on each affected
		// creature).
		{"Count$ChromaSource.White", 3},
		{"Count$ChromaSource.Blue", 2},
		{"Count$ChromaSource.Black", 0},
		{"Count$ChromaSource", 4},
		{"Count$ChromaSource.White/Twice", 6},
		// The ref spellings count the referenced objects' printed costs:
		// Namor's TriggeredCard$ChromaSource.Blue and Omnath's bare
		// Remembered$ChromaSource (every colour) are the corpus carriers.
		{"Remembered$ChromaSource.Blue", 2},
		{"Remembered$ChromaSource", 2},
		{"TriggeredCard$ChromaSource.Blue", 2},
		{"TriggeredCard$ChromaSource.Green", 0},
		// The sacrificed spelling reads the LKI capture (Fiery Bombardment),
		// /Op included.
		{"Sacrificed$ChromaSource.Red", 1},
		{"Sacrificed$ChromaSource.Blue", 0},
		{"Sacrificed$ChromaSource.Red/Twice", 2},
	} {
		if got := EvalCount(h, c, tc.expr); got != tc.want {
			t.Errorf("%s = %d, want %d", tc.expr, got, tc.want)
		}
	}
	// An unreadable colour fails closed (unresolved), not a fake zero.
	for _, expr := range []string{
		"Count$ChromaSource.Bogus",
		"Count$ChromaSource.",
		"Remembered$ChromaSource.Bogus",
		"Sacrificed$ChromaSource.Bogus",
	} {
		if n, ok := EvalCountOK(h, c, expr); ok || n != 0 {
			t.Errorf("%s = (%d, %v), want unresolved", expr, n, ok)
		}
	}
	// A face-down battlefield source has no mana cost (CR 708.5), the same
	// guard the devotion scan applies; the live read sees the flag.
	fd := &Ctx{Source: fo.ID, Controller: 1}
	if got := EvalCount(h, fd, "Count$ChromaSource.Blue"); got != 0 {
		t.Errorf("face-down Count$ChromaSource.Blue = %d, want 0", got)
	}
}

// TestChromaSourceSacrificedSnapshot pins the capture half: the Sacrificed$
// reader runs after the zone change, so the printed mana cost must ride the
// SacrificedInfo last-known-information snapshot (CR 608.2h) taken before
// the sacrifice -- the state population, not a hand-built one. The
// face-down flag rides it too, so a face-down sacrifice contributes nothing.
func TestChromaSourceSacrificedSnapshot(t *testing.T) {
	h := newHost(t, 2)
	plain := mkCard(t, "Name:Sac Fodder\nTypes:Creature Goblin\nPT:1/1\nManaCost:1 R R\nOracle:x\n")
	hidden := mkCard(t, "Name:Sac Hidden\nTypes:Creature Goblin\nPT:1/1\nManaCost:1 R R\nOracle:x\n")
	po := h.g.AddObject(plain, 0)
	po.Zone = state.ZBattlefield
	ho := h.g.AddObject(hidden, 0)
	ho.Zone = state.ZBattlefield
	ho.FaceDown = true
	snap := SacrificedLKI(h, po.ID)
	if snap.ManaCost != "1 R R" {
		t.Fatalf("snapshot ManaCost = %q, want %q", snap.ManaCost, "1 R R")
	}
	c := &Ctx{Source: po.ID, Controller: 0, Sacrificed: []state.SacrificedInfo{snap}}
	if got := EvalCount(h, c, "Sacrificed$ChromaSource.Red"); got != 2 {
		t.Errorf("Sacrificed$ChromaSource.Red = %d, want 2", got)
	}
	fdSnap := SacrificedLKI(h, ho.ID)
	if !fdSnap.FaceDown {
		t.Fatalf("face-down snapshot lost the FaceDown flag")
	}
	fdCtx := &Ctx{Source: ho.ID, Controller: 0, Sacrificed: []state.SacrificedInfo{fdSnap}}
	if got := EvalCount(h, fdCtx, "Sacrificed$ChromaSource.Red"); got != 0 {
		t.Errorf("face-down Sacrificed$ChromaSource.Red = %d, want 0", got)
	}
}
