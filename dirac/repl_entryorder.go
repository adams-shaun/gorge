package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The CR 616.1 entry-order ask (The Wandering Minstrel,
// Spelunking). A permanent that enters under both an enters-tapped body and
// an "enters untapped" body (kop.ExtEnterUntapped) is a competition over one
// bit: the last body applied wins. gorge poses it on the MoveZone event,
// before the move (replacement_updated.go composeUpdatedReplacements): the
// affected player (the entering object's controller before the move) picks
// which Updated replacement applies first; the rest is asked again while a
// tap and an untap are both left (CR 616.1f) and otherwise applies in scan
// order. The kernel poses the same ask with the object still in its zone, on
// the two routes that carry a land onto the battlefield in the lockstep
// pools (a land play; a resolving program's hand or ChangeZone move), as
// DkChoose + ChGenReplOrder over s.g.ro. A dig's or a search's found land (a
// DigUntil, a Dig, a library search) is asked the same way with the card still
// in its zone when exactly one tap and one untap compete (entryParkDig,
// entryParkFound); a longer competition there, and every other route that
// meets a pair, stalls (StallEntryOrder), so nothing ever applies a fixed
// order (GORGE-FINDINGS.md, D-entry-order).

// DebugEntryOrderTapLast skips the ask and always applies the tap last (the
// mutation test: gorge poses the order, so the lockstep must diverge).
var DebugEntryOrderTapLast bool

// StallEntryOrder: a tap and an untap body compete over a permanent entering
// by a route that cannot pose the order ask (a token, a clone, a spell
// resolving, a dig or search placement, a self enters program in the
// competition, a body this ask cannot class).
const StallEntryOrder = 65

func init() { stallNames[StallEntryOrder] = "entryorder" }

// The ask borrows replHdr's combat-only bytes, which a pass that is not
// paused leaves unused (the ask is parked only with ro.n, on and nca clear),
// so the State does not grow: prev / shld are the candidates that tap / untap
// the entrant (a bit per candidate, as the damage ask's prevention bits),
// pcol the state byte below (ent), cur the entering object (entObj).
func (h *replHdr) entObj() Obj { return Obj(h.cur) }

// entHdr: the header is the entry ask's. A damage ask (pcol there is the
// PreventColor candidates) always has on (combat) or nca (noncombat, the
// untap step) set, the entry ask never does: every read of the ent bits goes
// through this, or the damage ask's bits would be taken for a route.
func (h *replHdr) entHdr() bool { return !h.on && !h.nca }

// entryAsked: the entry-order ask is parked (posed or being answered), by a
// land play.
func (s *State) entryAsked(k *prep) bool {
	ro := &s.g.ro
	return k.gen.anyEntUntap && ro.n > 0 && ro.entHdr() && ro.pcol&entMode == entLand
}

// entryAnswered: the order ask of o was answered and its result is not yet
// consumed.
func (h *replHdr) entryAnswered(o Obj) bool {
	return h.entHdr() && h.pcol&entDone != 0 && h.entObj() == o
}

// raGenEntryOrder: the entry-order ask of a resolving program's move
// (s.g.ro, ro.pcol), asked with the card still in its zone.
const raGenEntryOrder = raGenManifestDread + 73

// ro.pcol: the low two bits name the parked route, entDone marks the answer
// that lets the route run again (enterGen consumes the result), bits 3-4 are
// that result (entOvrTap / entOvrUntap). gorge emits the move at the first
// answer, so when a tap and an untap are still left (CR 616.1f) the next ask
// is posed with the permanent already on the battlefield: entMore asks
// enterGen to leave the competition parked, entLate marks it entered.
const (
	entLand   = 1 // playLandNow
	entOp     = 2 // OpReturnEx (a ChangeZone to the battlefield)
	entSearch = 3 // a one-card hand move (OpChooseMove) to the battlefield
	entMode   = 3
	entDone   = 4
	entOvrSh  = 3
	entMore   = 32
	entLate   = 64
	entEarly  = 128 // the entering card's own line precedes its owner's battlefield in the scan

	entOvrTap   = 1
	entOvrUntap = 2
)

// body classes
const (
	entClsTap = iota + 1
	entClsUntap
	entClsOther
	entClsRecord
)

// entBodyClass classes a replacement program by its effect on the tapped bit.
func entBodyClass(prog []kop.Instr) uint8 {
	if len(prog) == 1 {
		if prog[0].Op == kop.OpEnterTapped {
			return entClsTap
		}
		if prog[0].Ext() == kop.ExtEnterUntapped {
			return entClsUntap
		}
	}
	return entClsOther
}

// entBodyRecord reports whether a replacement program is one of the
// as-enters choice records the entry ask poses (enterChoiceKind: a colour,
// a type, a number or a name): gorge classes such a body "record"
// (replacement_updated.go, updatedNeutralBodySet), which commutes with any
// other body, so it joins the CR 616.1 order ask without competing for a
// tapped bit. Never OpUnlessPaid (a shock land's gate is a tap behind its
// election, not a record).
func entBodyRecord(prog []kop.Instr) bool {
	if len(prog) != 1 {
		return false
	}
	switch prog[0].Op {
	case kop.OpChooseColor, kop.OpChooseType:
		return true
	case kop.OpExt:
		switch prog[0].Ext() {
		case kop.ExtChooseNumber, kop.ExtChooseName:
			return true
		}
	}
	return false
}

// entBodyInert reports whether a replacement program only adds counters to the
// entrant: gorge classes such a body "counter" (replacement_updated.go,
// updatedBodyClass), which composes with any other in either order.
func entBodyInert(prog []kop.Instr) bool {
	for i := range prog {
		if prog[i].Op != kop.OpEnterWithCounters {
			return false
		}
	}
	return len(prog) > 0
}

// entBodyHard reports whether a replacement program enters the entrant
// attacking (ExtEnterAttacking, Kira Nerys): gorge classes the ChangeZone body
// "other" (replacement_updated.go, updatedBodyClass), which commutes with no
// other body, so beside any second enter body gorge asks the CR 616.1 order,
// and the order decides the result (picked after a counter body the entrant has
// entered, and Kira's body no longer applies).
func entBodyHard(prog []kop.Instr) bool {
	for i := range prog {
		if prog[i].Ext() == kop.ExtEnterAttacking {
			return true
		}
	}
	return false
}

// prepEntUntap sets anyEntUntap: some card has an other-enter program that
// untaps the entrant (ExtEnterUntapped), and anyEntAttack: some card has one
// that enters it attacking.
func prepEntUntap(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplOthersEnterProgram {
				continue
			}
			g.anyEntUntap = g.anyEntUntap || entBodyClass(a.Prog) == entClsUntap
			g.anyEntAttack = g.anyEntAttack || entBodyHard(a.Prog)
		}
	}
}

