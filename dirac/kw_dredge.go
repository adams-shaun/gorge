package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Dredge N (CR 702.52, kop.KwDredge; CardDef.Dredge = N): "If you would draw
// a card, you may mill N cards instead. If you do, return this card from your
// graveyard to your hand." Only with at least N cards in the library
// (702.52b). gorge (engine/effects cardflow.go drawFor) asks before every
// draw whose player has such a card in their graveyard: a KModes decision
// whose options are each candidate in graveyard order ("dredge", the card),
// then the ordinary draw ("draw"); a multi-card draw asks again before each
// card. The pick mills N (each card an ordinary library-to-graveyard move)
// and returns the card to its owner's hand (rules applyDredge); no card is
// drawn, so nothing counts as a draw. A decline is the ordinary draw, which
// then meets the draw replacements (repl_draw.go). The ask comes before a
// CantDraw static is read, as gorge's does.
//
// The kernel poses the same decision (DkChoose ChGenBranch, mode i = the
// i-th candidate, the last mode the draw: canonical as gorge's KModes
// index) at the turn draw (dredgeTurnAsk, ctlGenAsk), at each card of an
// OpDraw (dredgeDraws, the resolution ask raDredge) and at every other
// effect-draw site (raDredgeRes, below: connive, Recruit, the monarch's
// end-step draw, a Draw Upto's body). A draw paid as a resolution cost is
// not an effect draw: gorge's draw-bearing triggered cost (rules
// payDrawCostParts) draws plainly, so the kernel latches each of its cards
// past the guard. A draw-replacement body (drawExcl with a chain) still
// stalls: its resume needs the lineage's applied chain, which no res frame
// holds (repl_draw.go).

// kwDredge is the manifest keyword CardDef.Dredge stands for.
const kwDredge = kop.KwDredge

// StallDredge: a draw the kernel cannot suspend on (not the turn draw, an
// OpDraw card, nor a site that poses raDredgeRes) meets a dredge candidate:
// a draw-replacement body (repl_draw.go) or another unresumable path; gorge
// would ask.
const StallDredge = 178

func init() { stallNames[StallDredge] = "dredge" }

// raDredge (dirac/manifest/ask/raDredge) is OpDraw's dredge ask: res.askP
// (= s.g.pickP) draws, res.left cards are left for them counting the asked
// one, s.g.pickLeft is the count per player and s.g.pickZ the players still
// to draw (seatsPack).

// cfgxDredgeAsked is transient, never a Config bit: the draw about to run is
// an answered ask's ordinary draw, so drawExcl's unasked-candidate stall
// (dredgeHeld) lets it through once.
const cfgxDredgeAsked uint8 = 1 << 4

// useDredge: the table has a card with printed Dredge.
var useDredge = newGenUse(func(t *kop.Table) bool {
	for i := range t.Cards {
		if t.Cards[i].Dredge != 0 {
			return true
		}
	}
	return false
})

// dredgeMaxCands bounds one ask's candidates (the mode byte, the draw last).
const dredgeMaxCands = 255

// dredgeOn reports whether the table has a dredge card.
func dredgeOn(k *prep) bool { return k.gen != nil && k.gen.uses.has(useDredge) }

// dredgeCands lists player p's graveyard cards with Dredge N at most their
// library size, in graveyard order (gorge dredgeCandidates), into buf (nil:
// only counted).
func (s *State) dredgeCands(k *prep, p uint8, buf *[dredgeMaxCands]Obj) int {
	pl := &s.pl[p]
	lib := pl.libLen()
	n := 0
	for _, o := range pl.grave[:pl.graveN] {
		d := int(k.t.Cards[s.objs[o].card].Dredge)
		if d == 0 || d > lib || n == dredgeMaxCands {
			continue
		}
		if buf != nil {
			buf[n] = o
		}
		n++
	}
	return n
}

// dredgeOptions poses the ask to p: one option per candidate (mode = its
// index), then the ordinary draw (mode = the candidate count).
func (s *State) dredgeOptions(k *prep, p uint8, d *Decision) {
	d.Kind, d.Sub, d.Player = DkChoose, ChGenBranch, p
	n := s.dredgeCands(k, p, nil)
	for i := 0; i <= n; i++ {
		d.add(MakeAction(AkChoose, 0, 0, uint8(i)))
	}
}

