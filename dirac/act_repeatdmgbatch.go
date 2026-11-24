package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A RepeatEach over players with DamageMap$ True (Acidic Soil, Typhoon,
// Heartless Hidetsugu: "deals damage to each player equal to ...") is one
// damage batch: gorge's effRepeatEach opens its damage bracket around the
// whole loop (BeginDamageBatch) and closes it after the last iteration, so a
// DamageDealtOnce line latches once over the loop's events and its amount is
// patched to the batch total (trig_dmgbatch.go). OpRepeatEachPlayer opens the
// bracket (kop.RepeatEachDmgBatch); loopBackGen closes it when the last
// player is done. The batch lives in genState (dmgBOn), so it spans a body's
// replacement-order ask as a DamageAll sweep's does.

// repeatDmgBatchClose ends the loop's damage batch when the loop that just
// finished opened one: its header, the instruction before the body, carries
// RepeatEachDmgBatch.
func (s *State) repeatDmgBatchClose(k *prep) {
	if !s.g.dmgBOn || s.g.repPC == 0 {
		return
	}
	if in := &s.prog(k)[s.g.repPC-1]; in.Op == kop.OpRepeatEachPlayer && in.C&kop.RepeatEachDmgBatch != 0 {
		s.dmgBatchClose()
	}
}

// repeatLoopOpen opens OpRepeatEachPlayer's batches: ChangeZoneTable$'s zone
// batch (zone_repeatbatch.go) and DamageMap$'s damage batch.
func (s *State) repeatLoopOpen(in *kop.Instr) {
	s.zoneLoopOpen(in)
	if in.C&kop.RepeatEachDmgBatch != 0 {
		s.dmgBatchOpen()
	}
}