// entComp is the competing bodies for one entering permanent, in gorge's
// scan order.
type entComp struct {
	n          uint8
	cand       [8]Obj
	tap, untap uint8 // bit per candidate
	inert      uint8 // bit per candidate whose body only adds counters (it commutes with every other such body)
	early      bool  // the entrant's own line was scanned before its owner's battlefield
	anyTap     bool  // some body taps / untaps, kept when the candidate list is full
	anyUntap   bool
	over       bool  // more than eight candidates
	other      bool  // a body this ask cannot class (a self enters program, counters)
	hard       bool  // some live body enters the entrant attacking (entBodyHard): it fails to commute with any other body
	selfLive   uint8 // the entrant's own classless bodies entryComp leaves out of cand (counters, another enter program): they still compete for the stall decision
}

func (c *entComp) add(src Obj, cls uint8) {
	switch cls {
	case entClsTap:
		c.anyTap = true
	case entClsUntap:
		c.anyUntap = true
	}
	if c.n == uint8(len(c.cand)) {
		c.over = true
		return
	}
	switch cls {
	case entClsTap:
		c.tap |= 1 << c.n
	case entClsUntap:
		c.untap |= 1 << c.n
	case entClsRecord:
		// no tap or untap bit: the body records an as-enters choice asked
		// before the move (preLandAsk), it changes no tapped bit here
	default:
		c.other = true
	}
	c.cand[c.n] = src
	c.n++
}

