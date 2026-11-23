package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ReplPreventDamageFilt and ReplPreventDamageN: a permanent's printed static
// prevention of damage (gorge R:Event$ DamageDone | Prevent$ True, and the
// DB$ ReplaceDamage | Amount$ N shield family: the six Spheres, Urza's Armor,
// Muzzle, Bubble Matrix, Statecraft). gorge (rules/replacement_damage.go)
// collects each as a candidate of a damage event whose source and recipient
// match its filters: a full prevention stops the event, a shield prevents
// min(N, what is left) and the event stands with the rest (CR 615.7). They
// are stateless: no pool, no depletion, no State byte; the card data carries
// the recipient filter (Repl.Filter and the player bits), the source filter
// (Repl.Filter2), the gate (Repl.Cond) and the amount (Repl.A / Repl.V).
//
// One candidate applies on its own. Two or more are gorge's CR 616.1 order ask
// unless every one is a bare full prevention (whichever applies first stops the
// whole event: repl_bareprev.go), which the kernel applies without the ask. A
// noncombat event of a one-event site (damageTo) with two or more other
// candidates parks on the ask (spPark): the options name each host, each pick
// is applied as made to the damage left (replHdr.an), the last candidate applies on its own and a
// prevention that leaves nothing ends the asks. An Optional$ shield alone
// (Battletide Alchemist) is asked of its controller with a skip option. A
// combat event's orderable competition is posed by the combat walk
// (spCombatCands): each pick applies as made (spPick), and dealAsked aligns
// the recomputed pass's event with the picked order because the candidates
// need not commute. A competition beside an optional line or a DamageAmount$
// gate, and a host with two shields, stall StallReplPlus: the walk cannot name
// such candidates. So does such a competition in a pass
// that re-executes its op on a plus-N order ask (damageBatch, a hit batch,
// DealDamageFrom): a static prevention table is never replPlusOnly, since that
// protocol discards the answer. A table that also has any other
// damage replacement (or a self-damage one) is refused at its start
// (initStall: staticPrevStall), since the kinds do not commute; the one pair
// lifted is a combat-only scoped Effect record (Arachnogenesis), whose full
// prevention the combat walk poses beside the static prevention (spBesidePrevAll).

// useStaticPrev: the table has a static damage prevention.
var useStaticPrev = newGenUse(tableHasStaticPrev)

// spMax bounds the candidates of one event; more stall StallReplCands.
const spMax = 4

// spOptionalAsk is replHdr.spv's bit for an ask whose lone candidate is an
// Optional$ line: a skip option follows the candidate.
const spOptionalAsk uint8 = 1 << 7

// spShieldAsk is replHdr.spv's bit for an ask with an all-but shield among its
// candidates beside a doubler or a plus-N: the picks do not commute, so each is
// applied to the running amount as made (roPlusPend, repl_pluswalk.go). The
// candidates use the low bits (four at most), so bits 6 and 7 are free.
const spShieldAsk uint8 = 1 << 6

// spHasShield: one of the c candidates cs is an all-but shield, not a full prevention.
func (s *State) spHasShield(k *prep, cs *[spMax]spCand, c int) bool {
	for i := 0; i < c; i++ {
		if s.spCandAb(k, cs[i]).Repl.Kind == kop.ReplPreventDamageN || s.spSelfCand(k, cs[i]) {
			return true // a candidate applied as picked and spent (a Phantom's counter) asks as a shield does
		}
	}
	return false
}

func tableHasStaticPrev(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if isStaticPrev(&t.Cards[i].Abilities[j]) {
				return true
			}
		}
	}
	return tableEmbHasStaticPrev(t)
}

// isStaticPrev: a is a static damage prevention (either kind).
func isStaticPrev(a *kop.Ability) bool {
	return a.Kind == kop.AbReplacement && (a.Repl.Kind == kop.ReplPreventDamageFilt || a.Repl.Kind == kop.ReplPreventDamageN)
}

// spEv is one damage event: its source (a permanent, else a card), its
// recipient (a permanent at sl controlled by rcp, or the player rcp when sl is
// NoSlot) and the amount.
type spEv struct {
	srcCtrl uint8
	srcSl   Slot
	srcCard CardID
	rcp     uint8
	sl      Slot
	combat  bool
	n       int
}

// spCand is one matching ability: the host permanent and the ability.
type spCand struct {
	b  Slot
	ab uint8
}

// spMatch reports whether the static prevention a of the host at b is a
// candidate of event e (gorge damageReplacementMatches).
func (s *State) spMatch(k *prep, b Slot, a *kop.Ability, e *spEv) bool {
	return s.spMatchAs(k, s.perms[b].ctrl, b, s.perms[b].obj, a, e)
}

