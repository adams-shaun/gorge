package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFDCGapsLowerings: the five coverage-ticket cards this round lowers,
// over their printed scripts (cli-20261125T191555Z-fdbcc0d7).
//
//   - Endless Atlas: Count$MostCardName Land.YouCtrl lowers to
//     ExMostCardName over the land filter, the activation's GE3 gate reading
//     it (kop.ExMostCardName, dirac/value_mostcardname.go).
//   - Padeem, Consul of Innovation: the greatestCMC_Artifact property arm
//     sets the count filter's CMC-== bound to the greatest mana value over
//     every battlefield artifact (greatestCMCFilter's RepeatEach twin shape).
//   - Haven of the Spirit Dragon: Planeswalker.Ugin is a planeswalker
//     subtype word (dirac.NonCreatureSubtype), so the ChangeZone target
//     filter lowers with the Ugin subtype clause.
//   - Fateful Absence: Investigate Defined$ TargetedController mints the Clue
//     to the Destroy target's controller (rTargetController).
//   - Chandra's Ignition: DamageAll DamageSource$ ParentTarget beside
//     ValidPlayers$ Player.Opponent lowers to SweepFrom with the
//     opponents-only player arm (C bit 4).
func TestFDCGapsLowerings(t *testing.T) {
	lw := lowerNamedPool(t, []string{
		"Endless Atlas", "Padeem, Consul of Innovation", "Haven of the Spirit Dragon",
		"Fateful Absence", "Chandra's Ignition",
	})
	byName := map[string]*lCard{}
	for _, c := range lw.cards {
		byName[c.Name] = c
	}
	for _, n := range []string{
		"Endless Atlas", "Padeem, Consul of Innovation", "Haven of the Spirit Dragon",
		"Fateful Absence", "Chandra's Ignition",
	} {
		if c := byName[n]; c == nil {
			t.Fatalf("%s: not lowered", n)
		} else if len(c.Refusals) != 0 {
			t.Errorf("%s refused: %s", n, abReasons(c))
		}
	}

	// Endless Atlas: the gate reads ExMostCardName over Land+YouCtrl.
	ea := byName["Endless Atlas"]
	var gate lCond
	for _, ab := range ea.Abilities {
		if ab.ActCond.Kind != cNone {
			gate = ab.ActCond
		}
	}
	if gate.Kind != cValue || gate.A.Kind != vMostCardName || gate.Op != opGE || gate.B.N != 3 {
		t.Errorf("Endless Atlas gate = %+v, want GE 3 over MostCardName", gate)
	} else if gate.A.F == nil || len(gate.A.F.Clauses) != 1 || gate.A.F.Clauses[0].Types != tLand {
		t.Errorf("Endless Atlas spec filter = %+v, want one Land clause", gate.A.F)
	}

	// Padeem: the count is the artifact count whose CMC equals the greatest
	// artifact mana value on the battlefield.
	pd := byName["Padeem, Consul of Innovation"]
	var trig *lTrigger
	for i := range pd.Abilities {
		if pd.Abilities[i].Trigger != nil {
			trig = pd.Abilities[i].Trigger
		}
	}
	if trig == nil || trig.Cond.Kind != cValue || trig.Cond.A.Kind != vCount || trig.Cond.Op != opGE || trig.Cond.B.N != 1 {
		t.Fatalf("Padeem trigger condition = %+v, want the CheckSVar count", trig)
	}
	fs := trig.Cond.A.F.Clauses
	if len(fs) != 1 || fs[0].Types != tArtifact || fs[0].Ctrl != whoYou || fs[0].CmpStat != cmpCMC || fs[0].CmpOp != opEQ {
		t.Fatalf("Padeem count filter clause = %+v, want the greatest-CMC artifact arm", fs)
	}
	if fs[0].CmpV == nil || fs[0].CmpV.Kind != vValidAgg || fs[0].CmpV.N != kop.AggGreatestMV {
		t.Fatalf("Padeem CMC bound = %+v, want the greatest mana value over the artifacts", fs[0].CmpV)
	} else if len(fs[0].CmpV.F.Clauses) != 1 || fs[0].CmpV.F.Clauses[0].Types != tArtifact {
		t.Fatalf("Padeem comparison-set filter = %+v, want the plain battlefield artifacts", fs[0].CmpV.F)
	}

	// Haven: the target filter's second alternative names the Ugin subtype.
	hv := byName["Haven of the Spirit Dragon"]
	ugin := false
	for _, ab := range hv.Abilities {
		for _, tg := range ab.Targets {
			if tg.F == nil {
				continue
			}
			for _, c := range tg.F.Clauses {
				if c.Subtype == "Ugin" || c.Subtype2 == "Ugin" {
					ugin = true
				}
			}
		}
	}
	if !ugin {
		t.Error("Haven: no target filter clause names the Ugin subtype")
	}

	// Fateful Absence: the Clue mints to the destroyed target's controller.
	fa := byName["Fateful Absence"]
	mint := false
	for _, ab := range fa.Abilities {
		for _, in := range ab.Prog {
			if in.Op == "CreateToken" && in.Obj.Kind == rTargetController {
				mint = true
			}
		}
	}
	if !mint {
		t.Error("Fateful Absence: no CreateToken mints to the target's controller")
	}

	// Chandra's Ignition: the sweep carries the opponents-only player arm.
	ci := byName["Chandra's Ignition"]
	sweep := false
	for _, ab := range ci.Abilities {
		for _, in := range ab.Prog {
			if in.Op != "SweepFrom" {
				continue
			}
			sweep = true
			if in.C&1 == 0 || in.C&4 == 0 || in.C&2 != 0 {
				t.Errorf("Chandra's Ignition SweepFrom C = %#x, want NotSrc | opponents (no all-players arm)", in.C)
			}
			if in.Obj.Kind != rTarget || in.Obj.N < 0 {
				t.Errorf("Chandra's Ignition sweep source = %+v, want the parent target", in.Obj)
			}
		}
	}
	if !sweep {
		t.Fatal("Chandra's Ignition: no SweepFrom instruction lowered")
	}

	// and the other two Count$MostCardName carriers lower (Sceptre of Eternal
	// Glory's gated mana ability is TestSceptreLowering's).
	lw2 := lowerNamedPool(t, []string{"Mechanized Production", "Chrome Replicator"})
	for _, c := range lw2.cards {
		if len(c.Refusals) != 0 {
			t.Errorf("%s refused: %s", c.Name, abReasons(c))
		}
	}
}
