package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A Combo Any source that pays a forage (Thornvault Forager: "{T}, Forage: Add two
// mana in any combination of colors", kop.ForageComboAnyAb) is the Combo Any gap
// (mana_comboany.go) with one more answer in its activation. gorge pays the forage
// in its mana-activation walk (pay manacostwalk.go, manaCostForageStage): the
// options are pay.ForageOptions' (forage_exile first, when the graveyard holds
// three cards, then one forage_food per Food the payer controls, in battlefield
// order); none drops the activation, one settles unasked, two or more pose the
// "forage" ask (AskManaForage). The ask comes before the colour ask of the
// combination; the cost's order is the {T}, the forage (the first three graveyard
// cards exiled in zone order, else the Food sacrificed), then the mana
// (SettleManaForage, resolveManaEffect).
//
// The Forager's other ability ({T}: Add {G}) is a planner unit, but gorge's search
// tries it too: the census gap is the permanent (scriptGap reads the object), so
// the source's one activate option, answered, poses the "choose a mana ability" ask
// (printed order, no hoist: scriptOrderManaAbilityAnswers) over the plain ability
// and the forage one whenever the forage is payable; the plain one adds its {G} and
// ends there, the forage one goes on to the forage ask (two or more options) and the
// colour ask. A card whose other mana ability is not the one plain fixed-colour {T}
// is refused at the table's start (StallComboAnyMix, forageMixGap).
//
// The forage source is a gap of the search alone: caGaps leaves it out, so the
// payment windows (availCA, windowComboTap: gorge's window auto-answers the ability
// ask with the printed {G}) and the filter, grant and tap-another searches never see
// it; a table holding it beside a gap kind the Combo Any search does not chain
// stalls by name instead (prepComboAnyMix). The search is caSearch's: a forage
// source is tried as its ability ask, forage ask and colour ask nodes, in gorge's
// order (forageSource), and a found script replays its tap, forage and mana.

// DebugForagePickLast answers the forage ask with the last option where the search
// and its replay choose (mutation testing only: the lockstep must catch it).
var DebugForagePickLast bool

// forageExileOpt marks the exile arm in a forage option list (a Food is its slot).
const forageExileOpt Slot = NoSlot

// forageCA reports whether card c's Combo Any census gap is the forage one.
func (k *prep) forageCA(c CardID) bool {
	g := k.gen
	return g != nil && k.anyForageCA && g.isComboAnyCard(c, &k.t.Cards[c]) && kop.ForageManaCost(&k.t.Cards[c].Abilities[g.gapOther(c)].Cost)
}

// forageSibling is the index of the mana ability beside ability j of cd the
// activation's ability ask also offers: its one plain fixed-colour {T} ability, -1
// when it has none. ok is false for any other shape (the table is refused).
func forageSibling(cd *kop.CardDef, j int) (sib int, ok bool) {
	sib = -1
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if i == j || a.Kind != kop.AbMana || a.AFlags&kop.AfLatent != 0 {
			continue
		}
		c := a.ManaColors
		if sib >= 0 || a.Cost.Flags != kop.CostTap || a.Cost.Mana != (kop.ManaCost{}) || a.AFlags != 0 || a.ManaRestrict != 0 || a.ManaRestrictAb != 0 ||
			a.ManaEach || a.ManaDeferred || a.ManaRider != kop.ManaRiderNone || len(a.Prog) != 0 || a.Gate.Ctr != 0 || a.Trig.Cond != 0 ||
			a.ManaN > 1 || c == 0 || c&(c-1) != 0 || a.ActZone != kop.ZoneNone && a.ActZone != kop.ZoneBattlefield {
			return -1, false
		}
		sib = i
	}
	return sib, true
}

