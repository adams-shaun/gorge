package hbot

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// choose.go: botpolicy.decide's KChoose, KModes, KTriggerOptional,
// KTriggerOrder, KReplacement and KMulligan arms, over the kernel's single
// picks. Each kernel sub-kind is the gorge option kind it canonicalises to
// (dirac/oracle fdnChoose); gorge's option order is dirac/feat optKey's.
// A multi-pick answer gorge gives as a set is served as its first member
// in gorge's order (the set itself is a greedy choice, so every later pick
// continues it).

// objKey is a choice object's place in gorge's zone lists (dirac/feat
// zoneOf): stack, battlefield, graveyard, exile, hand, library; seat; zone
// position.
func (bt *Bot) objKey(s *dirac.State, o dirac.Obj) uint32 {
	var zone, seat, idx uint32
	switch s.ObjZone(o) {
	case kop.ZoneStack:
		for i := 0; i < s.StackLen(); i++ {
			if _, src, _, _ := s.StackEntry(i); src == o {
				idx = uint32(i)
			}
		}
	case kop.ZoneBattlefield:
		zone, seat, idx = 1, uint32(s.Controller(o)), uint32(bt.b.bfPos[o])
	case kop.ZoneGraveyard:
		zone, seat, idx = 2, uint32(s.ObjOwner(o)), uint32(s.FeatZoneIndex(o))
	case kop.ZoneExile:
		zone, seat, idx = 3, uint32(s.ObjOwner(o)), uint32(s.FeatZoneIndex(o))
	case kop.ZoneHand:
		zone, seat, idx = 4, uint32(s.ObjOwner(o)), uint32(s.FeatZoneIndex(o))
	default:
		zone, seat, idx = 5, uint32(s.ObjOwner(o)), uint32(s.FeatZoneIndex(o))
	}
	return zone<<20 | (seat%dirac.MaxSeats)<<16 | idx
}

// firstBy returns the option (not Done) with the smallest key.
func (bt *Bot) firstBy(s *dirac.State, d *dirac.Decision, key func(a dirac.Action) uint32) int {
	best, bk := -1, uint32(0)
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == dirac.AkDone {
			continue
		}
		k := key(a)
		if best < 0 || k < bk {
			best, bk = i, k
		}
	}
	if best < 0 {
		return 0
	}
	return best
}

func (bt *Bot) firstObj(s *dirac.State, d *dirac.Decision) int {
	bt.b.fillPos(s)
	return bt.firstBy(s, d, func(a dirac.Action) uint32 { return bt.objKey(s, dirac.Obj(a.Source())) })
}

func firstMode(d *dirac.Decision) int {
	best := -1
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == dirac.AkDone {
			continue
		}
		if best < 0 || a.Mode() < d.Options[best].Mode() {
			best = i
		}
	}
	if best < 0 {
		return 0
	}
	return best
}

// firstPip is gorge's first offer of a hybrid pip (rules manaAsk): the symbol's
// own order, which runs clockwise on the colour wheel, so {G/U} offers green first.
func firstPip(d *dirac.Decision) int {
	best := firstMode(d)
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() != dirac.AkDone && i != best {
			if (d.Options[best].Mode()+5-a.Mode())%5 <= 2 {
				best = i
			}
		}
	}
	return best
}

func modeIndex(d *dirac.Decision, m uint8) int {
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() != dirac.AkDone && a.Mode() == m {
			return i
		}
	}
	return -1
}

// lowest serves chooseLowest(worth): the PicksLeft lowest-worth options
// (ties to gorge's earlier option), their first in gorge's order.
func (bt *Bot) lowest(s *dirac.State, d *dirac.Decision, worth func(o dirac.Obj) int32) int {
	b := &bt.b
	b.fillPos(s)
	m := s.PicksLeft()
	// rank: an option is in the set when fewer than m options beat it
	best, bk := -1, uint32(0)
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == dirac.AkDone {
			continue
		}
		oi := dirac.Obj(a.Source())
		wi, ki := worth(oi), bt.objKey(s, oi)
		beat := 0
		for j := 0; j < int(d.N); j++ {
			x := d.Options[j]
			if j == i || x.Kind() == dirac.AkDone {
				continue
			}
			oj := dirac.Obj(x.Source())
			wj := worth(oj)
			if wj < wi || wj == wi && bt.objKey(s, oj) < ki {
				beat++
			}
		}
		if beat >= m {
			continue
		}
		if best < 0 || ki < bk {
			best, bk = i, ki
		}
	}
	if best < 0 {
		return 0
	}
	return best
}

