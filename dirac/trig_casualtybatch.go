package dirac

// kernel follows for the gorge pin. Cold and out of
// line.

// casBatch is the zone batch of an SBA pass's creature casualties (lethal
// damage and zero toughness, the prefix of sba's order). gorge's
// destroyLethalDamage brackets those moves with openZoneBatch /
// closeZoneBatch (rules/sba.go; CR 704.3, 603.2c, 603.10a): a ChangesZoneAll
// line latches once inside it and an entering object's enters check is held
// to the close. Planeswalkers, battles and Auras are outside it.
type casBatch struct {
	hr    holdRec
	bo    uint8
	batch bool // the leave batch is open
	held  bool // the enters hold is open
}

// casOpen opens the casualty batch and hold before the first fireLeave. Both
// are no-ops on a table without a batch line (anyBatch) or a held entry
// (anyLinkRet), exactly as leaveAll's are.
//
//go:noinline
func (s *State) casOpen(k *prep) casBatch {
	var c casBatch
	c.hr = s.holdOpen(k)
	c.held = c.hr.on
	if k.gen.anyBatch {
		c.bo = s.batchOpen()
		c.batch = true
	}
	return c
}

// batchEnd closes the leave batch (after the last casualty's fireLeave).
//
//go:noinline
func (c *casBatch) batchEnd(s *State) {
	if c.batch {
		s.batchClose(c.bo)
		c.batch = false
	}
}

// holdEnd closes the enters hold (after the last casualty's removePerm).
//
//go:noinline
func (c *casBatch) holdEnd(s *State, k *prep) {
	if c.held {
		s.holdClose(k, c.hr)
		c.held = false
	}
}
