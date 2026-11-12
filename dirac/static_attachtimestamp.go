package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// CR 613.7e: an Aura, Equipment or Fortification receives a new timestamp
// when it becomes attached to an object (gorge events.foldAttach /
// restampLayer: any attach to a DIFFERENT object restamps the attached
// permanent's layer timestamp, Object.LayerStamp; the entry timestamp,
// perm.ts here, stays the permanent's ordering identity). The layer stamp
// lives beside perm in State.lstamp, written at entry (zones.go enter) and
// on an attach move, and nowhere else: perm.ts is also restamped by a
// control change (joined: the battlefield list order), which CR 613.7d and
// gorge do not treat as a new layer timestamp (CR 613.7).

// attachTo attaches the permanent at sl to host, restamping its layer
// timestamp when that is a move (gorge: AttachedTo changes), then applies
// any lose-all onset its static brings.
func (s *State) attachTo(k *prep, sl, host Slot) {
	if old := s.attachQuiet(k, sl, host); old != host && k.gen != nil && k.gen.anyAegis {
		s.aegisMoved(k, sl, old, host) // zone_exilethenattach.go
	}
}

// attachQuiet is attachTo without the attachment's Attached triggers (the
// caller fires them); it returns the previous bearer.
func (s *State) attachQuiet(k *prep, sl, host Slot) (old Slot) {
	old = s.perms[sl].attached
	if old != host {
		s.ts++
		s.lstamp[sl] = s.ts
	}
	s.perms[sl].attached = host
	s.perms[sl].flags &^= pfLastBear
	s.loseAllOnset(k, sl, host)
	return old
}

// attachPlayerLegal is gorge effAttach's Enchant:Player/Opponent destination
// re-check (engine/effects/attach.go's EnchantSelf branch): the aura
// attaches to a living seat, and an Enchant:Opponent word excludes the
// aura's own controller. It reports false for a permanent with no Enchant
// word at all: gorge takes that branch only for an aura whose Enchant spec
// is Player or Opponent, and its ordinary destination walk refuses players,
// so the kernel's player-destination arm (interp_gen.go's OpAttach W arm,
// tgt_attachremembered.go's attachObjPlayer) stays a no-op for one.
func (s *State) attachPlayerLegal(k *prep, sl Slot, seat uint8) bool {
	if s.out(seat) {
		return false
	}
	c := s.objs[s.perms[sl].obj].card
	if k.cards[c].types&kop.TAura == 0 || k.t.Cards[c].Enchant.Kind != kop.TgtPlayer {
		return false
	}
	if f := k.t.Cards[c].Enchant.Filter; f != 0 && k.t.Filters[f].Ctrl == kop.CtrlOpp && seat == s.perms[sl].ctrl {
		return false
	}
	return true
}

// attachPlayer attaches the permanent at sl to PLAYER seat (gorge
// events foldAttach's "attach to player" fold, engine/events/apply_zone.go:
// the Enchant:Player curses' spell resolution and Attach bodies). The
// object-host paths do not run: a player bearer is never in LastBearer or
// the linger set (gorge's UnattachPending/bearer-left machinery is
// object-bearer-only), and a player attach carries no lose-all onset (the
// "loses all abilities" statics name an object bearer).
func (s *State) attachPlayer(sl Slot, seat uint8) {
	s.attachQuietPlayer(sl, seat)
}

// attachQuietPlayer is attachPlayer without refreshTy; it restamps the
// layer timestamp when the host moved (CR 613.7e: attached to a different
// object OR player, gorge restampLayer).
func (s *State) attachQuietPlayer(sl Slot, seat uint8) {
	p := &s.perms[sl]
	moved := p.attached != NoSlot || p.atchP != seat+1
	if moved {
		s.ts++
		s.lstamp[sl] = s.ts
	}
	p.attached = NoSlot
	p.atchP = seat + 1
	p.flags &^= pfLastBear
	s.touchSl(sl)
}

// layerTs is the CR 613.7 timestamp the static abilities of the permanent
// at sl take (gorge Object.LayerTimestamp).
func (s *State) layerTs(sl Slot) uint16 {
	return s.lstamp[sl]
}
