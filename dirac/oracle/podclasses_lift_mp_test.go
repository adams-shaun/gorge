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

// Adeline, Resplendent Cathar: "whenever you attack, for each opponent,
// create a 1/1 white Human creature token that's tapped and attacking that
// player or a planeswalker they control" (RepeatEach RepeatPlayers$ Opponent
// | ChangeZoneTable$ True). gorge's effRepeatEach brackets the whole loop as
// one zone batch, so the tokens' enters checks fire together after the last
// opponent's (CR 603.6a; Cloakwood Swarmkeeper's "whenever one or more tokens
// you control enter, put a +1/+1 counter on it" triggers once, not once a
// token), and each token's defender among a player and their
// planeswalkers is asked in turn (CR 508.4). dirac holds the entries to the
// loop's end across those asks (zone_repeatbatch.go). The mutation
// (DebugZoneLoopPerIteration) fires each iteration's own.
var zoneLoopPodDeck = []cardset.Entry{
	{Name: "Adeline, Resplendent Cathar", Count: 8}, {Name: "Cloakwood Swarmkeeper", Count: 5},
	{Name: "Ajani Goldmane", Count: 3}, {Name: "Savannah Lions", Count: 4},
	{Name: "Plains", Count: 12}, {Name: "Forest", Count: 8},
}

// zoneLoopWatch: an Adeline trigger is on the stack with three or more
// players alive, as its controller holds priority.
func zoneLoopWatch(c *PickCtx) bool {
	g := c.G.E.G
	d := c.G.fr.d
	if d == nil || d.Kind != decision.KPriority || podAlive(g) < 3 {
		return false
	}
	for _, id := range g.Zone(state.ZStack, 0) {
		if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Adeline, Resplendent Cathar" && o.Controller == state.PlayerID(d.Player) {
			return true
		}
	}
	return false
}

// TestFourSeatZoneLoopLockstep: four-seat Adeline games play in lockstep,
// her tokens entering as one batch across the opponents.
func TestFourSeatZoneLoopLockstep(t *testing.T) {
	podClassLockstep(t, zoneLoopPodDeck, 24, 10, zoneLoopWatch, &dirac.DebugZoneLoopPerIteration)
}

// TestFourSeatZoneLoopDefenderMutation: Adeline's token attacks the opponent
// its iteration binds (gorge tokenAttackingPlayers' remembered player); the
// mutation (DebugTokAtkNextSeat) attacks the next living seat with every
// token, the two-seat reading, and must diverge.
func TestFourSeatZoneLoopDefenderMutation(t *testing.T) {
	st := runPodClass(t, zoneLoopPodDeck, 24, zoneLoopWatch, &dirac.DebugTokAtkNextSeat, true)
	if st.divs == 0 {
		t.Fatalf("mutation left %d games with no divergence", st.games)
	}
	t.Logf("mutation diverged in %d of %d games", st.divs, st.games)
}
