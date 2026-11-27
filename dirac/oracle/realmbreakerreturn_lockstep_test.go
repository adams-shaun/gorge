package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// realmbreakerReturnDecks: Realmbreaker, the Invasion Tree mills the opponent
// and puts a land card they own from their graveyard onto the battlefield
// tapped under its controller's control (ChangeType$ Land.TargetedPlayerCtrl,
// read as the targeted player's ownership off the battlefield; kgen
// lower_zone.go), animated with the leave-to-exile replacement. The opposing
// deck is land-heavy, so the mill reaches lands.
func realmbreakerReturnDecks() [2][]cardset.Entry {
	a := []cardset.Entry{
		{Name: "Realmbreaker, the Invasion Tree", Count: 10}, {Name: "Mind Stone", Count: 6},
		{Name: "Darksteel Relic", Count: 4}, {Name: "Island", Count: 10}, {Name: "Mountain", Count: 10},
	}
	b := []cardset.Entry{
		{Name: "Savannah Lions", Count: 6}, {Name: "Elite Vanguard", Count: 6},
		{Name: "Plains", Count: 14}, {Name: "Forest", Count: 14},
	}
	return [2][]cardset.Entry{a, b}
}

func TestRealmbreakerReturnPoolPlayable(t *testing.T) {
	t.Parallel()
	_, set, gd := lockstepPool(t, "realmbreakerreturn", realmbreakerReturnDecks())
	for s, d := range gd {
		if !SetDeckSupported(set, d) {
			t.Errorf("seat %d: deck not playable in the realmbreakerreturn pool", s)
		}
	}
}

func TestRealmbreakerReturnLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "realmbreakerreturn", realmbreakerReturnDecks(), 200, "REALMBREAKER_RETURN_SEEDS")
}

// TestRealmbreakerReturnMutation: with the pick's Tapped$ dropped (the
// returned land enters untapped) some seed diverges, so the pool reaches the
// graveyard return.
func TestRealmbreakerReturnMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "realmbreakerreturn", realmbreakerReturnDecks(), 200, false, func(tbl *kop.Table) {
		cs := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cs {
			if cs[i].Name != "Realmbreaker, the Invasion Tree" {
				continue
			}
			abs := append([]kop.Ability(nil), cs[i].Abilities...)
			for j := range abs {
				prog := append([]kop.Instr(nil), abs[j].Prog...)
				for x := range prog {
					if prog[x].Op == kop.OpChooseMove {
						prog[x].C &^= kop.SrTapped << 3 // the move flags ride C>>3 (kgen instr_search.go)
					}
				}
				abs[j].Prog = prog
			}
			cs[i].Abilities = abs
		}
		tbl.Cards = cs
	})
	if n == 0 {
		t.Error("no seed diverged with the return's Tapped$ dropped: the pool never reaches it")
	}
}
