package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Granted mana abilities in the cast offers (tokmana). A Continuous
// static's AddAbility$ grant of a mana ability reaches permanents its
// filter matches anywhere on the battlefield (Springleaf Parade's "Creature
// tokens you control have '{T}: Add one mana of any color'", Cryptolith
// Rite, Chromatic Lantern's lands, Gemhide Sliver's Slivers of either
// player), not only the permanent the static's source is attached to
// (grantedAbilitiesN's grantM walk: an Aura's or Equipment's host). The
// grant walk below adds those (contGrants), so manaPCGen / manaGranted see
// the granted source exactly as Imprisoned in the Moon's land.
//
// gorge (pin 81dd3f940) lists the granted ability in every mana walk
// (rules mana_activation.go appendAvailableManaAbilitiesGateCompute: the
// Continuous AddAbility$ scan, then grantedAbilities), but its payment
// planner never witnesses one (pay/board.go PaymentAbility excludes grants), so the granted source is a census gap
// (potential_plan.go paymentPlanCensusOf): a play the printed sources
// cannot pay gets a prefix search (potential_script.go
// potentialPrefixScript) that activates the gap sources first, branching
// over every answer (the colour of an "Any" ability, W U B R G in order)
// and visiting each distinct payer state once within potentialScriptNodes
// clones; the play is offered when the search finds a prefix after which
// the pool and the printed sources pay it, and the oracle plays that prefix
// before the play (prScript). grantSearch copies that search: the kernel's
// offer (grantCap) keeps only a play the search pays, and the play taps the
// found prefix into the pool (grantPrefix) before its own payment.
//
// A permanent with a printed mana ability AND a granted one (Llanowar Elves
// under Cryptolith Rite; every land under Chromatic Lantern) is a printed
// source for the planner (manaPCGen returns its own ability: gorge's
// census covers the printed ability, the granted one is the gap) and a
// prefix candidate whose activation first offers the printed ability, then
// the granted one (gorge's "choose a mana ability" ask, printed first).
//
// Shapes outside "{T}: Add <fixed colours or any> x N" (a spend
// restriction, a mana fee, a dynamic amount, a sacrifice or life part) are
// refused by kgen (dirac/kgen/lower_grantedmana.go grantedManaShape).

// (potentialScriptNodes, the node budget, is mana_filter.go's.)

// prepTokMana marks the cards with a Continuous grant of a mana ability
// whose filter is not limited to the source's host (contGrant).
func prepTokMana(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		c := &t.Cards[i]
		for j := range c.Abilities {
			a := &c.Abilities[j]
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous || a.Static.Grant == 0 {
				continue
			}
			if int(a.Static.Grant-1) >= len(c.Abilities) || c.Abilities[a.Static.Grant-1].Kind != kop.AbMana {
				continue
			}
			if filterAttachedOnly(t, a.Static.Filter) {
				g.anyContGrant = true // availFor counts the grant beside the host's own ability (New Horizons' Forest)
				continue              // the grantM walk's (an Aura's / Equipment's host)
			}
			g.setFlag(CardID(i), gfContGrant)
			g.anyContGrant = true
		}
	}
}

// contGrants calls f for the mana abilities unattached Continuous grants
// give sl: each contGrant static in force whose condition holds and whose
// filter matches sl (a grant older than a "loses all abilities" effect on
// sl is removed with the rest). A static attached to sl is the grantM
// walk's.
//
//go:noinline
func (s *State) contGrants(k *prep, sl Slot, lost bool, lts uint16, f func(c CardID, ab uint8, n int)) {
	// gorge lists the grants in its static walk's order (collectActionStatics:
	// each seat's battlefield in entry order), which the activation ask
	// and the prefix search follow (the first grant's colours are tried
	// first): the sources of player 0 then player 1, each in entry order
	var order [MaxPerm]Slot
	n := s.battlefieldOrder(0, s.statM.and(s.ctrlM[0]), order[:], 0)
	n = s.battlefieldOrder(1, s.statM.and(s.ctrlM[1]), order[:], n)
	for oi := 0; oi < n; oi++ {
		src := order[oi]
		cid := s.cardAt(src)
		if !k.gen.has(cid, gfContGrant) || s.perms[src].attached == sl || s.genLost(k, src) {
			continue
		}
		if lost && s.layerTs(src) < lts {
			continue
		}
		cd := &k.t.Cards[cid]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous || a.Static.Grant == 0 || cd.Abilities[a.Static.Grant-1].Kind != kop.AbMana {
				continue
			}
			st := statSrc{a: a, sl: src, ctrl: s.perms[src].ctrl, obj: s.perms[src].obj}
			if !s.statCond(k, st) || !s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, src, layerKw) {
				continue
			}
			prev := s.g.grantSrc
			s.g.grantSrc = uint8(st.obj) + 1 // grantIdxs orders by this source's place in the walk
			f(cid, a.Static.Grant-1, 1)
			s.g.grantSrc = prev
		}
	}
}

// maxGrants bounds the granted mana abilities read on one permanent (two
// Gemhide Slivers grant a Sliver two; the tokmana pool reaches 10 on a
// changeling token); one more stalls (StallGrants).
const maxGrants = 16

// grantIdxs lists the genPrep.grantPC indices of the mana abilities effects
// grant sl, in the grant walk's order, one per granting instance (two
// Gemhide Slivers give a Sliver two abilities, as gorge lists them: each
// is its own activation choice); n is their count.
//
//go:noinline
func (s *State) grantIdxs(k *prep, sl Slot, out *[maxGrants]int16) (n int) {
	g := k.gen
	// gorge lists every Continuous AddAbility$ grant in its static walk's
	// order (seat, then entry), an attached source's among the unattached
	// ones (an Honest Work's "{T}: Add {C}" on a land of the other seat
	// follows a Great Divide Guide's grant of the first); grantedAbilities
	// reports the attached statics first, so the list is stably reordered
	// by the granting source's place in that walk. A grant without a
	// source in it (an Animate record) keeps its place after them.
	var order [MaxPerm]Slot
	no := 0
	if g.anyContGrant {
		no = s.battlefieldOrder(0, s.statM.and(s.ctrlM[0]), order[:], 0)
		no = s.battlefieldOrder(1, s.statM.and(s.ctrlM[1]), order[:], no)
	}
	var key [maxGrants]uint8
	s.grantedAbilities(k, sl, func(c CardID, ab uint8) {
		pos := uint8(255)
		for oi := 0; oi < no; oi++ {
			if s.g.grantSrc != 0 && uint8(s.perms[order[oi]].obj)+1 == s.g.grantSrc {
				pos = uint8(oi)
				break
			}
		}
		for i, r := range g.grantOf {
			if r.card != c || r.ab != ab {
				continue
			}
			if n == maxGrants {
				s.stall(StallGrants) // capacity: loud, never a dropped grant
				continue
			}
			if g.grantSelf[i] && s.selfColMask(sl) == 0 {
				continue // a colourless grantee reflects nothing: gorge lists no such ability
			}
			j := n
			for j > 0 && key[j-1] > pos {
				out[j], key[j] = out[j-1], key[j-1]
				j--
			}
			out[j], key[j] = int16(i), pos
			n++
		}
	})
	return n
}

// grantIdx is the genPrep.grantPC index of the first mana ability an effect
// grants sl, -1 when none.
//
//go:noinline
func (s *State) grantIdx(k *prep, sl Slot) int {
	var gi [maxGrants]int16
	if s.grantIdxs(k, sl, &gi) == 0 {
		return -1
	}
	return int(gi[0])
}

// ownManaUsable reports whether sl's own card has a usable tap mana
// ability (one the planner prices: a plain or sacrificing tap, a chosen
// colour, a dynamic amount) that it has not lost. Beside a granted mana
// ability it is the planner's source (gorge's census covers it); the
// grant is the prefix search's alternative (grantAlt).
//
//go:noinline
func (s *State) ownManaUsable(k *prep, sl Slot) bool {
	c := s.cardAt(sl)
	if k.gen.flags[c]&gfManaGate != 0 {
		// a gated source's own ability is the gate's view, not the
		// variant flag: a gate-off single gated source has no mana ability
		// (the grant is then the source's only unit, as gorge offers it).
		pc := s.manaGateView(k, sl, c, k.gen.manaVar(c))
		return pc.manaAb >= 0 && (pc.manaTap || pc.manaSac || pc.manaSacOnly) && !s.genLost(k, sl)
	}
	pc := &k.cards[c]
	if !(pc.manaAb >= 0 && (pc.manaTap || pc.manaSac || pc.manaSacOnly) || k.gen.flags[c]&gfManaVar != 0) {
		return false
	}
	return !s.genLost(k, sl)
}

// windowGrantMask is the colours of the mana abilities effects grant sl,
// joined: what the cast window's clamp reads beside the source's own ability.
//
//go:noinline
func (s *State) windowGrantMask(k *prep, sl Slot) (mask uint8) {
	var gi [maxGrants]int16
	for i, n := 0, s.grantIdxs(k, sl, &gi); i < n; i++ {
		if g := s.grantView(k.gen, gi[i], sl); s.grantCostPayable(s.perms[sl].ctrl, g) {
			mask |= s.srcMaskAt(g, sl) // a life its controller cannot pay activates nothing (CR 119.4)
		}
	}
	return mask
}

