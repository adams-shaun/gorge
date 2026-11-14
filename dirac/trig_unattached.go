package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Mode$ Unattached (CR 701.3b/d), the "whenever this Equipment becomes
// unattached from a permanent" trigger the corpus carriers print (Captain's
// Hook, Grafted Wargear, Stitcher's Graft; Grafted Exoskeleton rides the
// same trigger). gorge publishes the detach as an events.Unattached whose
// IDs[0] is the FORMER BEARER and binds that bearer as the trigger's object
// (trigger_match.go triggerRemembered's Unattached arm), so Defined$
// TriggeredObjectLKICopy reads it dead or alive (CR 608.2h). The kernel has
// no such event: each arm that publishes one fires the attachment's own
// TrigUnattached abilities here, RefTriggerObj the bearer, at the boundary
// where gorge publishes. The publishing arms and the kernel site each maps
// to:
//
//   - a re-attach to a different bearer (gorge rules/emit.go, published
//     before the Attach): attachTo;
//   - the bearer's departure (gorge events' Move fold plus attachmentSBAs'
//     pending and bearer-left arms, published at the SBA checkpoint with the
//     bearer already gone): removePerm's detach loop, and phasedHostLeft for
//     the leaving host's hidden phased-out attachments (gorge publishes
//     theirs at the phase-in, the kernel's only detach boundary is here);
//   - the attachment's own departure (gorge's CR 603.10a look-back walk,
//     looksBackOnDeparture): fireLeave, beside its Dies/Leaves walk;
//   - the bestowed detach (CR 702.114b, gorge attachmentSBAs' "bestowed
//     attachment ended"): sbaBestowDetach;
//   - the CR 704.5p/704.5n SBA arms (gorge attachmentSBAs'
//     attachedKindIllegal and its Equipment-bearer arm): sbaAttachedCreature
//     and turn.go's CR 704.5n arm.
//
// An attach to a PLAYER publishes none (gorge foldAttach's player arm emits
// one Attach and no Unattached, and the empty-IDs detach emits none either),
// so attachQuietPlayer stays silent. An unattached Equipment destroyed fires
// nothing: fireLeave fires only a permanent that left ATTACHED.

// unattachedFires reports whether the attachment at sl fires its Mode$
// Unattached abilities on the bearer-left arm (removePerm's loop and
// phasedHostLeft): gorge's pending arm publishes an Unattached there for a
// bestowed card and an Equipment, while an Aura takes its CR 704.5m
// graveyard MoveZone, no Unattached. The re-attach, own-departure and SBA
// arms publish for any attachment, so they gate on anyUnattach alone.
func (s *State) unattachedFires(k *prep, sl Slot) bool {
	if k.gen == nil || !k.gen.anyUnattach {
		return false
	}
	if k.t.Cards[s.cardAt(sl)].HasBestow {
		return true
	}
	return s.typesAt(k, sl)&kop.TEquipment != 0
}

// fireUnattached queues the TrigUnattached abilities of the attachment at
// sl, which just became unattached from the permanent at bearer (dead or
// alive), RefTriggerObj that bearer. Mirrors fireAttached: no slot
// subscription, one walk over the attachment's own card, the Panharmonicon
// grant scanning the same way.
//
//go:noinline
func (s *State) fireUnattached(k *prep, sl, bearer Slot) {
	if bearer == NoSlot {
		return
	}
	o := s.perms[sl].obj
	ob := &s.objs[o]
	c := s.cardAt(sl)
	cd := &k.t.Cards[c]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbTriggered || a.Trig.Event != kop.TrigUnattached {
			continue
		}
		np := s.npend
		// No LKI snapshot on the referent: gorge binds the event's bearer id
		// LIVE (triggerRemembered's Unattached arm reads ev.IDs[0] at the
		// publish, and the bearer-left publish happens at the SBA checkpoint,
		// where a bearer Ephemerate exiled is already back on the
		// battlefield), so the body still finds a bearer that left and came
		// back before the trigger resolved. An LKI snapshot here would set
		// res.trigGone (resolve.go) and no-op the body as a new object,
		// which gorge does not do. A bearer still gone no-ops on objRef's
		// zone check, as gorge's Destroy/Sacrifice of a non-battlefield
		// object does. The bodies that read the referent (Destroy,
		// SacrificeAll) never read its characteristics, so li==0 loses
		// nothing; snapshotTrigSpellMV is SpellCast-only.
		s.enqueue(o, ob.ver, c, uint8(i), s.perms[sl].ctrl, s.perms[bearer].obj, s.perms[bearer].ctrl, 0)
		s.panharmKeyword(k, sl, kop.TrigUnattached, s.perms[bearer].obj, np)
	}
}

// prepUnattach sets anyUnattach.
func prepUnattach(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigUnattached {
				g.anyUnattach = true
				return
			}
		}
	}
}
