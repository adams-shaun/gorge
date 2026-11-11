package dirac

// A seat's second commander, a partner pair's (CR 702.124), and its
// bookkeeping live in a cold record, not in player: State's layout is pinned
// (statelayout_test.go) and its reserve is spent. Slot 0 is the player's
// cmd, cmdCast, cmdAsk and cmdDmg; slot 1 is this record's.

// cmdPartner is slot 1 of every seat's commander bookkeeping.
type cmdPartner struct {
	cmd  [MaxSeats]Obj             // the seat's partner, NoObj none (set at Init)
	cast [MaxSeats]uint8           // its casts from the command zone (CR 903.8: its own tax)
	ask  [MaxSeats]uint8           // its ver at its last CR 903.9a ask
	dmg  [MaxSeats][MaxSeats]uint8 // dmg[p][q]: combat damage player p was dealt by seat q's partner (CR 903.10a)
	// last bit p: seat p's partner entered the command zone after its first
	// commander, so the command-zone casts offer it second (gorge's zone order)
	last uint8
}

// partnerCold is s's partner record, nil for a State never Init'd.
func (s *State) partnerCold() *cmdPartner {
	l := s.cold.layout
	if l == nil || l.partner == 0 {
		return nil
	}
	at := 0
	_ = coldView[mayPlay](nil, &at, l.may)
	_ = coldView[embRec](nil, &at, l.emb)
	_ = coldView[uint8](nil, &at, l.ctrP)
	return &coldView[cmdPartner](s.cold.buf, &at, l.partner)[0]
}

// initPartners empties every seat's partner slot (a zeroed record would name
// object 0, a real card).
func (s *State) initPartners() {
	if c := s.partnerCold(); c != nil {
		for p := range c.cmd {
			c.cmd[p] = NoObj
		}
	}
}

// cmdObj is seat p's commander in slot j (1: a partner), NoObj none.
func (s *State) cmdObj(p uint8, j int) Obj {
	if j == 0 {
		return s.pl[p].cmd
	}
	if c := s.partnerCold(); c != nil {
		return c.cmd[p]
	}
	return NoObj
}

// cmdCastAt is the command-zone cast count of seat p's commander in slot j
// (CR 903.8: each commander its own tax). Slot 1 needs the record, which
// any seated partner implies.
func (s *State) cmdCastAt(p uint8, j int) *uint8 {
	if j == 0 {
		return &s.pl[p].cmdCast
	}
	return &s.partnerCold().cast[p]
}

// cmdAskAt is the ver at the last CR 903.9a ask of seat p's commander in
// slot j.
func (s *State) cmdAskAt(p uint8, j int) *uint8 {
	if j == 0 {
		return &s.pl[p].cmdAsk
	}
	return &s.partnerCold().ask[p]
}

// cmdDmgAt is the combat damage player p was dealt by seat q's commander in
// slot j (CR 903.10a: each commander its own tally).
func (s *State) cmdDmgAt(p, q uint8, j int) *uint8 {
	if j == 0 {
		return &s.pl[p].cmdDmg[q]
	}
	return &s.partnerCold().dmg[p][q]
}

// cmdDmg2 is cmdDmgAt's slot 1 read, 0 without the record.
func (s *State) cmdDmg2(p, q uint8) uint8 {
	if c := s.partnerCold(); c != nil {
		return c.dmg[p][q]
	}
	return 0
}

// cmdCast2 is cmdCastAt's slot 1 read, 0 without the record.
func (s *State) cmdCast2(p uint8) uint8 {
	if c := s.partnerCold(); c != nil {
		return c.cast[p]
	}
	return 0
}

// cmdSlot is o's slot among seat p's commanders, -1 when o is not one.
func (s *State) cmdSlot(p uint8, o Obj) int {
	switch {
	case o == NoObj:
		return -1
	case s.pl[p].cmd == o:
		return 0
	case s.cmdObj(p, 1) == o:
		return 1
	}
	return -1
}

// partnerLast reports whether seat p's partner entered the command zone after
// its first commander.
func (s *State) partnerLast(p uint8) bool {
	c := s.partnerCold()
	return c != nil && c.last&(1<<p) != 0
}

// noteCommandZone records that o, if a commander of its owner, just entered
// the command zone: gorge's command zone is an ordered list, and the
// command-zone casts are offered in its order (cast_commander.go).
func (s *State) noteCommandZone(o Obj) {
	p := s.objs[o].owner()
	c := s.partnerCold()
	if c == nil {
		return
	}
	switch s.cmdSlot(p, o) {
	case 0:
		c.last &^= 1 << p
	case 1:
		c.last |= 1 << p
	}
}
