package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An as-enters "choose a card type" (Archon of Valor's Reach, Cloud Key,
// Stenn, Paranoid Partisan; gorge effChooseType over TypeChoiceLabels("Card",
// ValidTypes$, InvalidTypes$)): the enters program's OpChooseType with C = the
// offered kop.TypeMask. The ask offers one option per type (chooseCardTypeOptions,
// gorge's alphabetical order) valued cardTypeLabelH, and the answer is stored
// in the permanent's one chosen byte as its cardTypeChoice index (typeVocabCode).
//
// perm.chosen stays one byte, so the same value 0..8 also reads as a creature
// subtype index when a reader does not know the source's kind. The kind is a
// per-card prep flag (genPrep.cardKind): the readers below take it from the
// source permanent's card, and a flagged source never reads its byte as a
// subtype. A source with no answer (255) matches nothing (gorge's
// s.ChosenType != "" gate).

// DebugCardKindIgnore is the mutation switch of TestChooseTypeCardKindsLockstep:
// a Card.ChosenType read from a card-type source ignores the chosen type and
// matches every card.
var DebugCardKindIgnore bool

// isCardKindChoose reports whether in is an as-enters card-type ask (inside an
// enters program; a mid-resolution one is Winding Way's, kw_madness.go).
func isCardKindChoose(in *kop.Instr) bool {
	return in.Op == kop.OpChooseType && in.C != 0
}

// prepCardKinds sets the per-card flag: the card's enters program asks for a
// card type. anyTypeValid also gates typeOptions, which builds this ask's options.
func prepCardKinds(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplEntersProgram {
				continue
			}
			for x := range a.Prog {
				if !isCardKindChoose(&a.Prog[x]) {
					continue
				}
				if g.cardKind == nil {
					g.cardKind = make([]bool, len(t.Cards))
				}
				g.cardKind[i] = true
				g.anyTypeValid = true
			}
		}
	}
}

// cardKindOptions fills the entering card's as-enters card-type ask; false when
// the card's enters program has none.
func cardKindOptions(cd *kop.CardDef, d *Decision) bool {
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplEntersProgram {
			continue
		}
		for j := range a.Prog {
			if in := &a.Prog[j]; isCardKindChoose(in) {
				chooseCardTypeOptions(d, kop.TypeMask(in.C))
				return true
			}
		}
	}
	return false
}

// cardKindValue is the cardTypeChoice index of option value v (an answer's
// target<<8|mode), 255 when v is no card-type label.
func cardKindValue(v int) uint8 {
	for i := range cardTypeLabelH {
		if int(cardTypeLabelH[i]) == v {
			return uint8(i)
		}
	}
	return 255
}

// cardKindOf reports whether card c's chosen byte is a card type.
func cardKindOf(k *prep, c CardID) bool {
	return k.gen != nil && int(c) < len(k.gen.cardKind) && k.gen.cardKind[c]
}

// srcCardKind reports whether the permanent at src holds a card-type answer (its
// card asks for one as it enters), and the answered type's bit (0: no answer).
func (s *State) srcCardKind(k *prep, src Slot) (kop.TypeMask, bool) {
	if src == NoSlot || k.gen == nil || k.gen.cardKind == nil || !cardKindOf(k, s.cardAt(src)) {
		return 0, false
	}
	return cardKindBit(s.perms[src].chosen), true
}

// cardKindBit is the card type bit of a chosen byte (0: none).
func cardKindBit(ch objWord) kop.TypeMask {
	if int(ch) >= len(cardTypeChoice) {
		return 0
	}
	return cardTypeChoice[ch].bit
}

// cardKindHas is a card-type source's Card.ChosenType: types t include the
// answered type. A source with no answer matches nothing.
func cardKindHas(bit, t kop.TypeMask) bool {
	if DebugCardKindIgnore {
		return true
	}
	return bit != 0 && t&bit != 0
}

// srcChosenHasCard is F2ChosenType for card cd read against the source
// permanent at src: its card-type answer, or its creature-subtype one
// (matchSpellAltCtx's reading).
func (s *State) srcChosenHasCard(k *prep, src Slot, cd *kop.CardDef) bool {
	if bit, ok := s.srcCardKind(k, src); ok {
		return cardKindHas(bit, cd.Types)
	}
	ch, ok := s.chosenTypeSub(src)
	has := ok && int(ch) < len(k.t.Subtypes) && cd.Subtypes.Has(int(ch))
	if ok && outsideSub(k, ch) {
		has = cd.AllCreatureTypes
	}
	return has
}

// permChosenType is F2ChosenType for the permanent at sl read against the
// source at src, whose creature-subtype answer is ch (ok: it has one): its
// card-type answer when its card asks for one, else the subtype compare
// (flags2Perm's reading).
func (s *State) permChosenType(k *prep, sl, src Slot, ch uint8, ok bool) bool {
	if bit, kind := s.srcCardKind(k, src); kind {
		return cardKindHas(bit, s.typesAt(k, sl))
	}
	return ok && (s.hasSubGen(k, sl, ch) || outsideSub(k, ch) && s.outsideChosenHas(k, sl, src))
}
