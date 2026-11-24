package effects

import "github.com/adams-shaun/gorge/engine/state"

// topGraveyardWordMatches is the reader of Forge's TopGraveyard,
// TopGraveyard2 and TopGraveyardCreature words. CR 404.1 puts a card on top
// of its owner's graveyard and CR 404.2 keeps the graveyard a single ordered
// pile; gorge appends to the graveyard slice, so its LAST index is the top.
//   - TopGraveyard: the object is the top card (Corpse Dance, Alms).
//   - TopGraveyard2: the object is one of the top two cards (Phyrexian
//     Grimoire).
//   - TopGraveyardCreature: the object is the topmost creature card, any
//     non-creature cards above it skipped (Shallow Grave, Bone Dancer).
//
// A card outside a graveyard never matches.
func topGraveyardWordMatches(kind wordKind, g *state.Game, o *state.Object, sc *SpecContext) bool {
	if o.Zone != state.ZGraveyard {
		return false
	}
	ids := g.Zone(state.ZGraveyard, o.Owner)
	switch kind {
	case wordTopGraveyard:
		return len(ids) > 0 && ids[len(ids)-1] == o.ID
	case wordTopGraveyard2:
		for i := len(ids) - 1; i >= 0 && i >= len(ids)-2; i-- {
			if ids[i] == o.ID {
				return true
			}
		}
		return false
	case wordTopGraveyardCreature:
		for i := len(ids) - 1; i >= 0; i-- {
			c := g.Obj(ids[i])
			if c != nil && hasTypeCtxPtr(c, "Creature", sc) {
				return ids[i] == o.ID
			}
		}
	}
	return false
}
