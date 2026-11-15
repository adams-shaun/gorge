package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// trigObjMoved: the trigger's object (another card than the source) is no
// longer the object that triggered (CR 400.7; gorge every zone
// change makes a new object). A card that died while snapshot on the
// battlefield is the same object only at the version one past the snapshot
// (the death); without a snapshot only its zone is checked: the graveyard,
// or a zone the trigger's own leave line names as its destination (Kaya's
// Ghostform's "dies or is put into exile": the exiled host is that card,
// gorge's TriggeredNewCardLKICopy; trig_zonepairs.go).
func (s *State) trigObjMoved(k *prep, e *stackEntry, ob *object) bool {
	if ob.zone != kop.ZoneGraveyard && !trigLeftTo(k, e, ob.zone) {
		return true
	}
	return e.li != 0 && ob.ver != s.lk(e.li).ver+1
}

// trigLeftTo reports whether e is a battlefield-leave trigger whose
// destination set (a TrigLeaves line's Trigger.N) holds zone z.
func trigLeftTo(k *prep, e *stackEntry, z kop.Zone) bool {
	abs := k.t.Cards[e.card].Abilities
	if int(e.ab) >= len(abs) {
		return false
	}
	a := &abs[e.ab]
	return a.Kind == kop.AbTriggered && czPairLine(a) && a.Trig.N&(1<<uint(z)) != 0
}
