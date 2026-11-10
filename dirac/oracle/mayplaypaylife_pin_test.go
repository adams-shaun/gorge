package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// castThroughPayLifeGrant is the condition that Inside Information is in a
// graveyard and a spell is on the stack that was cast through a may-play
// grant: the card it exiled was cast for life, not mana. The spell must be
// one its caster does not own: Inside Information exiles the opponent's
// library, so this excludes Snowslope Hunter's own-library permission (which
// carries no pay-life bit and never reaches the canon pay-life arm).
func castThroughPayLifeGrant(name string) func(ref *GorgeSide) bool {
	return func(ref *GorgeSide) bool {
		g := ref.E.G
		seen := false
		for p := range g.Players {
			seen = seen || inZoneNamed(g, state.ZGraveyard, state.PlayerID(p), name)
		}
		if !seen {
			return false
		}
		for _, id := range g.Zone(state.ZStack, 0) {
			if o := g.Obj(id); o != nil && o.CastFlags&state.FlagMayPlay != 0 && o.Owner != o.Controller {
				return true
			}
		}
		return false
	}
}

func TestPinnedInsideInformationPayLife(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "std-hob", Config{GorgeAutopaySurface: true}, castThroughPayLifeGrant("Inside Information"))
}
