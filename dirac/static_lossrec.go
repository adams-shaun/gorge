package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// loss6Rec is loss6 in a table without a lose-all override static
// (anyLoseStat false): the walk's lose-all entries are then sl's own
// override records only (lfxCollect's first maxOvr fxOverride records on
// sl, each applying unconditionally), so the verdict and timestamp come
// from those records without building sl's characteristics (kperf2: a
// table whose fxNeedL6 comes from an effect's keyword removal, Debuff,
// otherwise walks every slot on every genLost).
func (s *State) loss6Rec(k *prep, sl Slot, ts uint16, lost bool) (uint16, bool) {
	if !k.gen.anyLoseRec || !s.fxM.has(sl) {
		return ts, lost
	}
	c := 0
	for i := 0; i < int(s.g.nfx) && c < maxOvr; i++ {
		if r := &s.g.fx[i]; r.sl == sl && r.kind == fxOverride {
			c++
			if k.t.Overrides[r.r.idx-1].LoseAbilities {
				lost = true
				ts = max(ts, lossTs(&k.t.Overrides[r.r.idx-1], r.r.ts))
			}
		}
	}
	return ts, lost
}

// overridesUncreature reports whether some override of the table can leave
// a combatant without the creature type (gen.tyCombat): one that sets types
// (typesGen keeps only the supertypes then), or one that makes a creature,
// whose end leaves the printed noncreature types. Without one a combatant's
// typesAt is its printed types, an Animate's (always a creature, until the
// cleanup after combat) or a copy's taken as it entered, so CR 506.4's
// removal walk (removeNoncreatureCombatants) never removes anything.
func overridesUncreature(t *kop.Table) bool {
	for i := range t.Overrides {
		o := &t.Overrides[i]
		if o.HasTypes || (o.AddTypes|o.SetTypes)&kop.TCreature != 0 {
			return true
		}
	}
	return false
}

// loseStatOn reports whether a lose-all override static can be among sl's
// walk effects (lfxCollect's static sources: a loseStat card on the
// battlefield, not itself without abilities, global or attached to sl).
// Without one, loss6's verdict is the records' (loss6Rec).
func (s *State) loseStatOn(k *prep, sl Slot) bool {
	for m := s.statM; m.any(); m = m.rest() {
		src := m.first()
		cid := s.cardAt(src)
		if !k.gen.has(cid, gfLoseStat) || s.perms[src].ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 {
			continue
		}
		if k.gen.has(cid, gfGlobOvr) || s.perms[src].attached == sl && src != sl {
			return true
		}
	}
	return false
}