// discardWorth is chooseDiscard's worth: a basic land or a free noncreature
// is 1, anything else minus its mana value.
func (b *board) discardWorth(o dirac.Obj) int32 {
	c := b.cardOf(o)
	if c.basic || (!c.creature && c.cmc == 0) {
		return 1
	}
	return -c.cmc
}

func (bt *Bot) choose(s *dirac.State, d *dirac.Decision) int {
	b := &bt.b
	switch d.Sub {
	case dirac.ChArrange, dirac.ChScry, dirac.ChSurveil:
		// KArrange has no arm: the empty answer (every card to the
		// bottom / the graveyard)
		if i := doneIndex(d); i >= 0 {
			return i
		}
		// a dig's rest (min = max): Clamp tops up in gorge's offered order
		b.fillPos(s)
		return bt.firstObj(s, d)
	case dirac.ChX, dirac.ChTrigX, dirac.ChMultikick:
		// "x": the most it can pay for
		best := 0
		for i := 1; i < int(d.N); i++ {
			if d.Options[i].Mode() > d.Options[best].Mode() {
				best = i
			}
		}
		return best
	case dirac.ChCostDiscard, dirac.ChCleanupDiscard, dirac.ChBottom:
		b.fill(s, bt.f, d.Player)
		return bt.lowest(s, d, b.discardWorth)
	case dirac.ChDiscard:
		// gorge's KModes "discard": the first Min cards in hand order, so
		// nothing more once Min is met
		if i := doneIndex(d); i >= 0 {
			return i
		}
		return bt.firstObj(s, d)
	case dirac.ChCostSacrifice, dirac.ChSacrifice, dirac.ChGenPayPick, dirac.ChGenPayReturn:
		if doneIndex(d) >= 0 {
			return bt.firstObj(s, d) // optional: the first offered permanent
		}
		b.fill(s, bt.f, d.Player)
		return bt.lowest(s, d, b.cardWorth)
	case dirac.ChWardPay, dirac.ChGenPayMode:
		// the unless-pay modes: decline when the opposing board already
		// kills the payer, else pay
		b.fill(s, bt.f, d.Player)
		if b.facingLethal(d.Player) {
			if i := modeIndex(d, 0); i >= 0 {
				return i
			}
		}
		if i := modeIndex(d, 1); i >= 0 {
			return i
		}
		return 0
	case dirac.ChOptional, dirac.ChOptDiscMay, dirac.ChGenPay, dirac.ChGenTopBottom, dirac.ChGenLeyline, dirac.ChGenMayShuffle, dirac.ChGenConfirm, dirac.ChGenPeekReveal,
		dirac.ChGenDiscUnless:
		// yes / pay / top / unless first: gorge's first offer, which the
		// kernel's canonical order puts second (mode 0 sorts first)
		if i := modeIndex(d, 1); i >= 0 {
			return i
		}
		return 0
	case dirac.ChGenManaChoice:
		b.fill(s, bt.f, d.Player)
		if i := bt.manaColour(d); i >= 0 {
			return i
		}
		return 0
	case dirac.ChGenType:
		return bt.firstBy(s, d, func(a dirac.Action) uint32 {
			return s.FeatTypeKey(d.Player, int(a.Target())<<8|int(a.Mode()))
		})
	case dirac.ChGenConvoke:
		b.fill(s, bt.f, d.Player)
		return bt.castHelper(s, d)
	case dirac.ChWaterbend, dirac.ChCostTap, dirac.ChGenPayTap, dirac.ChWindowTap:
		// policy.go's tapHelperChoices: the first offer that makes no mana, since
		// one that does may be needed for the rest of the payment and can't do both
		b.fill(s, bt.f, d.Player)
		return bt.firstBy(s, d, func(a dirac.Action) uint32 {
			key := bt.objKey(s, dirac.Obj(a.Source()))
			if f := b.cardOf(dirac.Obj(a.Source())).f; f != nil && f.MakesMana {
				key |= 1 << 31
			}
			return key
		})
	case dirac.ChGenCastFree:
		// gorge's KModes over the castable cards: the first Min, so none when optional
		if i := doneIndex(d); i >= 0 {
			return i
		}
		return bt.firstBy(s, d, func(a dirac.Action) uint32 { return uint32(a.Source()) })
	case dirac.ChGenChooseCard:
		return bt.firstBy(s, d, func(a dirac.Action) uint32 { return uint32(a.Source()) })
	case dirac.ChGenDivide, dirac.ChCastDivide:
		return bt.firstBy(s, d, func(a dirac.Action) uint32 { return uint32(s.FeatDivideIndex(a.Source())) })
	case dirac.ChGenReplOrder:
		b.fill(s, bt.f, d.Player)
		best := -1
		var bw int32
		for i := 0; i < int(d.N); i++ {
			a := d.Options[i]
			w := b.cardWorth(dirac.Obj(a.Source()))
			if best < 0 || w > bw || w == bw && a.Mode() < d.Options[best].Mode() {
				best, bw = i, w
			}
		}
		if best < 0 {
			return 0
		}
		return best
	}
	switch d.Sub {
	case dirac.ChGenPip:
		return firstPip(d)
	case dirac.ChDivide, dirac.ChGenBranch, dirac.ChGenPileChoose, dirac.ChGenColor, dirac.ChAltAdd, dirac.ChGenGift, dirac.ChCostForage, dirac.ChGenKeepTapped:
		return firstMode(d)
	}
	if bt.fetchNeed && d.Sub == dirac.ChGenSearch {
		if i := bt.fetchLand(s, d); i >= 0 { // fetch.go
			return i
		}
	}
	// search / dig / hidden pick / hand move / piles / keep / cards /
	// enchant / cost taps / counters: the first offer in gorge's order
	return bt.firstObj(s, d)
}

