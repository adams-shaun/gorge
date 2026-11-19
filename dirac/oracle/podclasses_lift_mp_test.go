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
	{Name: "Savannah Lions", Count: 7},
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

// Melee and The Beacon of Kahless. Melee's pump is "+1/+1 for each opponent
// you attacked this combat": gorge captures the declaration's distinct
// defending seats as the trigger is queued, a planeswalker's attack counting
// its controller (meleeRemembered, cards.MeleePumpCount; CR 702.121a-b), and
// dirac reads the declaration's seat mask (ExOppsAttacked N 1,
// value_oppsattacked.go). The Beacon's Battlefront ability adds one mana of
// a chosen colour per opponent attacked this turn (the players named as
// defenders, gorge OpponentsAttackedThisTurn): gorge's census unit holds
// both abilities, the second at its amount, and its planner and castability
// read it; dirac plans it as the per-count pair view (mana_dynpair.go, kgen
// dynamicLargerAnyColour). The mutation (DebugMeleeOne) counts one opponent,
// the two-seat reading.
var meleePodDeck = []cardset.Entry{
	{Name: "Ferocious Colonist", Count: 8}, {Name: "Glory to the Empire!", Count: 4},
	{Name: "The Beacon of Kahless", Count: 4}, {Name: "Savannah Lions", Count: 4},
	{Name: "Goblin Piker", Count: 2},
	{Name: "Mountain", Count: 12}, {Name: "Plains", Count: 6},
}

// meleeWatch: gorge asks a player to act in the declare-attackers step of a
// declaration that attacked two or more players, three or more alive.
func meleeWatch(c *PickCtx) bool {
	g := c.G.E.G
	if podAlive(g) < 3 || g.Step != state.StepDeclareAttackers {
		return false
	}
	var seats [8]bool
	n := 0
	for _, id := range g.Zone(state.ZBattlefield, g.Active) {
		if o := g.Obj(id); o != nil && o.IsAttacking && o.Attacking >= 0 && int(o.Attacking) < len(seats) && !seats[o.Attacking] {
			seats[o.Attacking] = true
			n++
		}
	}
	return n >= 2
}

// TestFourSeatMeleeLockstep: four-seat melee games play in lockstep, the
// pumps counting every opponent the declaration attacked.
func TestFourSeatMeleeLockstep(t *testing.T) {
	podClassLockstep(t, meleePodDeck, 24, 10, meleeWatch, &dirac.DebugMeleeOne)
}

// beaconWatch: gorge asks a player who controls an untapped Beacon of
// Kahless after attacking two or more players this turn (its Battlefront
// ability then adds more than its first).
func beaconWatch(c *PickCtx) bool {
	g := c.G.E.G
	p := state.PlayerID(c.G.fr.d.Player)
	if p != g.Active || c.G.E.OpponentsAttackedThisTurn(p) < 2 {
		return false
	}
	for _, id := range g.Zone(state.ZBattlefield, p) {
		if o := g.Obj(id); o != nil && !o.Tapped && o.Face() != nil && o.Face().Name == "The Beacon of Kahless" {
			return true
		}
	}
	return false
}

// TestFourSeatBeaconLockstep: four-seat Beacon of Kahless games play in
// lockstep after attacks at two or more players; the mutation
// (DebugOppsAttOne) reads the opponents attacked this turn as at most one.
func TestFourSeatBeaconLockstep(t *testing.T) {
	podClassLockstep(t, meleePodDeck, 24, 10, beaconWatch, &dirac.DebugOppsAttOne)
}

// The attack tax (Propaganda, Ghostly Prison, Windborn Muse: "creatures
// can't attack you unless their controller pays {2} for each creature they
// control that's attacking you"). gorge prices each (attacker, defender)
// pair by the statics whose Target$ admits that defender, so only an attack
// on the tax's controller is charged, and budgets the declaration over every
// defender it offers (rules/attack_cost.go attackPairCharge, attackBudget;
// CR 508.1d, 508.1g-h); dirac prices the same pairs over every seat still in
// the game (combat_attacktax.go atkTaxScope, atkTaxTargets). The mutation
// (DebugAtkTaxFree) charges nothing.
var attackTaxPodDeck = []cardset.Entry{
	{Name: "Propaganda", Count: 4}, {Name: "Ghostly Prison", Count: 4},
	{Name: "Windborn Muse", Count: 2}, {Name: "Savannah Lions", Count: 6},
	{Name: "Coral Merfolk", Count: 6},
	{Name: "Plains", Count: 9}, {Name: "Island", Count: 9},
}

// attackTaxWatch: gorge asks the active player's attackers while another
// player controls an attack tax and two or more other players are alive.
func attackTaxWatch(c *PickCtx) bool {
	d := c.G.fr.d
	g := c.G.E.G
	if d == nil || d.Kind != decision.KAttackers || podAlive(g) < 3 {
		return false
	}
	for q := range g.Players {
		if state.PlayerID(q) == g.Active {
			continue
		}
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(q)) {
			if o := g.Obj(id); o != nil && o.Face() != nil {
				switch o.Face().Name {
				case "Propaganda", "Ghostly Prison", "Windborn Muse":
					return true
				}
			}
		}
	}
	return false
}

// TestFourSeatAttackTaxLockstep: four-seat attack-tax games play in
// lockstep, the declarations priced per defender.
func TestFourSeatAttackTaxLockstep(t *testing.T) {
	podClassLockstep(t, attackTaxPodDeck, 24, 10, attackTaxWatch, &dirac.DebugAtkTaxFree)
}

// Archangel of Tithes: an attack tax on attacks at its controller while it
// is untapped, and a block tax ("creatures can't block unless their
// controller pays {1} for each of those creatures") while it is attacking.
// gorge charges the block tax to each blocking player (rules attack_cost.go;
// CR 509.1c-e), whichever defender the Archangel attacks; dirac prices each
// defending player's blocks the same (combat_blocktax.go). The mutation
// (DebugAtkTaxFree) drops the attack half.
var tithesPodDeck = []cardset.Entry{
	{Name: "Archangel of Tithes", Count: 8}, {Name: "Savannah Lions", Count: 8},
	{Name: "Coral Merfolk", Count: 6}, {Name: "Plains", Count: 18},
}

// tithesWatch: gorge asks blockers of a defending player while an
// Archangel of Tithes attacks, three or more players alive.
func tithesWatch(c *PickCtx) bool {
	d := c.G.fr.d
	g := c.G.E.G
	if d == nil || d.Kind != decision.KBlockers || podAlive(g) < 3 {
		return false
	}
	for _, id := range g.Zone(state.ZBattlefield, g.Active) {
		if o := g.Obj(id); o != nil && o.IsAttacking && o.Face() != nil && o.Face().Name == "Archangel of Tithes" {
			return true
		}
	}
	return false
}

// TestFourSeatTithesLockstep: four-seat Archangel of Tithes games play in
// lockstep, both taxes priced per player.
func TestFourSeatTithesLockstep(t *testing.T) {
	podClassLockstep(t, tithesPodDeck, 24, 10, tithesWatch, &dirac.DebugAtkTaxFree)
}

// TestFourSeatTithesBlockMutation: the block half; the mutation
// (DebugBlkTaxFree) lets every block go uncharged and must diverge.
func TestFourSeatTithesBlockMutation(t *testing.T) {
	st := runPodClass(t, tithesPodDeck, 24, tithesWatch, &dirac.DebugBlkTaxFree, true)
	if st.divs == 0 {
		t.Fatalf("mutation left %d games with no divergence", st.games)
	}
	t.Logf("mutation diverged in %d of %d games", st.divs, st.games)
}