// availGrant adds slot sl's unit(s) to a (availFor) on a table with
// Continuous mana grants: its own ability's colours (pc) joined with
// every granted ability's, one unit (the permanent taps once; gorge's
// potential walk lists a play any prefix pays). Under availWalk -- the
// walk grantCap counts gorge's prefix searches over -- the relaxed census
// instead: gorge's unpayability proof (potential_plan.go
// paymentPlanRelaxedAlternatives) relaxes EACH granted ability of a source
// to its own free unit beside the printed one, so a play those units pay
// takes one of the four searches even when no real sequence pays it.
//
//go:noinline
func (s *State) availGrant(k *prep, sl Slot, pc *pcard, a *avail) {
	mask, n := s.srcMaskAt(pc, sl), pc.manaN
	var gi [maxGrants]int16
	ng := s.grantIdxs(k, sl, &gi)
	own := s.ownManaUsable(k, sl)
	if a.xdOn {
		if n := s.grantsAdmitted(k, sl, &gi, ng, &a.xd); n < ng {
			if ng = n; ng == 0 && !own {
				return // the source made mana only by a grant the payment cannot spend
			}
		}
	}
	if ng == 0 && k.painMana && pc.painCol != 0 {
		s.addPainAvail(k, sl, pc, s.perms[sl].ctrl, a, nil, false)
		return
	}
	if ng == 0 || (!own && ng == 1) {
		s.addGrantUnits(a, mask, n)
		if ng == 0 && pc.manaOne {
			s.addOneColour(a, n)
		}
		s.availGrantExtra(k, sl, pc, &gi, ng, own, a)
		return
	}
	if s.g.availX&availWalk != 0 {
		if own {
			s.addGrantUnits(a, mask, n)
		}
		for i := 0; i < ng; i++ {
			pc := s.grantView(k.gen, gi[i], sl)
			s.addGrantUnits(a, s.srcMaskAt(pc, sl), pc.manaN)
		}
		s.availGrantExtra(k, sl, pc, &gi, ng, own, a)
		return
	}
	if !own {
		mask, n = 0, 0
	}
	for i := 0; i < ng; i++ {
		pc := s.grantView(k.gen, gi[i], sl)
		mask |= s.srcMaskAt(pc, sl)
		n = max(n, pc.manaN)
	}
	s.addGrantUnits(a, mask, n)
	s.availGrantExtra(k, sl, pc, &gi, ng, own, a)
}

// availGrantExtra adds the extra mana a tap of sl brings (mana_tapextra.go) to
// a: gorge prices it on the printed ability, the planner's alternative, and the
// ability a prefix search activates floats the same extra, so the unit carries it
// once. The extra counts only while every ability of the source agrees on it
// (a carrier limited to a {C} tap reads the pick), and only a source whose own
// ability is usable carries it: the relaxed alternatives of a granted ability
// bring none (potential_plan.go paymentPlanRelaxedAlternatives), and a source
// with nothing but a grant is not priced here, so it stalls.
// grantsAdmitted keeps the first ng granted mana abilities of the source at sl
// that d may spend (a restricted grant refused by d is no unit of the window),
// returning how many remain.
func (s *State) grantsAdmitted(k *prep, sl Slot, gi *[maxGrants]int16, ng int, d *payDesc) int {
	n := 0
	for i := 0; i < ng; i++ {
		if r := k.gen.grantOf[gi[i]]; k.gen.grantRestr[gi[i]] != 0 && !s.restrSrcAdmits(k, s.perms[sl].ctrl, sl, &k.t.Cards[r.card].Abilities[r.ab], d) {
			continue
		}
		gi[n] = gi[i]
		n++
	}
	return n
}

// walkHandAvail is the potential walk's census for a hand alternative-cost
// payment d: availFor with each granted mana ability whose spend restriction
// refuses d left out (CR 106.6: Mm'menon's granted blue spends only on a spell
// not cast from the hand; gorge potentialRestrictionAdmits).
func (s *State) walkHandAvail(k *prep, p uint8, d payDesc, a *avail) {
	a.xd, a.xdOn = d, true
	s.availFor(k, p, a)
	a.xdOn = false
}

// castGrantAvail is the census o's plain hand cast is priced against: av (the
// player's shared census) unless the table has both a restricted granted mana
// ability and a tap's extra, else the census filtered by the cast's own
// payment into fav, with a restricted grant the cast cannot spend left out and
// with it a tap's extra on its source (CR 106.6, 605.3a; gorge
// potentialManaFor(payment) and potentialRestrictionAdmits). Without a
// tap-extra carrier grantCap removes the over-counted play afterwards
// (script_gen.go); it skips every play when one exists, so only then does the
// gate itself have to.
func (s *State) castGrantAvail(k *prep, p uint8, o Obj, av, fav *avail) *avail {
	if k.gen == nil || !k.gen.anyTapExtra || !k.gen.anyGrantRestr() {
		return av
	}
	s.walkHandAvail(k, p, s.grantPayDescOf(MakeAction(AkCast, uint8(o), 0, 0)), fav)
	fav.delve = av.delve
	return fav
}

func (s *State) availGrantExtra(k *prep, sl Slot, pc *pcard, gi *[maxGrants]int16, ng int, own bool, a *avail) {
	if !k.gen.anyTapExtra {
		return
	}
	if !own && ng > 0 && s.g.availX&availWalk != 0 {
		return // the relaxed census: a gap's alternative carries none
	}
	var xv [kop.NumManaKinds]uint8
	hit := false
	if pc.manaXv != xv {
		// an all-at-once source (mana_eachcolour.go): its rest comes with its
		// own ability only, so beside a granted one its units cannot say which
		if ng > 0 {
			s.capFull(StallTapExtra)
			return
		}
		for c, n := range pc.manaXv {
			if n > 0 {
				a.addUnits(1<<c, n)
			}
		}
	}
	if own || ng == 0 {
		xv, hit = s.srcExtraHit(k, sl, pc)
	}
	for i := 0; i < ng; i++ {
		if i == 0 && !(own || ng == 0) {
			xv, hit = s.srcExtraHit(k, sl, s.grantView(k.gen, gi[i], sl))
			continue
		}
		gx, ghit := s.srcExtraHit(k, sl, s.grantView(k.gen, gi[i], sl))
		if ghit != hit || gx != xv {
			hit, xv = true, [kop.NumManaKinds]uint8{}
		}
	}
	if !hit {
		return
	}
	if xv == [kop.NumManaKinds]uint8{} || !s.tapExtraPlain(pc) || pc.painCol != 0 {
		s.capFull(StallTapExtra)
		return
	}
	for c := range xv {
		if xv[c] > 0 {
			a.addUnits(1<<c, xv[c])
		}
	}
}

// availUngranted is p's availability over the pool and the printed mana
// sources only (affordsUngranted's walk), for an {X} witness: gorge's
// planner pays the largest X the PRINTED sources fund (a granted ability is
// no witness source; potentialModeCastPlan).
//
//go:noinline
func (s *State) availUngranted(k *prep, p uint8, a *avail) {
	a.total, a.n, a.nOne = 0, 0, 0
	pl := &s.pl[p&1]
	for i := 0; i < kop.NumManaKinds; i++ {
		if pl.pool[i] > 0 {
			a.addUnits(1<<i, pl.pool[i])
		}
	}
	for m := s.ctrlM[p&1]; m.any(); m = m.rest() {
		sl := m.first()
		pm := &s.perms[sl]
		if pm.flags&pfTapped != 0 || s.manaGranted(k, sl) {
			continue
		}
		pc := s.manaPCGen(k, sl)
		if pc.manaAb < 0 && pm.ovr.flags&ovCopy == 0 || !s.canTapForManaPC(k, sl, pc) {
			continue
		}
		a.addUnits(s.srcMaskAt(pc, sl), srcYield(pc))
		if pc.manaOne {
			s.addOneColour(a, pc.manaN)
		}
		if k.gen.anyTapExtra {
			xv := s.srcExtra(k, sl, pc) // the planner prices a tap's extra too
			for c := range xv {
				if xv[c] > 0 {
					a.addUnits(1<<c, xv[c])
				}
			}
		}
	}
}

