package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The own-vs-intrinsic pair unit (CR 305.6, 605.3b). A land under a granted
// basic land type has the type's intrinsic "{T}: Add [c]" beside its own
// ability, and gorge lists the two as alternatives of one permanent
// (rules appendAvailableManaAbilitiesGate): Stormtide Leviathan makes every
// land an Island, so a bounce land ("{T}: Add {B}{G}", all at once) or
// Temple of the False God ("{T}: Add {C}{C}", five or more lands) taps for its
// own production or for {U}, never both. The one-ability view cannot hold it
// (static_landgrantpair.go refused it), so a land of this shape carries a
// second alternative on its view: pcard.altCol, the intrinsic colour 1+c.
//
// The planner registers the source as a pair of unit types sharing it (the
// painland's mana_pain.go shape and restrPairUnit's), the own face the
// primary and the intrinsic {c} the alternative; gorge's key 8 sorts the
// intrinsic ability before the printed one (pay.CompareSteps: ability kind),
// so the alternative is the smaller step of one source. The payment window
// lists the printed ability first and answers it (payexec.WindowAnswer), so
// every window reads the own face and never the alternative. The census,
// the affordability bounds and the potential walk count both faces
// (avail.excl). One granted colour is modelled (Stormtide's {U}); several
// stay a runtime stall (StallLandGrant).
//
// prep-only: altCol rides the card's view, so State grows no byte.

// DebugLandGrantAltOff is the mutation: no land carries the alternative view, so
// the refusal is back (altCovers) and the pod is refused at its start.
var DebugLandGrantAltOff bool

// DebugLandGrantAltPlan is the mutation: the planner sees a land's own face
// alone (altPairUnit declines), as it did before the pair.
var DebugLandGrantAltPlan bool

// DebugLandGrantAltAvail is the mutation: the census and the affordability
// bounds count a land's own face alone, as they did before the pair.
var DebugLandGrantAltAvail bool

// landAltShape: card i is a land whose only mana ability is a plain {T} one
// making a literal amount above one of one colour, or all of several colours
// at once (a bounce land: prepEachMana), possibly gated (Temple of the False
// God, prepManaGate). pc is its printed view.
func landAltShape(t *kop.Table, g *genPrep, i int, pc *pcard) bool {
	c := &t.Cards[i]
	if c.Types&(kop.TLand|kop.TCreature) == 0 || pc.manaAb < 0 || pc.manaChosen || pc.manaDyn || pc.painCol != 0 || pc.ctrMana || pc.manaOne || pc.manaSac || pc.manaSacOnly {
		return false
	}
	if f := g.flags[i]; f&gfManaVar != 0 && (f&gfManaGate == 0 || f&(gfFiltFee|gfManaDynPair|gfManaRefl) != 0) {
		return false
	}
	n := 0
	for j := range c.Abilities {
		if a := &c.Abilities[j]; a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 {
			n++
		}
	}
	a := &c.Abilities[pc.manaAb]
	cols := bits.OnesCount8(uint8(a.ManaColors) & 0x3f)
	each := a.ManaEach && cols > 1
	if n != 1 || a.AFlags != 0 || a.ManaRestrict != 0 || a.ManaRider != kop.ManaRiderNone || a.Cost.Flags != kop.CostTap ||
		a.Cost.Mana.MV() != 0 || a.Cost.Mana.X != 0 || len(a.Prog) != 0 || !pc.manaTap || pc.wideN != 0 {
		return false
	}
	return each || cols == 1 && a.ManaN > 1 && !a.ManaEach
}

