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
// payment that sacrifices several permanents is also one zone batch (gorge
// opens one around them, CR 701.21a), so a ChangesZoneAll line latches in it
// too; a batch of one queues as a lone move would.

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

// payCommitSacs sacrifices the picks of an unless / if-you-do cost payment
// together, in pick order, in one cost-site sacrifice batch (gorge collects
// every pick, then PaySacrificeCost emits them, emitSacrifices). The picks
// stay on the battlefield until the last is answered (addSac), so a pick that
// ends a control effect does not drop the permanents it gave from the menu.
//
//go:noinline
func (s *State) payCommitSacs(k *prep) {
	n := int(s.g.nsac)
	if n == 0 {
		return
	}
	ord := s.g.sacOrd
	s.g.nsac, s.g.sacM = 0, slotMask{}
	if k.gen.anyBatch {
		defer s.batchClose(s.sacBatchOpen())
	}
	s.snapTrigSacrificed(k, ord[0]) // trig_sacrificedlki.go
	for _, sl := range ord[:n] {
		s.fireSac(k, sl)
		s.leaveParked(k, sl)
	}
}
