package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Tap-another-permanent mana (Saruli Caretaker, Springleaf
// Drum: "{T}, Tap an untapped creature you control: Add one mana of any
// color"). Before this file the kernel dropped the ability silently (its tap
// walk reads only a mana-free {T} or sacrifice); kgen now refuses every cost
// shape kop.ManaAbilityHonoured does not list, and honours this one.
//
// gorge's payment planner census cannot price the cost's creature choice
// (the source is a "relaxable" census gap with a free any-colour relaxed
// alternative, rules potential_plan.go paymentPlanRelaxProof), so autopay
// and the cast windows never use it (the mana walk here does not either:
// canTapForManaPC is false for it). The unless and ward windows list it and ask
// which permanent to tap (mana_windowtapx.go). A play only its mana pays is a scripted
// potential play (rules potentialPrefixScript): a depth-first search on
// clones that activates the gaps at priority, branching on every answer of
// the activation in engine order -- the creature to tap (the "tapcost" ask,
// only with two or more candidates; battlefield order), then the colour
// (W U B R G) -- and stops at the first payer priority where the play is
// listed. The kernel models it as one more script gap kind beside the
// each-colour, restricted, sacrifice-only and counter ones (script_gen.go):
// its any-colour unit in the gaps walk (availTapX), and at the play the same
// search on copies (tapXSearch, after reflSearch), its path then played on
// the state. The model covers boards whose census gaps are tap-another-
// permanent sources and counter-cost ones (Wall of Roots: one answer, no
// ask); beside an each-colour, restricted or sacrifice-only gap the tapX sources
// are left out entirely (modelled, tapXBoard), so a play only their mana pays is
// not offered rather than offered and unpayable, and a board that mixes them with
// a filter or a restricted any-colour source runs the filter search (mixBoard).
// Everything is behind prep.tapMana. Not modelled: the counters and the
// battlefield members gorge's dedup key reads beside (pool, tapped set,
// stack, hand and graveyard sizes).
//
// A sacrifice-another source (Bolg's Company: "{T}, Sacrifice another Goblin:
// Add {B}{R}", kop.SacXEachManaAb) is one more source kind of this search.
// gorge's census defers it (pay.PaymentPlanShapeTierOf: a Sac part that is not
// the source) and relaxes it to one free fixed alternative of its whole
// production (paymentPlanRelaxedAlternatives), so the gaps walk counts one of
// each of its colours (addTapXUnits). Activating it asks which permanent to
// sacrifice (pay.ManaCostChoiceStages: a "sacrifice" choose, only with two or
// more candidates, battlefield order, pay.ManaSacrificeCandidates), and asks
// no colour; it commits in gorge's order (commitManaDiscard): the source's
// {T}, the sacrifice (whose dies triggers queue), the mana (sacXDo). A path
// that sacrifices the searched ability's own source dead-ends at the next node
// (gorge scriptSearch.leftOwnZone). The manual ward and unless windows do not
// offer it (windowTapXOffer), nor does the grant search (grantTapXGaps) —
// beside a granted mana ability it activates the source as a candidate of its
// own now (mana_grantsacx.go).

// DebugNoTapMana hides the tap-another-permanent gaps (the caretaker
// mutation test proves the pool's plays need them).
var DebugNoTapMana bool

// DebugNoSacXMana hides only the sacrifice-another sources (kop.SacXEachManaAb;
// the Bolg's Company mutation test proves the pool's plays need them).
var DebugNoSacXMana bool

// sacXOf: card c's tap-another gap is the sacrifice-another all-at-once shape
// (kop.SacXEachManaAb: Bolg's Company).
func (k *prep) sacXOf(c CardID) bool {
	a := k.tapXAbOf(c)
	return a != nil && kop.SacXMana(a)
}

// debugTapXMutate selects a lockstep mutation: 1 = the first colour always
// (the search ignores the play's pips), 2 = the last creature candidate.
var debugTapXMutate int

const maxTapXPath = 24

// tapXAbOf is card c's tap-another-permanent mana ability (prep.tapXAb): its
// first mana ability (Saruli Caretaker), or a later one beside a plain {T}
// first (Scene of the Crime, kop.SecondTapXManaAb), nil none. The later one's
// source is a planner unit too (its first ability's {C}): gorge's relaxed
// census appends the gap's free any-colour unit beside it (rules
// paymentPlanRelaxProof), and the prefix search answers the activation's
// "which ability" ask with this one (CR 605.3b, 601.2g).
func (k *prep) tapXAbOf(c CardID) *kop.Ability {
	if int(c) >= len(k.tapXAb) || k.tapXAb[c] < 0 {
		return nil
	}
	return &k.t.Cards[c].Abilities[k.tapXAb[c]]
}

// scriptTapX: p's tap-another-permanent sources that can be activated now
// (untapped, not summoning sick, with an untapped candidate), a slot mask.
//
//go:noinline
func (s *State) scriptTapX(k *prep, p uint8) slotMask {
	return s.tapXSources(k, p, true)
}

// tapXCensusGap: p's tap-another-permanent sources gorge's window census
// leaves unpriced (castWindowSourceExcluded): its probe admits a mana ability
// whose tapXType cost it cannot price, so such a source makes the census
// incomplete (CR 601.2g). One with too few untapped candidates to tap right now
// is no gap: like a tapped source it has nothing to offer this window
// (castWindowTapPartsShort), so an untapped Springleaf Drum beside no untapped
// creature leaves the census complete.
func (s *State) tapXCensusGap(k *prep, p uint8) slotMask {
	return s.tapXSources(k, p, true)
}

// tapXSources: scriptTapX, the candidate count read only when cands is set.
func (s *State) tapXSources(k *prep, p uint8, cands bool) slotMask {
	var m slotMask
	if !k.tapMana || DebugNoTapMana {
		return m
	}
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		sl := b.first()
		if s.pickedForMana(s.perms[sl].obj) {
			continue // a Convoke/Waterbend pick is tapped for that cast, not for mana
		}
		a := k.tapXAbOf(s.cardAt(sl))
		if a == nil || DebugNoSacXMana && kop.SacXMana(a) {
			continue
		}
		pm := &s.perms[sl]
		if a.Cost.Flags&kop.CostTap != 0 && (pm.flags&pfTapped != 0 || s.activationSick(k, sl)) || s.genLost(k, sl) ||
			k.gen != nil && k.gen.anyCantAct && s.actForbidden(k, sl, true) { // the source's lock only: the tapped permanent activates nothing
			continue // (a cost with no {T} needs neither an untapped nor an unsick source)
		}
		var cs [MaxPerm]Slot
		if !cands || s.tapXCands(k, p, sl, &cs) >= int(a.Cost.N) {
			m = m.with(sl)
		}
	}
	return m
}

// sacXLive: p has a live sacrifice-another source (Bolg's Company).
func (s *State) sacXLive(k *prep, p uint8) bool {
	for b := s.scriptTapX(k, p); b.any(); b = b.rest() {
		if k.sacXOf(s.cardAt(b.first())) {
			return true
		}
	}
	return false
}

// tapXTwoLive: a no-{T} two-permanent mana source (Baylen) is activatable now
// on a board the search models. Its cost is not once-only, so gorge's
// census is no longer relaxable (paymentPlanRelaxedAlternatives) and every
// census-incomplete insufficient play takes a search slot.
//
//go:noinline
func (s *State) tapXTwoLive(k *prep, p uint8) bool {
	return s.tapXTwoLiveIn(k, s.scriptTapXModelled(k, p))
}

// tapXTwoLiveIn: b holds a live no-{T} unbounded tap-another source (Relic of
// Legends, Baylen), the shape whose cost is not once-only and whose census
// gorge's filter/restricted searches refuse (filtFundable, tapXTwoLive).
func (s *State) tapXTwoLiveIn(k *prep, b slotMask) bool {
	for ; b.any(); b = b.rest() {
		if ab := k.tapXAbOf(s.cardAt(b.first())); ab.Cost.Flags&kop.CostTap == 0 && !kop.BoundedOnceManaAb(ab) {
			return true
		}
	}
	return false
}

// tapXSacAnyLive: a no-{T} sacrifice source with a fixed production
// (kop.SacAnyManaAb: Krark-Clan Ironworks) is activatable now on a board the
// search models. Like Baylen's, its cost is not once-only, so gorge's census is
// unrelaxable (a repeatable cost that is not a bounded tap part) and every
// census-incomplete insufficient play takes a search slot; unlike Baylen's
// any-colour production, its potential bound is its fixed production once
// (PotentialMana admits the ability a single time), so the plays searched are
// the gaps walk's, not every play an open bound would list.
//
//go:noinline
func (s *State) tapXSacAnyLive(k *prep, p uint8) bool {
	for b := s.scriptTapXModelled(k, p); b.any(); b = b.rest() {
		if kop.BoundedOnceManaAb(k.tapXAbOf(s.cardAt(b.first()))) {
			return true
		}
	}
	return false
}

