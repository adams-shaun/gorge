package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// CR 709.5i "whenever you fully unlock a Room" (gorge
// Mode$ FullyUnlock, trigmatch room.go fullyUnlockMatches). The unlock
// special action emits the one DoorUnlock of a real locked->unlocked
// transition; gorge's ordinary per-face trigger walk visits every listener
// (battlefield permanents and graveyard cards with TriggerZones$ Graveyard),
// then checkUnlockTriggers queues the unlocked door's own UnlockDoor
// triggers. The kernel's unlock action copies that order: fireFullyUnlock
// runs after the door-state swap and before fireUnlockDoor.
//
// The scan is cold and guarded by prep.anyFullyUnlock, so tables without a
// watcher never pay it. A trigger whose ValidCard$ is absent or Card.Room is
// matched against the unlocked Room (a filter absent from the table matches
// anything, exactly gorge's absent clause defaulting to Card.Room because the
// event subject is always a Room); ValidPlayer$ You is the unlocker gate
// against the listener's controller. Secondary$ True pairs this line with the
// Eerie enchantment-ETB sibling: that sibling is a ChangesZone, which never
// matches a DoorUnlock, so the flag is inert here and kgen consumes it.

// DebugFullyUnlockAny drops the locked->unlocked transition gate (oracle
// mutation only): every unlock fires the watchers, including an uncast
// Room's first door, so TestFullyUnlockLockstep must diverge.
var DebugFullyUnlockAny bool

// fireFullyUnlock queues the FullyUnlock listeners for the Room roomSl/
// roomObj that unlocker p just made fully unlocked. Battlefield listeners
// come first in gorge's walk order, then graveyard cards (TriggerZones$
// Graveyard; Fear of Infinity) via k.gen.graveTrig.
//
//go:noinline
func (s *State) fireFullyUnlock(k *prep, roomSl Slot, roomObj Obj, unlocker uint8) {
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.occ, &ord)
	for _, sl := range ord[:n] {
		s.slotTriggers(k, sl, func(a *kop.Ability) bool { return a.Trig.Event == kop.TrigFullyUnlock }, func(l trigListener, a *kop.Ability) {
			if !whoseOK(a.Trig.Whose, l.ctrl, unlocker) {
				return
			}
			s.objTrigGen(k, kop.TrigFullyUnlock, l, a, roomSl, roomObj, unlocker, 0)
		})
	}
	for _, c := range k.gen.graveTrig {
		cd := &k.t.Cards[c]
		for pp := uint8(0); pp < s.nSeats(); pp++ {
			if s.out(pp) {
				continue
			}
			pl := &s.pl[pp]
			for i := 0; i < int(pl.graveN); i++ {
				o := pl.grave[i]
				if s.objs[o].card != c {
					continue
				}
				for j := range cd.Abilities {
					a := &cd.Abilities[j]
					if a.Kind != kop.AbTriggered || a.Trig.Zone != kop.ZoneGraveyard || a.Trig.Event != kop.TrigFullyUnlock {
						continue
					}
					l := trigListener{obj: o, sl: NoSlot, ctrl: pp, card: c, ab: uint8(j)}
					if !whoseOK(a.Trig.Whose, pp, unlocker) {
						continue
					}
					s.objTrigGen(k, kop.TrigFullyUnlock, l, a, roomSl, roomObj, unlocker, 0)
				}
			}
		}
	}
}

// anyETB reports whether an enters trigger can fire: a battlefield listener
// is subscribed, or the table has a graveyard ETB (genPrep.graveETB). The
// per-slot subscription mask alone misses Fear of Infinity when no
// battlefield ETB watcher is out, which is exactly the state fireGraveETB
// exists to cover. An Effect-created enters trigger (Thunder of Unity's
// chapters) listens from the command zone: with an emblem out and some
// emblem on the table listening for ETB, no battlefield watcher is needed. A
// commander in a command zone has no battlefield subscription either, so the
// printed command-zone trigger keeps the event alive too (Inalla's eminence).
func (s *State) anyETB(k *prep) bool {
	return s.subs[kop.TrigETB].any() ||
		k != nil && k.gen != nil && (k.gen.graveETB || k.gen.embObjEv&(1<<kop.TrigETB) != 0 && s.anyEmb()) ||
		k != nil && s.anyCommandTrigger(k, kop.TrigETB)
}

// fireGraveETB queues the graveyard cards' ETB triggers (TriggerZones$
// Graveyard) for the permanent that just entered, after the battlefield
// listeners. Fear of Infinity's Eerie line is the carrier: its graveyard ETB
// half ("whenever an enchantment you control enters, you may return this
// card") is the same shared Execute$ as its FullyUnlock half, and gorge's
// ordinary walk visits it in the graveyard zone. Without this scan the
// kernel missed the ETB half and diverged on the trigger-order decision
// (two watchers pending in gorge, one in the kernel). genPrep.graveETB keeps
// the scan off every table without a graveyard ETB.
//
//go:noinline
func (s *State) fireGraveETB(k *prep, evSl Slot, evObj Obj, evCtrl uint8) {
	for _, c := range k.gen.graveTrig {
		cd := &k.t.Cards[c]
		for pp := uint8(0); pp < s.nSeats(); pp++ {
			if s.out(pp) {
				continue
			}
			pl := &s.pl[pp]
			for i := 0; i < int(pl.graveN); i++ {
				o := pl.grave[i]
				if s.objs[o].card != c {
					continue
				}
				for j := range cd.Abilities {
					a := &cd.Abilities[j]
					if a.Kind != kop.AbTriggered || a.Trig.Zone != kop.ZoneGraveyard || a.Trig.Event != kop.TrigETB {
						continue
					}
					l := trigListener{obj: o, sl: NoSlot, ctrl: pp, card: c, ab: uint8(j)}
					s.objTrigGen(k, kop.TrigETB, l, a, evSl, evObj, evCtrl, 0)
				}
			}
		}
	}
}