// spMatchAs is spMatch for a prevention controlled by hc whose host is the
// permanent at b (object src); an emblem's prevention has no host (b NoSlot,
// src NoObj).
func (s *State) spMatchAs(k *prep, hc uint8, b Slot, src Obj, a *kop.Ability, e *spEv) bool {
	r := &a.Repl
	c := r.C
	if c&kop.ReplSPCombat != 0 && !e.combat || c&kop.ReplSPNonCombat != 0 && e.combat ||
		c&kop.ReplSPYourTurn != 0 && s.active != hc || c&kop.ReplSPSrcInPlay != 0 && e.srcSl == NoSlot ||
		c&kop.ReplSPDmgLE != 0 && e.n > int(r.A) || c&kop.ReplSPSrcSpell != 0 && (e.srcSl != NoSlot || s.res.e.kind != seSpell) {
		return false
	}
	ctx := evalCtx{src: src, srcSl: b, ctrl: hc}
	if r.Cond != 0 && !s.condEval(k, r.Cond, ctx) {
		return false
	}
	switch {
	case e.sl == NoSlot:
		if !(c&kop.ReplSPYou != 0 && e.rcp == hc) && !(c&kop.ReplSPOpp != 0 && e.rcp != hc) {
			return false
		}
	case r.Filter != 0:
		if !s.matchPermGen(k, r.Filter, e.sl, hc, b, layerFull) {
			return false
		}
	case c&kop.ReplSPAnyPerm == 0:
		return false
	}
	if c&kop.ReplSPRecipSnow != 0 && (e.sl == NoSlot || !k.t.Cards[s.cardAt(e.sl)].Snow) {
		return false
	}
	if r.Filter2 == 0 {
		return true
	}
	if e.srcSl != NoSlot {
		return s.matchPermGen(k, r.Filter2, e.srcSl, hc, b, layerFull)
	}
	return s.matchSourceCard(k, r.Filter2, e.srcCard, e.srcCtrl, hc, &ctx)
}

// matchSourceCard is matchCardGenAt for a damage source that has left the
// battlefield (or is a spell): a Card.Self alternative means the source is
// the host, but the host is on the battlefield (the static's ActiveZones$
// Battlefield) and the source is not, so it can never match. The generic card
// matcher ignores filter flags, and the empty type set of a bare Self clause
// would otherwise match every departed source (Indentured Oaf's prevention
// against damage from a sacrificed Land Mine token, CR 615.1a with CR 609.7).
func (s *State) matchSourceCard(k *prep, f kop.FilterID, c CardID, owner, ctrl uint8, ctx *evalCtx) bool {
	for ; f != 0; f = k.t.Filters[f].Or {
		fl := &k.t.Filters[f]
		if fl.Flags&kop.FSelf != 0 {
			continue
		}
		if s.matchCardOneAt(k, fl, c, owner, ctrl, NoObj, ctx, false) {
			return true
		}
	}
	return false
}

// spCollect fills out with the candidates of e in gorge's scan order (each
// seat's battlefield in entry order, the abilities in print order) and returns
// their number; -1 after a loud stall (more than spMax). A can't-prevent static
// (CR 615.12) leaves none.
func (s *State) spCollect(k *prep, e *spEv, out *[spMax]spCand) int {
	if s.noPrev(k, e.combat) {
		return s.spSelfNoPrev(k, e) // repl_selfph.go
	}
	g := k.gen
	n := 0
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		var bf [MaxPerm]Slot
		for _, b := range bf[:s.battlefieldOrder(p, s.statM.and(s.ctrlM[p]), bf[:], 0)] {
			if g.replBits[s.cardAt(b)]&(rbPlusDmg|rbPrevCtrSelf) == 0 || s.genLost(k, b) {
				continue
			}
			ab := k.t.Cards[s.cardAt(b)].Abilities
			for j := range ab {
				if !(isStaticPrev(&ab[j]) && s.spMatch(k, b, &ab[j], e) || s.spSelfMatch(g, b, &ab[j], e)) {
					continue
				}
				if n == spMax {
					s.capFull(StallReplCands) // loud: never truncate the candidates
					return -1
				}
				out[n] = spCand{b, uint8(j)}
				n++
			}
		}
	}
	return s.spCollectEmb(k, e, out, n)
}

// spPrevented is the damage the shield a of the host at b prevents from the
// left that remains: all of it for a full prevention, else min(amount, left).
func (s *State) spPrevented(k *prep, b Slot, a *kop.Ability, left int) int {
	if a.Repl.Kind == kop.ReplPreventDamageFilt || a.Repl.Kind == kop.ReplPreventDamageCounters {
		return left // a full prevention (the Phantom's line removes its counter in spSpend)
	}
	r := &a.Repl
	var m int
	switch c := r.C; {
	case c&kop.ReplSPAllBut != 0:
		m = left - int(r.A)
	case b >= MaxPerm:
		return 0 // an emblem has no host to read a counter or a value from (the lowering refuses them)
	case c&kop.ReplSPLimitCtr != 0:
		m = s.perms[b].ctrN(s, int(r.A))
	case c&kop.ReplSPAmtV != 0:
		m = s.evalV(k, r.V, evalCtx{src: s.perms[b].obj, srcSl: b, ctrl: s.perms[b].ctrl})
	default:
		m = int(r.A)
	}
	return max(0, min(m, left))
}