// sacRepeatLive: a live kop.SacAnyManaAb source whose cost sacrifices dedicated
// tokens -- the cost's filter requires a token (Jungle Patrol's "Sacrifice a
// token named Wood: Add {R}") -- so gorge's paymentPlanRelaxedAlternatives once
// is false on a closed bound too: the census is unrelaxable and runs no bounded
// proof, so gorge searches every census-incomplete insufficient play, as beside
// a live Baylen (tail-14-c000 pair 1: two Wood sacrifices and the untapped
// land's {B} pay Bone Sabres' Equip {3}).
//
// A sacrifice of a permanent TYPE (Krark-Clan Ironworks' artifact, Ashnod's
// Altar's or Phyrexian Altar's creature) is NOT this shape, though it too is
// kop.SacAnyManaAb: its fodder may be a mana source or the source itself, so
// the potential bound already prices the activations it can make and the KCI
// recordings pin the closed-bound narrowing (sacDrops). A {T} sacrifice-another
// one (Bolg's Company) is no shape either: its {T} makes once true, the relaxed
// proof holds, and the closed-bound candidates stay the gaps walk's.
func (s *State) sacRepeatLive(k *prep, p uint8) bool {
	if !k.tapMana {
		return false
	}
	for b := s.scriptTapXModelled(k, p); b.any(); b = b.rest() {
		ab := k.tapXAbOf(s.cardAt(b.first()))
		if kop.SacAnyManaAb(ab) && k.t.Filters[ab.Cost.Filter].Flags&kop.FToken != 0 {
			return true
		}
	}
	return false
}

// availSacAny adds each live sacrifice-any source's fixed production once to
// av, the potential census for payment d: gorge's PotentialMana bound admits
// the ability a single time. When the census already counted it -- the printed
// sacrifice-mana arm potentialGapCost admits while the sacrifice has its
// candidates, as gorge's walk does -- adding it again would count gorge's one
// admission twice (Skirk Prospector beside two Mountains priced Stingscourger's
// echo {3}{R} payable on a third {R}).
func (s *State) availSacAny(k *prep, p uint8, av *avail, d *payDesc) {
	for b := s.scriptTapXModelled(k, p); b.any(); b = b.rest() {
		sl := b.first()
		if ab := k.tapXAbOf(s.cardAt(sl)); kop.BoundedOnceManaAb(ab) &&
			!s.potentialAdmits(k, p, sl, int(k.tapXAb[s.cardAt(sl)]), ab, d) {
			addTapXUnits(av, ab)
		}
	}
}

// tapXSacAnyBound splits a live sacrifice-any source (tapXSacAnyLive) by
// gorge's PotentialMana bound (availPotentialFor). Closed: the bound counts its
// fixed production once, so the potential walk lists the gaps walk's plays.
// Open (an untapped any-colour source beside it, a Treasure: potentialUnbounded
// in every colour): the walk lists every play, and the census is neither
// relaxable nor bounded (paymentPlanRelaxedAlternatives: repeatable, no tap
// part), so each listed play takes a search slot and is searched, as beside a
// live Baylen. The search may then sacrifice the any-colour source to the
// Ironworks and the Ironworks to itself.
func (s *State) tapXSacAnyBound(k *prep, p uint8) (closed, open bool) {
	if !k.tapMana || !s.tapXSacAnyLive(k, p) {
		return false, false
	}
	var av avail
	open = s.availPotentialFor(k, p, &av, nil, false)
	return !open, open
}

// scriptTapXModelled is scriptTapX on the boards the search models: none
// beside a gap kind it does not run (each-colour, restricted, sacrifice-only,
// filter).
func (s *State) scriptTapXModelled(k *prep, p uint8) slotMask {
	if !k.tapMana || !s.tapXBoard(k, p) {
		return slotMask{}
	}
	return s.scriptTapX(k, p)
}

// tapXEachGaps: p's each-colour sources the tap-another search activates beside
// its own gaps: none unless a tap-another source is live on the board.
func (s *State) tapXEachGaps(k *prep, p uint8) slotMask {
	if !k.tapMana || !s.scriptTapX(k, p).any() || s.scriptRestr(k, p).or(s.scriptFilt(k, p)).any() {
		return slotMask{}
	}
	return s.scriptGaps(k, p)
}

// tapXAnyRBoard: p's any-colour restricted sources (a Flamebraider) sit
// beside a live tap-another source and gaps the filter search does not take
// (mixBoard: an each-colour or counter-cost one). gorge's one prefix search
// activates all of them in battlefield order, so the tap-another search takes
// the restricted source as a step of its own (tapXAnyR). A reflected-mana
// board (a Plaza) stays the reflected search's.
func (s *State) tapXAnyRBoard(k *prep, p uint8) bool {
	return k.tapMana && !k.gen.anyRefl && s.scriptTapX(k, p).any()
}

// tapXAnyRGaps: p's any-colour restricted sources the tap-another search
// activates beside its own gaps (tapXAnyRBoard).
func (s *State) tapXAnyRGaps(k *prep, p uint8) slotMask {
	if !s.scriptRestr(k, p).or(s.scriptFilt(k, p)).any() && s.tapXAnyRBoard(k, p) {
		return s.scriptAnyR(k, p)
	}
	return slotMask{}
}

// riderDeferMask is p's activatable bounded-rider sources, a slot mask (the
// rider arm of scriptTapsDefer, mana_tapsdefer.go, without that walk's
// tap-another-live guard: the search reads activeness per node on its copy,
// where the Drum it just tapped no longer makes the tap-another route live).
func (s *State) riderDeferMask(k *prep, p uint8) slotMask {
	var m slotMask
	for b := s.scriptTapsDefer(k, p); b.any(); b = b.rest() {
		sl := b.first()
		if _, ok := s.riderAbilityOf(k, sl); ok {
			m = m.with(sl)
		}
	}
	return m
}

// tapXRiderGaps: p's planner-deferred rider sources (a Sandstone Needle's
// "{T}, remove a depletion counter: Add {R}{R}", its sacrifice rider) the
// tap-another search activates beside its own gaps: none unless a
// tap-another source is live on the board (tapXEachGaps' guard), and not on a
// board the filter or restricted-any route owns (mixBoard).
func (s *State) tapXRiderGaps(k *prep, p uint8) slotMask {
	if !k.tapMana || !s.scriptTapX(k, p).any() || s.scriptRestr(k, p).or(s.scriptAnyR(k, p)).or(s.scriptFilt(k, p)).any() {
		return slotMask{}
	}
	return s.riderDeferMask(k, p)
}

// tapXActivatable reports whether the source sl's activatable mana ability is
// its tapXType one and its tap-another part has enough untapped candidates to
// pay right now: the ability is activatable (gorge untappedManaSource reads
// the ability's payability gate, pay.ManaTapsPayable, so such a source is an
// untapped mana source the CR 601.2g window may tap). The source's own lock is
// keyed on the cost's {T}, as tapXSources' gates are: a cost with no {T} needs
// neither an untapped nor an unsick source (gorge mana_available.go rejects
// only mf.Tap && o.Tapped; pay.TapFlagsSick gates the ability's own tap). An
// X-form tap (Cost.N 0) is affordable at X=0 and never falls short; a card
// whose usable ability is a plain one is canTapForMana's, not this read.
func (s *State) tapXActivatable(k *prep, p uint8, sl Slot) bool {
	c := s.cardAt(sl)
	ab := k.tapXAbOf(c)
	if ab == nil {
		return false
	}
	pc := &k.cards[c]
	if k.gen != nil {
		pc = s.manaPCGen(k, sl) // the profile canTapForMana priced
	}
	if pc.manaAb < 0 || k.tapXAb[c] != pc.manaAb {
		return false // the card's usable ability is a plain one
	}
	if ab.Cost.Flags&kop.CostTap != 0 &&
		(s.perms[sl].flags&pfTapped != 0 || s.activationSick(k, sl)) ||
		k.gen != nil && k.gen.anyCantAct && s.actForbidden(k, sl, true) {
		return false // the source's lock only (tapXSources' gates)
	}
	var cs [MaxPerm]Slot
	return s.tapXCands(k, p, sl, &cs) >= int(ab.Cost.N)
}

// tapXCands lists the permanents source sl's tap cost can tap, in gorge's
// candidate order (battlefield entry): p's untapped permanents matching the
// cost's filter, the source itself excluded (its own {T} is paid first;
// pay.ManaTapCandidates claims it). A sacrifice-another source's are the
// permanents its sacrifice can take (sacXCands).
func (s *State) tapXCands(k *prep, p uint8, sl Slot, out *[MaxPerm]Slot) int {
	ab := k.tapXAbOf(s.cardAt(sl))
	if ab.Cost.Flags&kop.CostExileGrave != 0 {
		return s.exileGCands(k, p, sl, ab, out) // graveyard indices (mana_exilegrave.go)
	}
	if ab.Cost.Flags&kop.CostSac != 0 {
		return s.sacXCands(k, p, sl, ab, out)
	}
	f := ab.Cost.Filter
	var m slotMask
	// A creature the cast being paid for announced for convoke is tapped when
	// that payment settles, so it is not untapped to tap again here (CR 601.2g-h;
	// gorge pay.ManaTapCandidates, CastCommitted). The activation being paid
	// for likewise keeps the source its own {T} cost taps (buildSelfTap).
	var spoken slotMask
	selfTap := NoSlot
	if s.buildLive() {
		spoken = s.convokeHidden()
		selfTap = s.buildSelfTap(k)
	}
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		x := b.first()
		if x == sl && ab.Cost.Flags&kop.CostTap != 0 || s.perms[x].flags&pfTapped != 0 || spoken.has(x) || x == selfTap || !s.matchPerm(k, f, x, p, sl) {
			continue
		}
		m = m.with(x)
	}
	return s.battlefieldOrder(p, m, out[:], 0)
}