// entryComp collects the Updated bodies that apply to object o entering
// under ctrl: gorge's scan (forEachReplacementSourceFor) visits each seat in
// turn, a seat's library and hand first, then its battlefield, then its
// graveyard and exile, so the entering card's own line comes before its
// owner's battlefield when it enters from a hand or library. skip is the
// entering permanent's own slot when it is already on the battlefield. pair
// is true when a tap and an untap compete.
func (s *State) entryComp(k *prep, o Obj, ctrl uint8, skip Slot, fixEarly bool) (c entComp, pair bool) {
	g := k.gen
	ob := &s.objs[o]
	own := ob.owner()
	cid := ob.card
	self := func() {
		if s.faceDownAt(k, o) {
			return // CR 708.2: a face-down permanent has no abilities, so its own enter bodies do not apply
		}
		if k.cards[cid].enterTap {
			c.add(o, entClsTap)
		}
		if k.cards[cid].enterCtr != 0 {
			c.other = true
			c.selfLive++ // gorge composes it as its own With body: it competes, but entryComp cannot list it in cand
		}
		if g.has(cid, gfEnterRepl) {
			// The card's own as-enters choice records (Sunken Citadel's "As
			// it enters, choose a color") join the ask as record candidates:
			// gorge classes each body "record" and lists it in the ask, and
			// the choice was already taken before the move (preLandAsk), so
			// a record pick changes no tapped bit here. enterTap stays
			// first, gorge's face order (kw_etbreplacement.go appends the
			// record bodies after the R: lines).
			nonRec := false
			for i := range k.t.Cards[cid].Abilities {
				a := &k.t.Cards[cid].Abilities[i]
				if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplEntersProgram {
					continue
				}
				if entBodyRecord(a.Prog) {
					c.add(o, entClsRecord)
				} else {
					nonRec = true
				}
			}
			if nonRec {
				if s.condEnterTapped(k, cid) != nil || s.shockCardAmount(k, cid) > 0 {
					c.add(o, entClsTap) // a shock land's body is a Tap behind its pay-life gate: gorge classes it a tap
				} else {
					c.other = true
					c.selfLive++ // a self enters program this ask cannot class, as gorge's "other" candidate
				}
			}
		}
	}
	early := ob.zone == kop.ZoneLibrary || ob.zone == kop.ZoneHand
	if skip != NoSlot {
		early = fixEarly // already entered (a re-ask, or enterGen): the scan order of the first ask
	}
	c.early = early
	// The entrant is already on the battlefield on the enterGen routes (a re-ask,
	// a late ask), where the filter reads it as it is: a face-down entrant is a
	// 2/2 colourless creature even over a noncreature card (CR 708.2). Before the
	// move it is not there yet, so NoObj keeps the battlefield-zone read off it.
	fo := NoObj
	if ob.zone == kop.ZoneBattlefield {
		fo = o
	}
	for pp := uint8(0); pp < s.nSeats(); pp++ {
		if s.out(pp) {
			continue
		}
		if own == pp && early {
			self()
		}
		var bf [MaxPerm]Slot
		for _, b := range bf[:s.battlefieldOrder(pp, s.statM.and(s.ctrlM[pp]), bf[:], 0)] {
			if b == skip || s.genLost(k, b) {
				continue
			}
			scid := s.cardAt(b)
			if !g.has(scid, gfOthersRep) {
				continue
			}
			cd := &k.t.Cards[scid]
			for i := range cd.Abilities {
				a := &cd.Abilities[i]
				if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplOthersEnterProgram {
					continue
				}
				if !s.matchCardGen(k, a.Repl.Filter, cid, ctrl, s.perms[b].ctrl, fo) {
					continue
				}
				// A body gated on IsPresent$ is a candidate only while its gate
				// holds (gorge's replacementMatches); the entrant is off the
				// battlefield on the ask-before-the-move routes.
				if a.Repl.Cond != 0 && !s.condEval(k, a.Repl.Cond, evalCtx{src: s.perms[b].obj, srcSl: b, ctrl: s.perms[b].ctrl}) {
					continue
				}
				n0 := c.n
				c.add(s.perms[b].obj, entBodyClass(a.Prog))
				if c.n > n0 && entBodyInert(a.Prog) {
					c.inert |= 1 << n0
				}
				if c.n > n0 && entBodyHard(a.Prog) {
					c.hard = true
				}
			}
		}
		if own == pp && !early {
			self()
		}
	}
	return c, c.anyTap && c.anyUntap
}

