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
// noncombat event with two or more other candidates parks on the ask
// (spPark): the options name each host, each pick is applied as made to the
// damage left (replHdr.an), the last candidate applies on its own and a
// prevention that leaves nothing ends the asks. An Optional$ shield alone
// (Battletide Alchemist) is asked of its controller with a skip option. A
// combat event, a competition beside an optional line or a DamageAmount$ gate,
// and a host with two shields stall StallReplPlus: the order walk of a combat
// pass does not carry these candidates. A table that also has any other
// damage replacement (or a self-damage one) is refused at its start
// (initStall: staticPrevStall), since the kinds do not commute.

// useStaticPrev: the table has a static damage prevention.
var useStaticPrev = newGenUse(tableHasStaticPrev)

// spMax bounds the candidates of one event; more stall StallReplCands.
const spMax = 4

// spOptionalAsk is replHdr.spv's bit for an ask whose lone candidate is an
// Optional$ line: a skip option follows the candidate.
const spOptionalAsk uint8 = 1 << 7

func tableHasStaticPrev(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if isStaticPrev(&t.Cards[i].Abilities[j]) {
				return true
			}
		}
	}
	return false
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
	r := &a.Repl
	c := r.C
	hc := s.perms[b].ctrl
	if c&kop.ReplSPCombat != 0 && !e.combat || c&kop.ReplSPNonCombat != 0 && e.combat ||
		c&kop.ReplSPYourTurn != 0 && s.active != hc || c&kop.ReplSPSrcInPlay != 0 && e.srcSl == NoSlot ||
		c&kop.ReplSPDmgLE != 0 && e.n > int(r.A) || c&kop.ReplSPSrcSpell != 0 && (e.srcSl != NoSlot || s.res.e.kind != seSpell) {
		return false
	}
	ctx := evalCtx{src: s.perms[b].obj, srcSl: b, ctrl: hc}
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
	return s.matchCardGenAt(k, r.Filter2, e.srcCard, e.srcCtrl, hc, NoObj, &ctx)
}

// spCollect fills out with the candidates of e in gorge's scan order (each
// seat's battlefield in entry order, the abilities in print order) and returns
// their number; -1 after a loud stall (more than spMax). A can't-prevent static
// (CR 615.12) leaves none.
func (s *State) spCollect(k *prep, e *spEv, out *[spMax]spCand) int {
	if s.noPrev(k, e.combat) {
		return 0
	}
	g := k.gen
	n := 0
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		var bf [MaxPerm]Slot
		for _, b := range bf[:s.battlefieldOrder(p, s.statM.and(s.ctrlM[p]), bf[:], 0)] {
			if g.replBits[s.cardAt(b)]&rbPlusDmg == 0 || s.genLost(k, b) {
				continue
			}
			ab := k.t.Cards[s.cardAt(b)].Abilities
			for j := range ab {
				if !isStaticPrev(&ab[j]) || !s.spMatch(k, b, &ab[j], e) {
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
	return n
}

// spPrevented is the damage the shield a of the host at b prevents from the
// left that remains: all of it for a full prevention, else min(amount, left).
func (s *State) spPrevented(k *prep, b Slot, a *kop.Ability, left int) int {
	if a.Repl.Kind == kop.ReplPreventDamageFilt {
		return left
	}
	r := &a.Repl
	var m int
	switch c := r.C; {
	case c&kop.ReplSPAllBut != 0:
		m = left - int(r.A)
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
		a := &k.t.Cards[s.cardAt(cs[i].b)].Abilities[cs[i].ab]
		if a.Repl.Kind != kop.ReplPreventDamageFilt || a.Repl.C&kop.ReplSPOptional != 0 {
			return false
		}
	}
	return true
}

// spOrderable reports whether the competition of c >= 2 candidates can be
// posed as the noncombat ask: no optional line, no DamageAmount$ gate (the
// amount changes as the picks apply and gorge re-collects), and no host with
// two shields (a pick names the host).
func (s *State) spOrderable(k *prep, cs *[spMax]spCand, c int) bool {
	for i := 0; i < c; i++ {
		a := &k.t.Cards[s.cardAt(cs[i].b)].Abilities[cs[i].ab]
		if a.Repl.C&(kop.ReplSPOptional|kop.ReplSPDmgLE) != 0 {
			return false
		}
		for j := 0; j < i; j++ {
			if cs[j].b == cs[i].b && (a.Repl.Kind == kop.ReplPreventDamageN ||
				k.t.Cards[s.cardAt(cs[i].b)].Abilities[cs[j].ab].Repl.Kind == kop.ReplPreventDamageN) {
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
	case c == 1:
		a := &k.t.Cards[s.cardAt(cs[0].b)].Abilities[cs[0].ab]
		if a.Repl.C&kop.ReplSPOptional != 0 {
			s.capFull(StallReplPlus) // gorge asks the optional line's "may"; only a noncombat event is posed (spPark)
			return e.n, false
		}
		return e.n - s.spPrevented(k, cs[0].b, a, e.n), true
	case s.spBare(k, &cs, c):
		return 0, true
	}
	s.capFull(StallReplPlus) // gorge asks the CR 616.1 order; only a noncombat event is posed (spPark)
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
	e := spEv{srcCtrl: ctrl, srcSl: srcSl, srcCard: srcCard, rcp: 255, sl: NoSlot, n: n}
	if isPlayerTarget(code) {
		e.rcp = CodeSeat(code)
	} else if int(code) < MaxObj && s.objs[code].zone == kop.ZoneBattlefield {
		e.sl = s.objs[code].slot
		e.rcp = s.perms[e.sl].ctrl
	} else {
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
	lone := k.t.Cards[s.cardAt(cs[0].b)].Abilities[cs[0].ab].Repl.C&kop.ReplSPOptional != 0
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

// spAbility is the static prevention of the host at b a pick of candidate
// obj names: its shield if it has one (a host with a shield and a second
// candidate is not posed, spOrderable), else its full prevention.
func (s *State) spAbility(k *prep, b Slot) *kop.Ability {
	var full *kop.Ability
	ab := k.t.Cards[s.cardAt(b)].Abilities
	for j := range ab {
		switch {
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
	}
}

// spPick is replOrderPick for a static prevention candidate: it applies as
// picked (gorge applies each pick and re-collects the rest). The asks end when
// nothing is left; one candidate left applies on its own; two or more are
// asked again.
func (s *State) spPick(i int) bool {
	ro := &s.g.ro
	s.spSpend(i)
	if ro.an <= 0 {
		return false
	}
	left, last := 0, -1
	for j := 0; j < int(ro.n); j++ {
		if ro.used&(1<<j) == 0 {
			left++
			last = j
		}
	}
	if left == 1 {
		ro.used |= 1 << last
		s.spSpend(last)
	}
	return left >= 2
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
