package rules

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// corpusInfectCard parses the live GPL corpus at test time so the
// kw:Infect primitives this ticket registers are exercised against the
// exact Forge scripts whose coverage entries they retire, the way
// combat_keywords_test.go's corpusKeywordCard does for its keywords.
func corpusInfectCard(t *testing.T, name string) *cards.Card {
	t.Helper()
	paths := map[string]string{
		"Blight Mamba":         "b/blight_mamba.txt",
		"Blightsteel Colossus": "b/blightsteel_colossus.txt",
		"Grafted Exoskeleton":  "g/grafted_exoskeleton.txt",
		"Ichor Rats":           "i/ichor_rats.txt",
		"Winding Constrictor":  "w/winding_constrictor.txt",
	}
	path, ok := paths[name]
	if !ok {
		t.Fatalf("no corpus path for %q", name)
	}
	c, ds := cards.Parse(filepath.Join("..", ".cards", "cardsfolder", path))
	if len(ds) != 0 {
		t.Fatalf("parse %s: %v", name, ds)
	}
	if ds = c.Link(); len(ds) != 0 {
		t.Fatalf("link %s: %v", name, ds)
	}
	return c
}

// TestInfectDamageToCreatureIsMinusOneCountersNotMarkedDamage pins CR
// 702.90b's creature half: Blight Mamba's combat damage to its blocker is
// dealt as a -1/-1 counter, with NO marked damage, while the blocker's own
// (non-infect) hit back stays ordinary marked damage.
func TestInfectDamageToCreatureIsMinusOneCountersNotMarkedDamage(t *testing.T) {
	e := combatEngine(t)
	mamba := onBoardCard(t, e, 0, corpusInfectCard(t, "Blight Mamba"))
	e.G.Obj(mamba).SummonSick = false
	guard := onBoard(t, e, 1, "Name:Guard\nManaCost:1 W\nTypes:Creature Soldier\nPT:2/2\nOracle:x\n")

	// Preconditions the assertions below depend on: the printed infect
	// keyword is readable before the step, and the blocker arrives
	// undamaged, uncountered and tough enough that the one counter does not
	// kill it (so the assertions read a live battlefield object).
	if !e.HasKeyword(mamba, "Infect") {
		t.Fatal("precondition: Blight Mamba does not read infect before the damage step")
	}
	if e.G.Obj(mamba).Face().PT != "1/1" {
		t.Fatalf("precondition: Blight Mamba PT = %s, want 1/1", e.G.Obj(mamba).Face().PT)
	}
	if got := e.G.Obj(guard).Counter("M1M1"); got != 0 {
		t.Fatalf("precondition: blocker already carries %d -1/-1 counters", got)
	}
	if got := e.G.Obj(guard).Damage; got != 0 {
		t.Fatalf("precondition: blocker already carries %d marked damage", got)
	}

	e.askAttackers()
	submitAttackers(t, e, mamba)
	submitBlockers(t, e, guard)

	if got := e.G.Obj(guard).Counter("M1M1"); got != 1 {
		t.Fatalf("blocker -1/-1 counters = %d, want 1 (the mamba's power, dealt in counter form)", got)
	}
	if got := e.G.Obj(guard).Damage; got != 0 {
		t.Fatalf("blocker marked damage = %d, want 0 (infect damage is never marked)", got)
	}
	if o := e.G.Obj(guard); o.Zone != state.ZBattlefield {
		t.Fatalf("2/2 blocker with one -1/-1 counter is in %v, want battlefield (toughness 1 survives)", o.Zone)
	}
	if got := e.G.Obj(mamba).Counter("M1M1"); got != 0 {
		t.Fatalf("attacker -1/-1 counters = %d, want 0 (the blocker has no infect, its hit back is marked damage)", got)
	}
	if o := e.G.Obj(mamba); o.Zone == state.ZBattlefield && o.Damage != 1 {
		t.Fatalf("attacker marked damage = %d, want 1 (the blocker's ordinary hit back)", o.Damage)
	}
}

