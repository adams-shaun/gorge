package kgen

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// a may-play permission's non-mana RaiseCost$ (Noctis,
// Prince of Lucis PayLife<3>; Alien Symbiosis Discard<1/Card>; Wickerfolk
// Indomitable PayLife<2> Sac<1/Artifact;Creature>). gorge charges the raise
// on top of the cast's cost (rules/mayplay.go mayPlayRaiseCost, CR 601.2f/h).
// The cost rides as a latent ability of the static's source card (flag
// "MayRaise", lStatic.RaiseAb = its index + 1); the kernel reads it through
// Static.B (kernel cost_mayplayraise.go).

func init() { mask[kop.StaticFlags]("kop.StMayRaise", kop.StMayRaise) }

// mayRaiseStatic lowers rc, the only shapes read: PayLife<N>, Discard<1/Card>,
// PayLife<N> + Sac<1/filter>; no mana and no other part.
func (lc *lowerCtx) mayRaiseStatic(rc string, st *lStatic) error {
	c, err := lc.cost(rc)
	if err != nil {
		return err
	}
	m := c.Mana
	if m.Generic != 0 || m.Pips != ([5]int{}) || len(m.Hybrid) != 0 || len(m.Twobrid) != 0 || len(m.Phyrexian) != 0 ||
		m.X || m.XX || m.C != 0 { // (a cost with no mana symbol reads None)
		return refuse("RaiseCost %q (mana part)", rc)
	}
	rest := c
	rest.PayLife, rest.DiscardN, rest.Sac, rest.SacN = 0, 0, nil, 0
	if !rest.pureMana() {
		return refuse("RaiseCost %q (cost part)", rc)
	}
	switch {
	case c.PayLife >= 1 && c.PayLife <= 255 && c.DiscardN == 0 && c.Sac == nil && c.SacN == 0: // PayLife<N>
	case c.DiscardN == 1 && c.PayLife == 0 && c.Sac == nil && c.SacN == 0: // Discard<1/Card>
	case c.PayLife >= 1 && c.PayLife <= 255 && c.Sac != nil && c.SacN == 1 && c.DiscardN == 0: // PayLife<N> Sac<1/filter>
	default:
		return refuse("RaiseCost %q", rc)
	}
	lat := lc.newLatent("MayRaise")
	lat.Cost = c
	st.RaiseAb = lat.Index + 1
	st.Flags = append(st.Flags, "MayRaise")
	return nil
}

// raiseCost builds a MayRaise latent ability's Cost: PayLife<N> beside a
// sacrifice keeps Cost.N for the sacrifice and carries the life in Cost.Ctr
// (cb.cost refuses two counted parts).
func (cb *cardBuilder) raiseCost(c lCost) (kop.Cost, error) {
	life := c.PayLife
	if c.Sac == nil || life == 0 {
		return cb.cost(c)
	}
	if life > 255 {
		return kop.Cost{}, fmt.Errorf("raise life %d", life)
	}
	c.PayLife = 0
	kc, err := cb.cost(c)
	kc.Flags |= kop.CostPayLife
	kc.Ctr = kop.CounterKind(life)
	return kc, err
}