// grantSrch is one prefix search (gorge scriptSearch under gapsOnly): the
// granted sources of p in battlefield order, the play's mana cost, the
// node budget and the visited payer states, and the found prefix.
type grantSrch struct {
	k    *prep
	p    uint8
	cost kop.ManaCost
	// full is the play's cost before a waterbend offer credit and dyn marks
	// that the credit depends on this node's pool (a paid filter's fee, a
	// once-a-turn ability already used): visit prices full again at every
	// node (gorge's potentialPlayVerdict -> withWaterbendOfferCredit).
	full kop.ManaCost
	dyn  bool
	ex   Slot // the play's own {T} source (an activation's), never tapped
	// d is the payment the play makes, read by a granted source's spend
	// restriction (Mm'menon's Spell.!wasCastFromYourHand): the payment read,
	// the cast's origin known and the spell on the stack when paid
	d        payDesc
	n        int
	cand     [MaxPerm]Slot
	gi       [MaxPerm][maxGrants]int16 // each candidate's granted abilities (grantPC indices)
	ng       [MaxPerm]uint8
	dual     [MaxPerm]bool // the candidate's own ability is tried first
	skipPool bool          // the next branch's mana is restricted past the play (a printed ability or a grant): the step is taken, its mana prices nothing
	// ghost: the units those steps floated, which price nothing but sit in gorge's
	// pool, so its payer-state key tells a path that made them from one that did not
	ghost [kop.NumManaKinds]uint8
	// filt: the candidate also has a filter ability ({1},{T}: Add one mana of any
	// color), a paid census gap of gorge's like the grants (mana_filter.go)
	filt [MaxPerm]bool
	// tapx: the candidate's own ability is a tap-another-permanent one, a census
	// gap of gorge's beside the grants (mana_granttapx.go)
	tapx [MaxPerm]bool
	// def: the candidate is a deferred source (Pyromancer's Goggles), a census
	// gap of gorge's beside the grants (mana_grantdeferred.go)
	def [MaxPerm]bool
	// none: the candidate is a source of no colour (Command Tower), a census gap
	// of gorge's that adds nothing (mana_grantnone.go)
	none [MaxPerm]bool
	// gap: the candidate is a census gap. Non-gap sources join the search only
	// to fund a paid gap's fee, as many as funding allows (gorge
	// scriptSearch.funding: census.fees()).
	gap     [MaxPerm]bool
	gapN    int
	funding int
	funded  int
	nodes   int
	nseen   int
	seen    [potentialScriptNodes]grantKey
	np      int
	path    [2 * MaxPerm]grantStep
	// sp0 is the stack at the root: a deeper one is a trigger a prefix step
	// caused, which gorge's search lets resolve (a pass) once its activations are
	// spent. tapsLive: a tap can fire such a trigger (Gran-Gran's "whenever this
	// becomes tapped, draw a card, then discard a card"), so those steps run on real
	// transitions (settle).
	sp0      uint8
	tapsLive bool
	// need is the colours the play's printed base announces (gorge's
	// scriptNeedColours, potential_script.go): a colour ask's answers branch
	// on those first (branch, filtBranch), the mirror of gorge's
	// scriptOrderAnswers, so a budget-capped search reaches a colour the
	// play needs before the colours it does not. It only reorders a colour
	// ask's answers, never the answer set.
	need [kop.NumManaKinds]bool
	// pver is the play's own object's version at the root (Obj.ver): the play
	// stands only while its card has not changed zones (waitsForStack)
	pver uint8
}

// grantKey identifies a payer state (gorge scriptStateKey: the pool and
// each own permanent's tapped bit).
type grantKey struct {
	tapped slotMask
	pool   [kop.NumManaKinds]uint8
	// art: the pool slots holding an artifact's mana (genState.poolArt):
	// gorge's key reads the typed units apart (Player.ManaUnits), so a
	// filter's tagged {W} over a plain {W} is a new state
	art uint8
	// ghost: the units an unspendable step floated (grantSrch.ghost)
	ghost [kop.NumManaKinds]uint8
	// gorge's key counts the stack, hand and graveyard too (scriptStateKey)
	sp, hand, gy uint8
}

// grantStep is one prefix activation: candidate i tapped for n mana of
// colour col.
type grantStep struct {
	i   uint8
	col uint8
	n   uint8
	gi  int16 // the grantPC index tapped; -1 the candidate's own printed ability; stepFilt, stepFund
	sl  Slot
	// kind: stepAct (a tap), or the pass that lets the trigger a tap caused resolve
	// and the answer act gives to the ask that trigger poses
	kind uint8
	act  Action
	cand Slot // stepTapX: the permanent its cost taps
	pain bool // the source's painful printed ability (mana_grantpain.go)
}

const (
	stepAct uint8 = iota
	stepPass
	stepAnswer
)

// stepFilt is the gi of a prefix step that is a candidate's filter ability (col the
// colour it makes, its fee paid from the pool).
const stepFilt int16 = -2

// candidates lists p's permanents with a granted mana ability (granted only, or
// beside their own) in battlefield order. On a board where a grant is in force the
// filters join them: gorge's census misses a filter's paid ability as it does a
// granted one, so one search activates the gaps of both in battlefield order (the
// funding gorge's search also allows, other sources tapped to pay a fee, is not
// modelled here).
func (g *grantSrch) candidates(s *State) {
	var order [MaxPerm]Slot
	n := s.battlefieldOrder(g.p, s.ctrlM[g.p&1].and(s.occ), order[:], 0)
	var fm slotMask
	if k := g.k; k.gen.anyFilt {
		fm = s.scriptFilt(k, g.p)
	}
	tm := s.grantTapXGaps(g.k, g.p)
	dm := s.scriptDeferred(g.k, g.p)
	nm := s.scriptManaNone(g.k, g.p)
	g.n = 0
	holders := 0
	for i := 0; i < n; i++ {
		sl := order[i]
		if sl == g.ex {
			continue
		}
		ng := s.grantIdxs(g.k, sl, &g.gi[g.n])
		if ng == 0 && !fm.has(sl) && !tm.has(sl) && !dm.has(sl) && !nm.has(sl) {
			continue
		}
		if ng > 0 {
			holders++
		}
		g.cand[g.n], g.ng[g.n], g.dual[g.n], g.filt[g.n], g.tapx[g.n] = sl, uint8(ng), !tm.has(sl) && (s.ownManaUsable(g.k, sl) || ng > 0 && s.ownRestrAb(g.k, sl) != nil), fm.has(sl), tm.has(sl)
		g.def[g.n] = ng == 0 && dm.has(sl)
		g.none[g.n] = ng == 0 && nm.has(sl)
		g.gap[g.n] = true
		if fm.has(sl) {
			g.funding += s.filtFee(g.k, sl)
		}
		g.n++
	}
	if holders == 0 {
		g.n, g.gapN = 0, 0
		return
	}
	// A paid gap's fee is funded by other sources (gorge scriptSearch's
	// funding loop: the gaps first, then non-gaps while funding lasts). The
	// non-gaps sit after every gap in battlefield order, exactly the walk
	// order that loop reads.
	g.gapN = g.n
	if !s.filtFunded(g.k, g.p, fm) {
		g.funding = 0 // (gorge's unrelaxable census funds no fee)
	}
	if g.funding == 0 {
		return
	}
	seen := func(sl Slot) bool {
		for i := 0; i < g.n; i++ {
			if g.cand[i] == sl {
				return true
			}
		}
		return false
	}
	for i := 0; i < n; i++ {
		sl := order[i]
		if sl == g.ex || seen(sl) || !s.canTapForMana(g.k, sl) {
			continue
		}
		g.cand[g.n], g.ng[g.n], g.dual[g.n], g.filt[g.n], g.tapx[g.n], g.def[g.n], g.none[g.n] = sl, 0, true, false, false, false, false
		g.gap[g.n] = false
		g.n++
	}
}

// tappedOf is the tapped bits of p's permanents.
func (s *State) tappedOf(p uint8) slotMask {
	var m slotMask
	for b := s.ctrlM[p&1].and(s.occ); b.any(); b = b.rest() {
		if s.perms[b.first()].flags&pfTapped != 0 {
			m = m.with(b.first())
		}
	}
	return m
}

// node charges one search node; false when the budget is spent.
func (g *grantSrch) node() bool {
	if g.nodes >= potentialScriptNodes {
		return false
	}
	g.nodes++
	return true
}

