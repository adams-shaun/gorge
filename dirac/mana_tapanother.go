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
// and the payment windows never use it (the mana walk here does not either:
// canTapForManaPC is false for it). A play only its mana pays is a scripted
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
// Everything is behind prep.tapMana. Not modelled: the search's dedup
// beyond (pool, artifact tag, tapped set), a trigger a tap queues.

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
		if s.tapXCands(k, p, sl, &cs) >= int(a.Cost.N) {
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

// tapXCands lists the permanents source sl's tap cost can tap, in gorge's
// candidate order (battlefield entry): p's untapped permanents matching the
// cost's filter, the source itself excluded (its own {T} is paid first;
// pay.ManaTapCandidates claims it).
func (s *State) tapXCands(k *prep, p uint8, sl Slot, out *[MaxPerm]Slot) int {
	c := s.cardAt(sl)
	f := k.t.Cards[c].Abilities[k.cards[c].manaAb].Cost.Filter
	var m slotMask
	for b := s.ctrlM[p&1]; b.any(); b = b.rest() {
		x := b.first()
		if x == sl && k.t.Cards[c].Abilities[k.cards[c].manaAb].Cost.Flags&kop.CostTap != 0 || s.perms[x].flags&pfTapped != 0 || !s.matchPerm(k, f, x, p, sl) {
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

// tapXBoard: p's census gaps are tap-another-permanent and counter-cost
// sources only, or a mix the filter search runs (mixBoard).
func (s *State) tapXBoard(k *prep, p uint8) bool {
	return !s.scriptGaps(k, p).or(s.scriptRestr(k, p)).or(s.scriptAnyR(k, p)).or(s.scriptSac(k, p)).or(s.scriptFilt(k, p)).any() || s.mixBoard(k, p)
}

// tapXPrefix plays the search's script for play a at a tapX board; it
// reports whether it handled the board (the caller then does nothing more).
//
//go:noinline
func (s *State) tapXPrefix(k *prep, a Action, p uint8) bool {
	if !s.scriptTapXModelled(k, p).any() {
		return false
	}
	var d Decision
	s.plainOptions(k, &d, 0)
	if !hasOption(&d, a) {
		s.tapXScript(k, a, p, true)
	}
	return true
}

// tapXStep is one activation: source sl taps candidate cand and makes col.
type tapXStep struct {
	sl, cand Slot
	cand2    Slot // the second tapped permanent of a two-permanent cost (NoSlot: none)
	col      uint8
}

type tapXKey struct {
	pool   [kop.NumManaKinds]uint8
	art    uint8
	tapped slotMask
}

type tapXSearch struct {
	a     Action
	p     uint8
	gaps  slotMask
	nodes int
	nseen int
	seen  [potentialScriptNodes]tapXKey
	n     int
	path  [maxTapXPath]tapXStep
}

// tapXScript runs gorge's prefix search (rules scriptSearch.visit under
// potentialPrefixScript: gapsOnly, funding 0, potentialScriptNodes) for play
// a of p: found. With apply it plays the found script on s.
//
//go:noinline
func (s *State) tapXScript(k *prep, a Action, p uint8, apply bool) bool {
	var rs tapXSearch
	rs.a, rs.p, rs.gaps = a, p, s.scriptTapXModelled(k, p).or(s.scriptCtr(k, p))
	t := s.fork()
	defer s.unfork()
	if !rs.visit(k, t, false) || rs.n == 0 {
		return false
	}
	if apply {
		for i := 0; i < rs.n; i++ {
			s.tapXDo(k, p, rs.path[i])
		}
	}
	return true
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
	if st.cand == NoSlot {
		s.ctrGap(k, p, st.sl) // a counter-cost gap: its counter, its use, its mana
		return
	}
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
	if c := s.cardAt(st.sl); k.t.Cards[c].Abilities[k.cards[c].manaAb].Cost.Flags&kop.CostTap != 0 {
		s.perms[st.sl].flags |= pfTapped
		s.touchSl(st.sl)
	}
	pc := &k.cards[s.cardAt(st.sl)]
	s.pl[p&1].pool[st.col] += max(pc.manaN, 1)
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
	if rs.visit(k, c, true) {
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
	pl := &t.pl[p&1]
	key := tapXKey{pool: pl.pool, art: t.g.poolArt[p&1]}
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
	if gapUsed {
		var d Decision
		t.restrOptions(k, &d, p)
		if hasOption(&d, rs.a) {
			return true
		}
	}
	live := t.scriptTapX(k, p)
	ctr := t.scriptCtr(k, p)
	var order [MaxPerm]Slot
	n := t.battlefieldOrder(p, t.ctrlM[p&1].and(t.occ), order[:], 0)
	for i := 0; i < n; i++ {
		sl := order[i]
		if !rs.gaps.has(sl) {
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
			// scriptAnswers enumerates as per-unit allocations (a first-half
			// option and a second-half one, 64 at most); an odd count has none
			if nc > 2 && !rs.tick() {
				return false
			}
			per := nc / 2
			if nc == 2 {
				per = 1
			}
			n := 0
			for i := 0; i < per && nc%2 == 0; i++ {
				for j := 0; j < per && n < 64; j++ {
					n++
					a, b := cs[i], cs[1]
					if nc > 2 {
						b = cs[per+j]
						if debugTapXMutate == 2 {
							a, b = cs[per-1-i], cs[nc-1-j] // the last pair first (mutation)
						}
					}
					if !rs.tick() { // the colour ask
						return false
					}
					for col := 0; col < 5; col++ {
						c := col
						if debugTapXMutate == 1 {
							c = 0
						}
						if rs.try(k, t, tapXStep{sl: sl, cand: a, cand2: b, col: uint8(c)}) {
							return true
						}
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
			if !rs.tick() { // the colour ask
				return false
			}
			for col := 0; col < 5; col++ {
				c := col
				if debugTapXMutate == 1 {
					c = 0
				}
				if rs.try(k, t, tapXStep{sl: sl, cand: cand, cand2: NoSlot, col: uint8(c)}) {
					return true
				}
			}
		}
	}
	return false
}

// SetDebugTapXMutate is the caretaker lockstep mutation switch (test only):
// 1 = the search always answers the first colour, 2 = it taps the last
// creature candidate first; 0 = off.
func SetDebugTapXMutate(m int) { debugTapXMutate = m }