// buildSelfTap is the slot of the activation being paid for when its own cost
// taps it for {T}: that permanent is committed to the payment and cannot also
// pay a mana ability's tap or sacrifice part (CR 601.2g-h, the whole cost is
// paid at once; gorge pay.ManaTapCandidates and pay.ManaSacrificeCandidates,
// CastCommitted's pc.cost.Tap arm). NoSlot when no such activation is live.
func (s *State) buildSelfTap(k *prep) Slot {
	if c := s.buildPickCost(k); c != nil && c.Flags&kop.CostTap != 0 {
		return s.srcSlot()
	}
	return NoSlot
}

// censusTapXClaim admits sl's tap-another mana ability j in the potential
// walk's census and claims the permanents its cost taps. pay.ManaTapCandidates
// elects and claims each candidate once, so two sources sharing a candidate
// pool fund one activation, not two: two Birchlore Rangers both see the same
// two Elves, but the first activation taps them (CR 601.2g). A source with no
// unclaimed candidate left contributes no unit, and the claim is skipped when
// the census is off (the over-bound fold keeps its per-source answer).
func (s *State) censusTapXClaim(k *prep, p uint8, sl Slot, j int, ab *kop.Ability, cen *potCensus) bool {
	if !cen.on || ab == nil || !kop.TapXMana(ab) || int(k.tapXAb[s.cardAt(sl)]) != j {
		return true
	}
	need := int(ab.Cost.N)
	if need <= 0 {
		return true
	}
	var cs [MaxPerm]Slot
	n := s.tapXCands(k, p, sl, &cs)
	left := 0
	for i := 0; i < n; i++ {
		if !cen.tapX.has(cs[i]) {
			left++
		}
	}
	if left < need {
		return false
	}
	for i, got := 0, 0; i < n && got < need; i++ {
		if x := cs[i]; !cen.tapX.has(x) {
			cen.tapX.set(x)
			got++
		}
	}
	return true
}

// sacXCands lists the permanents sacrifice-another source sl's cost can
// sacrifice, in gorge's candidate order (pay.ManaSacrificeCandidates: p's
// battlefield in entry order): those matching the cost's filter, none a
// CantSacrifice static withholds, none the cast being paid announced for
// convoke (it is tapped when that payment settles: CastCommitted), and none
// the activation being paid for taps for its own {T} (buildSelfTap). Tapped
// ones are candidates. The source itself is one when its own filter matches
// it: "sacrifice a creature" can sacrifice the creature that is also the
// source (CR 701.21a, 602.2b; Phyrexian Tower animated into a creature). The
// filter's FOther arm is what excludes the source for "sacrifice ANOTHER
// creature" (matchPerm), not this walk.
func (s *State) sacXCands(k *prep, p uint8, sl Slot, ab *kop.Ability, out *[MaxPerm]Slot) int {
	var spoken, m slotMask
	selfTap := NoSlot
	if s.buildLive() {
		spoken = s.convokeHidden()
		selfTap = s.buildSelfTap(k)
	}
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		x := b.first()
		if spoken.has(x) || x == selfTap || !s.matchPerm(k, ab.Cost.Filter, x, p, sl) || s.sacBlocked(k, x) {
			continue
		}
		m = m.with(x)
	}
	return s.battlefieldOrder(p, m, out[:], 0)
}

// availTapX adds p's tap-another-permanent sources' mana to the gaps walk:
// the relaxed census counts each as a free any-colour unit (a second
// ability's beside its source's own planner unit), a sacrifice-another one
// as one of each of its colours (addTapXUnits).
//
//go:noinline
func (s *State) availTapX(k *prep, p uint8, av *avail) {
	for b := s.scriptTapXModelled(k, p); b.any(); b = b.rest() {
		addTapXUnits(av, k.tapXAbOf(s.cardAt(b.first())))
	}
}

// addTapXUnits adds tap-another ability a's relaxed unit to av: one unit of
// its colours, or for a sacrifice-another all-at-once one (Bolg's Company)
// one of each colour (gorge's relaxed alternative is the whole {B}{R}).
func addTapXUnits(av *avail, a *kop.Ability) {
	cols, n := tapXUnit(a)
	if !kop.SacXEachManaAb(a) {
		av.addUnits(cols, n)
		return
	}
	for c := 0; c < kop.NumManaKinds; c++ {
		if cols&(1<<c) != 0 {
			av.addUnits(1<<c, n)
		}
	}
}

// tapXUnit is the colours and amount of tap-another ability a's unit.
func tapXUnit(a *kop.Ability) (uint8, uint8) {
	return uint8(a.ManaColors), max(a.ManaN, 1)
}

// scriptCtrModelled is scriptCtr on the boards the search models: the counter-cost
// source activates in the same prefix search as a tap-another-permanent one (rules
// scriptSearch.visit tries every census gap in battlefield order, depth first), so a
// play only a Wall of Roots pays is offered when some chain of activations pays it:
// one the Wall's death before an Overgrown Battlement counts the defenders leaves
// short is not.
func (s *State) scriptCtrModelled(k *prep, p uint8) slotMask {
	if !s.tapXBoard(k, p) {
		return slotMask{}
	}
	return s.scriptCtr(k, p)
}

// tapXTwoLiveSource: p has a live no-{T} tap-another source (Relic of
// Legends, Baylen), the shape whose cost is not once-only and whose census
// the filter/restricted searches refuse (filtFundable, tapXTwoLive). It
// reads scriptTapX directly so tapXBoard can call it without recursing
// through scriptTapXModelled.
func (s *State) tapXTwoLiveSource(k *prep, p uint8) bool {
	return s.tapXTwoLiveIn(k, s.scriptTapX(k, p))
}

// tapXBoard: p's census gaps are tap-another-permanent and counter-cost
// sources only, those beside each-colour sources (a Bloom Tender; the search
// activates them as steps, tapXEach), or a mix the filter search runs
// (mixBoard).
func (s *State) tapXBoard(k *prep, p uint8) bool {
	if s.scriptRestr(k, p).or(s.scriptFilt(k, p)).any() {
		// A live no-{T} tap-another source (Relic of Legends, Baylen) is the
		// tapX search's alone beside these gaps: the filter/restricted
		// searches are disabled for it (filtFundable, tapXTwoLive), so their
		// branch would drop every play the base sources plus that source pay.
		// The tapX search leaves the filter/restricted sources to the base
		// walk, so it cannot spend their mana on a play it refuses.
		if s.tapXTwoLiveSource(k, p) {
			return true
		}
		return s.mixBoard(k, p)
	}
	if s.scriptAnyR(k, p).any() {
		return s.mixBoard(k, p) || s.tapXAnyRBoard(k, p)
	}
	return !s.scriptGaps(k, p).any() || s.scriptTapX(k, p).any()
}

// tapXDeadEndRoute reports whether the board's Taps-deferred gaps (tapsDef,
// non-empty) belong to the dead-end route, not the tap-another search: no
// real tap-another source (a rider gap beside counter-cost ones, the DFS
// search's rider step), or one the restricted routes own (tapXRiderGaps'
// guard, whose mask empties beside them). Where the search's defer arm holds
// the gaps (mana_tapxdefer.go, a Burning Earth deferring the play's own land
// beside Ashnod's Altar, tx the modelled sources) the prefix is the search's.
// scriptOffers' tx arm and tapXPrefix share it, so offer and replay agree.
func (s *State) tapXDeadEndRoute(k *prep, p uint8, tx, tapsDef slotMask) bool {
	return tapsDef.any() && !s.tapXRiderGaps(k, p).any() &&
		(s.scriptRestr(k, p).or(s.scriptAnyR(k, p)).any() || !s.tapXDeferGaps(k, p, tx).any())
}

// tapXPrefix plays the search's script for play a at a tapX board; it
// reports whether it handled the board (the caller then does nothing more).
// A Taps-deferred rider gap beside counter-cost ones and no live real
// tap-another source (tapXRiderGaps empty, or the restricted routes' board
// where that mask empties) is the scriptDeadEndAny route's
// (scriptPrefixAny, mana_tapsdefer.go): the search here has no rider step
// there, so a play the rider's mana funds would be left unfunded. Where its
// defer arm holds the Taps-deferred gaps (mana_tapxdefer.go: a real
// tap-another source is modelled, a Burning Earth deferring the play's own
// land beside Ashnod's Altar) the search branches on them, and the board's
// prefix is this search's.
//
//go:noinline
func (s *State) tapXPrefix(k *prep, a Action, p uint8) bool {
	tx := s.scriptTapXModelled(k, p).or(s.scriptCtrModelled(k, p))
	if !tx.any() || s.tapXDeadEndRoute(k, p, tx, s.scriptTapsDefer(k, p)) {
		return false
	}
	var d Decision
	s.plainOptions(k, &d, 0)
	if !hasOption(&d, a) || s.dynSacScriptPriced(k, a, p) {
		s.tapXScript(k, a, p, true)
	}
	return true
}

