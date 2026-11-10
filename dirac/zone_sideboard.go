package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The sideboard: the cards "outside the game" (CR 400.11) that a play
// permission naming it reaches (The Holodeck: "a permanent card you own from
// outside the game or from your graveyard", Sideboard,Graveyard). A sideboard
// card is an object in the zone ZoneSideboard, numbered after both decks and
// before any token, and the zone has no list: it is the objects whose zone
// says so, found by a walk of the objects when a permission names the zone.
// Nothing ever moves a card into it, so no order is kept (gorge's sideboard
// is an unordered pool too: its options are canonical). The state grows by
// no bytes.

// DebugHolodeckNoSideboard is the mutation switch: the sideboard permission is
// never offered a card, so a game that plays from the sideboard diverges from
// gorge.
var DebugHolodeckNoSideboard bool

// initSideboards creates the objects of cfg's sideboards, seat 0's first,
// after the decks' objects.
func (s *State) initSideboards(cfg *Config) {
	for p := 0; p < int(s.nSeats()); p++ {
		n := int(cfg.SideN[p])
		if n > MaxSide {
			panic("kernel: sideboard larger than MaxSide")
		}
		for _, c := range cfg.Side[p][:n] {
			if int(s.nobj) >= MaxObj {
				panic("kernel: decks and sideboards larger than MaxObj")
			}
			o := Obj(s.nobj)
			s.nobj++
			s.objs[o] = object{card: c, chosen: 255, zone: kop.ZoneSideboard, slot: NoSlot, acts: uint8(p) << objOwnerShift}
			s.touch(o)
		}
	}
}

// eachSideboard calls f for every card in p's sideboard, in object order.
func (s *State) eachSideboard(p uint8, f func(Obj)) {
	for o := Obj(0); int(o) < int(s.nobj); o++ {
		if ob := &s.objs[o]; ob.zone == kop.ZoneSideboard && ob.owner() == p {
			f(o)
		}
	}
}

// mayPlayFromZone reports whether the MayPlayFrom static st reaches a card
// in zone z: its zone, or the sideboard when it also covers that
// (StAlsoSideboard).
func mayPlayFromZone(st *kop.Static, z kop.Zone) bool {
	return st.Zone == z || z == kop.ZoneSideboard && st.StFlags&kop.StAlsoSideboard != 0
}

// mayPlayZoneBits is the zones MayPlayFrom static st plays from, a bit per
// kop.Zone.
func mayPlayZoneBits(st *kop.Static) uint16 {
	if st.StFlags&kop.StFreeOnly != 0 {
		return 1 << kop.ZoneHand // opens no zone; a hand cast needs no permission
	}
	z := uint16(1) << st.Zone
	if st.StFlags&kop.StAlsoSideboard != 0 {
		z |= 1 << kop.ZoneSideboard
	}
	return z
}

// spendLandStatic uses the turn of the once-per-turn MayPlayFrom static that
// lets p play land o from a non-hand zone, as a cast marks the static it was
// cast through (castFlagsGen): gorge's land play consumes the static's key
// (mayPlayLandPermKey) unless an unlimited static covers the land too.
//
//go:noinline
func (s *State) spendLandStatic(k *prep, o Obj, p uint8) {
	s.mayPlayStatics(k, p, o, func(st statSrc) bool {
		if st.a.Static.StFlags&kop.StOncePerTurn != 0 && st.sl != NoSlot && !s.mayPlayFolds(k, p, o) {
			s.markUsed(st.obj, uint8(abIndex(k, s.cardAt(st.sl), st.a)))
		}
		return false
	})
}
