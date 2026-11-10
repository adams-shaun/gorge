package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A pain source in the granted-mana prefix search (mana_grantedoffers.go):
// a permanent whose printed mana abilities are a normal alternative beside
// a painful one (mana_pain.go: Haunted Screen's {W} or {B} beside its {G},
// {U} or {R} for 1 life; a painland's {C} beside its damaging colours) that
// also holds a granted ability (Haunted Screen animated under Enduring
// Vitality). gorge's "choose a mana ability" ask lists every ability the
// source has (appendAvailableManaAbilities: the printed ones in order, then
// the grants), each Combo flattened into one option per colour with its
// symbol and a single-colour ability as one option without, and its prefix
// search (potential_script.go scriptOrderAnswers) moves the options whose
// symbol the play needs first. So the painful ability's colours are answers
// of that ask too: gorge reaches {G} for 1 life before the normal {W} when
// the play needs {G}. A painful answer pays its life (CR 119.4) or deals its
// damage (CR 605.3b) as the ability resolves, in the search and in the
// replayed prefix (grantApply).

// painOpt is one option of a pain source's own ask: the colour, whether the
// painful ability makes it, and whether it carries a symbol.
type painOpt struct {
	col       uint8
	pain, sym bool
}

// grantPainShape: sl's printed view pc is a pain source with a normal
// alternative whose painful colours gorge flattens (not an Any ability: its
// stage-2 colour ask is another node, outside this model).
func grantPainShape(k *prep, pc *pcard) bool {
	return k.painMana && pc.painCol != 0 && !pc.painOnly && pc.manaAb >= 0 && uint8(pc.painCol)&0x1f != 0 && pc.painCol != kop.ColAny
}

// painAbility is the painful mana ability of card c: its second non-latent
// mana ability (prepPainMana's shape), nil when there is none.
func painAbility(c *kop.CardDef) *kop.Ability {
	n := 0
	for j := range c.Abilities {
		a := &c.Abilities[j]
		if a.Kind != kop.AbMana || a.AFlags&kop.AfLatent != 0 {
			continue
		}
		if n++; n == 2 {
			return a
		}
	}
	return nil
}

// painAskOrder lists sl's own ask options in gorge's order: the normal
// ability's, then the painful one's (left out when its life cannot be paid,
// CR 119.4), then those whose symbol need names first, engine order kept
// inside each half (needFirst's partition over the symbol options only).
func (s *State) painAskOrder(k *prep, p uint8, sl Slot, pc *pcard, need [kop.NumManaKinds]bool, out *[2 * kop.NumManaKinds]painOpt) int {
	c := &k.t.Cards[s.cardAt(sl)]
	n := 0
	add := func(a *kop.Ability, pain bool) {
		if a == nil {
			return
		}
		if uint8(a.ManaColors)&0x1f == 0 {
			out[n] = painOpt{col: kop.C, pain: pain}
			n++
			return
		}
		var ord [5]uint8
		m := manaColourOrder(a, &ord)
		for _, col := range ord[:m] {
			out[n] = painOpt{col: col, pain: pain, sym: m > 1}
			n++
		}
	}
	add(&c.Abilities[pc.manaAb], false)
	if !pc.painLife || int(s.pl[SeatIx(p)].life) >= int(pc.painN) {
		add(painAbility(c), true)
	}
	var first, rest [2 * kop.NumManaKinds]painOpt
	nf, nr := 0, 0
	for _, o := range out[:n] {
		if o.sym && need[o.col] {
			first[nf] = o
			nf++
		} else {
			rest[nr] = o
			nr++
		}
	}
	copy(out[:], first[:nf])
	copy(out[nf:], rest[:nr])
	return n
}

// painBranch is branch for a pain source's own abilities (grantPainShape):
// each option of painAskOrder in turn, the ask one node when ask and it has
// several options.
func (g *grantSrch) painBranch(t *State, i int, sl Slot, pc *pcard, ask bool) bool {
	var opts [2 * kop.NumManaKinds]painOpt
	no := t.painAskOrder(g.k, g.p, sl, pc, g.need, &opts)
	if ask && no > 1 && !g.node() {
		return false
	}
	n := pc.manaN
	var xpc *pcard
	if g.k.gen.anyTapExtra {
		xpc = t.stepView(g.k, sl, -1)
	}
	for _, o := range opts[:no] {
		u := t.fork()
		u.perms[sl].flags |= pfTapped
		u.pl[SeatIx(g.p)].pool[o.col] += n
		u.artTag(g.k, g.p, sl, int(o.col))
		xv := u.srcExtraPick(g.k, sl, xpc, int(o.col))
		u.addXv(g.p&1, &xv)
		if o.pain {
			u.painDamage(g.k, sl, g.p&1, int(pc.painN))
		}
		g.path[g.np] = grantStep{i: uint8(i), col: o.col, n: n, gi: -1, sl: sl, pain: o.pain}
		g.np++
		ok := g.next(u, sl)
		t.unfork()
		if ok {
			return true
		}
		g.np--
	}
	return false
}

// ownRestrPrinted: sl's printed ability the search taps is its restricted
// one (the dual branch's mana_grantrestrictedown.go case).
func (s *State) ownRestrPrinted(k *prep, sl Slot) bool {
	return s.ownRestrAb(k, sl) != nil && !s.ownManaUsable(k, sl)
}