// visit is one payer priority of the search on working copy t: the play
// is found when the pool and the printed sources pay it (gorge
// offered / priced); else each untapped candidate is activated in turn,
// its answers branched on in gorge's order, each payer state once.
func (g *grantSrch) visit(t *State) bool {
	if !g.node() {
		return false
	}
	cost := g.cost
	if g.dyn {
		cost = t.grantDynamicCost(g.k, g.p, g.d.play, g.full)
	}
	pl := &t.pl[g.p&1]
	key := grantKey{tapped: t.tappedOf(g.p), pool: pl.pool, sp: t.sp, hand: pl.handN, gy: pl.graveN}
	for c := range key.pool {
		key.pool[c] += g.ghost[c] // gorge's key reads the floated pool, whose restriction it does not (scriptStateKey)
	}
	if g.k.artMana {
		t.artSync(g.p)
		key.art = t.g.poolArt[g.p&1]
	}
	seen := false
	for i := 0; i < g.nseen; i++ {
		if g.seen[i] == key {
			seen = true
			break
		}
	}
	if seen {
		return false // (gorge's seen check precedes its pricing: scriptSearch.visit)
	}
	if t.affordsUngrantedPlay(g.k, g.p, g.d.play, &cost, g.ex, g.d) && !g.waitsForStack(t) {
		return true
	}
	if g.nseen < len(g.seen) {
		g.seen[g.nseen] = key
		g.nseen++
	}
	for i := 0; i < g.gapN; i++ {
		sl := g.cand[i]
		if g.ng[i] == 0 && g.filt[i] && g.k.gen.anyFiltFirst && t.filtFirstOnly(g.k, sl) {
			// a filter that is its source's only mana ability: no printed
			// ability to ask beside, one colour ask once its fee is in the pool
			if t.filtUsable(g.k, sl) && t.filtPaid(g.k, g.p, sl) && g.filtBranch(t, i, sl) {
				return true
			}
			continue
		}
		if g.none[i] {
			if g.noneBranch(t, i, sl) { // taps for nothing (mana_grantnone.go)
				return true
			}
			continue
		}
		if g.def[i] {
			if g.deferBranch(t, i, sl) { // its plain {R} (mana_grantdeferred.go)
				return true
			}
			continue
		}
		tx := g.tapx[i] && t.scriptTapX(g.k, g.p).has(sl)
		grants := t.canTapForMana(g.k, sl)
		fok := g.filt[i] && t.filtPaid(g.k, g.p, sl)
		if !grants && !tx && !(fok && t.filtUsable(g.k, sl)) {
			continue // (a summoning-sick Changeling's filter stays: only its granted {T} ability is out, CR 302.6)
		}
		// several abilities (its own beside a grant, two grants): the
		// "choose a mana ability" ask, one node, the printed ability's
		// answers first (gorge flattens a Combo's colours into that ask;
		// an Any ability still asks its colour)
		alts := 0
		if grants {
			alts = t.grantAlts(g.k, g.p, sl, &g.gi[i], int(g.ng[i]))
		}
		if g.dual[i] {
			alts++
		}
		if fok {
			alts++
		}
		if tx {
			alts++
		}
		if alts > 1 && !g.node() {
			return false
		}
		if tx && g.tapXBranch(t, i, sl) { // the printed tap-another ability first (mana_granttapx.go)
			return true
		}
		if !grants {
			if fok && g.filtBranch(t, i, sl) {
				return true
			}
			continue
		}
		pre := g.askNeededFirst(t, i, sl) // (the granted colours the ask answers first)
		if ppc := t.manaPCGen(g.k, sl); g.dual[i] && !t.ownRestrPrinted(g.k, sl) && grantPainShape(g.k, ppc) {
			if g.painBranch(t, i, sl, ppc, alts == 1) { // its painful ability's colours too (mana_grantpain.go)
				return true
			}
		} else if g.dual[i] {
			for j := 0; j < int(g.ng[i]); j++ {
				if pre[j] != 0 && g.preBranch(t, i, sl, j, pre[j], alts == 1) {
					return true
				}
			}
			mask, n := uint8(0), uint8(0)
			if ra := t.ownRestrAb(g.k, sl); ra != nil && !t.ownManaUsable(g.k, sl) {
				mask, n = uint8(ra.ManaColors)&0x3f, t.restrAbN(g.k, sl, ra) // the restricted printed ability
				// gorge's search taps it first whether or not the play may spend
				// it; mana the play cannot spend prices nothing
				g.skipPool = !t.restrSrcAdmits(g.k, g.p, sl, ra, &g.d)
			} else {
				ppc := t.manaPCGen(g.k, sl)
				mask, n = t.srcMaskAt(ppc, sl), ppc.manaN
			}
			if g.branch(t, i, sl, -1, mask, n, alts == 1) {
				return true
			}
		}
		if g.dual[i] && t.ownManaUsable(g.k, sl) {
			// a restricted printed ability beside the usable one (White Lotus
			// Hideout's Lesson-only any colour): gorge's search taps it too, after
			// the usable one, whether or not the play may spend it
			if ra := t.ownRestrAb(g.k, sl); ra != nil {
				g.skipPool = !t.restrSrcAdmits(g.k, g.p, sl, ra, &g.d)
				if g.branch(t, i, sl, -1, uint8(ra.ManaColors)&0x3f, t.restrAbN(g.k, sl, ra), false) {
					return true
				}
			}
		}
		if fok && g.filtBranch(t, i, sl) { // (listed after the printed ability, before the grants)
			return true
		}
		for j := 0; j < int(g.ng[i]); j++ {
			gi := g.gi[i][j]
			pc := t.grantView(g.k.gen, gi, sl)
			mask := t.srcMaskAt(pc, sl)
			if !t.grantCostPayable(g.p, pc) {
				continue // its life cannot be paid (CR 119.4)
			}
			if pre[j] != 0 && g.dual[i] {
				if mask &^= pre[j]; mask == 0 {
					continue // all its colours were answered first
				}
			}
			// gorge's search taps a granted census gap whether or not the play may
			// spend it (the mana floats, the tap fires its triggers); mana the
			// play's origin or class cannot spend prices nothing
			g.skipPool = !g.grantAdmits(t, sl, gi)
			ask := alts == 1 || t.srcMaskAt(pc, sl)&uint8(kop.ColAny) == uint8(kop.ColAny)
			if grantCostLife(pc) > 0 {
				if g.costBranch(t, i, sl, gi, pc, mask, ask) { // the step pays its life (mana_grantcost.go)
					return true
				}
				continue
			}
			if g.branch(t, i, sl, gi, mask, pc.manaN, ask) {
				return true
			}
		}
	}
	// A paid gap's fee is funded by other sources (gorge scriptSearch's
	// funding loop: the gaps first, then non-gaps while funding lasts).
	for i := g.gapN; i < g.n && g.funded < g.funding; i++ {
		sl := g.cand[i]
		if t.perms[sl].flags&pfTapped != 0 {
			continue
		}
		pc := t.manaPCGen(g.k, sl)
		if pc.manaAb < 0 || !t.canTapForManaPC(g.k, sl, pc) {
			continue
		}
		mask, n := t.srcMaskAt(pc, sl), pc.manaN
		if mask == 0 {
			continue
		}
		g.funded++
		ok := g.branch(t, i, sl, -1, mask, n, mask&(mask-1) != 0)
		g.funded--
		if ok {
			return true
		}
	}
	if t.sp > g.sp0 && g.np < len(g.path) {
		// a trigger a step caused: let it resolve, the pool floats (gorge
		// scriptSearch.visit, the last resort at a node)
		pass := MakeAction(AkPass, 0, 0, 0)
		c := t.fork()
		c.Apply(pass)
		g.path[g.np] = grantStep{kind: stepPass, act: pass, sl: NoSlot}
		g.np++
		ok := g.settle(c)
		t.unfork()
		if !ok {
			g.np--
		}
		return ok
	}
	return false
}

// waitsForStack: the play is sorcery-speed and the path left a trigger of its own
// on the stack, so the play is not offered until it resolves (CR 307.1, 602.5d;
// gorge scriptSearch.waitsForStack) and the search continues through a pass.
func (g *grantSrch) waitsForStack(t *State) bool {
	if !g.d.hasPlay {
		return false
	}
	if o := Obj(g.d.play.Source()); o != NoObj && int(o) < len(t.objs) && t.objs[o].ver != g.pver {
		// the play's own card left the zone it is played from (a Mechan
		// Navigator tap's draw-then-discard binning the library-top spell): a
		// new object (CR 400.7), so gorge's search finds no such play
		return true
	}
	// a stack the prefix grew, or a trigger's answer that may have moved the
	// play's own card (Gran-Gran's loot discarding the spell: gorge prices a
	// cast from a card no longer in its origin zone as unsupported)
	if t.sp <= g.sp0 && !g.answered() {
		return false
	}
	var d Decision
	t.plainOptions(g.k, &d, 0)
	return !hasOption(&d, g.d.play)
}

// answered: the path holds an answer to a trigger's ask.
func (g *grantSrch) answered() bool {
	for i := 0; i < g.np; i++ {
		if g.path[i].kind == stepAnswer {
			return true
		}
	}
	return false
}

// settle continues the search at t after a pass or a tap whose trigger is
// queued: t runs to p's next decision, a priority being a node (visit) and an
// ask the trigger poses (the discard of Gran-Gran's loot) a node branching on its
// answers (gorge scriptSearch.visit's scriptAnswers).
func (g *grantSrch) settle(t *State) bool {
	var d Decision
	if !t.tapXNext(g.p, &d) {
		return false
	}
	if d.Kind == DkPriority {
		return g.visit(t)
	}
	if !g.node() {
		return false
	}
	if d.Kind == DkTarget {
		t.gorgeTargetOrder(&d)
	}
	if !gorgeAskOrder(&d) {
		return false // (Jett Reno's surveil: the card to the graveyard first)
	}
	for i := 0; i < int(d.N) && g.np < len(g.path); i++ {
		c := t.fork()
		c.Apply(d.Options[i])
		g.path[g.np] = grantStep{kind: stepAnswer, act: d.Options[i], sl: NoSlot}
		g.np++
		ok := g.settle(c)
		t.unfork()
		if ok {
			return true
		}
		g.np--
	}
	return false
}

