package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A mana ability a GainsAbilitiesOf$ recipient has (CR 613.1f, "has all
// activated abilities of ...", CR 605.1a): Marvin, Murderous Mimic beside a
// creature that taps for mana taps for that mana itself, the {T} cost paid by
// the recipient (CR 602.2). The planner reads it as a granted mana ability:
// prepareGen registers each foreign card's plain printed mana ability as a
// genPrep.grantPC entry (gainedManaPlanned), and grantIdxs lists it, behind the
// statics' own grants, for each recipient of a battlefield GainsOf static
// whose filter matches a foreign permanent now, and of a GainsAbilitiesOfZones$
// Graveyard/Exile static whose filter matches a card in that zone (gainedIdxs,
// the zone arms of static_gainsof.go's gainedFrom). The offer walk
// (static_gainsof.go) never offers one, a mana ability being no priority
// option; gorge lists it in its payment census (grantedAbilitiesLimited).
//
// A restricted ability (Overgrown Zealot's face-up one, Creeping Peeper's) is
// a grant with a spend restriction (genPrep.grantRestr), as a static grant's.
// Not modelled, so a census gap: a dynamic, chosen-colour, reflected,
// each-colour, deferred or any-combination ability. A limited one (gainedManaLimited) stalls StallGainsOf
// in the offer walk.

// useGainedMana arms gainedIdxs for a table with a GainsAbilitiesOf$ static
// and a foreign printed mana ability the planner models (gainedManaPlanned):
// grantPC then holds a printed entry, and grantMay/grantsLive are true. It is
// a genPrep.uses bit, never a new genPrep bool (genneed.go).
var useGainedMana = newGenUse(tableHasGainedMana)

// tableHasGainedMana reports whether some printed mana ability is a planner
// unit for a GainsOf recipient (gainedManaPlanned), for a table with a
// GainsAbilitiesOf$ static.
func tableHasGainedMana(t *kop.Table) bool {
	if !tableHasGainsOf(t) {
		return false
	}
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if gainedManaPlanned(&t.Cards[i].Abilities[j]) {
				return true
			}
		}
	}
	return false
}

// isGrantMana reports whether mana ability a is a grantPC entry: a latent one
// (a static's AddAbility$ grant), or in a table with a GainsAbilitiesOf$
// static a plain printed one (gainedManaPlanned).
func (g *genPrep) isGrantMana(a *kop.Ability) bool {
	return a.AFlags&kop.AfLatent != 0 || g.uses.has(useGainsOf) && gainedManaPlanned(a)
}

// addGrantPC registers mana ability a of card c (index ab) as a grantPC entry
// with view gp. A printed one (a GainsAbilitiesOf$ recipient's: prepareGen
// registers it for a table with such a static) is what useGainedMana marks,
// and turns the availFor dual census on.
func (g *genPrep) addGrantPC(gp pcard, self bool, a *kop.Ability, c CardID, ab uint8) {
	g.grantSelf = append(g.grantSelf, self)
	g.grantPC = append(g.grantPC, gp)
	g.grantRestr = append(g.grantRestr, a.ManaRestrict)
	g.grantOf = append(g.grantOf, grantRef{c, ab})
	if a.AFlags&kop.AfLatent == 0 {
		g.anyContGrant = true
		g.contGrantAlways = true
	}
}

// gainedManaIntrinsic reports whether the foreign card's mana ability a is the
// CR 305.6 intrinsic of one of its basic land subtypes: a bare {T} making one
// mana of exactly the subtype's colour. gorge's census holds exactly the
// gained abilities whose line is the parser's "intrinsic: basic land mana"
// (pay/board.go PaymentAbility's intrinsic branch), which is the intrinsic one;
// a foreign authored ability with the same shape (the corpus holds none:
// Murmuring Bosk's authored {T}: Add {W} or {B} beside its Forest's intrinsic
// is a two-colour choice) is a foreign printed line gorge keeps out.
func gainedManaIntrinsic(cd *kop.CardDef, a *kop.Ability, subs []string) bool {
	if a.Kind != kop.AbMana || a.Cost.Flags != kop.CostTap || a.ManaN != 1 ||
		a.ManaColors&(a.ManaColors-1) != 0 || a.ManaColors&^kop.ColAny != 0 {
		return false
	}
	for i, name := range blgNames {
		if a.ManaColors != kop.ColorMask(1)<<uint(i) {
			continue
		}
		for j, n := range subs {
			if n == name && cd.Subtypes.Has(j) {
				return true
			}
		}
	}
	return false
}

// ungrGainedIntr credits slot sl's gained mana abilities whose foreign
// ability is a CR 305.6 intrinsic (read from the grant's own card, never a
// pinned genPrep field: TestNoNewFixedStateFields) to the ungranted census av:
// gorge's census holds a gained ability whose line is the parser's
// "intrinsic: basic land mana" as a plain unit of the recipient
// (pay/board.go PaymentAbility, CR 613.1f GainsAbilitiesOf$ beside an
// animated Forest), while a static AddAbility$ grant and a gained
// non-intrinsic one (a foreign authored line) stay out of gorge's planner
// (the V1 exclusion FeatPlannedCast prices over). It runs before ungrAvail's
// own-unit skips, so the unit lists beside the slot's own and without it.
func (s *State) ungrGainedIntr(k *prep, p uint8, sl Slot, av *avail, ms *modalSet) {
	g := k.gen
	if g == nil || len(g.grantPC) == 0 || !g.uses.has(useGainedMana) {
		return
	}
	var gi [maxGrants]int16
	n := s.grantIdxs(k, sl, &gi)
	for i := 0; i < n; i++ {
		j := gi[i]
		r := g.grantOf[j]
		a := &k.t.Cards[r.card].Abilities[r.ab]
		if g.grantRestr[j] != 0 || a.AFlags&kop.AfLatent != 0 || !gainedManaPlanned(a) ||
			!gainedManaIntrinsic(&k.t.Cards[r.card], a, k.t.Subtypes) {
			continue
		}
		gp := s.grantView(g, j, sl)
		if !s.canTapForManaPC(k, sl, gp) {
			continue // the {T} cost: a tapped, lost or once-blocked source makes nothing (CR 302.6)
		}
		av.addUnits(s.srcMaskAt(gp, sl), s.srcYieldAt(k, sl, gp))
	}
}

