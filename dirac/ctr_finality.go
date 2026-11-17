package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// prepFinality turns dieZone's finality check on for a table whose programs
// put finality counters (Zoraline's "with a finality counter on
// it": OpReturnEx with counter kind CtrFinality in B, OpPutCounter with it in
// C, a grave pick's entry counter in B bits 4..11 — Hand,Graveyard or plain
// Graveyard), and a mass return onto the battlefield with one (Shilgengar's
// "with a finality counter on it": OpChangeZoneAllEx with MvEntryRiders, the
// kind in A bits 8..13 — zone_sweepriders.go).
func prepFinality(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpReturnEx && in.B.K == kop.VConst && kop.CounterKind(in.B.N) == kop.CtrFinality ||
					in.Op == kop.OpPutCounter && kop.CounterKind(in.C) == kop.CtrFinality ||
					sweepFinalityCounter(&in) ||
					handGraveCounter(&in, t.Filters[kop.InstrFilter(&in)].Zone, kop.CtrFinality) {
					g.anyFinality = true
					return
				}
			}
		}
	}
}

// sweepFinalityCounter reports a ChangeZoneAll sweep onto the battlefield whose
// entrants get a finality counter from WithCountersType$ (the MvEntryRiders
// form: A is the constant count | counter kind<<8, masked by EntryKindMask so
// the AtEOT$ bits do not join the kind).
func sweepFinalityCounter(in *kop.Instr) bool {
	return in.Op == kop.OpChangeZoneAllEx && in.C>>3&kop.MvEntryRiders != 0 &&
		in.A.K == kop.VConst && kop.CounterKind((in.A.N>>8)&kop.EntryKindMask) == kop.CtrFinality
}
