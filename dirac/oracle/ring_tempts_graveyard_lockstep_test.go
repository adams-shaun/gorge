package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// ringwraithsGraveyardScenario opens with Call of the Ring on seat 0's
// battlefield and Ringwraiths in its graveyard: Call of the Ring's upkeep
// trigger tempts the Ring every turn, and Ringwraiths must return itself from
// the graveyard (CR 603.6: its TriggerZones$ Graveyard line is a
// graveyard-watcher). With no creature controlled the temptation still
// happens (CR 701.54d) and Ringwraiths' line has no ValidCard$, so it fires
// on the no-bearer event too.
const ringwraithsGraveyardScenario = `{
 "format":"gorge-start-1","focus":"Ringwraiths","players":2,"opponent":"explore","active":0,
 "setup":{
  "p0":{"battlefield":["Call of the Ring","Swamp","Swamp","Swamp"],"graveyard":["Ringwraiths"]},
  "p1":{"battlefield":["Grizzly Bears"]}
 }
}`

// TestRingTemptsGraveyardLockstep plays a seeded lockstep game from the
// scripted start above: the kernel's Ring-tempt walk must queue the
// graveyard listener exactly as gorge does, so Ringwraiths returns to hand
// in both engines. A kernel that scanned only the battlefield would leave
// Ringwraiths in the graveyard and diverge at the next compare; the test
// also checks gorge's own log for the temptation and the return, so a game
// that never reaches the trigger fails rather than passing vacuously.
func TestRingTemptsGraveyardLockstep(t *testing.T) {
	t.Parallel()
	c, err := OpenCorpus(corpusDirOrSkip(t), "")
	if err != nil {
		t.Fatal(err)
	}
	sc, err := ParseScenario([]byte(ringwraithsGraveyardScenario))
	if err != nil {
		t.Fatal(err)
	}
	m, err := c.ScenarioMatchup(sc, 0)
	if err != nil {
		t.Fatal(err)
	}
	set, tbl := scriptedStartTable(t, "thering", m)
	cfg := Config{}
	rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 6}
	for _, seed := range []uint64{1, 2, 3} {
		gameSeed := sc.GameSeed(seed)
		lead, err := NewGorgeSide(m, gameSeed, cfg, nil)
		if err != nil {
			t.Fatalf("seed %d: %v", gameSeed, err)
		}
		if got := int(lead.E.G.StartingPlayer); got != sc.Active {
			t.Fatalf("seed %d: starting player %d, the scenario wants %d", gameSeed, got, sc.Active)
		}
		scriptedStartPlaced(t, lead, m, gameSeed)
		if !ringwraithsInGraveyard(lead) {
			t.Fatalf("seed %d: the start placed no Ringwraiths in seat 0's graveyard", gameSeed)
		}
		k, err := SetSide(set, tbl, cfg)(m, gameSeed)
		if err != nil {
			t.Fatalf("seed %d: %v", gameSeed, err)
		}
		res := LockstepLed(m, gameSeed, lead, k, RandomPolicy(gameSeed), rc)
		CloseSide(k)
		if !res.Clean() {
			if res.Div == nil {
				t.Fatalf("seed %d: not clean without a divergence: stall %q, unmodelled %v",
					gameSeed, res.Stall, res.StallUnmodelled)
			}
			ringFail(t, gameSeed, res)
		}
		tempts, returns := ringwraithsGraveyardEvents(lead)
		if tempts == 0 {
			t.Fatalf("seed %d: no RingTemptsYou event: the scenario reached no temptation", gameSeed)
		}
		if returns == 0 {
			t.Fatalf("seed %d: Ringwraiths never returned from the graveyard: %d temptation(s), no graveyard trigger", gameSeed, tempts)
		}
		t.Logf("seed %d: %d decisions, %d temptation(s), %d graveyard return(s)", gameSeed, res.Decisions, tempts, returns)
	}
}

// ringwraithsInGraveyard reports whether seat 0's graveyard holds the card.
func ringwraithsInGraveyard(g *GorgeSide) bool {
	for _, id := range g.E.G.Zone(state.ZGraveyard, 0) {
		if firstFaceName(g.E.G, id) == "Ringwraiths" {
			return true
		}
	}
	return false
}

// ringwraithsGraveyardEvents counts the game's RingTemptsYou events and the
// Ringwraiths returns from a graveyard to a hand.
func ringwraithsGraveyardEvents(g *GorgeSide) (tempts, returns int) {
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		switch {
		case e.Kind == events.RingTemptsYou:
			tempts++
		case e.Kind == events.MoveZone && e.From == state.ZGraveyard && e.To == state.ZHand &&
			firstFaceName(g.E.G, e.Obj) == "Ringwraiths":
			returns++
		}
	}
	return tempts, returns
}