// prepLandGrantAlt builds the own-vs-intrinsic views of every land of the
// shape (prepLandGrant has run): per card a block of ten, the own view with
// the intrinsic colour c in [c], and in [5+c] the intrinsic ability alone (the
// view of a gated own ability that is off).
func prepLandGrantAlt(t *kop.Table, p *prep, g *genPrep) {
	if !g.blgOn {
		return
	}
	g.altIdx = make([]int16, len(t.Cards))
	for i := range t.Cards {
		g.altIdx[i] = -1
		base := &p.cards[i]
		if !landAltShape(t, g, i, base) {
			continue
		}
		on := *base
		if vi := g.manaVar(CardID(i)); vi >= 0 {
			on = g.varPC[int(vi)+1]
		}
		g.altIdx[i] = int16(len(g.altPC) / 10)
		for c := 0; c < 5; c++ {
			v := on
			v.altCol = uint8(c) + 1
			g.altPC = append(g.altPC, v)
		}
		for c := 0; c < 5; c++ {
			v := *base
			v.manaAb = 0 // the intrinsic one (manaAbView guards the reads)
			v.manaIntrinsic = true
			v.manaCol = kop.ColorMask(1) << c
			v.manaN, v.manaTap = 1, true
			v.manaXv = [kop.NumManaKinds]uint8{}
			v.manaMulti = false
			v.blgFirst = uint8(c) + 1
			g.altPC = append(g.altPC, v)
		}
	}
}

// altGrant is the one intrinsic colour slot sl's granted basic land types add
// to a land of the shape card c's own ability does not make: its bit as a
// colour index, ok false when it has none or several (a stall).
func (s *State) altGrant(k *prep, sl Slot, c CardID) (col int, ok bool) {
	g := k.gen
	if g.altIdx == nil || g.altIdx[c] < 0 || DebugLandGrantAltOff {
		return 0, false
	}
	m := s.blgMask(k, sl) &^ ownMadeCols(&k.cards[c]) & 0x1f
	if bits.OnesCount8(uint8(m)) != 1 {
		return 0, false
	}
	return bits.TrailingZeros8(uint8(m)), true
}

// altView is manaPCGen's view of a land of the shape at sl: the own view with
// its intrinsic alternative, or when a gate has switched the own ability off
// (own is the gate-off view) the intrinsic ability alone. nil when the card has
// no single granted alternative.
func (s *State) altView(k *prep, sl Slot, c CardID, own *pcard) *pcard {
	col, ok := s.altGrant(k, sl, c)
	if !ok {
		return nil
	}
	g := k.gen
	base := int(g.altIdx[c]) * 10
	if own.manaAb < 0 && own.manaCol == 0 {
		return &g.altPC[base+5+col]
	}
	return &g.altPC[base+col]
}

// altCovers: card i's granted colours that its own ability does not make are
// one colour, and it has the alternative view (the refusal's exception).
func altCovers(g *genPrep, p *prep, i int, granted kop.ColorMask) bool {
	return g.altIdx != nil && g.altIdx[i] >= 0 && !DebugLandGrantAltOff &&
		bits.OnesCount8(uint8(granted&^ownMadeCols(&p.cards[i])&0x1f)) == 1
}

// altPairUnit registers the land at sl (view v, whose altCol is set) at pcs
// index i as a pair of unit types sharing the source: the own face (mask, the
// all-at-once rest in xv) and the intrinsic {c}. gorge's Alt.Flex and
// Alt.Colours describe the source over every alternative, so both types rank
// over the union of the colours (dmask), as restrPairUnit's do. It reports
// whether it handled the source; false leaves the one-face unit.
//
//go:noinline
func (s *State) altPairUnit(k *prep, sl Slot, v *pcard, pk *planner, i int, ma uint8) bool {
	if DebugLandGrantAltPlan || k.cards[s.cardAt(sl)].noUntap != 0 || s.canSacOnlyForMana(k, sl, v) {
		return false
	}
	na := v.manaN
	xv := s.srcExtra(k, sl, v)
	var xm, xt uint8
	for c, n := range xv {
		if n > 0 {
			xm |= 1 << c
			xt += n
		}
	}
	mb := uint8(1) << (v.altCol - 1)
	dmk := ma | mb | xm
	cr := s.isCreature(k, sl)
	ta := -1
	for t := 0; t < pk.nt && ta < 0; t++ {
		a := &pk.types[t]
		if a.pair == 0 || a.alt || a.rb || a.pain != 0 || a.mask != ma || a.n != na || a.dmask != dmk || a.creat != cr || a.xv != xv {
			continue
		}
		if b := &pk.types[a.pair-1]; b.intr && b.mask == mb {
			ta = t
		}
	}
	if ta < 0 {
		if pk.nt+2 > maxTypes || pk.nu >= maxUnits {
			s.capFull(StallLandGrant)
			return true
		}
		fl := uint8(bits.OnesCount8(dmk))
		ta = pk.nt
		tb := ta + 1
		pk.nt += 2
		pk.types[ta] = utype{mask: ma, dmask: dmk, n: na, creat: cr, flex: fl, pair: uint8(tb) + 1, xv: xv, xmask: xm, xt: xt}
		pk.types[tb] = utype{mask: mb, dmask: dmk, n: 1, creat: cr, flex: fl, pair: uint8(ta) + 1, alt: true, intr: true}
		pk.maxXt = pk.maxXt || xt != 0
		pk.maxN = max(pk.maxN, int(na)+int(xt))
		pk.pairs = true
	}
	pk.types[ta].avail++
	pk.types[ta+1].avail++
	pk.units[pk.nu] = sl
	if pk.bfl {
		pk.uobj[pk.nu] = Obj(i)
	} else {
		pk.uobj[pk.nu] = s.perms[sl].obj
	}
	pk.utyp[pk.nu] = uint8(ta)
	pk.rkind[pk.nu] = 0
	pk.rAb[pk.nu] = v.manaAb
	pk.nu++
	for c := 0; c < 5; c++ {
		if dmk&(1<<c) != 0 {
			pk.colSrc[c]++
		}
	}
	return true
}

