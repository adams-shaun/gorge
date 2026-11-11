package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An Effect's OneOff$ trigger (kop.Emblem.Once) is a one-shot delayed
// trigger in gorge: the emblem is gone once it has fired. A NumAmount emblem
// carries its frozen number as the trigger amount (The Eagles Are Coming!:
// at the next upkeep, a Bird for each creature the spell returned).

// embTrigAmt is the amount emblem i of player p queues its trigger with: its
// frozen number for a NumAmount emblem, else the event's amt.
func (s *State) embTrigAmt(e *kop.Emblem, p uint8, i int, amt int16) int16 {
	if e.NumAmount {
		return int16(s.embNum(p, i))
	}
	return amt
}
