package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A printed trigger's own ExiledMoveToGrave<N/Card.OppOwn> cost (the Eldrazi
// processors; kgen lower_exiledmovegrave.go, kernel cost_exiledmovegrave.go):
// gorge's triggered-cost window asks pay/decline, then the "graveyard_cost"
// pick over every alive seat's exile zone (unasked with exactly N cards), and
// each chosen card goes to its owner's graveyard. Exile removal fills the
// opponents' exile zones. The expected answer at every decision is gorge's.
var exiledMoveGraveDecks = [2][]cardset.Entry{
	{ // A: W/B
		{Name: "Swords to Plowshares", Count: 4}, {Name: "Path to Exile", Count: 4}, {Name: "Castigate", Count: 3},
		{Name: "Journey to Nowhere", Count: 2}, {Name: "Mind Raker", Count: 3}, {Name: "Wasteland Strangler", Count: 4},
		{Name: "Blight Herder", Count: 3}, {Name: "Elite Vanguard", Count: 1},
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8},
	},
	{ // B: U/W
		{Name: "Swords to Plowshares", Count: 4}, {Name: "Path to Exile", Count: 3}, {Name: "Murk Strider", Count: 4},
		{Name: "Ulamog's Reclaimer", Count: 3}, {Name: "Ulamog's Nullifier", Count: 3}, {Name: "Blight Herder", Count: 2},
		{Name: "Ruin Processor", Count: 1}, {Name: "Cancel", Count: 2}, {Name: "Elite Vanguard", Count: 2},
		{Name: "Island", Count: 8}, {Name: "Plains", Count: 6}, {Name: "Swamp", Count: 2},
	},
}

// exiledMoveGraveDropSentinel clears the PayBSubMoveGrave sentinel off every
// pay instruction: the cost becomes a graveyard exile of the payer's own
// cards owned by an opponent, which nothing can pay.
func exiledMoveGraveDropSentinel(tbl *kop.Table) {
	cards := append([]kop.CardDef(nil), tbl.Cards...)
	for i := range cards {
		abs := append([]kop.Ability(nil), cards[i].Abilities...)
		for j := range abs {
			prog := append([]kop.Instr(nil), abs[j].Prog...)
			for k := range prog {
				if prog[k].Op == kop.OpIfPaid && int(prog[k].C&0xF) == kop.PayExileGrave && prog[k].B.N == kop.PayBSubMoveGrave {
					prog[k].B = kop.Const(0)
				}
			}
			abs[j].Prog = prog
		}
		cards[i].Abilities = abs
	}
	tbl.Cards = cards
}

func TestExiledMoveToGraveLockstep(t *testing.T) {
	t.Parallel()
	lockstepRun(t, "exiledmovegrave", exiledMoveGraveDecks, 60, true, nil)
}

// TestExiledMoveToGraveMutation: with the move-to-graveyard sentinel dropped
// some game diverges, so the pool reaches the cost.
func TestExiledMoveToGraveMutation(t *testing.T) {
	t.Parallel()
	if lockstepRun(t, "exiledmovegrave", exiledMoveGraveDecks, 60, false, exiledMoveGraveDropSentinel) == 0 {
		t.Fatal("dropping the PayBSubMoveGrave sentinel diverged in no game: the pool does not reach the cost")
	}
}

// exiledMoveGravePod4 are the four seats' decks: the two-seat decks twice.
func exiledMoveGravePod4() [4][]string {
	var d [2][]string
	for s, es := range exiledMoveGraveDecks {
		for _, e := range es {
			d[s] = append(d[s], deckOf(e.Count, e.Name)...)
		}
	}
	return [4][]string{d[0], d[1], d[0], d[1]}
}

func TestFourSeatExiledMoveToGraveLockstep(t *testing.T) {
	t.Parallel()
	if n := pod4FixtureLockstep(t, "exiledmovegrave", exiledMoveGravePod4(), 30, true, nil); n != 0 {
		t.Fatalf("%d diverged", n)
	}
}

func TestFourSeatExiledMoveToGraveMutation(t *testing.T) {
	t.Parallel()
	if pod4FixtureLockstep(t, "exiledmovegrave", exiledMoveGravePod4(), 30, false, exiledMoveGraveDropSentinel) == 0 {
		t.Fatal("dropping the PayBSubMoveGrave sentinel diverged in no four-seat game: the pool does not reach the cost")
	}
}