// next continues the search after sl's tap: a tap that fires a trigger runs on a
// copy whose trigger is stacked, as does a step whose own cost queued one (Haunted
// Screen's 1 life heard by Marina Vendrell's Grimoire, CR 119.4, 603.3).
func (g *grantSrch) next(t *State, sl Slot) bool {
	taps := g.tapTriggers(t, sl)
	if !taps && t.npend == 0 {
		return g.visit(t)
	}
	u := t.fork()
	if taps {
		u.manaTapped(g.k, sl)
	}
	ok := g.settle(u)
	t.unfork()
	return ok
}

// tapTriggers reports whether tapping sl may fire a trigger the search stacks
// before the next activation (a TapAll listener hears any permanent's tap:
// trig_tapall.go).
func (g *grantSrch) tapTriggers(t *State, sl Slot) bool {
	return g.tapsLive && (t.subs[kop.TrigTaps].has(sl) || g.k.gen.anyTapAll)
}

// branch tries tapping candidate i (slot sl) for each colour of mask in
// W U B R G C order: a choice of colours is an ask node when ask (gorge's
// colour ask, one node, every answer branched on).
//
// A step floats its tap's extra (mana_tapextra.go), whatever kind of effect
// granted the ability it taps (gorge adds the extra to every tapped source).
func (g *grantSrch) branch(t *State, i int, sl Slot, gi int16, mask, n uint8, ask bool) bool {
	if mask == 0 {
		mask = 1 << kop.C
	}
	skip := g.skipPool // the printed restricted ability's unspendable mana (mana_grantrestrictedown.go)
	g.skipPool = false
	if ask && mask&(mask-1) != 0 && !g.node() {
		return false
	}
	pn := n
	if skip {
		pn = 0
	}
	var pc *pcard
	if g.k.gen.anyTapExtra {
		pc = t.stepView(g.k, sl, gi)
	}
	var order [kop.NumManaKinds]uint8
	no := t.stepColours(g.k, sl, gi, mask, &order)
	needFirst(order[:no], g.need)
	for _, c := range order[:no] {
		t.perms[sl].flags |= pfTapped
		preBit := t.g.poolArt[g.p&1] // the typed-mana tag before the step: the untag restores exactly it (the slot may hold plain mana too)
		t.pl[g.p&1].pool[c] += pn
		t.artTag(g.k, g.p, sl, int(c))
		xv := t.srcExtraPick(g.k, sl, pc, int(c))
		var gx [kop.NumManaKinds]uint8 // the unspendable tap prices no extra either; its units sit in the ghost
		if skip {
			gx, xv = xv, [kop.NumManaKinds]uint8{}
			gx[c] += n
			for j := range gx {
				g.ghost[j] += gx[j]
			}
		}
		t.addXv(g.p&1, &xv)
		g.path[g.np] = grantStep{i: uint8(i), col: c, n: n, gi: gi, sl: sl}
		g.np++
		if g.next(t, sl) {
			return true
		}
		g.np--
		for j := range gx {
			g.ghost[j] -= gx[j]
		}
		t.subXv(g.p&1, &xv)
		t.pl[g.p&1].pool[c] -= pn
		t.g.poolArt[g.p&1] = preBit
		t.perms[sl].flags &^= pfTapped
	}
	return false
}

// stepColours lists the colours of mask a prefix step on sl may answer, in the
// order gorge's ask offers them: a Combo ability's printed order (Sun-Blessed
// Peak's "R or W"; manaColourOrder), else W U B R G C. gi is the granted
// ability the step taps, -1 for the source's own.
func (s *State) stepColours(k *prep, sl Slot, gi int16, mask uint8, out *[kop.NumManaKinds]uint8) (n int) {
	cd := s.cardAt(sl)
	var pc *pcard
	if gi < 0 {
		pc = s.manaPCGen(k, sl)
	}
	var ab *kop.Ability
	if gi < 0 {
		if ra := s.ownRestrAb(k, sl); ra != nil && !s.ownManaUsable(k, sl) {
			ab = ra // the printed restricted ability (mana_grantrestrictedown.go)
		} else if pc.manaAb >= 0 && (pc == &k.cards[cd] || pc.blgFirst != 0) {
			// the source's own profile or a granted-type view names one of the
			// card's own abilities; a grant/var/gate view's index names another
			// card's (mana_window.go windowCol)
			ab = manaAbView(&k.t.Cards[cd], pc)
		}
	} else if r := k.gen.grantOf[gi]; !k.gen.grantSelf[gi] {
		ab = &k.t.Cards[r.card].Abilities[r.ab]
	}
	var seen uint8
	if gi < 0 && pc.blgFirst != 0 {
		// a land with a granted basic type: gorge lists the face's own abilities
		// before the intrinsic ones the type adds (appendAvailableManaAbilities),
		// so a Forest made an Island asks G then U, not U then G; a printed
		// Combo asks its token order (Kyoshi Village's Combo G W asks G first)
		if own := &k.cards[cd]; own.manaAb >= 0 {
			if ownAb := manaAbView(&k.t.Cards[cd], own); ownAb != nil {
				var ord [5]uint8
				for _, c := range ord[:manaColourOrder(ownAb, &ord)] {
					if uint8(own.manaCol)&mask&(1<<c) != 0 {
						out[n] = c
						n++
						seen |= 1 << c
					}
				}
			}
		}
	}
	if ab != nil && ab.ManaColOrd != 0 && uint8(ab.ManaColors)&0x1f == mask&0x1f {
		var ord [5]uint8
		for _, c := range ord[:manaColourOrder(ab, &ord)] {
			out[n] = c
			n++
			seen |= 1 << c
		}
	}
	for c := uint8(0); c < kop.NumManaKinds; c++ {
		if mask&(1<<c) != 0 && seen&(1<<c) == 0 {
			out[n] = c
			n++
		}
	}
	return n
}

// filtBranch is the filter's colour ask at candidate i (slot sl): its fee comes out
// of the pool, then the colours in turn (gorge's askManaColor, mana_filter.go).
func (g *grantSrch) filtBranch(t *State, i int, sl Slot) bool {
	if !g.node() {
		return false
	}
	var order [5]uint8
	n := manaColourOrder(t.filtAb(g.k, sl), &order)
	needFirst(order[:n], g.need)
	for j := 0; j < n; j++ {
		c := order[j]
		u := t.fork()
		u.filtDo(g.k, g.p, filtStep{sl: sl, gap: true, ans: 1 + c})
		g.path[g.np] = grantStep{i: uint8(i), col: c, gi: stepFilt, sl: sl}
		g.np++
		var ok bool
		if g.tapsLive {
			ok = g.settle(u) // (the filter's tap may have fired a trigger)
		} else {
			ok = g.visit(u)
		}
		t.unfork()
		if ok {
			return true
		}
		g.np--
	}
	return false
}

// grantAdmits: the play g.d makes may spend granted ability gi's mana (an
// unrestricted grant always; a restricted one as restrSrcAdmits reads the
// play). The search is feasibility-only, so mana the play cannot spend
// must not price it (the offer would stall the payment); the grant is still
// a gap step, tapped as gorge's search taps it (visit sets skipPool).
func (g *grantSrch) grantAdmits(t *State, sl Slot, gi int16) bool {
	if g.k.gen.grantRestr[gi] == 0 {
		return true
	}
	r := g.k.gen.grantOf[gi]
	ok := t.restrSrcAdmits(g.k, g.p, sl, &g.k.t.Cards[r.card].Abilities[r.ab], &g.d)
	if st := DebugRestrGrantStats; st != nil {
		st.Reads++
		if !ok {
			st.Denied++
		}
	}
	return ok
}

// grantPayDesc is the payment play a makes, for a granted source's spend
// restriction: a cast is read as paid (cast=true: the spell is on the
// stack when the prefix's mana is spent), an activation as an ability.
func (s *State) grantPayDesc(a Action) payDesc {
	d := s.grantPayDescOf(a)
	d.play, d.hasPlay = a, true
	return d
}

func (s *State) grantPayDescOf(a Action) payDesc {
	o := Obj(a.Source())
	switch a.Kind() {
	case AkCast, AkCastCasualty, AkCastConspire:
		og := restrOrigin{from: s.objs[o].zone}
		return payDesc{class: pdSpell, card: s.objs[o].card, src: NoSlot, og: og}
	case AkActivate:
		d := payDesc{class: pdAct, card: s.objs[o].card, src: NoSlot}
		if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot {
			d.card, d.src = s.cardAt(ob.slot), ob.slot
		}
		return d
	}
	return payDesc{class: pdOther, src: NoSlot}
}