// facingLethal is policy.go's facingLethal: the untapped opposing
// creatures' power reaches p's life.
func (b *board) facingLethal(p uint8) bool {
	var total int32
	for i := 0; i < b.nCre; i++ {
		c := &b.cre[i]
		if c.ctrl == p || c.tapped || c.power <= 0 {
			continue
		}
		total += c.power
	}
	return total >= b.life[p&1]
}

// modes is KModes: the first Min options in order, so nothing more once the
// kernel offers Done (an optional pick starts there).
func (bt *Bot) modes(s *dirac.State, d *dirac.Decision) int {
	if i := doneIndex(d); i >= 0 {
		return i
	}
	return firstMode(d)
}

// triggerOrder is trigger.go's chooseTriggerOrder: the most valuable
// source first, ties to gorge's detection order.
func (bt *Bot) triggerOrder(s *dirac.State, d *dirac.Decision) int {
	b := &bt.b
	b.fill(s, bt.f, d.Player)
	best := -1
	var bw int32
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() != dirac.AkOrder {
			continue
		}
		w := b.cardWorth(dirac.Obj(a.Source()))
		if best < 0 || w > bw || w == bw && a.Target() < d.Options[best].Target() {
			best, bw = i, w
		}
	}
	if best < 0 {
		return 0
	}
	return best
}

// mulligan keeps (a search never reaches a mulligan; self-play runs its own
// mulligan rule).
func (bt *Bot) mulligan(s *dirac.State, d *dirac.Decision) int {
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == dirac.AkKeep {
			return i
		}
	}
	return 0
}

// fillPos refreshes the battlefield positions objKey reads (without the
// whole board).
func (b *board) fillPos(s *dirac.State) {
	for p := uint8(0); p < 2; p++ {
		for i, o := range s.FeatBattlefield(p, b.objs[:0]) {
			b.bfPos[o] = uint8(i)
		}
	}
}

// firstNoMana is policy.go's tapHelperChoices for one pick: the first offer that
// makes no mana, since one that does may be needed for the rest of the payment
// and can't do both. The options are ordered by objKey, a mana maker's after
// every other.
func (bt *Bot) firstNoMana(s *dirac.State, d *dirac.Decision) int {
	return bt.firstBy(s, d, func(a dirac.Action) uint32 {
		key := bt.objKey(s, dirac.Obj(a.Source()))
		if f := bt.b.cardOf(dirac.Obj(a.Source())).f; f != nil && f.MakesMana {
			key |= 1 << 31
		}
		return key
	})
}

