package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugPanharmNoEcho switches the Panharmonicon echo off (the panharm
// mutation test: the lockstep must then diverge).
var DebugPanharmNoEcho bool

// PanharmLog, when non-nil, records every echo placed (the panharm test
// counts them; nothing else reads it).
var PanharmLog *[]PanharmRec

// PanharmRec is one echoed placement: the source card's name and whether the
// trigger line is a dies line.
type PanharmRec struct {
	Card string
	Dies bool
}

// panharmEcho appends one copy of the pending entry at index n for each
// live Panharmonicon static matching the trigger's source (CR 603.2d; gorge
// queueTrigger: every triggered ability of an object, the original followed by
// its copies; a copy is not an event and is not echoed again). ev is the event
// that caused the trigger; the Saga chapter's is its lore counter, no entry.
// who is the player a player event is about (tpl: an object event's
// controller, which no mode-gated static reads).
func (s *State) panharmEcho(k *prep, l trigListener, ev kop.TrigEvent, tobj Obj, who uint8, n uint8) {
	for e := s.panharmEchoes(k, l, ev, tobj, who); e > 0; e-- {
		if int(s.npend) >= MaxPending {
			s.capFull(StallPending)
			return
		}
		s.pend[s.npend] = s.pend[n]
		s.npend++
		if PanharmLog != nil {
			*PanharmLog = append(*PanharmLog, PanharmRec{k.t.Cards[l.card].Name, ev == kop.TrigDies})
		}
	}
}

// panharmKeyword echoes the pending entry at index n, the keyword trigger of
// the permanent at sl that the walks outside queueTrig queued.
func (s *State) panharmKeyword(k *prep, sl Slot, ev kop.TrigEvent, tobj Obj, n uint8) {
	if k.gen != nil && k.gen.anyPanharm && s.npend > n {
		p := &s.perms[sl]
		s.panharmEcho(k, trigListener{obj: p.obj, sl: sl, ctrl: p.ctrl, card: s.cardAt(sl)}, ev, tobj, p.ctrl, n)
	}
}

// panharmEchoes counts the ADDITIONAL placements the battlefield's
// StatPanharmonicon statics demand for a trigger of listener l's source
// object (CR 603.2d; gorge statics_echo.go panharmoniconEchoes, ValidCard$
// only): one per live static whose filter matches the source relative to the
// static's controller and slot. ValidZone$ defaults to the battlefield, so a
// source still on the battlefield (a dying permanent is, while its dies trigger
// is checked) can match and a spell or a graveyard card cannot.
//
// A static lowered from ValidMode$ ChangesZone,ChangesZoneAll | ValidCause$ |
// Destination$ Battlefield (StPanhETB, Filter2 = the cause) echoes only a
// placement caused by a permanent entering the battlefield: gorge maps the
// MoveZone to the battlefield and a TokenCreate/CardToken mint to those modes
// (panharmoniconModes), and the kernel's event for a Mode$ ChangesZone trigger
// of a move or mint onto the battlefield is TrigETB, whose triggering object
// tobj is the entering permanent. A Mode$ TokenCreatedOnce trigger
// (TrigTokenCreated) is not a ChangesZone trigger, and Forge's ValidMode$ names
// the doubled trigger's own mode, so it is never echoed. The cause must match
// Filter2 relative to the static's controller.
//
// A static lowered from a ValidMode$ that names one player-event mode alone
// (Static.A, kop.PanhModeLifeGained / PanhModeDrawn) echoes only a trigger of
// that event whose player who matches ValidPlayer$ (Static.B) relative to the
// static's controller: gorge reads the trigger's own mode against ValidMode$
// and the event's player against ValidPlayer$.
func (s *State) panharmEchoes(k *prep, l trigListener, ev kop.TrigEvent, tobj Obj, who uint8) int {
	if DebugPanharmNoEcho || l.sl == NoSlot || !s.occ.has(l.sl) || s.perms[l.sl].obj != l.obj {
		return 0
	}
	n := 0
	for m := s.statM; m.any(); m = m.rest() {
		sl := m.first()
		if s.genLost(k, sl) {
			continue
		}
		p := &s.perms[sl]
		cd := &k.t.Cards[s.cardAt(sl)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatPanharmonicon || a.AFlags&kop.AfLatent != 0 {
				continue
			}
			if !s.statCond(k, statSrc{a: a, sl: sl, ctrl: p.ctrl, obj: p.obj}) {
				continue
			}
			if a.Static.StFlags&kop.StPanhETB != 0 && !s.panharmCause(k, a, ev, tobj, p.ctrl, sl) {
				continue
			}
			if a.Static.A != 0 && !panharmModeOK(a.Static.A, kop.Whose(a.Static.B), ev, p.ctrl, who) {
				continue
			}
			if s.matchPermGen(k, a.Static.Filter, l.sl, p.ctrl, sl, layerFull) {
				n++
			}
		}
	}
	return n + s.panharmGrants(l.obj)
}

// panharmModeOK reports whether a trigger of event ev about player who is of
// the one mode m a Panharmonicon static names, with who matching w relative to
// the static's controller ctrl.
func panharmModeOK(m int16, w kop.Whose, ev kop.TrigEvent, ctrl, who uint8) bool {
	switch {
	case m == kop.PanhModeLifeGained && ev == kop.TrigLifeGained, m == kop.PanhModeDrawn && ev == kop.TrigDrawn:
		return whoseOK(w, ctrl, who)
	}
	return false
}

// panharmCause reports whether a trigger caused by event ev on triggering
// object tobj is caused by a permanent entering the battlefield that matches
// the StPanhETB static a's cause filter (static controlled by ctrl at slot sl).
func (s *State) panharmCause(k *prep, a *kop.Ability, ev kop.TrigEvent, tobj Obj, ctrl uint8, sl Slot) bool {
	if ev != kop.TrigETB || tobj == NoObj {
		return false
	}
	ob := &s.objs[tobj]
	f := a.Static.Filter2
	if ob.zone == kop.ZoneBattlefield && s.occ.has(ob.slot) && s.perms[ob.slot].obj == tobj {
		return s.matchPermGen(k, f, ob.slot, ctrl, sl, layerFull)
	}
	return s.matchCardGen(k, f, ob.card, ob.owner(), ctrl, tobj)
}
