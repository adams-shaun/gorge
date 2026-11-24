package kgen

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"strings"
)

// effect dispatches on the API.
func (lc *lowerCtx) effect(sa *cards.SA, ps *params) error {
	// An AB$ inside a trigger body (an optional cost paid on resolution).
	if lc.ab.Kind == akTriggered || lc.ab.Kind == akLatent {
		if c, ok := ps.get("Cost"); ok && sa.Kind == "AB" && sa != lc.probeHead {
			if strings.HasSuffix(c, `\NumTimes>`) && strings.HasPrefix(c, "Mana<") {
				return lc.numTimesPay(sa, ps, c) // lower_numtimes.go
			}
			// A printed trigger's own optional cost reads the unless-cost
			// selection methods (gorge's triggered-cost window pays a Hand
			// discard with the whole hand whatever the count; CR 118.3,
			// 603.4). A latent body, an activated cost and a spell cost keep
			// refusing them.
			parse := lc.cost
			if lc.ab.Kind == akTriggered && len(lc.ab.Prog) == 0 {
				parse = lc.costUnless
				lc.trigBodyCost = true
			}
			cost, err := parse(c)
			lc.trigBodyCost = false
			if err != nil {
				return err
			}
			lc.trigOwnCost = lc.ab.Kind == akTriggered && len(lc.ab.Prog) == 0
			if lc.trigOwnCost && cost.Mana.X && !cost.Mana.None {
				// a printed trigger's own "you may pay {X}": gorge announces X
				// first, capped by the body's XMax$ (lower_triggerxcost.go)
				if err := lc.trigXCost(ps); err != nil {
					return err
				}
			}
			var chain *lInstr
			if lc.trigOwnCost && cost.ExileG != nil && !cost.Mana.None {
				if chain, cost, err = lc.exileChainSplit(cost); err != nil { // lower_exilechain.go
					lc.trigOwnCost = false
					return err
				}
			}
			ownCost := lc.trigOwnCost
			in, err := lc.payInstr(cost, lRef{Kind: rYou}, lc.trigOwnCost)
			lc.trigOwnCost = false
			lc.trigXMax = nil
			if err != nil {
				return err
			}
			if chain != nil && len(in.Hyb) != 0 {
				return refuse("resolution mana-and-exile cost with hybrid mana %s", cost.Mana) // the chain pays plain pips
			}
			if ownCost && in.CS == "SacrificeSelf" && strings.HasPrefix(c, "Mandatory") {
				in.CS = "SacrificeSelfMust" // no "may": gorge settles it unasked (cost_mandatorysac.go)
			}
			if _, ok := ps.m["UnlessResolveSubs"]; ok {
				return refuse("UnlessResolveSubs$ with a trigger's own cost")
			}
			in.Op = "IfPaid"
			at := len(lc.ab.Prog)
			lc.emit(in)
			if chain != nil {
				lc.emit(*chain)
			}
			if err := lc.unless(ps, func() error { return lc.api(sa, ps) }); err != nil {
				return err
			}
			// "If you do" covers the rest of the chain too.
			if !strings.HasPrefix(lc.ab.Prog[at].Note, noteRememberSacrificed) {
				// a sacrifice a value head reads keeps its remember note
				// the skip is patched either way
				lc.ab.Prog[at].Note = "covers the Sub chain"
			}
			lc.ab.Prog[at].Skip = -1 // patched by the trigger lowering to the chain end
			return nil
		}
	}
	if sa.API == "Ward" {
		// Ward's UnlessCost is the ward cost itself, not an unless wrapper.
		return lc.api(sa, ps)
	}
	// A Sacrifice's UnlessCost$ Discard<N/Card> lowers like any other unless
	// cost: gorge's shared unless gate prices the discard and pays it
	// (Masticore, The Great Juggernaut, Bebop & Rocksteady). A
	// filtered discard spec still refuses in the cost parser.
	return lc.unless(ps, func() error { return lc.api(sa, ps) })
}

// api lowers one effect by its api: name through the handler its family file
// registered (lower_api.go, api_<family>.go); an api: with no handler is
// refused.
func (lc *lowerCtx) api(sa *cards.SA, ps *params) error {
	ps.consume("Planeswalker")
	h := apiOps[sa.API]
	if h == nil {
		return refuseAPI(sa.API) // lower_refuseapi.go
	}
	return h(lc, sa, ps)
}

// amount reads an amount param (default dflt when absent).
func (lc *lowerCtx) amount(ps *params, key string, dflt int) (lValue, error) {
	v, ok := ps.get(key)
	if !ok {
		return cst(dflt), nil
	}
	return lc.value(v)
}