// tapXStep is one step of the search's path: an activation (source sl taps
// candidate cand and makes col), an answer to an ask the activation's trigger
// posed (act: the target of a Taps listener's trigger, an order, a discard),
// or p's pass while that trigger resolves.
type tapXStep struct {
	sl, cand Slot
	cand2    Slot // the second tapped permanent of a two-permanent cost (NoSlot: none)
	col      uint8
	kind     uint8
	gi       int16 // the granted ability a tapXGrant step taps
	act      Action
}

const (
	tapXAct          uint8 = iota // an activation
	tapXAnswer                    // act answers the ask then pending
	tapXPass                      // p passes with a trigger of the path on the stack
	tapXEach                      // an each-colour gap (a Bloom Tender) taps for its colours
	tapXPlain                     // a second-ability source answers its ability ask with its plain first ability (tapXPlainDo)
	tapXAnyR                      // an any-colour restricted gap (a Flamebraider), col its scriptDFS answer (scriptGapTap)
	tapXRider                     // a rider source activates its bounded rider ability (Sandstone Needle: mana_rider.go)
	tapXDefer                     // a Taps-deferred gap (a depletion land) taps for its answer col; act is the searched play (mana_tapxdefer.go)
	tapXAnyDifferent              // a Firemind Vessel (AnyDifferent) taps for N distinct colours: gorge's search activates it as a census gap
	tapXGrant                     // a plain granted {T} mana ability's holder taps for col (mana_tapxgrant.go)
)

type tapXKey struct {
	pool   [kop.NumManaKinds]uint8
	art    uint8
	tapped slotMask
	sp     uint8 // gorge's key counts the stack, hand and graveyard too
	hand   uint8
	gy     uint8
	// bf: p's battlefield objects (gorge's key lists each by id): the Lander
	// a tapped Seedship Agrarian's trigger made is a new state, though its
	// own enters trigger leaves the stack as deep as the first did
	bf uint64
	rb [maxRB]rbatch // restricted mana floating (gorge's key counts every mana unit)
}

type tapXSearch struct {
	a       Action
	p       uint8
	own     Slot // the source whose {T} is the play's own cost (NoSlot: none)
	sac     bool // a sacrifice-another source is among the gaps (leftOwnZone reads srcObj)
	src     Obj  // the searched ability's source (NoObj: a cast)
	srcZ    kop.Zone
	sp0     uint8 // the stack at the root: only a deeper one is the path's to resolve
	gaps    slotMask
	grants  slotMask               // the plain granted-mana holders among gaps (tapXGrantGaps)
	def     slotMask               // the Taps-deferred gaps among gaps (tapXDeferGaps)
	anyDiff slotMask               // the AnyDifferent sources among gaps: gorge's census lists them, they add N distinct colours
	need    [kop.NumManaKinds]bool // the play's announced colours (scriptNeedColours)
	stall   uint8                  // the first stall a failed line's fork latched (stallOf), StallNone none
	nodes   int
	nseen   int
	seen    [potentialScriptNodes]tapXKey
	n       int
	path    [maxTapXPath]tapXStep
}

// DebugNoTapXOwnSkip keeps the answers that tap the play's own {T} source in
// both prefix searches (mutation testing only: the lockstep must catch it).
var DebugNoTapXOwnSkip bool

// tapXOwnAnswer: a tap-another cost's answer that taps a or b, one of them the
// source whose {T} the searched play's own cost needs, is no answer the search
// tries. gorge's scriptSearch skips it (tapsOwnSource): the play could not be
// paid after it (CR 602.2b), though the node pricing, which reads no tapped
// state, would call it payable. Only an asked answer is skipped: with exactly
// the cost's count of candidates nothing is asked and both are tapped.
func (rs *tapXSearch) tapXOwnAnswer(a, b Slot) bool {
	return tapXOwnAnswer(rs.own, a, b)
}

func tapXOwnAnswer(own, a, b Slot) bool {
	return !DebugNoTapXOwnSkip && own != NoSlot && (a == own || b == own)
}

// tapXPairLimit returns how many pairs of positions below n the two prefix
// searches try: C(n,2), gorge's 64-answer cap at most.
func tapXPairLimit(n int) int {
	np := n * (n - 1) / 2
	if np > 64 {
		np = 64
	}
	return np
}

// tapXPairAt returns the qi-th pair of positions i < j below n in
// lexicographic order: the order gorge's scriptAnswers enumerates the
// n-combinations of a multi-object election in.
func tapXPairAt(n, qi int) (int, int) {
	for i := 0; i < n; i++ {
		row := n - 1 - i
		if qi < row {
			return i, i + 1 + qi
		}
		qi -= row
	}
	return 0, 0
}

// tapXScript runs gorge's prefix search (rules scriptSearch.visit under
// potentialPrefixScript: gapsOnly, funding 0, potentialScriptNodes) for play
// a of p: found. With apply it plays the found script on s. When not found,
// stall is the first rule-gap stall a failed line's fork latched (stallOf;
// StallNone none): the search declares nothing — a caller that keeps searching
// may still price the play, and the offer walk declares the stall only where
// it drops it (tapXScriptOffered, scriptOffers, grantCap).
//
//go:noinline
func (s *State) tapXScript(k *prep, a Action, p uint8, apply bool) (bool, uint8) {
	var rs tapXSearch
	rs.a, rs.p, rs.sp0, rs.gaps = a, p, s.sp, s.scriptTapXModelled(k, p).or(s.scriptCtrModelled(k, p)).or(s.tapXEachGaps(k, p)).or(s.tapXAnyRGaps(k, p)).or(s.tapXRiderGaps(k, p))
	rs.def = s.tapXDeferGaps(k, p, rs.gaps)
	rs.gaps = rs.gaps.or(rs.def)
	rs.anyDiff = s.anyDifferentGaps(k, p)
	rs.gaps = rs.gaps.or(rs.anyDiff)
	if gapFilteredPlay(a) && rs.gaps.any() {
		// gorge's prefix search keeps only the gaps the play's payment can
		// spend (scriptSpendableGaps, CR 106.6): Master of Dark Rites' mana
		// pays no Villain, and its sacrifice never reaches a discount
		d := s.grantPayDesc(k, a)
		rs.gaps = s.gapSpendableMask(k, rs.gaps, &d)
	}
	rs.need = s.playNeed(k, a) // the colour ask's order follows the play's pips
	rs.own, rs.src = NoSlot, NoObj
	if a.Kind() == AkActivate {
		_, rs.own, _ = s.activationMana(k, Obj(a.Source()), a.Mode())
		rs.src = Obj(a.Source())
		rs.srcZ = s.objs[rs.src].zone
	}
	for b := rs.gaps; b.any() && !rs.sac; b = b.rest() {
		rs.sac = k.sacXOf(s.cardAt(b.first()))
	}
	if s.tapXGrantBoard(k, p, a) {
		// beside a sacrifice-any source the grant holders are gaps of this search (mana_tapxgrant.go)
		rs.grants, _ = s.tapXGrantGaps(k, p, rs.gaps)
		rs.gaps = rs.gaps.or(rs.grants)
	}
	t := s.fork()
	defer s.unfork()
	// granted mana is the grant search's (grantPrefix / grantCap), for a cast as
	// for an activation: no gap of this search activates a grant, so none prices it
	t.g.noGrant = true
	if !rs.settle(k, t, false) || rs.n == 0 {
		// not found. A line that ran into a rule gap the kernel stalls by name
		// (gorge asks the order the kernel cannot model; the play would be
		// offered only its mana pays it) is reported to the caller, never
		// declared here: the callers keep searching (tapXScriptOffered's
		// fall-through beside the grants, grantCap's grantSearch) and may still
		// price the play, and a budget-stopped search leaves later lines
		// untried. A found path is not this: tapXPlay replays it on s, and a
		// stall on it latches there of its own (filtSearch's fs.spill).
		return false, rs.stall
	}
	if apply {
		s.tapXPlay(k, p, rs.path[:rs.n])
	}
	return true, 0
}