// spBare: every one of the cs candidates is a non-optional full prevention
// (gorge allBarePreventions), so no order ask is posed.
func (s *State) spBare(k *prep, cs *[spMax]spCand, c int) bool {
	for i := 0; i < c; i++ {
		a := s.spCandAb(k, cs[i])
		if a.Repl.Kind != kop.ReplPreventDamageFilt || a.Repl.C&kop.ReplSPOptional != 0 {
			return false
		}
	}
	return true
}

// spCandsOptional: one of the c candidates is an Optional$ line.
func (s *State) spCandsOptional(k *prep, cs *[spMax]spCand, c int) bool {
	for i := 0; i < c; i++ {
		if s.spCandAb(k, cs[i]).Repl.C&kop.ReplSPOptional != 0 {
			return true
		}
	}
	return false
}

// spOrderable reports whether the competition of c >= 2 candidates can be
// posed as the noncombat ask: no optional line, no DamageAmount$ gate (the
// amount changes as the picks apply and gorge re-collects), and no host with
// two shields (a pick names the host).
func (s *State) spOrderable(k *prep, cs *[spMax]spCand, c int) bool {
	for i := 0; i < c; i++ {
		a := s.spCandAb(k, cs[i])
		if a.Repl.C&(kop.ReplSPOptional|kop.ReplSPDmgLE) != 0 {
			return false
		}
		for j := 0; j < i; j++ {
			if cs[j].b == cs[i].b && (a.Repl.Kind == kop.ReplPreventDamageN ||
				s.spCandAb(k, cs[j]).Repl.Kind == kop.ReplPreventDamageN) {
				return false
			}
		}
	}
	return true
}

// spProtected: protection prevents the event before the replacements are
// collected (CR 702.16e; gorge emit), so no static prevention is a candidate.
func (s *State) spProtected(k *prep, e *spEv) bool {
	g := k.gen
	if e.sl == NoSlot {
		return g.anyRepl&rbPlProt != 0 && s.plProtected(k, e.srcCtrl, e.rcp, e.combat) || s.plProtAll(k, e.rcp) && !s.noPrev(k, e.combat)
	}
	return g.anyRepl&rbProtAll != 0 && s.kw(k, e.sl)&kop.KwProtectionAll != 0 && !s.noPrev(k, e.combat) ||
		g.anyRepl&rbProtCol != 0 && s.protFrom(k, e.sl, e.srcSl, e.srcCard, s.srcColors(k, e.srcSl, e.srcCard), e.combat)
}

// statPrevNC is replDamage's static-prevention step for noncombat damage n:
// the damage left. One candidate applies; several bare full preventions stop
// the event; any other competition (or an optional line) is a loud stall that
// the park (spPark) normally answers before this is reached.
func (s *State) statPrevNC(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, rcp uint8, sl Slot, combat bool, n int) int {
	if combat || n <= 0 || !k.gen.uses.has(useStaticPrev) {
		return n
	}
	e := spEv{srcCtrl, srcSl, srcCard, rcp, sl, false, n}
	m, ok := s.spApply(k, &e)
	if !ok {
		return n
	}
	return m
}

// spApply is the damage left of e after its static prevention candidates;
// false when there is none to apply (or after a stall).
func (s *State) spApply(k *prep, e *spEv) (int, bool) {
	var cs [spMax]spCand
	c := s.spCollect(k, e, &cs)
	switch {
	case c <= 0:
		return e.n, false
	case c == 1 && s.spSelfCand(k, cs[0]):
		return e.n, false // a lone Phantom is replDamage's selfDamageAll (repl_selfph.go)
	case c == 1:
		a := s.spCandAb(k, cs[0])
		if a.Repl.C&kop.ReplSPOptional != 0 {
			if s.g.ro.on && s.spWalkOpt(k, e) {
				// the frozen combat walk owns this ask (spWalkOpt: the event's
				// only candidate): the recomputed pass keeps the event whole
				// until the walk poses the "may" and the answer (pick or skip)
				// is dealt as asked (dealAsked); spPark poses a noncombat
				// event's own
				return e.n, false
			}
			s.capFull(StallReplPlus) // gorge asks the optional line's "may"; only a noncombat event is posed (spPark)
			return e.n, false
		}
		return e.n - s.spPrevented(k, cs[0].b, a, e.n), true
	case s.spBare(k, &cs, c):
		return 0, true
	case e.combat && s.spOrderable(k, &cs, c):
		// gorge's order ask is posed by the combat walk (spCombatCands), which
		// applies each pick as made (spPick). This is only the recomputed
		// pass's provisional accounting in scan order; dealAsked aligns the
		// event with the picked order (the candidates need not commute: an
		// all-but shield beside a prevent-N shield).
		left := e.n
		for i := 0; i < c; i++ {
			left -= s.spPrevented(k, cs[i].b, s.spCandAb(k, cs[i]), left)
		}
		return left, true
	}
	s.capFull(StallReplPlus) // gorge asks the CR 616.1 order; the walk poses only an orderable competition (spPark/spCombatCands)
	return e.n, false
}

