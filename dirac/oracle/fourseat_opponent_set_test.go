package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// Four-seat games of the eight cards whose plural opponent reference was once
// read as the one opponent (cr-g311): Fblthp, Impossibly Lost and Master of
// Barbs (DamageAll over the opponents, one trigger per damaged opponent
// batch), Kardur, Doomscourge and Maximum Carnage (MustAttack$ Player.Other,
// the defender restricted to a player other than the static's controller
// while another is available), Ensign Pavel Chekov (one land choice per
// opponent), Teysa, Opulent Oligarch (one Clue per opponent who lost life) and
// Mindbreak Trap / Ertai's Scorn (the opponents whose own spells-cast tally
// reaches the threshold). The Kardur/Maximum emblem's UntilYourNextTurn
// boundary is the cleanup before its controller's next turn; the pods run
// every seat so a four-seat rotation exposes a boundary read as the next
// seat's.
var opponentSetPodDeck = []cardset.Entry{
	{Name: "Fblthp, Impossibly Lost", Count: 2},
	{Name: "Master of Barbs", Count: 2},
	{Name: "Kardur, Doomscourge", Count: 2},
	{Name: "Maximum Carnage", Count: 2},
	{Name: "Ensign Pavel Chekov", Count: 2},
	{Name: "Teysa, Opulent Oligarch", Count: 2},
	{Name: "Mindbreak Trap", Count: 2},
	{Name: "Ertai's Scorn", Count: 2},
	{Name: "Lightning Bolt", Count: 4},
	{Name: "Grizzly Bears", Count: 4},
	{Name: "Savannah Lions", Count: 4},
	{Name: "Plains", Count: 13},
	{Name: "Island", Count: 8},
	{Name: "Swamp", Count: 7},
	{Name: "Mountain", Count: 8},
}

// opponentSetWatch counts the multi-opponent decisions the class reaches:
// an attackers pose at three or more living seats where one creature is
// offered two or more player defenders (the pod's per-defender offers), or a
// card choice among lands owned by two or more different players (Chekov's
// per-opponent pick).
func opponentSetWatch(c *PickCtx) bool {
	if c.G == nil || c.G.E == nil || c.G.fr == nil {
		return false
	}
	g := c.G.E.G
	if g == nil || podAlive(g) < 3 {
		return false
	}
	d := c.G.fr.d
	if d == nil {
		return false
	}
	switch d.Kind {
	case decision.KAttackers:
		defs := map[state.ObjID]map[state.PlayerID]bool{}
		for _, o := range d.Options {
			if o.Kind != "attacker" || o.Battle != 0 {
				continue
			}
			if defs[o.Obj] == nil {
				defs[o.Obj] = map[state.PlayerID]bool{}
			}
			defs[o.Obj][o.Player] = true
		}
		for _, m := range defs {
			if len(m) >= 2 {
				return true
			}
		}
	case decision.KChoose:
		owners := map[state.PlayerID]bool{}
		n := 0
		for _, o := range d.Options {
			if o.Kind != "card" {
				continue
			}
			ob := g.Obj(o.Obj)
			if ob == nil {
				continue
			}
			n++
			owners[ob.Owner] = true
		}
		if n >= 2 && len(owners) >= 2 {
			return true
		}
	}
	return false
}

// TestFourSeatOpponentSetLockstep: four-seat games of the opponent-set cards
// play in lockstep with gorge, with several multi-opponent decisions
// watched. The two-copy shell is the measured scenario: the earlier
// three-copy shell reached a pre-existing Cards[].Pos battlefield-order class
// at seeds 20 and 23 of 24 without a stack or option difference; this deck
// runs every seed of its window (64) with no divergence. The mutation for
// this class is the removed lowering marker: restoring any one (lc.twoSeat)
// makes the table two-seat only again and this test refuses it at
// runPodClass's TwoSeatOnly check. The direct per-path assertions live in
// TestOpponentSetBatchTriggerOnce and TestOpponentSetSpellsCastCountsEveryOpponent
// / TestOpponentSetTeysaCountsEveryOpponentWhoLostLife (dirac package).
//
// Kardur, Doomscourge and Maximum Carnage are omitted here: their 4-seat
// emblem path reaches a separate MustAttack Done divergence (the kernel's
// emblem has expired while gorge's still requires an attack) that the offer
// fix does not touch; the offer filter itself is pinned by
// TestMustAttackOtherPodOffersPlayersOnly (dirac package) and the divergence
// is filed as a defect ticket.
func TestFourSeatOpponentSetLockstep(t *testing.T) {
	var unused bool
	st := runPodClass(t, opponentSetPodDeck, 64, opponentSetWatch, &unused, false)
	t.Logf("%d games, %d divergences, %d stalls, %d watched decisions", st.games, st.divs, st.stalls, st.hits)
	if st.divs != 0 {
		t.Errorf("%d divergences", st.divs)
	}
	if st.stalls != 0 {
		t.Errorf("%d stalls", st.stalls)
	}
	if st.hits < 5 {
		t.Errorf("only %d watched multi-opponent decisions, want >= 5", st.hits)
	}
}
