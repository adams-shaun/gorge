package dirac

// The kernel follow-ons of gorge ..81 (pin ec2218895).
// Cold and out of line.

// --- SacrificedOnce once per sacrifice batch ---
//
// gorge latches Mode$ SacrificedOnce once per sacrifice batch (CR 603.2c;
// trigger_match.go checkFaceTriggers): an effect's departure batch
// (BatchDepartures, the kernel's flushSac / leaveAll batches) and one cost
// payment's sacrifices (once_batches.go emitSacrifices: a cast's or an
// activation's cost picks with a sacrificed source, an unless / triggered
// cost's picks). kgen lowers the line as Sacrificed with Tf2Batch. A cost
// payment's batch is a sacrifice batch only, not a zone batch (gorge opens
// no zone batch there): batchSac makes the latch collapse only Sacrificed
// lines inside it, so a ChangesZoneAll line still fires per move.

// batchSac: the open batch is a cost-site sacrifice batch.
const batchSac uint8 = 1 << 5

// sacBatchOpen opens a cost-site sacrifice batch unless a batch is open
// (gorge's brackets nest, the outermost counts); batchClose closes it.
func (s *State) sacBatchOpen() uint8 {
	old := s.g.batchLatch
	if old&batchOpenMask == 0 {
		s.g.batchLatch = old | batchSac | (s.npend+1)&batchOpenMask
	}
	return old
}

// payPickSacBatch opens an unless / if-you-do cost's sacrifice batch at its
// first sacrificed pick (gorge pays the picks together, emitSacrifices);
// the kernel sacrifices each pick as it is answered, so the batch stays open
// until the last pick (raGenPayPick closes it). An open batch is kept.
//
//go:noinline
func (s *State) payPickSacBatch(k *prep) {
	if s.g.batchLatch&batchOpenMask == 0 {
		s.sacBatchOpen()
	}
}