// spCombatEv applies the static preventions to combat event ev of r (the
// source controlled by srcCtrl, the recipient code): this event only is
// prevented or reduced, so a source's other events and the recipient's other
// damage stand. true when it did.
func (s *State) spCombatEv(k *prep, r *dmgRec, ev *dmgEv, srcCtrl uint8, code TCode) bool {
	if !k.gen.uses.has(useStaticPrev) || ev.n <= 0 {
		return false
	}
	e := spEv{srcCtrl, ev.src, s.cardAt(ev.src), s.recipientCtl(ev.dst, code), ev.dst, true, int(ev.n)}
	if s.spProtected(k, &e) {
		return false
	}
	m, ok := s.spApply(k, &e)
	switch {
	case !ok || m == int(ev.n):
		return false
	case m <= 0:
		s.preventCombatEv(k, r, ev)
	default:
		s.combatEvLess(r, ev, code, int(ev.n)-m)
	}
	return true
}

// spPark parks noncombat damage n from the resolving entry (controller ctrl,
// source permanent srcSl or card srcCard) to code on gorge's ask when its
// static preventions call for one: two or more candidates that are not all
// bare full preventions, or a lone Optional$ line. handled is false when the
// event is no static prevention's to ask (parkNC goes on, replDamage applies a
// lone candidate or a bare competition), and parked is whether the event is
// parked. Like parkNC it stalls (handled) a second competing event of one op.
func (s *State) spPark(k *prep, ctrl uint8, srcSl Slot, srcCard CardID, code TCode, n int, kw kop.KwMask) (parked, handled bool) {
	if !k.gen.uses.has(useStaticPrev) || n <= 0 || n > 1<<14 || ctrl >= MaxSeats {
		return false, false
	}
	ro := &s.g.ro
	pending := s.g.nc.n != 0
	if ro.on || ro.n > 0 && !pending {
		return false, false
	}
	e, ok := s.spNCEvent(ctrl, srcSl, srcCard, code, n)
	if !ok {
		return false, false
	}
	if s.spProtected(k, &e) {
		return false, false
	}
	var cs [spMax]spCand
	c := s.spCollect(k, &e, &cs)
	if c < 0 {
		return false, true
	}
	if c == 0 {
		return false, false
	}
	lone := s.spCandAb(k, cs[0]).Repl.C&kop.ReplSPOptional != 0
	switch {
	case c == 1 && !lone, c >= 2 && s.spBare(k, &cs, c):
		return false, false // applied on its own (statPrevNC)
	case pending:
		s.capFull(StallReplPlus) // a second competing event of one op: gorge asks it after the first (parkNC)
		return false, true
	case c >= 2 && !s.spOrderable(k, &cs, c):
		s.capFull(StallReplPlus)
		return false, true
	}
	if cs[0].b >= MaxPerm {
		s.capFull(StallReplCands) // an emblem is never named by an order ask
		return false, true
	}
	hdr := replHdr{n: uint8(c), p: e.rcp, an: int16(n)}
	for i := 0; i < c; i++ {
		hdr.cand[i] = s.perms[cs[i].b].obj
		hdr.spv |= 1 << i
	}
	if c == 1 {
		hdr.spv |= spOptionalAsk
		hdr.p = s.perms[cs[0].b].ctrl // the line's OptionalDecider$ You: its controller answers
	}
	ro.replHdr = hdr
	nc := ncDmg{code: code, ctrl: ctrl, ll: kw&kop.KwLifelink != 0, dt: kw&kop.KwDeathtouch != 0, n: int16(n)}
	if srcSl != NoSlot {
		nc.src = s.perms[srcSl].obj
	} else {
		nc.src, nc.spell = s.res.e.obj, s.res.e.kind == seSpell
	}
	s.g.nc = nc
	return true, true
}

