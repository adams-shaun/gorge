package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Two kernel defects the Station lockstep (kcounters) found, cold and out
// of line:
//
//   - fireLeave fired TrigDies for a creature only, while gorge's dies
//     trigger is Mode$ ChangesZone Origin$ Battlefield Destination$
//     Graveyard matched against ValidCard$ whatever the card's types: a
//     noncreature permanent put into a graveyard fired nothing (Necrosquito's
//     "another creature or artifact you control", an artifact's "when this is
//     put into a graveyard from the battlefield"). Behind genPrep.anyDiesNC
//     (some dies trigger can match a noncreature) fireLeave now fires it for
//     any permanent; the triggers' own filters decide.
//   - the layer walk (lfxCollect) collected every global override static for
//     every slot and silently dropped the ones past maxLfx: thirteen
//     Spacecraft with a STATION N+ self static dropped the last. A self-only
//     static is now collected for its source only, and an overflow that
//     could matter stalls (StallBoard) instead of dropping.

// prepDiesNC sets genPrep.anyDiesNC.
func prepDiesNC(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		c := &t.Cards[i]
		for j := range c.Abilities {
			if a := &c.Abilities[j]; a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigDies && diesNC(t, c, a) {
				g.anyDiesNC = true
			}
		}
	}
}

// diesNC reports whether dies trigger a of card c can match a noncreature
// permanent put into a graveyard from the battlefield. gorge's "dies"
// trigger is Mode$ ChangesZone Origin$ Battlefield Destination$ Graveyard,
// matched against ValidCard$ whatever the card's types (Necrosquito's
// "another creature or artifact you control", an artifact's "when this is
// put into a graveyard from the battlefield"); the kernel's fireLeave fires
// TrigDies for a creature only unless the table has such a trigger. A
// self trigger on a creature card, and a granted one (on a creature), stay
// creature-only.
func diesNC(t *kop.Table, c *kop.CardDef, a *kop.Ability) bool {
	if a.AFlags&kop.AfLatent != 0 {
		return false // granted to a creature ("when this creature dies")
	}
	if f := a.Trig.Filter; a.Trig.Flags&kop.TfSelf != 0 || f != 0 && int(f) < len(t.Filters) && t.Filters[f].Flags&kop.FSelf != 0 && t.Filters[f].Or == 0 {
		return c.Types&kop.TCreature == 0
	}
	for f := a.Trig.Filter; ; {
		if f == 0 || int(f) >= len(t.Filters) || t.Filters[f].Types != kop.TCreature {
			return true
		}
		if f = t.Filters[f].Or; f == 0 {
			return false
		}
	}
}

// selfOnlyFilter: filter f matches only the static's source (FSelf, no
// alternative), so the layer walk need not collect the static for any
// other slot. Found by the station lockstep: thirteen Spacecraft with a
// global STATION N+ override filled maxLfx and the last one was dropped.
func selfOnlyFilter(k *prep, f kop.FilterID) bool {
	if f == 0 || int(f) >= len(k.t.Filters) {
		return false
	}
	fl := &k.t.Filters[f]
	return fl.Flags&kop.FSelf != 0 && fl.Or == 0
}

// lfxFull stalls the game when the layer walk of sl has no room for a
// static that could apply to it (maxLfx effects already collected), rather
// than silently dropping it. A self-only static on another
// slot never applies, so it takes no room; a subtype grant the slot takes
// (sub) always does.
//
//go:noinline
func (s *State) lfxFull(k *prep, sl, src Slot, a *kop.Ability, sub bool) {
	if a.Static.Kind == kop.StatOverride && a.Static.Override > 0 && (src == sl || !selfOnlyFilter(k, a.Static.Filter)) {
		s.capFull(StallBoard)
		return
	}
	if sub && a.Static.Kind == kop.StatContinuous && (a.Static.AddSub.Any() || a.Static.AddChosen) {
		s.capFull(StallBoard) // a subtype grant the slot takes: never drop it
	}
}
