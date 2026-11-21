package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// zone_stageattach.go: what NewFromWorld admits beyond lands and plain
// creatures. Three shapes, each with state dirac can rebuild exactly:
//
//   - a plain creature token on the battlefield (a CardDef with TToken, no
//     ability), made after the decks' objects and so indexed after them;
//   - an Aura or Equipment whose abilities are statics that commute (a P/T
//     or keyword grant, can't attack, can't block) and the Equip ability,
//     attached to a creature on the battlefield;
//   - a sorcery or instant whose whole spell ability creates tokens, in a
//     hidden zone, the graveyard or the hand (it carries no state there).
//
// An attachment's layer timestamp is the order the staging attaches in
// (battlefield list order), not the order the game did: an observation does
// not show it. That is exact for effects that commute, and these are the only
// statics admitted.

// attachKeywords are the keywords an admitted static may grant.
const attachKeywords = combatKeywords | kop.KwHexproof

// stageableToken reports why def is not a plain creature token ("" if it is).
func stageableToken(def *kop.CardDef) string {
	if def.Types != kop.TCreature|kop.TToken {
		return "type"
	}
	if def.Keywords&^combatKeywords != 0 {
		return "keyword"
	}
	if len(def.Abilities) != 0 {
		return "ability"
	}
	return ""
}

// plainStatic reports whether ab is a static of an admitted shape: it
// affects the equipped or enchanted creature by a P/T or keyword grant, or
// by "can't attack" or "can't block", with no condition.
func plainStatic(ab *kop.Ability) bool {
	if ab.Kind != kop.AbStatic {
		return false
	}
	st := ab.Static
	switch st.Kind {
	case kop.StatContinuous, kop.StatCantAttack, kop.StatCantBlock:
	default:
		return false
	}
	if st.Kw&^attachKeywords != 0 {
		return false
	}
	st.Kind, st.Filter, st.A, st.B, st.Kw = 0, 0, 0, 0, 0
	return st == (kop.Static{})
}

// stageableAttachment reports why def is not an Aura or Equipment of the
// admitted shape ("" if it is).
func stageableAttachment(def *kop.CardDef) string {
	aura := def.Types == kop.TEnchantment|kop.TAura
	if !aura && def.Types != kop.TArtifact|kop.TEquipment {
		return "type"
	}
	if def.Keywords != 0 || def.Leyline || def.Dredge != 0 || def.UmbraArmor || def.Kicker != (kop.ManaCost{}) || def.AltCost != (kop.ManaCost{}) {
		return "keyword"
	}
	if aura == (def.EquipCost != (kop.ManaCost{})) || !aura && def.Enchant != (kop.TargetSpec{}) {
		return "ability"
	}
	for i := range def.Abilities {
		ab := &def.Abilities[i]
		if plainStatic(ab) || !aura && ab.Kind == kop.AbActivated && ab.AFlags == kop.AfEquip {
			continue
		}
		return "ability"
	}
	return ""
}

// stageableTokenSpell reports why def is not a sorcery or instant whose only
// ability creates tokens ("" if it is).
func stageableTokenSpell(def *kop.CardDef) string {
	if def.Types != kop.TSorcery && def.Types != kop.TInstant {
		return "type"
	}
	if def.Keywords != 0 || def.Kicker != (kop.ManaCost{}) || def.AltCost != (kop.ManaCost{}) || len(def.Abilities) != 1 {
		return "keyword"
	}
	ab := &def.Abilities[0]
	if ab.Kind != kop.AbSpell || ab.NTargets != 0 || ab.NModes != 0 || ab.AFlags != 0 || len(ab.Prog) == 0 {
		return "ability"
	}
	for i := range ab.Prog {
		if op := ab.Prog[i].Op; op != kop.OpCreateToken && op != kop.OpCreateTokenEx {
			return "ability"
		}
	}
	return ""
}

// checkTokenAttach is validate's per-object check of the token and
// attachment fields: it returns the ErrUnstageable for object i, or nil.
func (w *World) checkTokenAttach(i int, firstToken *int) error {
	o := &w.Objs[i]
	def := &w.Tbl.Cards[o.Card]
	if o.Token {
		if *firstToken < 0 {
			*firstToken = i
		}
		if o.Zone != kop.ZoneBattlefield && o.Zone != kop.ZoneNone { // ZoneNone: it has ceased to exist
			return unstageable(ReasonToken, "object %d: a token in zone %d", i, o.Zone)
		}
		if o.Zone == kop.ZoneNone && (o.Attached || o.Tapped || o.Sick || o.Damage != 0 || o.P1P1 != 0 || o.combatMarked()) {
			return unstageable(ReasonToken, "object %d: a ceased token carries a mark", i)
		}
	} else if *firstToken >= 0 {
		return unstageable(ReasonLayout, "object %d: a card after the tokens", i)
	}
	if def.Types&(kop.TAura|kop.TEquipment) == 0 {
		if o.Attached {
			return unstageable(ReasonAttach, "object %d is not an Aura or Equipment", i)
		}
		return nil
	}
	if o.Zone != kop.ZoneBattlefield {
		if o.Attached {
			return unstageable(ReasonAttach, "object %d is attached off the battlefield", i)
		}
		return nil // in a hidden zone or the graveyard it is a card like any other
	}
	if !o.Attached {
		if def.Types&kop.TAura != 0 {
			return unstageable(ReasonAttach, "object %d: an Aura attached to nothing", i)
		}
		return nil
	}
	if int(o.Host) >= len(w.Objs) || int(o.Host) == i {
		return unstageable(ReasonAttach, "object %d: host %d", i, o.Host)
	}
	if h := &w.Objs[o.Host]; h.Zone != kop.ZoneBattlefield || w.Tbl.Cards[h.Card].Types&kop.TCreature == 0 {
		return unstageable(ReasonAttach, "object %d: host %d is not a battlefield creature", i, o.Host)
	}
	return nil
}

// stageAttached attaches each attached permanent of w to its host.
func (s *State) stageAttached(k *prep, w *World) {
	for i := range w.Objs {
		if o := &w.Objs[i]; o.Attached {
			s.attachTo(k, s.objs[i].slot, s.objs[o.Host].slot)
			s.touchSl(s.objs[i].slot)
		}
	}
}

// stageTokens makes w's tokens, in order, as the objects after the decks'.
// It reports false when one does not land on its index. A token that has
// ceased to exist keeps its index and holds ZoneNone, as the token branch of
// moveTo leaves it.
func (s *State) stageTokens(w *World) bool {
	for i := range w.Objs {
		o := &w.Objs[i]
		if !o.Token {
			continue
		}
		if s.newToken(o.Card, o.Owner) != Obj(i) {
			return false
		}
		if o.Zone == kop.ZoneNone {
			s.objs[i].zone = kop.ZoneNone
		}
	}
	return true
}