// spCombatCands appends the static prevention candidates of frozen combat
// event fe (source controller sc, recipient controller rcp) to the walk's
// ro.cand after the n already collected, marking them in ro.spv, and returns
// the new count (-1 after a loud stall). Two or more orderable candidates
// that are not all bare full preventions are gorge's CR 616.1 order ask; each
// pick is applied as made (spPick) to the amount left (ro.an). A competition
// beside another kind of candidate stalls (the kinds are not walked together),
// and one the walk cannot pose is left to spApply's loud stall.
func (s *State) spCombatCands(k *prep, fe *fzEv, sc, rcp uint8, n int) int {
	e := spEv{sc, fe.src, fe.card, rcp, fe.dst, true, int(fe.n)}
	if s.spProtected(k, &e) {
		return n
	}
	var cs [spMax]spCand
	c := s.spCollect(k, &e, &cs)
	switch {
	case c < 0:
		return -1
	case c >= 1 && n > 0 && s.spBesidePrevAll(n) && !s.spBare(k, &cs, c) && s.spOrderable(k, &cs, c):
		// a lone static prevention beside full preventions (a granted Effect's,
		// a printed one): gorge asks the order over all of them
	case n > 0 && s.spBesideAmounts(n) && (s.uses(k, useStaticOrdered) || selfPh(k.gen)) && (c == 0 || s.spOrderable(k, &cs, c)):
		// a doubler or a plus-N beside bare full preventions or all-but shields:
		// gorge asks the order over all of them (scan order)
		return s.spMergeAmounts(k, &cs, c, n)
	case c >= 1 && n == 0 && s.shieldLateFz(k, fe):
		// a static prevention or a Phantom's line beside the recipient's shield counter,
		// which the walk appends after them (repl_selfph.go replShieldCand): gorge asks the order
	case n > 0 && c == 1 && s.spSelfCand(k, cs[0]):
		s.capFull(StallReplPlus) // a Phantom beside a competition the walk does not merge: gorge asks the order of all
		return -1
	case c < 2 || s.spBare(k, &cs, c) || !s.spOrderable(k, &cs, c):
		if c == 1 && s.spCandAb(k, cs[0]).Repl.C&kop.ReplSPOptional != 0 {
			if n > 0 {
				// the optional line beside competing full preventions (a scoped
				// Effect's, a granted one's): gorge asks the CR 616.1 order over
				// all of them with the line's skip (spOptionalAsk); whichever
				// answer, a full prevention remains and leaves nothing dealt.
				// Anything else beside the line is not posed: never drop the
				// line and deal the event as the prevention alone
				if n >= spMax || cs[0].b >= MaxPerm || !s.spBesidePrevAll(n) {
					s.capFull(StallReplPlus)
					return -1
				}
				s.g.ro.cand[n] = s.perms[cs[0].b].obj
				s.g.ro.spv |= 1<<n | spOptionalAsk
				return n + 1
			}
			if cs[0].b >= MaxPerm {
				s.capFull(StallReplCands) // an emblem is never named by an ask
				return -1
			}
			// the lone Optional$ line's "may" (CR 616.1g): a candidate the
			// walk poses (spWalkOpt marks it for spApply's defer), not one
			// spApply auto-applies
			s.g.ro.cand[n] = s.perms[cs[0].b].obj
			s.g.ro.spv |= 1 << n
			n++
		} else if n > 0 && c >= 2 && s.spCandsOptional(k, &cs, c) {
			// an optional line inside a competition of two or more beside the
			// walk's candidates: gorge asks the CR 616.1 order over all of them
			// with a skip option, which the walk cannot pose
			s.capFull(StallReplPlus)
			return -1
		}
		return n
	case n > 0:
		s.capFull(StallReplPlus)
		return -1
	}
	ro := &s.g.ro
	if n+c > len(ro.cand) {
		s.capFull(StallReplCands) // loud: never truncate the candidates
		return -1
	}
	for i := 0; i < c; i++ {
		if cs[i].b >= MaxPerm {
			s.capFull(StallReplCands) // an emblem is never named by an order ask
			return -1
		}
		ro.cand[n+i] = s.perms[cs[i].b].obj
		ro.spv |= 1 << (n + i)
	}
	return n + c
}

// spWalkOpt reports whether the frozen walk poses the lone Optional$ line's
// "may" for this event: replOrderCandsFz collected exactly one candidate and
// spCombatCands marked it (the walk's shape of spPark's lone-optional ask;
// gorge asks the line's OptionalDecider$, CR 616.1g). It probes like
// walkAsksRepl, with the ask scratch saved and restored; the probe may stall
// loud (a competition the walk cannot name), as the walk's own collection
// would.
func (s *State) spWalkOpt(k *prep, e *spEv) bool {
	if !s.g.ro.on || e.srcSl == NoSlot {
		return false
	}
	save := s.g.ro
	fe := fzEv{src: e.srcSl, dst: e.sl, n: int16(min(e.n, 1<<14)), card: e.srcCard, ctrl: e.srcCtrl, apl: e.rcp}
	c := s.replOrderCandsFz(k, &fe)
	mark := s.g.ro.spv&1 != 0 // seat-ok: ro.spv is the walk's candidate bitmask (read before the restore discards the probe's collection)
	s.g.ro = save
	return c == 1 && mark
}

// DebugSpBesidePcolOff leaves a static prevention beside an Ext PreventDamageScope
// record unposed (the walk then asks the record alone and applies no order): the
// oracle mutation test. False in normal play.
var DebugSpBesidePcolOff bool

