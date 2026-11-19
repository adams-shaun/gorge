package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// More four-seat pods of classes the Commander pod census once held to two
// seats (podclasses_mp_test.go has the harness).

// podAlive counts the players still in the game.
func podAlive(g *state.Game) int {
	n := 0
	for p := range g.Players {
		if !g.Players[p].Lost {
			n++
		}
	}
	return n
}

// podOnStack reports whether a spell or ability of the named card is on the
// stack.
func podOnStack(g *state.Game, name string) bool {
	for _, id := range g.Zone(state.ZStack, 0) {
		if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return true
		}
	}
	return false
}

// Fractured Identity: "exile target nonland permanent. Each player other
// than its controller creates a token that's a copy of it." gorge mints a
// copy for every living player from the caster in turn order but the
// exiled permanent's last-known controller, inside one zone batch
// (effCopyPermanent, copyPermanentController's NonRememberedController,
// definedSpec; CR 111.1, 603.6a); dirac walks the same owners
// (copy_nonremctrl.go), a copy's as-enters colour (Heraldic Banner) or an
// Aura copy's host (Pacifism) parking the walk on that owner's ask. The
// mutation (DebugNonRemOneOpp) makes the next living seat's copy alone, the
// two-seat reading.
var nonRemPodDeck = []cardset.Entry{
	{Name: "Fractured Identity", Count: 8}, {Name: "Heraldic Banner", Count: 4},
	{Name: "Pacifism", Count: 4}, {Name: "Soul Warden", Count: 4},
	{Name: "Coral Merfolk", Count: 3},
	{Name: "Plains", Count: 9}, {Name: "Island", Count: 8},
}

// nonRemWatch: gorge asks a player other than the caster while a Fractured
// Identity resolves with three or more players alive (a copy's owner
// answering its as-enters colour or its Aura's host).
func nonRemWatch(c *PickCtx) bool {
	g := c.G.E.G
	if d := c.G.fr.d; d == nil || d.Kind == decision.KPriority || podAlive(g) < 3 || !podOnStack(g, "Fractured Identity") {
		return false
	}
	for _, id := range g.Zone(state.ZStack, 0) {
		if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Fractured Identity" {
			return o.Controller != state.PlayerID(c.G.fr.d.Player)
		}
	}
	return false
}

// TestFourSeatNonRememberedControllerLockstep: four-seat Fractured Identity
// games play in lockstep, each other player making its copy.
func TestFourSeatNonRememberedControllerLockstep(t *testing.T) {
	podClassLockstep(t, nonRemPodDeck, 24, 10, nonRemWatch, &dirac.DebugNonRemOneOpp)
}
