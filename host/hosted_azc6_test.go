package host

// az-c6 (bots/azc6; config from docs/superpowers/reports/2026-10-15-az-c5.md) through the host. It is
// az-c4 with another network (bots/azc6), so it owes az-c4's rows
// (hosted_azc4_test.go), run here for the az-c6 registration:
//
//   - TestAZC6NeverSeesTheHumansHiddenCards: the privacy proof, the same three
//     hosted games against a human as az-c4's (closedPrivacyGames).
//   - TestHostedEnvSeatsIgnoreTheRealHiddenCardsAZC6: the hidden-card swap
//     row every hosted search entry owes (spec 2026-09-28 §5.3).
//
// The decklist gate, the closed-list guard and the determinism row cover
// az-c6 in hosted_azc4_test.go.

import (
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azc6"
	"github.com/adams-shaun/gorge/seat"
)

func TestAZC6NeverSeesTheHumansHiddenCards(t *testing.T) { closedPrivacyGames(t, azc6.Policy) }

func TestHostedEnvSeatsIgnoreTheRealHiddenCardsAZC6(t *testing.T) {
	enableClosedPolicy(t, azc6.Policy)
	azLeakRow(t, azc6.Policy, func(actor uint64) seat.Seat {
		s, err := bots.New(azc6.Policy, bots.Options{Seed: actor, AutoPayMana: true})
		if err != nil {
			t.Fatalf("az-c6: %v", err)
		}
		return s
	})
}
