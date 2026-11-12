package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Derived characteristics of a battlefield slot, applying (in this fixed
// order) the override record's copy-of, type set, lose-all-abilities and
// set-P/T, then +1/+1 and -1/-1 counters, until-EOT deltas and Continuous
// statics. This fixed order stands in for CR 613's layers.

func (s *State) cardAt(sl Slot) CardID {
	p := &s.perms[sl]
	if p.ovr.flags&ovCopy != 0 {
		return p.ovr.copyOf
	}
	return p.card
}

func (s *State) typesAt(k *prep, sl Slot) kop.TypeMask {
	p := &s.perms[sl]
	if p.ovr.flags&(ovTypes|ovTypesEOT|ovGenTy) != 0 {
		if p.ovr.flags&ovGenTy != 0 {
			// a generated table's overrides (refreshTy); a face-down
			// permanent always reads here (face_facedown.go)
			return p.gty &^ (gtyFaceDown | gtyProvMask)
		}
		return p.ovr.types
	}
	return k.cards[s.cardAt(sl)].types
}

func (s *State) isCreature(k *prep, sl Slot) bool { return s.typesAt(k, sl)&kop.TCreature != 0 }

// staticPT sums the Continuous statics that apply to sl.
func (s *State) staticPT(k *prep, sl Slot) (dp, dt int, kw kop.KwMask) {
	m := s.statM
	for m.any() {
		src := m.first()
		m = m.rest()
		sp := &s.perms[src]
		if sp.ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 {
			continue
		}
		cd := &k.t.Cards[s.cardAt(src)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous {
				continue
			}
			if !s.matchPerm(k, a.Static.Filter, sl, sp.ctrl, src) {
				continue
			}
			dp += int(a.Static.A)
			dt += int(a.Static.B)
			kw |= a.Static.Kw
		}
	}
	return
}

func (s *State) basePT(k *prep, sl Slot) (int, int) {
	p := &s.perms[sl]
	if p.ovr.flags&(ovSetPT|ovSetPTEOT) != 0 {
		return int(p.ovr.setP), int(p.ovr.setT)
	}
	pc := &k.cards[s.cardAt(sl)]
	return int(pc.power), int(pc.tough)
}

// pt returns sl's power and toughness.
func (s *State) pt(k *prep, sl Slot) (int, int) {
	if k.gen != nil {
		return s.ptGen(k, sl)
	}
	p := &s.perms[sl]
	bp, bt := s.basePT(k, sl)
	c := p.ctrLow(int(kop.CtrP1P1)) - p.ctrLow(int(kop.CtrM1M1))
	ep, et := int(p.eotP), int(p.eotT)
	if s.nptx != 0 {
		ep, et = s.eotPT(sl)
	}
	bp += c + ep
	bt += c + et
	if s.statM.any() {
		dp, dt, _ := s.staticPT(k, sl)
		bp += dp
		bt += dt
	}
	return bp, bt
}

func (s *State) power(k *prep, sl Slot) int { p, _ := s.pt(k, sl); return p }

// kw returns sl's keywords.
func (s *State) kw(k *prep, sl Slot) kop.KwMask {
	if k.gen != nil {
		return s.kwGen(k, sl)
	}
	p := &s.perms[sl]
	var m kop.KwMask
	if p.ovr.flags&(ovLoseAll|ovLoseAllEOT) == 0 {
		m = k.cards[s.cardAt(sl)].kw
	}
	m |= p.eotKw | p.ovr.kw
	// CR 122.1b + 613.1c: the counter grant is a BASE keyword (gorge
	// BaseKeywords, engine/rules/chars/derive.go), so an active
	// loses-all-abilities effect clears it too (Abigale's Animate then
	// keyword counters); a later layer-6 grant still re-adds on top.
	if p.ovr.flags&(ovLoseAll|ovLoseAllEOT) == 0 {
		m |= ctrKw(s, p)
	}
	if s.statM.any() {
		_, _, g := s.staticPT(k, sl)
		m |= g
	}
	return m
}

func (s *State) colorsAt(k *prep, sl Slot) kop.ColorMask {
	if k.gen != nil {
		return s.colorsGen(k, sl)
	}
	return k.t.Cards[s.cardAt(sl)].Colors
}

// matchPerm reports whether battlefield slot sl matches filter f, relative
// to a source controlled by ctrl at slot src (NoSlot when the source is not
// a permanent).
func (s *State) matchPerm(k *prep, f kop.FilterID, sl Slot, ctrl uint8, src Slot) bool {
	if f == 0 {
		return true
	}
	if k.gen != nil {
		return s.matchPermGen(k, f, sl, ctrl, src, layerFull)
	}
	fl := &k.t.Filters[f]
	p := &s.perms[sl]
	switch fl.Ctrl {
	case kop.CtrlYou:
		if p.ctrl != ctrl {
			return false
		}
	case kop.CtrlOpp:
		if p.ctrl == ctrl {
			return false
		}
	case kop.CtrlEachPl:
		if !s.ctrlEachPl(p.ctrl, ctrl) {
			return false
		}
	case kop.CtrlHitBySrc:
		if !s.combatHitBySrc(p.ctrl) {
			return false // value_turntallies.go
		}
	}
	if fl.Flags != 0 {
		if fl.Flags&kop.FOther != 0 && sl == src {
			return false
		}
		if fl.Flags&kop.FSelf != 0 && sl != src {
			return false
		}
		if fl.Flags&kop.FAttachedBySrc != 0 && (src == NoSlot || s.perms[src].attached != sl) {
			return false
		}
		if fl.Flags&kop.FNotAttachedBySrc != 0 && src != NoSlot && (s.perms[src].attached == sl) != DebugNotAttachedPositive {
			return false // filter_notattached.go
		}
		tok := s.objs[p.obj].of&ofToken != 0
		if fl.Flags&kop.FNontoken != 0 && tok {
			return false
		}
		if fl.Flags&kop.FToken != 0 && !tok {
			return false
		}
		if fl.Flags&kop.FTapped != 0 && p.flags&pfTapped == 0 {
			return false
		}
		if fl.Flags&kop.FUntapped != 0 && p.flags&pfTapped != 0 {
			return false
		}
		if fl.Flags&kop.FAttacking != 0 && p.flags&pfAttacking == 0 {
			return false
		}
		if fl.Flags&kop.FNotAttacking != 0 && p.flags&pfAttacking != 0 {
			return false
		}
		if fl.Flags&kop.FBlocking != 0 && p.flags&pfBlocking == 0 {
			return false
		}
		if fl.Flags&kop.FAttackingOrBlocking != 0 && p.flags&(pfAttacking|pfBlocking) == 0 {
			return false
		}
	}
	if fl.Types != 0 || fl.NotTypes != 0 {
		t := s.typesAt(k, sl)
		if fl.Types != 0 && t&fl.Types == 0 {
			return false
		}
		if t&fl.NotTypes != 0 {
			return false
		}
	}
	if fl.Colors != 0 || fl.NotColors != 0 {
		c := s.colorsAt(k, sl)
		if fl.Colors != 0 && c&fl.Colors == 0 {
			return false
		}
		if c&fl.NotColors != 0 {
			return false
		}
	}
	if fl.Pred != kop.PredNone {
		return s.predPerm(k, fl, sl)
	}
	return true
}

func (s *State) predPerm(k *prep, fl *kop.Filter, sl Slot) bool {
	n := int(fl.PredN)
	switch fl.Pred {
	case kop.PredPowerLE:
		return s.power(k, sl) <= n
	case kop.PredPowerGE:
		return s.power(k, sl) >= n
	case kop.PredToughnessLE:
		_, t := s.pt(k, sl)
		return t <= n
	case kop.PredCMCLE:
		return s.mvPerm(k, sl) <= n
	case kop.PredCMCGE:
		return s.mvPerm(k, sl) >= n
	case kop.PredHasKeyword:
		return s.kw(k, sl)&(1<<uint(n)) != 0
	case kop.PredHasCounter:
		return s.perms[sl].ctrN(s, int(n)) > 0
	case kop.PredSubtype:
		return k.t.Cards[s.cardAt(sl)].Subtypes.Has(int(n))
	case kop.PredNamed:
		return int(s.cardAt(sl)) == n
	}
	panic("kernel: filter predicate not implemented: " + predName(fl.Pred))
}

// matchCard reports whether a card (not on the battlefield) matches f.
func (s *State) matchCard(k *prep, f kop.FilterID, c CardID, owner, ctrl uint8) bool {
	if f == 0 {
		return true
	}
	if k.gen != nil {
		return s.matchCardGen(k, f, c, owner, ctrl, NoObj)
	}
	fl := &k.t.Filters[f]
	switch fl.Ctrl {
	case kop.CtrlYou:
		if owner != ctrl {
			return false
		}
	case kop.CtrlOpp:
		if owner == ctrl {
			return false
		}
	}
	pc := &k.cards[c]
	if fl.Types != 0 && pc.types&fl.Types == 0 {
		return false
	}
	if pc.types&fl.NotTypes != 0 {
		return false
	}
	col := k.t.Cards[c].Colors
	if fl.Colors != 0 && col&fl.Colors == 0 {
		return false
	}
	if col&fl.NotColors != 0 {
		return false
	}
	n := int(fl.PredN)
	switch fl.Pred {
	case kop.PredNone:
	case kop.PredPowerLE:
		return int(pc.power) <= n
	case kop.PredPowerGE:
		return int(pc.power) >= n
	case kop.PredToughnessLE:
		return int(pc.tough) <= n
	case kop.PredCMCLE:
		return int(pc.total) <= n
	case kop.PredCMCGE:
		return int(pc.total) >= n
	case kop.PredHasKeyword:
		return pc.kw&(1<<uint(n)) != 0
	case kop.PredSubtype:
		return k.t.Cards[c].Subtypes.Has(int(n))
	case kop.PredNamed:
		return int(c) == n
	default:
		panic("kernel: card predicate not implemented: " + predName(fl.Pred))
	}
	return true
}

func predName(p kop.Pred) string {
	names := [...]string{"None", "PowerLE", "PowerGE", "ToughnessLE", "CMCLE", "CMCGE", "HasKeyword", "HasCounter", "Subtype", "Named", "WithPowerLEX",
		"CMCEQ", "CMCLEV", "CountersGE", "PowerGEV", "ToughnessGE", "HasXCost", "PowerLTToughness", "MultiColor", "Historic", "Equipped", "Enchanted", "Attached", "ActiveCtrl", "TypesAll", "Modified", "HasCounters"}
	if p == kop.PredNotF2 {
		return "NotF2"
	}
	if p == kop.PredFaceDown {
		return "FaceDown"
	}
	if p == kop.PredWord7 {
		return "Word7"
	}
	if p == kop.PredTypesAllCMCLE {
		return "TypesAllCMCLE"
	}
	if p == kop.PredHasManaAbility {
		return "HasManaAbility"
	}
	if int(p) < len(names) {
		return names[p]
	}
	return "Pred(?)"
}
