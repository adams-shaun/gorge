package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A sacrifice-another mana source (kop.SacXMana: Krark-Clan Ironworks'
// "Sacrifice an artifact: Add {C}{C}", kop.SacAnyManaAb, or Bolg's Company's
// "{T}, Sacrifice another Goblin: Add {B}{R}", kop.SacXEachManaAb) beside a
// granted mana ability (Abundant Growth on a land): gorge's census defers the
// sacrifice part (a Sac part that is not the source) and its one prefix
// search (potential_script.go scriptSearch.visit) activates every census gap
// in battlefield order, the sacrifice source and the granted sources among
// them, so a cast only their mana together pays (Brood Monitor's {4}{G}{G}
// from the {C}{C}, the granted {G} and two lands) is found through it. Neither
// kernel search alone prices that play: the tap-another search's walk runs
// without the grants (mana_tapanother.go tapXScript's noGrant), and the grant
// search (grantSrch) left the sacrifice sources to it (mana_granttapx.go).
// So the grant search lists the sacrifice source as a candidate of its own
// (grantSacXGaps), the way it lists the tap-another one (mana_granttapx.go)
// and the Taps-deferred one (mana_granttapsdefer.go): the permanent to
// sacrifice (pay.ManaSacrificeCandidates' ask, only with two or more
// candidates, battlefield order; the source itself a candidate of
// kop.SacAnyManaAb, Krark-Clan Ironworks among them), then its fixed
// production, no colour ask (tapXUnit), as the tap-another search's
// sacrifice branch does (mana_tapanother.go visit). A grant holder whose own
// ability is a sacrifice-another shape stays on the grant side: its granted
// abilities branch first (visit), and its sacrifice shape is modelled on no
// board in the corpus.

// stepSacX is the gi of a prefix step that activates a candidate's
// sacrifice-another mana ability (cand the permanent it sacrifices; the
// production is fixed, no colour).
const stepSacX int16 = -8

// grantSacXGaps: p's sacrifice-another mana sources the tap-another search
// models (scriptTapXModelled), a slot mask, for a play the grant search
// prices. tapXOffGrant excludes them (the grant search covers a source the
// tap-another search alone would keep, mana_granttapx.go).
func (s *State) grantSacXGaps(k *prep, p uint8) slotMask {
	var m slotMask
	if !k.tapMana {
		return m
	}
	for b := s.scriptTapXModelled(k, p); b.any(); b = b.rest() {
		sl := b.first()
		if kop.SacXMana(k.tapXAbOf(s.cardAt(sl))) {
			m = m.with(sl)
		}
	}
	return m
}

// sacXBesideGrant: p has a live sacrifice-another mana source beside a live
// granted mana ability: gorge's one prefix search activates both, so a play
// the tap-another search alone cannot pay takes the grant search
// (script_gen.go scriptOffers).
func (s *State) sacXBesideGrant(k *prep, p uint8) bool {
	return s.grantSacXGaps(k, p).any() && s.liveGrantGap(k, p)
}

// sacXBranch activates candidate i's (slot sl) sacrifice-another mana ability
// on copies of t: each permanent its cost may sacrifice (the ask only with
// two or more candidates, battlefield order), its fixed production.
func (g *grantSrch) sacXBranch(t *State, i int, sl Slot) bool {
	if !t.occ.has(sl) || t.perms[sl].obj != g.cobj[i] {
		return false // sacrificed by an earlier step (mana_grantcost.go)
	}
	ab := g.k.tapXAbOf(t.cardAt(sl))
	if ab == nil || !t.scriptTapX(g.k, g.p).has(sl) {
		return false // tapped or its candidates gone since the root
	}
	var cs [MaxPerm]Slot
	nc := t.sacXCands(g.k, g.p, sl, ab, &cs)
	if nc == 0 {
		return false
	}
	if nc > 1 && !g.node() { // the sacrifice ask
		return false
	}
	for j := 0; j < nc; j++ {
		if g.np >= len(g.path) {
			return false
		}
		u := t.fork()
		u.tapXDo(g.k, g.p, tapXStep{sl: sl, cand: cs[j], cand2: NoSlot, kind: tapXAct})
		g.path[g.np] = grantStep{i: uint8(i), gi: stepSacX, sl: sl, cand: cs[j]}
		g.np++
		ok := g.next(u, sl) // (the sacrifice queues its dies triggers, mana_granttapsdefer.go)
		t.unfork()
		if ok {
			return true
		}
		g.np--
	}
	return false
}
