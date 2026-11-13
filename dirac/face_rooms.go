package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Rooms (CR 709.5; gorge rules face_rooms.go), cold: reached only
// when the table holds a two-door Room (prep.anyRoom, kop.FaceRoom).
//
// A Room's door state is its CardID (kop.CardDef.Faces: the two doors as
// plain split halves, then the six other battlefield states, kop.RoomState),
// so State holds no door bits. Each state def carries exactly the lines
// gorge's room-aware scans walk in that state (the FaceIdx door's while it
// is unlocked, the other door's once unlocked) and the unlocked doors'
// summed mana value. A cast Room enters as its plain face def (its cast
// door unlocked, gorge CastDoor); one that enters any other way, or a spell
// copy, enters with no door unlocked (gorge apply_zone.go: CastDoor =
// enteredFrom == ZStack && !IsCopy). Unlike every other split card, the
// face persists in every zone (gorge's split stack-exit reset excludes
// Rooms): leaving the battlefield turns the object back to the plain def of
// the face it shows.
//
// Either door is cast from the hand: the plain cast casts the face the card
// shows, and gorge's "room_alt" (AkCast castFace, target = the other face)
// turns the other door up first; a reversed room_alt turns the card back
// (gorge pc.faceBefore), a reversed plain cast leaves it.
//
// The unlock special action (CR 709.5e-f, gorge legal_walk_battlefield.go:
// AkUnlock, mode = the door) pays the door's printed cost from the floating
// pool at sorcery timing (gorge PayMana from the pool; the oracle drops an
// unpaid potential unlock) and swaps the permanent to the state with that
// door unlocked. The unlocked door's "when you unlock this door" triggers
// (Mode$ UnlockDoor) are fired on the Room itself; so are the cast door's
// as a cast Room enters, after its ordinary enters triggers (gorge
// trigger_match.go checkRoomEntryUnlockTriggers runs after the face walk).

// modeRoomAlt is the stackEntry.mode bit of a room_alt cast (chosen modes
// are bits 0..3, seFaceDown 4, the warp provenance 6..7): its reversal
// turns the card back to the face it showed.
const modeRoomAlt = 1 << 5

// Debug mutations for TestRoomsMutation (dirac/oracle): each breaks one
// gorge Room rule and must diverge in lockstep.
var (
	DebugRoomAllLive   bool // a Room enters with both doors unlocked
	DebugRoomFaceReset bool // a Room turns to face 0 on leaving the stack or the battlefield
	DebugRoomMVFace    bool // a battlefield Room's mana value is its face's printed one
)

// RoomStats counts the lifted Room mechanics; oracle tests only.
type RoomStats struct {
	AltCasts      int // room_alt casts begun
	Face1Casts    int // plain casts of a Room showing face 1
	UnlockOther   int // unlocks of the non-FaceIdx door
	UnlockFace    int // unlocks of the FaceIdx door (an uncast Room)
	EntryTrigs    int // cast-door UnlockDoor triggers queued on entry
	UnlockTrigs   int // UnlockDoor triggers queued by an unlock
	UncastEntries int // Rooms entering with no door unlocked
	Face1Leaves   int // Rooms showing face 1 leaving the battlefield (face kept)
	MVOneDoor     int // battlefield Room mana-value reads, one door unlocked
	MVTwoDoors    int // ... both doors unlocked
}

// DebugRoomStats is the test hook; nil in normal play.
var DebugRoomStats *RoomStats

// roomCastStat counts a Room cast as it begins.
func roomCastStat(k *prep, c CardID, mode uint8) {
	if DebugRoomStats == nil || !roomDef(k, c) {
		return
	}
	if mode&castFace != 0 {
		DebugRoomStats.AltCasts++
	} else if k.t.Cards[c].Face == 1 {
		DebugRoomStats.Face1Casts++
	}
}

// roomDef reports whether card c is a Room def.
func roomDef(k *prep, c CardID) bool {
	cd := &k.t.Cards[c]
	return cd.FaceMode == kop.FaceRoom && len(cd.Faces) == 8
}

// roomMask is the door mask (bit i = door i unlocked) of Room def c on the
// battlefield.
func roomMask(k *prep, c CardID) uint8 {
	cd := &k.t.Cards[c]
	f := cd.Face
	for _, m := range [4]uint8{0, 1, 2, 3} {
		if cd.Faces[kop.RoomState(f, m)] == c {
			return m
		}
	}
	return 1 << f
}