// TestInfectDamageToPlayerIsPoisonAndTenPoisonLoses pins CR 702.90b's
// player half and CR 704.5b's end of the same line: an unblocked Blightsteel
// Colossus deals its 11 power to the defending player as 11 poison counters
// (life untouched), and the tenth poison counter loses the game.
func TestInfectDamageToPlayerIsPoisonAndTenPoisonLoses(t *testing.T) {
	e := combatEngine(t)
	colo := onBoardCard(t, e, 0, corpusInfectCard(t, "Blightsteel Colossus"))
	e.G.Obj(colo).SummonSick = false

	// Preconditions: the keywords are readable and the defender is alive,
	// unpoisoned and at the engine's starting life.
	if !e.HasKeyword(colo, "Infect") {
		t.Fatal("precondition: Blightsteel Colossus does not read infect")
	}
	if got := e.G.Players[1].Counter("POISON"); got != 0 {
		t.Fatalf("precondition: defender already carries %d poison", got)
	}
	if lost := e.G.Players[1].Lost; lost {
		t.Fatal("precondition: defender already lost")
	}

	e.askAttackers()
	submitAttackers(t, e, colo)

	if got := e.G.Players[1].Counter("POISON"); got != 11 {
		t.Fatalf("defender poison = %d, want 11 (the colossus's power, dealt in poison form)", got)
	}
	if got := e.G.Players[1].Life; got != 20 {
		t.Fatalf("defender life = %d, want 20 (infect damage to a player is never life loss)", got)
	}
	if !e.G.Players[1].Lost {
		t.Fatal("defender with 11 poison counters did not lose (CR 704.5b)")
	}
}

