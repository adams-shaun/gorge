package kgen

import (
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// animateSelfEnchant takes an Animate's granted enchant out of its Keywords$
// value: the Licid cycle's "CARDNAME loses this ability and becomes an Aura
// enchantment with enchant creature" (AB$ Animate | Defined$ Self | Duration$
// Permanent | Types$ Enchantment,Aura | RemoveCardTypes$ True | Keywords$
// Enchant:Creature, SubAbility$ DB$ Attach). Gorge grants the keyword at
// layer 6 and its attachment state-based actions read the DERIVED enchant
// (engine/rules/attach.go auraStillMatchesEnchant): the attached Licid goes
// to the graveyard once its bearer stops matching the spec, or gains
// protection from it. The kernel's Aura arm (dirac/turn.go enchantLegal)
// reads the card's Enchant spec whenever the permanent's derived types hold
// Aura, so the spec lowers as the card's Enchant (lCard.AnimateEnchant). It
// is only ever reached while the Licid is an Aura, and the only thing that
// makes it one is this permanent self-animation, whose grant the spec is;
// every cast-time reader of the Enchant spec keys on the PRINTED Aura type,
// which the Licid lacks.
//
// The rest of the list returns for the other Keywords$ readers. A list with
// no Enchant word returns unchanged.
//
// RemoveThisAbility$ and RevertCost$ ride the same line: gorge reads neither
// (effAnimate never consults them), so the ability stays on the Aura and no
// revert is offered; the kernel's lowering matches that (both consumed here,
// only in this shape).
func (lc *lowerCtx) animateSelfEnchant(ps *params, kw string, o *lOverride, obj lRef, dur string) (string, error) {
	var rest []string
	spec := ""
	for _, w := range strings.Split(kw, "&") {
		w = strings.TrimSpace(w)
		if cards.KeywordHead(w) != "Enchant" {
			rest = append(rest, w)
			continue
		}
		if spec != "" {
			return "", refuse("keyword %q: a second granted Enchant", w)
		}
		spec = strings.TrimSpace(strings.TrimPrefix(w, "Enchant:"))
		if spec == "" || spec == w {
			return "", refuse("keyword %q", w)
		}
	}
	if spec == "" {
		return kw, nil
	}
	switch {
	case obj.Kind != rSelf || lc.granted > 0:
		return "", refuse("keyword \"Enchant:%s\": an Animate of another object", spec)
	case dur != "Permanent":
		return "", refuse("keyword \"Enchant:%s\": Animate Duration %q", spec, dur)
	case !o.HasSetTypes || !slices.Contains(o.SetSubtypes, "Aura"):
		return "", refuse("keyword \"Enchant:%s\": the Animate does not make it an Aura", spec)
	case slices.Contains(lc.out.Subtypes, "Aura") || lc.out.AnimateEnchant != nil:
		return "", refuse("keyword \"Enchant:%s\": the card already has an enchant", spec)
	}
	for _, k := range lc.out.Kw {
		if cards.KeywordHead(k) == "Enchant" {
			return "", refuse("keyword \"Enchant:%s\": the card already has an enchant", spec)
		}
	}
	// the spec's first field; any later one is the human prompt (gorge
	// auraStillMatchesEnchant's strings.Cut)
	spec, _, _ = strings.Cut(spec, ":")
	f, err := lc.filter(strings.TrimSpace(spec))
	if err != nil {
		return "", err
	}
	if f.Players != 0 {
		return "", refuse("keyword \"Enchant:%s\": a granted player enchant", spec)
	}
	lc.out.AnimateEnchant = &lTarget{F: f, Zone: zBattlefield, Mode: -1, Min: cst(1), Max: cst(1)}
	ps.consume("RemoveThisAbility", "RevertCost")
	return strings.Join(rest, " & "), nil
}

// animateRemoveEnchantmentTypes reads RemoveEnchantmentTypes$ True beside
// RemoveCardTypes$ True. Gorge's layer-4 walk (engine/rules/chars/types.go)
// strips the card types and every subtype they do not keep first, then the
// enchantment subtypes, and only then adds Types$: when Types$ names an
// enchantment subtype (the Licids' Aura) no enchantment subtype survives the
// RemoveCardTypes strip (effects.SubtypesKeptBy), so the second strip
// removes nothing and the override the Types$ block built is already the
// whole effect. Any other spelling refuses.
func (lc *lowerCtx) animateRemoveEnchantmentTypes(ps *params, o *lOverride) error {
	v, ok := ps.get("RemoveEnchantmentTypes")
	if !ok {
		return nil
	}
	if strings.TrimSpace(v) != "True" || !o.HasSetTypes || !o.HasSetSubtypes || o.Kept.Has("Aura") {
		return refuse("Animate RemoveEnchantmentTypes$ %q: only beside RemoveCardTypes$ that keeps no enchantment subtype", v)
	}
	return nil
}

// attachSelfOther excludes the source from an attach-Self target slot once
// the card's own animation makes it an Aura (the Licids' SubAbility$ DB$
// Attach | ValidTgts$ Creature, targeted while the Licid is still a
// creature): gorge's targetAsk never offers an attach SA's source as its
// destination (engine/rules/target_sub_offer.go attachSelfSource, CR
// 301.5c, 701.3a). Every other attach lowering keeps its slot as it was.
func (lc *lowerCtx) attachSelfOther(obj lRef) {
	if obj.Kind != rTarget || lc.out.AnimateEnchant == nil || obj.N >= len(lc.ab.Targets) {
		return
	}
	t := &lc.ab.Targets[obj.N]
	f := *t.F
	f.Clauses = slices.Clone(f.Clauses)
	for i := range f.Clauses {
		f.Clauses[i].Flags |= fOther
	}
	t.F = &f
}