// roomEnter runs in enterNF before the permanent is built: a Room that did
// not resolve as a cast spell enters with no door unlocked.
func (s *State) roomEnter(k *prep, o Obj, from kop.Zone) {
	ob := &s.objs[o]
	if !roomDef(k, ob.card) {
		return
	}
	cd := &k.t.Cards[ob.card]
	m := uint8(0)
	if DebugRoomAllLive {
		m = 3
	} else if from == kop.ZoneStack && !s.isSpellCopy(o) {
		return // the cast door is unlocked: its plain def
	}
	if DebugRoomStats != nil {
		DebugRoomStats.UncastEntries++
	}
	ob.card = cd.Faces[kop.RoomState(cd.Face, m)]
}

// roomLeave turns a Room that left the battlefield into the plain def of
// the face it shows (gorge clears the doors and keeps FaceIdx).
func (s *State) roomLeave(k *prep, o Obj) bool {
	ob := &s.objs[o]
	if !roomDef(k, ob.card) {
		return false
	}
	cd := &k.t.Cards[ob.card]
	c := cd.Faces[cd.Face]
	if DebugRoomStats != nil && cd.Face == 1 {
		DebugRoomStats.Face1Leaves++
	}
	if DebugRoomFaceReset {
		c = cd.Faces[0]
	}
	if c != ob.card {
		s.touch(o)
		ob.card = c
	}
	return true
}

// roomStackExit is stackExitFace's Room arm: the face persists (gorge's
// split stack-exit reset excludes Rooms) unless DebugRoomFaceReset.
func (s *State) roomStackExit(k *prep, o Obj) {
	if DebugRoomFaceReset {
		ob := &s.objs[o]
		if cd := &k.t.Cards[ob.card]; cd.Face != 0 {
			s.touch(o)
			ob.card = cd.Faces[0]
		}
	}
}

// roomUnturn is unturnFace's Room arm: a reversed room_alt cast turns the
// card back to the face it showed (it cast the other: 1 - Face); a reversed
// plain cast had no proposal flip (gorge faceBefore nil) and stays.
func (s *State) roomUnturn(k *prep, o Obj, mode uint8) {
	if mode&modeRoomAlt == 0 {
		return
	}
	ob := &s.objs[o]
	cd := &k.t.Cards[ob.card]
	s.touch(o)
	ob.card = cd.Faces[1-cd.Face] // seat-ok: the other face of a two-faced card
}

// offerRoomAlt offers hand Room o's other door (gorge "room_alt",
// legal_walk_hand.go: roomAlternateCastFace = 1 - FaceIdx).
func (s *State) offerRoomAlt(k *prep, d *Decision, p uint8, o Obj, sorc bool) {
	cd := &k.t.Cards[s.objs[o].card]
	alt := 1 - cd.Face // seat-ok: the other face
	if s.faceCastable(k, p, o, cd.Faces[alt], sorc) {
		d.add(MakeAction(AkCast, TCode(o), TCode(alt), castFace))
	}
}

// roomMV is splitMV's Room arm: off the stack and the battlefield a Room
// is a split card (its halves summed, whatever face it shows; gorge
// state/mana_value.go splitHalvesIn); on the stack the cast door's and on
// the battlefield the unlocked doors' (the state def's printed total).
func (s *State) roomMV(k *prep, c CardID, o Obj, mv int) int {
	cd := &k.t.Cards[c]
	if o == NoObj || len(cd.Faces) != 8 {
		return mv
	}
	switch s.objs[o].zone {
	case kop.ZoneStack:
		return mv
	case kop.ZoneBattlefield:
		if DebugRoomStats != nil {
			if m := roomMask(k, c); m == 3 {
				DebugRoomStats.MVTwoDoors++
			} else if m != 0 {
				DebugRoomStats.MVOneDoor++
			}
		}
		if DebugRoomMVFace {
			return int(k.cards[cd.Faces[cd.Face]].total)
		}
		return mv
	}
	return int(k.cards[cd.Faces[0]].total) + int(k.cards[cd.Faces[1]].total)
}

// offerUnlocks offers p's unlock special actions (gorge
// legal_walk_battlefield.go, sorcery block): for each Room p controls, its
// locked doors in [1-Face, Face] order whose printed cost the floating pool
// pays. A face-down Room is skipped (gorge faceDownPrintedHides).
func (s *State) offerUnlocks(k *prep, d *Decision, p uint8) {
	for sl := Slot(0); sl < MaxPerm; sl++ {
		if !s.occ.has(sl) {
			continue
		}
		pm := &s.perms[sl]
		if pm.ctrl != p || !roomDef(k, pm.card) || s.objs[pm.obj].card != pm.card || s.faceDown(k, sl) {
			continue // a face-down Room shows no doors (gorge faceDownPrintedHides)
		}
		cd := &k.t.Cards[pm.card]
		m := roomMask(k, pm.card)
		for _, fi := range [2]uint8{1 - cd.Face, cd.Face} { // seat-ok: the other face first
			if m&(1<<fi) != 0 {
				continue
			}
			cost := k.t.Cards[cd.Faces[fi]].Cost
			if cost.Hyb[0] != 0 {
				continue
			}
			if k.anyRoom {
				cost = s.unlockReduced(k, p, sl, cost) // face_unlockdoor.go
			}
			if !s.specialAffords(k, p, &cost, kop.SpmUnlock, sl, 0) {
				continue
			}
			d.add(MakeAction(AkUnlock, TCode(pm.obj), 0, fi))
		}
	}
}