// maxAltExcl bounds the distinct two-face lands one census holds; a land past it
// stalls StallLandGrant (five bounce lands, a Temple and a spare).
const maxAltExcl = 8

// altExcl is a group of cnt lands with the same two faces: the own face (a
// first unit of own mana of mask first, the all-at-once rest xv) or the
// intrinsic one mana of mask alt.
type altExcl struct {
	first, own, alt, cnt uint8
	xv                   [kop.NumManaKinds]uint8
}

// addAltExcl adds the land at sl (view v, altCol set) to a's census as one source
// with two faces, and reports whether it did.
func (s *State) addAltExcl(k *prep, sl Slot, v *pcard, a *avail) bool {
	if DebugLandGrantAltAvail {
		return false
	}
	e := altExcl{first: s.srcMaskAt(v, sl), own: s.srcYieldAt(k, sl, v), alt: 1 << (v.altCol - 1), cnt: 1, xv: s.srcExtra(k, sl, v)}
	big := int(e.own)
	for _, n := range e.xv {
		big += int(n)
	}
	for i := 0; i < int(a.nExcl); i++ {
		if x := &a.excl[i]; x.first == e.first && x.own == e.own && x.alt == e.alt && x.xv == e.xv {
			x.cnt++
			a.total += max(big, 1)
			a.xMax += uint16(max(big, 1))
			return true
		}
	}
	if int(a.nExcl) == len(a.excl) {
		s.capFull(StallLandGrant)
		return false
	}
	a.excl[a.nExcl] = e
	a.nExcl++
	a.total += max(big, 1)
	a.xMax += uint16(max(big, 1))
	return true
}

// exclAffords: some choice of a face for every land of a's groups from group i
// on lets the rest of the census afford cost (affordsPlain, or under the
// any-colour rider anyColor). The groups are interchangeable lands, so it
// tries how many of a group take the intrinsic face.
func (a *avail) exclAffords(i int, cost *kop.ManaCost, pipMask uint8, extra int, anyColor bool) bool {
	if i == int(a.nExcl) {
		b := *a
		b.total -= int(a.xMax)
		b.xMax, b.nExcl = 0, 0
		for g := 0; g < int(a.nExcl); g++ {
			e := &a.excl[g]
			for j := 0; j < int(e.cnt); j++ {
				if j < int(a.exclAlt[g]) {
					b.addUnits(e.alt, 1)
					continue
				}
				b.addUnits(e.first, e.own)
				for c, n := range e.xv {
					if n > 0 {
						b.addUnits(1<<c, n)
					}
				}
			}
		}
		if anyColor {
			return b.affordsAnyColorPlain(cost, extra)
		}
		return b.affordsPlain(cost, pipMask, extra)
	}
	for j := 0; j <= int(a.excl[i].cnt); j++ {
		a.exclAlt[i] = uint8(j)
		if a.exclAffords(i+1, cost, pipMask, extra, anyColor) {
			return true
		}
	}
	return false
}