// TestGraftedExoskeletonGrantedInfectDealsInCounterForm pins the
// static-grant path: the equipment's `AddKeyword$ Infect` makes the equipped
// creature read infect (so its combat damage converts), here as poison on
// the defending player for its boosted 4 power.
func TestGraftedExoskeletonGrantedInfectDealsInCounterForm(t *testing.T) {
	e := combatEngine(t)
	exo := onBoardCard(t, e, 0, corpusInfectCard(t, "Grafted Exoskeleton"))
	bearer := onBoard(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.G.Obj(bearer).SummonSick = false

	// Attach through the ordinary event, then prove the grant path before
	// the step: the equipped creature must read infect from the static's
	// AddKeyword$, and the +2/+2 rider must have raised its power to 4 --
	// the number the poison count below is asserted against.
	e.emit(events.Event{Kind: events.Attach, Obj: exo, IDs: []state.ObjID{bearer}})
	if !e.HasKeyword(bearer, "Infect") {
		t.Fatal("precondition: the equipped creature does not read granted infect")
	}
	if got := e.Power(bearer); got != 4 {
		t.Fatalf("precondition: equipped bearer power = %d, want 4 (the static's +2/+2)", got)
	}
	if got := e.G.Players[1].Counter("POISON"); got != 0 {
		t.Fatalf("precondition: defender already carries %d poison", got)
	}

	e.askAttackers()
	submitAttackers(t, e, bearer)

	if got := e.G.Players[1].Counter("POISON"); got != 4 {
		t.Fatalf("defender poison = %d, want 4 (the granted-infect bearer's boosted power)", got)
	}
	if got := e.G.Players[1].Life; got != 20 {
		t.Fatalf("defender life = %d, want 20 (granted infect converts the same way)", got)
	}
}

// TestInfectDamageToNonCreatureArtifactIsMarked pins the recipient-form
// split the r2 review demanded (CR 702.90b rewrites creature and player
// damage ONLY): an infect source's damage to a plain artifact -- a legal
// DealDamage target that is neither creature nor player -- is ordinary
// marked damage, with NO -1/-1 counters and NO -1/-1 CounterChange event in
// the log.
func TestInfectDamageToNonCreatureArtifactIsMarked(t *testing.T) {
	e := combatEngine(t)
	mamba := onBoardCard(t, e, 0, corpusInfectCard(t, "Blight Mamba"))
	artifact := onBoard(t, e, 1, "Name:Brass Sprocket\nManaCost:2\nTypes:Artifact\nOracle:x\n")

	// Preconditions the assertions below depend on: the source reads
	// infect, the artifact is on the battlefield, reads as a NON-creature,
	// and arrives undamaged and uncountered.
	if !e.HasKeyword(mamba, "Infect") {
		t.Fatal("precondition: Blight Mamba does not read infect")
	}
	if o := e.G.Obj(artifact); o == nil || o.Zone != state.ZBattlefield {
		t.Fatal("precondition: the artifact is not on the battlefield")
	}
	if e.IsCreature(artifact) {
		t.Fatal("precondition: the artifact reads as a creature")
	}

	effects.Resolve(e, &effects.Ctx{Source: mamba, Controller: 0,
		Targets: []state.Target{{Obj: artifact}}}, &cards.SA{Kind: "DB", API: "DealDamage",
		Params: map[string]string{"Defined": "Targeted", "NumDmg": "3"}})

	if got := e.G.Obj(artifact).Damage; got != 3 {
		t.Fatalf("artifact marked damage = %d, want 3 (a non-creature recipient takes infect damage normally)", got)
	}
	if got := e.G.Obj(artifact).Counter("M1M1"); got != 0 {
		t.Fatalf("artifact -1/-1 counters = %d, want 0 (CR 702.90b rewrites creature damage only)", got)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.CounterChange && ev.Obj == artifact && ev.Counter == "M1M1" {
			t.Fatalf("the log carries a -1/-1 CounterChange for the artifact (%+v): the fold must not convert a non-creature recipient", ev)
		}
	}
}

// TestInfectDamageToPlainPlaneswalkerOnlyRemovesLoyalty pins the same
// recipient-form split for a printed planeswalker: CR 306.8's loyalty
// exchange applies and nothing else -- no marked damage, no -1/-1 counters
// (the r1 defect put counters on the walker IN ADDITION to the loyalty).
func TestInfectDamageToPlainPlaneswalkerOnlyRemovesLoyalty(t *testing.T) {
	e := combatEngine(t)
	mamba := onBoardCard(t, e, 0, corpusInfectCard(t, "Blight Mamba"))
	walker := onBoard(t, e, 1, "Name:Plain Walker\nTypes:Planeswalker Jace\nLoyalty:5\nOracle:x\n")
	// onBoard's placement is eventless, so the printed starting loyalty is
	// not granted by it; place it through the ordinary counter event.
	e.emit(events.Event{Kind: events.CounterChange, Obj: walker, Counter: "LOYALTY", Amount: 5})

	// Preconditions: the source reads infect; the walker is on the
	// battlefield with 5 loyalty, reads as a NON-creature, and arrives
	// uncountered and undamaged.
	if !e.HasKeyword(mamba, "Infect") {
		t.Fatal("precondition: Blight Mamba does not read infect")
	}
	if o := e.G.Obj(walker); o == nil || o.Zone != state.ZBattlefield {
		t.Fatal("precondition: the walker is not on the battlefield")
	}
	if e.IsCreature(walker) {
		t.Fatal("precondition: the walker reads as a creature")
	}
	if got := e.G.Obj(walker).Counter("LOYALTY"); got != 5 {
		t.Fatalf("precondition: walker loyalty = %d, want 5", got)
	}

	effects.Resolve(e, &effects.Ctx{Source: mamba, Controller: 0,
		Targets: []state.Target{{Obj: walker}}}, &cards.SA{Kind: "DB", API: "DealDamage",
		Params: map[string]string{"Defined": "Targeted", "NumDmg": "3"}})

	if got := e.G.Obj(walker).Counter("LOYALTY"); got != 2 {
		t.Fatalf("walker loyalty = %d, want 2 (CR 306.8's loyalty exchange is the only form)", got)
	}
	if got := e.G.Obj(walker).Damage; got != 0 {
		t.Fatalf("walker marked damage = %d, want 0", got)
	}
	if got := e.G.Obj(walker).Counter("M1M1"); got != 0 {
		t.Fatalf("walker -1/-1 counters = %d, want 0 (CR 702.90b does not rewrite non-creature permanent damage)", got)
	}
}

// TestInfectCountersRideTheCounterReplacementPath pins the r2 review's
// second MAJOR: the conversion emits REAL CounterChange/PlayerCounterChange
// events, so the repl:AddCounter class replaces them -- Winding
// Constrictor's "one more" lines double both the -1/-1 counters an opposing
// infect creature puts on your blocker and the poison counters one puts on
// you, exactly as it doubles any other placement.
func TestInfectCountersRideTheCounterReplacementPath(t *testing.T) {
	constrictor := corpusInfectCard(t, "Winding Constrictor")

	// Creature half: the constrictor's controller blocks an opposing infect
	// creature, and its `ValidCard$ Creature.YouCtrl` line doubles the
	// -1/-1 counters the hit converts to.
	e := combatEngine(t)
	onBoardCard(t, e, 1, constrictor)
	mamba := onBoardCard(t, e, 0, corpusInfectCard(t, "Blight Mamba"))
	e.G.Obj(mamba).SummonSick = false
	// A 4/4 blocker so the DOUBLED pair of -1/-1 counters leaves it alive --
	// a 2/2 would die at toughness 0 and its counters would leave with it.
	blocker := onBoard(t, e, 1, "Name:Bear\nManaCost:3 G\nTypes:Creature Bear\nPT:4/4\nOracle:x\n")
	if got := e.G.Obj(blocker).Counter("M1M1"); got != 0 {
		t.Fatalf("precondition: blocker already carries %d -1/-1 counters", got)
	}
	e.askAttackers()
	submitAttackers(t, e, mamba)
	submitBlockers(t, e, blocker)
	if got := e.G.Obj(blocker).Counter("M1M1"); got != 2 {
		t.Fatalf("blocker -1/-1 counters = %d, want 2 (the mamba's 1, doubled by the constrictor's AddCounter replacement)", got)
	}

	// Player half: the same constrictor's controller is hit unblocked by an
	// opposing infect creature, and its `ValidPlayer$ You` line doubles the
	// poison the hit converts to.
	e2 := combatEngine(t)
	onBoardCard(t, e2, 1, constrictor)
	mamba2 := onBoardCard(t, e2, 0, corpusInfectCard(t, "Blight Mamba"))
	e2.G.Obj(mamba2).SummonSick = false
	if got := e2.G.Players[1].Counter("POISON"); got != 0 {
		t.Fatalf("precondition: defender already carries %d poison", got)
	}
	e2.askAttackers()
	submitAttackers(t, e2, mamba2)
	// The defender controls a creature (the constrictor), so the combat
	// flow waits for its (empty) block declaration before the damage step.
	submitBlockers(t, e2)
	if got := e2.G.Players[1].Counter("POISON"); got != 2 {
		t.Fatalf("defender poison = %d, want 2 (the mamba's 1, doubled by the constrictor's AddCounter replacement)", got)
	}
	if got := e2.G.Players[1].Life; got != 20 {
		t.Fatalf("defender life = %d, want 20", got)
	}
	if e2.G.Players[1].Lost {
		t.Fatal("defender lost with only 2 poison counters")
	}
}

// TestInfectDeathtouchKillsThroughCounters pins the counter-form
// half of CR 704.5g: a deathtouch infect creature's damage is dealt as
// -1/-1 counters with nothing marked, and the creature it damaged still
// dies -- the Deathtouched mark is lethal on its own, without marked
// damage, exactly as it is beside it.
func TestInfectDeathtouchKillsThroughCounters(t *testing.T) {
	e := combatEngine(t)
	stinger := onBoard(t, e, 0, "Name:Toxic Stinger\nManaCost:B\nTypes:Creature Insect\nPT:1/1\nK:Deathtouch\nK:Infect\nOracle:x\n")
	e.G.Obj(stinger).SummonSick = false
	blocker := onBoard(t, e, 1, "Name:Guard\nManaCost:1 W\nTypes:Creature Soldier\nPT:2/2\nOracle:x\n")

	// Preconditions: the stinger reads both keywords, the blocker is on the
	// battlefield, undamaged, uncountered, and tall enough that the one
	// -1/-1 counter ALONE would not kill it (toughness 1 after it) -- so
	// only the deathtouch half can be why it dies.
	if !e.HasKeyword(stinger, "Infect") || !e.HasKeyword(stinger, "Deathtouch") {
		t.Fatal("precondition: the stinger does not read deathtouch + infect")
	}
	if o := e.G.Obj(blocker); o.Zone != state.ZBattlefield || o.Damage != 0 || o.Counter("M1M1") != 0 {
		t.Fatal("precondition: the blocker does not arrive alive, undamaged and uncountered")
	}
	if got := e.Toughness(blocker); got != 2 {
		t.Fatalf("precondition: blocker toughness = %d, want 2 (an undamaged 2/2)", got)
	}

	e.askAttackers()
	submitAttackers(t, e, stinger)
	submitBlockers(t, e, blocker)

	// The blocker dies, so its counters leave with it (a zone change clears
	// them) -- the placement is read off the log instead: exactly one
	// -1/-1 CounterChange, and the damage event it rode is
	// infect-marked (so nothing was ever marked on it).
	counters := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.CounterChange && ev.Obj == blocker && ev.Counter == "M1M1" {
			if ev.Amount != 1 {
				t.Fatalf("the blocker's -1/-1 placement = %+v, want exactly 1 (the stinger's power, in counter form)", ev)
			}
			counters++
		}
	}
	if counters != 1 {
		t.Fatalf("the log carries %d -1/-1 placements for the blocker, want 1 (the infect form)", counters)
	}
	if o := e.G.Obj(blocker); o.Zone == state.ZBattlefield {
		t.Fatal("a 2/2 that took 1 deathtouch infect damage (one -1/-1 counter, nothing marked) survived -- CR 704.5g's mark-alone lethality did not fire")
	}
}