// condEnterTapped is card c's enters-tapped body with a condition in its effect
// (Starting Town), nil when it has none or another enters program. Gorge's scan lists
// it whatever the condition says: the condition is read as the body applies.
func (s *State) condEnterTapped(k *prep, c CardID) *kop.Ability {
	var found *kop.Ability
	if !k.gen.has(c, gfEnterRepl) {
		return nil
	}
	for i := range k.t.Cards[c].Abilities {
		a := &k.t.Cards[c].Abilities[i]
		if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplEntersProgram {
			continue
		}
		if entBodyClass(a.Prog) != entClsTap || found != nil {
			return nil
		}
		found = a
	}
	return found
}

// entryOrderPark poses the order ask for o entering under ctrl on route
// mode, with o still in its zone, when a tap and an untap compete and every
// body is one this ask orders; false when the entry proceeds (no pair, the
// ask already answered, or an unsupported shape that enterGen stalls on).
func (s *State) entryOrderPark(k *prep, o Obj, ctrl uint8, mode uint8) bool {
	if k.gen == nil || !k.gen.anyEntUntap {
		return false
	}
	ro := &s.g.ro
	if ro.entryAnswered(o) || DebugEntryOrderTapLast {
		return false
	}
	if ro.n > 0 || ro.on || ro.nca {
		return false
	}
	c, pair := s.entryComp(k, o, ctrl, NoSlot, false)
	if !pair || c.over || c.other || s.objs[o].owner() != ctrl {
		return false
	}
	if c.early {
		mode |= entEarly
	}
	ro.replHdr = replHdr{n: c.n, prev: c.tap, shld: c.untap, p: ctrl, pcol: mode, cur: Slot(o)}
	return true
}

// pick outcomes
const (
	entAsk = iota // ask again (the permanent is already on the battlefield)
	entGo         // the route runs now (the first answer, or the last)
	entFin        // the last answer of a late ask: the order is applied
)

// entryOrderPick records a pick. gorge re-checks the rest after each pick
// (CR 616.1e) and asks again only while the rest still holds a tap and an
// untap; otherwise it applies them in scan order, so the last body applied
// (the last of the rest, or the pick itself when none is left) decides the
// tapped bit.
func (s *State) entryOrderPick(k *prep, a Action) int {
	ro := &s.g.ro
	i := a.Mode()
	if i >= ro.n || ro.used&(1<<i) != 0 {
		return entAsk // not an option: ask again
	}
	ro.used |= 1 << i
	late := ro.pcol&entLate != 0
	cls := uint8(0) // a record candidate (no tap, no untap bit): the pick changes no tapped bit
	if ro.prev&(1<<i) != 0 {
		cls = entOvrTap
	} else if ro.shld&(1<<i) != 0 {
		cls = entOvrUntap
	}
	left := (uint8(1)<<ro.n - 1) &^ ro.used
	if left&ro.prev != 0 && left&ro.shld != 0 {
		if late {
			s.entryLateSet(k, i, cls)
			return entAsk
		}
		ro.pcol = ro.pcol&(entMode|entEarly) | entDone | entMore | cls<<entOvrSh
		return entGo
	}
	last := i
	for j := uint8(0); j < ro.n; j++ {
		// the rest applies in scan order (CR 616.1e): the last of it that
		// touches the tapped bit decides; a record candidate in between
		// re-applies the choice asked before the move and changes nothing
		if left&(1<<j) == 0 || ro.prev&(1<<j) == 0 && ro.shld&(1<<j) == 0 {
			continue
		}
		last = j
		cls = entOvrUntap
		if ro.prev&(1<<j) != 0 {
			cls = entOvrTap
		}
	}
	if late {
		s.entryLateSet(k, last, cls)
		s.g.ro.replHdr = replHdr{}
		return entFin
	}
	ro.n = 0
	ro.pcol = ro.pcol&(entMode|entEarly) | entDone | cls<<entOvrSh
	return entGo
}

