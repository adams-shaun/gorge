package kgen

import "testing"

// costExiledMoveToGraveForms: the ExiledMoveToGrave<N/Spec> cost token
// (lower_exiledmovegrave.go). A printed trigger's own Cost$ with a
// source-free spec lowers to IfPaid CS=MoveGrave (gorge's triggered-cost
// window, kernel cost_exiledmovegrave.go); every other form gorge reads stays
// refused by name.
var costExiledMoveToGraveForms = []formCase{
	// a trigger's own cost, Card.OppOwn, one and two cards
	{card: "Mind Raker", form: "trigger own ExiledMoveToGrave<1/Card.OppOwn>", lir: "IfPaid obj=You A=1 CS=MoveGrave F={types=0x0 owner=2}"},
	{card: "Blight Herder", form: "cast trigger own ExiledMoveToGrave<2/Card.OppOwn>", lir: "IfPaid obj=You A=2 CS=MoveGrave F={types=0x0 owner=2}"},
	{card: "Ulamog's Nullifier", form: "trigger own ExiledMoveToGrave<2/Card.OppOwn> before a spell target", lir: "IfPaid obj=You A=2 CS=MoveGrave"},
	// an activated ability's own cost (gorge's cast-time moveGraveAsk)
	{card: "Cryptic Cruiser", form: "activated ExiledMoveToGrave<1/Card.OppOwn>", refused: `cost token "ExiledMoveToGrave<1/Card.OppOwn/card an opponent owns>" outside a trigger's own cost`},
	// a spell's additional cost
	{card: "Processor Assault", form: "spell ExiledMoveToGrave<1/Card.OppOwn>", refused: `cost token "ExiledMoveToGrave<1/Card.OppOwn/card an opponent owns>" outside a trigger's own cost`},
	// the source-relative exile link
	{card: "Foreboding Steamboat", form: "trigger own ExiledMoveToGrave<1/Card.ExiledWithSource>", refused: `cost token "ExiledMoveToGrave<1/Card.ExiledWithSource/card exiled with CARDNAME>": an ExiledWithSource spec`},
	{card: "Jester's Scepter", form: "activated ExiledMoveToGrave<1/Card.ExiledWithSource>", refused: `cost token "ExiledMoveToGrave<1/Card.ExiledWithSource/card exiled with CARDNAME>" outside a trigger's own cost`},
	{card: "Shelob, Dread Weaver", form: "activated ExiledMoveToGrave<1/Creature.ExiledWithSource>", refused: `cost token "ExiledMoveToGrave<1/Creature.ExiledWithSource>" outside a trigger's own cost`},
	// a static (as-enters) trigger's body cost
	{card: "Ulamog's Despoiler", form: "static trigger body ExiledMoveToGrave", refused: `static trigger body Cost$ "ExiledMoveToGrave<2/Card.OppOwn/card an opponent owns>"`},
}

func TestCostExiledMoveToGrave(t *testing.T) { checkForms(t, costExiledMoveToGraveForms) }
