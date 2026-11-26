package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// K:Cipher (CR 702.99; dirac/kw_cipher.go).
//
// gorge's printed expansion is one SVar (engine/cards/kw_cipher.go) that
// rules.cipherTailFor appends as the LAST instruction of a resolving (non-copy)
// Cipher spell's chain: "Then you may exile this spell card encoded on a
// creature you control" (effects/cipher.go effCipher). The combat-damage
// trigger the encoded card grants is synthesized by rules
// (checkCipherTriggers, the queue's "__kwCipher:" push) and is the kernel's
// own (an abDelayed entry, kw_cipher.go), so the keyword lowers to one tail
// instruction: OpExt ExtCipherEncode appended to the printed spell program.
//
// Refused by name: a permanent or multi-face card (gorge splices the tail
// into an instant or sorcery's resolution only), a Cipher spell beside a
// cast route that moves the resolved spell (Flashback, Rebound, Buyback,
// Mayhem, Harmonize, Paradigm), and a spell whose body casts a card
// (OpCastFree: the kernel makes that cast after its program ends, gorge
// during the resolution, so the encode ask would come first; Arcane Heist).

func init() {
	kwHandlers["Cipher"] = (*lowerCtx).lowerCipherKw
	instrOp("CipherEncode", (*instrCtx).opCipherEncode)
	opsEntries = append(opsEntries,
		opsDoc{"ext:CipherEncode", "OpExt ExtCipherEncode (no operands)",
			"A resolving Cipher spell's tail (CR 702.99a; gorge cipherTailFor/effCipher): a spell card (not a copy) still on the stack asks its controller to exile it encoded on a creature they control (ChGenCipher: the creatures in battlefield order, then Done; posed with one creature too, none asking nothing). A pick moves the card from the stack to exile and records the link (the object-set store's osEncoded record, the creature at its ver): while the creature stays on the battlefield and the card in exile, each time it deals combat damage to a player its controller gets a trigger (an abDelayed entry, cipherTrigAmt) that may cast a copy of the card without paying its mana cost (OpCastFree over the card, CopyCard). dirac/kw_cipher.go. Call of the Nightwing, Hands of Binding, Last Thoughts."})
}

func (ic *instrCtx) opCipherEncode() error {
	ic.ki.Op, ic.ki.A = kop.OpExt, kop.Const(int(kop.ExtCipherEncode))
	return nil
}

// lowerCipherKw marks the face; lowerCipher (lowerTail) appends the tail.
func (lc *lowerCtx) lowerCipherKw(node, param string) {
	if param != "" {
		lc.refuseNode(node, "Cipher with a parameter %q", param)
		return
	}
	lc.cipher = true
}

// lowerCipher appends the encode instruction to the printed spell program,
// after the card's lines are lowered.
func (lc *lowerCtx) lowerCipher() {
	if !lc.cipher {
		return
	}
	out := lc.out
	const node = "K:Cipher"
	if out.Types&tPermanent != 0 || out.Types&(tInstant|tSorcery) == 0 {
		lc.refuseNode(node, "Cipher on a permanent card")
		return
	}
	if faceLayout(lc.card, out.Token) != kop.FaceNone || len(lc.card.Faces) != 1 {
		lc.refuseNode(node, "Cipher on a multi-face card")
		return
	}
	if out.Flashback != nil || out.Rebound != nil || out.Mayhem != nil || out.Harmonize != nil || out.Paradigm || out.Buyback != nil {
		lc.refuseNode(node, "Cipher beside a cast route that moves the resolved spell")
		return
	}
	ab := lc.spellAbility()
	if ab == nil || len(ab.Prog) == 0 {
		return // the printed spell is already refused
	}
	for _, in := range ab.Prog {
		if in.Op == "CastFree" {
			lc.refuseNode(node, "Cipher on a spell that casts a card (the kernel's free cast follows the program)")
			return
		}
	}
	ab.Prog = append(ab.Prog, lInstr{Op: "CipherEncode", Note: "CR 702.99a: then you may exile this spell card encoded on a creature you control"})
}
