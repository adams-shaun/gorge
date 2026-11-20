package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Pump Duration$ UntilYourNextUpkeep (CR 611.2b; Erhnam Djinn): a keyword
// grant lasts through its controller's untap step and ends as their upkeep
// begins (gorge dropUntilYourNextUpkeep, called from setStep before the
// StepChange emit, so a Phase$ Upkeep trigger never sees it).
//
// The class lives on the Override row (kop.Override.UntilUpkeep, read as
// hostLeft reads WhileHost), so no ovRec.fl bit is spent: ovRec.fl's bit 6 is
// the next-turn flag and bit 5 the copy grant. The waiting controller is the
// applied record's p, which a keyword-only grant leaves free (a WhileHost
// record keeps its host object and version there); ovNTWho's 0x30 bits are
// not used because they overlap ovCopyGrant whenever ovNT is clear. A
// controller that leaves the game never becomes active again, so gorge's
// predicate keeps the grant and so does this; that is not ovNT's frozen
// boundary (static_ovntleave.go), whose next turn has to be computable.

// DebugNoUpkeepDrop is the mutation switch: an until-your-next-upkeep record
// is never dropped, so a lockstep or unit test that expects the drop must
// fail.
var DebugNoUpkeepDrop bool

// useUpkeepOv is the genPrep.uses bit for such a table: a registered use, not a
// genPrep field, so the fixed layout (TestNoNewFixedStateFields) is unchanged.
var useUpkeepOv = newGenUse(tableHasUpkeepOv)

// tableHasUpkeepOv reports a table that carries an OpOverride /
// OpOverrideAll with DurUntilYourNextUpkeep, prepareGen's gate for
// genPrep.uses' useUpkeepOv bit.
func tableHasUpkeepOv(t *kop.Table) bool {
	hit := func(prog []kop.Instr) bool {
		for i := range prog {
			in := &prog[i]
			if (in.Op == kop.OpOverride || in.Op == kop.OpOverrideAll) && in.A.K == kop.VConst && in.A.N == kop.DurUntilYourNextUpkeep {
				return true
			}
		}
		return false
	}
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if hit(t.Cards[i].Abilities[j].Prog) {
				return true
			}
		}
	}
	for i := range t.Emblems {
		for j := range t.Emblems[i].Abilities {
			if hit(t.Emblems[i].Abilities[j].Prog) {
				return true
			}
		}
	}
	return false
}

// upkeepStep is beginStep's StepUpkeep entry: it drops the
// until-your-next-upkeep records before the step's triggers are queued.
func (s *State) upkeepStep(k *prep) {
	if k.gen != nil && k.gen.uses.has(useUpkeepOv) {
		s.dropUpkeepOv(k)
	}
}

// dropUpkeepOv ends the until-your-next-upkeep override records whose
// controller is the active player. It runs as StepUpkeep is entered, before
// firePhase queues the upkeep triggers (gorge drops before the StepChange
// emit).
func (s *State) dropUpkeepOv(k *prep) {
	if DebugNoUpkeepDrop {
		return
	}
	n := 0
	s.fxM = slotMask{}
	for i := 0; i < int(s.g.nfx); i++ {
		r := &s.g.fx[i]
		if r.kind == fxOverride && k.t.Overrides[r.r.idx-1].UntilUpkeep && int16(s.active) == r.r.p {
			s.touchSl(r.sl)
			continue
		}
		s.g.fx[n] = *r
		s.fxM.set(r.sl)
		n++
	}
	for i := n; i < int(s.g.nfx); i++ {
		s.g.fx[i] = fxRec{}
	}
	s.g.nfx = uint8(n)
}
