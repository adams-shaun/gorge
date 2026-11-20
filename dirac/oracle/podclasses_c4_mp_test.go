package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// Four-seat pods of the classes the C4 wave lifted off the two-seat census
// (podclasses_mp_test.go has the harness).

// UnlessDefender$ (S:Mode$ CantAttack | ValidCard$ Card.Self |
// UnlessDefender$ ...: Sea Serpent's "can't attack unless defending player
// controls an Island", Orgg's "can't attack if defending player controls an
// untapped creature with power 3 or greater", Goblin Goon's "unless you
// control more creatures than defending player"). gorge AttackBlocked
// evaluates UnlessDefenderHolds on each (attacker, defending player) pair, a
// planeswalker's on its controller, so in a pod the creature may attack one
// opponent and not another; dirac judges the static per pair with the
// pair's defending seat bound (combat_cantattack.go, kop.StCantAtkDef). The
// mutation (DebugUnlessDefBlind) reads every pair on the next live seat, the
// two-seat reading.
var unlessDefPodDeck = []cardset.Entry{
	{Name: "Sea Serpent", Count: 4}, {Name: "Gorilla Pack", Count: 4},
	{Name: "Goblin Goon", Count: 4}, {Name: "Branded Brawlers", Count: 4},
	{Name: "Orgg", Count: 2}, {Name: "Grizzly Bears", Count: 4},
	{Name: "Island", Count: 6}, {Name: "Forest", Count: 6}, {Name: "Mountain", Count: 6},
}

// unlessDefNames are the deck's UnlessDefender$ creatures.
var unlessDefNames = map[string]bool{"Sea Serpent": true, "Gorilla Pack": true, "Goblin Goon": true, "Branded Brawlers": true, "Orgg": true}

// unlessDefWatch: gorge's attackers decision, three or more players alive,
// offers an UnlessDefender$ creature some living opponents but not all of
// them (the per-defender read decided the offer).
func unlessDefWatch(c *PickCtx) bool {
	g := c.G.E.G
	d := c.G.fr.d
	if d == nil || d.Kind != decision.KAttackers || podAlive(g) < 3 {
		return false
	}
	defs := map[state.ObjID]map[state.PlayerID]bool{}
	for _, o := range d.Options {
		ob := g.Obj(o.Obj)
		if o.Kind != "attacker" || o.Battle != 0 || ob == nil || ob.Face() == nil || !unlessDefNames[ob.Face().Name] {
			continue
		}
		if defs[o.Obj] == nil {
			defs[o.Obj] = map[state.PlayerID]bool{}
		}
		defs[o.Obj][o.Player] = true
	}
	for _, m := range defs {
		if len(m) > 0 && len(m) < podAlive(g)-1 {
			return true
		}
	}
	return false
}

// TestFourSeatUnlessDefenderLockstep: four-seat UnlessDefender$ games play in
// lockstep, each defending player judged on its own board.
func TestFourSeatUnlessDefenderLockstep(t *testing.T) {
	podClassLockstep(t, unlessDefPodDeck, 24, 10, unlessDefWatch, &dirac.DebugUnlessDefBlind)
}

// Clash (CR 701.31a: "clash with an opponent"). gorge's controller chooses
// which living opponent to clash with (chooseClashOpponent: a KChoose of one
// "player" option per opponent in AliveFrom(0) order, asked only with two or
// more), then both reveal and place their cards; dirac asks the same
// (act_clash.go raPodClashOpp) and holds the seat while the placements are
// asked. The mutation (DebugClashNextSeat) clashes with the next live seat
// unasked, the two-seat reading.
var clashPodDeck = []cardset.Entry{
	{Name: "Lash Out", Count: 6}, {Name: "Oaken Brawler", Count: 4},
	{Name: "Fire Juggler", Count: 4}, {Name: "Paperfin Rascal", Count: 4},
	{Name: "Grizzly Bears", Count: 4},
	{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 5}, {Name: "Island", Count: 5},
}

// clashWatch: gorge asks the clash opponent with three or more players
// alive.
func clashWatch(c *PickCtx) bool {
	d := c.G.fr.d
	return d != nil && d.ResumeKind == "clash_opponent" && len(d.Options) >= 2
}

