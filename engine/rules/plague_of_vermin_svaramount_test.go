package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// ratsOf counts the Rat tokens seat p controls.
func ratsOf(e *Engine, p state.PlayerID) int {
	n := 0
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Rat Token" {
			n++
		}
	}
	return n
}

// TestPlagueOfVerminSVarAmountReadsStoredLife: Plague of Vermin's
// LifePaidAmount (PlayerCountRemembered$SVarAmount) reads the life each
// player paid across the repeat rounds, so each seat mints one Rat per life
// it paid. Round one: seat 0 pays 3, seat 1 pays 2 (real life lost, CR
// 119.4); round two both pay nothing and the repeat ends. The head must
// return each seat's OWN total (3 and 2), not the sum nor a flat zero.
func TestPlagueOfVerminSVarAmountReadsStoredLife(t *testing.T) {
	t.Parallel()
	e := handEngineTokens(t, corpusCard(t, "Plague of Vermin"))
	id := e.G.Zone(state.ZHand, 0)[0]
	if body := e.G.Obj(id).Face().SVars["LifePaidAmount"]; body != "PlayerCountRemembered$SVarAmount" {
		t.Fatalf("precondition: LifePaidAmount = %q", body)
	}
	if r0, r1 := ratsOf(e, 0), ratsOf(e, 1); r0 != 0 || r1 != 0 {
		t.Fatalf("precondition: rats before the cast = %d, %d", r0, r1)
	}
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MB] = 6, 1
	e.priorityRound()
	d := castSpellObj(t, e, id)
	round := [2]int{3, 2}
	numberAsks := 0
	for i := 0; i < 40 && d != nil; i++ {
		pick := 0 // an unless_pay ask: index 0 is "Pay"
		if d.Kind == decision.KChoose && d.ResumeKind == "choosenumber" {
			want := 0
			if numberAsks < 2 {
				want = round[numberAsks]
			}
			numberAsks++
			pick = -1
			for _, o := range d.Options {
				if o.Kind == "number" && o.Amount == want {
					pick = o.Index
				}
			}
			if pick < 0 {
				t.Fatalf("no %d option in %+v", want, d.Options)
			}
		}
		d = pickChoices(t, e, pick)
	}
	if d != nil {
		t.Fatalf("the resolution did not finish: %+v", d)
	}
	if numberAsks != 4 {
		t.Fatalf("number asks = %d, want 4 (two rounds, two seats)", numberAsks)
	}
	if l0, l1 := e.G.Players[0].Life, e.G.Players[1].Life; l0 != 17 || l1 != 18 {
		t.Fatalf("life after paying = %d, %d, want 17, 18", l0, l1)
	}
	if r0, r1 := ratsOf(e, 0), ratsOf(e, 1); r0 != 3 || r1 != 2 {
		t.Fatalf("rats = %d, %d, want 3, 2 (one per life each seat paid)", r0, r1)
	}
}