// grantSearch runs gorge's prefix search for p's play of cost (ex its own
// {T} source) on a copy of s: ok is false when no prefix within the node
// budget makes the pool and the printed sources pay it; np is 0 when they
// already do.
//
//go:noinline
func (s *State) grantSearch(k *prep, p uint8, cost *kop.ManaCost, ex Slot, d payDesc, need [kop.NumManaKinds]bool, g *grantSrch) bool {
	*g = grantSrch{k: k, p: p, cost: *cost, ex: ex, d: d, sp0: s.sp, tapsLive: k.gen.manaTaps, need: need}
	if d.hasPlay {
		if full, ok := s.grantWaterbendFull(k, d.play); ok {
			g.full, g.dyn = full, true
		}
	}
	if d.hasPlay {
		if o := Obj(d.play.Source()); o != NoObj && int(o) < len(s.objs) {
			g.pver = s.objs[o].ver
		}
	}
	g.candidates(s)
	if g.n == 0 {
		return s.affordsUngrantedFor(k, p, cost, ex, d)
	}
	t := s.fork()
	defer s.unfork()
	return g.visit(t)
}

// grantApply taps the found prefix into p's pool on s (the play's
// announcement: the mana floats from here, as the oracle's prScript
// activations do).
func (s *State) grantApply(k *prep, g *grantSrch) {
	n, passed := s.Decisions, false
	var d Decision
	for i := 0; i < g.np; i++ {
		st := &g.path[i]
		if st.kind != stepAct {
			// the pass and the trigger's answers, each as the decision then pending
			// (the other player passes for the trigger to resolve)
			passed = true
			if s.tapXNext(g.p, &d) && hasOption(&d, st.act) {
				s.Apply(st.act)
			}
			continue
		}
		sl := st.sl
		switch st.gi {
		case stepTapX:
			s.tapXDo(k, g.p, tapXStep{sl: sl, cand: st.cand, cand2: NoSlot, col: st.col, kind: tapXAct})
			continue
		case stepManaNone:
			s.noneApply(k, g.p, sl, st.col) // (mana_grantnone.go)
			if g.tapTriggers(s, sl) {
				passed = true
				s.tapXNext(g.p, &d)
			}
			continue
		case stepDeferred:
			s.deferApply(k, g.p, sl) // (mana_grantdeferred.go)
			if g.tapTriggers(s, sl) {
				passed = true
				s.tapXNext(g.p, &d)
			}
			continue
		case stepFilt:
			s.filtDo(k, g.p, filtStep{sl: sl, gap: true, ans: 1 + st.col})
			if !s.filtNoTap(k, sl) {
				s.manaTapped(k, sl) // (a once-each-turn filter taps nothing)
			}
			continue
		}
		if st.gi >= 0 && grantCostLife(s.grantView(k.gen, st.gi, sl)) > 0 {
			s.grantCostApply(k, g.p, st) // the tap and its life (mana_grantcost.go)
			if g.tapTriggers(s, sl) || s.npend != 0 {
				passed = true
				s.tapXNext(g.p, &d)
			}
			continue
		}
		sacrifice := false
		if st.gi < 0 {
			// the source's own ability sacrifices it (a Treasure beside a grant):
			// gorge pays that cost at priority, so the token has ceased before
			// the next activation, not at the play's state-based check
			pc := s.manaPCGen(k, sl)
			sacrifice = pc.manaSac || s.canSacOnlyForMana(k, sl, pc)
		}
		s.perms[sl].flags |= pfTapped
		s.touchSl(sl)
		s.manaTapped(k, sl)
		if ra := s.ownRestrAb(k, sl); st.gi < 0 && ra != nil && !s.ownManaUsable(k, sl) {
			// the printed restricted ability (mana_grantrestrictedown.go)
			if !s.floatRestrAb(k, g.p&1, sl, ra, int(st.col), st.n) {
				s.capFull(StallRestrSrc)
			}
		} else if ab := restrAbOfStep(k, st); ab != nil {
			// a restricted grant floats beside the pool (State.rb), spendable only
			// as its restriction allows (both halves, spells and activated
			// abilities); never a silent spill into the pool
			if !s.floatRestrAb(k, g.p&1, sl, ab, int(st.col), st.n) {
				s.capFull(StallRestrSrc)
			}
		} else {
			s.pl[g.p&1].pool[st.col] += st.n
		}
		if st.pain {
			s.painDamage(k, sl, g.p&1, int(s.manaPCGen(k, sl).painN)) // its life or damage (mana_grantpain.go)
		}
		if k.gen.anyTapExtra {
			xv := s.srcExtraPick(k, sl, s.stepView(k, sl, st.gi), int(st.col))
			s.addXv(g.p&1, &xv)
		}
		if sacrifice {
			// at priority, outside the build's and a resolution's mana steps, so the
			// move cannot park for gorge's order ask: two competing exile-instead
			// replacements stall it by name in dieZone (StallReplLeave, CR 616.1)
			o := s.perms[sl].obj
			s.fireSac(k, sl)
			s.leave(k, sl, kop.ZoneGraveyard, false)
			if s.g.on && s.objs[o].of&ofToken != 0 && s.objs[o].zone == kop.ZoneGraveyard {
				s.ceaseManaToken(o)
			}
		}
		if g.tapTriggers(s, sl) || s.npend != 0 {
			// the tap's trigger (or the one its paid life queued) is stacked before the next activation, as the
			// search's settle stacked it: gorge answers each activation as its own
			// decision, so two Taps triggers of one prefix (Beckett Mariner and
			// Erica Ortegas tapped for Rok-Tahk's {G}{G}) go on the stack one at a
			// time and never wait together for an order ask (CR 603.3b)
			passed = true
			s.tapXNext(g.p, &d)
		}
	}
	if passed {
		s.tapXNext(g.p, &d) // the trigger the last step queued is stacked and asked now
		s.Decisions = n
	}
}

// grantTapXPrefix plays gorge's prefix script for play a when a spell-only
// grant is the only census pricing it: the granted sources tapped in
// battlefield order, their mana floating beside the pool (State.rb), and the
// tap-another gap's step where it sits in that order (tapXScript's path), the
// play re-priced after each (gorge potential_script.go: gaps first, then
// priced; it stops at the first that prices it). The granted taps are
// dead-end taps on gorge's script — their mana cannot pay the play (CR
// 605.1a) — and the play then prices from the printed sources and the
// tap-another gap's own mana (CR 601.2g-h).
func (s *State) grantTapXPrefix(k *prep, a Action, p uint8) {
	if !s.grantsLive(k) || !s.tapXScript(k, a, p, false) {
		return // no tap-another script within gorge's node budget (mana_grantedoffers.go)
	}
	cost, ex, _, ok := s.grantPlayCost(k, p, a, true)
	if !ok {
		return
	}
	desc := s.grantPayDesc(a)
	var gaps, tx slotMask
	tx = s.scriptTapXModelled(k, p)
	for b := s.ctrlM[p&1].and(s.occ); b.any(); b = b.rest() {
		sl := b.first()
		if s.canTapForMana(k, sl) && s.grantIdx(k, sl) >= 0 {
			gaps = gaps.with(sl)
		}
	}
	var order [MaxPerm]Slot
	n := s.battlefieldOrder(p, gaps.or(tx), order[:], 0)
	for i := 0; i < n; i++ {
		sl := order[i]
		if tx.has(sl) {
			s.tapXScript(k, a, p, true) // the tap-another gap's step (mana_tapanother.go)
		} else {
			s.tapGrantedGap(k, p, sl)
		}
		if s.npend != 0 {
			s.flushPrefixTriggers(k) // the tap's trigger is asked as gorge's script answers it (CR 603.3b)
		}
		if s.affordsUngrantedPlay(k, p, a, &cost, ex, desc) {
			return // gorge's search stops: the play prices now
		}
	}
}

// tapGrantedGap taps one granted mana source of p in gorge's prefix script:
// the first granted ability's mana floats beside the pool (State.rb),
// spendable only as its restriction allows (grantApply's step; never a
// silent spill), in the colour the source's ask lists first (gorge's search
// answers in order).
func (s *State) tapGrantedGap(k *prep, p uint8, sl Slot) {
	var gi [maxGrants]int16
	if ng := s.grantIdxs(k, sl, &gi); ng == 0 {
		return
	}
	pc := s.grantView(k.gen, gi[0], sl)
	var order [kop.NumManaKinds]uint8
	no := s.stepColours(k, sl, gi[0], s.srcMaskAt(pc, sl), &order)
	if no == 0 {
		return
	}
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	n := max(pc.manaN, 1)
	if k.gen.anyTapExtra {
		xv := s.srcExtraPick(k, sl, pc, int(order[0]))
		s.addXv(p&1, &xv)
	}
	if ab := restrAbOfGrant(k, gi[0]); ab != nil {
		if !s.floatRestrAb(k, p&1, sl, ab, int(order[0]), n) {
			s.capFull(StallRestrSrc)
		}
		return
	}
	s.pl[p&1].pool[order[0]] += n
}

// stepView is the mana ability a prefix step taps on sl: the source's own
// (gi -1) or the granted one.
func (s *State) stepView(k *prep, sl Slot, gi int16) *pcard {
	if gi < 0 {
		return s.manaPCGen(k, sl)
	}
	return s.grantView(k.gen, gi, sl)
}

