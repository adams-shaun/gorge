package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// prepFinality turns dieZone's finality check on for a table whose programs
// put finality counters (Zoraline's "with a finality counter on
// it": OpReturnEx with counter kind CtrFinality in B, OpPutCounter with it in
// C, a grave pick's entry counter in B bits 4..11 — Hand,Graveyard or plain
// Graveyard).
func prepFinality(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpReturnEx && in.B.K == kop.VConst && kop.CounterKind(in.B.N) == kop.CtrFinality ||
					in.Op == kop.OpPutCounter && kop.CounterKind(in.C) == kop.CtrFinality ||
					handGraveCounter(&in, t.Filters[kop.InstrFilter(&in)].Zone, kop.CtrFinality) {
					g.anyFinality = true
					return
				}
			}
		}
	}
}
