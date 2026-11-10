package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

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

// DebugNoTapMana hides the tap-another-permanent gaps (the caretaker
// mutation test proves the pool's plays need them).
var DebugNoTapMana bool

// debugTapXMutate selects a lockstep mutation: 1 = the first colour always
// (the search ignores the play's pips), 2 = the last creature candidate.
var debugTapXMutate int

const maxTapXPath = 24

// scriptTapX: p's tap-another-permanent sources that can be activated now
// (untapped, not summoning sick, with an untapped candidate), a slot mask.
//
//go:noinline
func (s *State) scriptTapX(k *prep, p uint8) slotMask {
	return s.tapXSources(k, p, true)
}

// tapXCensusGap: p's tap-another-permanent sources gorge's window census
// leaves unpriced (castWindowSourceExcluded): its probe admits a mana ability
// without reading whether the tap part is payable (ignorePayable), so an
// untapped Springleaf Drum with no untapped creature to tap still makes the
// census incomplete (CR 601.2g).
func (s *State) tapXCensusGap(k *prep, p uint8) slotMask {
	return s.tapXSources(k, p, false)
}

// tapXSources: scriptTapX, the candidate count read only when cands is set.
func (s *State) tapXSources(k *prep, p uint8, cands bool) slotMask {
	var m slotMask
	if !k.tapMana || DebugNoTapMana {
		return m
	}
	for b := s.ctrlM[p&1]; b.any(); b = b.rest() {
		sl := b.first()
		c := s.cardAt(sl)
		if k.cards[c].manaAb < 0 || !kop.TapXMana(&k.t.Cards[c].Abilities[k.cards[c].manaAb]) {
			continue
		}
		pm := &s.perms[sl]
		a := &k.t.Cards[c].Abilities[k.cards[c].manaAb]
		if a.Cost.Flags&kop.CostTap != 0 && (pm.flags&pfTapped != 0 || pm.flags&pfSick != 0 && s.isCreature(k, sl) && s.kw(k, sl)&kop.KwHaste == 0) || s.genLost(k, sl) ||
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

// tapXTwoLive: a no-{T} two-permanent mana source (Baylen) is activatable now
// on a board the search models. Its cost is not once-only, so gorge's
// census is no longer relaxable (paymentPlanRelaxedAlternatives) and every
// census-incomplete insufficient play takes a search slot.
//
//go:noinline
func (s *State) tapXTwoLive(k *prep, p uint8) bool {
	for b := s.scriptTapXModelled(k, p); b.any(); b = b.rest() {
		c := s.cardAt(b.first())
		if k.t.Cards[c].Abilities[k.cards[c].manaAb].Cost.Flags&kop.CostTap == 0 {
			return true
		}
	}
	return false
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
	if !k.tapMana || !s.scriptTapX(k, p).any() || s.scriptRestr(k, p).or(s.scriptAnyR(k, p)).or(s.scriptFilt(k, p)).any() {
		return slotMask{}
	}
	return s.scriptGaps(k, p)
}

// tapXCands lists the permanents source sl's tap cost can tap, in gorge's
// candidate order (battlefield entry): p's untapped permanents matching the
// cost's filter, the source itself excluded (its own {T} is paid first;
// pay.ManaTapCandidates claims it).
func (s *State) tapXCands(k *prep, p uint8, sl Slot, out *[MaxPerm]Slot) int {
	c := s.cardAt(sl)
	f := k.t.Cards[c].Abilities[k.cards[c].manaAb].Cost.Filter
	var m slotMask
	// A creature the cast being paid for announced for convoke is tapped when
	// that payment settles, so it is not untapped to tap again here (CR 601.2g-h;
	// gorge pay.ManaTapCandidates, CastCommitted).
	var spoken slotMask
	if s.buildLive() {
		spoken = s.convokeHidden()
	}
	for b := s.ctrlM[p&1]; b.any(); b = b.rest() {
		x := b.first()
		if x == sl && k.t.Cards[c].Abilities[k.cards[c].manaAb].Cost.Flags&kop.CostTap != 0 || s.perms[x].flags&pfTapped != 0 || spoken.has(x) || !s.matchPerm(k, f, x, p, sl) {
			continue
		}
		m = m.with(x)
	}
	return s.battlefieldOrder(p, m, out[:], 0)
}

// availTapX adds p's tap-another-permanent sources' mana to the gaps walk:
// the relaxed census counts each as a free any-colour unit.
//
//go:noinline
func (s *State) availTapX(k *prep, p uint8, av *avail) {
	for b := s.scriptTapXModelled(k, p); b.any(); b = b.rest() {
		pc := &k.cards[s.cardAt(b.first())]
		av.addUnits(uint8(pc.manaCol), pc.manaN)
	}
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

// tapXBoard: p's census gaps are tap-another-permanent and counter-cost
// sources only, those beside each-colour sources (a Bloom Tender; the search
// activates them as steps, tapXEach), or a mix the filter search runs
// (mixBoard).
func (s *State) tapXBoard(k *prep, p uint8) bool {
	if s.scriptRestr(k, p).or(s.scriptAnyR(k, p)).or(s.scriptFilt(k, p)).any() {
		return s.mixBoard(k, p)
	}
	return !s.scriptGaps(k, p).any() || s.scriptTapX(k, p).any()
}

// tapXPrefix plays the search's script for play a at a tapX board; it
// reports whether it handled the board (the caller then does nothing more).
//
//go:noinline
func (s *State) tapXPrefix(k *prep, a Action, p uint8) bool {
	if !s.scriptTapXModelled(k, p).or(s.scriptCtrModelled(k, p)).any() {
		return false
	}
	var d Decision
	s.plainOptions(k, &d, 0)
	if !hasOption(&d, a) {
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
	act      Action
}

const (
	tapXAct    uint8 = iota // an activation
	tapXAnswer              // act answers the ask then pending
	tapXPass                // p passes with a trigger of the path on the stack
	tapXEach                // an each-colour gap (a Bloom Tender) taps for its colours
)

type tapXKey struct {
	pool   [kop.NumManaKinds]uint8
	art    uint8
	tapped slotMask
	sp     uint8 // gorge's key counts the stack, hand and graveyard too
	hand   uint8
	gy     uint8
}

type tapXSearch struct {
	a     Action
	p     uint8
	own   Slot  // the source whose {T} is the play's own cost (NoSlot: none)
	sp0   uint8 // the stack at the root: only a deeper one is the path's to resolve
	gaps  slotMask
	need  [kop.NumManaKinds]bool // the play's announced colours (scriptNeedColours)
	nodes int
	nseen int
	seen  [potentialScriptNodes]tapXKey
	n     int
	path  [maxTapXPath]tapXStep
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
// a of p: found. With apply it plays the found script on s.
//
//go:noinline
func (s *State) tapXScript(k *prep, a Action, p uint8, apply bool) bool {
	var rs tapXSearch
	rs.a, rs.p, rs.sp0, rs.gaps = a, p, s.sp, s.scriptTapXModelled(k, p).or(s.scriptCtrModelled(k, p)).or(s.tapXEachGaps(k, p))
	rs.need = s.playNeed(k, a) // the colour ask's order follows the play's pips
	rs.own = NoSlot
	if a.Kind() == AkActivate {
		_, rs.own, _ = s.activationMana(k, Obj(a.Source()), a.Mode())
	}
	t := s.fork()
	defer s.unfork()
	// granted mana is the grant search's (grantPrefix / grantCap), for a cast as
	// for an activation: no gap of this search activates a grant, so none prices it
	t.g.noGrant = true
	if !rs.settle(k, t, false) || rs.n == 0 {
		return false
	}
	if apply {
		s.tapXPlay(k, p, rs.path[:rs.n])
	}
	return true
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
		if d.Player == p&1 {
			return true
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
		if st.kind == tapXAct || st.kind == tapXEach {
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
			keys[i] = 1<<8 + uint32(c-PlayerTarget0)
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
		return false
	}
	if d.Kind == DkPriority {
		return rs.visit(k, t, gapUsed)
	}
	if !rs.tick() { // the ask
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

func (rs *tapXSearch) tick() bool {
	if rs.nodes >= potentialScriptNodes {
		return false
	}
	rs.nodes++
	return true
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
	if st.cand == NoSlot {
		s.ctrGap(k, p, st.sl) // a counter-cost gap: its counter, its use, its mana
		s.sba(k)              // the script returns to priority after the step: a Wall of Roots out of toughness dies before the next one
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
		if was == 0 && s.subs[kop.TrigTaps].any() {
			s.fireTaps(k, cd)
		}
	}
	s.tapBatchClose(tb)
	if c := s.cardAt(st.sl); k.t.Cards[c].Abilities[k.cards[c].manaAb].Cost.Flags&kop.CostTap != 0 {
		s.perms[st.sl].flags |= pfTapped
		s.touchSl(st.sl)
		s.manaTapped(k, st.sl)
	}
	pc := &k.cards[s.cardAt(st.sl)]
	s.poolAdd(p, int(st.col), int(max(pc.manaN, 1)))
	s.tapExtraStall(k, st.sl)
	s.artTag(k, p, st.sl, int(st.col))
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
	rs.n--
	return false
}

func (rs *tapXSearch) visit(k *prep, t *State, gapUsed bool) bool {
	if !rs.tick() { // a payer priority node
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
			return true
		}
	}
	pl := &t.pl[p&1]
	key := tapXKey{pool: pl.pool, art: t.g.poolArt[p&1], sp: t.sp, hand: pl.handN, gy: pl.graveN}
	for b := t.ctrlM[p&1].and(t.occ); b.any(); b = b.rest() {
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
	var cols [5]uint8
	rs.cols(&cols)
	var order [MaxPerm]Slot
	n := t.battlefieldOrder(p, t.ctrlM[p&1].and(t.occ), order[:], 0)
	for i := 0; i < n; i++ {
		sl := order[i]
		if !rs.gaps.has(sl) || sl == rs.own {
			continue // (tapped for mana, the play's source could not pay its own {T}: CR 602.2b)
		}
		if each.has(sl) {
			// an each-colour gap has one answer and asks nothing; tapped for
			// its own mana it is no longer there for a tap-another cost
			if rs.try(k, t, tapXStep{kind: tapXEach, sl: sl, cand: NoSlot, cand2: NoSlot}) {
				return true
			}
			continue
		}
		if ctr.has(sl) {
			// a counter-cost gap has one answer and asks nothing
			if rs.try(k, t, tapXStep{sl: sl, cand: NoSlot, cand2: NoSlot}) {
				return true
			}
			continue
		}
		if !live.has(sl) {
			continue
		}
		var cs [MaxPerm]Slot
		nc := t.tapXCands(k, p, sl, &cs)
		if sc := k.cards[t.cardAt(sl)].manaAb; k.t.Cards[t.cardAt(sl)].Abilities[sc].Cost.N == 2 {
			// a two-permanent cost (Baylen): two candidates are tapped with no
			// ask; more are asked as one "choose 2" whose answers gorge's
			// scriptAnswers enumerates as every pair of its candidates in
			// lexicographic order, 64 at most (a multi-object election, not a
			// per-unit colour allocation: an odd count is no early stop)
			if nc > 2 && !rs.tick() {
				return false
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
					return false
				}
				for i := 0; i < 5; i++ {
					c := cols[i]
					if debugTapXMutate == 1 {
						c = 0
					}
					if rs.try(k, t, tapXStep{sl: sl, cand: a, cand2: b, col: c}) {
						return true
					}
				}
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
			if !rs.tick() { // the colour ask
				return false
			}
			for i := 0; i < 5; i++ {
				c := cols[i]
				if debugTapXMutate == 1 {
					c = 0
				}
				if rs.try(k, t, tapXStep{sl: sl, cand: cand, cand2: NoSlot, col: c}) {
					return true
				}
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
		t.unfork()
		if ok {
			return true
		}
		rs.n--
	}
	return false
}

// SetDebugTapXMutate is the caretaker lockstep mutation switch (test only):
// 1 = the search always answers the first colour, 2 = it taps the last
// creature candidate first; 0 = off.
func SetDebugTapXMutate(m int) { debugTapXMutate = m }