// entryLateSet applies body class cls, the body at scan index idx, to the
// entered permanent. The entrant's own conditional enters-tapped body (The
// Unicomplex's "unless you control two or more other lands") reads its
// condition as it applies, as entryOrderEnter's first answer does, and taps
// nothing when it fails.
func (s *State) entryLateSet(k *prep, idx, cls uint8) {
	if cls == 0 {
		return // a record pick: the choice was asked before the move, it changes no tapped bit
	}
	o := s.g.ro.entObj()
	if s.objs[o].zone != kop.ZoneBattlefield {
		return
	}
	sl := s.objs[o].slot
	if cls == entOvrTap && s.entrantCondTapFails(k, o, sl, idx) {
		return
	}
	s.touch(o)
	if p := &s.perms[sl]; cls == entOvrTap {
		if s.shockLateTap(k, o, sl, idx) {
			// The body is the entrant's own shock-land Tap behind its
			// pay-life election, and the election is still queued: the tap
			// belongs to the election's decline (shockAnswer runs the Tap
			// body then), so applying it here would show a tapped land at
			// the election ask and pay the tap even when the player pays.
			p.flags &^= pfTapped
			return
		}
		p.flags |= pfTapped
	} else {
		p.flags &^= pfTapped
	}
}

// shockLateTap reports whether scan index idx of the late entry-order ask is
// the entering permanent's own shock-land Tap body whose pay-life election is
// still queued. The election's answer owns the tap: shockAnswer runs the Tap
// body on a decline and skips it on a pay.
func (s *State) shockLateTap(k *prep, o Obj, sl Slot, idx uint8) bool {
	cid := s.objs[o].card
	if s.shockCardAmount(k, cid) == 0 {
		return false
	}
	ro := &s.g.ro
	c, _ := s.entryComp(k, o, ro.p, sl, ro.pcol&entEarly != 0)
	if int(idx) >= int(c.n) || c.cand[idx] != o {
		return false
	}
	for i := 0; i < int(s.g.nask); i++ {
		if a := &s.g.ask[i]; a.kind == akPayLife && a.sl == sl {
			return true
		}
	}
	return false
}

// entrantCondTapFails: the body at scan index idx of the late ask is the
// entered permanent's own conditional enters-tapped body and its condition
// fails, so applying it taps nothing.
//
//go:noinline
func (s *State) entrantCondTapFails(k *prep, o Obj, sl Slot, idx uint8) bool {
	cid := s.objs[o].card
	a := s.condEnterTapped(k, cid)
	if a == nil || k.cards[cid].enterTap {
		return false
	}
	ro := &s.g.ro
	c, _ := s.entryComp(k, o, ro.p, sl, ro.pcol&entEarly != 0)
	if idx >= c.n || c.cand[idx] != o {
		return false
	}
	return !s.condEval(k, a.Repl.Cond, evalCtx{src: o, srcSl: sl, ctrl: s.perms[sl].ctrl})
}

// applyEntryLand answers the land play's order ask: once the first pick (or
// the last) is in, the play runs again with the order's result; a re-ask
// follows the entry.
func (s *State) applyEntryLand(k *prep, a Action) {
	switch s.entryOrderPick(k, a) {
	case entAsk:
		return
	case entFin:
		s.ctl = ctlPriority
		return
	}
	s.ctl = ctlPriority
	s.playLandNow(k, s.g.ro.entObj(), s.prio)
	if s.g.ro.n > 0 && s.g.ro.pcol&entLate != 0 {
		s.g.ctlRet, s.ctl = ctlPriority, ctlGenAsk
		return
	}
	s.entryOrderClear()
}

