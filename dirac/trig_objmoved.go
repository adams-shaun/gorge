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
	if ob.zone != kop.ZoneGraveyard && !trigLeftTo(k, e, ob.zone) && !trigSacExiled(k, e, ob) {
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

// trigSacExiled reports whether ob is the card a Sacrificed trigger fired on,
// still in the exile a replacement (Rest in Peace) sent it to instead of the
// graveyard: a sacrifice trigger looks back in time (CR 603.10a) and finds the
// card in the first public zone it went to (CR 603.6c, 400.7e), though the
// script's Origin$ Graveyard assumes the graveyard (gorge
// effects.originSatisfiedInExile; It That Betrays). The version check in
// trigObjMoved still applies, so a card moved again since is a new object.
func trigSacExiled(k *prep, e *stackEntry, ob *object) bool {
	if ob.zone != kop.ZoneExile || e.li == 0 {
		return false
	}
	abs := k.t.Cards[e.card].Abilities
	return int(e.ab) < len(abs) && abs[e.ab].Kind == kop.AbTriggered && abs[e.ab].Trig.Event == kop.TrigSacrificed
}