// spBesidePrevAll: the n candidates already collected are all full
// preventions (a granted Effect's, a printed one or a combat-only scoped Effect
// record such as Arachnogenesis's: repl_staticpreventpair.go usePrevColOpen)
// and nothing else (no shield, shield-counter, PreventNext shield, addition or
// doubling), so a
// static prevention beside them is posed in the same order ask: whichever is
// picked, the full prevention leaves nothing dealt.
func (s *State) spBesidePrevAll(n int) bool {
	ro := &s.g.ro
	return n < 8 && ro.prev == uint8(1<<n)-1 && ro.shld == 0 && ro.pnx == 0 && ro.plus == 0 && !(DebugSpBesidePcolOff && ro.pcol != 0)
}

// spAbility is the static prevention of the host at b a pick of candidate
// obj names: its shield if it has one (a host with a shield and a second
// candidate is not posed, spOrderable), else its full prevention.
func (s *State) spAbility(k *prep, b Slot) *kop.Ability {
	var full *kop.Ability
	ab := k.t.Cards[s.cardAt(b)].Abilities
	for j := range ab {
		switch {
		case isSelfPh(&ab[j]):
			return &ab[j] // a Phantom host: its line (repl_selfph.go)
		case !isStaticPrev(&ab[j]):
		case ab[j].Repl.Kind == kop.ReplPreventDamageN:
			return &ab[j]
		case full == nil:
			full = &ab[j]
		}
	}
	return full
}

// spSpend applies candidate i of the ask in progress to the damage left
// (replHdr.an).
func (s *State) spSpend(i int) {
	ro := &s.g.ro
	ob := &s.objs[ro.cand[i]]
	if ob.zone != kop.ZoneBattlefield {
		return
	}
	k := s.k()
	if a := s.spAbility(k, ob.slot); a != nil {
		ro.an -= int16(s.spPrevented(k, ob.slot, a, int(ro.an)))
		if isSelfPh(a) {
			s.spSelfRemove(ob.slot)
		}
	}
}

// spPick is replOrderPick for a static prevention candidate: it applies as
// picked (gorge applies each pick and re-collects the rest). The asks end when
// nothing is left; one candidate left applies on its own; two or more are
// asked again. Beside a doubler or a plus-N (spShieldAsk) the candidate left
// is the last one gorge applies without asking: a static one applies here, a
// parked noncombat event's amount changer here too (ncPlusPickNC), a combat
// event's is left to dealAsked, which applies it to the running amount.
func (s *State) spPick(i int) bool {
	ro := &s.g.ro
	s.spSpend(i)
	if ro.an <= 0 {
		return false
	}
	left, last := 0, -1
	for j := 0; j < int(ro.n); j++ {
		if ro.used&(1<<j) == 0 && s.spAskLive(j) {
			left++
			last = j
		}
	}
	if left == 1 && (ro.spv&(1<<last) != 0 || ro.spv&spShieldAsk != 0 && !ro.on) {
		ro.used |= 1 << last
		if ro.spv&(1<<last) != 0 {
			s.spSpend(last)
		} else {
			s.ncPlusPickNC(last)
		}
	}
	return left >= 2
}

// spAskLive: unused candidate j of the ask in progress is one gorge still
// collects at this pick (CR 616.1e): a combat candidate by its own liveness, a
// parked noncombat event's raise while the amount is below its host's power.
func (s *State) spAskLive(j int) bool {
	ro := &s.g.ro
	switch {
	case ro.spv&spShieldAsk == 0:
		return true // a static competition alone: every candidate is live (the ask's old count)
	case ro.on:
		return s.replCandLive(s.k(), j)
	}
	return s.ncPlusLive(j)
}

// spNC is the end of a parked noncombat event's ask: the candidates were
// applied as picked, so the damage left is what the event deals (nothing left:
// the ask was posed and the event is prevented, ncDmg.n -1).
func (s *State) spNC() {
	if ro := &s.g.ro; ro.spv != 0 {
		s.g.nc.n = max(ro.an, -1)
		if s.g.nc.n == 0 {
			s.g.nc.n = -1
		}
	}
}

// spOptions adds the skip option of an optional line's ask.
func (s *State) spOptions(d *Decision) {
	if ro := &s.g.ro; ro.spv&spOptionalAsk != 0 {
		d.add(MakeAction(AkChoose, TCode(NoTarget), 0, ro.n))
	}
}

// spBesideAmounts: the n candidates already collected are all doublers or
// plus-N additions (no prevention, shield or record of any kind).
func (s *State) spBesideAmounts(n int) bool {
	ro := &s.g.ro
	return n < 4 && ro.prev == 0 && ro.shld == 0 && ro.pnx == 0 && ro.pcol == 0 && !DebugSpBesideAmountsOff
}