// forageMixGap reports whether card i is a census-gap kind a table holding a forage
// Combo Any source does not search with it (caMixGap): any gap kind outside the
// Combo Any one, or a forage card whose other mana ability is not the plain
// fixed-colour {T} (forageSibling).
func forageMixGap(t *kop.Table, g *genPrep, p *prep, i int) bool {
	c := CardID(i)
	if !p.anyForageCA || !g.has(c, gfGapOther) {
		return false
	}
	if !g.isComboAnyCard(c, &t.Cards[i]) {
		return true
	}
	j := int(g.gapOther(c))
	if !kop.ForageManaCost(&t.Cards[i].Abilities[j].Cost) {
		return false
	}
	_, ok := forageSibling(&t.Cards[i], j)
	return !ok
}

// forageMixTable reports whether t holds, beside a forage Combo Any source, a gap
// kind the Combo Any search does not chain with it (prepComboAnyMix): an
// each-colour, restricted, Taps-deferred, rider or counter-cost source.
func forageMixTable(g *genPrep, p *prep) bool {
	return p.anyForageCA && (g.anyEachCol || g.anyRestr || g.anyAnyR || g.anyTapsDefer || g.anyManaRider || g.anyDeferred || g.anyRestrScript || p.ctrMana)
}

// foragePayable reports whether the forage source at sl can pay now: gorge's offer
// gate (pay.ManaForagePayable): three graveyard cards or a Food.
func (s *State) foragePayable(k *prep, p uint8, sl Slot) bool {
	return s.foragePayableRes(k, p, k.forageAbOf(s.cardAt(sl)).Cost.Filter)
}

func (k *prep) forageAbOf(c CardID) *kop.Ability {
	return &k.t.Cards[c].Abilities[k.gen.gapOther(c)]
}

// forageOpts lists the forage election's options for the source at sl: the exile
// arm (forageExileOpt) when the graveyard holds three cards, then each Food in
// battlefield order (the cast path's costObjOptions order).
func (s *State) forageOpts(k *prep, p uint8, sl Slot, out *[MaxPerm + 1]Slot) int {
	n := 0
	if s.pl[p].graveN >= 3 {
		out[n] = forageExileOpt
		n++
	}
	f := k.forageAbOf(s.cardAt(sl)).Cost.Filter
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		if fs := m.first(); s.sacMatch(k, f, fs, p, sl) {
			out[n] = fs
			n++
		}
	}
	return n
}

// availForageCA adds each payable forage source's units (availCA's, for the
// sources caGaps leaves out): the offers' relaxed bound counts them as gorge's
// census does.
func (s *State) availForageCA(k *prep, p uint8, a *avail) {
	if !k.anyForageCA {
		return
	}
	for b := s.caGapsF(k, p, true).andNot(s.caGaps(k, p)); b.any(); b = b.rest() {
		sl := b.first()
		if s.pickCommittedSrc(k, sl) {
			continue
		}
		a.addUnits(0x1f, uint8(s.caAmount(k, sl)))
	}
}

// forageActivate activates the forage source at sl in gorge's cost order: the
// {T}, the forage elected as opt (forageExileOpt: the first three graveyard
// cards, else the Food at slot opt), then the mana of colour answer ans.
func (s *State) forageActivate(k *prep, p uint8, sl, opt Slot, ans int) {
	n := s.caAmount(k, sl)
	s.caTap(k, sl)
	if opt == forageExileOpt {
		s.forageExile3(p)
	} else {
		s.forageSacFood(k, opt)
	}
	s.fireKwAct(k, kop.TrigForage, p, 0)
	s.caAdd(k, p, sl, ans, n)
	s.sba(k) // a sacrificed Food token ceases to exist at once (CR 704.5d): it is no graveyard card for the next forage
}

// forageSibActivate activates the forage source's plain ability sib: the {T} and
// its fixed colour.
func (s *State) forageSibActivate(k *prep, p uint8, sl Slot, sib int) {
	ab := &k.t.Cards[s.cardAt(sl)].Abilities[sib]
	s.caTap(k, sl)
	cols, n := tapXUnit(ab)
	col := bits.TrailingZeros8(cols)
	s.poolAdd(p, col, int(n))
	s.artTag(k, p, sl, col)
	s.tapExtraStall(k, sl)
}

