package kgen

import "testing"

// cdaflashForms: the conditional "this spell has flash" characteristic-
// defining static (CR 604.3, 613.3) lowers to the carrier's own
// CastWithFlash static (CardDef.SelfFlash), whose gate rides Static.Cond2.
var cdaflashForms = []formCase{
	{card: "Colossal Rattlewurm", form: "CDA flash, IsPresent Desert.YouCtrl",
		lir: `static CastWithFlash F={types=0x0 Self} F2=nil cond=Present({types=0x0 sub=Desert ctrl=1},op1,1)`},
	{card: "Crashing Tide", form: "CDA flash, IsPresent Merfolk.YouCtrl",
		lir: `static CastWithFlash F={types=0x0 Self} F2=nil cond=Present({types=0x0 sub=Merfolk ctrl=1},op1,1)`},
	{card: "Supernatural Rescue", form: "CDA flash, IsPresent Spirit.YouCtrl",
		lir: `static CastWithFlash F={types=0x0 Self} F2=nil cond=Present({types=0x0 sub=Spirit ctrl=1},op1,1)`},
	{card: "Take for a Ride", form: "CDA flash, Count$CommittedCrimeThisTurn",
		lir: `static CastWithFlash F={types=0x0 Self} F2=nil cond=Value(op1,CommittedCrime(1,0),1)`},
	{card: "Graveyard Shift", form: "CDA flash, Count$ValidGraveyard Card.YouOwn$DifferentCardManaCost",
		lir: `static CastWithFlash F={types=0x0 Self} F2=nil cond=Value(op1,GraveAgg({types=0x0 owner=1}),5)`},
	{card: "Mutual Destruction", form: "gate reads a battlefield keyword", refused: `CDA flash IsPresent "Permanent.YouCtrl+hasKeywordFlash"`},
}

func TestCDAFlashForms(t *testing.T) { checkForms(t, cdaflashForms) }
