package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A mana ability a GainsAbilitiesOf$ recipient has (CR 613.1f, "has all
// activated abilities of ...", CR 605.1a): Marvin, Murderous Mimic beside a
// creature that taps for mana taps for that mana itself, the {T} cost paid by
// the recipient (CR 602.2). The planner reads it as a granted mana ability:
// prepareGen registers each foreign card's plain printed mana ability as a
// genPrep.grantPC entry (gainedManaPlanned), and grantIdxs lists it, behind the
// statics' own grants, for each recipient of a battlefield GainsOf static
// whose filter matches a foreign permanent now (gainedIdxs). The offer walk
// (static_gainsof.go) never offers one, a mana ability being no priority
// option; gorge lists it in its payment census (grantedAbilitiesLimited).
//
// A restricted ability (Overgrown Zealot's face-up one, Creeping Peeper's) is
// a grant with a spend restriction (genPrep.grantRestr), as a static grant's.
// Not modelled, so a census gap: a dynamic, chosen-colour, reflected,
// each-colour, deferred or any-combination ability, and one a graveyard or
// exile GainsOf reads. A limited one (gainedManaLimited) stalls StallGainsOf
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
// mana abilities sl gains from a battlefield GainsOf static, in the statics'
// walk order and each foreign permanent's battlefield order, and returns the
// new count. A recipient that lost all abilities gains nothing.
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
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous || a.Static.GainsOf == 0 ||
				a.Static.Zone == kop.ZoneGraveyard || a.Static.Zone == kop.ZoneExile {
				continue
			}
			st := statSrc{a: a, sl: src, ctrl: s.perms[src].ctrl, obj: s.perms[src].obj}
			if !s.statCond(k, st) || !s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerKw) {
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
