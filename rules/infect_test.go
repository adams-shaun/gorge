package rules

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/cards"
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
