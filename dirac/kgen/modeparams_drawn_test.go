package kgen

import "testing"

// TestModeParamDrawnScopes: a Drawn trigger's ValidCard$ / ValidPlayer$ read
// through the shared mode-param reader (lower_modeparams.go) lowers every
// spelling of "which player drew" that the kernel's player-trigger walk gates
// with Whose (gorge drawnMatches reads ValidCard$
// against the drawn card and ValidPlayer$ against the drawing player).
func TestModeParamDrawnScopes(t *testing.T) {
	for _, c := range []struct {
		name     string
		params   map[string]string
		who      uint8
		yourTurn bool
		nth      int
	}{
		{"any card", map[string]string{"ValidCard": "Card"}, whoAny, false, 0},
		{"player.opponent", map[string]string{"ValidPlayer": "Player.Opponent", "Number": "2"}, whoOpp, false, 2},
		{"you", map[string]string{"ValidPlayer": "You", "Number": "1"}, whoYou, false, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			c.params["TriggerZones"] = "Battlefield"
			tr := kdrawnvpTrigger(t, kdrawnvpLower(t, c.params))
			if tr.Whose != c.who || tr.OnlyYourTurn != c.yourTurn || tr.OnlyOppTurn || tr.NthDraw != c.nth {
				t.Errorf("%v: Whose %d OnlyYourTurn %v NthDraw %d, want %d %v %d", c.params, tr.Whose, tr.OnlyYourTurn, tr.NthDraw, c.who, c.yourTurn, c.nth)
			}
		})
	}
}

// TestModeParamDrawnRefuses: the spellings the shared reader does not model
// still refuse by name: Player.Active is relative to the turn, not to the
// listener's controller, and a card-side qualifier beside a player scope is
// never read by the lowering.
func TestModeParamDrawnRefuses(t *testing.T) {
	for _, c := range []struct {
		name   string
		params map[string]string
		want   string
	}{
		{"active player", map[string]string{"ValidPlayer": "Player.Active"}, `ValidPlayer "Player.Active"`},
		{"non-active opponent", map[string]string{"ValidPlayer": "Player.Opponent+NonActive"}, `ValidPlayer "Player.Opponent+NonActive"`},
		// gorge's OwnedBy classifier reads only a Player.<qualifier> referent, so
		// it never matches this spelling (Tataru Taru never triggers there): the
		// kernel must not fire a line gorge cannot
		{"owned by a bare opponent spec", map[string]string{"ValidCard": "Card.OwnedBy Opponent.NonActive"}, `Drawn ValidCard "Card.OwnedBy Opponent.NonActive"`},
		{"card beside a player scope", map[string]string{"ValidCard": "Card", "ValidPlayer": "Opponent"}, `Drawn ValidCard "Card" ValidPlayer "Opponent"`},
		{"card type filter", map[string]string{"ValidCard": "Card.Land"}, `Drawn ValidCard "Card.Land"`},
	} {
		t.Run(c.name, func(t *testing.T) {
			c.params["TriggerZones"] = "Battlefield"
			enterstappedRefuseWant(t, kdrawnvpLower(t, c.params).Refusals, c.want)
		})
	}
}