// DebugSpBesideAmountsOff leaves a bare combat prevention out of the ask beside a
// doubler or a plus-N (the walk then asks the amount changers alone and the
// prevention applies after them): the oracle mutation test. False in normal play.
var DebugSpBesideAmountsOff bool

// uses reports a table-wide property of k.
func (s *State) uses(k *prep, u genUse) bool { return k.gen != nil && k.gen.uses.has(u) }

// spScanBefore: the host at a precedes the host at b in gorge's object scan,
// which the walk's collections mirror: seat order, then entry order.
func (s *State) spScanBefore(a, b Slot) bool {
	if pa, pb := s.perms[a].ctrl, s.perms[b].ctrl; pa != pb {
		return pa < pb
	}
	var bf [MaxPerm]Slot
	for _, x := range bf[:s.battlefieldOrder(s.perms[a].ctrl, s.statM.and(s.ctrlM[s.perms[a].ctrl]), bf[:], 0)] {
		if x == a {
			return true
		}
		if x == b {
			return false
		}
	}
	return false
}

// spMergeAmounts inserts the c bare static prevention candidates cs into the
// walk's n amount candidates (doublers and plus-N additions) at their scan
// position: gorge scans every object's replacements in one pass, so the order
// ask lists them interleaved (replacement_damage.go remainingDamageReplacements).
// It returns the new count (-1 after a loud stall).
func (s *State) spMergeAmounts(k *prep, cs *[spMax]spCand, c, n int) int {
	ro := &s.g.ro
	if n+c > len(ro.cand) {
		s.capFull(StallReplCands)
		return -1
	}
	var cand [4]Obj
	var plus, spv uint8
	i, j, m := 0, 0, 0
	for i < n || j < c {
		if j < c && cs[j].b >= MaxPerm {
			s.capFull(StallReplCands) // an emblem is never named by an order ask
			return -1
		}
		if i < n && (j == c || s.spScanBefore(s.objs[ro.cand[i]].slot, cs[j].b)) {
			cand[m] = ro.cand[i]
			if ro.plus&(1<<i) != 0 {
				plus |= 1 << m
			}
			i++
		} else {
			cand[m] = s.perms[cs[j].b].obj
			spv |= 1 << m
			j++
		}
		m++
	}
	copy(ro.cand[:m], cand[:m])
	ro.plus, ro.spv = plus, ro.spv|spv
	if s.spHasShield(k, cs, c) {
		ro.spv |= spShieldAsk
	}
	return m
}

// spCandLive: asked static prevention candidate i of the combat event still
// matches it: its host on the battlefield and the recipient still there (gorge
// re-collects after each pick, CR 616.1e).
func (s *State) spCandLive(k *prep, i int) bool {
	ro := &s.g.ro
	ob := &s.objs[ro.cand[i]]
	return ob.zone == kop.ZoneBattlefield && (ro.askD == NoSlot || s.occ.has(ro.askD)) && !s.noPrev(k, true)
}

// spApplyLeft applies the static prevention candidates still unpicked when an
// ask ends with fewer than two candidates left: gorge applies the last
// remaining one on its own.
func (s *State) spApplyLeft(k *prep) {
	ro := &s.g.ro
	for j := 0; j < int(ro.n); j++ {
		if b := uint8(1) << j; ro.spv&b != 0 && ro.used&b == 0 && s.spCandLive(k, j) {
			ro.used |= b
			s.spSpend(j)
		}
	}
}

// spMergeNC inserts the c static prevention candidates cs of a noncombat event
// into parkNC's cc amount candidates (doublers, plus-N additions, raises) at
// their scan position, as spMergeAmounts does for the combat walk: gorge scans
// every object's replacements in one pass. It returns the new count and the
// static candidates' mask (spShieldAsk set when one is an all-but shield), or
// -1 after a loud stall: more candidates than the walk names, an emblem (never
// named by an ask), or a competition the ask cannot pose (an optional line or a
// DamageAmount$ gate: spOrderable).
func (s *State) spMergeNC(k *prep, cs *[spMax]spCand, c int, cand *[4]Obj, cc int, plus, rs *uint8) (int, uint8) {
	if !s.spOrderable(k, cs, c) || cc+c > len(cand) {
		s.capFull(StallReplCands)
		return -1, 0
	}
	var out [4]Obj
	var np, nr, spv uint8
	i, j, m := 0, 0, 0
	for i < cc || j < c {
		if j < c && cs[j].b >= MaxPerm {
			s.capFull(StallReplCands) // an emblem is never named by an order ask
			return -1, 0
		}
		if i < cc && (j == c || s.spScanBefore(s.objs[cand[i]].slot, cs[j].b)) {
			out[m] = cand[i]
			if *plus&(1<<i) != 0 {
				np |= 1 << m
			}
			if *rs&(1<<i) != 0 {
				nr |= 1 << m
			}
			i++
		} else {
			out[m] = s.perms[cs[j].b].obj
			spv |= 1 << m
			j++
		}
		m++
	}
	*cand, *plus, *rs = out, np, nr
	if s.spHasShield(k, cs, c) {
		spv |= spShieldAsk
	}
	return m, spv
}

