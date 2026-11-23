package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// zone_stagecmd.go: the Commander part of NewFromWorld. A World may be a
// Commander game (CR 903): one commander a seat, in the command zone or any
// zone staging admits, with its command-zone cast count (CR 903.8) and the
// combat damage each player has taken from it (CR 903.10a). A partner pair, a
// second card in a command zone and a commander in exile stay out of scope.

// cmdNone reports whether o is the value of a seat with no commander: NoObj,
// or the zero value of a World that is not a Commander game.
func cmdNone(o Obj) bool { return o == NoObj || o == 0 }

// validateCommanders checks the Commander fields of w.
func (w *World) validateCommanders() error {
	for p := 0; p < MaxSeats; p++ {
		seated := w.Commander && p < int(w.NSeat) && w.Cmd[p] != NoObj
		if !seated {
			if !cmdNone(w.Cmd[p]) || w.CmdCasts[p] != 0 {
				return unstageable(ReasonCommander, "seat %d: a commander or cast count with no commander", p)
			}
			for q := 0; q < MaxSeats; q++ {
				if w.CmdDamage[q][p] != 0 {
					return unstageable(ReasonCommander, "seat %d: commander damage from no commander", p)
				}
			}
			continue
		}
		c := int(w.Cmd[p])
		if c >= len(w.Objs) {
			return unstageable(ReasonCommander, "seat %d commander %d outside the objects", p, c)
		}
		o := &w.Objs[c]
		def := &w.Tbl.Cards[o.Card] // validateObjs checked the card
		if o.Token || o.Owner != uint8(p) || def.Types&kop.TLegendary == 0 || def.Types&kop.TCreature == 0 {
			return unstageable(ReasonCommander, "seat %d commander %d (%s) is not its legendary creature", p, c, def.Name)
		}
		for q := int(w.NSeat); q < MaxSeats; q++ {
			if w.CmdDamage[q][p] != 0 {
				return unstageable(ReasonCommander, "seat %d: commander damage to seat %d", p, q)
			}
		}
	}
	return nil
}

// validateCommandObj checks object i of w standing in a command zone: only
// its owner's commander may, and a command zone holds no order.
func (w *World) validateCommandObj(i int) error {
	o := &w.Objs[i]
	if !w.Commander || o.Owner >= w.NSeat || w.Cmd[o.Owner] != Obj(i) || o.Pos != 0 {
		return unstageable(ReasonCommander, "object %d (seat %d) is not the only card of its command zone", i, o.Owner)
	}
	return nil
}

// cmdConfig is the Config.Commanders InitN seats w's commanders from: each is
// an index into its owner's deck as InitN lays it out (the seat's objects, in
// order, from its first).
func (w *World) cmdConfig() [MaxSeats][2]int16 {
	c := noCommanders()
	if !w.Commander {
		return c
	}
	var first [MaxSeats]int
	for i := range w.Objs {
		if o := &w.Objs[i]; !o.Token {
			for p := int(o.Owner) + 1; p < MaxSeats; p++ {
				first[p]++
			}
		}
	}
	for p := 0; p < int(w.NSeat); p++ {
		if w.Cmd[p] != NoObj {
			c[p] = [2]int16{int16(int(w.Cmd[p]) - first[p]), -1}
		}
	}
	return c
}

// stageCommanders sets the Commander bookkeeping of the staged game: cmdGenesis
// (InitN) seated each commander, and the objects were placed since.
func (s *State) stageCommanders(w *World) {
	for p := uint8(0); p < w.NSeat; p++ {
		c := w.Cmd[p]
		if c == NoObj {
			continue
		}
		*s.cmdCastAt(p, 0) = w.CmdCasts[p]
		for q := uint8(0); q < w.NSeat; q++ {
			s.pl[q].cmdDmg[p] = w.CmdDamage[q][p]
		}
		if s.objs[c].zone == kop.ZoneGraveyard {
			// a commander in a graveyard was offered the command zone and
			// declined, or the ask would be pending (CR 903.9a)
			*s.cmdAskAt(p, 0) = s.objs[c].ver
		}
	}
}
