package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// selfcast: "when you cast this spell" (CR 603.2: the spell is the
// trigger's source, on the stack; Mode$ SpellCast | ValidCard$ Card.Self,
// kop.TfSelf). gorge's trigger walk (rules/object_query.go forEachObject)
// visits seat 0's battlefield, then the stack (once, on seat 0's pass),
// then every later living seat's battlefield, and sortPendingTriggers keeps
// that order within a controller (APNAP, stable). So the kernel fires the
// spell's own cast triggers from the stack entry at cast time, after the
// caster's battlefield listeners when the caster is player 0 and before
// them for any later seat (onCast; the other player's entries are ordered
// by controller, never relative to it). A battlefield permanent carrying such
// a trigger never fires it on another spell (the TfSelf skip in onCast's
// listener walk and castTrigVanilla): the trigger reads the cast spell,
// which is never the permanent.
//
// The listener is the spell object (trigListener.sl NoSlot, as a cycled
// card's graveyard trigger); the trigger object is the spell too, so a
// Tf2XBound body reads the spell's paid X (bindTrigX finds the stack
// entry; Hydroid Krasis, Old Stickfingers, Genesis Hydra, Gluttonous
// Hellkite). Guard: prep.selfCast (prepare: some card has one).

// fireSelfCast queues spell e's own SpellCast triggers (TfSelf; zone none
// or stack: Demigod of Revenge's TriggerZones$ Stack).
//
//go:noinline
func (s *State) fireSelfCast(k *prep, e *stackEntry) {
	cd := &k.t.Cards[e.card]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		t := &a.Trig
		if a.Kind != kop.AbTriggered || t.Event != kop.TrigSpellCast || t.Flags&kop.TfSelf == 0 || a.AFlags&kop.AfLatent != 0 {
			continue
		}
		if t.Zone != kop.ZoneNone && t.Zone != kop.ZoneStack {
			continue
		}
		if t.Filter != 0 {
			if !s.selfCastFilterOK(k, t.Filter, e) {
				continue
			}
			ac := *a // the Filter is read against the spell here, not as a card filter
			ac.Trig.Filter = 0
			a = &ac
		}
		l := trigListener{obj: e.obj, sl: NoSlot, ctrl: e.ctrl, card: e.card, ab: uint8(i)}
		if k.gen != nil {
			s.castTrigGen(k, l, a, e) // the SpellCast arms (Whose, turn, Nth, TargetsValid$), then queueTrig
			continue
		}
		if !whoseOK(t.Whose, e.ctrl, e.ctrl) {
			continue
		}
		s.enqueue(e.obj, s.objs[e.obj].ver, e.card, uint8(i), e.ctrl, e.obj, e.ctrl, 0)
	}
}

// prepSelfCast: some card of t has a self cast trigger (prep.selfCast).
func prepSelfCast(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigSpellCast && a.Trig.Flags&kop.TfSelf != 0 {
				return true
			}
		}
	}
	return false
}

// selfCastFilterOK reads a self cast trigger's Filter against the spell
// (kfix-cast; kgen lowers "Card.Self+kicked" to TfSelf + {F2Kicked} and
// refuses every other property): Kicked is the stack entry's kick flag,
// as gorge's ValidCard$ kicked reads the cast's provenance. Sneaked is
// the same stack entry's trigPlSneak mark: a stack
// read needs no per-permanent storage.
func (s *State) selfCastFilterOK(k *prep, f kop.FilterID, e *stackEntry) bool {
	fl := &k.t.Filters[f]
	if fl.Flags2&kop.F2Kicked != 0 && e.flags&sfKicked == 0 {
		return false
	}
	if fl.Flags2&kop.F2Sneaked != 0 && !sneakEntry(e) {
		return false
	}
	if fl.Pred == kop.PredEscaped {
		return escapeEntry(e) == (fl.PredN == 1)
	}
	return true
}
