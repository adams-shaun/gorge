package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ReplSpellsCantBeCountered (Chimil, the Inner Sun,
// Sphinx of the Final Word): a battlefield "spells you control can't be
// countered" replacement. gorge stops the Counter effect's event
// (counterReplacementMatchesAll, effCounter -> CounterAllowed). The Counter
// effect and ward (effWard; both through effects.counterStopped) are such
// events: a bounce or a move of a spell off the stack (changeZone) and a
// fizzle on illegal targets (CR 608.2b) never ask it, so those kernel sites
// stay as they were.

// StallUncounterOrder: two or more "can't be countered" replacements match
// one Counter event in a shape the order ask below does not pose (a delayed
// or an Effect scope record in the competition, a conditional line), or at
// a kernel site the ask does not run on (the ward, the counter sweep). gorge
// CounterAllowed parks every multi-match on the CR 616.1 order ask
// (replacement_choice.go replChoiceCounter), though all candidates only stop
// the event.
const StallUncounterOrder = 119

// raGenUncOrder is the CR 616.1 order ask of a Counter competition whose
// candidates are all battlefield "spells you control can't be countered"
// replacements (repl_uncounterable.go): the countered spell's controller
// picks which applies first; every candidate only stops the event, so the
// counter is stopped whatever the pick was.
const raGenUncOrder = raGenManifestDread + 82

// uncOrdTag marks the order header (replHdr.ac) as the uncounter order
// ask's; bit 0 set is the answered competition (uncGate consumes it).
const uncOrdTag = 0xe0

func init() { stallNames[StallUncounterOrder] = "uncounterorder" }

// DebugUncounterOff skips the battlefield scan (the mutation: the kernel
// counters a spell gorge refuses to).
var DebugUncounterOff bool

// DebugUncounterStats counts, for the tests (a table with the kind only):
// UncounterHeld the spells the battlefield scan protected from a Counter,
// UncounterPassed those it let through (the Counter went on),
// UncounterStalls the multi-match events that stalled.
var (
	DebugUncounterStats bool
	UncounterHeld       int
	UncounterPassed     int
	UncounterStalls     int
)

// uncStats records one ordinary Counter's outcome for DebugUncounterStats:
// the battlefield scan held the spell (n > 0) or let it through (n == 0).
// self is the spell's own "this spell can't be countered" line, which the
// tests' tables never hold (their count is the battlefield kind alone).
func uncStats(n, self int) {
	if !DebugUncounterStats || self != 0 {
		return
	}
	if n > 0 {
		UncounterHeld++
	} else {
		UncounterPassed++
	}
}

// uncounterable reports whether the spell at stack index i can't be
// countered: its own "this spell can't be countered" (noCtr), a
// battlefield ReplSpellsCantBeCountered source its controller controls,
// still with its abilities, whose Filter matches the spell card, or a fired
// uncounter record naming it. Two or more matches is the CR 616.1 order
// competition, which only the OpCounter path (uncGate) poses; the ward and
// the sweep keep the stall.
//
//go:noinline
func (s *State) uncounterable(k *prep, i int) bool {
	n, self, _, _, _ := s.uncCount(k, i)
	scan := k.anyUncounter && !DebugUncounterOff
	if !scan && n == 0 {
		return false
	}
	uncStats(n, self)
	if n > 1 {
		if DebugUncounterStats {
			UncounterStalls++
		}
		s.capFull(StallUncounterOrder)
	}
	return n > 0
}

// uncCount is uncounterable's count: the matches of a Counter on the spell
// at stack index i — its own "this spell can't be countered", the fired
// delayed records naming it, the Effect scope records over its controller,
// and the battlefield ReplSpellsCantBeCountered sources its controller
// controls — as (total, self, delayed, scope, scan).
func (s *State) uncCount(k *prep, i int) (n, self, nDelayed, nScope, nScan int) {
	e := &s.stack[i]
	if k.cards[e.card].noCtr {
		self = 1
	} else if c := k.cards[e.card].noCtrCond; c != 0 {
		// "if X is 5 or more, this spell can't be countered" (Banefire): the gate reads the entry's X
		if s.condEval(k, c, evalCtx{e: e, src: e.obj, ctrl: e.ctrl}) {
			self = 1
		}
	}
	nDelayed = 0
	if k.gen != nil && k.gen.anyDelaySpell && s.g.ndp != 0 && !DebugUncounterDelayOff {
		nDelayed = s.uncounterDelayed(e)
	}
	nScope = 0
	if k.gen != nil && k.gen.anyUncScope && len(s.cold.exMark) != 0 {
		nScope = s.uncounterScope(k, e) // repl_turnrepl.go: Veil of Summer, Domri
	}
	if k.anyUncounter && !DebugUncounterOff {
		nScan, _ = s.uncounterScan(k, e, nil)
	}
	n = nDelayed + nScope + nScan + self
	return n, self, nDelayed, nScope, nScan
}

// uncGate is the OpCounter path's uncounterable check (gorge effCounter's
// counterStopped). When two or more replacements match — battlefield
// "spells you control can't be countered" sources, the spell's own
// "this spell can't be countered" line, or both — the CR 616.1 order ask is
// posed before the counter proceeds (gorge CounterAllowed parks the
// multi-match, replChoiceCounter): every candidate only stops the event, so
// the counter is stopped whatever the pick was. A delayed or an Effect
// scope record in the competition, a conditional line, a game without a
// lowered table, and the ward and the sweep (which never run this) keep the
// stall.
//
//go:noinline
func (s *State) uncGate(k *prep, i int) (unc, park bool) {
	ro := &s.g.ro
	if ro.ac&uncOrdTag == uncOrdTag {
		// the answered competition: CR 616.1f applies the picked candidate,
		// whose body stops the Counter event, so the event never happened
		// and no candidate is applicable any more
		ro.replHdr = replHdr{}
		return true, false
	}
	n, self, nDelayed, nScope, nScan := s.uncCount(k, i)
	if n > 0 || k.anyUncounter && !DebugUncounterOff {
		uncStats(n, self)
	}
	if n <= 1 {
		return n > 0, false
	}
	if nDelayed != 0 || nScope != 0 || nScan+self != n || k.gen == nil {
		// a competition of shapes this ask does not order keeps the stall
		if DebugUncounterStats {
			UncounterStalls++
		}
		s.capFull(StallUncounterOrder)
		return true, false
	}
	p := s.stack[i].ctrl
	if s.out(p) {
		// CR 800.4a: the affected player has left the game and makes no
		// choices; the candidates apply in scan order
		return true, false
	}
	if !s.uncOrderPose(k, i, p, nScan, self) {
		if DebugUncounterStats {
			UncounterStalls++
		}
		s.capFull(StallUncounterOrder)
		return true, false
	}
	return false, true
}

// uncOrderPose poses the ask: the countered spell's controller (CR 616.1's
// affected player, gorge replChoiceCounter's t.Controller) picks which
// candidate applies first. The candidates are gorge counterReplacementMatchesAll's,
// in its scan order: the battlefield matches (the spell's controller's
// battlefield, entry order) and the spell's own line, which the printed
// walk reads on the stack — a zone forEachObject walks under seat 0
// (object_query.go), so it precedes another seat's battlefield and follows
// the spell controller's own when that is seat 0. false when this run
// cannot pose it (another order header is live, or the candidates do not
// re-gather cleanly), which keeps the stall.
func (s *State) uncOrderPose(k *prep, i int, p uint8, nScan, self int) bool {
	ro := &s.g.ro
	total := nScan + self
	if total > len(ro.cand) || ro.n != 0 || s.g.nc.n != 0 || !s.res.active {
		return false
	}
	var cands [4]Obj
	if n, cond := s.uncounterScan(k, &s.stack[i], &cands); n != nScan || cond {
		return false
	}
	e := &s.stack[i]
	at := 0
	if e.ctrl != 0 && self != 0 {
		ro.cand[0] = e.obj
		at = 1
	}
	for j := 0; j < nScan; j++ {
		ro.cand[at+j] = cands[j]
	}
	if e.ctrl == 0 && self != 0 {
		ro.cand[at+nScan] = e.obj
	}
	ro.n, ro.p, ro.ac, ro.fin = uint8(total), p, uncOrdTag, false
	s.res.ask, s.res.askP = raGenUncOrder, p
	return true
}

// uncOrderOptions is the ask's menu: one candidate a line, in gorge's scan
// order (replOrderOptions' shape, gorge's KReplacement order ask).
func (s *State) uncOrderOptions(d *Decision) {
	ro := &s.g.ro
	d.Kind, d.Sub, d.Player = DkChoose, ChGenReplOrder, ro.p
	for i := 0; i < int(ro.n); i++ {
		d.add(MakeAction(AkChoose, TCode(ro.cand[i]), 0, uint8(i))) // mode: gorge's scan index
	}
}

// applyUncOrder answers the ask: every candidate only stops the event, so
// any pick is recorded as the answered competition and the same instruction
// runs again (uncGate consumes it and skips the counter).
func (s *State) applyUncOrder(k *prep, a Action) {
	ro := &s.g.ro
	i := a.Mode()
	if i >= ro.n || ro.ac&uncOrdTag != uncOrdTag {
		return
	}
	ro.used |= 1 << i
	ro.fin = true
	ro.ac = uncOrdTag | 1
	s.res.ask = raNone
	s.runProg(k) // the same instruction runs again, the competition settled
	if ro.fin {
		ro.replHdr = replHdr{} // nothing consumed the answer
	}
}

// The delayed "the next spell you cast this turn can't be countered"
// (Mistrise Village, Theorist's Proxy; kop.DelaySpellUncounter): gorge's
// Static$ True SpellCast registration resolves its body at the cast with no
// stack object, an Effect whose Counter replacement (Layer$ CantHappen)
// stops a Counter on the remembered cast spell until that spell leaves the
// stack (ForgetOnMoved$ Stack; the Effect has no turn bound). The kernel
// carries the registration as a dpSpell record with dprog.ext
// dpExtUncounter; at the cast, fireDelayedSpell rewrites it in place into
// the fired form (dpSpell clear, objs[0]/vers[0] the cast spell), one more
// match here per record. A record whose spell has left the stack is
// dropped (dropUncounterGone), so it holds its delayed-program slot only
// while the spell is on the stack.

// dpExtUncounter marks a dprog record kop.DelaySpellUncounter's.
const dpExtUncounter uint8 = 3

// DebugUncounterDelayOff ignores the fired uncounter records (the mutation:
// the kernel counters a spell gorge refuses to). Never set in production.
var DebugUncounterDelayOff bool

// dpUncounterFired reports whether d is a fired uncounter record.
func dpUncounterFired(d *dprog) bool {
	return d.ext == dpExtUncounter && d.n&dpSpell == 0
}

// uncounterDelayed counts the fired uncounter records naming the spell of
// stack entry e as it is now.
func (s *State) uncounterDelayed(e *stackEntry) (n int) {
	for i := 0; i < int(s.g.ndp); i++ {
		if d := &s.g.dp[i]; dpUncounterFired(d) && d.objs[0] == e.obj && d.vers[0] == s.objs[e.obj].ver {
			n++
		}
	}
	return n
}

// dropUncounterGone drops every fired uncounter record whose spell has left
// the stack (a new object, CR 400.7), keeping the others in order.
//
//go:noinline
func (s *State) dropUncounterGone() {
	n := int(s.g.ndp)
	keep := 0
	for i := 0; i < n; i++ {
		d := &s.g.dp[i]
		if dpUncounterFired(d) && (s.objs[d.objs[0]].ver != d.vers[0] || s.objs[d.objs[0]].zone != kop.ZoneStack) {
			continue
		}
		s.g.dp[keep], s.g.dpLKI[keep] = *d, s.g.dpLKI[i]
		keep++
	}
	for i := keep; i < n; i++ {
		s.g.dp[i], s.g.dpLKI[i] = dprog{}, 0
	}
	s.g.ndp = uint8(keep)
}

// uncounterScan counts the battlefield replacements that stop a Counter on
// the spell of stack entry e: each ReplSpellsCantBeCountered ability of a
// permanent the spell's controller controls, still with its abilities,
// whose Filter matches the spell card. cands, when not nil, receives each
// match's source object in the same order (gorge's scan order, the
// spell's controller's battlefield); cond reports a counted ability whose
// Repl.Cond this count does not price, which keeps the order ask off.
func (s *State) uncounterScan(k *prep, e *stackEntry, cands *[4]Obj) (n int, cond bool) {
	for m := s.statM.and(s.ctrlM[e.ctrl]); m.any(); m = m.rest() {
		src := m.first()
		if s.perms[src].ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 || k.gen != nil && s.ovrLoses(k, src) {
			continue // the source lost its abilities
		}
		cd := &k.t.Cards[s.cardAt(src)]
		for j := range cd.Abilities {
			a := &cd.Abilities[j]
			if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplSpellsCantBeCountered && s.uncounterMatch(k, a.Repl.Filter, e) {
				if a.Repl.Cond != 0 {
					cond = true // gorge replacementConditionHolds gates this line too
				}
				if cands != nil && n < len(cands) {
					cands[n] = s.perms[src].obj
				}
				n++
			}
		}
	}
	return n, cond
}

// uncounterMatch is f read against the spell of stack entry e.
func (s *State) uncounterMatch(k *prep, f kop.FilterID, e *stackEntry) bool {
	if k.gen != nil {
		return s.matchSpellCard(k, f, e.card, e.ctrl, e.ctrl, e.obj, NoSlot)
	}
	return s.matchCard(k, f, e.card, e.ctrl, e.ctrl)
}

// wardCounterAt is every ward resolution's counter of its warded entry idx:
// a spell that can't be countered stays on the stack (Out Cold, Long
// Goodbye: "This includes by the ward ability"; CR 101.2, 702.21a), as gorge
// effWard asks counterStopped before its MoveZone.
func (s *State) wardCounterAt(k *prep, idx int) {
	if s.stack[idx].kind == seSpell && s.uncounterable(k, idx) {
		return
	}
	s.counterAt(k, idx)
}
