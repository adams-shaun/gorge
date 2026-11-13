package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// playerPumpGrantForms pins the answers to a Pump keyword grant to the
// resolving controller. With Duration$ UntilYourNextTurn (pumpPlayerGrant) a
// protection-from-everything grant lowers onto a next-turn emblem of the
// controller, which the kernel's plProtAll reads (static_plprot.go), and a
// hexproof grant lowers onto the same kind of emblem carrying a plain
// StatPlayerHexproof, which playerHexproof reads through embPlHex
// (static_emblemphex.go). Until end of turn (pumpPlayerEOT)
// hexproof is the TurnReplPlayerHex record those readers do see, and shroud,
// which no kernel player read carries, refuses: neither may lower to a
// keyword grant, which lands on no permanent and so does nothing.
var playerPumpGrantForms = []formCase{
	{
		card: "The Stasis Coffin",
		form: "Defined$ You KW$ Protection from everything Duration$ UntilYourNextTurn",
		lir:  "CreateEmblem obj=You",
	},
	{
		card: "Blossoming Calm",
		form: "Defined$ You KW$ Hexproof Duration$ UntilYourNextTurn",
		lir:  "CreateEmblem obj=You",
	},
	{
		card: "The Needs of the Many",
		form: "Defined$ You KW$ Hexproof",
		lir:  "TurnRepl C=5",
	},
	{
		card: "Earthshape",
		form: "Defined$ You KW$ Hexproof",
		lir:  "TurnRepl C=5",
	},
	{
		card:    "Gilded Light",
		form:    "Defined$ You KW$ Shroud",
		refused: `player-scoped Pump grant keyword "Shroud" until end of turn`,
	},
}

func TestPlayerPumpGrantForms(t *testing.T) {
	checkForms(t, playerPumpGrantForms)
}

// TestPlayerPumpGrantEmblemStatic: Blossoming Calm lowers and builds with no
// refusal to one next-turn emblem carrying one PlayerHexproof static with no
// Amount, built as kop.StatPlayerHexproof with A == 0 (plain hexproof, not
// protection from each opponent).
func TestPlayerPumpGrantEmblemStatic(t *testing.T) {
	lw := lowerNamedPool(t, []string{"Blossoming Calm"})
	all, _, pt, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	var calm *lCard
	for _, c := range all {
		if c.Name == "Blossoming Calm" {
			calm = c
		}
	}
	if calm == nil {
		t.Fatal("Blossoming Calm: not lowered")
	}
	if len(calm.Refusals) > 0 {
		t.Fatalf("Blossoming Calm: refused: %v", calm.Refusals)
	}
	if len(calm.Emblems) != 1 || !calm.Emblems[0].UntilNextTurn {
		t.Fatalf("Blossoming Calm emblems = %+v, want one UntilNextTurn emblem", calm.Emblems)
	}
	sts := calm.Emblems[0].Statics
	if len(sts) != 1 || sts[0].Kind != "PlayerHexproof" || sts[0].Amount.Kind != vNone {
		t.Fatalf("Blossoming Calm emblem statics = %+v, want one PlayerHexproof with no Amount", sts)
	}
	if len(pt.emblems) != 1 {
		t.Fatalf("pool emblems = %d, want 1", len(pt.emblems))
	}
	ke := pt.emblems[0]
	if !ke.UntilNextTurn || len(ke.Abilities) != 1 {
		t.Fatalf("built emblem = %+v, want UntilNextTurn and one ability", ke)
	}
	if a := ke.Abilities[0]; a.Kind != kop.AbStatic || a.Static.Kind != kop.StatPlayerHexproof || a.Static.A != 0 {
		t.Fatalf("built emblem ability = %+v, want a StatPlayerHexproof static with A == 0", a)
	}
}
