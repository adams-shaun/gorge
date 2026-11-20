package dirac

import "testing"

// TestPumpControlDropsSkipNonOverrideRecords pins that ovCtrlDrop and
// ovCtrlPhaseDrop read an override row only for override records: a layer-2
// control record (idx 0) in the pool used to index Overrides[255] and panic
// (TestEDHBenchDeterministic on a real gainControl).
func TestPumpControlDropsSkipNonOverrideRecords(t *testing.T) {
	sc, lc, idx := pumpCtrlScen(t)
	src, tgt := sc.perm(lc, 0), sc.perm(lc, 1)
	sc.pumpOn(tgt, src, 0, idx)
	if sc.s.g.nfx == 0 {
		t.Fatal("precondition: the pump left no record")
	}
	// a non-override record with idx 0 ahead of the grant
	sc.s.g.fx[sc.s.g.nfx] = sc.s.g.fx[0]
	sc.s.g.fx[0] = fxRec{sl: sc.slot(tgt), kind: fxControl}
	sc.s.g.nfx++
	sc.s.fxM.set(sc.slot(tgt))

	sc.s.ovCtrlDrop(sc.k, src, 1)
	if n := sc.pumpCtrlCount(tgt, idx); n != 0 {
		t.Fatalf("ovCtrlDrop kept %d grant records, want 0", n)
	}
	sc.pumpOn(tgt, src, 0, idx)
	sc.s.ovCtrlPhaseDrop(sc.k, src)
	if n := sc.pumpCtrlCount(tgt, idx); n != 0 {
		t.Fatalf("ovCtrlPhaseDrop kept %d grant records, want 0", n)
	}
	var kept bool
	for i := 0; i < int(sc.s.g.nfx); i++ {
		kept = kept || sc.s.g.fx[i].kind == fxControl
	}
	if !kept {
		t.Fatal("the control record was dropped")
	}
}