// witnessReaches is visit's gapUsed acceptance' witness for the plan a node's
// options offer for p's plain cast (gorge's scriptSearch.priced runs the same
// question, potential_witness_targets.go witnessAnnouncesBeforePaying): a
// spell payable only by paying with a target it announced has no witness, so
// the node is no terminal. When poolOnly is false the planner prices what the
// floating pool lacks on a fork — kickWitness's arm: a plan that prices no
// plan, consumes no sacrifice, or leaves every mandatory target leaves the
// node as the options read it, and one whose sacrifices' triggers end the
// game in p's favour is still taken (witnessEndsGameWon, gorge's winOK
// fallback). A pool-only plan is the steps and the floating pool: their
// sacrifices are the steps' own, read off this state.
func (rs *tapXSearch) witnessReaches(k *prep, t *State, poolOnly bool) bool {
	o := Obj(rs.a.Source())
	if t.objs[o].zone != kop.ZoneHand {
		return true // a graveyard or exile cast announces its targets from the stack (spellTargetsFeasibleFrom)
	}
	c := t.objs[o].card
	ab := spellAb(k, c)
	if ab == 255 || !t.anySacSource(k, rs.p) {
		return true // no mandatory target the plan could strand, no source the plan could sacrifice
	}
	abp := &k.t.Cards[c].Abilities[ab]
	if !t.abilityTargetsFeasible(k, abp, rs.p, NoSlot, 0xff) {
		return false // the plan's steps ate a target candidate
	}
	if poolOnly {
		return true // the plan is the steps and the floating pool: no payment left to eat one
	}
	f := t.fork()
	defer t.unfork()
	cost := f.featCost(k, rs.p, c, o, k.cards[c].cost)
	n0 := f.npend
	var pk planner
	pk.try = true
	f.tapForPK(k, rs.p, &cost, 0, NoObj, false, false, &pk)
	if !pk.found || !plannerConsumesSac(&pk) || f.abilityTargetsFeasible(k, abp, rs.p, NoSlot, 0xff) {
		return true
	}
	return f.witnessEndsGameWon(k, rs.p, c, n0)
}

// tapXNext runs t to p's next decision, passing for the other player while a
// trigger of the path resolves (gorge scriptSearch.visit assumes they pass),
// and reports whether there is one. Priority is not offered with the script
// hook: the search prices the play itself.
func (t *State) tapXNext(p uint8, d *Decision) bool {
	for guard := 0; guard < 16; guard++ {
		t.g.availX = availPlain
		t.stepRaw(d)
		t.g.availX = 0
		if t.over || d.Kind == DkNone || d.N == 0 {
			return false
		}
		if d.Player == SeatIx(p) {
			return true
		}
		if d.Kind == DkTriggerOrder {
			// gorge's search answers an order ask the line set off (rules
			// scriptSearch.visit: scriptAnswers' first answer, every pick in
			// pose order, CR 603.3b) and continues at the next node: the
			// opponent's two Profane Mementos triggered by the searched
			// cast's sacrifice of its own mana source do not end the line.
			for i := 0; i < int(d.N); i++ {
				t.Apply(d.Options[i])
			}
			continue
		}
		pass := MakeAction(AkPass, 0, 0, 0)
		if d.Kind != DkPriority || !hasOption(d, pass) {
			return false
		}
		t.Apply(pass)
	}
	return false
}

// tapXPlay plays the found path on s: each activation as the search made it,
// each answer and pass as the decision then pending (gorge answers the
// activation's asks and the stacked trigger's one at a time).
func (s *State) tapXPlay(k *prep, p uint8, path []tapXStep) {
	n := s.Decisions
	var d Decision
	for _, st := range path {
		if !s.tapXNext(p, &d) {
			break
		}
		if st.kind == tapXAct || st.kind == tapXEach || st.kind == tapXPlain || st.kind == tapXAnyR || st.kind == tapXRider || st.kind == tapXDefer || st.kind == tapXAnyDifferent || st.kind == tapXGrant {
			s.tapXDo(k, p, st)
		} else if hasOption(&d, st.act) {
			s.Apply(st.act)
		}
	}
	s.tapXNext(p, &d) // the trigger the last step queued is stacked and asked now
	s.Decisions = n
}

// gorgeTargetOrder puts a target ask's options in the order gorge offers
// them (the empty answer of an up-to-one ask first, scriptAnswers; target_legal.go candidatesFor: the players by seat, then each seat's
// battlefield in entry order, then the rest as listed): the script takes the
// first answer that works, so the kernel's slot order would pick another
// creature.
func (t *State) gorgeTargetOrder(d *Decision) {
	var keys [MaxOptions]uint32
	n := int(d.N)
	for i := 0; i < n; i++ {
		c := d.Options[i].Target()
		switch {
		case c == NoTarget:
			keys[i] = 0 // gorge scriptAnswers tries the empty answer of an up-to-one ask first
		case isPlayerTarget(c):
			keys[i] = 1<<8 + uint32(CodeSeat(c))
		case c < MaxObj && t.objs[c].zone == kop.ZoneBattlefield && t.objs[c].slot != NoSlot:
			pm := &t.perms[t.objs[c].slot]
			keys[i] = 1<<24 | uint32(pm.ctrl)<<20 | uint32(pm.ts)
		default:
			keys[i] = 2<<24 | uint32(i)
		}
	}
	for i := 1; i < n; i++ {
		k, a := keys[i], d.Options[i]
		j := i
		for j > 0 && keys[j-1] > k {
			keys[j], d.Options[j] = keys[j-1], d.Options[j-1]
			j--
		}
		keys[j], d.Options[j] = k, a
	}
}

// gorgeAskOrder puts a trigger's ask's answers in the order gorge's prefix
// search tries them (rules scriptAnswers: gorge's option order). A yes-or-no
// election (a "you may", an if-you-do cost, an unless, top or bottom) lists its
// yes first (trigger_queue.go optionalAtResolutionDecision), so Spider-Gwen,
// Free Spirit's loot discards before it declines. An up-to-one pick tries its
// empty answer (the kernel's Done) first, so Erica Ortegas's accepted trigger
// puts no card from hand onto the battlefield. A scry or surveil look at one
// card is an up-to-one ask, so the empty top pile (the kernel's Done: the card
// to the graveyard or the bottom) is tried before keeping the card on top. A
// look at two or more cards is an up-to-n ask scriptAnswers enumerates no answer
// for, so that line is a dead end there; ok is false then.
func gorgeAskOrder(d *Decision) (ok bool) {
	if d.Kind != DkChoose {
		return true
	}
	if yesFirstAsk(d.Sub) && !DebugAskNoYesFirst {
		for i := 1; i < int(d.N); i++ { // stable: mode 1 (yes, pay, top) before mode 0
			a := d.Options[i]
			j := i
			for j > 0 && d.Options[j-1].Mode()&1 < a.Mode()&1 {
				d.Options[j] = d.Options[j-1]
				j--
			}
			d.Options[j] = a
		}
		return true
	}
	if d.Sub != ChArrange {
		for i := 1; i < int(d.N) && !DebugAskNoYesFirst; i++ {
			if a := d.Options[i]; a.Kind() == AkDone {
				copy(d.Options[1:i+1], d.Options[:i])
				d.Options[0] = a
				break
			}
		}
		return true
	}
	if d.N != 2 || d.Options[1].Kind() != AkDone {
		return false
	}
	d.Options[0], d.Options[1] = d.Options[1], d.Options[0]
	return true
}

// DebugAskNoYesFirst leaves a yes-or-no election and an up-to-one pick in the
// kernel's order in the prefix searches (mutation testing only: the lockstep must catch it).
var DebugAskNoYesFirst bool

// yesFirstAsk: sub is a two-answer election whose mode 1 answer gorge lists
// first (the oracle's chooseKey reads the same set).
func yesFirstAsk(sub ChoiceKind) bool {
	switch sub {
	case ChOptional, ChWardPay, ChOptDiscMay, ChGenPay, ChGenPayMode, ChGenTopBottom, ChGenLeyline, ChGenDiscUnless:
		return true
	}
	return false
}

// settle continues the search at t after a step: t runs to p's next
// decision, a priority being a node (visit) and an ask a branch over its
// answers (gorge scriptSearch.visit's scriptAnswers).
func (rs *tapXSearch) settle(k *prep, t *State, gapUsed bool) bool {
	var d Decision
	if !t.tapXNext(rs.p, &d) {
		rs.stallOf(t)
		return false
	}
	if d.Kind == DkPriority {
		if rs.visit(k, t, gapUsed) {
			return true
		}
		rs.stallOf(t)
		return false
	}
	if !rs.tick() { // the ask
		return false
	}
	if rs.leftOwnZone(t) {
		return false
	}
	if d.Kind == DkTarget {
		t.gorgeTargetOrder(&d)
	}
	if !gorgeAskOrder(&d) {
		return false
	}
	for i := 0; i < int(d.N) && rs.n < maxTapXPath; i++ {
		c := t.fork()
		c.Apply(d.Options[i])
		rs.path[rs.n] = tapXStep{kind: tapXAnswer, act: d.Options[i], sl: NoSlot, cand: NoSlot, cand2: NoSlot}
		rs.n++
		ok := rs.settle(k, c, gapUsed)
		if !ok {
			rs.stallOf(c)
		}
		t.unfork()
		if ok {
			return true
		}
		rs.n--
	}
	return false
}

// cols lists a colour ask's answers in the order gorge's script search tries
// them: the play's announced colours first (scriptOrderAnswers on the ask),
// then the rest W U B R G.
func (rs *tapXSearch) cols(out *[5]uint8) {
	for col := uint8(0); col < 5; col++ {
		out[col] = col
	}
	needFirst(out[:], rs.need)
}