// dredgeTake applies answer mode to p's asked draw: a candidate's index
// mills its N and returns it to hand (gorge applyDredge), true; the draw's
// mode returns false, the caller then draws.
func (s *State) dredgeTake(k *prep, p uint8, mode uint8) bool {
	var buf [dredgeMaxCands]Obj
	if int(mode) >= s.dredgeCands(k, p, &buf) {
		return false
	}
	o := buf[mode]
	n := int(k.t.Cards[s.objs[o].card].Dredge)
	pl := &s.pl[p]
	switch {
	case k.gen.anyRepl&rbGraveLib != 0:
		s.millShuffling(k, p, n, 0)
	case len(s.cold.parked) != 0 && s.parkMills(k, p, n, 0):
	default:
		for i := 0; i < n && pl.libLo < pl.libHi; i++ {
			m := pl.lib[pl.libLo]
			pl.libLo++
			s.touch(m)
			s.objs[m].zone = kop.ZoneNone
			s.placeFromK(k, m, kop.ZoneGraveyard, false, kop.ZoneLibrary, false)
		}
	}
	if z := s.objs[o].zone; z != kop.ZoneHand && z != kop.ZoneNone && z != kop.ZoneBattlefield {
		from := s.detach(o)
		s.placeFromK(k, o, kop.ZoneHand, false, from, false)
	}
	return true
}

// dredgeHeld is drawExcl's guard: a draw that no ask was posed for, by a
// player with a dredge candidate, stalls (true: the draw is not made). The
// ordinary draw of an answered ask passes once.
func (s *State) dredgeHeld(p uint8) bool {
	if s.cfgx&cfgxDredgeAsked != 0 {
		s.cfgx &^= cfgxDredgeAsked
		return false
	}
	k := s.k()
	if !dredgeOn(k) || s.dredgeCands(k, p, nil) == 0 {
		return false
	}
	s.capFull(StallDredge)
	return true
}

// --- the turn draw (CR 504.1) ---

// dredgeTurnAsk suspends the draw step on the active player's dredge ask,
// before the turn draw; false: nothing to ask, the step draws as usual.
// The ask is known by genState.drawFirst, true only from the step's start to
// its draw (dredgeTurnAsked).
func (s *State) dredgeTurnAsk(k *prep) bool {
	if !dredgeOn(k) || s.dredgeCands(k, s.active, nil) == 0 {
		return false
	}
	s.g.ctlRet, s.ctl = ctlStepBegin, ctlGenAsk
	return true
}

// dredgeTurnAsked reports whether the pending generated ask is the turn
// draw's dredge ask.
func (s *State) dredgeTurnAsked() bool {
	return s.g.drawFirst && s.step == kop.StepDraw && s.ctl == ctlGenAsk
}

// dredgeTurnAnswer applies the turn draw's answer and finishes the draw step
// as beginStep does after its draw.
func (s *State) dredgeTurnAnswer(k *prep, a Action) {
	s.ctl = ctlStepBegin
	if !s.dredgeTake(k, s.active, a.Mode()) {
		s.cfgx |= cfgxDredgeAsked
		s.draw(s.active)
		s.cfgx &^= cfgxDredgeAsked
	}
	s.g.drawFirst = false
	if s.over {
		return
	}
	s.firePhase(k, kop.StepDraw)
	s.openPriority()
}

// --- OpDraw ---

// drawOp is OpDraw: in.A cards for each player in.Obj names; in a dredge
// table each card asks first (dredgeDraws). false: an ask is posed.
func (s *State) drawOp(k *prep, in *kop.Instr) bool {
	if dredgeOn(k) {
		return s.dredgeDraws(k, in)
	}
	q := s.players(k, in.Obj)
	n := s.value(k, in.A)
	for i := uint8(0); i < q.n; i++ {
		for j := 0; j < n && !s.over; j++ {
			s.dredgeDrawOne(in, q.s[i])
		}
	}
	return true
}

// dredgeDraws is OpDraw in a dredge table: in.A cards for each player in.Obj
// names, each card asked first when its player has a candidate; false: an
// ask is posed.
func (s *State) dredgeDraws(k *prep, in *kop.Instr) bool {
	q := s.players(k, in.Obj)
	if q.n == 0 {
		return true
	}
	n := min(max(s.value(k, in.A), 0), 255)
	return s.dredgeDrawLoop(k, in, q.s[0], n, seatsPack(q, 1), n)
}

// dredgeDrawLoop draws left cards for p, then n for each player of the
// chain rest, asking before each card that has a candidate.
func (s *State) dredgeDrawLoop(k *prep, in *kop.Instr, p uint8, left int, rest uint8, n int) bool {
	for {
		for ; left > 0 && !s.over; left-- {
			if s.dredgeCands(k, p, nil) != 0 {
				s.g.pickP, s.g.pickZ, s.g.pickLeft = p, rest, uint8(n)
				s.res.left = uint8(left)
				s.res.ask, s.res.askP = raDredge, p
				return false
			}
			s.dredgeDrawOne(in, p)
		}
		if s.over {
			return true
		}
		if p = seatsPop(&rest); p == 255 {
			return true
		}
		left = n
	}
}

// dredgeDrawOne is one card of OpDraw: RememberDrawn$ draws remember
// (zone_rememberwrites.go).
func (s *State) dredgeDrawOne(in *kop.Instr, p uint8) {
	if in.C&kop.MvRemember != 0 {
		s.drawRemember(p, in.C&kop.DrawRememberAll != 0)
		return
	}
	s.draw(p)
}