// spNCEvent is the static prevention's view of noncombat damage n from the
// resolving entry (controller ctrl, source permanent srcSl or card srcCard) to
// code: false when the recipient is neither a player nor a permanent.
func (s *State) spNCEvent(ctrl uint8, srcSl Slot, srcCard CardID, code TCode, n int) (spEv, bool) {
	e := spEv{srcCtrl: ctrl, srcSl: srcSl, srcCard: srcCard, rcp: 255, sl: NoSlot, n: n}
	if isPlayerTarget(code) {
		e.rcp = CodeSeat(code)
	} else if int(code) < MaxObj && s.objs[code].zone == kop.ZoneBattlefield {
		e.sl = s.objs[code].slot
		e.rcp = s.perms[e.sl].ctrl
	} else {
		return e, false
	}
	return e, true
}

// DebugSpNCMergeOff leaves the static prevention out of a parked noncombat
// event's ask beside an amount changer (the walk then asks the changers alone and
// the prevention applies after them): the oracle mutation test. False in normal play.
var DebugSpNCMergeOff bool

// parkNCStatic is parkNC's static prevention step for a table whose static
// preventions are the ordered shapes (useStaticOrdered): the c candidates
// already collected (amount changers; other is the mask of any prevention among
// them) get the event's static candidates merged in at their scan position
// (spMergeNC). With no other candidate the static competition alone is spPark's
// ask. done is 0 to go on with the returned count and static mask, 1 when the
// event is dealt now and 2 when it is parked; the count is -1 after a loud stall.
func (s *State) parkNCStatic(k *prep, ctrl uint8, srcSl Slot, srcCard CardID, code TCode, n int, kw kop.KwMask, c int, other uint8, cand *[4]Obj, plus, rs *uint8) (int, uint8, uint8) {
	if !(k.gen.uses.has(useStaticOrdered) || selfPh(k.gen)) || DebugSpNCMergeOff {
		return c, 0, 0
	}
	c, spv, done := s.parkNCStaticMerge(k, ctrl, srcSl, srcCard, code, n, kw, c, other, cand, plus, rs)
	if c < 0 {
		return c, 0, 1 // a loud stall: the event is not dealt here
	}
	return c, spv, done
}

// parkNCStaticMerge is parkNCStatic for an ordered table.
func (s *State) parkNCStaticMerge(k *prep, ctrl uint8, srcSl Slot, srcCard CardID, code TCode, n int, kw kop.KwMask, c int, other uint8, cand *[4]Obj, plus, rs *uint8) (int, uint8, uint8) {
	if c == 0 && !s.shieldLateAt(k, code) {
		parked, handled := s.spPark(k, ctrl, srcSl, srcCard, code, n, kw)
		switch {
		case parked:
			return c, 0, 2
		case handled:
			return c, 0, 1
		}
		return c, 0, 0
	}
	e, ok := s.spNCEvent(ctrl, srcSl, srcCard, code, n)
	if !ok || s.spProtected(k, &e) {
		return c, 0, 0
	}
	var cs [spMax]spCand
	sc := s.spCollect(k, &e, &cs)
	switch {
	case sc < 0:
		return -1, 0, 0
	case sc == 0:
		return c, 0, 0
	case other != 0:
		s.capFull(StallReplPlus) // a prevention beside the competition: not walked together
		return -1, 0, 0
	}
	m, spv := s.spMergeNC(k, &cs, sc, cand, c, plus, rs)
	return m, spv, 0
}

// spParkEarly is parkNC's first look at a static prevention's own ask: for a
// table whose static preventions are not the ordered shapes (the ordered ones are
// merged into the amount changers' ask, parkNCStatic).
func (s *State) spParkEarly(k *prep, ctrl uint8, srcSl Slot, srcCard CardID, code TCode, n int, kw kop.KwMask) (parked, handled bool) {
	if k.gen.uses.has(useStaticOrdered) {
		return false, false
	}
	return s.spPark(k, ctrl, srcSl, srcCard, code, n, kw)
}

// parkNCNoKinds: no damage replacement kind parkNC walks can match, so it parks
// nothing (a table of ordered static preventions always looks).
func (s *State) parkNCNoKinds(g *genPrep, plusOnly bool) bool {
	return !plusOnly && !g.uses.has(useStaticOrdered) && !selfPh(g) && g.anyRepl&(rbDblDmg|rbPrevNC|rbShield|rbPrevCol|rbPrevDmgTo) == 0
}

// parkNCStaticStall stalls loud a static prevention beside the amount changers
// that the pend walk could not carry (pendBlocked): the park built without it
// would be wrong.
func (s *State) parkNCStaticStall(spv uint8) bool {
	if spv == 0 {
		return false
	}
	s.capFull(StallReplPlus)
	return true
}