// leftOwnZone: the path sacrificed the searched ability's own source (a
// sacrifice-another cost took it), so no continuation plays it (gorge
// scriptSearch.leftOwnZone, CR 602.2b): the node is spent, the line ends.
func (rs *tapXSearch) leftOwnZone(t *State) bool {
	return rs.sac && rs.src != NoObj && t.objs[rs.src].zone != rs.srcZ
}

func (rs *tapXSearch) tick() bool {
	if rs.nodes >= potentialScriptNodes {
		return false
	}
	rs.nodes++
	return true
}

// stallOf records the first stall a failed line's fork latched (capFull) while
// the line ran, so the offer walk declares it on the real state where it drops
// the play instead of reporting it unpayable: the fork cannot go on (its next
// stepGen ends the game), but the play could have been payable along it, and a
// caller that keeps searching may still price it.
func (rs *tapXSearch) stallOf(c *State) {
	if rs.stall == 0 {
		rs.stall = c.g.stalled
	}
}

// anyDifferentStep branches a Firemind Vessel gap at a search node on its N
// distinct colours; handled reports that the gap was one (found is the
// branch's result).
func (rs *tapXSearch) anyDifferentStep(k *prep, t *State, sl Slot) (found, handled bool) {
	if !rs.anyDiff.has(sl) || !t.anyDifferentAt(k, sl) {
		// rs.anyDiff is the root's mask: a Vessel an earlier node of this
		// line tapped is no gap any more, and activating it again would float
		// its colours a second time (CR 602.2b; gorge's search visits each
		// source once)
		return false, false
	}
	if !t.anyDifferentAt(k, sl) {
		// tapped by an earlier step of the path: its {T} is spent, the
		// mask is the root's (CR 602.2b; gorge's gaps are read live)
		return false, false
	}
	if !rs.tick() { // the colour ask
		return false, true
	}
	a := t.anyDifferentAbilityAt(k, sl)
	n := max(int(a.ManaN), 1)
	na := anyDifferentAnswers(n)
	for r := 0; r < na; r++ {
		ans := anyDifferentRank(n, &rs.need, r)
		if rs.try(k, t, tapXStep{kind: tapXAnyDifferent, sl: sl, cand: NoSlot, cand2: NoSlot, col: uint8(ans)}) {
			return true, true
		}
	}
	return false, true
}

// tapXDo plays step st: the elected permanent taps first, then the source
// (gorge commitManaDiscard: the tapXType picks, then the source's {T}), and
// the chosen colour floats.
func (s *State) tapXDo(k *prep, p uint8, st tapXStep) {
	if st.kind == tapXEach {
		var held [kop.NumManaKinds]uint8
		s.tapGap(k, p, st.sl, slotMask{}, &held)
		return
	}
	if st.kind == tapXPlain {
		s.tapXPlainDo(k, p, st.sl, st.col)
		return
	}
	if st.kind == tapXAnyR {
		// an any-colour restricted gap: the anyR search's application of its answer
		var spill [kop.NumManaKinds]uint8
		s.scriptGapTap(k, p, st.act, st.sl, slotMask{}, slotMask{}, s.scriptAnyR(k, p), slotMask{}, slotMask{}, int(st.col), &spill)
		return
	}
	if st.kind == tapXRider {
		// the rider source's own cost (its counter, mana_rider.go), then its
		// mana and rider: the script returns to priority after the step, so
		// the SBAs and the queued triggers settle before the next node (as
		// ctrGap's step does; a Needle down to its last counter sacrifices)
		if s.riderPlainFirst(k, st.sl) {
			s.riderPlainGap(k, p, st.act, st.sl, int(st.col))
		} else {
			s.riderGap(k, p, st.sl, int(st.col))
		}
		s.sba(k)
		return
	}
	if st.kind == tapXDefer {
		s.tapXDeferDo(k, p, st)
		return
	}
	if st.kind == tapXAnyDifferent {
		s.anyDifferentTap(k, p, st.sl, int(st.col))
		s.sba(k)
		return
	}
	if st.kind == tapXGrant {
		s.tapXGrantDo(k, p, st)
		return
	}
	if st.cand == NoSlot {
		s.ctrGap(k, p, st.sl, int(st.col)) // a counter-cost gap: its counter, its use, its mana
		s.sba(k)                           // the script returns to priority after the step: a Wall of Roots out of toughness dies before the next one
		return
	}
	if k.exileGOf(s.cardAt(st.sl)) {
		s.exileGDo(k, p, st.sl, st.cand, st.col) // cand is a graveyard index (mana_exilegrave.go)
		return
	}
	if k.sacXOf(s.cardAt(st.sl)) {
		s.sacXDo(k, p, st.sl, st.cand, st.col)
		return
	}
	tb := s.tapBatchOpen(k) // the ability's elected taps are one tapping action (gorge mana_activation.go; trig_tapall.go)
	for _, cd := range [2]Slot{st.cand, st.cand2} {
		if cd == NoSlot {
			continue
		}
		cp := &s.perms[cd]
		was := cp.flags & pfTapped
		cp.flags |= pfTapped
		s.touchSl(cd)
		if was == 0 {
			if s.subs[kop.TrigTaps].any() {
				s.fireTaps(k, cd)
			}
			s.markTapped(k, cd) // only a tap that happens marks
		}
	}
	s.tapBatchClose(tb)
	ab := k.tapXAbOf(s.cardAt(st.sl))
	if ab.Cost.Flags&kop.CostTap != 0 {
		s.perms[st.sl].flags |= pfTapped
		s.touchSl(st.sl)
		s.manaTapped(k, st.sl)
	}
	_, n := tapXUnit(ab)
	s.poolAdd(p, int(st.col), int(n))
	s.tapExtraStall(k, st.sl)
	s.artTag(k, p, st.sl, int(st.col))
}

// sacXDo activates sacrifice-another source sl for p, sacrificing cand, in
// gorge's commit order (commitManaDiscard): the source's {T}, then the
// sacrifice (its Sacrificed and dies triggers queue), then every colour of the
// production. The script returns to priority after the step, so the SBAs and
// the queued triggers settle before the next node (as ctrGap's step does).
// A restricted one (kop.SacXRestrManaAb, Master of Dark Rites) floats its
// fixed production restricted (floatRestrAb): the payment the search prices at
// the next node spends it as gorge's plan does, what it leaves stays
// restricted (CR 106.6).
func (s *State) sacXDo(k *prep, p uint8, sl, cand Slot, colour uint8) {
	ab := k.tapXAbOf(s.cardAt(sl))
	if ab.Cost.Flags&kop.CostTap != 0 { // (Krark-Clan Ironworks has no {T})
		s.perms[sl].flags |= pfTapped
		s.touchSl(sl)
		s.manaTapped(k, sl)
	}
	s.sacFilterSource(k, cand)
	cols, n := tapXUnit(ab)
	if kop.SacXAnyColour(ab) {
		cols = 1 << colour // the activation asked the colour after the sacrifice
	}
	if ab.ManaRestrict != 0 {
		if !s.floatRestrAb(k, p, sl, ab, int(bits.TrailingZeros8(cols)), n) {
			s.capFull(StallRestrSrc) // every batch is taken: never a silent spill
		}
		s.tapExtraStall(k, sl)
		s.sba(k)
		return
	}
	for col := 0; col < kop.NumManaKinds; col++ {
		if cols&(1<<col) != 0 {
			s.poolAdd(p, col, int(n))
			s.artTag(k, p, sl, col)
		}
	}
	s.tapExtraStall(k, sl)
	s.sba(k)
}

// tapXSecond: card c's tap-another ability is a later one beside a plain {T}
// first (Scene of the Crime), so activating the source asks which ability.
func (k *prep) tapXSecond(c CardID) bool {
	return int(c) < len(k.tapXAb) && k.tapXAb[c] >= 0 && k.tapXAb[c] != k.cards[c].manaAb
}

// tapXPlainDo plays a second-ability source's first ability (its {T}: Add
// {C}): the answer gorge's prefix search tries first at the source's ability
// ask (scriptAnswers in the ask's order), and its only one once the tap-another
// cost has nothing left to tap. The source taps and its colour floats: its
// fixed colour, or the colour asked when the plain ability is an any-colour one
// (Weather Maker's, beside its counter ability: mana_countercost.go).
func (s *State) tapXPlainDo(k *prep, p uint8, sl Slot, ask uint8) {
	pc := &k.cards[s.cardAt(sl)]
	col := bits.TrailingZeros8(uint8(pc.manaCol))
	if pc.manaCol&(pc.manaCol-1) != 0 {
		col = int(ask)
	}
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	s.poolAdd(p, col, int(max(pc.manaN, 1)))
	s.tapExtraStall(k, sl)
	s.artTag(k, p, sl, col)
}

// tapXAbilityAsk branches a second-ability gap source sl at a search node on
// its ability ask (rules scriptSearch.visit, an activation of a census gap,
// then scriptAnswers over the ask): the plain first ability first, and the
// caller then tries the tap-another one when it has a candidate (live). With
// one payable ability nothing is asked. handled: the search found a path
// (found) or ran out of nodes (!found); the caller returns found then.
func (rs *tapXSearch) tapXAbilityAsk(k *prep, t *State, sl Slot, live bool) (found, handled bool) {
	if !k.tapXSecond(t.cardAt(sl)) || !t.canTapForMana(k, sl) {
		return false, false
	}
	if live && !rs.tick() { // the ability ask
		return false, true
	}
	if rs.try(k, t, tapXStep{kind: tapXPlain, sl: sl, cand: NoSlot, cand2: NoSlot}) {
		return true, true
	}
	return false, false
}