// unlock performs the unlock special action on Room o's door fi: pay the
// door's printed cost from the pool, swap the permanent to the state with
// the door unlocked (transform's subscription swap; gorge emits no
// timestamp event), reset the passes, and fire the door's UnlockDoor
// triggers.
func (s *State) unlock(k *prep, o Obj, p uint8, fi uint8) {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield || fi > 1 {
		return
	}
	sl := ob.slot
	pm := &s.perms[sl]
	if pm.ctrl != p || !roomDef(k, pm.card) || ob.card != pm.card || s.faceDown(k, sl) {
		return
	}
	cd := &k.t.Cards[pm.card]
	m := roomMask(k, pm.card)
	if m&(1<<fi) != 0 {
		return
	}
	cost := k.t.Cards[cd.Faces[fi]].Cost
	if cost.Hyb[0] != 0 {
		return
	}
	if k.anyRoom {
		cost = s.unlockReduced(k, p, sl, cost) // face_unlockdoor.go
	}
	if !s.specialAffords(k, p, &cost, kop.SpmUnlock, sl, 0) {
		return
	}
	s.paySpecial(k, p, &cost, kop.SpmUnlock, sl, 0)
	s.touchSl(sl)
	s.subscribe(sl, &k.cards[pm.card], false)
	c := cd.Faces[kop.RoomState(cd.Face, m|1<<fi)]
	ob.card, pm.card = c, c
	s.subscribe(sl, &k.cards[c], true)
	s.subscribeGrants(k, sl)
	s.refreshTy(k)
	if k.anyFullyUnlock {
		// CR 709.5i: only the unlock that leaves both doors unlocked is a
		// full unlock (the pre-swap mask m had one door). DebugFullyUnlockAny
		// drops that transition gate (oracle mutation). fireFullyUnlock runs
		// before fireUnlockDoor, as gorge's ordinary face walk queues the
		// watchers before checkUnlockTriggers.
		if after := roomMask(k, ob.card); DebugFullyUnlockAny || m != 3 && after == 3 {
			s.fireFullyUnlock(k, sl, o, p)
		}
	}
	n := s.npend
	s.fireUnlockDoor(k, sl, o, fi)
	if DebugRoomStats != nil {
		if fi == cd.Face {
			DebugRoomStats.UnlockFace++
		} else {
			DebugRoomStats.UnlockOther++
		}
		DebugRoomStats.UnlockTrigs += int(s.npend - n)
	}
	s.passes = 0
}

// roomEntryUnlock fires the cast door's UnlockDoor triggers as a cast Room
// spell enters (CR 709.5h, gorge checkRoomEntryUnlockTriggers: CastDoor and
// the other door still locked), after the entry's ordinary triggers.
func (s *State) roomEntryUnlock(k *prep, sl Slot, o Obj) {
	c := s.perms[sl].card
	if !roomDef(k, c) || s.isSpellCopy(o) {
		return
	}
	cd := &k.t.Cards[c]
	if roomMask(k, c) != 1<<cd.Face {
		return
	}
	n := s.npend
	s.fireUnlockDoor(k, sl, o, cd.Face)
	if DebugRoomStats != nil {
		DebugRoomStats.EntryTrigs += int(s.npend - n)
	}
}

// fireUnlockDoor queues Room o's door-fi UnlockDoor triggers (gorge
// queueUnlockTriggers: only the unlocked door's own, read off the Room
// itself; RoomSplit says which of the def's abilities are door Face's).
func (s *State) fireUnlockDoor(k *prep, sl Slot, o Obj, fi uint8) {
	pm := &s.perms[sl]
	c := pm.card
	cd := &k.t.Cards[c]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbTriggered || a.Trig.Event != kop.TrigUnlockDoor || a.AFlags&kop.AfLatent != 0 {
			continue
		}
		if (i < int(cd.RoomSplit)) != (fi == cd.Face) {
			continue
		}
		s.objTrigGen(k, kop.TrigUnlockDoor, trigListener{obj: pm.obj, sl: sl, ctrl: pm.ctrl, card: c, ab: uint8(i)}, a, sl, o, pm.ctrl, 0)
	}
}

// FeatRoomUnlock is the AkUnlock kind for the oracle.
const FeatRoomUnlock = AkUnlock