// applyEntryResolve answers a resolving program's order ask.
func (s *State) applyEntryResolve(k *prep, a Action) {
	switch s.entryOrderPick(k, a) {
	case entAsk:
		return
	case entFin:
		if s.res.left != 0 && s.unparkTail(k) {
			return // a dig's or search's later cards, parked behind the late ask
		}
		s.continueRes(k, 0) // the move was made: the program goes on
		return
	}
	s.res.ask = raNone
	if s.g.ro.pcol&entMode == entOp && entryRerunOp(s.prog(k)[s.res.pc].Op) {
		if s.entryDigRerun(k) {
			s.entryOrderClear()
			s.continueRes(k, 0)
		}
		return
	}
	if s.g.ro.pcol&entMode == entSearch {
		s.finishSearch(k) // the one-card hand move again, from its first step
	} else {
		s.runProg(k) // the same instruction again
	}
	s.entryOrderClear()
}

// entryLatePark suspends the resolution on the re-ask that follows an entry
// (the permanent is on the battlefield, a tap and an untap are still left).
func (s *State) entryLatePark() bool {
	ro := &s.g.ro
	if ro.n == 0 || !ro.entHdr() || ro.pcol&entLate == 0 {
		return false
	}
	s.res.ask, s.res.askP = raGenEntryOrder, ro.p
	s.ctl = ctlResolve
	return true
}

// entryOrderClear drops an answered result nothing consumed.
func (s *State) entryOrderClear() {
	if s.g.ro.entHdr() && s.g.ro.pcol&entDone != 0 {
		s.g.ro.replHdr = replHdr{}
	}
}

// entryParkOp parks OpReturnEx instruction in's move of o to the battlefield
// under ctrl (before detach or any other effect of the move); true when the
// resolution is suspended on the ask.
func (s *State) entryParkOp(k *prep, in *kop.Instr, o Obj, ctrl uint8) bool {
	if k.gen == nil || !k.gen.anyEntUntap || !s.res.active || in != &s.prog(k)[s.res.pc] || in.C>>3&kop.MvForgetOther != 0 {
		return false
	}
	if !s.entryOrderPark(k, o, ctrl, entOp) {
		return false
	}
	s.res.ask, s.res.askP = raGenEntryOrder, s.g.ro.p
	return true
}

// entryParkDig parks the move of o onto the battlefield that a dig or a mass move makes (a
// DigUntil's found card, a Dig's taken card, a ChangeZoneAll's card), o still in the library with
// the rest of the look. Only the two-body competition is ordered here, one tap
// against one untap: the first pick decides it, so no re-ask follows the entry
// (CR 616.1e) and the dig runs again from the answer. A longer competition
// stalls in enterGen (StallEntryOrder).
func (s *State) entryParkDig(k *prep, o Obj, ctrl uint8) bool {
	if k.gen == nil || !k.gen.anyEntUntap || !s.res.active || s.res.ask != raNone {
		return false
	}
	if !entryRerunOp(s.prog(k)[s.res.pc].Op) {
		return false
	}
	if !s.entryOrderPark(k, o, ctrl, entOp) {
		return false
	}
	s.res.ask, s.res.askP = raGenEntryOrder, s.g.ro.p
	return true
}

// entryParkFound parks the move of a searched-for card o onto the battlefield
// under ctrl (moveFoundAns), o still in its zone: the same two-body ask as a
// dig's, answered by running the search's move again from o.
func (s *State) entryParkFound(k *prep, o Obj, ctrl uint8) bool {
	if k.gen == nil || !k.gen.anyEntUntap || !s.res.active || s.res.ask != raNone {
		return false
	}
	if !s.entryOrderPark(k, o, ctrl, entSearch) {
		return false
	}
	s.res.ask, s.res.askP = raGenEntryOrder, s.g.ro.p
	return true
}

// entryLateTail parks the dig or search behind the re-ask (CR 616.1f) that follows the
// entry of the card it just moved, when a tap and an untap are still left; its later
// cards move once the order is applied (unparkTail runs the tail kind). false: no re-ask.
func (s *State) entryLateTail(k *prep, kind uint8) bool {
	if k.gen == nil || !k.gen.anyEntUntap || s.g.nask != 0 || !s.res.active || !s.entryLatePark() {
		return false
	}
	s.res.left = kind
	return true
}

