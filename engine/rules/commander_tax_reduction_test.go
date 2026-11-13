package rules

import (
	"strings"
	"testing"
)

// The CR 903.8 commander tax is an additional cost, so CR 601.2f counts it in
// the total cost the reductions and floors compose over. These pin the two
// places where composing it after the modifiers instead would charge more.

// commanderTaxGhaltaSetup is Ghalta, Primal Hunger ({10}{G}{G}, {X} less where
// X is the total power of creatures you control) in p0's command zone, cast
// once before, beside two 6/6 creatures: total power 12. The total cost is
// {10}{G}{G} plus the {2} tax, {12}{G}{G}, less {12}: {G}{G}.
const commanderTaxGhaltaSetup = `{"name":"commander-tax-ghalta","format":"commander","cr":["601.2f","903.8"],"why":"a generic reduction larger than the printed generic also takes off the commander tax",
"setup":{"p0":{"command":["Ghalta, Primal Hunger"],"commander_casts":{"Ghalta, Primal Hunger":1},
               "battlefield":["Colossal Dreadmaw","Colossal Dreadmaw"]},
         "p1":{}}`

// TestCommanderTaxIsReducedWithTheTotalCost: Ghalta with tax {2} at total
// power 12 is paid by {G}{G}, and {G} alone is still short.
func TestCommanderTaxIsReducedWithTheTotalCost(t *testing.T) {
	short := commanderTaxGhaltaSetup + `,"steps":[{"op":"cast","seat":0,"card":"p0:Ghalta, Primal Hunger","mana":"G"}]}`
	run, fails := runOracleCommander(t, short)
	if len(fails) == 0 {
		t.Fatalf("a {G} cast paid {G}{G}: %s", strings.Join(run.log, "\n"))
	}

	paid := commanderTaxGhaltaSetup + `,"steps":[{"op":"cast","seat":0,"card":"p0:Ghalta, Primal Hunger","mana":"GG"}]}`
	run, fails = runOracleCommander(t, paid)
	if len(fails) != 0 {
		t.Fatalf("a {G}{G} cast was refused (the tax was charged after the reduction): %v\n%s", fails, strings.Join(run.log, "\n"))
	}
	last := run.snaps[len(run.snaps)-1]
	if len(last.Stack) != 1 || last.Stack[0].Source != "p0:Ghalta, Primal Hunger" {
		t.Fatalf("stack = %+v, want Ghalta on the stack", last.Stack)
	}
}

// commanderTaxFloorSetup is Isamaru, Hound of Konda ({W}) in p0's command
// zone, cast once before, under an untapped Trinisphere: the total cost
// {2}{W} is already three mana, so the floor adds nothing.
const commanderTaxFloorSetup = `{"name":"commander-tax-floor","format":"commander","cr":["601.2f","903.8"],"why":"a SetCost floor measures a total that already carries the commander tax",
"setup":{"p0":{"command":["Isamaru, Hound of Konda"],"commander_casts":{"Isamaru, Hound of Konda":1},
               "battlefield":["Trinisphere"]},
         "p1":{}}`

// TestCommanderTaxCountsTowardTheCostFloor: Isamaru with tax {2} under
// Trinisphere costs {2}{W}, not the floor's {2}{W} plus the tax.
func TestCommanderTaxCountsTowardTheCostFloor(t *testing.T) {
	short := commanderTaxFloorSetup + `,"steps":[{"op":"cast","seat":0,"card":"p0:Isamaru, Hound of Konda","mana":"WC"}]}`
	run, fails := runOracleCommander(t, short)
	if len(fails) == 0 {
		t.Fatalf("a {W}{C} cast paid {2}{W}: %s", strings.Join(run.log, "\n"))
	}

	paid := commanderTaxFloorSetup + `,"steps":[{"op":"cast","seat":0,"card":"p0:Isamaru, Hound of Konda","mana":"WCC"}]}`
	run, fails = runOracleCommander(t, paid)
	if len(fails) != 0 {
		t.Fatalf("a {W}{C}{C} cast was refused (the floor was measured before the tax): %v\n%s", fails, strings.Join(run.log, "\n"))
	}
	last := run.snaps[len(run.snaps)-1]
	if len(last.Stack) != 1 || last.Stack[0].Source != "p0:Isamaru, Hound of Konda" {
		t.Fatalf("stack = %+v, want Isamaru on the stack", last.Stack)
	}
}