// restrAbOfStep is the granted mana ability a prefix step tapped when it
// carries a spend restriction (nil: none, or the candidate's own printed
// ability).
func restrAbOfStep(k *prep, st *grantStep) *kop.Ability {
	if st.gi < 0 {
		return nil
	}
	return restrAbOfGrant(k, st.gi)
}

// restrAbOfGrant is grant gi's mana ability when it carries a spend
// restriction (nil: none), so every float of its mana names both halves of
// the restriction (spells and activated abilities).
func restrAbOfGrant(k *prep, gi int16) *kop.Ability {
	if k.gen.grantRestr[gi] == 0 {
		return nil
	}
	r := k.gen.grantOf[gi]
	return &k.t.Cards[r.card].Abilities[r.ab]
}

// grantPlayCost is the mana cost gorge's prefix search prices for p's
// play a (grantPrefix / grantCap), ok false for a play it never searches.
// need is the colours the play's printed base announces (gorge's
// scriptNeedColours, potential_script.go): empty for an alternative, a
// kicked, face-down or MayPlay cast, as gorge's scriptNeedColours leaves
// it -- the search branches a colour ask on those first (grantSrch.branch).
func (s *State) grantPlayCost(k *prep, p uint8, a Action, kicked bool) (cost kop.ManaCost, ex Slot, need [kop.NumManaKinds]bool, ok bool) {
	if s.g.raiseO == 0 {
		// the cast owes its permission's surcharge (Zero's {2}) once it is
		// built; the search prices what the cast will pay (CR 601.2f, 118.8)
		if r := s.mayRaiseOwed(k, p, a); r != 0 {
			s.g.raiseO = r
			defer func() { s.g.raiseO = 0 }()
		}
	}
	cost, ex, need, ok = s.grantPlayCostRaise(k, p, a, kicked)
	if ok && a.Kind() == AkCast && a.Mode()&castMayPlay != 0 && a.Mode()&castFree == 0 && (s.g.mayPlayAny || s.mayPlayAnyType(k, p, Obj(a.Source()))) {
		// an any-type may-play rider: gorge's planner funds the cast under
		// the rider (pay.PlanManaUnderRider), so the printed lands pay
		// Fuss's {R/W} from Outrageous Robbery's exile and no granted
		// source is tapped first (CR 601.2g-h, 609.4b)
		cost = planUnderRider(cost, true, false)
	}
	return cost, ex, need, ok
}

// grantPlayCostRaise is grantPlayCost with raiseO set as the caller left it.
func (s *State) grantPlayCostRaise(k *prep, p uint8, a Action, kicked bool) (cost kop.ManaCost, ex Slot, need [kop.NumManaKinds]bool, ok bool) {
	ex = NoSlot
	need = s.playNeed(k, a)
	switch a.Kind() {
	case AkActivate:
		if a.Mode()&0x80 != 0 {
			return cost, ex, need, false
		}
		cost, ex, ok = s.activationMana(k, Obj(a.Source()), a.Mode())
		if ok {
			cost = s.waterbendPlanCost(k, p, Obj(a.Source()), a.Mode(), cost)
		}
		return cost, ex, need, ok
	case AkCast, AkCastCasualty, AkCastConspire:
		if alt, ok := s.altPlayCost(k, p, a); ok {
			return alt, ex, need, true
		}
		if a.Mode()&castFace != 0 && a.Mode()&3 == 0 {
			// a face cast (an Adventure from the hand) pays the face's own
			// cost, not the front's: the search priced it as the front and
			// removed a cast gorge offers
			c := s.objs[a.Source()].card
			if cd := &k.t.Cards[c]; roomDef(k, c) {
				c = cd.Faces[a.Target()] // a room_alt cast: the other door, whichever door the card shows
			} else if len(cd.Faces) == 2 && a.Target() == 1 {
				c = cd.Faces[1]
			}
			return s.adjustCost(k, p, c, Obj(a.Source()), k.cards[c].cost), ex, need, true
		}
		if a.Mode() == 0 && a.Kind() == AkCast {
			// an impended or evoked cast pays its alternative cost (CR 118.9)
			c := s.objs[a.Source()].card
			switch cd := &k.t.Cards[c]; {
			case a.Target() == castImpTgt && cd.Impending > 0:
				return s.adjustCost(k, p, c, Obj(a.Source()), cd.ImpendingCost), ex, need, true
			case a.Target() == castEvokeTgt && cd.HasEvoke:
				return s.adjustCost(k, p, c, Obj(a.Source()), cd.EvokeCost), ex, need, true
			}
		}
		if a.Mode()&1 != 0 {
			if !kicked {
				return cost, ex, need, false // kicked: lowered by its witness (beginCast)
			}
			if cost, ok = s.plannedCastCostOf(k, p, Obj(a.Source()), a.Mode()&2, false, true); !ok {
				return cost, ex, need, false
			}
			// the composed cost (pair 8638)
			c := s.objs[a.Source()].card
			cost = s.featCost(k, p&1, c, Obj(a.Source()), addCost(k.cards[c].cost, k.t.Cards[c].Kicker))
			if s.convAnyType(k, p&1, Obj(a.Source())) {
				cost = anyTypePips(cost) // a creature's kicked total under Vizier's conversion, as plannedCastCostOf prices the plain one (featview.go)
			}
			return cost, ex, need, true
		}
		if c := s.objs[a.Source()].card; a.Kind() == AkCast && (a.Mode() == 0 || a.Mode() == castMayPlay) && k.t.Cards[c].Cost.X != 0 && !k.t.Cards[c].HasAltAdd && k.t.Cards[c].Cost.Hyb[0] == 0 {
			// an {X} cast: gorge's potentialModeCastPlan prices it at the
			// largest X the sources pay, insufficient when they cannot
			// pay X = 0, so the prefix search runs on the fixed part and
			// the witness (lowerWitness) then pays the largest X from the
			// pool and the printed sources. A may-play one too (Worldsoul's
			// Rage from Outrageous Robbery's exile), its pips generic under
			// an any-type rider (pay.PlanManaUnderRider, CR 609.4b): the
			// search's first colour then pays it (CR 601.2g-h)
			cost := s.featCost(k, p&1, c, Obj(a.Source()), k.cards[c].cost)
			if a.Mode() == castMayPlay && (s.g.mayPlayAny || s.mayPlayAnyType(k, p, Obj(a.Source()))) {
				cost = anyTypePips(cost)
			}
			return cost, ex, need, true
		}
		if c := s.objs[a.Source()].card; a.Kind() == AkCast && a.Mode() == 0 && a.Target() == 0 && k.t.Cards[c].HasAltAdd && k.t.Cards[c].Cost.X == 0 && !s.contribPays(k, p, c, Obj(a.Source()), s.objs[a.Source()].zone == kop.ZoneHand) {
			// an either-or additional cost: the planner prices the spell with the part
			// gorgeLowers prices, so the search funds that (Deadly Precision's {B})
			return s.altAddPlanCost(k, p, c, Obj(a.Source())), ex, need, true
		}
		if c := s.objs[a.Source()].card; a.Kind() == AkCast && a.Mode() == 0 && a.Target() == 0 && modalFloorZero(k, c) {
			// a Spree or Tiered face: the cast planner declines the shape
			// ("shape:modal_cost"), so gorge's potential planner prices the printed
			// cost from its census and the prefix search taps a granted source the
			// census misses (CR 601.2g); the mode costs are added at the cast
			// (CR 702.172a)
			cost := s.featCost(k, p&1, c, Obj(a.Source()), k.cards[c].cost)
			if s.g.mayPlayAny {
				cost = anyType(cost)
			}
			return cost, ex, need, true
		}
		if c := s.objs[a.Source()].card; s.fixedTapsCast(k, a) {
			// a fixed tap of other permanents beside the printed cost: gorge's
			// potential planner prices the composed cost (cost_fixedtaps.go), so
			// the prefix search taps a granted source the census misses (CR 601.2g-h)
			cost := s.featCost(k, p&1, c, Obj(a.Source()), k.cards[c].cost)
			if s.g.mayPlayAny {
				cost = anyType(cost)
			}
			return cost, ex, need, true
		}
		cost, ok = s.plannedCastCostOf(k, p, Obj(a.Source()), a.Mode()&(3|castMayPlay), true, true)
		return cost, ex, need, ok
	}
	return cost, ex, need, false
}

// playNeed is the colours play a's printed base announces (gorge's
// scriptNeedColours): a plain cast's printed mana cost, a printed ability's
// compiled cost. An alternative, kicked, face-down or may-play cast, and the
// impended/evoked alternative costs, leave it empty, as gorge's
// scriptNeedColours does. The read is independent of grantPlayCost's ok: the
// filter search also prices plays the grant search declines (a waterbend or
// behold cast, a target-priced shape), and both searches must branch on the
// same set. The card's printed cost is read unreduced (CR 601.2b announces
// the printed pip colours).
func (s *State) playNeed(k *prep, a Action) (need [kop.NumManaKinds]bool) {
	switch a.Kind() {
	case AkActivate:
		if a.Mode()&0x80 == 0 {
			need = s.abilityNeed(k, Obj(a.Source()), a.Mode())
		}
	case AkCast:
		if a.Mode() == 0 && a.Target() != castImpTgt && a.Target() != castEvokeTgt {
			need = grantNeedColours(k.cards[s.objs[a.Source()].card].cost)
		}
	}
	return need
}