// tick counts one search node (caSearch's budget); false when it is spent.
func (cs *caSearch) tick() bool {
	if cs.nodes >= potentialScriptNodes {
		return false
	}
	cs.nodes++
	return true
}

// branch plays one activation on a fork of t and searches below it
// (caSearch.visit's per-answer step).
func (cs *caSearch) branch(k *prep, t *State, d *Decision, st caStep, do func(c *State)) bool {
	c := t.fork()
	do(c)
	cs.path[cs.np] = st
	cs.np++
	found := cs.visit(k, c, d, false)
	if !found {
		cs.stallOf(c)
	}
	t.unfork()
	if found {
		return true
	}
	cs.np--
	return false
}

// forageSource is visit's branch for the forage source at sl, in gorge's ask
// order. The activation's first ask is the ability ask beside a plain sibling
// (answered in printed order: the sibling's {G}, which ends there, or the forage
// ability), else the forage ask; the forage ability then asks which forage (two or
// more options; one settles unasked), then the colour answers of the combination.
// Every ask is one node (a Submit's visit): the ability ask, the forage ask or the
// colour ask it leads to, and the colour ask after each forage answer.
func (cs *caSearch) forageSource(k *prep, t *State, d *Decision, sl Slot) bool {
	c := t.cardAt(sl)
	if cs.own != NoObj && t.perms[sl].obj == cs.own {
		return false // the searched activation's own {T} source: its mana leaves that cost unpayable (CR 602.2b; gorge's ownTapSource)
	}
	j := int(k.gen.gapOther(c))
	sib, _ := forageSibling(&k.t.Cards[c], j)
	if !cs.tick() { // the activation's first ask
		return false
	}
	var opts [MaxPerm + 1]Slot
	n := t.forageOpts(k, cs.p, sl, &opts)
	units := t.caAmount(k, sl)
	return forageAsks(sib, j, n, units, &cs.need, cs.tick,
		func(o, ans int) bool {
			opt := opts[o]
			return cs.branch(k, t, d, caStep{sl: sl, ans: uint8(ans), fo: uint8(o)}, func(f *State) { f.forageActivate(k, cs.p, sl, opt, ans) })
		},
		func() bool {
			return cs.branch(k, t, d, caStep{sl: sl, sib: true}, func(f *State) { f.forageSibActivate(k, cs.p, sl, sib) })
		})
}

// forageAsks walks the forage source's asks in gorge's ask order, the one home of
// that order for both searches (forageSource, forageOther): sib is the plain
// sibling's ability index (-1 for none), j the forage ability's, n the forage
// options (forageOpts), units the colour answers' size. tick spends one node per
// ask; forage branches one (option o, colour answer ans); plain branches the
// sibling's activation. The sibling comes first when its ability prints first.
func forageAsks(sib, j, n, units int, need *[kop.NumManaKinds]bool, tick func() bool, forage func(o, ans int) bool, plain func() bool) bool {
	viaForage := func() bool {
		if sib >= 0 && !tick() { // the forage ask (two or more options), else the colour ask
			return false
		}
		for oi := 0; oi < n; oi++ {
			o := oi
			if DebugForagePickLast {
				o = n - 1 - oi
			}
			if n >= 2 && !tick() { // the colour ask after the forage answer
				return false
			}
			for r, na := 0, caAnswerCount(units); r < na; r++ {
				if forage(o, caRank(units, need, r)) {
					return true
				}
			}
		}
		return false
	}
	if sib >= 0 && sib < j {
		return plain() || viaForage()
	}
	return viaForage() || sib >= 0 && plain()
}