// castHelper serves a Convoke, Improvise or Harmonize announcement. Gorge's bot
// answers it as one set, so each step recomputes that set from the first offer
// and takes its next member, then Done.
func (bt *Bot) castHelper(s *dirac.State, d *dirac.Decision) int {
	// chooseHarmonize: the largest power, ties to the first offered
	best, bp := -1, int16(0)
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == dirac.AkDone || a.Mode() != dirac.ConvokeHarmonizeMode {
			continue
		}
		o := dirac.Obj(a.Source())
		pw, _ := s.PowerToughness(o)
		if best < 0 || int16(pw) > bp || int16(pw) == bp && bt.objKey(s, o) < bt.objKey(s, dirac.Obj(d.Options[best].Source())) {
			best, bp = i, int16(pw)
		}
	}
	if best >= 0 {
		return best
	}
	var picked [dirac.MaxPerm]dirac.Obj
	var pips [dirac.MaxPerm]int8
	n := bt.castHelperSet(s, &picked, &pips)
	for i := 0; i < n; i++ {
		if s.FeatConvokePicked(picked[i]) {
			continue
		}
		return castHelperOffer(s, d, picked[i], pips[i])
	}
	if i := doneIndex(d); i >= 0 && (n > 0 || s.FeatConvokePickCount() > 0) {
		return i
	}
	return bt.firstNoMana(s, d)
}

// castHelperOffer is policy.go's helperOffer: the option that commits o to the
// pip, else o's first.
func castHelperOffer(s *dirac.State, d *dirac.Decision, o dirac.Obj, pip int8) int {
	first := -1
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == dirac.AkDone || dirac.Obj(a.Source()) != o {
			continue
		}
		if m := a.Mode(); pip >= 0 && m == uint8(pip) || pip < 0 && m >= 5 {
			return i
		}
		if first < 0 {
			first = i
		}
	}
	if first >= 0 {
		return first
	}
	if i := doneIndex(d); i >= 0 {
		return i
	}
	return 0
}

// castHelperSet is policy.go's castHelperChoices: the permanents that make no
// mana and leave the rest of the printed cost payable, with the pip each pays
// (-1 one generic), in offer order. 0: the seat's mana pays alone, or they don't
// reach it.
func (bt *Bot) castHelperSet(s *dirac.State, out *[dirac.MaxPerm]dirac.Obj, pip *[dirac.MaxPerm]int8) int {
	b := &bt.b
	_, _, _, ok := s.Building()
	if !ok {
		return 0
	}
	cf := bt.f.card(s.ObjCard(s.FeatSource()))
	var pips [5]int32
	generic := int32(cf.CMC)
	for i, n := range cf.Pips {
		pips[i] = int32(n)
		generic -= int32(n)
	}
	if b.manaReaches(pips, generic) {
		return 0
	}
	var offers [dirac.MaxPerm]dirac.Obj
	cands, _ := s.FeatConvokeCands(offers[:0])
	convoke, _ := s.FeatConvokeKinds()
	n := 0
	reached := false
	for _, o := range cands {
		if cf := b.cardOf(o).f; cf != nil && cf.MakesMana {
			continue
		}
		slot := int8(-1)
		if convoke {
			colours := s.FeatColorsOf(o)
			for c, m := range pips {
				if m > 0 && colours&(1<<c) != 0 {
					slot = int8(c)
					break
				}
			}
		}
		switch {
		case slot >= 0:
			pips[slot]--
		case generic > 0:
			generic--
		default:
			continue
		}
		out[n], pip[n] = o, slot
		n++
		if b.manaReaches(pips, generic) {
			reached = true
			break
		}
	}
	if !reached {
		return 0
	}
	m := 0
	for i := 0; i < n; i++ {
		backPips, backGeneric := pips, generic
		if pip[i] >= 0 {
			backPips[pip[i]]++
		} else {
			backGeneric++
		}
		if b.manaReaches(backPips, backGeneric) {
			pips, generic = backPips, backGeneric
			continue
		}
		out[m], pip[m] = out[i], pip[i]
		m++
	}
	return m
}

// manaColour is tap.go's chooseManaColour on a resolving mana effect's colour
// pick: the first offered colour the card the tap gate would aim at still needs,
// or -1. The ask carries no list of potential casts, so every castable card in
// hand is a candidate.
func (bt *Bot) manaColour(d *dirac.Decision) int {
	b := &bt.b
	var off [5]bool
	for i := 0; i < int(d.N); i++ {
		if m := d.Options[i].Mode(); m < 5 {
			off[m] = true
		}
	}
	c := bt.bestUnpayableOf(d, off, false)
	if c == nil {
		return -1
	}
	for i := 0; i < int(d.N); i++ {
		if m := d.Options[i].Mode(); m < 5 && int32(c.f.Pips[m]) > b.pool[m] {
			return i
		}
	}
	return -1
}
