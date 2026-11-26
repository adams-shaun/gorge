package kgen

import "testing"

// optAttackCostForms: S:Mode$ OptionalAttackCost (CR 702.100a "you may exert
// CARDNAME as it attacks. When you do, ...") lowers as the kernel's
// StatOptionalAttackCost offer plus the exert rider as a self-Exerted trigger.
var optAttackCostForms = []formCase{
	{card: "Khenra Scrapper", form: "the exert offer is one filterless static (its source is the carrier)", lir: "static OptionalAttackCost F=nil"},
	{card: "Khenra Scrapper", form: "the Trigger$ rider is a self Exerted trigger", lir: "trigger Exerted"},
	{card: "Khenra Scrapper", form: "the rider's body is +2/+0 over itself until end of turn", lir: "PumpUntilEOT obj=Self A=2 B=0"},
}

func TestOptionalAttackCostLowers(t *testing.T) { checkForms(t, optAttackCostForms) }
