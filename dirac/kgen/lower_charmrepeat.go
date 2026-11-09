package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// A charm that may choose a mode more than once (CanRepeatModes$: Cosmium
// Confluence, Moment of Reckoning, the Seasons; kop/repeat.go has the
// encoding). Each mode's block is lowered once and copied for every further
// instance the mode can be chosen, so the block of instance r runs only when
// the mode was chosen more than r times and every copy reads the mode's one
// template target slot. A pawprint budget (Pawprint$ on the charm, CR 700.2i
// and 107.18) prices each mode with its body's own Pawprint$ and bounds how
// often a mode can be chosen.

// repeatPlan is the layout of a repeatable charm's chosen-mode count byte.
type repeatPlan struct {
	max, cost    [4]uint8 // the most times each mode is chosen; its pawprint price
	budget       uint8    // the pawprint budget; 0 = none
	shift, width [4]uint8 // kop.RepeatLayout
	pick         int
}

// c is the OpCharm C of instance r of mode m.
func (rp *repeatPlan) c(m, r int) int {
	return int(kop.RepeatCharmC(int(rp.shift[m]), int(rp.width[m]), r, m))
}

// charmRepeatPlan reads CanRepeatModes$ and Pawprint$ (nil plan: the charm
// does not repeat). It refuses by name the shapes the kernel does not read.
func (lc *lowerCtx) charmRepeatPlan(ps *params, choices []string, pick, twPick int, dyn bool) (*repeatPlan, error) {
	v, rep := ps.get("CanRepeatModes")
	budgetS, paw := ps.get("Pawprint")
	if !rep {
		if paw {
			return nil, refuse("charm Pawprint$ without CanRepeatModes$")
		}
		return nil, nil
	}
	if strings.TrimSpace(v) != "True" {
		return nil, refuse("charm CanRepeatModes$ %q", v)
	}
	ps.consume("CanRepeatModes")
	switch {
	case lc.ab.Kind != akSpell:
		return nil, refuse("repeatable charm on a %s ability", abilityKindNames[lc.ab.Kind])
	case twPick > 0 || dyn || lc.out.ModesMinCount != 0 || lc.out.Spree:
		return nil, refuse("repeatable charm with a variable, teamwork or per-mode-cost count")
	case len(choices) > 4:
		return nil, refuse("repeatable charm with %d modes (the count byte holds 4)", len(choices))
	case pick < 2:
		return nil, refuse("repeatable charm choosing %d", pick)
	}
	rp := &repeatPlan{pick: pick}
	budget := -1
	if paw {
		n, err := strconv.Atoi(strings.TrimSpace(budgetS))
		if err != nil || n < 1 || n > 255 {
			return nil, refuse("charm Pawprint$ %q", budgetS)
		}
		ps.consume("Pawprint")
		budget, rp.budget = n, uint8(n)
	}
	for i, name := range choices {
		sa := cards.ResolveSVar(lc.face.SVars, strings.TrimSpace(name))
		if sa == nil {
			return nil, refuse("charm choice %q is not an ability", name)
		}
		cost := 0
		if c, ok := sa.Params["Pawprint"]; ok {
			n, err := strconv.Atoi(strings.TrimSpace(c))
			if budget < 0 || err != nil || n < 0 || n > 255 {
				return nil, refuse("charm mode Pawprint$ %q (budget %d)", c, budget)
			}
			cost = n
		}
		most := pick
		if budget >= 0 && cost > 0 {
			most = min(pick, budget/cost)
		}
		if most == 0 || most > 7 {
			return nil, refuse("charm mode chosen up to %d times (cost %d of %d pawprints)", most, cost, budget)
		}
		rp.max[i], rp.cost[i] = uint8(most), uint8(cost)
	}
	var n int
	rp.shift, rp.width, n = kop.RepeatLayout(rp.max)
	if n > 8 {
		return nil, refuse("repeatable charm: mode counts need %d bits (the mode byte holds 8)", n)
	}
	return rp, nil
}

// replicate appends the copies of the block lowered at prog[at:] for mode m's
// instances 1..max-1.
func (rp *repeatPlan) replicate(lc *lowerCtx, at, m int) {
	block := append([]lInstr(nil), lc.ab.Prog[at:]...)
	for r := 1; r < int(rp.max[m]); r++ {
		b := append([]lInstr(nil), block...)
		b[0].C = rp.c(m, r)
		lc.ab.Prog = append(lc.ab.Prog, b...)
	}
}

// charmRepeatTargets refuses the target shapes a repeatable charm's slots do
// not carry: a mode owns one 1..1 declaration (its template slot), and the
// template slots plus the most extra instances must fit the target slots.
func (lc *lowerCtx) charmRepeatTargets(rp *repeatPlan, nModes int) error {
	tmpl, sumMax := 0, 0
	for m := 0; m < nModes; m++ {
		n := 0
		for _, t := range lc.ab.Targets {
			if t.Mode != m {
				continue
			}
			n++
			if !t.Min.isConst() || t.Min.N != 1 || !t.Max.isConst() || t.Max.N != 1 {
				return refuse("repeatable charm mode target bounds %s..%s", t.Min, t.Max)
			}
		}
		if n > 1 {
			return refuse("repeatable charm mode with %d target declarations", n)
		}
		if n == 1 {
			tmpl++
			sumMax += int(rp.max[m])
		}
	}
	if tmpl > 0 {
		if need := tmpl + max(min(rp.pick, sumMax)-1, 0); need > kop.MaxTargets {
			return refuse("repeatable charm needs %d target slots (%d fit)", need, kop.MaxTargets)
		}
	}
	return nil
}
