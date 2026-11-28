package kgen

import "testing"

// TestDrawnOwnerScopeShapes: a Mode$ Drawn line whose ValidCard$ scopes the
// draw by the DRAWN CARD's OWNER lowers as a player scope on the drawing
// player (a card is drawn from its owner's library into its owner's hand,
// CR 108.4a; gorge drawnMatches reads ValidCard$ against the drawn object).
// Card.OwnedBy Opponent.NonActive is Tataru Taru's "whenever an opponent
// draws a card, if it isn't that player's turn" (opponent of the controller,
// not the active player); Card.OwnedBy Player.EnchantedBy is the draw Curses'
// "whenever enchanted player draws a card" (Curse of Fool's Wisdom, Psychic
// Possession: the player the source Aura is attached to).
func TestDrawnOwnerScopeShapes(t *testing.T) {
	for _, c := range []struct {
		name         string
		who          uint8
		drawerNotAct bool
		oncePer      bool
	}{
		{"Tataru Taru", whoOpp, true, true},
		{"Curse of Fool's Wisdom", whoEnchanted, false, false},
		{"Psychic Possession", whoEnchanted, false, false},
	} {
		lc := lowerNamedCard(t, c.name)
		if len(lc.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", c.name, lc.Refusals)
			continue
		}
		tr := kdrawnvpTrigger(t, lc)
		if tr.Whose != c.who || tr.DrawerNotActive != c.drawerNotAct || tr.OncePer != c.oncePer {
			t.Errorf("%s: Whose %d DrawerNotActive %v OncePer %v, want %d %v %v",
				c.name, tr.Whose, tr.DrawerNotActive, tr.OncePer, c.who, c.drawerNotAct, c.oncePer)
		}
	}
}