// altStat counts a tap of a pair unit's face (BlgStats).
func altStat(pk *planner, ty *utype) {
	if !pk.pairs {
		return
	}
	if ty.intr {
		BlgStats.AltIntr.Add(1)
	} else if ty.pair != 0 && pk.types[ty.pair-1].intr {
		BlgStats.AltOwn.Add(1)
	}
}

// blgPainLand: pc is a painland whose normal ability is the first mana ability
// ({C} or a colour, mana_pain.go prepPainMana) and whose painful second ability
// deals damage: the granted intrinsic {c} joins the normal alternative as one
// free mana (gorge lists the printed {C}, the intrinsic {U}, then the painful
// choice; the painful one is priced last, so the view's normal unit is the merge
// of the first two).
func blgPainLand(pc *pcard) bool {
	return !DebugLandGrantPainOff && pc.manaAb >= 0 && pc.manaTap && !pc.painOnly && !pc.painLife && pc.painCol != 0 && pc.painN > 0
}

// DebugLandGrantPainOff is the mutation: a painland under a granted basic type is
// refused again.
var DebugLandGrantPainOff bool

// DebugLandGrantPainView is the mutation: a painland's granted view keeps its
// printed normal colour alone, the intrinsic {c} dropped.
var DebugLandGrantPainView bool

// DebugLandGrantFiltOff is the mutation: a filter land under a granted basic type
// is refused again.
var DebugLandGrantFiltOff bool

// DebugLandGrantFiltOrder is the mutation: the prefix search takes a filter
// land's intrinsic tap before its printed filter, as it takes a printed {C}.
var DebugLandGrantFiltOrder bool

// blgFilterFirst: a is a land's first and only mana ability and a filter ("{1},
// {T}: Add {G}{W}", Sungrass Prairie, kop.FilterManaAb): gorge's planner prices
// it as a census gap (a paid ability is no plain unit), so under a granted basic
// land type the plain census unit is the intrinsic ability alone and the filter
// stays the prefix search's (mana_filterfirst.go), as a rider-first source's
// rider does (blgRiderFirst).
func blgFilterFirst(c *kop.CardDef, pc *pcard, a *kop.Ability) bool {
	if DebugLandGrantFiltOff || c.Types&kop.TLand == 0 || pc.manaTap || !kop.FilterManaAb(a) {
		return false
	}
	n := 0
	for j := range c.Abilities {
		if x := &c.Abilities[j]; x.Kind == kop.AbMana && x.AFlags&kop.AfLatent == 0 {
			n++
		}
	}
	return n == 1
}

// filtIntrinsicFree: the free ability of the filter source sl is a granted basic
// land type's intrinsic one (blgFilterFirst's view), which gorge's ability ask lists
// after the printed filter (printed abilities first, CR 305.6): the prefix
// search answers the filter first, the intrinsic tap after it.
func (s *State) filtIntrinsicFree(k *prep, sl Slot) bool {
	return k.gen != nil && k.gen.blgOn && !DebugLandGrantFiltOrder && s.manaPCGen(k, sl).manaIntrinsic && k.gen.has(s.cardAt(sl), gfFilt)
}

// DebugLandGrantAltFund is the mutation: a prefix search's funding tap of a
// two-face land answers its own face alone.
var DebugLandGrantAltFund bool

// fundAlt taps the two-face funding source sl (view pc, altCol set) for its own
// face, then for the granted intrinsic one: gorge's ability ask lists the
// printed ability before the intrinsic one (CR 305.6) and the search takes the
// answers in that order.
func (fs *filtSearch) fundAlt(k *prep, t *State, sl Slot, pc *pcard, funded int, gapUsed bool) bool {
	ans := uint8(bits.TrailingZeros8(t.fundMaskAt(k, pc, sl)))
	if fs.try(k, t, filtStep{sl: sl, ans: ans}, funded+1, gapUsed) {
		return true
	}
	return fs.tick() && fs.try(k, t, filtStep{sl: sl, ans: pc.altCol - 1, intr: true}, funded+1, gapUsed)
}

// prepLandGrantViews builds the landgrant prep-time view tables: the
// own-vs-intrinsic alternatives and the gated lands' merged gate views.
func prepLandGrantViews(t *kop.Table, p *prep, g *genPrep) {
	prepLandGrantAlt(t, p, g)
	prepLandGrantGate(t, p, g)
}