// entryRerunOp: the instruction rerunMove runs again from an order answer, a dig's or
// a mass move's.
func entryRerunOp(op kop.Op) bool {
	switch op {
	case kop.OpDigUntil, kop.OpDigEx, kop.OpChangeZoneAll, kop.OpChangeZoneAllEx:
		return true
	}
	return false
}

// entryDigRerun runs the parked dig again from the order answer; false when
// it suspended at another ask.
func (s *State) entryDigRerun(k *prep) bool {
	return !s.rerunMove(k, &s.prog(k)[s.res.pc])
}

// entryParkSearch parks a one-card hand move to the battlefield that a search
// op is about to make (finishSearch).
func (s *State) entryParkSearch(k *prep) bool {
	if k.gen == nil || !k.gen.anyEntUntap || s.g.nlook != 1 || kop.Zone(s.g.pickZ) != kop.ZoneBattlefield {
		return false
	}
	if in := &s.prog(k)[s.res.pc]; in.Op != kop.OpChooseMove || s.searchZone(k) != kop.ZoneHand || s.g.pickF&(kop.SrGraveExile|kop.SrUnderYourControl) != 0 {
		return false
	}
	o := s.resArm().look[0]
	if s.objs[o].zone != kop.ZoneHand || s.handAuraArmed(k) {
		return false
	}
	if !s.entryOrderPark(k, o, s.g.pickP, entSearch) {
		return false
	}
	s.res.ask, s.res.askP = raGenEntryOrder, s.g.ro.p
	return true
}

// entryOrderEnter is enterGen's hook for the permanent o at sl: the answered
// result of the order ask (consumed), else 0. A pair that reaches here
// unanswered entered by a route that cannot pose the ask: stall.
func (s *State) entryOrderEnter(k *prep, sl Slot, o Obj) uint8 {
	ro := &s.g.ro
	if ro.entryAnswered(o) {
		v := ro.pcol >> entOvrSh & 3
		if a := s.condEnterTapped(k, s.objs[o].card); v == entOvrTap && a != nil && !s.condEval(k, a.Repl.Cond, evalCtx{src: o, srcSl: sl, ctrl: s.perms[sl].ctrl}) {
			v = entOvrUntap // the body applied last taps nothing when its condition fails
		}
		if ro.pcol&entMore != 0 {
			ro.pcol = ro.pcol&(entMode|entEarly) | entLate // the rest is asked once the entry is done
		} else {
			ro.replHdr = replHdr{}
		}
		return v
	}
	if c, pair := s.entryComp(k, o, s.perms[sl].ctrl, sl, false); pair || c.hard && c.n+c.selfLive > 1 {
		// A hard (enters-attacking) body does not commute with any second body,
		// the entrant's own counters included: gorge composes the entry's
		// candidates and asks, which this route cannot pose.
		if DebugEntryOrderTapLast {
			return entOvrTap
		}
		if c.over {
			s.capFull(StallReplCands)
		} else {
			s.capFull(StallEntryOrder)
		}
	}
	return 0
}

// entryOrderOptions poses the ask: the affected player picks which body
// applies next among those not yet applied. The candidates are derived from
// the state again (nothing they read changes between asks), in the scan
// order of the first ask.
func (s *State) entryOrderOptions(k *prep, d *Decision) {
	ro := &s.g.ro
	d.Kind, d.Sub, d.Player = DkChoose, ChGenReplOrder, ro.p
	skip := NoSlot
	if ro.pcol&entLate != 0 {
		skip = s.objs[ro.entObj()].slot
	}
	c, _ := s.entryComp(k, ro.entObj(), ro.p, skip, ro.pcol&entEarly != 0)
	for i := uint8(0); i < ro.n && i < c.n; i++ {
		if ro.used&(1<<i) == 0 {
			d.add(MakeAction(AkChoose, uint8(c.cand[i]), 0, i)) // mode: gorge's scan index
		}
	}
}