// forageOther is forageSource for the filter search (mixComboAny): the forage
// source at sl as a gap beside a filter, branched in gorge's ask order -- the
// activation's ability ask (a plain sibling's {G}, which ends there, or the forage
// ability), the forage ask (two or more options), then the colour answers --
// each ask one node (fs.tick), the steps replayed by forageReplay.
func (fs *filtSearch) forageOther(t *State, k *prep, sl Slot, try func(otherStep) bool) bool {
	c := t.cardAt(sl)
	j := int(k.gen.gapOther(c))
	sib, _ := forageSibling(&k.t.Cards[c], j)
	if !fs.tick() { // the activation's first ask
		return false
	}
	var opts [MaxPerm + 1]Slot
	n := t.forageOpts(k, fs.p, sl, &opts)
	units := t.caAmount(k, sl)
	return forageAsks(sib, j, n, units, &fs.need, fs.tick,
		func(o, ans int) bool {
			return try(otherStep{kind: otherComboAny, ans: uint8(ans), fo: uint8(o)})
		},
		func() bool { return try(otherStep{kind: otherComboAny, plain: true}) })
}

// forageReplay plays the found script's step st of the forage source on s.
func (s *State) forageReplay(k *prep, p uint8, st caStep) {
	if st.sib {
		sib, _ := forageSibling(&k.t.Cards[s.cardAt(st.sl)], int(k.gen.gapOther(s.cardAt(st.sl))))
		s.forageSibActivate(k, p, st.sl, sib)
		return
	}
	var opts [MaxPerm + 1]Slot
	s.forageOpts(k, p, st.sl, &opts)
	s.forageActivate(k, p, st.sl, opts[st.fo], int(st.ans))
}

// ownTapObj is the object whose {T} the activation a costs, NoObj for any other
// play (gorge's scriptSearch.ownTapSource: the search never activates that source
// for mana, so the play's own {T} stays payable).
func (s *State) ownTapObj(k *prep, a Action) Obj {
	if a.Kind() != AkActivate {
		return NoObj
	}
	o := Obj(a.Source())
	cd := &k.t.Cards[s.objs[o].card]
	if m := int(a.Mode()); m >= len(cd.Abilities) || cd.Abilities[m].Cost.Flags&kop.CostTap == 0 {
		return NoObj
	}
	return o
}

// comboAnyGapAb reports whether ability a is a Combo Any census gap on its card:
// the card's first mana ability (first), or a forage Combo Any one beside a plain
// ability (it also sets p.anyForageCA). prepareGen calls it, so the arm stays one line.
func comboAnyGapAb(p *prep, a *kop.Ability, first bool) bool {
	if !comboAnyMana(a) {
		return false
	}
	forage := kop.ForageComboAnyAb(a)
	p.anyForageCA = p.anyForageCA || forage
	return first || forage
}

// forageCAOnly reports whether every Combo Any census gap of t is a forage one: no
// plain Combo Any source and no reflected Amount$ one (the two other sources of
// genPrep.anyComboAny). StallComboAnyMix then follows the forage sources alone.
func forageCAOnly(t *kop.Table, g *genPrep) bool {
	for i := range t.Cards {
		cd := &t.Cards[i]
		if c := CardID(i); g.isComboAnyCard(c, cd) && !kop.ForageManaCost(&cd.Abilities[g.gapOther(c)].Cost) {
			return false
		}
		first := firstManaAb(cd)
		for j := range cd.Abilities {
			if a := &cd.Abilities[j]; a.Kind == kop.AbMana && j == first && reflAmtManaAb(a) {
				return false
			}
		}
	}
	return true
}

// initStallDecks is the stall the game just laid out stalls at: prep.initStall, but
// a table whose Combo Any mix is the forage sources' alone (caMixForage: Thornvault
// Forager beside the blb pool's filters, grants and tap-another sources) stalls only
// when a deck or sideboard holds a forage source. A whole-pool table played with
// decks that hold none is a game the mix never reaches; a built matchup holds its
// decks' cards alone, so its pair refusal (stallclass.go pairRefusalN) is unchanged.
func (s *State) initStallDecks(k *prep) uint8 {
	skip := k.caMixForage
	for o := Obj(0); skip && int(o) < int(s.nobj); o++ {
		skip = !k.forageCA(s.objs[o].card)
	}
	return k.initStallSkip(s.nSeats(), skip)
}