// ctrAbilityAsk branches a second-ability counter gap sl (kop.SecondCtrTapManaAb)
// at a search node on its ability ask: the plain first ability first, behind
// the colour ask its any-colour production poses, the colours in the order of
// the play's announced ones (cols). The caller then tries the counter ability.
// handled: the search found a path (found) or ran out of nodes (!found); the
// caller returns found then.
func (rs *tapXSearch) ctrAbilityAsk(k *prep, t *State, sl Slot, cols *[5]uint8) (found, handled bool) {
	c := t.cardAt(sl)
	if !k.ctrSecond(c) {
		return false, false
	}
	if !rs.tick() { // the ability ask
		return false, true
	}
	if mc := k.cards[c].manaCol; mc&(mc-1) != 0 {
		if !rs.tick() { // the colour ask
			return false, true
		}
		for i := 0; i < 5; i++ {
			if rs.try(k, t, tapXStep{kind: tapXPlain, sl: sl, cand: NoSlot, cand2: NoSlot, col: cols[i]}) {
				return true, true
			}
		}
		return false, false
	}
	if rs.try(k, t, tapXStep{kind: tapXPlain, sl: sl, cand: NoSlot, cand2: NoSlot}) {
		return true, true
	}
	return false, false
}

func (rs *tapXSearch) try(k *prep, t *State, st tapXStep) bool {
	if rs.n == maxTapXPath {
		return false
	}
	c := t.fork()
	defer t.unfork()
	c.tapXDo(k, rs.p, st)
	rs.path[rs.n] = st
	rs.n++
	if rs.settle(k, c, true) {
		return true
	}
	rs.stallOf(c) // the line failed; the fork may have latched a stall (c is live until the deferred unfork)
	rs.n--
	return false
}

// pairAsk is visit's two-permanent cost (Baylen) at source sl with its nc
// candidates cs: done reports a result (found, or the budget spent) visit
// returns; otherwise no pair paid and the search goes on to the next gap.
func (rs *tapXSearch) pairAsk(k *prep, t *State, sl Slot, cs *[MaxPerm]Slot, nc int, cols *[5]uint8) (ret, done bool) {
	// a two-permanent cost (Baylen): two candidates are tapped with no
	// ask; more are asked as one "choose 2" whose answers gorge's
	// scriptAnswers enumerates as every pair of its candidates in
	// lexicographic order, 64 at most (a multi-object election, not a
	// per-unit colour allocation: an odd count is no early stop)
	if nc > 2 && !rs.tick() {
		return false, true
	}
	npair := tapXPairLimit(nc)
	for pi := 0; pi < npair; pi++ {
		qi := pi
		if debugTapXMutate == 2 {
			qi = npair - 1 - pi // the last pair first (mutation)
		}
		i, j := tapXPairAt(nc, qi)
		a, b := cs[i], cs[j]
		if rs.tapXOwnAnswer(a, b) {
			// A forced tap (no ask: exactly the cost's candidates)
			// that takes the play's own {T} source is no continuation
			// (CR 602.2b): gorge's tappedOwnSource rejects the node
			// the tap reaches, before the colour ask branches, so the
			// branch costs one node there and none here.
			continue
		}
		if !rs.tick() { // the colour ask
			return false, true
		}
		for i := 0; i < 5; i++ {
			c := cols[i]
			if debugTapXMutate == 1 {
				c = 0
			}
			if rs.try(k, t, tapXStep{sl: sl, cand: a, cand2: b, col: c}) {
				return true, true
			}
		}
	}
	return false, false
}

func (rs *tapXSearch) visit(k *prep, t *State, gapUsed bool) bool {
	if !rs.tick() { // a payer priority node
		return false
	}
	if rs.leftOwnZone(t) {
		return false
	}
	p := rs.p
	// gorge asks two questions at a node, either side of its dedup: is the
	// play offered as is (the floating pool alone pays it) before, and does
	// the planner price it from the pool and the untapped sources after. The
	// dedup key counts the hand, not its cards, so a Taps listener's
	// draw-then-discard reaches one key twice: with the play discarded and
	// with it kept. The kept one is offered when the pool pays it and
	// otherwise found seen (gorge scriptSearch.visit).
	offered := false
	if gapUsed {
		var d Decision
		t.restrOptions(k, &d, p)
		offered = hasOption(&d, rs.a)
		if offered && rs.a.Kind() == AkCast && rs.a.Mode()&^3 == 0 && t.FeatPoolPays(p, Obj(rs.a.Source()), rs.a.Mode()&3) {
			if rs.witnessReaches(k, t, true) {
				return true
			}
			offered = false // the witness does not reach the pool-only plan: the node is no terminal, the search branches on
		} else if offered && rs.a.Kind() == AkCast && rs.a.Mode()&^3 == 0 && !rs.witnessReaches(k, t, false) {
			offered = false // nor the planned one (the branch below): the node is no terminal, the search branches on
		}
	}
	pl := &t.pl[SeatIx(p)]
	key := tapXKey{pool: pl.pool, art: t.g.poolArt[SeatIx(p)], sp: t.sp, hand: pl.handN, gy: pl.graveN, bf: t.bfObjKey(p), rb: t.rb[SeatIx(p)]}
	for b := t.ctrlM[SeatIx(p)].and(t.occ); b.any(); b = b.rest() {
		if sl := b.first(); t.perms[sl].flags&pfTapped != 0 {
			key.tapped = key.tapped.with(sl)
		}
	}
	for i := 0; i < rs.nseen; i++ {
		if rs.seen[i] == key {
			return false
		}
	}
	if rs.nseen < len(rs.seen) {
		rs.seen[rs.nseen] = key
		rs.nseen++
	}
	if offered {
		return true
	}
	live := t.scriptTapX(k, p)
	ctr := t.scriptCtr(k, p)
	each := t.scriptGaps(k, p) // (rs.gaps keeps the root's: the Drum tapped, the Tender is still a gap)
	anyR := t.scriptAnyR(k, p)
	rider := t.riderDeferMask(k, p)
	var cols [5]uint8
	rs.cols(&cols)
	var order [MaxPerm]Slot
	n := t.battlefieldOrder(p, t.ctrlM[SeatIx(p)].and(t.occ), order[:], 0)
	for i := 0; i < n; i++ {
		sl := order[i]
		if !rs.gaps.has(sl) || sl == rs.own {
			continue // (tapped for mana, the play's source could not pay its own {T}: CR 602.2b)
		}
		if found, handled := rs.anyDifferentStep(k, t, sl); handled {
			return found
		}
		if rs.def.has(sl) {
			if found, handled := rs.deferAsk(k, t, sl); handled {
				return found
			}
			continue
		}
		if rs.grants.has(sl) {
			if rs.grantAsk(k, t, sl) {
				return true
			}
			continue
		}
		if each.has(sl) {
			// an each-colour gap has one answer and asks nothing; tapped for
			// its own mana it is no longer there for a tap-another cost
			if rs.try(k, t, tapXStep{kind: tapXEach, sl: sl, cand: NoSlot, cand2: NoSlot}) {
				return true
			}
			continue
		}
		if anyR.has(sl) {
			// an any-colour restricted gap: gorge's search branches on its
			// activation's answers like the anyR search's DFS does (scriptDFS),
			// each answer one step (scriptGapTap)
			for c := uint8(0); c < t.anyRAnswers(k, sl); c++ {
				if rs.try(k, t, tapXStep{kind: tapXAnyR, sl: sl, cand: NoSlot, cand2: NoSlot, col: c, act: rs.a}) {
					return true
				}
			}
			continue
		}
		if ctr.has(sl) {
			// a counter-cost gap has one answer and asks nothing, unless it is
			// a second ability beside a plain first one: activating the source
			// then asks which ability, and gorge's search tries the plain one
			// first (with its colour ask when it makes any colour), so a later
			// gap's counters may pay where this one's would have
			if found, handled := rs.ctrAbilityAsk(k, t, sl, &cols); handled {
				return found
			}
			if ab := k.ctrAbOf(t.cardAt(sl)); ctrAnyColour(ab) {
				// an any-colour counter ability (Vivid Grove's second) asks
				// its colour after the ability: the announced ones first
				if !rs.tick() { // the colour ask
					return false
				}
				for i := 0; i < 5; i++ {
					if rs.try(k, t, tapXStep{sl: sl, cand: NoSlot, cand2: NoSlot, col: cols[i]}) {
						return true
					}
				}
				continue
			}
			if rs.try(k, t, tapXStep{sl: sl, cand: NoSlot, cand2: NoSlot}) {
				return true
			}
			continue
		}
		if found, handled := rs.tapXAbilityAsk(k, t, sl, live.has(sl)); handled {
			return found
		}
		if rider.has(sl) {
			if found, stop := rs.riderStep(k, t, sl); stop {
				return found
			}
			continue
		}
		if !live.has(sl) {
			continue
		}
		var cs [MaxPerm]Slot
		nc := t.tapXCands(k, p, sl, &cs)
		if k.pickXOf(t.cardAt(sl)) {
			if found, stop := rs.sacStep(k, t, sl, &cs, nc, &cols); stop {
				return found
			}
			continue
		}
		if k.tapXAbOf(t.cardAt(sl)).Cost.N == 2 {
			// a two-permanent cost (Baylen): pairAsk
			if ret, done := rs.pairAsk(k, t, sl, &cs, nc, &cols); done {
				return ret
			}
			continue
		}
		if nc > 1 && !rs.tick() { // the creature ask
			return false
		}
		for ci := 0; ci < nc; ci++ {
			cand := cs[ci]
			if debugTapXMutate == 2 {
				cand = cs[nc-1-ci]
			}
			if rs.tapXOwnAnswer(cand, NoSlot) {
				continue // a forced tap of the play's own {T} source (CR 602.2b)
			}
			if found, stop := rs.colourStep(k, t, sl, cand, &cols); stop {
				return found
			}
		}
	}
	if t.sp > rs.sp0 && rs.n < maxTapXPath {
		// a trigger an activation caused: let it resolve, the pool floats
		// (gorge: the last resort at a node, so an activation that still
		// funds the play is tried first)
		pass := MakeAction(AkPass, 0, 0, 0)
		c := t.fork()
		c.Apply(pass)
		rs.path[rs.n] = tapXStep{kind: tapXPass, act: pass, sl: NoSlot, cand: NoSlot, cand2: NoSlot}
		rs.n++
		ok := rs.settle(k, c, true)
		if !ok {
			rs.stallOf(c)
		}
		t.unfork()
		if ok {
			return true
		}
		rs.n--
	}
	return false
}