// gainedManaPlanned reports whether foreign mana ability a is a planner unit
// for a GainsOf recipient: an unlimited printed ability whose colours and
// amount are fixed (a spend restriction is the grant's own, grantRestr).
func gainedManaPlanned(a *kop.Ability) bool {
	const unmodelled = kop.AfLatent | kop.AfManaDynamic | kop.AfManaChosen | kop.AfManaReflect | kop.AfManaEachColor
	return a.Kind == kop.AbMana && a.AFlags&unmodelled == 0 && !gainedManaLimited(a) &&
		!a.ManaDeferred && a.ManaColors != 0 && !comboAnyMana(a)
}

// gainedManaLimited reports whether foreign mana ability a carries a limit
// the planner keeps no record of: once per game or turn, a loyalty cost,
// exhaust, a host cost, or an activation zone.
func gainedManaLimited(a *kop.Ability) bool {
	const limits = kop.CostOncePerGame | kop.CostOncePerTurn | kop.CostLoyalty
	return a.Kind == kop.AbMana && (a.Cost.Flags&limits != 0 || a.Exhaust || a.Cost.Host != 0 || a.ActZone != kop.ZoneNone)
}

// gainedIdxs appends to out (n entries) the genPrep.grantPC indices of the
// mana abilities sl gains from a GainsOf static, in the statics' walk order
// and, for a battlefield static, each foreign permanent's battlefield order;
// for a GainsAbilitiesOfZones$ Graveyard/Exile static each seat's zone in
// seat order, each card in stored order (gainedZoneIdxs). Returns the new
// count. A recipient that lost all abilities gains nothing.
//
//go:noinline
func (s *State) gainedIdxs(k *prep, sl Slot, out *[maxGrants]int16, n int) int {
	if s.genLost(k, sl) {
		return n
	}
	g := k.gen
	var order, foreign [MaxPerm]Slot
	no := s.listenerOrder(s.statM, &order)
	nf := -1
	for oi := 0; oi < no; oi++ {
		src := order[oi]
		if s.genLost(k, src) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(src)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous || a.Static.GainsOf == 0 {
				continue
			}
			st := statSrc{a: a, sl: src, ctrl: s.perms[src].ctrl, obj: s.perms[src].obj}
			if !s.statCond(k, st) || !s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerKw) {
				continue
			}
			if a.Static.Zone == kop.ZoneGraveyard || a.Static.Zone == kop.ZoneExile {
				n = s.gainedZoneIdxs(k, st, g, out, n)
				continue
			}
			if nf < 0 {
				nf = s.listenerOrder(s.occ, &foreign)
			}
			for fi := 0; fi < nf; fi++ {
				fs := foreign[fi]
				if s.faceDown(k, fs) || !s.matchPermGen(k, a.Static.GainsOf, fs, st.ctrl, st.sl, layerKw) {
					continue
				}
				fc := s.cardAt(fs)
				for gi, r := range g.grantOf {
					if r.card != fc || k.t.Cards[fc].Abilities[r.ab].AFlags&kop.AfLatent != 0 {
						continue
					}
					if n == maxGrants {
						s.stall(StallGrants) // capacity: loud, never a dropped grant
						return n
					}
					out[n] = int16(gi)
					n++
				}
			}
		}
	}
	return n
}

// gainedZoneIdxs is gainedIdxs's arm for a GainsAbilitiesOfZones$
// Graveyard/Exile static (the zone arms of static_gainsof.go's gainedFrom):
// every seat's zone in seat order, each card in stored order, one grant
// unit per matching card the way the battlefield arm matches each foreign
// permanent. ExiledWithSource matches through the static's source (the
// exiled-with list, exwFlag), exactly as the offer walk's exile arm reads
// it.
//
//go:noinline
func (s *State) gainedZoneIdxs(k *prep, st statSrc, g *genPrep, out *[maxGrants]int16, n int) int {
	ctx := evalCtx{src: st.obj, srcSl: st.sl, ctrl: st.ctrl}
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		pl := &s.pl[p]
		zn := int(pl.graveN)
		if st.a.Static.Zone == kop.ZoneExile {
			zn = int(pl.exileN)
		}
		for i := 0; i < zn; i++ {
			o := pl.grave[i]
			if st.a.Static.Zone == kop.ZoneExile {
				o = pl.exile[i]
			}
			card := s.objs[o].card
			if !s.matchCardGenAt(k, st.a.Static.GainsOf, card, p, st.ctrl, o, &ctx) {
				continue
			}
			for gi, r := range g.grantOf {
				if r.card != card || k.t.Cards[card].Abilities[r.ab].AFlags&kop.AfLatent != 0 {
					continue
				}
				if n == maxGrants {
					s.stall(StallGrants) // capacity: loud, never a dropped grant
					return n
				}
				out[n] = int16(gi)
				n++
			}
		}
	}
	return n
}