// TestFourSeatClashLockstep: four-seat clash games play in lockstep, the
// controller choosing the opponent it clashes with.
func TestFourSeatClashLockstep(t *testing.T) {
	podClassLockstep(t, clashPodDeck, 24, 10, clashWatch, &dirac.DebugClashNextSeat)
}

// ChoosePlayer among the opponents read by a control change (Akroan Horse's
// "an opponent gains control of it", Fateful Handoff, Wishclaw Talisman,
// Discerning Financier's Player.Other, Zuko, Conflicted's return). gorge's
// effChoosePlayer asks the chosen opponent mid-resolution, and its
// GainControl NewController$ / ChangeZone GainControl$ read it; dirac asks it
// with OpExt ExtChoosePlayerMid and the new controller reads the
// resolution's choice (static_control.go ctrlPlayerRef,
// zone_returndefender.go retUnderOppSeat). The mutation (DebugChosenNextSeat)
// hands control to the next live seat, the two-seat reading.
var chooseCtrlPodDeck = []cardset.Entry{
	{Name: "Akroan Horse", Count: 6}, {Name: "Fateful Handoff", Count: 6},
	{Name: "Grizzly Bears", Count: 6}, {Name: "Llanowar Elves", Count: 4},
	{Name: "Forest", Count: 10}, {Name: "Swamp", Count: 8},
}

// chooseCtrlWatch: gorge asks a ChoosePlayer among two or more opponents of a
// control-changing card.
func chooseCtrlWatch(c *PickCtx) bool {
	g := c.G.E.G
	d := c.G.fr.d
	if d == nil || d.Kind != decision.KChoose || len(d.Options) < 2 || d.Options[0].Kind != "player" {
		return false
	}
	ob := g.Obj(d.Source)
	return ob != nil && ob.Face() != nil && (ob.Face().Name == "Akroan Horse" || ob.Face().Name == "Fateful Handoff")
}

// TestFourSeatChoosePlayerControlLockstep: four-seat games of opponent-chosen
// control changes play in lockstep, control going to the chosen opponent.
func TestFourSeatChoosePlayerControlLockstep(t *testing.T) {
	podClassLockstep(t, chooseCtrlPodDeck, 24, 10, chooseCtrlWatch, &dirac.DebugChosenNextSeat)
}

// Demonstrate (CR 702.152a: "choose an opponent to also copy it"). gorge's
// demonstrate trigger asks the caster, after the may-copy election, which of
// two or more living opponents also copies the spell (effDemonstrate stage
// 1), and emits the caster's copy and that opponent's only once it is
// answered. dirac asks the same (kw_demonstrate.go raTrigDemoOpp) and mints
// nothing before it; it used to copy for every opponent at once (a stray copy
// on the stack, an extra instance in the projection: Healing Technique in a
// real 4-seat precon pod). The mutation (DebugDemoAllOpps) copies for every
// opponent unasked, the two-seat reading.
var demoPodDeck = []cardset.Entry{
	{Name: "Excavation Technique", Count: 3}, {Name: "Healing Technique", Count: 3},
	{Name: "Incarnation Technique", Count: 2}, {Name: "Replication Technique", Count: 2},
	{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 3},
	{Name: "Plains", Count: 7}, {Name: "Forest", Count: 7}, {Name: "Swamp", Count: 4}, {Name: "Island", Count: 4},
}

// demoWatch: gorge asks the demonstrate opponent with two or more candidates.
func demoWatch(c *PickCtx) bool {
	d := c.G.fr.d
	return d != nil && d.ResumeKind == "demonstrate" && len(d.Options) >= 2 && d.Options[0].Kind == "player"
}

// TestFourSeatDemonstrateLockstep: four-seat demonstrate games play in
// lockstep, the chosen opponent alone copying alongside the caster.
func TestFourSeatDemonstrateLockstep(t *testing.T) {
	podClassLockstep(t, demoPodDeck, 24, 3, demoWatch, &dirac.DebugDemoAllOpps)
}
