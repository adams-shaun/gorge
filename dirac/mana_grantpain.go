package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A grant-search step on a pain source's own abilities (Haunted Screen
// animated under Enduring Vitality: {W} or {B}, then {G}, {U} or {R} for 1
// life, beside the granted any colour). gorge's prefix search answers the
// source's "choose a mana ability" ask, which flattens every printed
// ability's colours into one list (the normal ability's, then the painful
// one's, each in printed order; the granted ability's after them), and
// scriptOrderAnswers moves the options whose symbol the play needs to the
// front (potential_script.go). So a play needing {R} tries the painful {R}
// first and pays its life (CR 119.4, 605.3b). The kernel's search used to
// list only the normal colours, so it took the {W}.

// stepOrder lists the answers branch tries for a step on sl: the colours
// stepColours names, followed by the painful alternative's colours when gi
// is the source's own ability and it has one gorge offers (a painful life
// cost the payer can pay), with the needed colours moved to the front
// (needFirst over the whole list). Bit j of pain marks order[j] as a
// painful answer.
func (g *grantSrch) stepOrder(t *State, sl Slot, gi int16, own []uint8, order *[2 * kop.NumManaKinds]uint8) (n int, pain uint16) {
	n = copy(order[:], own)
	if gi < 0 && g.k.painMana {
		if pc := t.manaPCGen(g.k, sl); pc.painCol != 0 && !pc.painOnly && (!pc.painLife || int(t.pl[g.p&1].life) >= int(pc.painN)) {
			var pord [5]uint8
			for _, c := range pord[:painColourOrder(g.k, t.cardAt(sl), pc.painCol, &pord)] {
				order[n] = c
				pain |= 1 << n
				n++
			}
		}
	}
	// a stable partition, needed colours first, carrying the pain bits
	var out [2 * kop.NumManaKinds]uint8
	var outPain uint16
	m := 0
	for pass := 0; pass < 2; pass++ {
		for j := 0; j < n; j++ {
			if g.need[order[j]] == (pass == 0) {
				out[m] = order[j]
				if pain&(1<<j) != 0 {
					outPain |= 1 << m
				}
				m++
			}
		}
	}
	*order = out
	return n, outPain
}

// painColourOrder writes the painful alternative's colours (mask col) in its
// ability's printed order: the card's second mana ability (prepPainMana).
func painColourOrder(k *prep, c CardID, col kop.ColorMask, out *[5]uint8) int {
	nm := 0
	for j := range k.t.Cards[c].Abilities {
		a := &k.t.Cards[c].Abilities[j]
		if a.Kind != kop.AbMana || a.AFlags&kop.AfLatent != 0 {
			continue
		}
		if nm++; nm == 2 && uint8(a.ManaColors)&0x1f == uint8(col)&0x1f {
			return manaColourOrder(a, out)
		}
	}
	n := 0
	for c := uint8(0); c < 5; c++ {
		if uint8(col)&(1<<c) != 0 {
			out[n] = c
			n++
		}
	}
	return n
}

// grantPainPay pays a painful prefix step's cost or rider: the life its
// {T}, Pay N life ability costs, or the painland's damage (painDamage).
func (s *State) grantPainPay(k *prep, p uint8, sl Slot) {
	s.painDamage(k, sl, p, int(s.manaPCAt(k, sl).painN))
}
