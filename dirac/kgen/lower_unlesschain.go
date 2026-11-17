package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// unlessPartChain: an UnlessCost$ of several parts in ONE election.
//
// The kernel's unless instruction pays one kind, but gorge poses a multi-part
// UnlessCost$ as a single pay / decline ask for the whole cost
// (pay.BeginUnless / AdvanceUnless, engine/rules/pay/unlessflow.go), so a
// chain of per-part UnlessPaid elections would pose one ask per part where
// gorge poses one, and would price each part alone where gorge prices them
// together (unless.go unlessComponentsPayable). The chain is therefore ONE
// instruction: the Sac<N/Spec> part is the head (PaySac, A = N, F = Spec) and
// the PayLife<L> / Draw<D/Who> parts ride its B.N (kop.PaySacRidersB), the
// draw's recipient on W. Only this family is lowered: the Sac part is the one
// pick-bearing part, so no two parts compete for a candidate pool (gorge's
// bipartite reservation, unlessChoiceComponentsPayable, has nothing to
// reserve), and life and a draw are synchronous.
//
// The chain reproduces gorge's order, part by part:
//
//	ask        one pay / decline election, offered only while EVERY part is
//	           payable: N sacrifice candidates, life >= L, and D > 0 cards
//	           (the kernel's canPayGen over the packed riders)
//	picks      the Sac part's N picks, nothing charged yet (AdvanceUnless's
//	           pick walk: askUnlessPicks is ONE KChoose of Min=Max=N selects
//	           where the kernel asks one card per answer; the lockstep
//	           adapter, adapter_side.go, folds the picks into gorge's one
//	           answer. An exactly-counted candidate set is recorded unasked)
//	charge     life first (PayManaConv's LifeChange), then the sacrifices as
//	           one batch (PaySacrificeCost), then the draws, drawer by drawer
//	           (the engine's settleUnlessStep draws after the payment)
//	declined   the body runs, as with any UnlessPaid
//
// dirac/cost_unlesssacriders.go runs it: the life before payCommitSacs (and
// before autoPayPick's own commit), the draw in payDone's paid branch.
// Anything else multi-part keeps payInstr's one-part shape refusal.
func (lc *lowerCtx) unlessPartChain(c lCost, payer lRef) (in lInstr, ok bool, err error) {
	if !lc.unlessCost || c.Sac == nil || c.SacN < 1 || c.PayLife == 0 && c.DrawN == 0 {
		return in, false, nil
	}
	rest := c
	rest.Sac, rest.SacN, rest.PayLife, rest.DrawN, rest.DrawWho = nil, 0, 0, 0, ""
	if !rest.Mana.None || !rest.pureMana() {
		return in, false, nil // another part rides the cost: payInstr's shape check refuses it
	}
	b, packed := kop.PaySacRidersB(c.PayLife, c.DrawN)
	if !packed {
		return in, false, refuse("unless cost Sac with life %d and draw %d", c.PayLife, c.DrawN)
	}
	in = lInstr{Obj: payer, CS: "SacrificeRiders", A: cst(c.SacN), B: cst(b), F: c.Sac}
	if c.DrawN > 0 {
		if in.W, err = lc.drawRecipient(c.DrawWho); err != nil {
			return in, false, err
		}
	}
	return in, true, nil
}

// unlessPayInstr is payInstr for an UnlessCost$: the multi-part chain first,
// every other cost the one-part instruction.
func (lc *lowerCtx) unlessPayInstr(c lCost, payer lRef, subCtrOK bool) (lInstr, error) {
	if in, ok, err := lc.unlessPartChain(c, payer); ok || err != nil {
		return in, err
	}
	return lc.payInstr(c, payer, subCtrOK)
}
