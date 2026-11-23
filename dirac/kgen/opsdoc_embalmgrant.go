package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"op:GrantEmbalmOwnCost", "OpGrantEmbalmOwnCost (Obj the graveyard card, C the granting card's latent embalm body)",
			"The graveyard card Obj gains embalm until end of turn, its embalm cost being its mana cost (CR 702.128a; gorge's pump registers the derived keyword [Embalm:CardManaCost] and the granted-keyword offer runs the printed kwEmbalm expander). One object bit (mechEmbalmGrant), ended by a zone change or the cleanup. The body is the granting card's own latent ability, gorge's expansion lowered with a zero mana cost; the kernel offers it at sorcery speed for each creature card in the controller's graveyard holding the grant, charging the card's mana cost and exiling it, and the copy is a white Zombie with no mana cost. Only the Embalm:CardManaCost grant lowers: every other granted keyword, Eternalize:CardManaCost included, refuses by name. dirac/kw_embalmgrant.go. Cursecloth Wrappings.",
		},
	)
}
