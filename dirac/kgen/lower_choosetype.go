package kgen

import (
	"math/bits"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/effects"
)

// an as-enters "choose a creature type" narrowed by
// ValidTypes$ (Dawn-Blessed Pennant, Eclipsed Realms: "choose Elemental,
// Elf, ..."). gorge creatureTypeOptions filters its option list with
// effects.FilterTypeWords; the card carries the allowed vocabulary indexes
// (CardDef.TypeValid, in vocabulary order) and the instruction's B = 1
// tells the kernel's ask to offer only those (act_choosetype.go).

// chooseTypeValid reads ValidTypes$ (InvalidTypes$ refuses) onto in and
// the card's TypeValid list. Every listed word must be in gorge's
// creature-type vocabulary.
func (lc *lowerCtx) chooseTypeValid(ps *params, in *lInstr) error {
	if _, ok := ps.get("InvalidTypes"); ok {
		return refuse("ChooseType InvalidTypes$")
	}
	list, ok := ps.get("ValidTypes")
	if !ok {
		return nil
	}
	if lc.out.TypeValid != nil {
		return refuse("ChooseType ValidTypes$ twice on one card")
	}
	vocab := effects.CreatureTypeWordList()
	allow := map[string]bool{}
	for _, w := range strings.Split(list, ",") {
		w = strings.TrimSpace(w)
		found := false
		for _, v := range vocab {
			if v == w {
				found = true
			}
		}
		if !found {
			return refuse("ChooseType ValidTypes %q", w)
		}
		allow[w] = true
	}
	var idx []uint16
	for v, w := range vocab {
		if allow[w] {
			idx = append(idx, uint16(v))
		}
	}
	if len(idx) == 0 {
		return refuse("ChooseType ValidTypes$ empty")
	}
	lc.out.TypeValid = idx
	in.B = cst(1)
	in.CS = "valid=" + strings.TrimSpace(list)
	return nil
}

// cardTypeChoice maps gorge's chooseCardTypes vocabulary to kop type bits.
var cardTypeChoice = map[string]kop.TypeMask{
	"Artifact": kop.TArtifact, "Battle": kop.TBattle, "Creature": kop.TCreature, "Enchantment": kop.TEnchantment,
	"Instant": kop.TInstant, "Kindred": kop.TKindred, "Land": kop.TLand, "Planeswalker": kop.TPlaneswalker,
	"Sorcery": kop.TSorcery,
}

// chooseCardType lowers a mid-resolution ChooseType | Type$ Card |
// ValidTypes$ <card types> (Winding Way; gorge effChooseType over
// TypeChoiceLabels("Card", ...): one KChoose option per listed card type,
// in the vocabulary's alphabetical order) to OpChooseType with C = the
// offered types' kop.TypeMask. The answer is the resolution's chosen card
// type, which a later Card.ChosenType filter reads (F2ChosenType).
func (lc *lowerCtx) chooseCardType(ps *params) error {
	if d, ok := ps.get("Defined"); ok && d != "You" {
		return refuse("ChooseType Card Defined %q", d)
	}
	var m kop.TypeMask
	for _, v := range strings.Split(ps.str("ValidTypes"), ",") {
		b, ok := cardTypeChoice[strings.TrimSpace(v)]
		if !ok {
			return refuse("ChooseType Card ValidTypes %q", v)
		}
		m |= b
	}
	if bits.OnesCount16(uint16(m)) < 2 {
		return refuse("ChooseType Card with fewer than two types (gorge's no-ask fallback)")
	}
	if _, ok := ps.get("AtRandom"); ok {
		return refuse("ChooseType AtRandom")
	}
	lc.emit(lInstr{Op: "ChooseType", Obj: lRef{Kind: rYou}, C: int(m), Note: "choice among card types"})
	return nil
}

// chooseCreatureTypeMid lowers a mid-resolution creature-type ChooseType
// (Kindred Charge; A Killer Among Us's enters trigger, "then secretly
// choose Human, Merfolk, or Goblin"). gorge effChooseType: an absent
// Defined$ is the controller, ValidTypes$ narrows the option list exactly as
// the as-enters ask's (h.TypeChoices over the same filter), so it rides the
// card's TypeValid list and B = 1 (the kernel's mid-resolution ask offers
// only those, act_choosetype.go midTypeOptions). Secretly$ True changes no
// decision: gorge marks the Choose event Secret so the view redacts the
// answer from the other seats, and the options, the chooser and the stored
// ChosenType are the public ask's.
func (lc *lowerCtx) chooseCreatureTypeMid(ps *params) error {
	if d, ok := ps.get("Defined"); ok && strings.TrimSpace(d) != "You" {
		return refuse("ChooseType shape")
	}
	if s, ok := ps.get("Secretly"); ok && !strings.EqualFold(strings.TrimSpace(s), "True") {
		return refuse("ChooseType Secretly$ %q", s)
	}
	in := lInstr{Op: "ChooseType", Obj: lRef{Kind: rYou}}
	if err := lc.chooseTypeValid(ps, &in); err != nil {
		return err
	}
	lc.emit(in)
	return nil
}