// grantNeedColours reports the colours m's symbols name, gorge's
// scriptNeedColours for a printed mana cost: a coloured pip, a hybrid pip's
// faces, a monocoloured hybrid or Phyrexian pip's colour (kop.HasColour) --
// the printed base's pips, never a reduction's or an additional cost's
// (CR 601.2b announces the printed pip colours before targets). A colourless
// pip is {C}: a granted "{T}: Add {C}" asks its colour too.
func grantNeedColours(m kop.ManaCost) (need [kop.NumManaKinds]bool) {
	for c := 0; c < 5; c++ {
		need[c] = m.HasColour(c)
	}
	need[kop.C] = m.Pips[kop.C] > 0
	return need
}

// abilityNeed is the printed ability ab of object o's colours for
// grantPlayCost's need set (gorge's scriptNeedColours prices a printed
// ability's compiled cost), read from the card's printed cost, unreduced.
func (s *State) abilityNeed(k *prep, o Obj, ab uint8) [kop.NumManaKinds]bool {
	ob := &s.objs[o]
	card := ob.card
	if ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot {
		card = s.cardAt(ob.slot)
	}
	abs := k.t.Cards[card].Abilities
	if int(ab) < len(abs) {
		return grantNeedColours(abs[ab].Cost.Mana)
	}
	return [kop.NumManaKinds]bool{}
}

// needFirst stably reorders the colours of order so the ones need names
// come first, engine order kept inside each half: the mirror of gorge's
// scriptOrderAnswers (potential_script.go), which reorders an ask's valid
// answers the same way so a budget-capped search reaches a colour the play
// announces before the colours it does not. The answer set is unchanged.
func needFirst(order []uint8, need [kop.NumManaKinds]bool) {
	for i := 1; i < len(order); i++ {
		c := order[i]
		if !need[c] {
			continue // a non-needed colour stays: the needed ones already sit before it
		}
		j := i
		for j > 0 && !need[order[j-1]] {
			order[j] = order[j-1]
			j--
		}
		order[j] = c
	}
}

// altPlayCost is what cast a pays when that isn't the card's printed cost: an
// airbend's {2}, a warp's cost, nothing for a free cast, a face-down cast's {3} or
// the card's own alternative cost. A witness priced on the printed cost has no plan.
func (s *State) altPlayCost(k *prep, p uint8, a Action) (kop.ManaCost, bool) {
	if a.Kind() != AkCast {
		return kop.ManaCost{}, false
	}
	o := Obj(a.Source())
	c := s.objs[o].card
	cd := &k.t.Cards[c]
	switch m := a.Mode(); {
	case m == castAirbend:
		return s.airbendCost(k, p, c, o), true
	case m&castWarp != 0:
		return s.adjustCost(k, p, c, o, cd.WarpCost), true
	case m&castFree != 0:
		return kop.ManaCost{}, true
	case m&castFaceDown != 0:
		return s.adjustCost(k, p, c, o, kop.ManaCost{Generic: 3}), true
	case m == 0 && a.Target() == castAltTgt && k.gen != nil && k.gen.altFree != nil && k.gen.altFree[c] != 0:
		return s.adjustCostGen(k, p, c, o, cd.Abilities[k.gen.altFree[c]-1].Cost.Mana), true
	}
	return kop.ManaCost{}, false
}

// FeatGrantAvail is what battlefield object o adds to gorge's
// AvailableMana total through a granted mana ability (rules
// mana_available.go AvailableMana: one free ability counts every colour it
// can make times its amount; a permanent with several free abilities --
// its own beside a granted one -- is omitted): ok is false when o has no
// granted mana ability.
func (s *State) FeatGrantAvail(o Obj) (w int32, ok bool) {
	ob := &s.objs[o]
	k := s.k()
	if ob.zone != kop.ZoneBattlefield || ob.slot == NoSlot || k.gen == nil || len(k.gen.grantPC) == 0 {
		return 0, false
	}
	var gi [maxGrants]int16
	ng := s.grantIdxs(k, ob.slot, &gi)
	if ng == 0 {
		return 0, false
	}
	if ng > 1 || s.ownManaUsable(k, ob.slot) {
		return 0, true // several free abilities: omitted
	}
	pc := s.grantView(k.gen, gi[0], ob.slot)
	m := s.srcMaskAt(pc, ob.slot)
	if m == 0 {
		return int32(pc.manaN), true
	}
	for c := uint8(0); c < kop.NumManaKinds; c++ {
		if m&(1<<c) != 0 {
			w += int32(pc.manaN)
		}
	}
	return w, true
}

// grantsFixedMana reports whether an effect grants sl a mana ability of fixed
// production (a plain tap for a set colour): gorge's shared census lists such a
// source at its battlefield position whatever shape its own ability has.
func (s *State) grantsFixedMana(k *prep, sl Slot) bool {
	var gi [maxGrants]int16
	n := s.grantIdxs(k, sl, &gi)
	for _, i := range gi[:n] {
		gp := &k.gen.grantPC[i]
		if gp.manaTap && !gp.manaSac && !gp.manaSacOnly && !gp.manaChosen && !gp.manaDyn && !gp.manaMulti && gp.manaCol != kop.ColAny && srcMask(gp) != 0 {
			return true
		}
	}
	return false
}

// filterSource: p controls a permanent with a second mana ability that costs
// mana (a filter: {1}, {T}: add one mana of any color). affordsUngrantedFor
// counts a source's first mana ability only, so its verdict on a play such a
// source funds is not gorge's.
func (s *State) filterSource(k *prep, p uint8) bool {
	for m := s.ctrlM[p&1]; m.any(); m = m.rest() {
		c := s.cardAt(m.first())
		if k.cards[c].manaAb < 0 {
			continue
		}
		cd := &k.t.Cards[c]
		seen := false
		for j := range cd.Abilities {
			a := &cd.Abilities[j]
			if a.Kind != kop.AbMana || a.AFlags&kop.AfLatent != 0 {
				continue
			}
			if seen && a.Cost.Mana.MV() > 0 {
				return true
			}
			seen = true
		}
	}
	return false
}

// askNeededFirst: gorge's search answers the "choose a mana ability" ask in
// scriptOrderAnswers' order, the answers naming a colour the play's cost needs
// first (potential_script.go); the printed Any ability's answer names no colour,
// so a granted fixed-colour ability's needed colours are tried before it, its
// others after (Clement's {G}{U} for a {1}{G}{U} cast, Helga's own restricted
// any colour tried last). It returns, per granted ability of candidate i, the
// colours answered ahead of the printed one.
func (g *grantSrch) askNeededFirst(t *State, i int, sl Slot) (pre [maxGrants]uint8) {
	if !g.dual[i] {
		return pre
	}
	var need uint8
	for c, b := range g.need {
		if b {
			need |= 1 << c
		}
	}
	if need == 0 {
		return pre
	}
	var own uint8
	if ra := t.ownRestrAb(g.k, sl); ra != nil && !t.ownManaUsable(g.k, sl) {
		own = uint8(ra.ManaColors) & 0x3f
	} else {
		own = t.srcMaskAt(t.manaPCGen(g.k, sl), sl)
	}
	if own&uint8(kop.ColAny) != uint8(kop.ColAny) && own&need != 0 {
		return pre // the printed ability names a needed colour itself: engine order
	}
	for j := 0; j < int(g.ng[i]); j++ {
		pc := t.grantView(g.k.gen, g.gi[i][j], sl)
		if m := t.srcMaskAt(pc, sl); m&uint8(kop.ColAny) != uint8(kop.ColAny) && grantCostLife(pc) == 0 && t.grantCostPayable(g.p, pc) {
			pre[j] = m & need
		}
	}
	return pre
}

// preBranch taps granted ability j of candidate i for the colours the ask
// answers ahead of the printed ability (askNeededFirst).
func (g *grantSrch) preBranch(t *State, i int, sl Slot, j int, mask uint8, one bool) bool {
	gi := g.gi[i][j]
	pc := t.grantView(g.k.gen, gi, sl)
	g.skipPool = !g.grantAdmits(t, sl, gi)
	return g.branch(t, i, sl, gi, mask, pc.manaN, one)
}

// restrAbN is the mana a tap of sl's printed restricted ability ra makes: an
// evaluated amount (Helga's X, her power) is read off the board, where kgen's
// ManaN is the placeholder 1 for every dynamic ability.
func (s *State) restrAbN(k *prep, sl Slot, ra *kop.Ability) uint8 {
	if ra.AFlags&kop.AfManaDynamic != 0 {
		return uint8(max(s.dynManaN(k, ra, sl), 1))
	}
	return uint8(max(ra.ManaN, 1))
}