// dredgeAskOptions poses OpDraw's dredge ask.
//
//kmanifest:askopts raDredge
func (s *State) dredgeAskOptions(k *prep, d *Decision) {
	s.dredgeOptions(k, s.res.askP, d)
}

// dredgeAskAnswer applies OpDraw's dredge answer and draws on.
//
//kmanifest:askapply raDredge
func (s *State) dredgeAskAnswer(k *prep, a Action) {
	s.res.ask = raNone
	p := s.g.pickP
	in := &s.prog(k)[s.res.pc]
	if !s.dredgeTake(k, p, a.Mode()) {
		s.cfgx |= cfgxDredgeAsked
		s.dredgeDrawOne(in, p)
		s.cfgx &^= cfgxDredgeAsked
	}
	if s.dredgeDrawLoop(k, in, p, int(s.res.left)-1, s.g.pickZ, int(s.g.pickLeft)) {
		s.continueRes(k, 0)
	}
}

// --- the other effect-draw sites (gorge drawFor asks before each) ---

// raDredgeRes (dirac/manifest/ask/raDredgeRes) is a non-OpDraw effect-draw
// site's dredge ask: res.askP the drawer, res.left the cards its site still
// owes counting the asked one, res.chosen the site (dredgeRes*). The site
// suspends on it (dredgeResPose); the answer's ordinary draw passes
// dredgeHeld once (cfgxDredgeAsked), the rest of the batch is drawn through
// dredgeResAnswer, and the site's tail (dredgeResTail) finishes it. Each
// site keeps its own tail state where it lives: connive's count in g.conn,
// Recruit's in res.e, Draw Upto's and the monarch's in nothing.
const (
	dredgeResConnive objWord = 1 // act_connive.go: the count and discard after the draws
	dredgeResRecruit objWord = 2 // act_recruit.go: the discard after the draw
	dredgeResUpto    objWord = 3 // act_drawupto.go: the program goes on
	dredgeResMonarch objWord = 4 // act_monarch.go: the delayed trigger finishes
)

// dredgeResPose poses the dredge ask of a non-OpDraw draw site at p for its
// left remaining cards; true: no candidate, the caller draws plainly.
func (s *State) dredgeResPose(k *prep, p uint8, site objWord, left int) bool {
	if !dredgeOn(k) || s.dredgeCands(k, p, nil) == 0 {
		return true
	}
	s.res.ask, s.res.askP, s.res.left = raDredgeRes, p, uint8(left)
	s.res.chosen = site
	return false
}

// dredgeResDraws draws n cards for p at a non-OpDraw draw site, asking the
// site's dredge ask before each card that has a candidate; false: an ask is
// posed and the caller suspends.
func (s *State) dredgeResDraws(k *prep, p uint8, site objWord, n int) bool {
	for i := 0; i < n && !s.over; i++ {
		if !s.dredgeResPose(k, p, site, n-i) {
			return false
		}
		s.draw(p)
	}
	return true
}

// dredgeResOptions poses a non-OpDraw draw site's dredge ask.
//
//kmanifest:askopts raDredgeRes
func (s *State) dredgeResOptions(k *prep, d *Decision) {
	s.dredgeOptions(k, s.res.askP, d)
}

// dredgeResAnswer applies a non-OpDraw draw site's dredge answer: the taken
// candidate, or the ordinary draw (latched past dredgeHeld once), then the
// rest of the site's batch and its tail.
//
//kmanifest:askapply raDredgeRes
func (s *State) dredgeResAnswer(k *prep, a Action) {
	p, site := s.res.askP, s.res.chosen
	s.res.ask = raNone
	if !s.dredgeTake(k, p, a.Mode()) {
		s.cfgx |= cfgxDredgeAsked
		s.draw(p)
		s.cfgx &^= cfgxDredgeAsked
	}
	left := int(s.res.left) - 1
	s.res.left = 0
	for left > 0 && !s.over {
		if !s.dredgeResPose(k, p, site, left) {
			return // the ask is posed again for the rest of the batch
		}
		s.draw(p)
		left--
	}
	s.dredgeResTail(k, site, p)
}

// dredgeResTail finishes the site whose batch is drawn.
func (s *State) dredgeResTail(k *prep, site objWord, p uint8) {
	switch site {
	case dredgeResConnive:
		s.conniveTail(k, p, int(s.g.conn), true) // g.conn holds the drawn count (act_connive.go)
	case dredgeResRecruit:
		s.recruitTail(k, true) // act_recruit.go
	case dredgeResUpto:
		s.continueRes(k, 0)
	case dredgeResMonarch:
		if k.gen != nil && s.parkReturnAsk() {
			return // the mill parked a return's ask (zone_untilleavesbatch.go)
		}
		if s.g.nask != 0 && s.midResAsk(k) {
			return // the answered card's as-enters choice (zone_untilleavesbatch.go)
		}
		s.finishResolve(k, false)
	default:
		panic("kernel: bad dredge draw site")
	}
}