// colourStep asks the colour of source sl's activation after its permanent
// pick cand and tries each answer, the play's announced colours first; a
// fixed production asks none. stop: visit returns found (an answer paid, or the
// node budget ran out on the ask), else it goes on to the next candidate.
func (rs *tapXSearch) colourStep(k *prep, t *State, sl, cand Slot, cols *[5]uint8) (found, stop bool) {
	if fc, ok := kop.TapXFixedColour(k.tapXAbOf(t.cardAt(sl))); ok {
		f := rs.try(k, t, tapXStep{sl: sl, cand: cand, cand2: NoSlot, col: uint8(fc)})
		return f, f
	}
	if !rs.tick() { // the colour ask
		return false, true
	}
	for i := 0; i < 5; i++ {
		c := cols[i]
		if debugTapXMutate == 1 {
			c = 0
		}
		if rs.try(k, t, tapXStep{sl: sl, cand: cand, cand2: NoSlot, col: c}) {
			return true, true
		}
	}
	return false, false
}

// sacStep is visit's branch for a sacrifice-another source sl with nc
// candidates cs: the "sacrifice" ask only with two or more, in battlefield
// order, then the colour ask of an any-colour one (the announced colours
// first, as the tap-another cost's); a fixed or all-at-once production asks
// none. An answer that takes the play's own source is tried: the next node ends
// it (leftOwnZone). stop: visit returns found.
func (rs *tapXSearch) sacStep(k *prep, t *State, sl Slot, cs *[MaxPerm]Slot, nc int, cols *[5]uint8) (found, stop bool) {
	if k.exileGOf(t.cardAt(sl)) {
		return rs.exileGStep(k, t, sl, cs, nc, cols) // an exile-from-graveyard source (mana_exilegrave.go)
	}
	if nc > 1 && !rs.tick() {
		return false, true
	}
	anyCol := kop.SacXAnyColour(k.tapXAbOf(t.cardAt(sl)))
	for ci := 0; ci < nc; ci++ {
		if anyCol {
			if f, st := rs.colourStep(k, t, sl, cs[ci], cols); st {
				return f, true
			}
			continue
		}
		if rs.try(k, t, tapXStep{sl: sl, cand: cs[ci], cand2: NoSlot}) {
			return true, true
		}
	}
	return false, false
}

// riderStep is visit's branch for a planner-deferred rider source sl (a
// Sandstone Needle's "{T}, remove a depletion counter: Add {R}{R}", its
// sacrifice rider, or a later rider beside a plain first ability, Grove of the
// Burnwillows): one production ask, whose answers are the plain first ability
// and the rider's colours (riderPlainGap, the play's announced ones first), or
// one activation of a rider-first source, its fixed production with no ask and
// a multi-colour one branched on its colours (the announced ones first,
// deferGapAnswers' count). stop: visit returns found, else it goes on to the
// next source.
func (rs *tapXSearch) riderStep(k *prep, t *State, sl Slot) (found, stop bool) {
	if t.riderPlainFirst(k, sl) {
		if !rs.tick() { // the production ask
			return false, true
		}
		for c := uint8(0); c < t.deferGapAnswers(k, sl); c++ {
			if rs.try(k, t, tapXStep{kind: tapXRider, sl: sl, col: c, act: rs.a}) {
				return true, true
			}
		}
		return false, false
	}
	ra, ok := t.riderAbilityOf(k, sl)
	if !ok {
		return false, false
	}
	mask := uint8(ra.ManaColors)
	if !ra.ManaEach && mask&(mask-1) != 0 {
		// the activation asks its colour: the announced ones first
		if !rs.tick() { // the colour ask
			return false, true
		}
		var order [kop.NumManaKinds]uint8
		no := 0
		for col := uint8(0); col < kop.NumManaKinds; col++ {
			if mask&(1<<col) != 0 {
				order[no] = col
				no++
			}
		}
		needFirst(order[:no], rs.need)
		for i := 0; i < no; i++ {
			if rs.try(k, t, tapXStep{kind: tapXRider, sl: sl, col: uint8(riderAnswerBit(mask, int(order[i])))}) {
				return true, true
			}
		}
		return false, false
	}
	// a fixed production (or every colour at once) asks nothing
	if rs.try(k, t, tapXStep{kind: tapXRider, sl: sl}) {
		return true, true
	}
	return false, false
}

// SetDebugTapXMutate is the caretaker lockstep mutation switch (test only):
// 1 = the search always answers the first colour, 2 = it taps the last
// creature candidate first; 0 = off.
func SetDebugTapXMutate(m int) { debugTapXMutate = m }

// sacPotPays reports whether gorge's PotentialMana bound pays play a on a board
// with a live sacrifice-any source and a closed bound (tapXSacAnyBound): the
// bound counts a choice source (Fellwar Stone's reflected {B} or {R}) once per
// colour it can make, where the gaps walk counts it once, so it lists a play
// the gaps walk's sum falls short of ({4} against Ironworks' {C}{C} and one
// Fellwar Stone unit). The search then runs for that play too.
func (s *State) sacPotPays(k *prep, p uint8, a Action) bool {
	cost, _, _, ok := s.grantPlayCost(k, p, a, true)
	if !ok {
		return false
	}
	var pot avail
	d := s.grantPayDesc(k, a)
	s.availPotentialFor(k, p, &pot, &d, false)
	s.availSacAny(k, p, &pot, &d) // the bound admits each sacrifice-any ability once
	return pot.affords(&cost, pipMaskOf(&cost), 0)
}

// gapsOrBoundPlay: play a is one the gaps walk lists, or, on a closed
// sacrifice-any bound (sacAny; tapXSacAnyBound), one gorge's PotentialMana bound
// pays on its own (sacPotPays).
func (s *State) gapsOrBoundPlay(k *prep, p uint8, sacAny bool, gaps *Decision, a Action) bool {
	return hasOption(gaps, a) || sacAny && s.sacPotPays(k, p, a)
}

// sacRepeatBoard is scriptOffers' twoLive: the boards gorge's census is
// unrelaxable on regardless of the PotentialMana bound — a live two-permanent
// source (Baylen, tapXTwoLive), an open sacrifice-any bound (Krark-Clan
// Ironworks, tapXSacAnyBound's open arm), or a live sacrifice-any one whose
// cost sacrifices dedicated tokens (Jungle Patrol, sacRepeatLive). sacRepeat
// reports the last of these, the one that lifts the closed-bound narrowing
// (sacDrops).
func (s *State) sacRepeatBoard(k *prep, p uint8, sacOpen bool) (sacRepeat, twoLive bool) {
	sacRepeat = s.sacRepeatLive(k, p)
	return sacRepeat, k.tapMana && s.tapXTwoLive(k, p) || sacOpen || sacRepeat
}

// sacDrops: on a closed sacrifice-any bound (sacAny) play a is neither the gaps
// walk's nor one gorge's bound pays (gapsOrBoundPlay), so it is no candidate —
// unless a live repeatable sacrifice-any source makes gorge's census
// unrelaxable (sacRepeatBoard): gorge searches every census-incomplete play
// then, the closed bound's plays included.
func (s *State) sacDrops(k *prep, p uint8, sacAny, sacRepeat bool, gaps *Decision, a Action) bool {
	return sacAny && !sacRepeat && !s.gapsOrBoundPlay(k, p, sacAny, gaps, a)
}
